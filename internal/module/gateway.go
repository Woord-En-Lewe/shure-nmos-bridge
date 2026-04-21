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
	deviceIndex    int                             // Sequential index for OID generation
	deviceOID      int                             // OID of the device block in NCP tree
	deviceInstance string                          // Device instance name (e.g. from mDNS discovery)
	parameterOIDs  map[string]int                  // param_key -> oid (e.g. "1_AUDIO_GAIN" -> 101)
	channelOIDs    map[int]int                     // channel -> OID of channel block
	sourceIDs      map[int]map[string]string       // channel -> param -> sourceID
	flowIDs        map[int]map[string]string       // channel -> param -> flowID
	senderIDs      map[int]map[string]string       // channel -> param -> senderID
}

// gatewayImpl is the concrete implementation of the Gateway interface
type gatewayImpl struct {
	shureAddr      string
	nmosAddr       string
	registryConfig infrastructure.RegistryDiscoveryConfig
	shureCtrls     map[string]*shureDeviceInfo
	nmosCtrl       infrastructure.NMOSController
	messageBus     infrastructure.MessageBus
	discoverer     *infrastructure.ShureDiscoverer
	mu             sync.RWMutex
}

// NewGateway creates a new Gateway instance
func NewGateway(shureAddr, nmosAddr string, registryConfig infrastructure.RegistryDiscoveryConfig) Gateway {
	return &gatewayImpl{
		shureAddr:      shureAddr,
		nmosAddr:       nmosAddr,
		registryConfig: registryConfig,
		shureCtrls:     make(map[string]*shureDeviceInfo),
	}
}

// Start initializes and starts the gateway components
func (g *gatewayImpl) Start(ctx context.Context) error {
	// Initialize infrastructure components
	g.messageBus = infrastructure.NewInMemoryMessageBus()
	if g.messageBus == nil {
		return fmt.Errorf("failed to create message bus")
	}

	g.nmosCtrl = infrastructure.NewNMOSControllerWithConfig(g.nmosAddr, g.registryConfig)
	if g.nmosCtrl == nil {
		return fmt.Errorf("failed to create nmos controller")
	}

	// Discover and set externally reachable address for NMOS hrefs
	if localIP := discoverLocalIP(); localIP != "" {
		g.nmosCtrl.SetAdvertisedAddr(localIP)
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

			// Filter out non-Shure devices (default to AxientDigital means unknown model)
			family := infrastructure.DetectModelFamily(dev.Instance)
			if family == infrastructure.ModelFamilyAxientDigital && !strings.HasPrefix(strings.ToUpper(dev.Instance), "AD") {
				slog.Debug("Ignoring non-Shure device", "address", dev.Address, "port", dev.Port, "instance", dev.Instance)
				continue
			}

			addr := fmt.Sprintf("%s:%d", dev.Address, dev.Port)
			g.addShureController(ctx, addr, dev)
		}
	}
}

