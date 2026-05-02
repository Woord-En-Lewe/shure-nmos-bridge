package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/grandcat/zeroconf"
)

// DiscoveryMode defines how the registry is discovered
type DiscoveryMode string

const (
	DiscoveryModeDNSSD  DiscoveryMode = "dns_sd"
	DiscoveryModeMDNS                 = "mdns"
	DiscoveryModeStatic               = "static"
)

// RegistryDiscoveryConfig configures how the NMOS registry is discovered
type RegistryDiscoveryConfig struct {
	Mode           DiscoveryMode // Discovery mode: dns_sd, mdns, or static
	Domain         string        // DNS domain for DNS-SD discovery (e.g., "local.", "example.com.")
	StaticRegistry string        // Static registry URL when Mode is static
}

// ncpClient represents a connected NCP client with synchronization for WebSocket writes
type ncpClient struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (c *ncpClient) WriteJSON(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteJSON(v)
}

func (c *ncpClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.Close()
}

func (c *ncpClient) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

func (c *ncpClient) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

type nmosController struct {
	registryURL    string
	registryConfig RegistryDiscoveryConfig
	nodeAddr       string
	nodeID         string
	advertisedAddr string // externally reachable address for hrefs (if set, overrides listenAddr)
	httpClient     *http.Client
	httpServer     *http.Server
	isRunning      bool
	listenAddr     string
	nodes          []interface{}
	db             Database
	resources      map[string][]interface{} // Fallback in-memory map when db is nil
	deviceControls map[string][]map[string]interface{}
	eventsChan     chan interface{}
	done           chan struct{}
	wg             sync.WaitGroup
	mu             sync.RWMutex

	// Websocket support (IS-07)
	upgrader         websocket.Upgrader
	clients          map[*websocket.Conn]map[string]bool // client -> sourceID -> subscribed
	lastEvents       map[string]map[string]interface{}   // sourceID -> last event message
	clientLastHealth map[*websocket.Conn]time.Time       // client -> last health timestamp

	// Registry discovery
	registryResolved chan string
	resolver         *zeroconf.Resolver
	ctx              context.Context
	cancel           context.CancelFunc

	controlCallback func(deviceID, controlID string, value interface{})

	// Heartbeat
	heartbeatInterval time.Duration

	// IS-12 NCP support
	ncpObjects         map[int]NcObject
	ncpMu              sync.RWMutex
	ncpClients         map[*ncpClient]bool
	ncpClientsMu       sync.Mutex
	ncpSubscriptions   map[*ncpClient]map[int]int // client -> (oid -> subscriptionID)
	ncpSubMu           sync.RWMutex
	nextSubscriptionID int

	// mDNS advertisement
	nodeServer   *zeroconf.Server
	eventsServer *zeroconf.Server
	ncpServer    *zeroconf.Server

	// Connection Management (IS-05)
	stagedConnections map[string]ConnectionStaged // resourceID -> staged state
	activeConnections map[string]ConnectionActive // resourceID -> active state

	// IS-04 Registration state
	registered   bool
	registeredMu sync.RWMutex

	// Network interfaces for IS-04 node self resource
	nodeInterfaces []map[string]interface{}

	// Registry monitoring state
	monitoringRegistry  bool
	registryMonitorMu   sync.RWMutex
	currentRegistryURL  string

	// Peer-to-peer mode version counters (for mDNS TXT records)
	// These increment when resources change and wrap at 255
	peerToPeerMu      sync.RWMutex
	versionCounters   map[string]uint8 // resource type -> counter
	peerToPeerMode    bool              // true when operating without registry
}

// NewNMOSController creates a new NMOSController instance with default DNS-SD/mDNS discovery
func NewNMOSController(addr string) NMOSController {
	return NewNMOSControllerWithConfig(addr, RegistryDiscoveryConfig{
		Mode:   DiscoveryModeMDNS,
		Domain: "local.",
	})
}

// NewNMOSControllerWithConfig creates a new NMOSController instance with custom registry discovery config
func NewNMOSControllerWithConfig(addr string, registryConfig RegistryDiscoveryConfig) NMOSController {
	if addr == "" {
		addr = "0.0.0.0:8080" // Default NMOS Node API address
	}
	ctx, cancel := context.WithCancel(context.Background())
	nodeID := uuid.New().String()
	ctrl := &nmosController{
		nodeAddr:           addr,
		nodeID:             nodeID,
		registryURL:        "http://localhost:8000", // Default NMOS registry address
		registryConfig:     registryConfig,
		httpClient:         &http.Client{Timeout: 10 * time.Second},
		heartbeatInterval:  5 * time.Second,
		db:                  nil, // Set via SetDatabase
		resources:           make(map[string][]interface{}), // Fallback when db is nil
		deviceControls:     make(map[string][]map[string]interface{}),
		ncpObjects:         make(map[int]NcObject),
		ncpClients:         make(map[*ncpClient]bool),
		ncpSubscriptions:   make(map[*ncpClient]map[int]int),
		nextSubscriptionID: 1,
		eventsChan:         make(chan interface{}, 100),
		done:               make(chan struct{}),
		clients:            make(map[*websocket.Conn]map[string]bool),
		lastEvents:         make(map[string]map[string]interface{}),
		clientLastHealth:   make(map[*websocket.Conn]time.Time),
		registryResolved:   make(chan string, 1),
		ctx:                ctx,
		cancel:             cancel,
		stagedConnections:  make(map[string]ConnectionStaged),
		activeConnections:  make(map[string]ConnectionActive),
		versionCounters:    map[string]uint8{"self": 0, "sources": 0, "flows": 0, "devices": 0, "senders": 0, "receivers": 0},
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for the gateway
			},
		},
	}

	// Discover and cache network interfaces for IS-04 node self resource
	ctrl.discoverNodeInterfaces()

	// Register Root Block (OID 1)
	rootBlock := NewNcBlock(1, nil, "Root", "Root Block")
	ctrl.RegisterNCPObject(1, rootBlock)

	// Register Device Manager (OID 2)
	deviceManager := NewNcDeviceManager(2, nil)
	ctrl.RegisterNCPObject(2, deviceManager)
	rootBlock.Items = append(rootBlock.Items, 2)

	// Register Class Manager (OID 3)
	classManager := NewNcClassManager(3, nil)
	ctrl.RegisterNCPObject(3, classManager)

	// Add ClassManager to RootBlock items
	rootBlock.Items = append(rootBlock.Items, 3)

	return ctrl
}

// Start begins the NMOS controller
func (c *nmosController) Start(ctx context.Context) error {
	if c.isRunning {
		return nil
	}

	c.isRunning = true

	// Start the Node API server
	if err := c.startServer(); err != nil {
		return fmt.Errorf("failed to start NMOS Node API server: %w", err)
	}

	// Start mDNS advertisement
	if err := c.startMDNS(); err != nil {
		slog.Warn("Failed to start mDNS advertisement", "error", err)
	}

	// Start registry discovery and registration
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.discoverAndRegister(ctx)
	}()

	// Wait for registration to complete (or timeout)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.registryResolved:
		slog.Info("NMOS node registered with registry", "registry", c.registryURL)
	case <-time.After(30 * time.Second):
		slog.Warn("Registry discovery timed out, continuing without registration")
	}

	// Start goroutine to listen for NMOS events (IS-05)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.listenForEvents(ctx)
	}()

	// Start IS-07 WebSocket heartbeat timeout checker (IS-07: 12s timeout)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.checkIS07Heartbeats(ctx)
	}()

	// Start IS-07 WebSocket ping goroutine to keep connections alive
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.pingIS07Clients(ctx)
	}()

	return nil
}

// discoverAndRegister discovers NMOS registries via mDNS and registers this node
func (c *nmosController) discoverAndRegister(ctx context.Context) {
	c.registryMonitorMu.Lock()
	c.monitoringRegistry = true
	c.registryMonitorMu.Unlock()

	registryURL := c.registryURL
	if url, err := c.discoverRegistry(ctx); err == nil {
		registryURL = url
		c.registryURL = url
		c.currentRegistryURL = url
		slog.Info("Discovered NMOS registry", "url", registryURL)
	} else {
		slog.Warn("No registry found via mDNS, operating in peer-to-peer mode", "error", err)
		c.currentRegistryURL = ""
	}

	if registryURL == "" {
		c.registryMonitorMu.Lock()
		c.monitoringRegistry = false
		c.registryMonitorMu.Unlock()
		c.registeredMu.Lock()
		c.registered = false
		c.registeredMu.Unlock()
		c.setPeerToPeerMode(true)
		slog.Info("Node operating in peer-to-peer mode (no registry)")
		return
	}

	if err := c.registerWithRegistry(ctx, registryURL); err != nil {
		slog.Warn("Failed to register node with registry, operating in peer-to-peer mode", "error", err)
		c.registryMonitorMu.Lock()
		c.monitoringRegistry = false
		c.registryMonitorMu.Unlock()
		c.registeredMu.Lock()
		c.registered = false
		c.registeredMu.Unlock()
		c.setPeerToPeerMode(true)
		return
	}

	c.registeredMu.Lock()
	c.registered = true
	c.registeredMu.Unlock()
	c.setPeerToPeerMode(false)

	select {
	case c.registryResolved <- registryURL:
	default:
	}

	slog.Info("Node registered with registry", "registry", registryURL)

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.startHeartbeat(ctx)
	}()

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.monitorRegistry(ctx)
	}()
}

// registerWithRegistry attempts to register the node with the given registry URL
// and registers all resources (devices, sources, flows, senders, receivers)
func (c *nmosController) registerWithRegistry(ctx context.Context, registryURL string) error {
	node := c.buildNodeResource(c.nodeID)
	c.registryURL = registryURL
	c.currentRegistryURL = registryURL

	if err := c.RegisterNode(node); err != nil {
		return err
	}

	slog.Info("Node registered with registry", "registry", registryURL)

	// Re-register all resources with the new registry
	c.reRegisterAll(ctx)

	return nil
}

// monitorRegistry continuously monitors for registry changes via mDNS and re-registers if needed
func (c *nmosController) monitorRegistry(ctx context.Context) {
	registryCheckTicker := time.NewTicker(30 * time.Second)
	defer registryCheckTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case <-registryCheckTicker.C:
			c.registryMonitorMu.RLock()
			monitoring := c.monitoringRegistry
			c.registryMonitorMu.RUnlock()

			if !monitoring {
				return
			}

			if newURL, err := c.discoverRegistry(ctx); err == nil {
				c.registryMonitorMu.RLock()
				currentURL := c.currentRegistryURL
				c.registryMonitorMu.RUnlock()

				if newURL != currentURL {
					slog.Info("Registry changed, re-registering", "old", currentURL, "new", newURL)
					if err := c.registerWithRegistry(ctx, newURL); err != nil {
						slog.Error("Failed to re-register with new registry", "error", err)
					}
				}
			}
		}
	}
}

