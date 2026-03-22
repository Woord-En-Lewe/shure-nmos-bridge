package module

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/Woord-En-Lewe/shure-nmos-bridge/internal/infrastructure"
	"github.com/google/uuid"
)

// Gateway defines the interface for the Shure-NMOS gateway
type Gateway interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// shureDeviceInfo tracks an active Shure controller and its metadata
type shureDeviceInfo struct {
	ctrl           infrastructure.ShureController
	lastSeen       time.Time
	modelFamily    infrastructure.ShureModelFamily // Detected model family
	nmosDeviceIDs  map[int]string                  // channel -> deviceID
	deviceOID      int                             // OID of the device block in NCP tree
	deviceInstance string                          // Device instance name (e.g. from mDNS discovery)
	parameterOIDs  map[string]int                  // param_key -> oid (e.g. "1_AUDIO_GAIN" -> 101)
	sourceIDs      map[int]map[string]string       // channel -> param -> sourceID
	flowIDs        map[int]map[string]string       // channel -> param -> flowID
	senderIDs      map[int]map[string]string       // channel -> param -> senderID
}

// gatewayImpl is the concrete implementation of the Gateway interface
type gatewayImpl struct {
	shureAddr  string
	nmosAddr   string
	shureCtrls map[string]*shureDeviceInfo
	nmosCtrl   infrastructure.NMOSController
	messageBus infrastructure.MessageBus
	discoverer *infrastructure.ShureDiscoverer
	mu         sync.RWMutex
}

// NewGateway creates a new Gateway instance
func NewGateway(shureAddr, nmosAddr string) Gateway {
	return &gatewayImpl{
		shureAddr:  shureAddr,
		nmosAddr:   nmosAddr,
		shureCtrls: make(map[string]*shureDeviceInfo),
	}
}

// Start initializes and starts the gateway components
func (g *gatewayImpl) Start(ctx context.Context) error {
	// Initialize infrastructure components
	g.messageBus = infrastructure.NewInMemoryMessageBus()
	if g.messageBus == nil {
		return fmt.Errorf("failed to create message bus")
	}

	g.nmosCtrl = infrastructure.NewNMOSController(g.nmosAddr)
	if g.nmosCtrl == nil {
		return fmt.Errorf("failed to create nmos controller")
	}

	if err := g.nmosCtrl.Start(ctx); err != nil {
		return err
	}

	// Start mDNS discovery
	g.discoverer = infrastructure.NewShureDiscoverer()
	devices, err := g.discoverer.Discover(ctx)
	if err != nil {
		return fmt.Errorf("failed to start discovery: %w", err)
	}

	// Handle discovered devices
	go g.handleDiscovery(ctx, devices)

	// Start device reaper
	go g.reapStaleDevices(ctx)

	// If a specific address was provided, also connect to it
	if g.shureAddr != "" {
		g.addShureController(ctx, g.shureAddr, infrastructure.DiscoveredDevice{
			Instance: "manual",
			Address:  net.ParseIP(strings.Split(g.shureAddr, ":")[0]),
			Port:     2202,
			Info:     map[string]string{"source": "manual"},
		})
	}

	// Start message processing
	go g.processMessages(ctx)

	return nil
}

// handleDiscovery listens for discovered devices and adds them
func (g *gatewayImpl) handleDiscovery(ctx context.Context, devices <-chan infrastructure.DiscoveredDevice) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
			// Periodically check ctx without blocking on devices channel
			continue
		case dev, ok := <-devices:
			if !ok {
				// Channel closed
				return
			}
			// Filter out Wireless Workbench (WWB) instances
			if strings.Contains(strings.ToUpper(dev.Instance), "WWB") {
				continue
			}

			addr := fmt.Sprintf("%s:%d", dev.Address, dev.Port)
			g.addShureController(ctx, addr, dev)
		}
	}
}

