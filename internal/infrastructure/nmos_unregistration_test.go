package infrastructure

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
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

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		if r.Method == "DELETE" {
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
	ctrl.registered = true

	ctrl.resources["devices"] = append(ctrl.resources["devices"], map[string]interface{}{"id": "device-1"})
	ctrl.resources["sources"] = append(ctrl.resources["sources"], map[string]interface{}{"id": "source-1", "device_id": "device-1"})
	ctrl.resources["flows"] = append(ctrl.resources["flows"], map[string]interface{}{"id": "flow-1", "device_id": "device-1"})
	ctrl.resources["senders"] = append(ctrl.resources["senders"], map[string]interface{}{"id": "sender-1", "device_id": "device-1"})
	ctrl.resources["receivers"] = append(ctrl.resources["receivers"], map[string]interface{}{"id": "receiver-1", "device_id": "device-1"})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := ctrl.Stop(ctx)
	assert.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()

	byResourceType := make(map[string][]string)
	for _, d := range deletions {
		if d.resourceType != "nodes" {
			byResourceType[d.resourceType] = append(byResourceType[d.resourceType], d.id)
		}
	}

	assert.Contains(t, byResourceType, "receivers", "should unregister receivers")
	assert.Contains(t, byResourceType, "senders", "should unregister senders")
	assert.Contains(t, byResourceType, "flows", "should unregister flows")
	assert.Contains(t, byResourceType, "sources", "should unregister sources")
	assert.Contains(t, byResourceType, "devices", "should unregister devices")

	sort.Strings(byResourceType["receivers"])
	sort.Strings(byResourceType["senders"])
	sort.Strings(byResourceType["flows"])
	sort.Strings(byResourceType["sources"])
	assert.Equal(t, []string{"receiver-1"}, byResourceType["receivers"])
	assert.Equal(t, []string{"sender-1"}, byResourceType["senders"])
	assert.Equal(t, []string{"flow-1"}, byResourceType["flows"])
	assert.Equal(t, []string{"source-1"}, byResourceType["sources"])
	assert.Equal(t, []string{"device-1"}, byResourceType["devices"])

	nodeDeleted := false
	for _, d := range deletions {
		if d.resourceType == "nodes" && d.id == "test-node-id" {
			nodeDeleted = true
			break
		}
	}
	assert.True(t, nodeDeleted, "node should be unregistered last")
}
