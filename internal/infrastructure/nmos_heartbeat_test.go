package infrastructure

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNMOSHeartbeat(t *testing.T) {
	var mu sync.Mutex
	heartbeatCount := 0
	registrationCount := 0
	resourceRegistrationCount := 0

	// Mock NMOS Registry
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		if r.Method == "POST" {
			if r.URL.Path == "/x-nmos/registration/v1.3/resource" {
				var body map[string]interface{}
				json.NewDecoder(r.Body).Decode(&body)
				if body["type"] == "node" {
					registrationCount++
				} else {
					resourceRegistrationCount++
				}
				w.WriteHeader(http.StatusCreated)
				return
			}
			if r.URL.Path == "/x-nmos/registration/v1.3/health/nodes/test-node-id" {
				heartbeatCount++
				if heartbeatCount == 2 {
					// Simulate node expiration on second heartbeat
					w.WriteHeader(http.StatusNotFound)
				} else {
					w.WriteHeader(http.StatusOK)
				}
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	ctrl := NewNMOSController("localhost:8080").(*nmosController)
	ctrl.nodeID = "test-node-id"
	ctrl.registryURL = ts.URL
	ctrl.httpClient = ts.Client()
	
	// Add a dummy resource to check re-registration
	ctrl.resources["devices"] = append(ctrl.resources["devices"], map[string]interface{}{"id": "device-1"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// We need to set isRunning to true so RegisterNode works
	ctrl.isRunning = true

	// Manually start registration to setup state
	node := ctrl.buildNodeResource(ctrl.nodeID)
	err := ctrl.RegisterNode(node)
	assert.NoError(t, err)

	// First heartbeat: should succeed
	ctrl.performHeartbeat(ctx)
	assert.Equal(t, 1, heartbeatCount)
	assert.Equal(t, 1, registrationCount)

	// Second heartbeat: should return 404 and trigger re-registration
	ctrl.performHeartbeat(ctx)
	assert.Equal(t, 2, heartbeatCount)
	assert.Equal(t, 2, registrationCount, "Should have re-registered node after 404")
	assert.Equal(t, 1, resourceRegistrationCount, "Should have re-registered resources after 404")

	// Third heartbeat: should succeed again after re-registration
	ctrl.performHeartbeat(ctx)
	assert.Equal(t, 3, heartbeatCount)
}
