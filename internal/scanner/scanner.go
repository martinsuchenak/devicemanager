package scanner

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/martinsuchenak/rackd/internal/log"
	"github.com/martinsuchenak/rackd/internal/model"
)

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

// DiscoveryScanner performs network discovery
type DiscoveryScanner struct {
	storage DiscoveryStorage

	// Scanners
	pingScanner    *PingScanner
	portScanner    *PortScanner
	arpScanner     *ARPScanner
	serviceScanner *ServiceScanner
}

// NewDiscoveryScanner creates a new scanner
func NewDiscoveryScanner(storage DiscoveryStorage) *DiscoveryScanner {
	return &DiscoveryScanner{
		storage:       storage,
		pingScanner:   NewPingScanner(),
		portScanner:   NewPortScanner(),
		arpScanner:    NewARPScanner(),
		serviceScanner: NewServiceScanner(),
	}
}

// ScanNetwork scans a network based on discovery rules
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

	log.Info("Starting network scan", "network_id", networkID, "hosts", len(ips))

	// Log scan type for debugging
	log.Debug("Scan configuration", "type", rule.ScanType, "scan_ports", rule.ScanPorts, "timeout", rule.TimeoutSeconds)

	// Limit concurrent scans to avoid overwhelming the system and database
	maxConcurrent := 5 // Reduced from 20 to prevent SQLite write contention
	sem := make(chan struct{}, maxConcurrent)

	// Scan hosts concurrently
	var wg sync.WaitGroup
	var mu sync.Mutex
	foundCount := 0
	scannedCount := 0

	log.Info("Spawning goroutines", "count", len(ips), "max_concurrent", maxConcurrent)

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
				log.Info("Scan progress", "scanned", scannedCount, "total", len(ips))
			}
			mu.Unlock()

			// Scan the host
			device, err := ds.scanHost(ctx, ip, networkID, rule, scan.ID)
			if err != nil {
				log.Debug("Host scan failed", "ip", ip, "error", err)
				return // Continue with other hosts
			}

			if device != nil {
				mu.Lock()
				foundCount++
				scan.FoundHosts = foundCount
				mu.Unlock()

				log.Debug("Device discovered", "ip", ip, "status", device.Status, "ports", len(device.OpenPorts))

				// Save discovered device with retry
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

	log.Info("All host scan goroutines completed", "scanned", scannedCount)

	// Complete scan
	now = time.Now()
	scan.Status = "completed"
	scan.CompletedAt = &now
	scan.DurationSeconds = int(now.Sub(*scan.StartedAt).Seconds())

	if updateFunc != nil {
		updateFunc(scan)
	}

	log.Info("Network scan completed", "network_id", networkID, "found", foundCount, "duration", scan.DurationSeconds)
	return nil
}

// scanHost performs multi-stage scanning on a single host
func (ds *DiscoveryScanner) scanHost(ctx context.Context, ip, networkID string, rule *model.DiscoveryRule, scanID string) (*model.DiscoveredDevice, error) {
	log.Debug("Scanning host", "ip", ip)
	timeout := time.Duration(rule.TimeoutSeconds) * time.Second

	// Stage 1: Ping check (if privileged)
	var alive bool
	var pingErr error
	if ps := ds.pingScanner; ps != nil {
		alive, pingErr = ds.pingScanner.Ping(ctx, ip, timeout)
	}

	// For quick scans, only report hosts that respond to ping
	if rule.ScanType == "quick" && (!alive || pingErr != nil) {
		return nil, nil // Host is down or unreachable
	}

	// Create device record
	device := &model.DiscoveredDevice{
		ID:        generateID("discovered"),
		IP:        ip,
		NetworkID: networkID,
		Status:    "unknown", // Will be updated based on what we find
		LastScanID: scanID,
		LastSeen:  time.Now(),
	}

	// Stage 2: MAC address and hostname (if alive/ping succeeded)
	if alive {
		device.Status = "online"
		if mac, err := ds.arpScanner.GetMAC(ctx, ip); err == nil {
			device.MACAddress = mac
		}

		if hostname, err := ds.getHostname(ip); err == nil {
			device.Hostname = hostname
		}
	}

	// Stage 3: Port scanning (always do for full/deep scans, even if ping failed)
	if rule.ScanPorts && rule.ScanType != "quick" {
		ports, err := ds.portScanner.ScanPorts(ctx, ip, rule)
		if err == nil && len(ports) > 0 {
			device.OpenPorts = ports
			// Update status if we found open ports
			if device.Status == "unknown" {
				device.Status = "online"
			}
		}
	}

	// Stage 4: Service fingerprinting
	if rule.ServiceDetection && len(device.OpenPorts) > 0 {
		services := ds.serviceScanner.DetectServices(ctx, ip, device.OpenPorts)
		device.Services = services
	}

	// Stage 5: OS fingerprinting (if enabled)
	if rule.OSDetection {
		osGuess := ds.guessOS(device)
		device.OSGuess = osGuess.OS
		device.OSFamily = osGuess.Family
	}

	// Calculate confidence score
	device.Confidence = ds.calculateConfidence(device)

	return device, nil
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

// OSGuess represents an OS guess
type OSGuess struct {
	OS     string
	Family string
}

// guessOS guesses the OS based on available data
func (ds *DiscoveryScanner) guessOS(device *model.DiscoveredDevice) *OSGuess {
	// Basic passive fingerprinting based on open ports
	osGuess := &OSGuess{
		OS:     "Unknown",
		Family: "Unknown",
	}

	// Check for common port patterns
	hasWindowsPorts := containsAny(device.OpenPorts, []int{135, 139, 445, 3389})
	hasLinuxPorts := containsAny(device.OpenPorts, []int{22, 111, 2049})
	hasUnixPorts := containsAny(device.OpenPorts, []int{22, 111})

	if hasWindowsPorts && !hasLinuxPorts {
		osGuess.OS = "Windows"
		osGuess.Family = "Windows"
	} else if hasLinuxPorts && !hasWindowsPorts {
		osGuess.OS = "Linux"
		osGuess.Family = "Unix"
	} else if hasUnixPorts {
		osGuess.OS = "Unix-like"
		osGuess.Family = "Unix"
	}

	// Check for specific services
	for _, svc := range device.Services {
		if svc.Service == "SSH" {
			if osGuess.Family == "Unknown" {
				osGuess.OS = "Linux/Unix"
				osGuess.Family = "Unix"
			}
		}
	}

	return osGuess
}

// calculateConfidence calculates a confidence score
func (ds *DiscoveryScanner) calculateConfidence(device *model.DiscoveredDevice) int {
	score := 50

	if device.MACAddress != "" {
		score += 20
	}
	if device.Hostname != "" {
		score += 15
	}
	if len(device.OpenPorts) > 0 {
		score += 10
	}
	if device.OSGuess != "" {
		score += 5
	}

	if score > 100 {
		score = 100
	}

	return score
}

// getDepthFromType converts scan type to depth level
func (ds *DiscoveryScanner) getDepthFromType(scanType string) int {
	switch scanType {
	case "quick":
		return 1
	case "full":
		return 3
	case "deep":
		return 5
	default:
		return 2
	}
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

// containsAny checks if a slice contains any of the specified values
func containsAny(slice []int, values []int) bool {
	for _, v := range values {
		for _, s := range slice {
			if s == v {
				return true
			}
		}
	}
	return false
}

// generateID generates a unique ID
func generateID(prefix string) string {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.New().String()
	}
	return id.String()
}
