package scanner

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/martinsuchenak/rackd/internal/model"
)

// PortScanner performs TCP port scanning
type PortScanner struct{}

// NewPortScanner creates a new port scanner
func NewPortScanner() *PortScanner {
	return &PortScanner{}
}

// CommonPorts to scan by default
var CommonPorts = []int{
	21, 22, 23, 25, 53, 80, 110, 111, 135, 139,
	143, 443, 445, 993, 995, 1723, 3306, 3389, 5900, 8080,
}

// ScanPorts scans ports on a host
func (ps *PortScanner) ScanPorts(ctx context.Context, ip string, rule *model.DiscoveryRule) ([]int, error) {
	ports := ps.getPortsToScan(rule)
	if len(ports) == 0 {
		return []int{}, nil
	}

	var openPorts []int
	var mu sync.Mutex
	var wg sync.WaitGroup

	timeout := time.Duration(rule.TimeoutSeconds) * time.Second

	for _, port := range ports {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()

			address := fmt.Sprintf("%s:%d", ip, p)
			conn, err := net.DialTimeout("tcp", address, timeout)

			if err == nil {
				conn.Close()
				mu.Lock()
				openPorts = append(openPorts, p)
				mu.Unlock()
			}
		}(port)
	}

	wg.Wait()
	return openPorts, nil
}

// getPortsToScan determines which ports to scan
func (ps *PortScanner) getPortsToScan(rule *model.DiscoveryRule) []int {
	if !rule.ScanPorts {
		return []int{}
	}

	switch rule.PortScanType {
	case "common":
		return CommonPorts
	case "full":
		// Scan common ports only for performance
		// Full range (1-65535) would take too long
		return CommonPorts
	case "custom":
		if len(rule.CustomPorts) > 0 {
			return rule.CustomPorts
		}
		return CommonPorts
	default:
		return CommonPorts
	}
}