// startHeartbeat manages the periodic heartbeat to the registration API
func (c *nmosController) startHeartbeat(ctx context.Context) {
	ticker := time.NewTicker(c.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case <-ticker.C:
			c.performHeartbeat(ctx)
		}
	}
}

// performHeartbeat executes a single heartbeat and handles re-registration if needed
func (c *nmosController) performHeartbeat(ctx context.Context) {
	heartbeatURL := fmt.Sprintf("%s/x-nmos/registration/v1.3/health/nodes/%s", c.registryURL, c.nodeID)
	req, err := http.NewRequestWithContext(ctx, "POST", heartbeatURL, nil)
	if err != nil {
		slog.Error("Failed to create heartbeat request", "error", err)
		return
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Warn("Heartbeat failed, attempting registry re-discovery", "url", heartbeatURL, "error", err)
		c.rediscoverAndReregister(ctx)
		return
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		slog.Debug("NMOS Heartbeat successful")
	case http.StatusNotFound, http.StatusGone:
		slog.Warn("NMOS Registry returned error for heartbeat, re-registering with discovered registry")
		c.rediscoverAndReregister(ctx)
	default:
		slog.Warn("NMOS Registry returned unexpected status for heartbeat", "status", resp.StatusCode)
	}
}

// rediscoverAndReregister discovers a new registry via mDNS and re-registers
func (c *nmosController) rediscoverAndReregister(ctx context.Context) {
	c.registryMonitorMu.Lock()
	c.monitoringRegistry = false
	c.registryMonitorMu.Unlock()

	newURL, err := c.discoverRegistry(ctx)
	if err != nil {
		slog.Warn("Registry re-discovery failed", "error", err)
		return
	}

	c.registryMonitorMu.Lock()
	c.currentRegistryURL = newURL
	c.registryURL = newURL
	c.monitoringRegistry = true
	c.registryMonitorMu.Unlock()

	if err := c.registerWithRegistry(ctx, newURL); err != nil {
		slog.Error("Failed to re-register with new registry", "error", err)
	}
}

// reRegisterAll re-registers the node and all cached resources in order
func (c *nmosController) reRegisterAll(ctx context.Context) {
	// 1. Re-register Node
	node := c.buildNodeResource(c.nodeID)
	if err := c.RegisterNode(node); err != nil {
		slog.Error("Failed to re-register node", "error", err)
		return
	}

	// 2. Re-register all other resources in order
	// NMOS order: Devices -> Sources -> Flows -> Senders -> Receivers
	resourceOrder := []string{"devices", "sources", "flows", "senders", "receivers"}

	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, resourceType := range resourceOrder {
		resources := c.resources[resourceType]
		for _, res := range resources {
			if err := c.registerResourceToRegistry(ctx, resourceType, res); err != nil {
				slog.Error("Failed to re-register resource", "type", resourceType, "error", err)
			}
		}
	}
}

func (c *nmosController) toSingular(resourceType string) string {
	switch resourceType {
	case "nodes":
		return "node"
	case "devices":
		return "device"
	case "sources":
		return "source"
	case "flows":
		return "flow"
	case "senders":
		return "sender"
	case "receivers":
		return "receiver"
	default:
		return resourceType
	}
}

func (c *nmosController) toPlural(resourceType string) string {
	switch resourceType {
	case "node":
		return "nodes"
	case "device":
		return "devices"
	case "source":
		return "sources"
	case "flow":
		return "flows"
	case "sender":
		return "senders"
	case "receiver":
		return "receivers"
	default:
		return resourceType
	}
}

// registerResourceToRegistry POSTs a resource to the NMOS IS-04 registry
func (c *nmosController) registerResourceToRegistry(ctx context.Context, resourceType string, resource interface{}) error {
	// Wrap resource in IS-04 resource envelope
	// Per IS-04 spec, the "type" field in the POST body must be singular (e.g., "node", not "nodes")
	wrapper := map[string]interface{}{
		"type": c.toSingular(resourceType),
		"data": resource,
	}

	resourceJSON, err := json.Marshal(wrapper)
	if err != nil {
		return fmt.Errorf("failed to marshal resource: %w", err)
	}

	slog.Debug("Registering resource with registry",
		"type", resourceType,
		"url", fmt.Sprintf("%s/x-nmos/registration/v1.3/resource", c.registryURL),
		"requestBody", string(resourceJSON))

	// POST to registry
	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/x-nmos/registration/v1.3/resource", c.registryURL),
		bytes.NewReader(resourceJSON))
	if err != nil {
		return fmt.Errorf("failed to create registry request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to register resource with registry: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for logging
	respBody, _ := io.ReadAll(resp.Body)
	slog.Debug("Registry response",
		"type", resourceType,
		"statusCode", resp.StatusCode,
		"responseBody", string(respBody))

	// Check for error code in JSON response body (NMOS registries may return 200 with error in body)
	var respJSON map[string]interface{}
	if err := json.Unmarshal(respBody, &respJSON); err == nil {
		if code, ok := respJSON["code"].(float64); ok && int(code) >= 400 {
			slog.Error("Registry returned error",
				"type", resourceType,
				"httpStatus", resp.StatusCode,
				"errorCode", int(code),
				"errorMessage", respJSON["error"])
			return fmt.Errorf("registry error: %s (code %d)", respJSON["error"], int(code))
		}
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		slog.Info("Registered resource with registry", "type", resourceType)
		return nil
	}

	return fmt.Errorf("registry rejected resource registration: status %d", resp.StatusCode)
}

// unregisterResourceFromRegistry DELETEs a resource from the NMOS registry
func (c *nmosController) unregisterResourceFromRegistry(ctx context.Context, resourceType string, id string) error {
	// Per IS-04 spec, the resource type in the DELETE URL must be plural (e.g., "/nodes/", not "/node/")
	url := fmt.Sprintf("%s/x-nmos/registration/v1.3/resource/%s/%s", c.registryURL, c.toPlural(resourceType), id)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create unregistration request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute unregistration request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		slog.Info("Unregistered resource from registry", "type", resourceType, "id", id)
		return nil
	}

	if resp.StatusCode == http.StatusNotFound {
		slog.Debug("Resource already removed from registry", "type", resourceType, "id", id)
		return nil
	}

	return fmt.Errorf("registry rejected resource unregistration: status %d", resp.StatusCode)
}

// unregisterAll performs a controlled unregistration of all resources in order
func (c *nmosController) unregisterAll(ctx context.Context) {
	c.registeredMu.RLock()
	wasRegistered := c.registered
	c.registeredMu.RUnlock()

	if !wasRegistered {
		slog.Info("Skipping NMOS unregistration - node was never registered")
		return
	}

	slog.Info("Performing NMOS controlled unregistration")

	childTypes := []string{"receivers", "senders", "flows", "sources"}

	c.mu.RLock()
	nodeID := c.nodeID

	type resourceInfo struct {
		id       string
		deviceID string
		resType  string
	}

	deviceIDs := make(map[string]struct{})
	var allResources []resourceInfo

	for _, t := range childTypes {
		for _, res := range c.resources[t] {
			if resMap, ok := res.(map[string]interface{}); ok {
				id, idOK := resMap["id"].(string)
				deviceID, devOK := resMap["device_id"].(string)
				if idOK && devOK {
					deviceIDs[deviceID] = struct{}{}
					allResources = append(allResources, resourceInfo{id: id, deviceID: deviceID, resType: t})
				}
			}
		}
	}

	devices := make([]string, 0, len(deviceIDs))
	for d := range deviceIDs {
		devices = append(devices, d)
	}
	c.mu.RUnlock()

	sort.Strings(devices)
	const maxConcurrent = 4

	for _, deviceID := range devices {
		deviceResources := make([]resourceInfo, 0)
		for _, r := range allResources {
			if r.deviceID == deviceID {
				deviceResources = append(deviceResources, r)
			}
		}

		if len(deviceResources) == 0 {
			continue
		}

		sem := make(chan struct{}, maxConcurrent)
		var wg sync.WaitGroup
		errChan := make(chan error, len(deviceResources))

		for _, r := range deviceResources {
			wg.Add(1)
			go func(rid, rt string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				unregCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
				defer cancel()

				if err := c.unregisterResourceFromRegistry(unregCtx, rt, rid); err != nil {
					errChan <- fmt.Errorf("%s/%s: %w", rt, rid, err)
				}
			}(r.id, r.resType)
		}

		wg.Wait()
		close(errChan)

		for err := range errChan {
			slog.Warn("Failed to unregister resource", "error", err)
		}

		{
			unregCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()
			if err := c.unregisterResourceFromRegistry(unregCtx, "devices", deviceID); err != nil {
				slog.Warn("Failed to unregister device", "id", deviceID, "error", err)
			}
		}
	}

	if err := c.unregisterResourceFromRegistry(context.Background(), "node", nodeID); err != nil {
		slog.Warn("Failed to unregister node", "id", nodeID, "error", err)
	}
}

// discoverRegistry discovers the NMOS registry based on the configured discovery mode
func (c *nmosController) discoverRegistry(ctx context.Context) (string, error) {
	switch c.registryConfig.Mode {
	case DiscoveryModeStatic:
		return c.discoverStatic()
	case DiscoveryModeMDNS:
		return c.discoverDNS(ctx, "local.")
	case DiscoveryModeDNSSD:
		return c.discoverDNS(ctx, c.registryConfig.Domain)
	default:
		return c.discoverDNS(ctx, "local.")
	}
}

// discoverStatic returns the statically configured registry URL
func (c *nmosController) discoverStatic() (string, error) {
	if c.registryConfig.StaticRegistry == "" {
		return "", errors.New("static registry URL is not configured")
	}
	slog.Info("Using static registry URL", "url", c.registryConfig.StaticRegistry)
	return c.registryConfig.StaticRegistry, nil
}

// discoverDNS discovers the registry via DNS-SD on the specified domain
func (c *nmosController) discoverDNS(ctx context.Context, domain string) (string, error) {
	if domain == "local." {
		return c.discoverMDNS(ctx)
	}
	return c.discoverUnicastDNS(ctx, domain)
}

