package main

import (
	"fmt"

	"github.com/Woord-En-Lewe/shure-nmos-bridge/internal/infrastructure"
)

func main() {
	fmt.Println("Shure TPCI Control Protocol Implementation")
	fmt.Println("==========================================")

	// Demonstrate the builder pattern for TPCI commands
	fmt.Println("\n--- 1. TPCI Builder Pattern ---")

	// Create a SET command for Audio Gain
	gainCmd := infrastructure.NewShureCommand("SET").
		WithIndex(1).
		WithParam("AUDIO_GAIN", infrastructure.NewAudioGain(18)).
		Build()

	fmt.Printf("Built Gain Command: %s\n", gainCmd)

	// Create a GET command for Frequency
	freqCmd := infrastructure.NewShureCommand("GET").
		WithIndex(1).
		WithParam("FREQUENCY", nil).
		Build()

	fmt.Printf("Built Frequency Command: %s\n", freqCmd)

	// Demonstrate response parsing
	fmt.Println("\n--- 2. TPCI Response Parsing ---")

	sampleResponse := "< REP 1 CHAN_NAME {VOCAL_1} >"
	event := infrastructure.ParseTPCIResponse(sampleResponse)
	if event == nil {
		fmt.Println("Failed to parse response")
	} else {
		fmt.Printf("Parsed TPCI Report:\n")
		fmt.Printf("  Channel: %d\n", event.Channel)
		fmt.Printf("  Param:   %s\n", event.Param)
		fmt.Printf("  Value:   %s\n", event.Value)
	}

	// Demonstrate mDNS discovery
	fmt.Println("\n--- 3. mDNS & Shure Discovery ---")

	discoverer := infrastructure.NewShureDiscoverer()
	fmt.Printf("Created Discoverer: %T\n", discoverer)

	fmt.Println("Discoverer monitors:")
	fmt.Println("  - mDNS: _shure-control._tcp, _shure._tcp, _http._tcp")
	fmt.Println("  - Shure: 239.255.254.253:8427 (SLP)")

	fmt.Println("\n--- Implementation Summary ---")
	fmt.Println("✓ TPCI communication with Shure devices (brackets format)")
	fmt.Println("✓ Support for QLXD4, ULXD4, and Axient Digital protocols")
	fmt.Println("✓ Automatic capability discovery upon connection (GET DEVICE_ID, etc.)")
	fmt.Println("✓ Dual-protocol discovery (mDNS + Shure SLP)")
	fmt.Println("✓ Mapping of Shure controls to NMOS Node API")
}