// addShureController creates and starts a new Shure controller for an address
func (g *gatewayImpl) addShureController(ctx context.Context, addr string, dev infrastructure.DiscoveredDevice) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if info, ok := g.shureCtrls[addr]; ok {
		info.lastSeen = time.Now() // Refresh heartbeat
		return
	}

	ctrl := infrastructure.NewShureController(addr)
	if err := ctrl.Start(ctx); err != nil {
		slog.Error("Failed to start Shure controller", "address", addr, "error", err)
		return
	}

	deviceID := uuid.New().String()
	deviceOID := 100 + (len(g.shureCtrls)+1)*10
	g.shureCtrls[addr] = &shureDeviceInfo{
		ctrl:           ctrl,
		lastSeen:       time.Now(),
		nmosDeviceIDs:  map[int]string{0: deviceID},
		deviceOID:      deviceOID,
		deviceInstance: dev.Instance,
		parameterOIDs:  make(map[string]int),
		sourceIDs:      make(map[int]map[string]string),
		flowIDs:        make(map[int]map[string]string),
		senderIDs:      make(map[int]map[string]string),
	}
	slog.Info("Connected to Shure device", "address", addr)

	// Start event listener for this controller
	go g.listenToShureEvents(ctx, addr, ctrl.ReceiveEvents())

	// Discovery Sequence
	go func() {
		time.Sleep(500 * time.Millisecond)
		slog.Info("Requesting full device discovery", "address", addr)

		// 1. Get device-level parameters first (MODEL, DEVICE_ID, FW_VER) to detect family
		ctrl.SendCommand(infrastructure.NewShureCommand("GET").
			WithIndex(0).
			WithParam("MODEL", nil).
			Build())
		ctrl.SendCommand(infrastructure.NewShureCommand("GET").
			WithIndex(0).
			WithParam("DEVICE_ID", nil).
			Build())
		ctrl.SendCommand(infrastructure.NewShureCommand("GET").
			WithIndex(0).
			WithParam("FW_VER", nil).
			Build())

		// Wait for model detection before querying channel params
		// The model family will be set when we receive REP(MODEL)
		// Retry MODEL query if no response after 500ms
		waited := 0
		for {
			time.Sleep(250 * time.Millisecond)
			waited += 250
			family := infrastructure.ModelFamilyAxientDigital
			g.mu.RLock()
			if info, ok := g.shureCtrls[addr]; ok && info.modelFamily != "" {
				family = info.modelFamily
			}
			g.mu.RUnlock()
			if family != infrastructure.ModelFamilyAxientDigital || waited >= 1000 {
				break
			}
			// Retry MODEL query
			ctrl.SendCommand(infrastructure.NewShureCommand("GET").
				WithIndex(0).
				WithParam("MODEL", nil).
				Build())
		}

		// 2. Query channel-specific parameters for channels 1-4
		// Get current model family (may still be default, FormatParamName handles this)
		family := infrastructure.ModelFamilyAxientDigital
		g.mu.RLock()
		if info, ok := g.shureCtrls[addr]; ok && info.modelFamily != "" {
			family = info.modelFamily
		}
		g.mu.RUnlock()

		// Channel control params (use underscores - FormatParamName will convert for ULX-D/QLX-D)
		controlParams := []string{
			"AUDIO_GAIN", "AUDIO_MUTE", "CHAN_NAME",
			"FREQUENCY", "GROUP_CHANNEL",
		}

		for ch := 1; ch <= 4; ch++ {
			for _, param := range controlParams {
				cmd := infrastructure.NewShureCommandWithModel("GET", family).
					WithIndex(ch).
					WithParam(param, nil).
					Build()
				ctrl.SendCommand(cmd)
			}
			time.Sleep(50 * time.Millisecond)
		}

		// 3. Query battery/telemetry params (common names across families)
		telemetryParams := []string{
			"TX_BATT_BARS", "TX_BATT_CHARGE_PERCENT", "TX_BATT_MINS",
			"TX_BATT_TEMP_C", "TX_BATT_CYCLE_COUNT", "TX_BATT_HEALTH_PERCENT",
		}

		for ch := 1; ch <= 4; ch++ {
			for _, param := range telemetryParams {
				cmd := infrastructure.NewShureCommandWithModel("GET", family).
					WithIndex(ch).
					WithParam(param, nil).
					Build()
				ctrl.SendCommand(cmd)
			}
			time.Sleep(50 * time.Millisecond)
		}

		// 4. Set METER_RATE to 1000ms for all channels 1-4
		for ch := 1; ch <= 4; ch++ {
			ctrl.SendCommand(fmt.Sprintf("< SET %d METER_RATE 01000 >\n", ch))
			time.Sleep(20 * time.Millisecond)
		}

		// 5. Start SAMPLE ALL for channels 1-4
		for ch := 1; ch <= 4; ch++ {
			ctrl.SendCommand(fmt.Sprintf("< SAMPLE %d ALL >\n", ch))
			time.Sleep(20 * time.Millisecond)
		}
	}()

	// Initial NMOS Registration
	g.nmosCtrl.RegisterResource("devices", map[string]interface{}{
		"id":          deviceID,
		"version":     fmt.Sprintf("%d:%d", time.Now().Unix(), time.Now().Nanosecond()),
		"label":       dev.Instance,
		"description": fmt.Sprintf("Axient Digital at %s", addr),
		"tags":        map[string]interface{}{"source": dev.Info["source"]},
		"node_id":     g.nmosCtrl.GetNodeID(),
		"senders":     []string{},
		"receivers":   []string{},
		"controls": []interface{}{
			map[string]interface{}{
				"type": "urn:x-nmos:control:sr-ctrl/v1.0",
				"href": fmt.Sprintf("http://%s/x-nmos/connection/v1.1/", g.nmosAddr),
			},
			map[string]interface{}{
				"type": "urn:x-nmos:control:events/v1.0",
				"href": fmt.Sprintf("http://%s/x-nmos/events/v1.0/", g.nmosAddr),
			},
			map[string]interface{}{
				"type": "urn:x-nmos:control:ncp/v1.0",
				"href": fmt.Sprintf("ws://%s/x-nmos/node/v1.3/ncp", g.nmosAddr),
			},
		},
	})

	// Initialize channel maps for lazy IS-07 resource registration
	// IS-04/IS-07 resources (Sources, Flows, Senders) are only registered
	// when the device actually sends data for a parameter
	for i := 1; i <= 4; i++ {
		g.shureCtrls[addr].nmosDeviceIDs[i] = deviceID
		// Initialize maps for lazy IS-07 resource registration
		// Resources are only registered when the device actually sends data for a parameter
		g.shureCtrls[addr].sourceIDs[i] = make(map[string]string)
		g.shureCtrls[addr].flowIDs[i] = make(map[string]string)
		g.shureCtrls[addr].senderIDs[i] = make(map[string]string)
	}

	// IS-12 NCP Setup
	// Register custom classes if they are not already registered
	g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
		Name:    "GainWorker",
		ClassID: []int{1, 2, 1, 1},
		Properties: []infrastructure.NcPropertyDescriptor{
			{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean"},
			{Name: "gain", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcFloat32"},
		},
	})
	g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
		Name:    "MuteWorker",
		ClassID: []int{1, 2, 1, 2},
		Properties: []infrastructure.NcPropertyDescriptor{
			{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean"},
			{Name: "mute", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean"},
		},
	})

	// Use a simple OID allocation (In a real app, this should be more robust)
	devBlock := infrastructure.NewNcBlock(deviceOID, nil, "Device", dev.Instance)
	g.nmosCtrl.RegisterNCPObject(deviceOID, devBlock)

	// Add to Root Block (OID 1)
	if root := g.nmosCtrl.GetNCPObject(1); root != nil {
		if rb, ok := root.(*infrastructure.NcBlock); ok {
			rb.AddItem(deviceOID)
		}
	}
}

