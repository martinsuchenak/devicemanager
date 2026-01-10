package scanner

import (
	"context"
	"fmt"
	"net"

	"github.com/j-keck/arping"
)

// ARPScanner performs ARP requests to get MAC addresses
type ARPScanner struct{}

// NewARPScanner creates a new ARP scanner
func NewARPScanner() *ARPScanner {
	return &ARPScanner{}
}

// GetMAC gets the MAC address for an IP using ARP
func (as *ARPScanner) GetMAC(ctx context.Context, ip string) (string, error) {
	mac, _, err := arping.Ping(net.ParseIP(ip))
	if err != nil {
		return "", fmt.Errorf("arping failed: %w", err)
	}

	return mac.String(), nil
}
