package main

import (
	"context"
	"encoding/hex"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/net/ipv4"
	"golang.org/x/sys/unix"
)

const (
	mDNSMulticastAddr   = "224.0.0.251:5353"
	shureMulticastAddr  = "239.255.254.253:8427"
)

func main() {
	slog.Info("Starting Unified Shure & mDNS Diagnostic Tool...")
	slog.Info("Listening on:", 
		"mDNS", mDNSMulticastAddr, 
		"Shure Discovery", shureMulticastAddr)

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ctx, cancel := context.WithTimeout(sigCtx, 3*time.Minute)
	defer cancel()

	var wg sync.WaitGroup

	// 1. Setup mDNS Listener (SO_REUSEPORT)
	wg.Add(1)
	go func() {
		defer wg.Done()
		setupMDNSListener(ctx)
	}()

	// 2. Setup Shure Discovery Listener (UDP 8427)
	wg.Add(1)
	go func() {
		defer wg.Done()
		setupShureListener(ctx)
	}()

	slog.Info("Diagnostics Running. Press Ctrl+C to stop.")
	<-ctx.Done()
	slog.Info("Shutting down diagnostics...")
	wg.Wait()
}

func setupMDNSListener(ctx context.Context) {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, unix.IPPROTO_UDP)
	if err != nil {
		slog.Error("mDNS: Failed to create socket", "error", err)
		return
	}
	defer unix.Close(fd)

	unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
	unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)

	addr := unix.SockaddrInet4{Port: 5353}
	copy(addr.Addr[:], net.IPv4zero.To4())
	if err := unix.Bind(fd, &addr); err != nil {
		slog.Error("mDNS: Failed to bind", "error", err)
		return
	}

	file := os.NewFile(uintptr(fd), "mdns-socket")
	c, _ := net.FilePacketConn(file)
	defer c.Close()

	conn := c.(*net.UDPConn)
	p := ipv4.NewPacketConn(conn)
	p.JoinGroup(nil, &net.UDPAddr{IP: net.ParseIP("224.0.0.251")})

	// Periodic Probes
	go func() {
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		sendMDNSProbes(conn)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sendMDNSProbes(conn)
			}
		}
	}()

	buffer := make([]byte, 2048)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, src, err := conn.ReadFromUDP(buffer)
			if err != nil {
				continue
			}

			msg := new(dns.Msg)
			if err := msg.Unpack(buffer[:n]); err == nil {
				handleMDNSMessage(msg, src)
			}
		}
	}
}

func setupShureListener(ctx context.Context) {
	addr, _ := net.ResolveUDPAddr("udp4", shureMulticastAddr)
	conn, err := net.ListenMulticastUDP("udp4", nil, addr)
	if err != nil {
		// Fallback if ListenMulticastUDP fails (often due to SO_REUSEPORT needs)
		slog.Warn("Shure Discovery: Standard multicast listen failed, trying reuseport", "error", err)
		conn, err = net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 8427})
		if err != nil {
			slog.Error("Shure Discovery: Failed to bind", "error", err)
			return
		}
	}
	defer conn.Close()

	slog.Info("Shure Discovery: Listener active", "addr", shureMulticastAddr)

	buffer := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, src, err := conn.ReadFromUDP(buffer)
			if err != nil {
				continue
			}

			// Shure Discovery payloads are often SLP or proprietary.
			// For diagnostics, we log the hex and any printable strings.
			payload := buffer[:n]
			slog.Info(">>> SHURE DISCOVERY PACKET",
				"from", src.String(),
				"len", n,
				"hex", hex.EncodeToString(payload[:min(n, 32)]),
				"text", string(cleanString(payload)),
			)
		}
	}
}

func handleMDNSMessage(msg *dns.Msg, src *net.UDPAddr) {
	records := append(msg.Answer, msg.Extra...)
	for _, rr := range records {
		switch r := rr.(type) {
		case *dns.PTR:
			slog.Info("mDNS: PTR", "from", src.String(), "service", r.Hdr.Name, "instance", r.Ptr)
		case *dns.SRV:
			slog.Info("mDNS: SRV", "from", src.String(), "name", r.Hdr.Name, "port", r.Port, "target", r.Target)
		case *dns.TXT:
			slog.Info("mDNS: TXT", "from", src.String(), "name", r.Hdr.Name, "data", r.Txt)
		}
	}
}

func sendMDNSProbes(conn *net.UDPConn) {
	target, _ := net.ResolveUDPAddr("udp4", mDNSMulticastAddr)
	types := []string{"_shure-control._tcp.local.", "_shure._tcp.local.", "_nmos-node._tcp.local."}
	for _, t := range types {
		m := new(dns.Msg)
		m.SetQuestion(t, dns.TypePTR)
		buf, _ := m.Pack()
		conn.WriteToUDP(buf, target)
	}
}

func cleanString(b []byte) []byte {
	res := make([]byte, len(b))
	for i, v := range b {
		if v >= 32 && v <= 126 {
			res[i] = v
		} else {
			res[i] = '.'
		}
	}
	return res
}

func min(a, b int) int {
	if a < b { return a }
	return b
}