// listenToShureEvents listens for events from a specific Shure controller
func (g *gatewayImpl) listenToShureEvents(ctx context.Context, addr string, events <-chan interface{}) {
	slog.Info("listenToShureEvents started", "address", addr)
	defer slog.Info("listenToShureEvents exiting", "address", addr)
	for {
		select {
		case <-ctx.Done():
			slog.Info("listenToShureEvents ctx done", "address", addr)
			return
		case ev, ok := <-events:
			if !ok {
				// Channel closed, exit
				slog.Info("listenToShureEvents events channel closed", "address", addr)
				return
			}
			if report, ok := ev.(*infrastructure.TPCIReport); ok {
				// Log ALL responses for discovery debugging
				if strings.Contains(report.Raw, "REP") {
					slog.Debug("Axient Report Received",
						"address", addr,
						"raw", report.Raw)
				}

				// Log significant changes
				if report.Param == "MODEL" || report.Param == "DEVICE_ID" || report.Param == "FW_VER" {
					slog.Info("Axient Capability Discovered",
						"address", addr,
						"param", report.Param,
						"value", report.Value)
				}

				// Forward to message bus for NMOS translation
				slog.Debug("listenToShureEvents sending to messageBus", "address", addr, "param", report.Param)
				if err := g.messageBus.Send(infrastructure.Message{
					Type:    infrastructure.ShureDeviceMsg,
					Payload: report,
					Source:  addr,
				}); err != nil {
					slog.Warn("listenToShureEvents send failed", "address", addr, "error", err)
				}
			}
		}
	}
}

// Stop gracefully shuts down the gateway components
func (g *gatewayImpl) Stop(ctx context.Context) error {
	slog.Info("Gateway Stop called")
	g.mu.Lock()
	defer g.mu.Unlock()
	slog.Info("Gateway Stop lock acquired")

	// Stop discoverer
	if g.discoverer != nil {
		slog.Info("Stopping discoverer")
		if err := g.discoverer.Stop(); err != nil {
			slog.Error("Error stopping discoverer", "error", err)
		}
	}

	// Stop all Shure controllers
	slog.Info("Stopping Shure controllers", "count", len(g.shureCtrls))
	for addr, info := range g.shureCtrls {
		slog.Info("Stopping Shure controller", "address", addr)
		if err := info.ctrl.Stop(ctx); err != nil {
			slog.Error("Error stopping Shure controller", "address", addr, "error", err)
		}
	}
	slog.Info("All Shure controllers stopped")

	// Stop NMOS controller
	slog.Info("Stopping NMOS controller")
	if err := g.nmosCtrl.Stop(ctx); err != nil {
		slog.Error("Error stopping NMOS controller", "error", err)
		return err
	}
	slog.Info("NMOS controller stopped")

	// Stop message bus
	slog.Info("Stopping message bus")
	if mb, ok := g.messageBus.(*infrastructure.InMemoryMessageBus); ok {
		mb.Close()
	}
	slog.Info("Message bus stopped")

	slog.Info("Gateway Stop complete")

	return nil
}

// processMessages handles message passing between Shure and NMOS controllers
func (g *gatewayImpl) processMessages(ctx context.Context) {
	slog.Info("processMessages started")
	defer slog.Info("processMessages exiting")
	for {
		select {
		case <-ctx.Done():
			slog.Info("processMessages ctx done")
			return
		case msg, ok := <-g.messageBus.Receive():
			if !ok {
				slog.Info("processMessages receive channel closed")
				return
			}
			// Translate Shure messages to NMOS and vice versa
			// This is where the actual protocol translation happens
			switch msg.Type {
			case infrastructure.ShureDeviceMsg:
				g.handleShureDevice(msg)
			case infrastructure.NMOSNodeMsg:
				g.handleNMOSNode(msg)
			}
		}
	}
}