// addShureController creates and starts a new Shure controller for an address
func (g *gatewayImpl) addShureController(ctx context.Context, addr string, dev infrastructure.DiscoveredDevice) {
	// Check if already exists (quick lock release)
	g.mu.Lock()
	if info, ok := g.shureCtrls[addr]; ok {
		info.lastSeen = time.Now() // Refresh heartbeat
		g.mu.Unlock()
		return
	}
	g.mu.Unlock()

	// Start TCP connection WITHOUT holding lock
	ctrl := infrastructure.NewShureController(addr)
	if err := ctrl.Start(ctx); err != nil {
		slog.Error("Failed to start Shure controller", "address", addr, "error", err)
		return
	}

	// Now add to map with lock held
	g.mu.Lock()
	defer g.mu.Unlock()

	// Double-check after acquiring lock
	if info, ok := g.shureCtrls[addr]; ok {
		info.lastSeen = time.Now()
		return
	}

	deviceID := uuid.New().String()
	deviceIndex := len(g.shureCtrls) + 1
	deviceOID := 100 + deviceIndex*10

	// Determine model family from discovery (mDNS broadcast contains model name)
	// This is secure - the model string comes from device firmware, not user-settable
	family := infrastructure.DetectModelFamily(dev.Instance)
	slog.Info("Model family from discovery", "address", addr, "instance", dev.Instance, "family", family)

	// Compute maxChannels early for NCP setup
	maxChannels := infrastructure.MaxChannelsFromModel(dev.Instance)

	g.shureCtrls[addr] = &shureDeviceInfo{
		ctrl:           ctrl,
		lastSeen:       time.Now(),
		modelFamily:    family, // Set from discovery, not user input
		nmosDeviceIDs:  map[int]string{0: deviceID},
		deviceIndex:    deviceIndex,
		deviceOID:      deviceOID,
		deviceInstance: dev.Instance,
		parameterOIDs:  make(map[string]int),
		channelOIDs:    make(map[int]int),
		sourceIDs:      make(map[int]map[string]string),
		flowIDs:        make(map[int]map[string]string),
		senderIDs:      make(map[int]map[string]string),
	}
	slog.Info("Connected to Shure device", "address", addr)

	// Setup NCP hierarchy BEFORE sending discovery commands
	// This ensures parameterOIDs map is populated before REP responses arrive
	info := g.shureCtrls[addr]
	g.setupDeviceNCP(addr, dev, deviceOID, family, maxChannels, ctrl, info)

	// Start event listener for this controller
	go g.listenToShureEvents(ctx, addr, ctrl.ReceiveEvents())

	// On connection, we send GET x ALL for each channel and SET x METER_RATE
	// Model family is already set from discovery (mDNS broadcast contains model name)

	// For multi-channel receivers (Axient Digital, ULXD4D, ULXD4Q), GET 0 ALL is efficient
	// Per Shure spec, channel "0" means "all channels"
	if maxChannels > 1 && (family == infrastructure.ModelFamilyAxientDigital || family == infrastructure.ModelFamilyULXD) {
		getAllCmd := infrastructure.NewShureCommand("GET").
			WithIndex(0).
			WithParam("ALL", nil).
			Build()
		slog.Info("Discovery GET 0 ALL", "address", addr, "cmd", getAllCmd)
		ctrl.SendCommand(getAllCmd)
		time.Sleep(100 * time.Millisecond)
	}

	// Query each channel individually to ensure full state
	for ch := 1; ch <= maxChannels; ch++ {
		cmd := infrastructure.NewShureCommand("GET").
			WithIndex(ch).
			WithParam("ALL", nil).
			Build()
		slog.Info("Discovery GET ALL", "address", addr, "channel", ch, "cmd", cmd)
		ctrl.SendCommand(cmd)
		time.Sleep(50 * time.Millisecond)
	}

	// Set METER_RATE to enable push-based metering (SAMPLE responses)
	// The device will automatically send SAMPLE responses at the configured interval
	for ch := 1; ch <= maxChannels; ch++ {
		meterCmd := fmt.Sprintf("< SET %d METER_RATE 01000 >\n", ch)
		slog.Info("Discovery SET METER_RATE", "address", addr, "channel", ch, "cmd", meterCmd)
		ctrl.SendCommand(meterCmd)
		time.Sleep(20 * time.Millisecond)
	}

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
				"href": fmt.Sprintf("http://%s/x-nmos/connection/v1.1/", g.nmosCtrl.GetListenAddr()),
			},
			map[string]interface{}{
				"type": "urn:x-nmos:control:events/v1.0",
				"href": fmt.Sprintf("http://%s/x-nmos/events/v1.0/", g.nmosCtrl.GetListenAddr()),
			},
			map[string]interface{}{
				"type": "urn:x-nmos:control:ncp/v1.0",
				"href": fmt.Sprintf("ws://%s/x-nmos/node/v1.3/ncp", g.nmosCtrl.GetListenAddr()),
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
}

// setupDeviceNCP sets up the NCP hierarchy for a device
	// This must be called BEFORE sending discovery commands to ensure parameterOIDs map is populated
