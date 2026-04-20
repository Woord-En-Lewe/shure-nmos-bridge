package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
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

// nmosController is the concrete implementation of NMOSController
type nmosController struct {
	registryURL    string
	registryConfig RegistryDiscoveryConfig
	nodeAddr       string
	nodeID         string
	httpClient     *http.Client
	httpServer     *http.Server
	isRunning      bool
	nodes          []interface{}
	resources      map[string][]interface{}
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
	ncpSubscriptions   map[int]map[int]NcObject // subscriptionID -> (objectID -> object) for tracking subscriptions
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
		addr = "localhost:8080" // Default NMOS Node API address
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
		resources:          make(map[string][]interface{}),
		deviceControls:     make(map[string][]map[string]interface{}),
		ncpObjects:         make(map[int]NcObject),
		ncpClients:         make(map[*ncpClient]bool),
		ncpSubscriptions:   make(map[int]map[int]NcObject),
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
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for the gateway
			},
		},
	}

	// Register Root Block (OID 1)
	rootBlock := NewNcBlock(1, nil, "Root", "Root Block")
	ctrl.RegisterNCPObject(1, rootBlock)

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

	return nil
}

// discoverAndRegister discovers NMOS registries via mDNS and registers this node
func (c *nmosController) discoverAndRegister(ctx context.Context) {
	// Try to discover registry via mDNS
	registryURL, err := c.discoverRegistry(ctx)
	if err != nil {
		slog.Warn("Registry discovery failed, using default", "error", err)
	} else {
		c.registryURL = registryURL
	}

	// Build node resource using the persistent node ID
	node := c.buildNodeResource(c.nodeID)

	// Register with registry
	if err := c.RegisterNode(node); err != nil {
		slog.Warn("Failed to register node with registry", "error", err)
		return
	}

	// Mark as successfully registered
	c.registeredMu.Lock()
	c.registered = true
	c.registeredMu.Unlock()

	// Signal that registration is complete
	select {
	case c.registryResolved <- c.registryURL:
	default:
	}

	// Start heartbeating
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.startHeartbeat(ctx)
	}()
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
		slog.Warn("Heartbeat failed", "url", heartbeatURL, "error", err)
		return
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		slog.Debug("NMOS Heartbeat successful")
	case http.StatusNotFound:
		slog.Warn("NMOS Registry returned 404 for heartbeat, re-registering everything")
		c.reRegisterAll(ctx)
	default:
		slog.Warn("NMOS Registry returned unexpected status for heartbeat", "status", resp.StatusCode)
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
	// Check if we were ever successfully registered
	c.registeredMu.RLock()
	wasRegistered := c.registered
	c.registeredMu.RUnlock()

	if !wasRegistered {
		slog.Info("Skipping NMOS unregistration - node was never registered")
		return
	}

	slog.Info("Performing NMOS controlled unregistration")

	// Order of deletion (reverse of registration): Receivers -> Senders -> Flows -> Sources -> Devices
	resourceTypes := []string{"receivers", "senders", "flows", "sources", "devices"}

	c.mu.RLock()
	nodeID := c.nodeID
	// Copy resource IDs to avoid holding lock during network calls
	allResourceIDs := make(map[string][]string)
	for _, t := range resourceTypes {
		for _, res := range c.resources[t] {
			if resMap, ok := res.(map[string]interface{}); ok {
				if id, ok := resMap["id"].(string); ok {
					allResourceIDs[t] = append(allResourceIDs[t], id)
				}
			}
		}
	}
	c.mu.RUnlock()

	for _, resourceType := range resourceTypes {
		for _, id := range allResourceIDs[resourceType] {
			if err := c.unregisterResourceFromRegistry(ctx, resourceType, id); err != nil {
				slog.Warn("Failed to unregister resource", "type", resourceType, "id", id, "error", err)
			}
		}
	}

	// Finally, unregister the Node itself
	if err := c.unregisterResourceFromRegistry(ctx, "node", nodeID); err != nil {
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

	select {
	case <-ctx.Done():
		return "", ctx.Err()
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
	host, portStr := splitHostPort(c.nodeAddr)
	port := 8080
	if p, err := strconv.Atoi(portStr); err == nil {
		port = p
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
		"hostname":   host,
		"interfaces": []interface{}{},
		"clocks":     []interface{}{},
	}
}

// splitHostPort separates host and port from an address string
func splitHostPort(addr string) (host, port string) {
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx], addr[idx+1:]
	}
	return addr, ""
}

