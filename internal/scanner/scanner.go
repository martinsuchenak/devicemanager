package scanner

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/martinsuchenak/rackd/pkg/discovery"
	"github.com/martinsuchenak/rackd/internal/log"
	"github.com/martinsuchenak/rackd/internal/model"
)

// Common TCP ports to check for liveness (no special privileges required)
var commonPorts = []int{80, 443, 22, 3389, 23, 21, 25, 53, 110, 143, 993, 995}

// maxTCPPortTimeout is the maximum time to wait for a TCP connection attempt
const maxTCPPortTimeout = 2 * time.Second

// Compile-time interface check to ensure DiscoveryScanner implements discovery.Scanner
var _ discovery.Scanner = (*DiscoveryScanner)(nil)

// DiscoveryStorage interface for storage operations
type DiscoveryStorage interface {
	GetNetwork(id string) (*model.Network, error)
	CreateOrUpdateDiscoveredDevice(device *model.DiscoveredDevice) error
	UpdateDiscoveryScan(scan *model.DiscoveryScan) error
}

// Network interface for getting network info
type Network interface {
	GetSubnet() string
}

// DiscoveryScanner performs basic network discovery (OSS version)
// Premium features (ping, port scanning, ARP, service detection) are available in rackd-enterprise
type DiscoveryScanner struct {
	storage DiscoveryStorage
}

// NewDiscoveryScanner creates a new basic scanner
func NewDiscoveryScanner(storage DiscoveryStorage) *DiscoveryScanner {
	return &DiscoveryScanner{
		storage: storage,
	}
}

// ScanNetwork scans a network based on discovery rules (basic discovery only)
func (ds *DiscoveryScanner) ScanNetwork(ctx context.Context, networkID string, rule *model.DiscoveryRule, updateFunc func(*model.DiscoveryScan)) error {
	// Create scan record
	scan := &model.DiscoveryScan{
		ID:        generateID("scan"),
		NetworkID: networkID,
		Status:    "running",
		ScanType:  rule.ScanType,
		ScanDepth: ds.getDepthFromType(rule.ScanType),
	}

	now := time.Now()
	scan.StartedAt = &now

	if updateFunc != nil {
		updateFunc(scan)
	}

	// Get network details
	network, err := ds.storage.GetNetwork(networkID)
	if err != nil {
		scan.Status = "failed"
		scan.ErrorMessage = fmt.Sprintf("getting network: %v", err)
		now = time.Now()
		scan.CompletedAt = &now
		if updateFunc != nil {
			updateFunc(scan)
		}
		return fmt.Errorf("getting network: %w", err)
	}

	// Parse CIDR and generate IP list
	ips, err := ds.generateIPList(network.Subnet)
	if err != nil {
		scan.Status = "failed"
		scan.ErrorMessage = fmt.Sprintf("generating IP list: %v", err)
		now = time.Now()
		scan.CompletedAt = &now
		if updateFunc != nil {
			updateFunc(scan)
		}
		return fmt.Errorf("generating IP list: %w", err)
	}

	scan.TotalHosts = len(ips)
	if updateFunc != nil {
		updateFunc(scan)
	}

	log.Info("Starting basic network discovery", "network_id", networkID, "hosts", len(ips))

	// Limit concurrent scans to avoid overwhelming the system and database
	maxConcurrent := 5
	sem := make(chan struct{}, maxConcurrent)

	// Scan hosts concurrently
	var wg sync.WaitGroup
	var mu sync.Mutex
	foundCount := 0
	scannedCount := 0

	for _, ip := range ips {
		wg.Add(1)

		go func(ip string) {
			defer wg.Done()

			// Acquire semaphore slot
			sem <- struct{}{}
			defer func() { <-sem }()

			// Skip excluded IPs
			if ds.isExcluded(ip, rule.ExcludeIPs) {
				return
			}

			// Log every 50th host to show progress
			mu.Lock()
			scannedCount++
			if scannedCount%50 == 0 {
				log.Info("Discovery progress", "scanned", scannedCount, "total", len(ips))
			}
			mu.Unlock()

			// Scan the host (basic discovery only)
			device, err := ds.scanHost(ctx, ip, networkID, scan.ID)
			if err != nil {
				log.Debug("Host discovery failed", "ip", ip, "error", err)
				return
			}

			if device != nil {
				mu.Lock()
				foundCount++
				scan.FoundHosts = foundCount
				mu.Unlock()

				log.Debug("Device discovered", "ip", ip, "status", device.Status)

				// Save discovered device
				err := ds.storage.CreateOrUpdateDiscoveredDevice(device)
				if err != nil {
					log.Error("Failed to save discovered device", "ip", ip, "error", err)
				} else {
					log.Debug("Device saved", "ip", ip)
				}
			}

			// Update progress (throttled to every 50 hosts to reduce DB load)
			mu.Lock()
			scan.ScannedHosts++
			shouldUpdate := scan.ScannedHosts%50 == 0 || scan.ScannedHosts == scan.TotalHosts
			if scan.TotalHosts > 0 {
				scan.ProgressPercent = float64(scan.ScannedHosts) / float64(scan.TotalHosts) * 100
			}
			mu.Unlock()

			if shouldUpdate && updateFunc != nil {
				updateFunc(scan)
			}
		}(ip)
	}

	wg.Wait()

	// Complete scan
	now = time.Now()
	scan.Status = "completed"
	scan.CompletedAt = &now
	scan.DurationSeconds = int(now.Sub(*scan.StartedAt).Seconds())

	if updateFunc != nil {
		updateFunc(scan)
	}

	log.Info("Network discovery completed", "network_id", networkID, "found", foundCount, "duration", scan.DurationSeconds)
	return nil
}

