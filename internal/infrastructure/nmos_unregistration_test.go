package infrastructure

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestControlledUnregistration(t *testing.T) {
	var mu sync.Mutex
	deletions := make([]struct {
		resourceType string
		id           string
	}, 0)

	// Mock NMOS Registry
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		if r.Method == "DELETE" {
			// Path format: /x-nmos/registration/v1.3/resource/{type}/{id}
			path := strings.Trim(r.URL.Path, "/")
			segments := strings.Split(path, "/")
			
			resourceType := ""
			id := ""
			
			if len(segments) >= 6 {
				resourceType = segments[4]
				id = segments[5]
			}

			deletions = append(deletions, struct {
				resourceType string
				id           string
			}{resourceType, id})
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	ctrl := NewNMOSController("localhost:8080").(*nmosController)
	ctrl.nodeID = "test-node-id"
	ctrl.registryURL = ts.URL
	ctrl.httpClient = ts.Client()
	ctrl.isRunning = true

	// Pre-populate resources
	ctrl.resources["devices"] = append(ctrl.resources["devices"], map[string]interface{}{"id": "device-1"})
	ctrl.resources["sources"] = append(ctrl.resources["sources"], map[string]interface{}{"id": "source-1"})
	ctrl.resources["flows"] = append(ctrl.resources["flows"], map[string]interface{}{"id": "flow-1"})
	ctrl.resources["senders"] = append(ctrl.resources["senders"], map[string]interface{}{"id": "sender-1"})
	ctrl.resources["receivers"] = append(ctrl.resources["receivers"], map[string]interface{}{"id": "receiver-1"})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Call Stop, which should trigger controlled unregistration
	err := ctrl.Stop(ctx)
	assert.NoError(t, err)

	// Verify deletions happened in the correct order
	// Receivers -> Senders -> Flows -> Sources -> Devices -> Node
	expectedOrder := []string{"receivers", "senders", "flows", "sources", "devices", "node"}
	
	mu.Lock()
	defer mu.Unlock()
	
	assert.Equal(t, len(expectedOrder), len(deletions), "Should have deleted all resources plus the node")
	
	for i, expectedType := range expectedOrder {
		if i < len(deletions) {
			assert.Equal(t, expectedType, deletions[i].resourceType, "Wrong deletion order at index %d", i)
		}
	}
}
