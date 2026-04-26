package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Woord-En-Lewe/shure-nmos-bridge/internal/infrastructure"
	"github.com/Woord-En-Lewe/shure-nmos-bridge/internal/module"
)

func main() {
	// Parse command line flags
	shureAddr := flag.String("shure-addr", "", "Shure Axient control protocol address (optional, defaults to mDNS discovery)")
	nmosAddr := flag.String("nmos-addr", "0.0.0.0:8080", "NMOS Node API address")
	registryDiscoveryMode := flag.String("registry-discovery", "mdns", "Registry discovery mode: mdns, dns_sd, or static")
	registryDomain := flag.String("registry-domain", "local.", "DNS domain for DNS-SD discovery (used when registry-discovery is dns_sd)")
	registryStaticURL := flag.String("registry-url", "", "Static registry URL (used when registry-discovery is static)")
	logLevel := flag.String("log-level", "debug", "Log level: debug, info, warn, error")
	flag.Parse()

	// Configure logging level
	var level slog.Level
	switch *logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))

	// Build registry discovery config
	registryConfig := infrastructure.RegistryDiscoveryConfig{
		Mode:           infrastructure.DiscoveryMode(*registryDiscoveryMode),
		Domain:         *registryDomain,
		StaticRegistry: *registryStaticURL,
	}

	slog.Info("Starting Shure-NMOS Gateway",
		"shureAddr", *shureAddr,
		"nmosAddr", *nmosAddr,
		"registryDiscovery", *registryDiscoveryMode,
		"registryDomain", *registryDomain,
		"registryStaticURL", *registryStaticURL)

	// Create context that listens for SIGINT/SIGTERM
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Create the gateway module
	gateway := module.NewGateway(*shureAddr, *nmosAddr, registryConfig)

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