// discoverMDNS discovers the registry via mDNS (multicast DNS on local domain)
func (c *nmosController) discoverMDNS(ctx context.Context) (string, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return "", fmt.Errorf("failed to create mDNS resolver: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry)
	done := make(chan struct{})

	go func(ctx context.Context, results <-chan *zeroconf.ServiceEntry) {
		defer close(done)
		for entry := range results {
			apiVersion := "v1.3"
			for _, txt := range entry.Text {
				if strings.HasPrefix(txt, "api_ver=") {
					apiVersion = strings.TrimPrefix(txt, "api_ver=")
					break
				}
			}

			var host string
			if len(entry.AddrIPv4) > 0 {
				host = entry.AddrIPv4[0].String()
			} else if len(entry.AddrIPv6) > 0 {
				host = entry.AddrIPv6[0].String()
			} else {
				host = entry.HostName
			}

			registryURL := fmt.Sprintf("http://%s:%d/x-nmos/%s/", host, entry.Port, apiVersion)
			slog.Info("Discovered NMOS registry via mDNS", "url", registryURL)
			select {
			case entries <- entry:
			case <-ctx.Done():
				return
			}
		}
	}(ctx, entries)

	err = resolver.Browse(ctx, "_nmos-register._tcp", "local.", entries)
	if err != nil {
		return "", fmt.Errorf("failed to browse for registry via mDNS: %w", err)
	}

	discoveryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	select {
	case <-discoveryCtx.Done():
		return "", errors.New("mDNS registry discovery timed out")
	case <-done:
		return "", errors.New("no registries found via mDNS")
	case entry := <-entries:
		var host string
		if len(entry.AddrIPv4) > 0 {
			host = entry.AddrIPv4[0].String()
		} else if len(entry.AddrIPv6) > 0 {
			host = entry.AddrIPv6[0].String()
		} else {
			host = entry.HostName
		}
		return fmt.Sprintf("http://%s:%d", host, entry.Port), nil
	}
}

// discoverUnicastDNS discovers the registry via DNS-SD using standard unicast DNS
func (c *nmosController) discoverUnicastDNS(ctx context.Context, domain string) (string, error) {
	dnsName := fmt.Sprintf("_nmos-register._tcp.%s", strings.TrimSuffix(domain, "."))

	_, srvResults, err := net.LookupSRV("", "", dnsName)
	if err != nil {
		return "", fmt.Errorf("failed to lookup SRV record for %s: %w", dnsName, err)
	}

	if len(srvResults) == 0 {
		return "", fmt.Errorf("no SRV records found for %s", dnsName)
	}

	srvRecord := srvResults[0]
	target := strings.TrimSuffix(srvRecord.Target, ".")
	port := srvRecord.Port

	txtRecords, _ := net.LookupTXT(dnsName)
	apiVersion := "v1.3"
	for _, txt := range txtRecords {
		if strings.HasPrefix(txt, "api_ver=") {
			apiVersion = strings.TrimPrefix(txt, "api_ver=")
			break
		}
	}

	registryURL := fmt.Sprintf("http://%s:%d/x-nmos/%s/", target, port, apiVersion)
	slog.Info("Discovered NMOS registry via DNS-SD", "url", registryURL, "domain", domain)

	return fmt.Sprintf("http://%s:%d", target, port), nil
}

// buildNodeResource creates the node resource for IS-04 registration
func (c *nmosController) buildNodeResource(nodeID string) map[string]interface{} {
	listenAddr := c.GetListenAddr()
	host, portStr := splitHostPort(listenAddr)
	port := 8080
	if p, err := strconv.Atoi(portStr); err == nil {
		port = p
	}

	hostname := host
	if c.advertisedAddr != "" {
		hostname = c.advertisedAddr
	}

	return map[string]interface{}{
		"id":          nodeID,
		"version":     fmt.Sprintf("%d:%d", time.Now().Unix(), time.Now().Nanosecond()),
		"label":       "Shure-NMOS Gateway Node",
		"description": "Gateway connecting Shure Axient to NMOS",
		"tags":        map[string]interface{}{},
		"caps":        map[string]interface{}{},
		"api": map[string]interface{}{
			"versions": []string{"v1.3"},
			"endpoints": []map[string]interface{}{
				{"host": host, "port": port, "protocol": "http"},
			},
		},
"hostname": hostname,
		"interfaces": c.nodeInterfaces,
		"services":  []map[string]interface{}{},
		"clocks": []map[string]interface{}{
			{
				"name":     "clk0",
				"ref_type": "internal",
			},
		},
	}
}

// splitHostPort separates host and port from an address string
func splitHostPort(addr string) (host, port string) {
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx], addr[idx+1:]
	}
	return addr, ""
}

// discoverNodeInterfaces discovers and caches network interface information for IS-04 node self resource
func (c *nmosController) discoverNodeInterfaces() {
	ifaces, err := net.Interfaces()
	if err != nil {
		slog.Warn("Failed to discover network interfaces", "error", err)
		c.nodeInterfaces = []map[string]interface{}{
			{"name": "eth0", "port": 8080, "mac": "00:00:00:00:00:00", "enabled": true, "mtu": 1500, "type": "ethernet", "primary": true},
		}
		return
	}

	var primaryInterface *net.Interface

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil || len(addrs) == 0 {
			continue
		}
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.To4() != nil && !ipNet.IP.IsLoopback() {
				primaryInterface = &iface
				break
			}
		}
		if primaryInterface != nil {
			break
		}
	}

	if primaryInterface == nil {
		slog.Warn("No suitable network interface found, using default")
		c.nodeInterfaces = []map[string]interface{}{
			{
				"name":       "eth0",
				"chassis_id": nil,
				"port_id":    "00:00:00:00:00:00",
			},
		}
		return
	}

	macStr := primaryInterface.HardwareAddr.String()
	c.nodeInterfaces = []map[string]interface{}{
		{
			"name":       primaryInterface.Name,
			"chassis_id": nil,
			"port_id":    macStr,
		},
	}

	slog.Info("Discovered network interface", "name", primaryInterface.Name, "mac", macStr)
}

// startServer initializes and starts the NMOS Node API HTTP server
func (c *nmosController) startServer() error {
	r := c.setupRouter()

	listener, err := net.Listen("tcp", c.nodeAddr)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	c.listenAddr = listener.Addr().String()

	c.httpServer = &http.Server{
		Handler: r,
	}

	go func() {
		slog.Info("Starting NMOS Node API server", "address", c.nodeAddr)
		if err := c.httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("NMOS Node API server error", "error", err)
		}
	}()

	return nil
}

func (c *nmosController) setupRouter() *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.CleanPath)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)

	r.Get("/x-nmos", c.handleNodeRoot)
	r.Get("/x-nmos/", c.handleNodeRoot)
	r.Get("/x-nmos/node", c.handleNodeRoot)
	r.Get("/x-nmos/node/", c.handleNodeRoot)
	r.Get("/x-nmos/node/v1.3", c.handleNodeRoot)
	r.Get("/x-nmos/node/v1.3/", c.handleNodeRoot)
	r.Get("/x-nmos/node/v1.3/self", c.handleNodeSelf)
	r.Get("/x-nmos/node/v1.3/self/", c.handleNodeSelf)
	r.Get("/x-nmos/node/v1.3/devices", c.handleNodeDevices)
	r.Get("/x-nmos/node/v1.3/devices/", c.handleNodeDevices)
	r.Get("/x-nmos/node/v1.3/devices/{id}", c.handleNodeDeviceById)
	r.Get("/x-nmos/node/v1.3/devices/{id}/", c.handleNodeDeviceById)
	r.Route("/x-nmos/node/v1.3/devices/{id}/controls", func(r chi.Router) {
		r.Get("/", c.handleDeviceControls)
		r.Post("/", c.handleDeviceControls)
		r.Put("/", c.handleDeviceControls)
		r.Patch("/", c.handleDeviceControls)
	})
	r.Get("/x-nmos/node/v1.3/sources", c.handleNodeSources)
	r.Get("/x-nmos/node/v1.3/sources/", c.handleNodeSources)
	r.Get("/x-nmos/node/v1.3/sources/{id}", c.handleNodeSourceById)
	r.Get("/x-nmos/node/v1.3/sources/{id}/", c.handleNodeSourceById)
	r.Get("/x-nmos/node/v1.3/flows", c.handleNodeFlows)
	r.Get("/x-nmos/node/v1.3/flows/", c.handleNodeFlows)
	r.Get("/x-nmos/node/v1.3/flows/{id}", c.handleNodeFlowById)
	r.Get("/x-nmos/node/v1.3/flows/{id}/", c.handleNodeFlowById)
	r.Get("/x-nmos/node/v1.3/senders", c.handleNodeSenders)
	r.Get("/x-nmos/node/v1.3/senders/", c.handleNodeSenders)
	r.Get("/x-nmos/node/v1.3/senders/{id}", c.handleNodeSenderById)
	r.Get("/x-nmos/node/v1.3/senders/{id}/", c.handleNodeSenderById)
	r.Get("/x-nmos/node/v1.3/receivers", c.handleNodeReceivers)
	r.Get("/x-nmos/node/v1.3/receivers/", c.handleNodeReceivers)
	r.Get("/x-nmos/node/v1.3/receivers/{id}", c.handleNodeReceiverById)
	r.Get("/x-nmos/node/v1.3/receivers/{id}/", c.handleNodeReceiverById)

	r.Get("/x-nmos/node/v1.3/ncp", c.handleNCP)
	r.Get("/x-nmos/node/v1.3/ncp/", c.handleNCP)

	r.Route("/x-nmos/events/v1.0", func(r chi.Router) {
		r.Get("/", c.handleEventsRoot)
		r.Get("/ws", c.handleWebsocket)
		r.Get("/events", c.handleWebsocket)
		r.Get("/devices/{deviceId}", c.handleWebsocket)
		r.Route("/sources", func(r chi.Router) {
			r.Get("/", c.handleEventsSourcesList)
			r.Route("/{sourceId}", func(r chi.Router) {
				r.Get("/state", c.handleEventsSourceState)
				r.Get("/type", c.handleEventsSourceType)
			})
		})
	})

	r.Route("/x-nmos/connection/v1.1", func(r chi.Router) {
		r.Get("/", c.handleConnectionRoot)
		r.Route("/single", func(r chi.Router) {
			r.Get("/", c.handleConnectionSingleRoot)
			r.Route("/senders", func(r chi.Router) {
				r.Get("/", c.handleConnectionSendersList)
				r.Route("/{senderId}", func(r chi.Router) {
					r.Get("/active", c.handleConnectionSenderActive)
					r.Get("/staged", c.handleConnectionSenderStaged)
					r.Patch("/staged", c.handleConnectionSenderStaged)
					r.Get("/constraints", c.handleConnectionSenderConstraints)
				})
			})
		})
	})

	r.NotFound(c.handleNotFound)

	return r
}

