package infrastructure

import (
	"context"
	"testing"
	"time"
)

// TestShureDiscoverer tests the mDNS discoverer creation and basic functionality
func TestShureDiscoverer(t *testing.T) {
	// Test that we can create a discoverer
	disc := NewShureDiscoverer()
	if disc == nil {
		t.Error("Expected discoverer, got nil")
	}

	// Test that we can start discovery (will not find anything without actual devices)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	devChan, err := disc.Discover(ctx)
	if err != nil {
		t.Errorf("Unexpected error starting discovery: %v", err)
	}

	// Give it a moment to try to discover
	time.Sleep(50 * time.Millisecond)

	// Clean up
	if err := disc.Stop(); err != nil {
		t.Errorf("Error stopping discoverer: %v", err)
	}

	// Channel should be closed after Stop
	select {
	case _, ok := <-devChan:
		if ok {
			t.Error("Expected closed channel after Stop")
		}
	case <-time.After(10 * time.Millisecond):
		t.Error("Channel not closed after Stop")
	}
}