// getNMOSEventType returns the NMOS event type for a Shure parameter
// ensureIS07Resources registers IS-04/IS-07 resources for a channel/param if not already registered
// This enables lazy registration - we only create IS-07 senders when the device actually sends data
func (g *gatewayImpl) ensureIS07Resources(info *shureDeviceInfo, channel int, param string, deviceID string, deviceInstance string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Check if already registered - must check if maps exist first
	if info.sourceIDs != nil && info.sourceIDs[channel] != nil {
		if _, exists := info.sourceIDs[channel][param]; exists {
			return
		}
	}

	// Initialize maps if needed
	if info.sourceIDs == nil {
		info.sourceIDs = make(map[int]map[string]string)
	}
	if info.sourceIDs[channel] == nil {
		info.sourceIDs[channel] = make(map[string]string)
	}
	if info.flowIDs == nil {
		info.flowIDs = make(map[int]map[string]string)
	}
	if info.flowIDs[channel] == nil {
		info.flowIDs[channel] = make(map[string]string)
	}
	if info.senderIDs == nil {
		info.senderIDs = make(map[int]map[string]string)
	}
	if info.senderIDs[channel] == nil {
		info.senderIDs[channel] = make(map[string]string)
	}

	// Generate IDs
	sourceID := uuid.New().String()
	flowID := uuid.New().String()
	senderID := uuid.New().String()

	// Store the IDs
	info.sourceIDs[channel][param] = sourceID
	info.flowIDs[channel][param] = flowID
	info.senderIDs[channel][param] = senderID

	eventType := getNMOSEventType(param)

	// Register Source (IS-04)
	g.nmosCtrl.RegisterResource("sources", map[string]interface{}{
		"id":          sourceID,
		"version":     fmt.Sprintf("%d:%d", time.Now().Unix(), time.Now().Nanosecond()),
		"label":       fmt.Sprintf("%s Channel %d %s", deviceInstance, channel, param),
		"description": fmt.Sprintf("Event source for %s on Channel %d", param, channel),
		"format":      "urn:x-nmos:format:data",
		"caps":        map[string]interface{}{},
		"device_id":   deviceID,
		"parents":     []string{},
		"clock_name":  nil,
		"event_type":  eventType,
	})

	// Register Flow (IS-04)
	g.nmosCtrl.RegisterResource("flows", map[string]interface{}{
		"id":          flowID,
		"version":     fmt.Sprintf("%d:%d", time.Now().Unix(), time.Now().Nanosecond()),
		"label":       fmt.Sprintf("%s Channel %d %s Flow", deviceInstance, channel, param),
		"description": fmt.Sprintf("Event flow for %s on Channel %d", param, channel),
		"format":      "urn:x-nmos:format:data",
		"tags":        map[string]interface{}{},
		"source_id":   sourceID,
		"device_id":   deviceID,
		"parents":     []string{},
		"media_type":  "application/json",
		"event_type":  eventType,
	})

	// Register Sender (IS-04) with IS-05/IS-07 transport parameters
	g.nmosCtrl.RegisterResource("senders", map[string]interface{}{
		"id":                 senderID,
		"version":            fmt.Sprintf("%d:%d", time.Now().Unix(), time.Now().Nanosecond()),
		"label":              fmt.Sprintf("%s Channel %d %s Sender", deviceInstance, channel, param),
		"description":        fmt.Sprintf("IS-07 event sender for %s on Channel %d", param, channel),
		"device_id":          deviceID,
		"flow_id":            flowID,
		"transport":          "urn:x-nmos:transport:websocket",
		"interface_bindings": []string{"eth0"},
		"manifest_href":      nil,
		"transport_params": []map[string]interface{}{
			{
				"ext_is_07_rest_api_url": fmt.Sprintf("http://%s/x-nmos/events/v1.0/sources/%s/", g.nmosAddr, sourceID),
				"ext_is_07_source_id":    sourceID,
			},
		},
	})
}

func getNMOSEventType(param string) string {
	switch param {
	case "AUDIO_MUTE", "MUTE":
		return "boolean"
	case "AUDIO_GAIN", "AUDIO_LEVEL_PEAK", "AUDIO_LEVEL_RMS", "CHAN_QUALITY",
		"RF_RSSI", "RF_RSSI_A", "RF_RSSI_B", "RF_RSSI_C", "RF_RSSI_D",
		"RF_RSSI_F1", "RF_RSSI_F2", "RF_LEVEL",
		"TX_BATT_BARS", "TX_BATT_CHARGE_PERCENT", "TX_BATT_MINS", "TX_BATT_TEMP_C",
		"TX_BATT_CYCLE_COUNT", "TX_BATT_HEALTH_PERCENT",
		"AUDIO_LED_BITMAP", "RF_LED_BITMAP_A", "RF_LED_BITMAP_B",
		"RF_LED_BITMAP_C", "RF_LED_BITMAP_D", "RF_LED_BITMAP_F1", "RF_LED_BITMAP_F2":
		return "number"
	default:
		return "string"
	}
}

