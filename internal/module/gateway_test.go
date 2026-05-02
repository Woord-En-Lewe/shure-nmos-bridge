package module

import (
	"context"
	"testing"

	"github.com/Woord-En-Lewe/shure-nmos-bridge/internal/infrastructure"
)

type mockNMOSController struct {
	infrastructure.NMOSController
	broadcastEvents []struct {
		sourceID  string
		flowID    string
		eventType string
		data      interface{}
	}
	ncpObjects map[int]infrastructure.NcObject
}

func (m *mockNMOSController) Start(ctx context.Context) error { return nil }
func (m *mockNMOSController) Stop(ctx context.Context) error  { return nil }
func (m *mockNMOSController) BroadcastEvent(sourceID string, flowID string, eventType string, data interface{}) {
	m.broadcastEvents = append(m.broadcastEvents, struct {
		sourceID  string
		flowID    string
		eventType string
		data      interface{}
	}{sourceID, flowID, eventType, data})
}
func (m *mockNMOSController) RegisterNCPObject(oid int, obj infrastructure.NcObject) {
	if m.ncpObjects == nil {
		m.ncpObjects = make(map[int]infrastructure.NcObject)
	}
	m.ncpObjects[oid] = obj
}
func (m *mockNMOSController) RegisterResource(resourceType string, resource interface{}) error {
	return nil
}
func (m *mockNMOSController) GetNCPObject(oid int) infrastructure.NcObject {
	return m.ncpObjects[oid]
}
func (m *mockNMOSController) UpdateResource(resourceType string, id string, updateFn func(interface{}) interface{}) error {
	return nil
}
func (m *mockNMOSController) UpdateResourceInRegistry(resourceType string, id string, updateFn func(interface{}) interface{}) error {
	return nil
}
func (m *mockNMOSController) GetControls(deviceID string) []map[string]interface{}           { return nil }
func (m *mockNMOSController) SetControls(deviceID string, controls []map[string]interface{}) {}
func (m *mockNMOSController) GetListenAddr() string { return "127.0.0.1:8080" }

type mockShureController struct {
	infrastructure.ShureController
}

func (m *mockShureController) Start(ctx context.Context) error       { return nil }
func (m *mockShureController) Stop(ctx context.Context) error        { return nil }
func (m *mockShureController) SendCommand(command interface{}) error { return nil }
func (m *mockShureController) ReceiveEvents() <-chan interface{}     { return nil }

