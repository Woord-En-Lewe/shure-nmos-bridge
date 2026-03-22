package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestHandleConnectionStagedPatch(t *testing.T) {
	ctrl := NewNMOSController("localhost:8080").(*nmosController)
	senderID := "test-sender-id"

	ctrl.resources["senders"] = append(ctrl.resources["senders"], map[string]interface{}{
		"id": senderID,
	})

	t.Run("GET /staged", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/x-nmos/connection/v1.1/single/senders/"+senderID+"/staged", nil)
		w := httptest.NewRecorder()
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("senderId", senderID)
		req = req.WithContext(contextWithRouteContext(rctx))

		ctrl.handleConnectionSenderStaged(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %d", w.Code)
		}

		var staged ConnectionStaged
		if err := json.NewDecoder(w.Body).Decode(&staged); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("PATCH /staged", func(t *testing.T) {
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
		req := httptest.NewRequest("PATCH", "/x-nmos/connection/v1.1/single/senders/"+senderID+"/staged", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("senderId", senderID)
		req = req.WithContext(contextWithRouteContext(rctx))

		ctrl.handleConnectionSenderStaged(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK for PATCH, got %d", w.Code)
		}
	})

	t.Run("GET /active after activation", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/x-nmos/connection/v1.1/single/senders/"+senderID+"/active", nil)
		w := httptest.NewRecorder()
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("senderId", senderID)
		req = req.WithContext(contextWithRouteContext(rctx))

		ctrl.handleConnectionSenderActive(w, req)

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
	})
}

func contextWithRouteContext(rctx *chi.Context) context.Context {
	return context.WithValue(context.Background(), chi.RouteCtxKey, rctx)
}
