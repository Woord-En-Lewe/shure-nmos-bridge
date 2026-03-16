package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Woord-En-Lewe/shure-nmos-bridge/internal/infrastructure"
)

func main() {
	nmosAddr := flag.String("nmos", "localhost:8081", "NMOS Node API address")
	flag.Parse()

	slog.Info("Starting Dummy NMOS Node", "addr", *nmosAddr)

	nmosCtrl := infrastructure.NewNMOSController(*nmosAddr)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := nmosCtrl.Start(ctx); err != nil {
		slog.Error("Failed to start NMOS controller", "error", err)
		os.Exit(1)
	}

	deviceID := "dummy-device-1"
	
	// Register the dummy device
	nmosCtrl.RegisterResource("devices", map[string]interface{}{
		"id":          deviceID,
		"version":     fmt.Sprintf("%d:%d", time.Now().Unix(), time.Now().Nanosecond()),
		"label":       "Dummy Audio Node",
		"description": "A simulated NMOS node with gain and fader controls",
		"tags":        map[string]interface{}{"type": []string{"dummy"}},
		"node_id":     nmosCtrl.GetNodeID(),
		"controls": []interface{}{
			map[string]interface{}{
				"type": "urn:x-nmos:control:sr-ctrl/v1.0",
				"href": fmt.Sprintf("http://%s/x-nmos/node/v1.3/devices/%s/controls/", *nmosAddr, deviceID),
			},
			map[string]interface{}{
				"type": "urn:x-nmos:control:ncp/v1.0",
				"href": fmt.Sprintf("ws://%s/x-nmos/node/v1.3/ncp", *nmosAddr),
			},
		},
	})

	// IS-12 NCP Setup for Dummy Device
	nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
		Name:    "GainWorker",
		ClassID: []int{1, 2, 1, 1},
		Properties: []infrastructure.NcPropertyDescriptor{
			{Name: "gain", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcFloat32"},
		},
	})
	nmosCtrl.RegisterClass(infrastructure.NcClassDescriptor{
		Name:    "FaderWorker",
		ClassID: []int{1, 2, 1, 2},
		Properties: []infrastructure.NcPropertyDescriptor{
			{Name: "fader", ID: infrastructure.NCPPropertyID{Level: 2, Index: 1}, TypeName: "NcFloat32"},
		},
	})

	deviceOID := 100
	devBlock := infrastructure.NewNcBlock(deviceOID, nil, "Device", "Dummy Device")
	nmosCtrl.RegisterNCPObject(deviceOID, devBlock)

	// Add to Root Block (OID 1)
	if root := nmosCtrl.GetNCPObject(1); root != nil {
		if rb, ok := root.(*infrastructure.NcBlock); ok {
			rb.AddItem(deviceOID)
		}
	}

	// Create NCP Workers for Gain and Fader
	gainWorker := infrastructure.NewNcWorker(101, []int{1, 2, 1, 1}, nil, "gain", "Audio Gain")
	gainWorker.Value = 0
	gainWorker.OnSet = func(val interface{}) error {
		slog.Info("NCP Gain Set", "value", val)
		return nil
	}
	nmosCtrl.RegisterNCPObject(101, gainWorker)
	devBlock.AddItem(101)

	faderWorker := infrastructure.NewNcWorker(102, []int{1, 2, 1, 2}, nil, "fader", "Audio Fader")
	faderWorker.Value = 50
	faderWorker.OnSet = func(val interface{}) error {
		slog.Info("NCP Fader Set", "value", val)
		return nil
	}
	nmosCtrl.RegisterNCPObject(102, faderWorker)
	devBlock.AddItem(102)

	// Set up legacy controls (optional, but kept for compatibility)
	controls := []map[string]interface{}{
		{
			"name":        "gain",
			"type":        "number",
			"value":       0,
			"min":         -18,
			"max":         42,
			"step":        1,
			"unit":        "dB",
			"description": "Audio gain control",
			"read_only":   false,
		},
		{
			"name":        "fader",
			"type":        "number",
			"value":       50,
			"min":         0,
			"max":         100,
			"step":        1,
			"unit":        "%",
			"description": "Audio fader control",
			"read_only":   false,
		},
	}
	nmosCtrl.SetControls(deviceID, controls)

	// Log control changes
	nmosCtrl.OnControlChange(func(devID, ctrlID string, value interface{}) {
		if devID == deviceID {
			slog.Info("Control Change Received", "control", ctrlID, "value", value)
			
			// Update NCP objects if they match
			if ctrlID == "gain" {
				if obj := nmosCtrl.GetNCPObject(101); obj != nil {
					obj.SetProperty(infrastructure.NCPPropertyID{Level: 2, Index: 1}, value)
				}
			} else if ctrlID == "fader" {
				if obj := nmosCtrl.GetNCPObject(102); obj != nil {
					obj.SetProperty(infrastructure.NCPPropertyID{Level: 2, Index: 1}, value)
				}
			}

			// Update the control's value in the parameters list
			params := nmosCtrl.GetControls(devID)
			for i, p := range params {
				if p["name"] == ctrlID {
					params[i]["value"] = value
					break
				}
			}
			nmosCtrl.SetControls(devID, params)

			// Also update the resource tags for IS-04 visibility
			nmosCtrl.UpdateResource("devices", devID, func(r interface{}) interface{} {
				res, ok := r.(map[string]interface{})
				if !ok {
					return r
				}
				tags, ok := res["tags"].(map[string]interface{})
				if !ok {
					tags = make(map[string]interface{})
					res["tags"] = tags
				}
				tags[ctrlID] = value
				res["version"] = fmt.Sprintf("%d:%d", time.Now().Unix(), time.Now().Nanosecond())
				return res
			})
		}
	})

	// Simulate volume events
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Simulate a volume level between -60 and 0 dB
				volume := -60.0 + rand.Float64()*60.0
				nmosCtrl.BroadcastEvent(deviceID, "volume", volume)
				slog.Debug("Sent simulated volume", "volume", volume)
			}
		}
	}()

	// Wait for interruption
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	slog.Info("Shutting down Dummy NMOS Node")
	nmosCtrl.Stop(ctx)
}
