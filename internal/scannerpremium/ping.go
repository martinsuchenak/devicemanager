package scannerpremium

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// PingScanner performs ICMP ping checks
type PingScanner struct {
	timeout    time.Duration
	privileged bool
}

// NewPingScanner creates a new ping scanner
func NewPingScanner(timeout time.Duration, privileged bool) *PingScanner {
	return &PingScanner{
		timeout:    timeout,
		privileged: privileged,
	}
}

// Ping checks if a host is alive using ICMP
// Returns (alive, rtt)
func (ps *PingScanner) Ping(ctx context.Context, ip string) (bool, time.Duration) {
	if !ps.privileged {
		return false, 0 // Not privileged, skip ping
	}

	// Start time for RTT calculation
	start := time.Now()

	// Create ICMP echo request
	message := icmp.Message{
		Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{
			ID:   os.Getpid() & 0xffff,
			Seq:  1,
			Data: []byte("rackd-ping"),
		},
	}

	// Marshal message
	data, err := message.Marshal(nil)
	if err != nil {
		return false, 0
	}

	// Create connection
	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return false, 0
	}
	defer conn.Close()

	// Set deadline
	deadline := time.Now().Add(ps.timeout)
	if err := conn.SetReadDeadline(deadline); err != nil {
		return false, 0
	}

	// Send packet
	dst := &net.IPAddr{IP: net.ParseIP(ip)}
	if _, err := conn.WriteTo(data, dst); err != nil {
		return false, 0
	}

	// Wait for reply
	reply := make([]byte, 1500)
	if err := conn.SetReadDeadline(time.Now().Add(ps.timeout)); err != nil {
		return false, 0
	}

	n, peer, err := conn.ReadFrom(reply)
	if err != nil {
		return false, 0
	}

	// Calculate RTT
	rtt := time.Since(start)

	// Parse reply
	rm, err := icmp.ParseMessage(1, reply[:n])
	if err != nil {
		return false, 0
	}

	// Check if this is an echo reply
	if rm.Type == ipv4.ICMPTypeEchoReply {
		return true, rtt
	}

	_ = peer // Use peer variable
	return false, 0
}

// PingBatch pings multiple hosts concurrently
func (ps *PingScanner) PingBatch(ctx context.Context, ips []string, maxConcurrent int) map[string]bool {
	results := make(map[string]bool)
	sem := make(chan struct{}, maxConcurrent)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, ip := range ips {
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			alive, _ := ps.Ping(ctx, ip)
			mu.Lock()
			results[ip] = alive
			mu.Unlock()
		}(ip)
	}

	wg.Wait()
	return results
}

// SetTimeout updates the ping timeout
func (ps *PingScanner) SetTimeout(timeout time.Duration) {
	ps.timeout = timeout
}

// SetPrivileged updates privileged mode
func (ps *PingScanner) SetPrivileged(privileged bool) {
	ps.privileged = privileged
}

// Simple TCP fallback ping (when ICMP is not available)
func (ps *PingScanner) TCPPing(ctx context.Context, ip string, port int) bool {
	timeout := ps.timeout
	if timeout == 0 {
		timeout = 2 * time.Second
	}

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
