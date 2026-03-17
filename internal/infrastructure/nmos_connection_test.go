package infrastructure

import (
	"bytes"

	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleConnectionStagedPatch(t *testing.T) {
	ctrl := NewNMOSController("localhost:8080").(*nmosController)
	senderID := "test-sender-id"

	// Register a dummy sender so the handler finds it
	ctrl.resources["senders"] = append(ctrl.resources["senders"], map[string]interface{}{
		"id": senderID,
	})

	// 1. Initial GET /staged
	path := fmt.Sprintf("/x-nmos/connection/v1.1/single/senders/%s/staged", senderID)
	req := httptest.NewRequest("GET", path, nil)
	w := httptest.NewRecorder()
	ctrl.handleConnectionSenders(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %d", w.Code)
	}

	var staged ConnectionStaged
	if err := json.NewDecoder(w.Body).Decode(&staged); err != nil {
		t.Fatal(err)
	}

	// 2. PATCH /staged
	patchBody := ConnectionStaged{
		MasterEnable: false,
		Activation: ConnectionActivation{
			Mode: ActivationModeImmediate,
		},
		TransportParams: []map[string]interface{}{
			{"destination_ip": "10.0.0.1"},
		},
	}
	body, _ := json.Marshal(patchBody)
	req = httptest.NewRequest("PATCH", path, bytes.NewReader(body))
	w = httptest.NewRecorder()
	ctrl.handleConnectionSenders(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status OK for PATCH, got %d", w.Code)
	}

	// 3. Verify Active state after immediate activation
	activePath := fmt.Sprintf("/x-nmos/connection/v1.1/single/senders/%s/active", senderID)
	req = httptest.NewRequest("GET", activePath, nil)
	w = httptest.NewRecorder()
	ctrl.handleConnectionSenders(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status OK for active, got %d", w.Code)
	}

	var active ConnectionActive
	if err := json.NewDecoder(w.Body).Decode(&active); err != nil {
		t.Fatal(err)
	}

	if active.MasterEnable != false {
		t.Errorf("Expected master_enable false, got %v", active.MasterEnable)
	}

	if len(active.TransportParams) == 0 || active.TransportParams[0]["destination_ip"] != "10.0.0.1" {
		t.Errorf("Expected destination_ip 10.0.0.1, got %v", active.TransportParams)
	}

	if active.Activation.ActivationTime == nil {
		t.Error("Expected activation_time to be set")
	}
}