// handleShureDevice processes Shure device messages and translates to NMOS
func (g *gatewayImpl) handleShureDevice(msg infrastructure.Message) {
	report, ok := msg.Payload.(*infrastructure.TPCIReport)
	if !ok {
		return
	}

	g.mu.RLock()
	info, ok := g.shureCtrls[msg.Source]
	g.mu.RUnlock()
	if !ok {
		return
	}

	deviceID, ok := info.nmosDeviceIDs[report.Channel]
	if !ok {
		// Use channel 0 ID if channel-specific ID doesn't exist
		deviceID = info.nmosDeviceIDs[0]
	}

	// IS-07 (Events) Logic
	if report.Type == "SAMPLE" && report.Param == "ALL" {
		// Use specialized parsers based on detected model family
		switch info.modelFamily {
		case infrastructure.ModelFamilyAxientDigital:
			if sample := infrastructure.ParseSampleReport(report.Raw); sample != nil {
				if sID, ok := info.sourceIDs[report.Channel]["CHAN_QUALITY"]; ok {
					g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["CHAN_QUALITY"], "number", sample.Quality)
				}
				if sID, ok := info.sourceIDs[report.Channel]["AUDIO_LED_BITMAP"]; ok {
					g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["AUDIO_LED_BITMAP"], "number", sample.AudioLEDBitmap)
				}
				if sID, ok := info.sourceIDs[report.Channel]["AUDIO_LEVEL_PEAK"]; ok {
					g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["AUDIO_LEVEL_PEAK"], "number", sample.AudioLevelPeakDBFS())
				}
				if sID, ok := info.sourceIDs[report.Channel]["AUDIO_LEVEL_RMS"]; ok {
					g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["AUDIO_LEVEL_RMS"], "number", sample.AudioLevelRMSDBFS())
				}
				if sID, ok := info.sourceIDs[report.Channel]["ANTENNA_STATUS"]; ok {
					g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["ANTENNA_STATUS"], "string", sample.RFAntStatus)
				}
				if sID, ok := info.sourceIDs[report.Channel]["RF_LED_BITMAP_A"]; ok {
					g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["RF_LED_BITMAP_A"], "number", sample.RFBitmapA)
				}
				if sID, ok := info.sourceIDs[report.Channel]["RF_RSSI_A"]; ok {
					g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["RF_RSSI_A"], "number", sample.RFRSSI_A_DBM())
				}
				if sID, ok := info.sourceIDs[report.Channel]["RF_LED_BITMAP_B"]; ok {
					g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["RF_LED_BITMAP_B"], "number", sample.RFBitmapB)
				}
				if sID, ok := info.sourceIDs[report.Channel]["RF_RSSI_B"]; ok {
					g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["RF_RSSI_B"], "number", sample.RFRSSI_B_DBM())
				}
				// Quadversity (4 antennas)
				if sample.RFBitmapC > 0 || sample.RFRSSI_C > 0 {
					if sID, ok := info.sourceIDs[report.Channel]["RF_LED_BITMAP_C"]; ok {
						g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["RF_LED_BITMAP_C"], "number", sample.RFBitmapC)
					}
					if sID, ok := info.sourceIDs[report.Channel]["RF_RSSI_C"]; ok {
						g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["RF_RSSI_C"], "number", sample.RFRSSI_C_DBM())
					}
					if sID, ok := info.sourceIDs[report.Channel]["RF_LED_BITMAP_D"]; ok {
						g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["RF_LED_BITMAP_D"], "number", sample.RFBitmapD)
					}
					if sID, ok := info.sourceIDs[report.Channel]["RF_RSSI_D"]; ok {
						g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["RF_RSSI_D"], "number", sample.RFRSSI_D_DBM())
					}
				}
				// Frequency Diversity (F1/F2)
				if sample.RFBitmapF1 > 0 || sample.RFRSSI_F1 > 0 {
					if sID, ok := info.sourceIDs[report.Channel]["RF_LED_BITMAP_F1"]; ok {
						g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["RF_LED_BITMAP_F1"], "number", sample.RFBitmapF1)
					}
					if sID, ok := info.sourceIDs[report.Channel]["RF_RSSI_F1"]; ok {
						g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["RF_RSSI_F1"], "number", sample.RFRSSI_F1_DBM())
					}
				}
				if sample.RFBitmapF2 > 0 || sample.RFRSSI_F2 > 0 {
					if sID, ok := info.sourceIDs[report.Channel]["RF_LED_BITMAP_F2"]; ok {
						g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["RF_LED_BITMAP_F2"], "number", sample.RFBitmapF2)
					}
					if sID, ok := info.sourceIDs[report.Channel]["RF_RSSI_F2"]; ok {
						g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["RF_RSSI_F2"], "number", sample.RFRSSI_F2_DBM())
					}
				}
			}
		case infrastructure.ModelFamilyULXD, infrastructure.ModelFamilyQLXD:
			if sample := infrastructure.ParseULXDSampleReport(report.Raw); sample != nil {
				if sID, ok := info.sourceIDs[report.Channel]["ANTENNA_STATUS"]; ok {
					g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["ANTENNA_STATUS"], "string", string(sample.AntStatus))
				}
				if sID, ok := info.sourceIDs[report.Channel]["RF_LEVEL"]; ok {
					g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["RF_LEVEL"], "number", sample.RFLevelDBM())
				}
				if sID, ok := info.sourceIDs[report.Channel]["AUDIO_LEVEL_PEAK"]; ok {
					g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["AUDIO_LEVEL_PEAK"], "number", sample.AudioLevelDBFS())
				}
				if sID, ok := info.sourceIDs[report.Channel]["AUDIO_LEVEL_RMS"]; ok {
					g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["AUDIO_LEVEL_RMS"], "number", sample.AudioLevelDBFS())
				}
			}
		case infrastructure.ModelFamilySLXD, infrastructure.ModelFamilySLXDPlus:
			if sample := infrastructure.ParseSLDXSampleReport(report.Raw); sample != nil {
				if sID, ok := info.sourceIDs[report.Channel]["AUDIO_LEVEL_PEAK"]; ok {
					g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["AUDIO_LEVEL_PEAK"], "number", sample.AudioPeakDBFS())
				}
				if sID, ok := info.sourceIDs[report.Channel]["AUDIO_LEVEL_RMS"]; ok {
					g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["AUDIO_LEVEL_RMS"], "number", sample.AudioRMSDBFS())
				}
				if sID, ok := info.sourceIDs[report.Channel]["RF_RSSI"]; ok {
					g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["RF_RSSI"], "number", sample.RFRSSIDBM())
				}
			}
		default:
			// Fallback: detect format automatically
			format := infrastructure.DetectSampleFormat(report.Raw)
			switch format {
			case "axient":
				// Re-parse as Axient
				if sample := infrastructure.ParseSampleReport(report.Raw); sample != nil {
					info.modelFamily = infrastructure.ModelFamilyAxientDigital
					g.handleAxientSampleEvents(info, report.Channel, sample)
				}
			case "ulxd":
				if sample := infrastructure.ParseULXDSampleReport(report.Raw); sample != nil {
					info.modelFamily = infrastructure.ModelFamilyULXD
					g.handleULXDSampleEvents(info, report.Channel, sample)
				}
			case "sldx":
				if sample := infrastructure.ParseSLDXSampleReport(report.Raw); sample != nil {
					info.modelFamily = infrastructure.ModelFamilySLXD
					g.handleSLDXSampleEvents(info, report.Channel, sample)
				}
			}
		}
	}

	// IS-07: Metered params create senders and broadcast events
	if infrastructure.IsMeteredParam(report.Param) {
		// REP responses establish current state - broadcast as IS-07 event
		// This allows consumers to subscribe and receive current values
		deviceID := info.nmosDeviceIDs[0]
		deviceInstance := info.deviceInstance
		g.ensureIS07Resources(info, report.Channel, report.Param, deviceID, deviceInstance)
		if sID, ok := info.sourceIDs[report.Channel][report.Param]; ok {
			g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel][report.Param], getNMOSEventType(report.Param), report.Value)
		}
	}

	// IS-12 NCP Parameter Updates
	// REP messages should go to NCP (including metered params for control)
	if report.Type == "REP" && report.Param != "ALL" {
		paramKey := fmt.Sprintf("%d_%s", report.Channel, report.Param)
		g.mu.Lock()
		oid, exists := info.parameterOIDs[paramKey]

		// Decide if we should create a worker for this parameter
		// We avoid purely informational or transient parameters
		shouldCreateWorker := !exists &&
			report.Param != "METER_RATE" &&
			report.Param != "SAMPLE" &&
			report.Param != "FLASH"

		if shouldCreateWorker {
			// Allocate a new OID for this parameter
			oid = 1000 + len(info.parameterOIDs) + (len(g.shureCtrls) * 100)
			info.parameterOIDs[paramKey] = oid

			// Create Worker based on parameter type
			var classID []int
			switch report.Param {
			case "AUDIO_GAIN":
				classID = []int{1, 2, 1, 1} // Gain Worker
			case "AUDIO_MUTE", "MUTE":
				classID = []int{1, 2, 1, 2} // Mute Worker
			default:
				classID = []int{1, 2} // Generic Worker
			}

			worker := infrastructure.NewNcWorker(oid, classID, nil, report.Param, fmt.Sprintf("%s Channel %d", report.Param, report.Channel))
			worker.Value = report.Value

			// Set callback to send command back to Shure
			paramToSet := report.Param
			worker.OnSet = func(val interface{}) error {
				cmd := fmt.Sprintf("< SET %d %s %v >\n", report.Channel, paramToSet, val)
				return info.ctrl.SendCommand(cmd)
			}

			g.nmosCtrl.RegisterNCPObject(oid, worker)

			// Add to Device Block
			if devObj := g.nmosCtrl.GetNCPObject(info.deviceOID); devObj != nil {
				if db, ok := devObj.(*infrastructure.NcBlock); ok {
					db.AddItem(oid)
				}
			}
		}
		g.mu.Unlock()

		// Update existing worker value
		if exists {
			if obj := g.nmosCtrl.GetNCPObject(oid); obj != nil {
				if worker, ok := obj.(*infrastructure.NcWorker); ok {
					worker.SetProperty(infrastructure.NCPPropertyID{Level: 2, Index: 1}, report.Value)
				}
			}
		}
	}

	if deviceID == "" {
		return
	}

	// Update the NMOS resource (IS-04) with the new data
	g.nmosCtrl.UpdateResource("devices", deviceID, func(r interface{}) interface{} {
		res, ok := r.(map[string]interface{})
		if !ok {
			return r
		}

		tags, ok := res["tags"].(map[string]interface{})
		if !ok {
			tags = make(map[string]interface{})
			res["tags"] = tags
		}

		// Handle different parameters for tags
		if report.Param == "MODEL" {
			res["description"] = fmt.Sprintf("%s at %s", report.Value, msg.Source)
			// Detect and store model family
			family := infrastructure.DetectModelFamily(report.Value)
			g.mu.Lock()
			if info, ok := g.shureCtrls[msg.Source]; ok {
				info.modelFamily = family
				slog.Info("Detected Shure model family", "address", msg.Source, "model", report.Value, "family", family)
			}
			g.mu.Unlock()
		}

		if report.Param == "DEVICE_ID" {
			res["label"] = report.Value
		}

		// Update tags for visibility
		tags[report.Param] = []string{fmt.Sprint(report.Value)}

		// Dynamically assign controls if this is a new parameter
		// Exclude internal/metering commands
		if report.Param != "METER_RATE" && report.Param != "SAMPLE" && report.Param != "ALL" {
			controls := g.nmosCtrl.GetControls(deviceID)
			found := false
			for _, c := range controls {
				if c["name"] == report.Param {
					found = true
					break
				}
			}

			if !found {
				newControl := map[string]interface{}{
					"name":  report.Param,
					"type":  "number",
					"value": report.Value,
				}
				if report.Param == "AUDIO_MUTE" || report.Param == "MUTE" {
					newControl["type"] = "boolean"
				}

				controls = append(controls, newControl)
				g.nmosCtrl.SetControls(deviceID, controls)
			}
		}

		// Update tags from SAMPLE ALL too
		if report.Type == "SAMPLE" && report.Param == "ALL" {
			vals := strings.Fields(report.Value)
			if len(vals) >= 9 {
				tags["channel_quality"] = []string{vals[0]}
				tags["audio_led_bitmap"] = []string{vals[1]}
				tags["audio_peak"] = []string{vals[2]}
				tags["audio_rms"] = []string{vals[3]}
				tags["antenna_status"] = []string{vals[4]}
				tags["rf_led_bitmap_a"] = []string{vals[5]}
				tags["rf_rssi_a"] = []string{vals[6]}
				tags["rf_led_bitmap_b"] = []string{vals[7]}
				tags["rf_rssi_b"] = []string{vals[8]}
			}
		}

		res["version"] = fmt.Sprintf("%d:%d", time.Now().Unix(), time.Now().Nanosecond())
		return res
	})
}