// handleNotFound returns a JSON error response for unhandled routes
func (c *nmosController) handleNotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(map[string]string{"error": "Not Found"})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// startMDNS advertises NMOS services via mDNS
func (c *nmosController) startMDNS() error {
	host, portStr := splitHostPort(c.nodeAddr)
	port := 8080
	if p, err := strconv.Atoi(portStr); err == nil {
		port = p
	}

	// Advertise NMOS Node API
	nodeServer, err := zeroconf.Register(
		"nmos-node-"+c.nodeID,
		"_nmos-node._tcp",
		"local.",
		port,
		[]string{"api_ver=v1.3", "api_proto=http"},
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to register NMOS Node mDNS: %w", err)
	}
	c.nodeServer = nodeServer

	// Advertise NMOS NCP (IS-12)
	ncpServer, err := zeroconf.Register(
		"nmos-ncp-"+c.nodeID,
		"_nmos-ncp._tcp",
		"local.",
		port,
		[]string{"api_ver=v1.0", "api_proto=ws"},
		nil,
	)
	if err != nil {
		slog.Warn("Failed to register NMOS NCP mDNS", "error", err)
	} else {
		c.ncpServer = ncpServer
	}

	// Advertise NMOS Events API
	eventsServer, err := zeroconf.Register(
		"nmos-events-"+c.nodeID,
		"_nmos-events._tcp",
		"local.",
		port,
		[]string{"api_ver=v1.0", "api_proto=http"},
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to register NMOS Events mDNS: %w", err)
	}
	c.eventsServer = eventsServer

	// Advertise NMOS Connection API
	_, err = zeroconf.Register(
		"nmos-connection-"+c.nodeID,
		"_nmos-connection._tcp",
		"local.",
		port,
		[]string{"api_ver=v1.1", "api_proto=http"},
		nil,
	)
	if err != nil {
		slog.Warn("Failed to register NMOS Connection mDNS", "error", err)
	}

	slog.Info("NMOS services advertised via mDNS", "host", host, "port", port)
	return nil
}

// incrementAndGetVersion increments the peer-to-peer version counter for a resource type
// and returns the new value (wraps at 255)
func (c *nmosController) incrementAndGetVersion(resourceType string) uint8 {
	c.peerToPeerMu.Lock()
	defer c.peerToPeerMu.Unlock()
	count := c.versionCounters[resourceType]
	if count == 255 {
		count = 0
	} else {
		count++
	}
	c.versionCounters[resourceType] = count
	slog.Info("Peer-to-peer version updated", "resource", resourceType, "version", count)
	return count
}

// updatePeerToPeerMDNS logs the current peer-to-peer version counters
// Note: The zeroconf library v1.0.0 doesn't support dynamic TXT record updates,
// so we just log the current state for debugging purposes
func (c *nmosController) updatePeerToPeerMDNS() {
	if !c.peerToPeerMode {
		return
	}

	c.peerToPeerMu.RLock()
	verSlf := c.versionCounters["self"]
	verSrc := c.versionCounters["sources"]
	verFlw := c.versionCounters["flows"]
	verDvc := c.versionCounters["devices"]
	verSnd := c.versionCounters["senders"]
	verRcv := c.versionCounters["receivers"]
	c.peerToPeerMu.RUnlock()

	slog.Info("Peer-to-peer version state",
		"ver_slf", verSlf,
		"ver_src", verSrc,
		"ver_flw", verFlw,
		"ver_dvc", verDvc,
		"ver_snd", verSnd,
		"ver_rcv", verRcv)
}

// setPeerToPeerMode enables or disables peer-to-peer mode
func (c *nmosController) setPeerToPeerMode(enabled bool) {
	c.peerToPeerMode = enabled
	if enabled {
		slog.Info("Switched to peer-to-peer mode, version TXT records will be tracked")
		c.updatePeerToPeerMDNS()
	} else {
		slog.Info("Exiting peer-to-peer mode, registry mode active")
	}
}

// BroadcastNCPNotification sends a notification to subscribed NCP clients only
func (c *nmosController) BroadcastNCPNotification(oid int, eventID NCPEventID, eventData PropertyChangedEventData) {
	c.ncpClientsMu.Lock()
	defer c.ncpClientsMu.Unlock()

	msg := NCPMessage{
		MessageType: NCPMessageTypeNotification,
		Notifications: []NCPNotification{
			{
				OID:       oid,
				EventID:   eventID,
				EventData: eventData,
			},
		},
	}

	for client := range c.ncpClients {
		// Check if this client is subscribed to this OID
		c.ncpSubMu.RLock()
		_, subscribed := c.ncpSubscriptions[client][oid]
		c.ncpSubMu.RUnlock()

		if !subscribed {
			continue
		}

		if err := client.WriteJSON(msg); err != nil {
			slog.Warn("Failed to send NCP notification to client", "error", err)
		}
	}
}

// RegisterNCPObject registers a control object with a specific OID
func (c *nmosController) RegisterNCPObject(oid int, obj NcObject) {
	c.ncpMu.Lock()
	defer c.ncpMu.Unlock()

	// Set notification callback
	obj.SetNotifyCallback(c.BroadcastNCPNotification)

	if block, ok := obj.(*NcBlock); ok {
		block.SetResolver(func(oid int) NcObject {
			// This is safe because ncpMu is not held during GetProperty in dispatchNCPCommand
			return c.GetNCPObject(oid)
		})
	}
	c.ncpObjects[oid] = obj
}

func (c *nmosController) RegisterClass(class NcClassDescriptor) {
	c.ncpMu.RLock()
	cm := c.ncpObjects[3]
	c.ncpMu.RUnlock()

	if manager, ok := cm.(*NcClassManager); ok {
		key := classIDToKey(class.ClassID)
		manager.ClassManager.Classes[key] = NcClassDescriptorToClassDescriptor(class)
	}
}

func (c *nmosController) GetNCPObject(oid int) NcObject {
	c.ncpMu.RLock()
	defer c.ncpMu.RUnlock()
	return c.ncpObjects[oid]
}

// isWebSocketUpgrade checks if the HTTP request is a proper WebSocket upgrade
func isWebSocketUpgrade(r *http.Request) bool {
	if r.Header.Get("Upgrade") == "" {
		return false
	}
	return strings.Contains(strings.ToLower(r.Header.Get("Upgrade")), "websocket")
}

// handleNCP handles the /x-nmos/node/v1.3/ncp WebSocket endpoint
func (c *nmosController) handleNCP(w http.ResponseWriter, r *http.Request) {
	// Verify this is a proper WebSocket upgrade request
	if !isWebSocketUpgrade(r) {
		http.Error(w, "NCP endpoint requires WebSocket upgrade", http.StatusBadRequest)
		return
	}

	conn, err := c.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("Failed to upgrade NCP connection", "error", err)
		return
	}

	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	client := &ncpClient{conn: conn}

	c.ncpClientsMu.Lock()
	c.ncpClients[client] = true
	c.ncpClientsMu.Unlock()

	defer func() {
		c.ncpClientsMu.Lock()
		delete(c.ncpClients, client)
		c.ncpClientsMu.Unlock()
		c.ncpSubMu.Lock()
		delete(c.ncpSubscriptions, client)
		c.ncpSubMu.Unlock()
		conn.Close()
	}()

	slog.Info("New NCP client connected", "remote", r.RemoteAddr)

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Error("NCP read error", "error", err)
			}
			break
		}

		conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		var ncpMsg NCPMessage
		if err := json.Unmarshal(message, &ncpMsg); err != nil {
			slog.Warn("Failed to unmarshal NCP message", "error", err)
			continue
		}

		switch ncpMsg.MessageType {
		case NCPMessageTypeCommand:
			responses := make([]NCPCommandResponse, 0, len(ncpMsg.Commands))
			for _, cmd := range ncpMsg.Commands {
				resp := c.dispatchNCPCommand(cmd)
				responses = append(responses, resp)
			}

			respMsg := NCPMessage{
				MessageType:      NCPMessageTypeResponse,
				CommandResponses: responses,
			}

			conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
			if err := client.WriteJSON(respMsg); err != nil {
				slog.Error("Failed to send NCP response", "error", err)
				break
			}

		case NCPMessageTypeSubscription:
			subscribed := c.handleSubscriptions(client, ncpMsg.Subscriptions)
			respMsg := NCPMessage{
				MessageType:   NCPMessageTypeSubscriptionResponse,
				Subscriptions: subscribed,
			}
			client.SetWriteDeadline(time.Now().Add(30 * time.Second))
			if err := client.WriteJSON(respMsg); err != nil {
				slog.Error("Failed to send NCP subscription response", "error", err)
				break
			}
		}
	}
}

func (c *nmosController) dispatchNCPCommand(cmd NCPCommand) NCPCommandResponse {
	// Attempt to unmarshal arguments for property methods if needed
	c.ncpMu.RLock()
	obj, ok := c.ncpObjects[cmd.OID]
	c.ncpMu.RUnlock()

	if !ok {
		return NCPCommandResponse{
			Handle: cmd.Handle,
			Result: NCPMethodResult{Status: 404},
		}
	}

	// Handle Get (1m1) and Set (1m2) as special cases for properties
	if cmd.MethodID == NCMethodGet {
		var args struct {
			ID NCPPropertyID `json:"id"`
		}
		if err := json.Unmarshal(cmd.Arguments, &args); err != nil {
			return NCPCommandResponse{
				Handle: cmd.Handle,
				Result: NCPMethodResult{Status: 400},
			}
		}
		val, err := obj.GetProperty(args.ID)
		if err != nil {
			return NCPCommandResponse{
				Handle: cmd.Handle,
				Result: NCPMethodResult{Status: 404},
			}
		}
		return NCPCommandResponse{
			Handle: cmd.Handle,
			Result: NCPMethodResult{Status: 200, Value: val},
		}
	}

	if cmd.MethodID == NCMethodSet {
		var args struct {
			ID    NCPPropertyID `json:"id"`
			Value interface{}   `json:"value"`
		}
		if err := json.Unmarshal(cmd.Arguments, &args); err != nil {
			return NCPCommandResponse{
				Handle: cmd.Handle,
				Result: NCPMethodResult{Status: 400},
			}
		}
		if err := obj.SetProperty(args.ID, args.Value); err != nil {
			return NCPCommandResponse{
				Handle: cmd.Handle,
				Result: NCPMethodResult{Status: 500},
			}
		}
		return NCPCommandResponse{
			Handle: cmd.Handle,
			Result: NCPMethodResult{Status: 200},
		}
	}

	// General method invocation
	val, err := obj.InvokeMethod(cmd.MethodID, cmd.Arguments)
	if err != nil {
		return NCPCommandResponse{
			Handle: cmd.Handle,
			Result: NCPMethodResult{Status: 500},
		}
	}

	return NCPCommandResponse{
		Handle: cmd.Handle,
		Result: NCPMethodResult{Status: 200, Value: val},
	}
}

