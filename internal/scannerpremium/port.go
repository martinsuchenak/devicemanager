package scannerpremium

import (
	"context"
	"net"
	"strconv"
	"sync"
	"time"
)

// PortScanner performs TCP port scanning
type PortScanner struct {
	timeout    time.Duration
	scanType   string // "common", "full", "custom"
	commonPorts []int
}

// NewPortScanner creates a new port scanner
func NewPortScanner(timeout time.Duration, scanType string) *PortScanner {
	commonPorts := []int{
		21, 22, 23, 25, 53, 80, 110, 111, 135, 139,
		143, 443, 445, 993, 995, 1723, 3306, 3389, 5900, 8080,
	}

	return &PortScanner{
		timeout:     timeout,
		scanType:    scanType,
		commonPorts: commonPorts,
	}
}

// ScanPorts scans a host for open ports
// Returns list of open port numbers (integers)
func (ps *PortScanner) ScanPorts(ctx context.Context, ip string, rule interface{}) ([]int, error) {
	portsToScan := ps.getPortsToScan()

	results := make([]int, 0, len(portsToScan))
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Limit concurrent connections
	maxConcurrent := 50
	if ps.scanType == "common" {
		maxConcurrent = 100
	}
	sem := make(chan struct{}, maxConcurrent)

	for _, port := range portsToScan {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if ps.scanPort(ctx, ip, p) {
				mu.Lock()
				results = append(results, p)
				mu.Unlock()
			}
		}(port)
	}

	wg.Wait()
	return results, nil
}

// scanPort scans a single port
func (ps *PortScanner) scanPort(ctx context.Context, ip string, port int) bool {
	address := net.JoinHostPort(ip, strconv.Itoa(port))

	conn, err := net.DialTimeout("tcp", address, ps.timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// getPortsToScan returns the list of ports to scan based on scan type
func (ps *PortScanner) getPortsToScan() []int {
	switch ps.scanType {
	case "common":
		return ps.commonPorts
	case "full":
		// Scan top 1000 ports
		ports := make([]int, 1000)
		for i := 1; i <= 1000; i++ {
			ports[i-1] = i
		}
		return ports
	default:
		return ps.commonPorts
	}
}

// SetTimeout updates the port scan timeout
func (ps *PortScanner) SetTimeout(timeout time.Duration) {
	ps.timeout = timeout
}

// SetScanType updates the scan type
func (ps *PortScanner) SetScanType(scanType string) {
	ps.scanType = scanType
}

// SetConcurrent sets the max concurrent connections
func (ps *PortScanner) SetConcurrent(concurrent int) {
	// Reserved for future use
}