// handleNMOSNode processes NMOS node messages and translates to Shure
func (g *gatewayImpl) handleNMOSNode(msg infrastructure.Message) {
	// Implementation would translate NMOS node/device/resource to Shure device state
	// For now, this is a placeholder
}

func (g *gatewayImpl) handleAxientSampleEvents(info *shureDeviceInfo, channel int, sample *infrastructure.SampleReport) {
	deviceID := info.nmosDeviceIDs[0]
	deviceInstance := info.deviceInstance

	// Ensure IS-07 resources exist for each parameter present in the SAMPLE data
	g.ensureIS07Resources(info, channel, "CHAN_QUALITY", deviceID, deviceInstance)
	if sID, ok := info.sourceIDs[channel]["CHAN_QUALITY"]; ok {
		g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[channel]["CHAN_QUALITY"], "number", sample.Quality)
	}

	g.ensureIS07Resources(info, channel, "AUDIO_LED_BITMAP", deviceID, deviceInstance)
	if sID, ok := info.sourceIDs[channel]["AUDIO_LED_BITMAP"]; ok {
		g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[channel]["AUDIO_LED_BITMAP"], "number", sample.AudioLEDBitmap)
	}

	g.ensureIS07Resources(info, channel, "AUDIO_LEVEL_PEAK", deviceID, deviceInstance)
	if sID, ok := info.sourceIDs[channel]["AUDIO_LEVEL_PEAK"]; ok {
		g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[channel]["AUDIO_LEVEL_PEAK"], "number", sample.AudioLevelPeakDBFS())
	}

	g.ensureIS07Resources(info, channel, "AUDIO_LEVEL_RMS", deviceID, deviceInstance)
	if sID, ok := info.sourceIDs[channel]["AUDIO_LEVEL_RMS"]; ok {
		g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[channel]["AUDIO_LEVEL_RMS"], "number", sample.AudioLevelRMSDBFS())
	}

	g.ensureIS07Resources(info, channel, "ANTENNA_STATUS", deviceID, deviceInstance)
	if sID, ok := info.sourceIDs[channel]["ANTENNA_STATUS"]; ok {
		g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[channel]["ANTENNA_STATUS"], "string", sample.RFAntStatus)
	}

	g.ensureIS07Resources(info, channel, "RF_LED_BITMAP_A", deviceID, deviceInstance)
	if sID, ok := info.sourceIDs[channel]["RF_LED_BITMAP_A"]; ok {
		g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[channel]["RF_LED_BITMAP_A"], "number", sample.RFBitmapA)
	}

	g.ensureIS07Resources(info, channel, "RF_RSSI_A", deviceID, deviceInstance)
	if sID, ok := info.sourceIDs[channel]["RF_RSSI_A"]; ok {
		g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[channel]["RF_RSSI_A"], "number", sample.RFRSSI_A_DBM())
	}

	g.ensureIS07Resources(info, channel, "RF_LED_BITMAP_B", deviceID, deviceInstance)
	if sID, ok := info.sourceIDs[channel]["RF_LED_BITMAP_B"]; ok {
		g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[channel]["RF_LED_BITMAP_B"], "number", sample.RFBitmapB)
	}

	g.ensureIS07Resources(info, channel, "RF_RSSI_B", deviceID, deviceInstance)
	if sID, ok := info.sourceIDs[channel]["RF_RSSI_B"]; ok {
		g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[channel]["RF_RSSI_B"], "number", sample.RFRSSI_B_DBM())
	}

	if sample.RFBitmapC > 0 || sample.RFRSSI_C > 0 {
		g.ensureIS07Resources(info, channel, "RF_LED_BITMAP_C", deviceID, deviceInstance)
		if sID, ok := info.sourceIDs[channel]["RF_LED_BITMAP_C"]; ok {
			g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[channel]["RF_LED_BITMAP_C"], "number", sample.RFBitmapC)
		}
		g.ensureIS07Resources(info, channel, "RF_RSSI_C", deviceID, deviceInstance)
		if sID, ok := info.sourceIDs[channel]["RF_RSSI_C"]; ok {
			g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[channel]["RF_RSSI_C"], "number", sample.RFRSSI_C_DBM())
		}
		g.ensureIS07Resources(info, channel, "RF_LED_BITMAP_D", deviceID, deviceInstance)
		if sID, ok := info.sourceIDs[channel]["RF_LED_BITMAP_D"]; ok {
			g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[channel]["RF_LED_BITMAP_D"], "number", sample.RFBitmapD)
		}
		g.ensureIS07Resources(info, channel, "RF_RSSI_D", deviceID, deviceInstance)
		if sID, ok := info.sourceIDs[channel]["RF_RSSI_D"]; ok {
			g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[channel]["RF_RSSI_D"], "number", sample.RFRSSI_D_DBM())
		}
	}
	if sample.RFBitmapF1 > 0 || sample.RFRSSI_F1 > 0 {
		g.ensureIS07Resources(info, channel, "RF_LED_BITMAP_F1", deviceID, deviceInstance)
		if sID, ok := info.sourceIDs[channel]["RF_LED_BITMAP_F1"]; ok {
			g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[channel]["RF_LED_BITMAP_F1"], "number", sample.RFBitmapF1)
		}
		g.ensureIS07Resources(info, channel, "RF_RSSI_F1", deviceID, deviceInstance)
		if sID, ok := info.sourceIDs[channel]["RF_RSSI_F1"]; ok {
			g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[channel]["RF_RSSI_F1"], "number", sample.RFRSSI_F1_DBM())
		}
	}
	if sample.RFBitmapF2 > 0 || sample.RFRSSI_F2 > 0 {
		g.ensureIS07Resources(info, channel, "RF_LED_BITMAP_F2", deviceID, deviceInstance)
		if sID, ok := info.sourceIDs[channel]["RF_LED_BITMAP_F2"]; ok {
			g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[channel]["RF_LED_BITMAP_F2"], "number", sample.RFBitmapF2)
		}
		g.ensureIS07Resources(info, channel, "RF_RSSI_F2", deviceID, deviceInstance)
		if sID, ok := info.sourceIDs[channel]["RF_RSSI_F2"]; ok {
			g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[channel]["RF_RSSI_F2"], "number", sample.RFRSSI_F2_DBM())
		}
	}
}