// handleSubscriptions processes subscription requests from NCP clients
// Returns list of subscription IDs for successfully subscribed OIDs
func (c *nmosController) handleSubscriptions(conn *ncpClient, subscriptionOIDs []int) []int {
	var subscribed []int

	c.ncpSubMu.Lock()
	defer c.ncpSubMu.Unlock()

	for _, oid := range subscriptionOIDs {
		c.ncpMu.RLock()
		_, ok := c.ncpObjects[oid]
		c.ncpMu.RUnlock()

		if !ok {
			continue
		}

		// Create client subscription map if not exists
		if c.ncpSubscriptions[conn] == nil {
			c.ncpSubscriptions[conn] = make(map[int]int)
		}

		// Assign subscription ID
		subID := c.nextSubscriptionID
		c.nextSubscriptionID++
		c.ncpSubscriptions[conn][oid] = subID
		subscribed = append(subscribed, subID)
	}

	return subscribed
}

// Stop halts the NMOS controller
func (c *nmosController) Stop(ctx context.Context) error {
	if !c.isRunning {
		return nil
	}

	c.isRunning = false

	// Signal all goroutines to stop FIRST
	close(c.done)

	// Perform controlled unregistration using a fresh context (not the canceled shutdown context)
	unregCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	c.unregisterAll(unregCtx)
	cancel()

	// Stop mDNS advertisement
	if c.nodeServer != nil {
		c.nodeServer.Shutdown()
	}
	if c.eventsServer != nil {
		c.eventsServer.Shutdown()
	}

	// Close all WebSocket connections gracefully to unblock handlers
	c.closeAllWebsocketConnections()

	// Wait for goroutines to finish (with timeout)
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		slog.Debug("All goroutines finished")
	case <-time.After(5 * time.Second):
		slog.Warn("Goroutines did not finish in time, continuing anyway")
	}

	// Force close HTTP server after handlers have exited
	if c.httpServer != nil {
		slog.Info("Closing NMOS Node API server")
		c.httpServer.Close()
	}

	// Close events channel
	close(c.eventsChan)

	return nil
}

// handleNodeRoot handles the root of the Node API
func (c *nmosController) handleNodeRoot(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	if path == "/x-nmos" || path == "/x-nmos/" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]string{"node/"})
		return
	}

	if path == "/x-nmos/node" || path == "/x-nmos/node/" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]string{"v1.3/"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]string{
		"self/",
		"sources/",
		"flows/",
		"devices/",
		"senders/",
		"receivers/",
	})
}

func (c *nmosController) handleEventsRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]string{"sources/"})
}

func (c *nmosController) handleEventsSourcesList(w http.ResponseWriter, r *http.Request) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var sourceIDs []string
	for _, sources := range c.resources["sources"] {
		if src, ok := sources.(map[string]interface{}); ok {
			if id, ok := src["id"].(string); ok {
				sourceIDs = append(sourceIDs, id+"/")
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sourceIDs)
}

func (c *nmosController) handleEventsSourceState(w http.ResponseWriter, r *http.Request) {
	sourceID := chi.URLParam(r, "sourceId")
	if sourceID == "" {
		http.NotFound(w, r)
		return
	}

	c.mu.RLock()
	event, exists := c.lastEvents[sourceID]
	c.mu.RUnlock()

	if !exists {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(event)
}

func (c *nmosController) handleEventsSourceType(w http.ResponseWriter, r *http.Request) {
	sourceID := chi.URLParam(r, "sourceId")
	if sourceID == "" {
		http.NotFound(w, r)
		return
	}

	c.mu.RLock()
	var eventType string
	for _, sources := range c.resources["sources"] {
		if src, ok := sources.(map[string]interface{}); ok {
			if id, ok := src["id"].(string); ok && id == sourceID {
				if et, ok := src["event_type"].(string); ok {
					eventType = et
					break
				}
			}
		}
	}
	c.mu.RUnlock()

	if eventType == "" {
		http.NotFound(w, r)
		return
	}

	typeDef := map[string]interface{}{
		"type": eventType,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(typeDef)
}

// handleWebsocket handles IS-07 websocket connections
func (c *nmosController) handleWebsocket(w http.ResponseWriter, r *http.Request) {
	conn, err := c.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("Failed to upgrade websocket", "error", err)
		return
	}

	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	c.mu.Lock()
	c.clients[conn] = make(map[string]bool)
	c.mu.Unlock()

	slog.Info("New NMOS IS-07 websocket client connected")

	c.wg.Add(1)
	go func() {
		defer func() {
			c.mu.Lock()
			delete(c.clients, conn)
			delete(c.clientLastHealth, conn)
			c.mu.Unlock()
			conn.Close()
			slog.Info("NMOS IS-07 websocket client disconnected")
			c.wg.Done()
		}()

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					slog.Debug("WebSocket read error", "error", err)
				}
				break
			}

			conn.SetReadDeadline(time.Now().Add(60 * time.Second))

			// Handle IS-07 Commands
			var msg map[string]interface{}
			if err := json.Unmarshal(message, &msg); err == nil {
				if cmd, ok := msg["command"].(string); ok {
					switch cmd {
					case "health":
						originTimestamp := ""
						if ts, ok := msg["timestamp"].(string); ok {
							originTimestamp = ts
						}

						now := time.Now()
						creationTimestamp := fmt.Sprintf("%d:%d", now.Unix(), now.Nanosecond())

						// Update last health time for this client (IS-07: track for 12s timeout)
						c.mu.Lock()
						c.clientLastHealth[conn] = now
						c.mu.Unlock()

						response := map[string]interface{}{
							"message_type": "health",
							"timing": map[string]string{
								"origin_timestamp":   originTimestamp,
								"creation_timestamp": creationTimestamp,
							},
						}

						if respJSON, err := json.Marshal(response); err == nil {
							c.mu.RLock()
							if _, exists := c.clients[conn]; exists {
								conn.WriteMessage(websocket.TextMessage, respJSON)
							}
							c.mu.RUnlock()
						}

					case "subscription":
						if sources, ok := msg["sources"].([]interface{}); ok {
							subs := make(map[string]bool)
							var eventsToSend []map[string]interface{}

							c.mu.Lock()
							for _, s := range sources {
								if sID, ok := s.(string); ok {
									subs[sID] = true
									if lastEvent, exists := c.lastEvents[sID]; exists {
										eventsToSend = append(eventsToSend, lastEvent)
									}
								}
							}

							if _, exists := c.clients[conn]; exists {
								c.clients[conn] = subs
								slog.Info("Client updated subscriptions", "count", len(subs))
							}
							c.mu.Unlock()

							// Send initial states
							for _, evt := range eventsToSend {
								if jsonBytes, err := json.Marshal(evt); err == nil {
									c.mu.RLock()
									if _, exists := c.clients[conn]; exists {
										conn.WriteMessage(websocket.TextMessage, jsonBytes)
									}
									c.mu.RUnlock()
								}
							}
						}
					}
				}
			}
		}
	}()
}

func (c *nmosController) broadcastUpdate(resourceType string, resource interface{}) {
	c.mu.RLock()
	// Create a copy of clients to avoid holding the lock while writing
	clients := make([]*websocket.Conn, 0, len(c.clients))
	for client := range c.clients {
		clients = append(clients, client)
	}
	c.mu.RUnlock()

	if len(clients) == 0 {
		return
	}

	update := map[string]interface{}{
		"type":          "resource_update",
		"resource_type": resourceType,
		"data":          resource,
		"timestamp":     time.Now().Format(time.RFC3339),
	}

	updateJSON, _ := json.Marshal(update)

	var failed []*websocket.Conn
	for _, client := range clients {
		client.SetWriteDeadline(time.Now().Add(30 * time.Second))
		if err := client.WriteMessage(websocket.TextMessage, updateJSON); err != nil {
			slog.Error("Failed to send websocket update", "error", err)
			failed = append(failed, client)
		}
	}

	if len(failed) > 0 {
		c.mu.Lock()
		for _, client := range failed {
			delete(c.clients, client)
			delete(c.clientLastHealth, client)
		}
		c.mu.Unlock()
		for _, client := range failed {
			client.Close()
		}
	}
}

const is07HeartbeatTimeout = 12 * time.Second

func (c *nmosController) checkIS07Heartbeats(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case <-ticker.C:
			now := time.Now()
			c.mu.Lock()
			var expired []*websocket.Conn
			for conn, lastHealth := range c.clientLastHealth {
				if now.Sub(lastHealth) > is07HeartbeatTimeout {
					expired = append(expired, conn)
				}
			}
			for _, conn := range expired {
				delete(c.clients, conn)
				delete(c.clientLastHealth, conn)
			}
			c.mu.Unlock()
			for _, conn := range expired {
				slog.Warn("IS-07 client heartbeat timeout, closing connection", "timeout", is07HeartbeatTimeout)
				conn.Close()
			}
		}
	}
}

func (c *nmosController) pingIS07Clients(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case <-ticker.C:
			c.mu.RLock()
			clients := make([]*websocket.Conn, 0, len(c.clients))
			for conn := range c.clients {
				clients = append(clients, conn)
			}
			c.mu.RUnlock()

			for _, conn := range clients {
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					slog.Debug("Failed to send ping to IS-07 client", "error", err)
				}
			}
		}
	}
}

func (c *nmosController) closeAllWebsocketConnections() {
	c.mu.Lock()
	for conn := range c.clients {
		conn.Close()
	}
	c.mu.Unlock()

	c.ncpClientsMu.Lock()
	for client := range c.ncpClients {
		client.Close()
	}
	c.ncpClientsMu.Unlock()
}

