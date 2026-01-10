package scanner

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/go-ping/ping"
)

// PingScanner performs ICMP ping checks
type PingScanner struct {
	privileged bool
}

// NewPingScanner creates a new ping scanner
func NewPingScanner() *PingScanner {
	// Check if we're running as root or have appropriate permissions
	privileged := os.Geteuid() == 0 || canUseRawSocket()
	return &PingScanner{privileged: privileged}
}

// Ping checks if a host is alive using ICMP
func (ps *PingScanner) Ping(ctx context.Context, ip string, timeout time.Duration) (bool, error) {
	// Skip ping if not privileged - would block/hang without raw socket access
	if !ps.privileged {
		return false, nil // Treat as "not alive" so we fall back to port scan
	}

	pinger, err := ping.NewPinger(ip)
	if err != nil {
		return false, fmt.Errorf("creating pinger: %w", err)
	}

	pinger.Count = 1
	pinger.Timeout = timeout
	pinger.SetPrivileged(true)

	// Run the ping with a timeout
	pinger.Run()
	// Note: Run() blocks until completion (handled by timeout setting)

	stats := pinger.Statistics()
	return stats.PacketsRecv > 0, nil
}

// canUseRawSocket checks if we can use raw sockets
func canUseRawSocket() bool {
	conn, err := net.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
