package infrastructure

import (
	"context"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

// ShureDiscoverer discovers Shure devices using both mDNS and Shure Discovery Protocol (SLP)
type ShureDiscoverer struct {
	services []string
	domain   string
	devices  chan DiscoveredDevice
	done     chan struct{}
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	lastSent map[string]time.Time
	mu       sync.Mutex
}

// DiscoveredDevice represents a discovered Shure device
type DiscoveredDevice struct {
	Instance string
	Host     string
	Port     int
	Address  net.IP
	Info     map[string]string
}

// NewShureDiscoverer creates a new discoverer for Shure devices
func NewShureDiscoverer() *ShureDiscoverer {
	return &ShureDiscoverer{
		services: []string{"_shure-control._tcp", "_shure._tcp", "_http._tcp"},
		domain:   "local.",
		devices:  make(chan DiscoveredDevice, 20),
		done:     make(chan struct{}),
		lastSent: make(map[string]time.Time),
	}
}

// Discover starts the dual-protocol discovery process
func (d *ShureDiscoverer) Discover(ctx context.Context) (<-chan DiscoveredDevice, error) {
	d.ctx, d.cancel = context.WithCancel(ctx)

	// 1. Start mDNS discovery (zeroconf)
	for _, service := range d.services {
		d.wg.Add(1)
		go func(svc string) {
			defer d.wg.Done()
			d.browseMDNS(svc)
		}(service)
	}

	// 2. Start Shure Discovery Protocol (UDP 8427)
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		d.listenShureDiscovery()
	}()

	return d.devices, nil
}

// Stop halts the discovery process
func (d *ShureDiscoverer) Stop() error {
	if d.cancel != nil {
		d.cancel()
	}
	d.wg.Wait()
	close(d.done)
	close(d.devices)
	return nil
}

// browseMDNS performs the mDNS browsing using zeroconf
func (d *ShureDiscoverer) browseMDNS(service string) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		slog.Error("Failed to initialize mDNS resolver", "service", service, "error", err)
		return
	}

	entries := make(chan *zeroconf.ServiceEntry)
	go func(results <-chan *zeroconf.ServiceEntry) {
		for entry := range results {
			var addr net.IP
			if len(entry.AddrIPv4) > 0 {
				addr = entry.AddrIPv4[0]
			} else if len(entry.AddrIPv6) > 0 {
				addr = entry.AddrIPv6[0]
			}

			if addr == nil {
				continue
			}

			d.sendDevice(DiscoveredDevice{
				Instance: entry.Instance,
				Host:     entry.HostName,
				Port:     entry.Port,
				Address:  addr,
				Info:     map[string]string{"source": "mdns"},
			})
		}
	}(entries)

	if err := resolver.Browse(d.ctx, service, d.domain, entries); err != nil {
		slog.Error("Failed to browse mDNS", "service", service, "error", err)
		return
	}

	<-d.ctx.Done()
}

// listenShureDiscovery listens for Shure's proprietary SLP-based discovery on 239.255.254.253:8427
func (d *ShureDiscoverer) listenShureDiscovery() {
	multicastAddr := "239.255.254.253:8427"
	addr, _ := net.ResolveUDPAddr("udp4", multicastAddr)
	
	// Join multicast group
	conn, err := net.ListenMulticastUDP("udp4", nil, addr)
	if err != nil {
		// Fallback to standard UDP listen if multicast fails
		conn, err = net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 8427})
		if err != nil {
			slog.Error("Shure Discovery: Failed to bind", "error", err)
			return
		}
	}
	defer conn.Close()

	slog.Info("Shure Discovery: Proprietary protocol listener active", "addr", multicastAddr)

	buffer := make([]byte, 4096)
	for {
		select {
		case <-d.ctx.Done():
			return
		default:
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, _, err := conn.ReadFromUDP(buffer)
			if err != nil {
				continue
			}

			payload := string(buffer[:n])
			if !strings.Contains(payload, "acn-fctn") {
				continue
			}

			model := d.extractValue(payload, "acn-fctn")
			ip := d.extractIPFromPayload(payload)
			
			if ip != "" {
				d.sendDevice(DiscoveredDevice{
					Instance: model,
					Host:     model,
					Port:     2202, // Default Shure control port
					Address:  net.ParseIP(ip),
					Info:     map[string]string{"source": "shure_discovery", "model": model},
				})
			}
		}
	}
}

// sendDevice sends a discovered device to the channel if it's new or needs a heartbeat update
func (d *ShureDiscoverer) sendDevice(dev DiscoveredDevice) {
	key := dev.Address.String()
	
	d.mu.Lock()
	if last, ok := d.lastSent[key]; ok && time.Since(last) < 30*time.Second {
		d.mu.Unlock()
		return
	}
	d.lastSent[key] = time.Now()
	d.mu.Unlock()

	select {
	case d.devices <- dev:
	case <-d.ctx.Done():
	default:
		// Channel full, drop discovery to avoid blocking
	}
}

// extractValue finds a key=value pair in the Shure payload
func (d *ShureDiscoverer) extractValue(payload, key string) string {
	search := key + "="
	start := strings.Index(payload, search)
	if start == -1 {
		return ""
	}
	start += len(search)
	end := strings.Index(payload[start:], ")")
	if end == -1 {
		end = strings.Index(payload[start:], ",")
	}
	if end == -1 {
		return payload[start:]
	}
	return payload[start : start+end]
}

// extractIPFromPayload looks for an IP address followed by a port or semicolon
func (d *ShureDiscoverer) extractIPFromPayload(payload string) string {
	marker := "esta.sdt/"
	start := strings.Index(payload, marker)
	if start == -1 {
		return ""
	}
	start += len(marker)
	
	end := -1
	delimiters := []string{":", "/", ",", ")"}
	for _, del := range delimiters {
		pos := strings.Index(payload[start:], del)
		if pos != -1 && (end == -1 || pos < end) {
			end = pos
		}
	}
	
	if end == -1 {
		return payload[start:]
	}
	return payload[start : start+end]
}