// BroadcastEvent sends an IS-07 event to all connected websocket clients using the NMOS state message format
func (c *nmosController) BroadcastEvent(sourceID string, flowID string, eventType string, data interface{}) {
	c.mu.RLock()
	// Filter clients that have subscribed to this source
	clients := make([]*websocket.Conn, 0, len(c.clients))
	for client, subs := range c.clients {
		if subs == nil || len(subs) == 0 {
			// If no subscriptions are set, we assume broadcast/all (or strictly conform to IS-07 which requires subscription)
			// However, for debugging/legacy clients, we might want to default to allowing unless strict.
			// IS-07 says: "After establishing the subscriptions list, the client will start receiving events only for the sources it has subscribed to."
			// This implies if no subscription list is established, no events are received.
			// But for initial compatibility, let's keep it strict: only send if subscribed.
			continue
		}
		if subs[sourceID] {
			clients = append(clients, client)
		}
	}
	c.mu.RUnlock()

	if len(clients) == 0 {
		return
	}

	now := time.Now()
	timestamp := fmt.Sprintf("%d:%d", now.Unix(), now.Nanosecond())

	event := map[string]interface{}{
		"message_type": "state",
		"identity": map[string]string{
			"source_id": sourceID,
			"flow_id":   flowID,
		},
		"event_type": eventType,
		"timing": map[string]string{
			"creation_timestamp": timestamp,
		},
		"payload": map[string]interface{}{
			"value": data,
		},
	}

	// Update cache
	c.mu.Lock()
	c.lastEvents[sourceID] = event
	c.mu.Unlock()

	eventJSON, _ := json.Marshal(event)

	for _, client := range clients {
		if err := client.WriteMessage(websocket.TextMessage, eventJSON); err != nil {
			slog.Error("Failed to send IS-07 event", "error", err)
			c.mu.Lock()
			delete(c.clients, client)
			c.mu.Unlock()
			client.Close()
		}
	}
}