func (g *gatewayImpl) handleULXDSampleEvents(info *shureDeviceInfo, channel int, sample *infrastructure.ULXDSampleReport) {
	deviceID := info.nmosDeviceIDs[0]
	deviceInstance := info.deviceInstance

	g.ensureIS07Resources(info, channel, "ANTENNA_STATUS", deviceID, deviceInstance)
	if sID, ok := info.sourceIDs[channel]["ANTENNA_STATUS"]; ok {
		g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[channel]["ANTENNA_STATUS"], "string", string(sample.AntStatus))
	}

	g.ensureIS07Resources(info, channel, "RF_LEVEL", deviceID, deviceInstance)
	if sID, ok := info.sourceIDs[channel]["RF_LEVEL"]; ok {
		g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[channel]["RF_LEVEL"], "number", sample.RFLevelDBM())
	}

	g.ensureIS07Resources(info, channel, "AUDIO_LEVEL_PEAK", deviceID, deviceInstance)
	if sID, ok := info.sourceIDs[channel]["AUDIO_LEVEL_PEAK"]; ok {
		g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[channel]["AUDIO_LEVEL_PEAK"], "number", sample.AudioLevelDBFS())
	}

	g.ensureIS07Resources(info, channel, "AUDIO_LEVEL_RMS", deviceID, deviceInstance)
	if sID, ok := info.sourceIDs[channel]["AUDIO_LEVEL_RMS"]; ok {
		g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[channel]["AUDIO_LEVEL_RMS"], "number", sample.AudioLevelDBFS())
	}
}

