package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// nmosController is the concrete implementation of NMOSController
type nmosController struct {
	registryURL    string
	nodeAddr       string
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
	upgrader websocket.Upgrader
	clients  map[*websocket.Conn]bool
}

// corsMiddleware adds CORS headers to all responses
func (c *nmosController) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// NewNMOSController creates a new NMOSController instance
func NewNMOSController(addr string) NMOSController {
	if addr == "" {
		addr = "localhost:8080" // Default NMOS Node API address
	}
	return &nmosController{
		nodeAddr:       addr,
		registryURL:    "http://localhost:8000", // Default NMOS registry address
		httpClient:     &http.Client{Timeout: 10 * time.Second},
		resources:      make(map[string][]interface{}),
		deviceControls: make(map[string][]map[string]interface{}),
		eventsChan:     make(chan interface{}, 100),
		done:           make(chan struct{}),
		clients:        make(map[*websocket.Conn]bool),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for the gateway
			},
		},
	}
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

	// Start goroutine to handle connection lifecycle
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		<-ctx.Done()
		c.Stop(context.Background())
	}()

	// Start goroutine to listen for NMOS events (IS-05)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.listenForEvents(ctx)
	}()

	return nil
}

// startServer initializes and starts the NMOS Node API HTTP server
func (c *nmosController) startServer() error {
	mux := http.NewServeMux()

	// Wrap mux with CORS middleware
	corsMux := c.corsMiddleware(mux)

	// Implement basic IS-04 Node API endpoints
	mux.HandleFunc("/x-nmos/node/v1.3/", c.handleNodeRoot)
	mux.HandleFunc("/x-nmos/node/v1.3/self/", c.handleNodeSelf)
	mux.HandleFunc("/x-nmos/node/v1.3/devices/", c.handleNodeDevices)
	mux.HandleFunc("/x-nmos/node/v1.3/devices/{id}/controls/", c.handleDeviceControls)
	mux.HandleFunc("/x-nmos/node/v1.3/sources/", c.handleNodeSources)
	mux.HandleFunc("/x-nmos/node/v1.3/flows/", c.handleNodeFlows)
	mux.HandleFunc("/x-nmos/node/v1.3/senders/", c.handleNodeSenders)
	mux.HandleFunc("/x-nmos/node/v1.3/receivers/", c.handleNodeReceivers)

	// Implement IS-07 Event & Tally Websocket endpoint
	mux.HandleFunc("/x-nmos/events/v1.0/", c.handleEventsRoot)
	mux.HandleFunc("/x-nmos/events/v1.0/ws", c.handleWebsocket)

	c.httpServer = &http.Server{
		Addr:    c.nodeAddr,
		Handler: corsMux,
	}

	go func() {
		slog.Info("Starting NMOS Node API server", "address", c.nodeAddr)
		if err := c.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("NMOS Node API server error", "error", err)
		}
	}()

	return nil
}

// Stop halts the NMOS controller
func (c *nmosController) Stop(ctx context.Context) error {
	if !c.isRunning {
		return nil
	}

	c.isRunning = false
	close(c.done)

	// Gracefully shut down the HTTP server
	if c.httpServer != nil {
		slog.Info("Shutting down NMOS Node API server")
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := c.httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("Error during NMOS Node API server shutdown", "error", err)
		}
	}

	// Wait for goroutines to finish
	c.wg.Wait()

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
	}
	// Note: In a full NMOS implementation, /events would be discovered via mDNS
	// or documented at the root level if using a unified API
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(endpoints)
}