func TestHandleShureDevice(t *testing.T) {
	mockNMOS := &mockNMOSController{}
	g := &gatewayImpl{
		nmosCtrl:   mockNMOS,
		shureCtrls: make(map[string]*shureDeviceInfo),
	}

	addr := "192.168.1.10:2202"
	sourceMap := map[string]string{
		"CHAN_QUALITY":     "source-quality",
		"AUDIO_LED_BITMAP": "source-led",
		"AUDIO_LEVEL_PEAK": "source-peak",
		"AUDIO_LEVEL_RMS":  "source-rms",
		"ANTENNA_STATUS":   "source-ant",
		"ANTENNA_A_ACTIVE": "source-ant-a",
		"ANTENNA_B_ACTIVE": "source-ant-b",
		"RF_LED_BITMAP_A":  "source-rf-led-a",
		"RF_RSSI_A":        "source-rssi-a",
		"RF_LED_BITMAP_B":  "source-rf-led-b",
		"RF_RSSI_B":        "source-rssi-b",
		"RF_LED_BITMAP_C":  "source-rf-led-c",
		"RF_RSSI_C":        "source-rssi-c",
		"RF_LED_BITMAP_D":  "source-rf-led-d",
		"RF_RSSI_D":        "source-rssi-d",
		"RF_LED_BITMAP_F1": "source-rf-led-f1",
		"RF_RSSI_F1":       "source-rssi-f1",
		"RF_LED_BITMAP_F2": "source-rf-led-f2",
		"RF_RSSI_F2":       "source-rssi-f2",
	}

	flowMap := map[string]string{
		"CHAN_QUALITY":     "flow-quality",
		"AUDIO_LED_BITMAP": "flow-led",
		"AUDIO_LEVEL_PEAK": "flow-peak",
		"AUDIO_LEVEL_RMS":  "flow-rms",
		"ANTENNA_STATUS":   "flow-ant",
		"ANTENNA_A_ACTIVE": "flow-ant-a",
		"ANTENNA_B_ACTIVE": "flow-ant-b",
		"RF_LED_BITMAP_A":  "flow-rf-led-a",
		"RF_RSSI_A":        "flow-rssi-a",
		"RF_LED_BITMAP_B":  "flow-rf-led-b",
		"RF_RSSI_B":        "flow-rssi-b",
		"RF_LED_BITMAP_C":  "flow-rf-led-c",
		"RF_RSSI_C":        "flow-rssi-c",
		"RF_LED_BITMAP_D":  "flow-rf-led-d",
		"RF_RSSI_D":        "flow-rssi-d",
		"RF_LED_BITMAP_F1": "flow-rf-led-f1",
		"RF_RSSI_F1":       "flow-rssi-f1",
		"RF_LED_BITMAP_F2": "flow-rf-led-f2",
		"RF_RSSI_F2":       "flow-rssi-f2",
	}

	g.shureCtrls[addr] = &shureDeviceInfo{
		ctrl:          &mockShureController{},
		modelFamily:   infrastructure.ModelFamilyAxientDigital,
		nmosDeviceIDs: map[int]string{1: "device-1"},
		sourceIDs:     map[int]map[string]string{1: sourceMap},
		flowIDs:       map[int]map[string]string{1: flowMap},
		senderIDs:     map[int]map[string]string{1: make(map[string]string)},
		parameterOIDs: make(map[string]int),
	}

	t.Run("SAMPLE ALL routing", func(t *testing.T) {
		mockNMOS.broadcastEvents = nil
		report := &infrastructure.TPCIReport{
			Type:    "SAMPLE",
			Channel: 1,
			Param:   "ALL",
			Value:   "005 000 045 062 BB 31 099 31 085",
			Raw:     "< SAMPLE 1 ALL 005 000 045 062 BB 31 099 31 085 >",
		}
		g.handleShureDevice(infrastructure.Message{
			Source:  addr,
			Payload: report,
		})

		// Axient SAMPLE ALL broadcasts: CHAN_QUALITY, AUDIO_LED_BITMAP, AUDIO_LEVEL_PEAK,
		// AUDIO_LEVEL_RMS, ANTENNA_A_ACTIVE, ANTENNA_B_ACTIVE, ANTENNA_STATUS, RF_LED_BITMAP_A,
		// RF_RSSI_A, RF_LED_BITMAP_B, RF_RSSI_B, RF_LED_BITMAP_C, RF_RSSI_C (13 events)
		if len(mockNMOS.broadcastEvents) != 13 {
			t.Errorf("Expected 13 broadcast events, got %d", len(mockNMOS.broadcastEvents))
		}
	})

	t.Run("Metered parameter in REP routing - broadcasts if pre-registered", func(t *testing.T) {
		mockNMOS.broadcastEvents = nil
		report := &infrastructure.TPCIReport{
			Type:    "REP",
			Channel: 1,
			Param:   "CHAN_QUALITY",
			Value:   "005",
			Raw:     "< REP 1 CHAN_QUALITY 005 >",
		}
		g.handleShureDevice(infrastructure.Message{
			Source:  addr,
			Payload: report,
		})

		// REP metered parameters only broadcast if IS-07 resources are already registered
		// (IS-07 resources are now registered eagerly when device connects)
		if len(mockNMOS.broadcastEvents) != 1 {
			t.Errorf("Expected 1 broadcast event (CHAN_QUALITY was pre-registered), got %d", len(mockNMOS.broadcastEvents))
		}
	})

	t.Run("Non-metered parameter in REP routing to NCP", func(t *testing.T) {
		mockNMOS.broadcastEvents = nil
		mockNMOS.ncpObjects = make(map[int]infrastructure.NcObject)

		chanNameOID := 102
		g.shureCtrls[addr].parameterOIDs["1_CHAN_NAME"] = chanNameOID
		chanNameWorker := infrastructure.NewNcWorker(chanNameOID, []int{1, 2, 1, 12}, nil, "ChannelName", "Ch1 ChannelName")
		mockNMOS.ncpObjects[chanNameOID] = chanNameWorker

		report := &infrastructure.TPCIReport{
			Type:    "REP",
			Channel: 1,
			Param:   "CHAN_NAME",
			Value:   "Lead Vox",
			Raw:     "< REP 1 CHAN_NAME {Lead Vox} >",
		}
		g.handleShureDevice(infrastructure.Message{
			Source:  addr,
			Payload: report,
		})

		// Should NOT be in IS-07
		if len(mockNMOS.broadcastEvents) != 0 {
			t.Errorf("Expected 0 broadcast events, got %d", len(mockNMOS.broadcastEvents))
		}

		// Should be in NCP (worker should be updated)
		if len(mockNMOS.ncpObjects) != 1 {
			t.Errorf("Expected 1 NCP object, got %d", len(mockNMOS.ncpObjects))
		}

		// Get the object and check its value
		worker := mockNMOS.ncpObjects[chanNameOID].(*infrastructure.NcWorker)
		if worker == nil {
			t.Fatal("NCP object is not an NcWorker")
		}
		if worker.Value != "Lead Vox" {
			t.Errorf("Expected NCP worker value 'Lead Vox', got '%v'", worker.Value)
		}
	})
}
