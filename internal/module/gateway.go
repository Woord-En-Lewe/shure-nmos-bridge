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
	ctrl          infrastructure.ShureController
	lastSeen      time.Time
	nmosDeviceIDs map[int]string            // channel -> deviceID
	parameterOIDs map[string]int            // param_key -> oid (e.g. "1_AUDIO_GAIN" -> 101)
	sourceIDs     map[int]map[string]string // channel -> param -> sourceID
	flowIDs       map[int]map[string]string // channel -> param -> flowID
	senderIDs     map[int]map[string]string // channel -> param -> senderID
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
		case dev := <-devices:
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
	g.shureCtrls[addr] = &shureDeviceInfo{
		ctrl:          ctrl,
		lastSeen:      time.Now(),
		nmosDeviceIDs: map[int]string{0: deviceID},
		parameterOIDs: make(map[string]int),
		sourceIDs:     make(map[int]map[string]string),
		flowIDs:       make(map[int]map[string]string),
		senderIDs:     make(map[int]map[string]string),
	}
	slog.Info("Connected to Shure device", "address", addr)

	// Start event listener for this controller
	go g.listenToShureEvents(ctx, addr, ctrl.ReceiveEvents())

	// Discovery Sequence
	go func() {
		time.Sleep(500 * time.Millisecond)
		slog.Info("Requesting full device discovery", "address", addr)
		ctrl.SendCommand(infrastructure.GetAllCommand(0))

		// Set METER_RATE to 1000ms (1 second) for all channels to start periodic sampling
		time.Sleep(100 * time.Millisecond)
		ctrl.SendCommand(fmt.Sprintf("< SET 0 METER_RATE 01000 >\n"))

		// Start SAMPLE ALL for all channels to receive metered values
		time.Sleep(100 * time.Millisecond)
		ctrl.SendCommand(fmt.Sprintf("< SAMPLE 0 ALL >\n"))
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

	// Register Sources, Flows and Senders for up to 4 channels
	meteredParams := []string{
		"CHAN_QUALITY", "AUDIO_LED_BITMAP", "AUDIO_LEVEL_PEAK", "AUDIO_LEVEL_RMS",
		"ANTENNA_STATUS", "RF_LED_BITMAP_A", "RF_RSSI_A", "RF_LED_BITMAP_B", "RF_RSSI_B",
		"RF_LED_BITMAP_C", "RF_RSSI_C", "RF_LED_BITMAP_D", "RF_RSSI_D",
	}

	for i := 1; i <= 4; i++ {
		g.shureCtrls[addr].nmosDeviceIDs[i] = deviceID
		g.shureCtrls[addr].sourceIDs[i] = make(map[string]string)
		g.shureCtrls[addr].flowIDs[i] = make(map[string]string)
		g.shureCtrls[addr].senderIDs[i] = make(map[string]string)

		for _, param := range meteredParams {
			sourceID := uuid.New().String()
			flowID := uuid.New().String()
			senderID := uuid.New().String()
			
			// Normalize param key (to lower case for internal lookups if needed, or keep upper for consistency with Shure)
			// Let's keep upper case key for consistency with Shure param names
			g.shureCtrls[addr].sourceIDs[i][param] = sourceID
			g.shureCtrls[addr].flowIDs[i][param] = flowID
			g.shureCtrls[addr].senderIDs[i][param] = senderID

			eventType := getNMOSEventType(param)

			// Register Source (IS-04)
			g.nmosCtrl.RegisterResource("sources", map[string]interface{}{
				"id":          sourceID,
				"version":     fmt.Sprintf("%d:%d", time.Now().Unix(), time.Now().Nanosecond()),
				"label":       fmt.Sprintf("%s Channel %d %s", dev.Instance, i, param),
				"description": fmt.Sprintf("Event source for %s on Channel %d", param, i),
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
				"label":       fmt.Sprintf("%s Channel %d %s Flow", dev.Instance, i, param),
				"description": fmt.Sprintf("Event flow for %s on Channel %d", param, i),
				"format":      "urn:x-nmos:format:data",
				"tags":        map[string]interface{}{},
				"source_id":   sourceID,
				"device_id":   deviceID,
				"parents":     []string{},
				"media_type":  "application/json",
				"event_type":  eventType,
			})

			// Register Sender (IS-04)
			g.nmosCtrl.RegisterResource("senders", map[string]interface{}{
				"id":          senderID,
				"version":     fmt.Sprintf("%d:%d", time.Now().Unix(), time.Now().Nanosecond()),
				"label":       fmt.Sprintf("%s Channel %d %s Sender", dev.Instance, i, param),
				"description": fmt.Sprintf("IS-07 event sender for %s on Channel %d", param, i),
				"device_id":   deviceID,
				"flow_id":     flowID,
				"transport":   "urn:x-nmos:transport:websocket",
				"interface_bindings": []string{"eth0"},
				"manifest_href":      nil,
			})
		}
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
	deviceOID := 100 + len(g.shureCtrls)*10
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
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-events:
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
				g.messageBus.Send(infrastructure.Message{
					Type:    infrastructure.ShureDeviceMsg,
					Payload: report,
					Source:  addr,
				})
			}
		}
	}
}

