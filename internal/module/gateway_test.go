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
func (m *mockNMOSController) GetNCPObject(oid int) infrastructure.NcObject {
	return m.ncpObjects[oid]
}
func (m *mockNMOSController) UpdateResource(resourceType string, id string, updateFn func(interface{}) interface{}) error {
	// Minimal implementation for test
	return nil
}
func (m *mockNMOSController) GetControls(deviceID string) []map[string]interface{} { return nil }
func (m *mockNMOSController) SetControls(deviceID string, controls []map[string]interface{}) {}

type mockShureController struct {
	infrastructure.ShureController
}

func (m *mockShureController) Start(ctx context.Context) error { return nil }
func (m *mockShureController) Stop(ctx context.Context) error  { return nil }
func (m *mockShureController) SendCommand(command interface{}) error { return nil }
func (m *mockShureController) ReceiveEvents() <-chan interface{} { return nil }

func TestHandleShureDevice(t *testing.T) {
	mockNMOS := &mockNMOSController{}
	g := &gatewayImpl{
		nmosCtrl:   mockNMOS,
		shureCtrls: make(map[string]*shureDeviceInfo),
	}

	addr := "192.168.1.10:2202"
	sourceMap := make(map[string]string)
	sourceMap["CHAN_QUALITY"] = "source-quality"
	sourceMap["AUDIO_LED_BITMAP"] = "source-led"
	sourceMap["AUDIO_LEVEL_PEAK"] = "source-peak"
	sourceMap["AUDIO_LEVEL_RMS"] = "source-rms"
	sourceMap["ANTENNA_STATUS"] = "source-ant"
	sourceMap["RF_LED_BITMAP_A"] = "source-rf-led-a"
	sourceMap["RF_RSSI_A"] = "source-rssi-a"
	sourceMap["RF_LED_BITMAP_B"] = "source-rf-led-b"
	sourceMap["RF_RSSI_B"] = "source-rssi-b"
	// ... add others as needed for test

	flowMap := make(map[string]string)
	flowMap["CHAN_QUALITY"] = "flow-quality"
	flowMap["AUDIO_LED_BITMAP"] = "flow-led"
	flowMap["AUDIO_LEVEL_PEAK"] = "flow-peak"
	flowMap["AUDIO_LEVEL_RMS"] = "flow-rms"
	flowMap["ANTENNA_STATUS"] = "flow-ant"
	flowMap["RF_LED_BITMAP_A"] = "flow-rf-led-a"
	flowMap["RF_RSSI_A"] = "flow-rssi-a"
	flowMap["RF_LED_BITMAP_B"] = "flow-rf-led-b"
	flowMap["RF_RSSI_B"] = "flow-rssi-b"

	g.shureCtrls[addr] = &shureDeviceInfo{
		ctrl:          &mockShureController{},
		nmosDeviceIDs: map[int]string{1: "device-1"},
		sourceIDs:     map[int]map[string]string{1: sourceMap},
		flowIDs:       map[int]map[string]string{1: flowMap},
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

		if len(mockNMOS.broadcastEvents) != 9 {
			t.Errorf("Expected 9 broadcast events, got %d", len(mockNMOS.broadcastEvents))
		}
	})

	t.Run("Metered parameter in REP routing", func(t *testing.T) {
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

		if len(mockNMOS.broadcastEvents) != 1 {
			t.Errorf("Expected 1 broadcast event, got %d", len(mockNMOS.broadcastEvents))
		}
		if mockNMOS.broadcastEvents[0].eventType != "number" {
			t.Errorf("Expected event type 'number', got '%s'", mockNMOS.broadcastEvents[0].eventType)
		}
	})

	t.Run("Non-metered parameter in REP routing to NCP", func(t *testing.T) {
		mockNMOS.broadcastEvents = nil
		mockNMOS.ncpObjects = make(map[int]infrastructure.NcObject)
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

		// Should be in NCP
		if len(mockNMOS.ncpObjects) != 1 {
			t.Errorf("Expected 1 NCP object, got %d", len(mockNMOS.ncpObjects))
		}
		
		// Get the object and check its value
		var worker *infrastructure.NcWorker
		for _, obj := range mockNMOS.ncpObjects {
			if w, ok := obj.(*infrastructure.NcWorker); ok {
				worker = w
				break
			}
		}
		
		if worker == nil {
			t.Fatal("NCP object is not an NcWorker")
		}
		if worker.Value != "Lead Vox" {
			t.Errorf("Expected NCP worker value 'Lead Vox', got '%v'", worker.Value)
		}
	})
}