func (g *gatewayImpl) handleSLDXSampleEvents(info *shureDeviceInfo, channel int, sample *infrastructure.SLDXSampleReport) {
	deviceID := info.nmosDeviceIDs[0]
	deviceInstance := info.deviceInstance

	g.ensureIS07Resources(info, channel, "AUDIO_LEVEL_PEAK", deviceID, deviceInstance)
	if sID, ok := info.sourceIDs[channel]["AUDIO_LEVEL_PEAK"]; ok {
		g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[channel]["AUDIO_LEVEL_PEAK"], "number", sample.AudioPeakDBFS())
	}

	g.ensureIS07Resources(info, channel, "AUDIO_LEVEL_RMS", deviceID, deviceInstance)
	if sID, ok := info.sourceIDs[channel]["AUDIO_LEVEL_RMS"]; ok {
		g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[channel]["AUDIO_LEVEL_RMS"], "number", sample.AudioRMSDBFS())
	}

	g.ensureIS07Resources(info, channel, "RF_RSSI", deviceID, deviceInstance)
	if sID, ok := info.sourceIDs[channel]["RF_RSSI"]; ok {
		g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[channel]["RF_RSSI"], "number", sample.RFRSSIDBM())
	}
}

// reapStaleDevices periodically removes devices that haven't been seen recently
func (g *gatewayImpl) reapStaleDevices(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			g.mu.Lock()
			for addr, info := range g.shureCtrls {
				// If we haven't seen the device for 2 minutes, remove it
				if time.Since(info.lastSeen) > 2*time.Minute {
					slog.Info("Removing stale Shure device", "address", addr)
					info.ctrl.Stop(ctx)
					delete(g.shureCtrls, addr)
				}
			}
			g.mu.Unlock()
		}
	}
}
