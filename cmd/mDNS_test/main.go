package main

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/grandcat/zeroconf"
)

func main() {
	fmt.Println("Starting robust mDNS discovery test using zeroconf...")
	
	// Create context that listens for SIGINT/SIGTERM
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Initialize the resolver
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Fatalln("Failed to initialize resolver:", err.Error())
	}

	// Channel to receive discovered service entries
	entries := make(chan *zeroconf.ServiceEntry)
	
	// Start a goroutine to process results
	go func(results <-chan *zeroconf.ServiceEntry) {
		for entry := range results {
			fmt.Println("--------------------------------------------------")
			fmt.Printf("Found service: %s\n", entry.Instance)
			fmt.Printf("  Service Type: %s.%s\n", entry.Service, entry.Domain)
			fmt.Printf("  Hostname:     %s\n", entry.HostName)
			fmt.Printf("  Port:         %d\n", entry.Port)
			if len(entry.AddrIPv4) > 0 {
				fmt.Printf("  IPv4 Addrs:   %v\n", entry.AddrIPv4)
			}
			if len(entry.AddrIPv6) > 0 {
				fmt.Printf("  IPv6 Addrs:   %v\n", entry.AddrIPv6)
			}
			if len(entry.Text) > 0 {
				fmt.Printf("  TXT Records:  %v\n", entry.Text)
			}
		}
	}(entries)

	// List of services to browse for
	services := []string{
		"_shure-control._tcp",
		"_shure._tcp",
		"_http._tcp",
		"_dante._udp",
		"_services._dns-sd._udp", // Browse for all services
	}

	fmt.Println("Browsing for Shure and other services. Press Ctrl+C to stop.")

	for _, service := range services {
		fmt.Printf("Starting browse for %s...\n", service)
		// We use a background context for the browse itself so it doesn't block the loop
		// But we pass the main entries channel
		err = resolver.Browse(ctx, service, "local.", entries)
		if err != nil {
			fmt.Printf("Failed to browse for %s: %v\n", service, err)
		}
	}

	// Wait for shutdown signal
	<-ctx.Done()
	fmt.Println("\nStopping discovery...")
	
	// Give it a second to clean up
	time.Sleep(1 * time.Second)
}