// handleEventsRoot handles the root of the Events API
func (c *nmosController) handleEventsRoot(w http.ResponseWriter, r *http.Request) {
	endpoints := []string{
		"ws",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(endpoints)
}

// handleWebsocket handles IS-07 websocket connections
func (c *nmosController) handleWebsocket(w http.ResponseWriter, r *http.Request) {
	conn, err := c.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("Failed to upgrade websocket", "error", err)
		return
	}

	c.mu.Lock()
	c.clients[conn] = true
	c.mu.Unlock()

	slog.Info("New NMOS IS-07 websocket client connected")

	// Read loop to handle client disconnects and keep-alives
	go func() {
		defer func() {
			c.mu.Lock()
			delete(c.clients, conn)
			c.mu.Unlock()
			conn.Close()
			slog.Info("NMOS IS-07 websocket client disconnected")
		}()

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
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

	for _, client := range clients {
		if err := client.WriteMessage(websocket.TextMessage, updateJSON); err != nil {
			slog.Error("Failed to send websocket update", "error", err)
			c.mu.Lock()
			delete(c.clients, client)
			c.mu.Unlock()
			client.Close()
		}
	}
}

// handleNodeSelf handles the /self endpoint
func (c *nmosController) handleNodeSelf(w http.ResponseWriter, r *http.Request) {
	// Simple self representation
	self := map[string]interface{}{
		"id":          "00000000-0000-0000-0000-000000000000",
		"version":     fmt.Sprintf("%d:%d", time.Now().Unix(), time.Now().Nanosecond()),
		"label":       "Shure-NMOS Gateway Node",
		"description": "Gateway connecting Shure Axient to NMOS",
		"tags":        map[string]interface{}{},
		"caps":        map[string]interface{}{},
		"api":         map[string]interface{}{"versions": []string{"v1.3"}, "endpoints": []map[string]interface{}{{"host": "localhost", "port": 8080, "protocol": "http"}}},
		"hostname":    "localhost",
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

	c.mu.RLock()
	controls, ok := c.deviceControls[deviceID]
	c.mu.RUnlock()

	if !ok {
		// Fallback for devices without specific controls set yet
		controls = []map[string]interface{}{
			{
				"name":        "Device Discovery",
				"parameter":   "GET_ALL",
				"type":        "trigger",
				"description": "Trigger dynamic capability discovery",
			},
		}
	}

	slog.Debug("Serving controls", "deviceID", deviceID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(controls)
}

func (c *nmosController) SetControls(deviceID string, controls []map[string]interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deviceControls[deviceID] = controls
}

func (c *nmosController) GetControls(deviceID string) []map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.deviceControls[deviceID]
}

// RegisterResource registers a device, source, etc. with the NMOS Node API
func (c *nmosController) RegisterResource(resourceType string, resource interface{}) error {
	c.mu.Lock()

	// If it's a map with an ID, check if it already exists
	if resMap, ok := resource.(map[string]interface{}); ok {
		if id, ok := resMap["id"].(string); ok {
			for i, r := range c.resources[resourceType] {
				if rMap, ok := r.(map[string]interface{}); ok {
					if rMap["id"] == id {
						c.resources[resourceType][i] = resource
						slog.Debug("Updated NMOS resource", "type", resourceType, "id", id)
						c.mu.Unlock()
						c.broadcastUpdate(resourceType, resource)
						return nil
					}
				}
			}
		}
	}

	c.resources[resourceType] = append(c.resources[resourceType], resource)
	slog.Info("Registered NMOS resource", "type", resourceType)
	c.mu.Unlock()
	c.broadcastUpdate(resourceType, resource)
	return nil
}

// UpdateResource updates an existing NMOS resource
func (c *nmosController) UpdateResource(resourceType string, id string, updateFn func(interface{}) interface{}) error {
	c.mu.Lock()

	for i, r := range c.resources[resourceType] {
		if rMap, ok := r.(map[string]interface{}); ok {
			if rMap["id"] == id {
				updated := updateFn(r)
				c.resources[resourceType][i] = updated
				slog.Debug("Updated NMOS resource", "type", resourceType, "id", id)
				c.mu.Unlock()
				c.broadcastUpdate(resourceType, updated)
				return nil
			}
		}
	}

	c.mu.Unlock()
	return fmt.Errorf("resource not found: %s/%s", resourceType, id)
}

// RegisterNode registers a node with the NMOS IS-04 registry
func (c *nmosController) RegisterNode(node interface{}) error {
	if !c.isRunning {
		return errors.New("controller not running")
	}

	// Convert node to JSON
	nodeJSON, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("failed to marshal node: %w", err)
	}

	// Send POST request to NMOS IS-04 registry
	req, err := http.NewRequestWithContext(context.Background(), "POST",
		fmt.Sprintf("%s/nodes", c.registryURL),
		bytes.NewReader(nodeJSON))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to register node: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("NMOS registry returned error status: %d", resp.StatusCode)
	}

	// Store node locally
	c.mu.Lock()
	c.nodes = append(c.nodes, node)
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