// startServer initializes and starts the NMOS Node API HTTP server
func (c *nmosController) startServer() error {
	r := c.setupRouter()

	c.httpServer = &http.Server{
		Addr:    c.nodeAddr,
		Handler: r,
	}

	go func() {
		slog.Info("Starting NMOS Node API server", "address", c.nodeAddr)
		if err := c.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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

	r.Get("/x-nmos/node/v1.3", c.handleNodeRoot)
	r.Get("/x-nmos/node/v1.3/", c.handleNodeRoot)
	r.Get("/x-nmos/node/v1.3/self", c.handleNodeSelf)
	r.Get("/x-nmos/node/v1.3/self/", c.handleNodeSelf)
	r.Get("/x-nmos/node/v1.3/devices", c.handleNodeDevices)
	r.Get("/x-nmos/node/v1.3/devices/", c.handleNodeDevices)
	r.Route("/x-nmos/node/v1.3/devices/{id}/controls", func(r chi.Router) {
		r.Get("/", c.handleDeviceControls)
		r.Post("/", c.handleDeviceControls)
		r.Put("/", c.handleDeviceControls)
		r.Patch("/", c.handleDeviceControls)
	})
	r.Get("/x-nmos/node/v1.3/sources", c.handleNodeSources)
	r.Get("/x-nmos/node/v1.3/sources/", c.handleNodeSources)
	r.Get("/x-nmos/node/v1.3/flows", c.handleNodeFlows)
	r.Get("/x-nmos/node/v1.3/flows/", c.handleNodeFlows)
	r.Get("/x-nmos/node/v1.3/senders", c.handleNodeSenders)
	r.Get("/x-nmos/node/v1.3/senders/", c.handleNodeSenders)
	r.Get("/x-nmos/node/v1.3/receivers", c.handleNodeReceivers)
	r.Get("/x-nmos/node/v1.3/receivers/", c.handleNodeReceivers)

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

	return r
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

// BroadcastNCPNotification sends a notification to all connected NCP clients
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
			subscribed := c.handleSubscriptions(conn, ncpMsg.Subscriptions)
			respMsg := NCPMessage{
				MessageType:   NCPMessageTypeSubscriptionResponse,
				Subscriptions: subscribed,
			}
			conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
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
// Returns list of OIDs that were successfully subscribed (per IS-12 spec)
func (c *nmosController) handleSubscriptions(conn *websocket.Conn, subscriptionOIDs []int) []int {
	var subscribed []int

	for _, oid := range subscriptionOIDs {
		c.ncpMu.RLock()
		_, ok := c.ncpObjects[oid]
		c.ncpMu.RUnlock()

		if !ok {
			continue
		}

		subscribed = append(subscribed, oid)
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
	endpoints := []string{
		"self/",
		"devices/",
		"sources/",
		"flows/",
		"senders/",
		"receivers/",
		"ncp/",
	}
	// Note: In a full NMOS implementation, /events would be discovered via mDNS
	// or documented at the root level if using a unified API
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(endpoints)
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

	// Self representation
	self := map[string]interface{}{
		"id":          c.nodeID,
		"version":     fmt.Sprintf("%d:%d", time.Now().Unix(), time.Now().Nanosecond()),
		"label":       "Shure-NMOS Gateway Node",
		"description": "Gateway connecting Shure Axient to NMOS",
		"tags":        map[string]interface{}{},
		"caps":        map[string]interface{}{},
		"api":         map[string]interface{}{"versions": []string{"v1.3"}, "endpoints": []map[string]interface{}{{"host": host, "port": port, "protocol": "http"}}},
		"hostname":    host,
		"interfaces":  []interface{}{},
		"clocks":      []interface{}{},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(self)
}

// handleNodeDevices handles the /devices endpoint
func (c *nmosController) handleNodeDevices(w http.ResponseWriter, r *http.Request) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(c.resources["devices"])
}

// handleNodeSources handles the /sources endpoint
func (c *nmosController) handleNodeSources(w http.ResponseWriter, r *http.Request) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(c.resources["sources"])
}

// handleNodeFlows handles the /flows endpoint
func (c *nmosController) handleNodeFlows(w http.ResponseWriter, r *http.Request) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(c.resources["flows"])
}

// handleNodeSenders handles the /senders endpoint
func (c *nmosController) handleNodeSenders(w http.ResponseWriter, r *http.Request) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(c.resources["senders"])
}

// handleNodeReceivers handles the /receivers endpoint
func (c *nmosController) handleNodeReceivers(w http.ResponseWriter, r *http.Request) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(c.resources["receivers"])
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

	// If it's a map with an ID, check if it already exists
	if resMap, ok := resource.(map[string]interface{}); ok {
		if id, ok := resMap["id"].(string); ok {
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

			c.mu.Unlock()
			c.broadcastUpdate(resourceType, resource)
			// Also update in registry
			go c.registerResourceToRegistry(c.ctx, resourceType, resource)
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
