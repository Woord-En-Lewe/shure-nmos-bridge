package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jean-pierrecoetzee/shure_nmos_gateway/internal/module"
)

func main() {
	// Parse command line flags
	shureAddr := flag.String("shure-addr", "", "Shure Axient control protocol address (optional, defaults to mDNS discovery)")
	nmosAddr := flag.String("nmos-addr", "localhost:8000", "NMOS IS-04/IS-05 address")
	flag.Parse()

	slog.Info("Starting Shure-NMOS Gateway", "shureAddr", *shureAddr, "nmosAddr", *nmosAddr)

	// Create context that listens for SIGINT/SIGTERM
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Create the gateway module
	gateway := module.NewGateway(*shureAddr, *nmosAddr)

	// Start the gateway
	if err := gateway.Start(ctx); err != nil {
		slog.Error("Failed to start gateway", "error", err)
		os.Exit(1)
	}

	// Wait for shutdown signal
	<-ctx.Done()
	slog.Info("Shutting down gateway...")

	// Graceful shutdown
	if err := gateway.Stop(ctx); err != nil {
		slog.Error("Error during shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("Gateway stopped")
}