// scanHost performs basic discovery on a single host
// OSS version: TCP-based liveness checking + hostname lookup
// Premium features (ICMP ping, ARP, advanced port scanning, service detection, OS detection) are available in rackd-enterprise
func (ds *DiscoveryScanner) scanHost(ctx context.Context, ip, networkID, scanID string) (*model.DiscoveredDevice, error) {
	log.Debug("Discovering host", "ip", ip)

	// Create device record with basic information
	device := &model.DiscoveredDevice{
		ID:        generateID("discovered"),
		IP:        ip,
		NetworkID: networkID,
		Status:    "offline",
		LastScanID: scanID,
		LastSeen:  time.Now(),
	}

	// Attempt hostname lookup (basic DNS reverse lookup)
	if hostname, err := ds.getHostname(ip); err == nil {
		device.Hostname = hostname
	}

	// Check liveness using TCP connection attempts to common ports
	// This works without special privileges and gives us basic online/offline status
	openPorts := ds.checkTCPPorts(ctx, ip)
	if len(openPorts) > 0 {
		device.Status = "online"
		device.OpenPorts = openPorts
	}

	// Calculate confidence score based on what we found
	device.Confidence = ds.calculateConfidence(device)

	return device, nil
}

// checkTCPPorts checks common TCP ports to determine if a host is online
// Returns a list of open ports (no privileges required for TCP connect)
func (ds *DiscoveryScanner) checkTCPPorts(ctx context.Context, ip string) []int {
	var openPorts []int
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Use a semaphore to limit concurrent connections
	sem := make(chan struct{}, 10) // Check up to 10 ports concurrently

	for _, port := range commonPorts {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Check if context is cancelled
			select {
			case <-ctx.Done():
				return
			default:
			}

			address := fmt.Sprintf("%s:%d", ip, p)
			conn, err := net.DialTimeout("tcp", address, maxTCPPortTimeout)
			if err == nil {
				conn.Close()
				mu.Lock()
				openPorts = append(openPorts, p)
				mu.Unlock()
				log.Debug("Port open", "ip", ip, "port", p)
			}
		}(port)
	}

	wg.Wait()
	return openPorts
}

// calculateConfidence calculates a confidence score based on discovered data
func (ds *DiscoveryScanner) calculateConfidence(device *model.DiscoveredDevice) int {
	score := 30 // Base score for any discovered IP

	if device.Hostname != "" {
		score += 20 // Has hostname
	}
	if len(device.OpenPorts) > 0 {
		score += 30 // At least one port open (device is online)
		if len(device.OpenPorts) > 2 {
			score += 10 // Multiple ports open increases confidence
		}
	}

	// Cap at 100
	if score > 100 {
		score = 100
	}

	return score
}

// generateIPList generates all IPs in a CIDR range
func (ds *DiscoveryScanner) generateIPList(cidr string) ([]string, error) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	var ips []string
	for ip := ipNet.IP.Mask(ipNet.Mask); ipNet.Contains(ip); inc(ip) {
		// Skip network and broadcast addresses for /30 and smaller
		ones, _ := ipNet.Mask.Size()
		if ones <= 30 {
			// Skip first (network) address
			if ip.Equal(ipNet.IP) {
				continue
			}
			// Skip last (broadcast) address
			broadcast := make(net.IP, len(ipNet.IP))
			copy(broadcast, ipNet.IP)
			for i := range ipNet.Mask {
				broadcast[i] |= ^ipNet.Mask[i]
			}
			if ip.Equal(broadcast) {
				continue
			}
		}
		ips = append(ips, ip.String())
	}

	return ips, nil
}

// isExcluded checks if an IP is in the exclusion list
func (ds *DiscoveryScanner) isExcluded(ip string, excludeList []string) bool {
	for _, excl := range excludeList {
		_, exclNet, err := net.ParseCIDR(excl)
		if err == nil && exclNet.Contains(net.ParseIP(ip)) {
			return true
		}
		if excl == ip {
			return true
		}
	}
	return false
}

// getHostname performs reverse DNS lookup
func (ds *DiscoveryScanner) getHostname(ip string) (string, error) {
	names, err := net.LookupAddr(ip)
	if err != nil || len(names) == 0 {
		return "", err
	}
	return names[0], nil
}

// getDepthFromType returns scan depth (always 2 for OSS basic discovery)
func (ds *DiscoveryScanner) getDepthFromType(scanType string) int {
	return 2 // OSS basic discovery always has same depth
}

// inc increments an IP address
func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// generateID generates a unique ID
func generateID(prefix string) string {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.New().String()
	}
	return id.String()
}