// Stop gracefully shuts down the gateway components
func (g *gatewayImpl) Stop(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Stop discoverer
	if g.discoverer != nil {
		if err := g.discoverer.Stop(); err != nil {
			slog.Error("Error stopping discoverer", "error", err)
		}
	}

	// Stop all Shure controllers
	for addr, info := range g.shureCtrls {
		if err := info.ctrl.Stop(ctx); err != nil {
			slog.Error("Error stopping Shure controller", "address", addr, "error", err)
		}
	}

	// Stop NMOS controller
	if err := g.nmosCtrl.Stop(ctx); err != nil {
		return err
	}

	return nil
}

// processMessages handles message passing between Shure and NMOS controllers
func (g *gatewayImpl) processMessages(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-g.messageBus.Receive():
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
func getNMOSEventType(param string) string {
	switch param {
	case "AUDIO_MUTE", "MUTE":
		return "boolean"
	case "AUDIO_GAIN", "AUDIO_LEVEL_PEAK", "AUDIO_LEVEL_RMS", "RSSI", "RF_RSSI_A", "RF_RSSI_B", "CHAN_QUALITY":
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
		vals := strings.Fields(report.Value)
		if len(vals) >= 9 {
			if sID, ok := info.sourceIDs[report.Channel]["CHAN_QUALITY"]; ok {
				g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["CHAN_QUALITY"], "number", vals[0])
			}
			if sID, ok := info.sourceIDs[report.Channel]["AUDIO_LED_BITMAP"]; ok {
				g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["AUDIO_LED_BITMAP"], "string", vals[1])
			}
			if sID, ok := info.sourceIDs[report.Channel]["AUDIO_LEVEL_PEAK"]; ok {
				g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["AUDIO_LEVEL_PEAK"], "number", vals[2])
			}
			if sID, ok := info.sourceIDs[report.Channel]["AUDIO_LEVEL_RMS"]; ok {
				g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["AUDIO_LEVEL_RMS"], "number", vals[3])
			}
			if sID, ok := info.sourceIDs[report.Channel]["ANTENNA_STATUS"]; ok {
				g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["ANTENNA_STATUS"], "string", vals[4])
			}
			if sID, ok := info.sourceIDs[report.Channel]["RF_LED_BITMAP_A"]; ok {
				g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["RF_LED_BITMAP_A"], "string", vals[5])
			}
			if sID, ok := info.sourceIDs[report.Channel]["RF_RSSI_A"]; ok {
				g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["RF_RSSI_A"], "number", vals[6])
			}
			if sID, ok := info.sourceIDs[report.Channel]["RF_LED_BITMAP_B"]; ok {
				g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["RF_LED_BITMAP_B"], "string", vals[7])
			}
			if sID, ok := info.sourceIDs[report.Channel]["RF_RSSI_B"]; ok {
				g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["RF_RSSI_B"], "number", vals[8])
			}
			
			// If it's Quadversity, there are more
			if len(vals) >= 13 {
				if sID, ok := info.sourceIDs[report.Channel]["RF_LED_BITMAP_C"]; ok {
					g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["RF_LED_BITMAP_C"], "string", vals[9])
				}
				if sID, ok := info.sourceIDs[report.Channel]["RF_RSSI_C"]; ok {
					g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["RF_RSSI_C"], "number", vals[10])
				}
				if sID, ok := info.sourceIDs[report.Channel]["RF_LED_BITMAP_D"]; ok {
					g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["RF_LED_BITMAP_D"], "string", vals[11])
				}
				if sID, ok := info.sourceIDs[report.Channel]["RF_RSSI_D"]; ok {
					g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["RF_RSSI_D"], "number", vals[12])
				}
			}
		}
	} else if infrastructure.IsMeteredParam(report.Param) {
		// Even if it comes via REP (GET response), if it's a metered param, it goes to IS-07
		if sID, ok := info.sourceIDs[report.Channel][report.Param]; ok {
			g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel][report.Param], getNMOSEventType(report.Param), report.Value)
		}
	}

	// IS-12 NCP Parameter Updates
	// REP messages should go to NCP
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
			deviceOID := 100 + (len(g.shureCtrls)-1)*10 // Approximate device OID
			if devObj := g.nmosCtrl.GetNCPObject(deviceOID); devObj != nil {
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
