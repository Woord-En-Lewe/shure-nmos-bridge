package module

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jean-pierrecoetzee/shure_nmos_gateway/internal/infrastructure"
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
	nmosDeviceIDs  map[int]string // Channel -> NMOS Device ID
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
		nmosDeviceIDs: map[int]string{0: deviceID, 1: deviceID}, // Default mapping for initial discovery
	}
	slog.Info("Connected to Shure device", "address", addr)

	// Start event listener for this controller
	go g.listenToShureEvents(ctx, addr, ctrl.ReceiveEvents())

	// Discovery Sequence: Use the "God Command" to get everything
	go func() {
		time.Sleep(500 * time.Millisecond)
		slog.Info("Requesting full device discovery", "address", addr)
		ctrl.SendCommand(infrastructure.GetAllCommand(0))
		
		// Set METER_RATE to 1000ms (1 second) for channel 1 to start monitoring
		time.Sleep(100 * time.Millisecond)
		ctrl.SendCommand(fmt.Sprintf("< SET 1 METER_RATE 01000 >\n"))
	}()

	// Initial NMOS Registration (will be updated as ALL reports come in)
	g.nmosCtrl.RegisterResource("devices", map[string]interface{}{
		"id":          deviceID,
		"version":     fmt.Sprintf("%d:%d", time.Now().Unix(), time.Now().Nanosecond()),
		"label":       dev.Instance,
		"description": fmt.Sprintf("Axient Digital at %s", addr),
		"tags":        map[string]interface{}{"source": dev.Info["source"]},
		"node_id":     "00000000-0000-0000-0000-000000000000",
		"controls": []interface{}{
			map[string]interface{}{
				"type": "href",
				"href": fmt.Sprintf("http://localhost:8080/x-nmos/node/v1.3/devices/%s/controls", deviceID),
			},
		},
	})
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

	if deviceID == "" {
		return
	}

	// Update the NMOS resource with the new data
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

		// Handle different parameters
		if strings.Contains(report.Raw, "SAMPLE") && report.Param == "ALL" {
			// Parse sample ALL values according to documentation:
			// qual audBitmap audPeak audRms rfAntStats rfBitmapA rfRssiA rfBitmapB rfRssiB
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
		} else {
			switch report.Param {
			case "TX_BATT_BARS", "BATT_BARS":
				tags["battery_bars"] = []string{report.Value}
			case "TX_BATT_CHARGE_PERCENT", "BATT_CHARGE":
				tags["battery_percent"] = []string{report.Value}
			case "AUDIO_LEVEL_PEAK", "AUDIO_LVL":
				tags["audio_peak"] = []string{report.Value}
			case "AUDIO_LEVEL_RMS":
				tags["audio_rms"] = []string{report.Value}
			case "BATT_RUN_TIME":
				tags["battery_runtime"] = []string{report.Value}
			case "METER_RATE":
				tags["meter_rate"] = []string{report.Value}
			case "CHAN_NAME":
				res["label"] = report.Value
			case "DEVICE_ID":
				res["label"] = report.Value
			}
		}

		res["version"] = fmt.Sprintf("%d:%d", time.Now().Unix(), time.Now().Nanosecond())
		return res
	})

	// Dynamically assign controls if this is a new parameter (Moved outside to prevent deadlock)
	if report.Param != "" && report.Param != "ALL" {
		g.ensureControlExists(deviceID, report.Param)
	}
}

// ensureControlExists adds a control to the NMOS device if it doesn't already exist
func (g *gatewayImpl) ensureControlExists(deviceID string, param string) {
	controls := g.nmosCtrl.GetControls(deviceID)
	for _, c := range controls {
		if c["parameter"] == param {
			return
		}
	}

	// Define standard control metadata for known parameters
	newControl := map[string]interface{}{
		"name":      param,
		"parameter": param,
		"type":      "string", // Default type
	}

	// Specialize known parameters
	switch param {
	case "AUDIO_GAIN":
		newControl["name"] = "Audio Gain"
		newControl["type"] = "number"
		newControl["min"], newControl["max"], newControl["step"] = -18, 42, 1
		newControl["unit"] = "dB"
	case "AUDIO_MUTE":
		newControl["name"] = "Audio Mute"
		newControl["type"] = "boolean"
	case "FLASH":
		newControl["name"] = "Identify (Flash)"
		newControl["type"] = "boolean"
	case "FREQUENCY":
		newControl["name"] = "Frequency"
		newControl["type"] = "number"
		newControl["unit"] = "kHz"
	case "CHAN_NAME":
		newControl["name"] = "Channel Name"
	case "METER_RATE":
		newControl["name"] = "Meter Rate"
		newControl["type"] = "number"
		newControl["unit"] = "ms"
	case "ENCRYPTION_MODE":
		newControl["name"] = "Encryption"
		newControl["type"] = "boolean"
	}

	controls = append(controls, newControl)
	g.nmosCtrl.SetControls(deviceID, controls)
	slog.Info("Dynamically assigned NMOS control", "deviceID", deviceID, "param", param)
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