func (g *gatewayImpl) setupDeviceNCP(addr string, dev infrastructure.DiscoveredDevice, deviceOID int, family infrastructure.ShureModelFamily, maxChannels int, ctrl infrastructure.ShureController, info *shureDeviceInfo) {
	// IS-12 NCP Setup
	// Register custom classes matching the class IDs used by CreateChannelWorkers
	g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
		Name:    "GainWorker",
		ClassID: []int{1, 2, 1, 10},
		Properties: []infrastructure.NcPropertyDescriptor{
			{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
			{Name: "gain", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcFloat32"},
		},
	})
	g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
		Name:    "MuteWorker",
		ClassID: []int{1, 2, 1, 11},
		Properties: []infrastructure.NcPropertyDescriptor{
			{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
			{Name: "mute", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcBoolean"},
		},
	})
	g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
		Name:    "ChannelNameWorker",
		ClassID: []int{1, 2, 1, 12},
		Properties: []infrastructure.NcPropertyDescriptor{
			{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
			{Name: "channelName", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcString"},
		},
	})
	g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
		Name:    "FrequencyWorker",
		ClassID: []int{1, 2, 1, 13},
		Properties: []infrastructure.NcPropertyDescriptor{
			{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
			{Name: "frequency", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcString"},
		},
	})
	g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
		Name:    "GroupChannelWorker",
		ClassID: []int{1, 2, 1, 14},
		Properties: []infrastructure.NcPropertyDescriptor{
			{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
			{Name: "groupChannel", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcString"},
		},
	})
	g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
		Name:    "TransmitterWorker",
		ClassID: []int{1, 2, 1, 15},
		Properties: []infrastructure.NcPropertyDescriptor{
			{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
			{Name: "transmitter", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcString"},
		},
	})
	g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
		Name:    "BatteryWorker",
		ClassID: []int{1, 2, 1, 16},
		Properties: []infrastructure.NcPropertyDescriptor{
			{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
			{Name: "battery", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcString"},
		},
	})
	g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
		Name:    "AudioLevelWorker",
		ClassID: []int{1, 2, 1, 17},
		Properties: []infrastructure.NcPropertyDescriptor{
			{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
			{Name: "audioLevel", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcString"},
		},
	})
	g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
		Name:    "RSSIWorker",
		ClassID: []int{1, 2, 1, 18},
		Properties: []infrastructure.NcPropertyDescriptor{
			{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
			{Name: "rssi", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcString"},
		},
	})

	// QLX-D specific classes for device-level workers
	if family == infrastructure.ModelFamilyQLXD {
		g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
			Name:    "FwVerWorker",
			ClassID: []int{1, 2, 1, 19},
			Properties: []infrastructure.NcPropertyDescriptor{
				{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
				{Name: "fwVer", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcString", IsReadOnly: true},
			},
		})
		g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
			Name:    "DeviceIDWorker",
			ClassID: []int{1, 2, 1, 20},
			Properties: []infrastructure.NcPropertyDescriptor{
				{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
				{Name: "deviceID", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcString"},
			},
		})
		g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
			Name:    "EncryptionWorker",
			ClassID: []int{1, 2, 1, 21},
			Properties: []infrastructure.NcPropertyDescriptor{
				{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
				{Name: "encryption", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcBoolean"},
			},
		})
		g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
			Name:    "MacAddrWorker",
			ClassID: []int{1, 2, 1, 22},
			Properties: []infrastructure.NcPropertyDescriptor{
				{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
				{Name: "macAddr", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcString", IsReadOnly: true},
			},
		})
		g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
			Name:    "BattCycleWorker",
			ClassID: []int{1, 2, 1, 23},
			Properties: []infrastructure.NcPropertyDescriptor{
				{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
				{Name: "battCycle", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcString", IsReadOnly: true},
			},
		})
		g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
			Name:    "BattRunTimeWorker",
			ClassID: []int{1, 2, 1, 24},
			Properties: []infrastructure.NcPropertyDescriptor{
				{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
				{Name: "battRunTime", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcString", IsReadOnly: true},
			},
		})
		g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
			Name:    "BattTempFWorker",
			ClassID: []int{1, 2, 1, 25},
			Properties: []infrastructure.NcPropertyDescriptor{
				{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
				{Name: "battTempF", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcString", IsReadOnly: true},
			},
		})
		g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
			Name:    "BattTempCWorker",
			ClassID: []int{1, 2, 1, 26},
			Properties: []infrastructure.NcPropertyDescriptor{
				{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
				{Name: "battTempC", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcString", IsReadOnly: true},
			},
		})
		g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
			Name:    "BattTypeWorker",
			ClassID: []int{1, 2, 1, 27},
			Properties: []infrastructure.NcPropertyDescriptor{
				{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
				{Name: "battType", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcString", IsReadOnly: true},
			},
		})
		g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
			Name:    "BattChargeWorker",
			ClassID: []int{1, 2, 1, 28},
			Properties: []infrastructure.NcPropertyDescriptor{
				{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
				{Name: "battCharge", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcString", IsReadOnly: true},
			},
		})
		g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
			Name:    "BattHealthWorker",
			ClassID: []int{1, 2, 1, 29},
			Properties: []infrastructure.NcPropertyDescriptor{
				{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
				{Name: "battHealth", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcString", IsReadOnly: true},
			},
		})
		g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
			Name:    "BattBarsWorker",
			ClassID: []int{1, 2, 1, 30},
			Properties: []infrastructure.NcPropertyDescriptor{
				{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
				{Name: "battBars", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcString", IsReadOnly: true},
			},
		})
		g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
			Name:    "TxTypeWorker",
			ClassID: []int{1, 2, 1, 31},
			Properties: []infrastructure.NcPropertyDescriptor{
				{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
				{Name: "txType", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcString", IsReadOnly: true},
			},
		})
		g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
			Name:    "TxOffsetWorker",
			ClassID: []int{1, 2, 1, 32},
			Properties: []infrastructure.NcPropertyDescriptor{
				{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
				{Name: "txOffset", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcString", IsReadOnly: true},
			},
		})
		g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
			Name:    "TxRFPowerWorker",
			ClassID: []int{1, 2, 1, 33},
			Properties: []infrastructure.NcPropertyDescriptor{
				{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
				{Name: "txRFPower", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcString", IsReadOnly: true},
			},
		})
		g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
			Name:    "TxPwrLockWorker",
			ClassID: []int{1, 2, 1, 34},
			Properties: []infrastructure.NcPropertyDescriptor{
				{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
				{Name: "txPwrLock", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcString", IsReadOnly: true},
			},
		})
		g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
			Name:    "TxMenuLockWorker",
			ClassID: []int{1, 2, 1, 35},
			Properties: []infrastructure.NcPropertyDescriptor{
				{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
				{Name: "txMenuLock", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcString", IsReadOnly: true},
			},
		})
		g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
			Name:    "TxDeviceIDWorker",
			ClassID: []int{1, 2, 1, 36},
			Properties: []infrastructure.NcPropertyDescriptor{
				{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
				{Name: "txDeviceID", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcString", IsReadOnly: true},
			},
		})
		g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
			Name:    "TxMuteStatusWorker",
			ClassID: []int{1, 2, 1, 37},
			Properties: []infrastructure.NcPropertyDescriptor{
				{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
				{Name: "txMuteStatus", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcString", IsReadOnly: true},
			},
		})
		g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
			Name:    "TxMuteButtonStatusWorker",
			ClassID: []int{1, 2, 1, 38},
			Properties: []infrastructure.NcPropertyDescriptor{
				{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
				{Name: "txMuteButtonStatus", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcString", IsReadOnly: true},
			},
		})
		g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
			Name:    "TxPowerSourceWorker",
			ClassID: []int{1, 2, 1, 39},
			Properties: []infrastructure.NcPropertyDescriptor{
				{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
				{Name: "txPowerSource", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcString", IsReadOnly: true},
			},
		})
		g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
			Name:    "EncryptionWarningWorker",
			ClassID: []int{1, 2, 1, 40},
			Properties: []infrastructure.NcPropertyDescriptor{
				{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
				{Name: "encryptionWarning", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
			},
		})
		g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
			Name:    "AntennaStatusWorker",
			ClassID: []int{1, 2, 1, 41},
			Properties: []infrastructure.NcPropertyDescriptor{
				{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
				{Name: "antennaStatus", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcString", IsReadOnly: true},
			},
		})
		g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
			Name:    "RFLevelWorker",
			ClassID: []int{1, 2, 1, 42},
			Properties: []infrastructure.NcPropertyDescriptor{
				{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
				{Name: "rfLevel", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcString", IsReadOnly: true},
			},
		})
		g.nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
			Name:    "AudioLevelMeterWorker",
			ClassID: []int{1, 2, 1, 43},
			Properties: []infrastructure.NcPropertyDescriptor{
				{Name: "enabled", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcBoolean", IsReadOnly: true},
				{Name: "audioLevel", ID: infrastructure.NCPPropertyID{Level: 3, Index: 1}, TypeName: "NcString", IsReadOnly: true},
			},
		})
	}

	// Use a simple OID allocation (In a real app, this should be more robust)
	devBlock := infrastructure.NewNcBlock(deviceOID, nil, "Device", dev.Instance)
	g.nmosCtrl.RegisterNCPObject(deviceOID, devBlock)

	// QLX-D has a different hierarchy structure
	if family == infrastructure.ModelFamilyQLXD {
		// Create device-level workers (FW_VER, DEVICE_ID, ENCRYPTION, MAC_ADDR)
		baseDeviceOID := deviceOID * 100
		deviceWorkers, deviceParamOIDMap := infrastructure.CreateDeviceWorkers(family, baseDeviceOID, ctrl)
		for paramKey, oid := range deviceParamOIDMap {
			info.parameterOIDs[paramKey] = oid
		}
		for _, worker := range deviceWorkers {
			g.nmosCtrl.RegisterNCPObject(worker.GetOID(), worker)
			devBlock.AddItem(worker.GetOID())
		}

		// Create channel blocks under the device block
		for ch := 1; ch <= maxChannels; ch++ {
			channelOID := deviceOID + ch
			info.channelOIDs[ch] = channelOID
			channelBlock := infrastructure.NewNcBlock(channelOID, nil, fmt.Sprintf("Channel%d", ch), fmt.Sprintf("Channel %d", ch))
			g.nmosCtrl.RegisterNCPObject(channelOID, channelBlock)
			devBlock.AddItem(channelOID)

			// Create channel-level workers (CHAN_NAME, FREQUENCY, GROUP_CHAN, ENCRYPTION_WARNING)
			baseOID := channelOID * 100
			workers, paramOIDMap := infrastructure.CreateChannelWorkers(family, ch, baseOID, ctrl)
			for paramKey, oid := range paramOIDMap {
				info.parameterOIDs[paramKey] = oid
			}
			for _, worker := range workers {
				g.nmosCtrl.RegisterNCPObject(worker.GetOID(), worker)
				channelBlock.AddItem(worker.GetOID())
			}

			// Create nested sub-blocks: AudioGain, Battery, TX
			subBlockBase := channelOID * 1000

			// AudioGain block with SetAudioGain, INC_AUDIO_GAIN, DEC_AUDIO_GAIN
			audioGainBlock := infrastructure.NewNcBlock(subBlockBase+1, nil, "AudioGain", fmt.Sprintf("Ch%d AudioGain", ch))
			g.nmosCtrl.RegisterNCPObject(audioGainBlock.GetOID(), audioGainBlock)
			channelBlock.AddItem(audioGainBlock.GetOID())

			setAudioGainWorker := infrastructure.NewNcWorker(subBlockBase+10, []int{1, 2, 1, 10}, nil, "SetAudioGain", fmt.Sprintf("Ch%d SetAudioGain", ch))
			setAudioGainWorker.OnSet = func(val interface{}) error {
				cmd := fmt.Sprintf("< SET %d AUDIO_GAIN %v >\n", ch, val)
				return ctrl.SendCommand(cmd)
			}
			g.nmosCtrl.RegisterNCPObject(setAudioGainWorker.GetOID(), setAudioGainWorker)
			audioGainBlock.AddItem(setAudioGainWorker.GetOID())
			info.parameterOIDs[fmt.Sprintf("%d_%s", ch, "AUDIO_GAIN")] = setAudioGainWorker.GetOID()

			incAudioGainWorker := infrastructure.NewNcWorker(subBlockBase+11, []int{1, 2, 1, 10}, nil, "IncAudioGain", fmt.Sprintf("Ch%d IncAudioGain", ch))
			incAudioGainWorker.OnSet = func(val interface{}) error {
				cmd := fmt.Sprintf("< SET %d AUDIO_GAIN INC %v >\n", ch, val)
				return ctrl.SendCommand(cmd)
			}
			g.nmosCtrl.RegisterNCPObject(incAudioGainWorker.GetOID(), incAudioGainWorker)
			audioGainBlock.AddItem(incAudioGainWorker.GetOID())

			decAudioGainWorker := infrastructure.NewNcWorker(subBlockBase+12, []int{1, 2, 1, 10}, nil, "DecAudioGain", fmt.Sprintf("Ch%d DecAudioGain", ch))
			decAudioGainWorker.OnSet = func(val interface{}) error {
				cmd := fmt.Sprintf("< SET %d AUDIO_GAIN DEC %v >\n", ch, val)
				return ctrl.SendCommand(cmd)
			}
			g.nmosCtrl.RegisterNCPObject(decAudioGainWorker.GetOID(), decAudioGainWorker)
			audioGainBlock.AddItem(decAudioGainWorker.GetOID())

			// Battery block
			batteryBlock := infrastructure.NewNcBlock(subBlockBase+2, nil, "Battery", fmt.Sprintf("Ch%d Battery", ch))
			g.nmosCtrl.RegisterNCPObject(batteryBlock.GetOID(), batteryBlock)
			channelBlock.AddItem(batteryBlock.GetOID())

			batteryWorkers, batteryParamMap := infrastructure.CreateChannelSubWorkers(family, ch, subBlockBase+20, ctrl)
			for _, workers := range batteryWorkers["Battery"] {
				g.nmosCtrl.RegisterNCPObject(workers.GetOID(), workers)
				batteryBlock.AddItem(workers.GetOID())
			}
			for param, oid := range batteryParamMap["Battery"] {
				info.parameterOIDs[fmt.Sprintf("%d_%s", ch, param)] = oid
			}

			// TX block
			txBlock := infrastructure.NewNcBlock(subBlockBase+3, nil, "Transmitter", fmt.Sprintf("Ch%d Transmitter", ch))
			g.nmosCtrl.RegisterNCPObject(txBlock.GetOID(), txBlock)
			channelBlock.AddItem(txBlock.GetOID())

			txWorkers, txParamMap := infrastructure.CreateChannelSubWorkers(family, ch, subBlockBase+30, ctrl)
			for _, workers := range txWorkers["Transmitter"] {
				g.nmosCtrl.RegisterNCPObject(workers.GetOID(), workers)
				txBlock.AddItem(workers.GetOID())
			}
			for param, oid := range txParamMap["Transmitter"] {
				info.parameterOIDs[fmt.Sprintf("%d_%s", ch, param)] = oid
			}
		}
	} else {
		// Original behavior for other model families
		for ch := 1; ch <= maxChannels; ch++ {
			channelOID := deviceOID + ch
			info.channelOIDs[ch] = channelOID
			channelBlock := infrastructure.NewNcBlock(channelOID, nil, fmt.Sprintf("Channel%d", ch), fmt.Sprintf("Channel %d", ch))
			g.nmosCtrl.RegisterNCPObject(channelOID, channelBlock)
			devBlock.AddItem(channelOID)

			baseOID := channelOID * 100
			workers, paramOIDMap := infrastructure.CreateChannelWorkers(family, ch, baseOID, ctrl)
			for paramKey, oid := range paramOIDMap {
				info.parameterOIDs[paramKey] = oid
			}
			for _, worker := range workers {
				g.nmosCtrl.RegisterNCPObject(worker.GetOID(), worker)
				channelBlock.AddItem(worker.GetOID())
			}
		}
	}

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
				// Log SAMPLE responses at Debug level to reduce log spam during normal operation
				slog.Debug("Shure REP Received", "address", addr, "type", report.Type, "channel", report.Channel, "param", report.Param, "value", report.Value)

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

	// Close message bus FIRST to unblock processMessages and listenToShureEvents
	// This breaks the deadlock chain where g.mu blocks processMessages which blocks
	// listenToShureEvents which blocks readEvents which blocks ShureController.Stop
	slog.Info("Stopping message bus")
	if mb, ok := g.messageBus.(*infrastructure.InMemoryMessageBus); ok {
		mb.Close()
	}
	slog.Info("Message bus stopped")

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
		"tags": map[string]interface{}{
			"ext_is_07_source_id":    []string{sourceID},
			"ext_is_07_rest_api_url": []string{fmt.Sprintf("http://%s/x-nmos/events/v1.0/sources/%s/", g.nmosCtrl.GetListenAddr(), sourceID)},
		},
		"transport_params": []map[string]interface{}{
			{
				"ext_is_07_rest_api_url": fmt.Sprintf("http://%s/x-nmos/events/v1.0/sources/%s/", g.nmosCtrl.GetListenAddr(), sourceID),
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
				aActive := sample.AntStatus == infrastructure.AntennaAOn
				bActive := sample.AntStatus == infrastructure.AntennaBOn
				if sID, ok := info.sourceIDs[report.Channel]["ANTENNA_A_ACTIVE"]; ok {
					g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["ANTENNA_A_ACTIVE"], "boolean", aActive)
				}
				if sID, ok := info.sourceIDs[report.Channel]["ANTENNA_B_ACTIVE"]; ok {
					g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[report.Channel]["ANTENNA_B_ACTIVE"], "boolean", bActive)
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

		// Also update the NCP worker for this metered param
		paramKey := fmt.Sprintf("%d_%s", report.Channel, report.Param)
		g.mu.Lock()
		oid, exists := info.parameterOIDs[paramKey]
		g.mu.Unlock()
		if exists {
			if obj := g.nmosCtrl.GetNCPObject(oid); obj != nil {
				if worker, ok := obj.(*infrastructure.NcWorker); ok {
					worker.SetProperty(infrastructure.NCPPropertyID{Level: 3, Index: 1}, report.Value)
				}
			}
		}
	}

	// SAMPLE ALL messages also update metered worker values
	if report.Type == "SAMPLE" && report.Param == "ALL" {
		g.updateMeteredWorkersFromSample(info, report)
	}

	// IS-12 NCP Parameter Updates
	// REP messages update existing workers that were pre-created via CreateChannelWorkers
	// Dynamic worker creation is disabled - all workers are pre-created based on model family
	if report.Type == "REP" && report.Param != "ALL" {
		paramKey := fmt.Sprintf("%d_%s", report.Channel, report.Param)
		normalizedParam := strings.ReplaceAll(report.Param, " ", "")

		g.mu.Lock()
		oid, exists := info.parameterOIDs[paramKey]
		g.mu.Unlock()

		// Also try normalized param key (with spaces removed) for QLXD compatibility
		if !exists {
			paramKey = fmt.Sprintf("%d_%s", report.Channel, normalizedParam)
			g.mu.Lock()
			oid, exists = info.parameterOIDs[paramKey]
			g.mu.Unlock()
		}

		// For QLX-D, also try device-level param lookup (no channel prefix)
		// Device-level params: FW_VER, DEVICE_ID, ENCRYPTION, MAC_ADDR
		if !exists && info.modelFamily == infrastructure.ModelFamilyQLXD {
			g.mu.Lock()
			oid, exists = info.parameterOIDs[report.Param]
			g.mu.Unlock()
			if !exists {
				// Try normalized without channel
				g.mu.Lock()
				oid, exists = info.parameterOIDs[normalizedParam]
				g.mu.Unlock()
			}
		}

		if !exists {
			slog.Debug("REP param not found in parameterOIDs",
				"channel", report.Channel,
				"param", report.Param,
				"deviceOID", info.deviceOID,
				"family", info.modelFamily)
		}

		// Update existing worker value only (no dynamic worker creation)
		if exists {
			if obj := g.nmosCtrl.GetNCPObject(oid); obj != nil {
				if worker, ok := obj.(*infrastructure.NcWorker); ok {
					worker.SetProperty(infrastructure.NCPPropertyID{Level: 3, Index: 1}, report.Value)
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
				controlType := "number"
				switch report.Value {
				case "ON", "OFF":
					controlType = "boolean"
				default:
					if strings.HasPrefix(report.Value, "{") {
						controlType = "string"
					}
				}

				newControl := map[string]interface{}{
					"name":  report.Param,
					"type":  controlType,
					"value": report.Value,
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

	g.ensureIS07Resources(info, channel, "ANTENNA_A_ACTIVE", deviceID, deviceInstance)
	if sID, ok := info.sourceIDs[channel]["ANTENNA_A_ACTIVE"]; ok {
		g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[channel]["ANTENNA_A_ACTIVE"], "boolean", sample.AntennaAActive())
	}

	g.ensureIS07Resources(info, channel, "ANTENNA_B_ACTIVE", deviceID, deviceInstance)
	if sID, ok := info.sourceIDs[channel]["ANTENNA_B_ACTIVE"]; ok {
		g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[channel]["ANTENNA_B_ACTIVE"], "boolean", sample.AntennaBActive())
	}

	if sample.RFBitmapC > 0 || sample.RFRSSI_C > 0 {
		g.ensureIS07Resources(info, channel, "ANTENNA_C_ACTIVE", deviceID, deviceInstance)
		if sID, ok := info.sourceIDs[channel]["ANTENNA_C_ACTIVE"]; ok {
			g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[channel]["ANTENNA_C_ACTIVE"], "boolean", sample.AntennaCActive())
		}
	}

	if sample.RFBitmapD > 0 || sample.RFRSSI_D > 0 {
		g.ensureIS07Resources(info, channel, "ANTENNA_D_ACTIVE", deviceID, deviceInstance)
		if sID, ok := info.sourceIDs[channel]["ANTENNA_D_ACTIVE"]; ok {
			g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[channel]["ANTENNA_D_ACTIVE"], "boolean", sample.AntennaDActive())
		}
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

	aActive := sample.AntStatus == infrastructure.AntennaAOn
	bActive := sample.AntStatus == infrastructure.AntennaBOn

	g.ensureIS07Resources(info, channel, "ANTENNA_A_ACTIVE", deviceID, deviceInstance)
	if sID, ok := info.sourceIDs[channel]["ANTENNA_A_ACTIVE"]; ok {
		g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[channel]["ANTENNA_A_ACTIVE"], "boolean", aActive)
	}

	g.ensureIS07Resources(info, channel, "ANTENNA_B_ACTIVE", deviceID, deviceInstance)
	if sID, ok := info.sourceIDs[channel]["ANTENNA_B_ACTIVE"]; ok {
		g.nmosCtrl.BroadcastEvent(sID, info.flowIDs[channel]["ANTENNA_B_ACTIVE"], "boolean", bActive)
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

// updateMeteredWorkersFromSample updates NCP worker values from SAMPLE data
func (g *gatewayImpl) updateMeteredWorkersFromSample(info *shureDeviceInfo, report *infrastructure.TPCIReport) {
	switch info.modelFamily {
	case infrastructure.ModelFamilyAxientDigital:
		if sample := infrastructure.ParseSampleReport(report.Raw); sample != nil {
			channel := report.Channel
			g.updateWorkerValue(info, channel, "AUDIO_LEVEL_PEAK", sample.AudioLevelPeakDBFS())
			g.updateWorkerValue(info, channel, "AUDIO_LEVEL_RMS", sample.AudioLevelRMSDBFS())
			g.updateWorkerValue(info, channel, "RF_RSSI_A", sample.RFRSSI_A_DBM())
			g.updateWorkerValue(info, channel, "RF_RSSI_B", sample.RFRSSI_B_DBM())
		}
	case infrastructure.ModelFamilyULXD, infrastructure.ModelFamilyQLXD:
		if sample := infrastructure.ParseULXDSampleReport(report.Raw); sample != nil {
			channel := report.Channel
			g.updateWorkerValue(info, channel, "ANT_STATUS", string(sample.AntStatus))
			g.updateWorkerValue(info, channel, "RF_LEVEL", sample.RFLevelDBM())
			g.updateWorkerValue(info, channel, "AUDIO_LEVEL", sample.AudioLevelDBFS())
		}
	case infrastructure.ModelFamilySLXD, infrastructure.ModelFamilySLXDPlus:
		if sample := infrastructure.ParseSLDXSampleReport(report.Raw); sample != nil {
			channel := report.Channel
			g.updateWorkerValue(info, channel, "AUDIO_LEVEL_PEAK", sample.AudioPeakDBFS())
			g.updateWorkerValue(info, channel, "AUDIO_LEVEL_RMS", sample.AudioRMSDBFS())
			g.updateWorkerValue(info, channel, "RF_RSSI", sample.RFRSSIDBM())
		}
	}
}

// updateWorkerValue updates a worker's value if it exists
func (g *gatewayImpl) updateWorkerValue(info *shureDeviceInfo, channel int, param string, value interface{}) {
	paramKey := fmt.Sprintf("%d_%s", channel, param)
	g.mu.Lock()
	oid, exists := info.parameterOIDs[paramKey]
	g.mu.Unlock()
	if !exists {
		return
	}
	if obj := g.nmosCtrl.GetNCPObject(oid); obj != nil {
		if worker, ok := obj.(*infrastructure.NcWorker); ok {
			worker.SetProperty(infrastructure.NCPPropertyID{Level: 3, Index: 1}, value)
		}
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
			// Collect stale devices first
			var staleAddrs []string
			g.mu.RLock()
			for addr, info := range g.shureCtrls {
				if time.Since(info.lastSeen) > 2*time.Minute {
					staleAddrs = append(staleAddrs, addr)
				}
			}
			g.mu.RUnlock()

			// Stop devices outside the lock to prevent deadlock
			for _, addr := range staleAddrs {
				g.mu.Lock()
				info, ok := g.shureCtrls[addr]
				g.mu.Unlock()
				if ok {
					slog.Info("Removing stale Shure device", "address", addr)
					info.ctrl.Stop(ctx)
					g.mu.Lock()
					delete(g.shureCtrls, addr)
					g.mu.Unlock()
				}
			}
		}
	}
}

// discoverLocalIP returns the first non-loopback IPv4 address found on the machine
func discoverLocalIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ip4 := addr.(*net.IPNet).IP.To4(); ip4 != nil && !ip4.IsLoopback() {
				return ip4.String()
			}
		}
	}
	return ""
}