// handleNodeSelf handles the /self endpoint
func (c *nmosController) handleNodeSelf(w http.ResponseWriter, r *http.Request) {
	host, portStr := splitHostPort(c.nodeAddr)
	port := 8080
	if p, err := strconv.Atoi(portStr); err == nil {
		port = p
	}

	hostname := host
	if c.advertisedAddr != "" {
		hostname = c.advertisedAddr
	}

	self := map[string]interface{}{
		"id":          c.nodeID,
		"version":     fmt.Sprintf("%d:%d", time.Now().Unix(), time.Now().Nanosecond()),
		"label":       "Shure-NMOS Gateway Node",
		"description": "Gateway connecting Shure Axient to NMOS",
		"tags":        map[string]interface{}{},
		"caps":        map[string]interface{}{},
		"href":        fmt.Sprintf("http://%s:%d/", host, port),
		"api": map[string]interface{}{
			"versions": []string{"v1.3"},
			"endpoints": []map[string]interface{}{
				{"host": host, "port": port, "protocol": "http"},
			},
		},
		"hostname":   hostname,
		"interfaces": c.nodeInterfaces,
		"services":   []map[string]interface{}{},
		"clocks": []map[string]interface{}{
			{
				"name":     "clk0",
				"ref_type": "internal",
			},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(self)
}

// handleNodeDevices handles the /devices endpoint
func (c *nmosController) handleNodeDevices(w http.ResponseWriter, r *http.Request) {
	c.mu.RLock()
	db := c.db
	c.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")

	if db != nil {
		devices, err := db.GetDevices()
		if err != nil {
			slog.Error("Failed to get devices from database", "error", err)
			json.NewEncoder(w).Encode([]interface{}{})
			return
		}
		// Convert to generic interface slice for compatibility
		result := make([]interface{}, len(devices))
		for i, d := range devices {
			result[i] = d
		}
		json.NewEncoder(w).Encode(result)
		return
	}

	// Fallback to in-memory if db not set
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.resources == nil || c.resources["devices"] == nil {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	json.NewEncoder(w).Encode(c.resources["devices"])
}

// handleNodeSources handles the /sources endpoint
func (c *nmosController) handleNodeSources(w http.ResponseWriter, r *http.Request) {
	c.mu.RLock()
	db := c.db
	c.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")

	if db != nil {
		sources, err := db.GetSources()
		if err != nil {
			slog.Error("Failed to get sources from database", "error", err)
			json.NewEncoder(w).Encode([]interface{}{})
			return
		}
		result := make([]interface{}, len(sources))
		for i, s := range sources {
			result[i] = s
		}
		json.NewEncoder(w).Encode(result)
		return
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.resources == nil || c.resources["sources"] == nil {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	json.NewEncoder(w).Encode(c.resources["sources"])
}

// handleNodeFlows handles the /flows endpoint
func (c *nmosController) handleNodeFlows(w http.ResponseWriter, r *http.Request) {
	c.mu.RLock()
	db := c.db
	c.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")

	if db != nil {
		flows, err := db.GetFlows()
		if err != nil {
			slog.Error("Failed to get flows from database", "error", err)
			json.NewEncoder(w).Encode([]interface{}{})
			return
		}
		result := make([]interface{}, len(flows))
		for i, f := range flows {
			result[i] = f
		}
		json.NewEncoder(w).Encode(result)
		return
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.resources == nil || c.resources["flows"] == nil {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	json.NewEncoder(w).Encode(c.resources["flows"])
}

// handleNodeSenders handles the /senders endpoint
func (c *nmosController) handleNodeSenders(w http.ResponseWriter, r *http.Request) {
	c.mu.RLock()
	db := c.db
	c.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")

	if db != nil {
		senders, err := db.GetSenders()
		if err != nil {
			slog.Error("Failed to get senders from database", "error", err)
			json.NewEncoder(w).Encode([]interface{}{})
			return
		}
		result := make([]interface{}, len(senders))
		for i, s := range senders {
			result[i] = s
		}
		json.NewEncoder(w).Encode(result)
		return
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.resources == nil || c.resources["senders"] == nil {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	json.NewEncoder(w).Encode(c.resources["senders"])
}

// handleNodeReceivers handles the /receivers endpoint
func (c *nmosController) handleNodeReceivers(w http.ResponseWriter, r *http.Request) {
	c.mu.RLock()
	db := c.db
	c.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")

	if db != nil {
		receivers, err := db.GetReceivers()
		if err != nil {
			slog.Error("Failed to get receivers from database", "error", err)
			json.NewEncoder(w).Encode([]interface{}{})
			return
		}
		result := make([]interface{}, len(receivers))
		for i, r := range receivers {
			result[i] = r
		}
		json.NewEncoder(w).Encode(result)
		return
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.resources == nil || c.resources["receivers"] == nil {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	json.NewEncoder(w).Encode(c.resources["receivers"])
}

// handleNodeDeviceById handles GET /devices/{id}
func (c *nmosController) handleNodeDeviceById(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	c.mu.RLock()
	db := c.db
	c.mu.RUnlock()

	if db != nil {
		device, err := db.GetDevice(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if device == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(device)
		return
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.resources == nil || c.resources["devices"] == nil {
		http.NotFound(w, r)
		return
	}
	for _, d := range c.resources["devices"] {
		if dev, ok := d.(map[string]interface{}); ok {
			if dev["id"] == id {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(dev)
				return
			}
		}
	}
	http.NotFound(w, r)
}

// handleNodeSourceById handles GET /sources/{id}
func (c *nmosController) handleNodeSourceById(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	c.mu.RLock()
	db := c.db
	c.mu.RUnlock()

	if db != nil {
		source, err := db.GetSource(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if source == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(source)
		return
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.resources == nil || c.resources["sources"] == nil {
		http.NotFound(w, r)
		return
	}
	for _, s := range c.resources["sources"] {
		if src, ok := s.(map[string]interface{}); ok {
			if src["id"] == id {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(src)
				return
			}
		}
	}
	http.NotFound(w, r)
}

// handleNodeFlowById handles GET /flows/{id}
func (c *nmosController) handleNodeFlowById(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	c.mu.RLock()
	db := c.db
	c.mu.RUnlock()

	if db != nil {
		flow, err := db.GetFlow(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if flow == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(flow)
		return
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.resources == nil || c.resources["flows"] == nil {
		http.NotFound(w, r)
		return
	}
	for _, f := range c.resources["flows"] {
		if fl, ok := f.(map[string]interface{}); ok {
			if fl["id"] == id {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(fl)
				return
			}
		}
	}
	http.NotFound(w, r)
}

// handleNodeSenderById handles GET /senders/{id}
func (c *nmosController) handleNodeSenderById(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	c.mu.RLock()
	db := c.db
	c.mu.RUnlock()

	if db != nil {
		sender, err := db.GetSender(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if sender == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sender)
		return
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.resources == nil || c.resources["senders"] == nil {
		http.NotFound(w, r)
		return
	}
	for _, s := range c.resources["senders"] {
		if sndr, ok := s.(map[string]interface{}); ok {
			if sndr["id"] == id {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(sndr)
				return
			}
		}
	}
	http.NotFound(w, r)
}

// handleNodeReceiverById handles GET /receivers/{id}
func (c *nmosController) handleNodeReceiverById(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	c.mu.RLock()
	db := c.db
	c.mu.RUnlock()

	if db != nil {
		receiver, err := db.GetReceiver(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if receiver == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(receiver)
		return
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.resources == nil || c.resources["receivers"] == nil {
		http.NotFound(w, r)
		return
	}
	for _, rcv := range c.resources["receivers"] {
		if r, ok := rcv.(map[string]interface{}); ok {
			if r["id"] == id {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(r)
				return
			}
		}
	}
	http.NotFound(w, r)
}

// handleDeviceControls handles the /devices/{id}/controls/ endpoint
func (c *nmosController) handleDeviceControls(w http.ResponseWriter, r *http.Request) {
	// Extract device ID from URL
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 6 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	deviceID := parts[5]

	if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
		// Handle control update if a control ID is provided in the URL
		if len(parts) >= 8 && parts[7] != "" {
			controlID := parts[7]
			var body struct {
				Value interface{} `json:"value"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "Invalid body", http.StatusBadRequest)
				return
			}

			c.mu.RLock()
			cb := c.controlCallback
			c.mu.RUnlock()

			if cb != nil {
				cb(deviceID, controlID, body.Value)
			}

			// Also broadcast update as an IS-07 event
			c.BroadcastEvent(fmt.Sprintf("%s/controls/%s", deviceID, controlID), controlID, "number", body.Value)

			w.WriteHeader(http.StatusAccepted)
			return
		}
	}

	c.mu.RLock()
	controls, ok := c.deviceControls[deviceID]
	c.mu.RUnlock()

	host, portStr := splitHostPort(c.nodeAddr)
	port := 8080
	if p, err := strconv.Atoi(portStr); err == nil {
		port = p
	}

	// Build IS-04 compliant controls array with NCP control
	is04Controls := []map[string]interface{}{
		{
			"type": "urn:x-nmos:control:ncp/v1.0",
			"href": fmt.Sprintf("ws://%s:%d/x-nmos/node/v1.3/ncp", host, port),
		},
		{
			"type": "urn:x-nmos:control:events/v1.0",
			"href": fmt.Sprintf("http://%s:%d/x-nmos/events/v1.0/", host, port),
		},
		{
			"type": "urn:x-nmos:control:sr-ctrl/v1.0",
			"href": fmt.Sprintf("http://%s:%d/x-nmos/connection/v1.1/", host, port),
		},
	}

	// Add device-specific controls if they exist
	if ok && len(controls) > 0 {
		is04Controls = append(is04Controls, controls...)
	}

	slog.Debug("Serving controls", "deviceID", deviceID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(is04Controls)
}

func (c *nmosController) SetControls(deviceID string, controls []map[string]interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deviceControls[deviceID] = controls
}

func (c *nmosController) OnControlChange(callback func(deviceID, controlID string, value interface{})) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.controlCallback = callback
}

func (c *nmosController) GetControls(deviceID string) []map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.deviceControls[deviceID]
}

// RegisterResource registers a device, source, etc. with the NMOS Node API
func (c *nmosController) RegisterResource(resourceType string, resource interface{}) error {
	resourceType = c.toPlural(resourceType)
	c.mu.Lock()
	db := c.db
	c.mu.Unlock()

	// If it's a map with an ID, check if it already exists
	if resMap, ok := resource.(map[string]interface{}); ok {
		if id, ok := resMap["id"].(string); ok {
			// Try database first
			if db != nil {
				switch resourceType {
				case "devices":
					dev := Device{
						ID:          id,
						Version:     getStringField(resMap, "version"),
						Label:       getStringField(resMap, "label"),
						Description: getStringField(resMap, "description"),
						NodeID:      getStringField(resMap, "node_id"),
						Tags:        getMapField(resMap, "tags"),
						Senders:     getStringSliceField(resMap, "senders"),
						Receivers:   getStringSliceField(resMap, "receivers"),
						Controls:    getInterfaceSliceField(resMap, "controls"),
					}
					if err := db.UpsertDevice(dev); err != nil {
						slog.Error("Failed to upsert device to database", "id", id, "error", err)
					}
				case "sources":
					src := Source{
						ID:          id,
						Version:     getStringField(resMap, "version"),
						Label:       getStringField(resMap, "label"),
						Description: getStringField(resMap, "description"),
						Format:      getStringField(resMap, "format"),
						DeviceID:    getStringField(resMap, "device_id"),
						EventType:   getStringField(resMap, "event_type"),
						Tags:        getMapField(resMap, "tags"),
						GrainRate:   getIntMapField(resMap, "grain_rate"),
					}
					if err := db.UpsertSource(src); err != nil {
						slog.Error("Failed to upsert source to database", "id", id, "error", err)
					}
				case "flows":
					fl := Flow{
						ID:          id,
						Version:     getStringField(resMap, "version"),
						Label:       getStringField(resMap, "label"),
						Description: getStringField(resMap, "description"),
						Format:      getStringField(resMap, "format"),
						SourceID:    getStringField(resMap, "source_id"),
						DeviceID:    getStringField(resMap, "device_id"),
						Parents:     getStringSliceField(resMap, "parents"),
						Tags:        getMapField(resMap, "tags"),
						MediaType:   getStringField(resMap, "media_type"),
						EventType:   getStringField(resMap, "event_type"),
						GrainRate:   getIntMapField(resMap, "grain_rate"),
					}
					if err := db.UpsertFlow(fl); err != nil {
						slog.Error("Failed to upsert flow to database", "id", id, "error", err)
					}
				case "senders":
					sndr := Sender{
						ID:                 id,
						Version:            getStringField(resMap, "version"),
						Label:              getStringField(resMap, "label"),
						Description:        getStringField(resMap, "description"),
						DeviceID:           getStringField(resMap, "device_id"),
						FlowID:             getStringField(resMap, "flow_id"),
						Transport:          getStringField(resMap, "transport"),
						InterfaceBindings:  getStringSliceField(resMap, "interface_bindings"),
						Tags:               getMapField(resMap, "tags"),
						TransportParams:    getMapSliceField(resMap, "transport_params"),
					}
					if err := db.UpsertSender(sndr); err != nil {
						slog.Error("Failed to upsert sender to database", "id", id, "error", err)
					}
				case "receivers":
					rcv := Receiver{
						ID:                 id,
						Version:            getStringField(resMap, "version"),
						Label:              getStringField(resMap, "label"),
						Description:        getStringField(resMap, "description"),
						DeviceID:           getStringField(resMap, "device_id"),
						Transport:          getStringField(resMap, "transport"),
						InterfaceBindings:  getStringSliceField(resMap, "interface_bindings"),
						Tags:               getMapField(resMap, "tags"),
						TransportParams:    getMapSliceField(resMap, "transport_params"),
					}
					if err := db.UpsertReceiver(rcv); err != nil {
						slog.Error("Failed to upsert receiver to database", "id", id, "error", err)
					}
				}
			}

			// Also update in-memory if using fallback
			if db == nil || c.resources != nil {
				c.mu.Lock()
				defer c.mu.Unlock()

				// Check for updates
				updated := false
				for i, r := range c.resources[resourceType] {
					if rMap, ok := r.(map[string]interface{}); ok {
						if rMap["id"] == id {
							c.resources[resourceType][i] = resource
							slog.Debug("Updated NMOS resource", "type", resourceType, "id", id)
							updated = true
							break
						}
					}
				}

				if !updated {
					c.resources[resourceType] = append(c.resources[resourceType], resource)
					slog.Info("Registered NMOS resource", "type", resourceType)
				}

				// Auto-update parent Device's senders/receivers list
				if resourceType == "senders" || resourceType == "receivers" {
					if deviceID, ok := resMap["device_id"].(string); ok {
						for i, d := range c.resources["devices"] {
							if dMap, ok := d.(map[string]interface{}); ok {
								if dMap["id"] == deviceID {
									// Found parent device, update list
									listKey := resourceType // "senders" or "receivers"

									// Create list if missing
									if _, ok := dMap[listKey]; !ok {
										dMap[listKey] = []string{}
									}

									// Check if ID already in list
									list, _ := dMap[listKey].([]string) // Type assertion might fail if it was []interface{}, need care

									// Handle potential type mismatch if initialized as []interface{}
									if list == nil {
										if interfaceList, ok := dMap[listKey].([]interface{}); ok {
											for _, item := range interfaceList {
												if s, ok := item.(string); ok {
													list = append(list, s)
												}
											}
										}
									}

									exists := false
									for _, existingID := range list {
										if existingID == id {
											exists = true
											break
										}
									}

									if !exists {
										list = append(list, id)
										dMap[listKey] = list
										c.resources["devices"][i] = dMap // Save back

										// Notify registry of device update
										// We do this in a goroutine to avoid blocking/deadlock if registerResourceToRegistry calls back
										go c.registerResourceToRegistry(c.ctx, "devices", dMap)
									}
									break
								}
							}
						}
					}
				}
			}

			c.broadcastUpdate(resourceType, resource)

			// Update peer-to-peer version counter and mDNS TXT records if in peer-to-peer mode
			if c.peerToPeerMode {
				c.incrementAndGetVersion(resourceType)
				c.updatePeerToPeerMDNS()
			}

			// Deep copy resource before passing to registry to avoid concurrent map access
			resourceCopy, err := deepcopy(resource)
			if err != nil {
				slog.Warn("Failed to copy resource for registry", "type", resourceType, "error", err)
				return nil
			}
			// Synchronously update registry to ensure correct ordering (Source → Flow → Sender)
			if err := c.registerResourceToRegistry(c.ctx, resourceType, resourceCopy); err != nil {
				slog.Error("Failed to register resource with registry", "type", resourceType, "error", err)
			}
			return nil
		}
	}

	c.mu.Unlock()
	return fmt.Errorf("invalid resource format (missing id)")
}

// UpdateResource updates an existing NMOS resource
func (c *nmosController) UpdateResource(resourceType string, id string, updateFn func(interface{}) interface{}) error {
	resourceType = c.toPlural(resourceType)
	c.mu.Lock()

	var resourceIndex int
	var found bool
	for i, r := range c.resources[resourceType] {
		if rMap, ok := r.(map[string]interface{}); ok {
			if rMap["id"] == id {
				resourceIndex = i
				found = true
				break
			}
		}
	}
	if !found {
		c.mu.Unlock()
		return fmt.Errorf("resource not found: %s/%s", resourceType, id)
	}

	original := c.resources[resourceType][resourceIndex]
	c.mu.Unlock()

	updated := updateFn(original)

	c.mu.Lock()
	c.resources[resourceType][resourceIndex] = updated
	slog.Debug("Updated NMOS resource", "type", resourceType, "id", id)
	c.mu.Unlock()

	c.broadcastUpdate(resourceType, updated)
	return nil
}

// UpdateResourceInRegistry sends a PATCH request to update a resource in the NMOS registry
func (c *nmosController) UpdateResourceInRegistry(resourceType string, id string, updateFn func(interface{}) interface{}) error {
	resourceType = c.toPlural(resourceType)

	c.mu.Lock()
	var resourceIndex int
	var found bool
	for i, r := range c.resources[resourceType] {
		if rMap, ok := r.(map[string]interface{}); ok {
			if rMap["id"] == id {
				resourceIndex = i
				found = true
				break
			}
		}
	}
	if !found {
		c.mu.Unlock()
		return fmt.Errorf("resource not found: %s/%s", resourceType, id)
	}

	original := c.resources[resourceType][resourceIndex]
	c.mu.Unlock()

	updated := updateFn(original)

	c.mu.Lock()
	c.resources[resourceType][resourceIndex] = updated
	c.mu.Unlock()

	c.broadcastUpdate(resourceType, updated)

	wrapper := map[string]interface{}{
		"type": c.toSingular(resourceType),
		"data": updated,
	}

	resourceJSON, err := json.Marshal(wrapper)
	if err != nil {
		return fmt.Errorf("failed to marshal resource: %w", err)
	}

	url := fmt.Sprintf("%s/x-nmos/registration/v1.3/resource/%s/%s", c.registryURL, resourceType, id)
	req, err := http.NewRequestWithContext(context.Background(), "PATCH", url, bytes.NewReader(resourceJSON))
	if err != nil {
		return fmt.Errorf("failed to create registry request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to update resource in registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		slog.Info("Updated resource in registry", "type", resourceType, "id", id)
		return nil
	}

	respBody, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("registry rejected resource update: status %d, body: %s", resp.StatusCode, string(respBody))
}

// RegisterNode registers a node with the NMOS IS-04 registry
func (c *nmosController) RegisterNode(node interface{}) error {
	if !c.isRunning {
		return errors.New("controller not running")
	}

	// Use common registration function
	if err := c.registerResourceToRegistry(context.Background(), "node", node); err != nil {
		return err
	}

	// Store node locally
	c.mu.Lock()
	exists := false
	if nodeMap, ok := node.(map[string]interface{}); ok {
		if id, ok := nodeMap["id"].(string); ok {
			for i, n := range c.nodes {
				if nMap, ok := n.(map[string]interface{}); ok {
					if nMap["id"] == id {
						c.nodes[i] = node
						exists = true
						break
					}
				}
			}
		}
	}
	if !exists {
		c.nodes = append(c.nodes, node)
	}
	c.mu.Unlock()

	return nil
}

// GetNodes returns all registered nodes from the NMOS IS-04 registry
func (c *nmosController) GetNodes() ([]interface{}, error) {
	if !c.isRunning {
		return nil, errors.New("controller not running")
	}

	// Send GET request to NMOS IS-04 registry
	req, err := http.NewRequestWithContext(context.Background(), "GET",
		fmt.Sprintf("%s/nodes", c.registryURL),
		nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("NMOS registry returned error status: %d", resp.StatusCode)
	}

	// Decode response
	var nodes []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&nodes); err != nil {
		return nil, fmt.Errorf("failed to decode nodes response: %w", err)
	}

	// Update local cache
	c.mu.Lock()
	c.nodes = nodes
	c.mu.Unlock()

	return nodes, nil
}

// SubscribeToEvents returns a channel for receiving NMOS IS-05 events
func (c *nmosController) SubscribeToEvents() <-chan interface{} {
	return c.eventsChan
}

// GetNodeID returns the node's unique identifier
func (c *nmosController) GetNodeID() string {
	return c.nodeID
}

// GetListenAddr returns the actual address the HTTP server is listening on
func (c *nmosController) GetListenAddr() string {
	if c.advertisedAddr != "" && c.listenAddr != "" {
		_, port, _ := net.SplitHostPort(c.listenAddr)
		return net.JoinHostPort(c.advertisedAddr, port)
	}
	if c.listenAddr != "" {
		return c.listenAddr
	}
	return c.nodeAddr
}

// SetAdvertisedAddr sets the externally reachable address for NMOS hrefs
func (c *nmosController) SetAdvertisedAddr(addr string) {
	c.advertisedAddr = addr
}

// SetDatabase sets the database for state storage
func (c *nmosController) SetDatabase(db Database) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.db = db
}

// GetDatabase returns the current database
func (c *nmosController) GetDatabase() Database {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.db
}

// GetPrimaryInterfaceName returns the name of the primary network interface
func (c *nmosController) GetPrimaryInterfaceName() string {
	if len(c.nodeInterfaces) > 0 {
		if name, ok := c.nodeInterfaces[0]["name"].(string); ok {
			return name
		}
	}
	return "eth0"
}

// listenForEvents listens for NMOS IS-05 events from the registry
func (c *nmosController) listenForEvents(ctx context.Context) {
	// In a full implementation, this would:
	// 1. Create an mDNS query for NMOS services (optional)
	// 2. Establish a long-polling or websocket connection to the NMOS events endpoint
	// 3. Parse incoming events and send them to eventsChan
	//
	// For now, we'll implement a simple polling mechanism as a placeholder
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case <-ticker.C:
			// Poll for events (simplified implementation)
			go c.pollEvents(ctx)
		}
	}
}

// pollEvents polls the NMOS registry for events (simplified implementation)
func (c *nmosController) pollEvents(ctx context.Context) {
	// This is a simplified implementation - a real NMOS IS-05 implementation
	// would use websockets or long-polling with proper event formatting
	//
	// For demonstration purposes, we'll just check if there are any nodes
	// and simulate an event if the node count changes

	nodes, err := c.GetNodes()
	if err != nil {
		// Log error but don't break the polling loop
		return
	}

	c.mu.RLock()
	currentCount := len(c.nodes)
	c.mu.RUnlock()

	if len(nodes) != currentCount {
		// Node count changed, send an event
		event := map[string]interface{}{
			"type": "node_change",
			"data": map[string]interface{}{
				"previous_count": currentCount,
				"current_count":  len(nodes),
				"timestamp":      time.Now().Unix(),
			},
		}

		select {
		case c.eventsChan <- event:
		case <-ctx.Done():
		case <-c.done:
		}

		// Update local cache
		c.mu.Lock()
		c.nodes = nodes
		c.mu.Unlock()
	}
}

func (c *nmosController) handleConnectionRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]string{"single/"})
}

func (c *nmosController) handleConnectionSingleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]string{"senders/"})
}

func (c *nmosController) handleConnectionSendersList(w http.ResponseWriter, r *http.Request) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var ids []string
	for _, s := range c.resources["senders"] {
		if sMap, ok := s.(map[string]interface{}); ok {
			if id, ok := sMap["id"].(string); ok {
				ids = append(ids, id+"/")
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ids)
}

func (c *nmosController) handleConnectionSenderActive(w http.ResponseWriter, r *http.Request) {
	senderID := chi.URLParam(r, "senderId")
	if senderID == "" {
		http.NotFound(w, r)
		return
	}

	c.mu.RLock()
	active, exists := c.activeConnections[senderID]
	if !exists {
		active = ConnectionActive{
			SenderID:        &senderID,
			MasterEnable:    true,
			Activation:      ConnectionActivation{Mode: ActivationModeImmediate},
			TransportParams: c.getSenderTransportParams(senderID),
		}
	}
	c.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(active)
}

func (c *nmosController) getSenderTransportParams(senderID string) []map[string]interface{} {
	for _, s := range c.resources["senders"] {
		if sMap, ok := s.(map[string]interface{}); ok {
			if sMap["id"] == senderID {
				if tp, ok := sMap["transport_params"].([]map[string]interface{}); ok && len(tp) > 0 {
					return tp
				}
				if deviceID, ok := sMap["device_id"].(string); ok {
					if sourceID, ok := sMap["source_id"].(string); ok {
						return []map[string]interface{}{
							{
								"connection_uri":           fmt.Sprintf("ws://%s/x-nmos/events/v1.0/devices/%s", c.nodeAddr, deviceID),
								"connection_authorization": false,
								"ext_is_07_rest_api_url":   fmt.Sprintf("http://%s/x-nmos/events/v1.0/sources/%s/", c.nodeAddr, sourceID),
								"ext_is_07_source_id":      sourceID,
							},
						}
					}
					return []map[string]interface{}{
						{
							"connection_uri":           fmt.Sprintf("ws://%s/x-nmos/events/v1.0/devices/%s", c.nodeAddr, deviceID),
							"connection_authorization": false,
						},
					}
				}
			}
		}
	}
	return []map[string]interface{}{
		{
			"connection_uri":           fmt.Sprintf("ws://%s/x-nmos/events/v1.0/events", c.nodeAddr),
			"connection_authorization": false,
		},
	}
}

func (c *nmosController) handleConnectionSenderStaged(w http.ResponseWriter, r *http.Request) {
	senderID := chi.URLParam(r, "senderId")
	if senderID == "" {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		c.mu.RLock()
		staged, exists := c.stagedConnections[senderID]
		if !exists {
			active := c.activeConnections[senderID]
			staged = ConnectionStaged{
				SenderID:        &senderID,
				MasterEnable:    active.MasterEnable,
				Activation:      active.Activation,
				TransportParams: active.TransportParams,
			}
		}
		c.mu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(staged)

	case http.MethodPatch:
		var patch ConnectionStaged
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			http.Error(w, "Invalid body", http.StatusBadRequest)
			return
		}

		c.mu.Lock()
		current, exists := c.stagedConnections[senderID]
		if !exists {
			active := c.activeConnections[senderID]
			current = ConnectionStaged{
				SenderID:        &senderID,
				MasterEnable:    active.MasterEnable,
				Activation:      active.Activation,
				TransportParams: active.TransportParams,
			}
		}

		current.MasterEnable = patch.MasterEnable
		if patch.Activation.Mode != "" {
			current.Activation = patch.Activation
		}
		if len(patch.TransportParams) > 0 {
			current.TransportParams = patch.TransportParams
		}

		if current.Activation.Mode == ActivationModeImmediate {
			now := time.Now()
			nowStr := fmt.Sprintf("%d:%d", now.Unix(), now.Nanosecond())
			current.Activation.ActivationTime = &nowStr

			active := ConnectionActive{
				SenderID:        current.SenderID,
				MasterEnable:    current.MasterEnable,
				Activation:      current.Activation,
				TransportParams: current.TransportParams,
			}
			c.activeConnections[senderID] = active
			current.Activation.Mode = ActivationModeNull
		}
		c.stagedConnections[senderID] = current
		c.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(current)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (c *nmosController) handleConnectionSenderConstraints(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]interface{}{})
}

func getStringField(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getMapField(m map[string]interface{}, key string) map[string]interface{} {
	if v, ok := m[key].(map[string]interface{}); ok {
		return v
	}
	return nil
}

func getStringSliceField(m map[string]interface{}, key string) []string {
	if v, ok := m[key].([]string); ok {
		return v
	}
	if v, ok := m[key].([]interface{}); ok {
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

func getInterfaceSliceField(m map[string]interface{}, key string) []interface{} {
	if v, ok := m[key].([]interface{}); ok {
		return v
	}
	return nil
}

func getIntMapField(m map[string]interface{}, key string) map[string]int {
	if v, ok := m[key].(map[string]int); ok {
		return v
	}
	if v, ok := m[key].(map[string]interface{}); ok {
		result := make(map[string]int)
		for k, val := range v {
			if i, ok := val.(int); ok {
				result[k] = i
			}
		}
		return result
	}
	return nil
}

func getMapSliceField(m map[string]interface{}, key string) []map[string]interface{} {
	if v, ok := m[key].([]map[string]interface{}); ok {
		return v
	}
	return nil
}

// deepcopy creates a deep copy using reflection
// This prevents concurrent map access issues when passing maps to goroutines
func deepcopy(src interface{}) (interface{}, error) {
	v := reflect.ValueOf(src)
	if v.Kind() != reflect.Map {
		return nil, fmt.Errorf("deepcopy only works on maps, got %T", src)
	}
	if v.Type().Key().Kind() != reflect.String {
		return nil, fmt.Errorf("deepcopy only works on map[string]interface{}, got %T", src)
	}
	dst := reflect.MakeMap(v.Type())
	for _, key := range v.MapKeys() {
		val := v.MapIndex(key)
		if val.Kind() == reflect.Interface {
			val = val.Elem()
		}
		var copiedVal reflect.Value
		switch val.Kind() {
		case reflect.Map:
			subDst, err := deepcopy(val.Interface())
			if err != nil {
				return nil, err
			}
			copiedVal = reflect.ValueOf(subDst)
		case reflect.Slice:
			newSlice := reflect.MakeSlice(val.Type(), val.Len(), val.Cap())
			reflect.Copy(newSlice, val)
			copiedVal = newSlice
		default:
			copiedVal = val
		}
		dst.SetMapIndex(key, copiedVal)
	}
	return dst.Interface(), nil
}
