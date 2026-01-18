package scannerpremium

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/martinsuchenak/rackd/pkg/discovery"
	"github.com/martinsuchenak/rackd/pkg/storage"
	"github.com/martinsuchenak/rackd/internal/model"
)

// Compile-time interface check to ensure PremiumScanner implements discovery.Scanner
var _ discovery.Scanner = (*PremiumScanner)(nil)

// PremiumScanOptions configures the premium scanner behavior
type PremiumScanOptions struct {
	Privileged       bool          // Use raw sockets for ping
	PingTimeout      time.Duration // Ping timeout
	PortTimeout      time.Duration // Port scan timeout per port
	ARPTimeout       time.Duration // ARP timeout
	PortScanType     string        // "common", "full", or "custom"
	ServiceDetection bool          // Enable service fingerprinting
	ARPScan          bool          // Enable ARP scanning
	OSDetection      bool          // Enable OS detection
	MaxConcurrency   int           // Maximum concurrent host scans
}

// PremiumStorage extends the basic DiscoveryStorage interface for premium features
// Deprecated: Use storage.DiscoveryStorage instead
type PremiumStorage = storage.DiscoveryStorage

// PremiumScanner performs enterprise-grade network discovery
// Includes ping, port scanning, ARP, service detection, and OS fingerprinting
type PremiumScanner struct {
	storage storage.DiscoveryStorage

	// Scanners
	pingScanner    *PingScanner
	portScanner    *PortScanner
	arpScanner     *ARPScanner
	serviceScanner *ServiceScanner

	// Options
	options *PremiumScanOptions
}

// NewPremiumScanner creates a new premium scanner
func NewPremiumScanner(store storage.DiscoveryStorage, options *PremiumScanOptions) *PremiumScanner {
	if options == nil {
		options = &PremiumScanOptions{
			Privileged:       true,
			PingTimeout:      2 * time.Second,
			PortTimeout:      500 * time.Millisecond,
			ARPTimeout:       500 * time.Millisecond,
			PortScanType:     "common",
			ServiceDetection: true,
			ARPScan:          true,
			OSDetection:      true,
			MaxConcurrency:   50,
		}
	}

	ps := &PremiumScanner{
		storage: store,
		options: options,
	}

	// Initialize scanners
	ps.pingScanner = NewPingScanner(options.PingTimeout, options.Privileged)
	ps.portScanner = NewPortScanner(options.PortTimeout, options.PortScanType)
	ps.arpScanner = NewARPScanner(options.ARPTimeout)
	ps.serviceScanner = NewServiceScanner()

	return ps
}

// ScanNetwork scans a network based on discovery rules (premium version with all features)
func (ps *PremiumScanner) ScanNetwork(ctx context.Context, networkID string, rule *model.DiscoveryRule, updateFunc func(*model.DiscoveryScan)) error {
	// Create scan record
	scan := &model.DiscoveryScan{
		ID:        generateID("scan"),
		NetworkID: networkID,
		Status:    "running",
		ScanType:  rule.ScanType,
		ScanDepth: ps.getDepthFromType(rule.ScanType),
	}

	now := time.Now()
	scan.StartedAt = &now

	if updateFunc != nil {
		updateFunc(scan)
	}

	// Get network details
	network, err := ps.storage.GetNetwork(networkID)
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
	ips, err := ps.generateIPList(network.Subnet)
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

	log.Printf("Starting premium network discovery: network_id=%s hosts=%d scan_type=%s", networkID, len(ips), rule.ScanType)

	// Limit concurrent scans
	maxConcurrent := ps.options.MaxConcurrency
	if maxConcurrent <= 0 {
		maxConcurrent = 5
	}
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

			// Check for cancellation
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Skip excluded IPs
			if ps.isExcluded(ip, rule.ExcludeIPs) {
				return
			}

			// Log every 50th host to show progress
			mu.Lock()
			scannedCount++
			if scannedCount%50 == 0 {
				log.Printf("Discovery progress: scanned=%d total=%d", scannedCount, len(ips))
			}
			mu.Unlock()

			// Scan the host with all premium features
			device, err := ps.scanHost(ctx, ip, networkID, rule, scan.ID)
			if err != nil {
				// Continue with other hosts
				return
			}

			if device != nil {
				mu.Lock()
				foundCount++
				scan.FoundHosts = foundCount
				mu.Unlock()

				log.Printf("Device discovered: ip=%s status=%s mac=%s ports=%d", ip, device.Status, device.MACAddress, len(device.OpenPorts))

				// Save discovered device
				err := ps.storage.CreateOrUpdateDiscoveredDevice(device)
				if err != nil {
					log.Printf("Failed to save discovered device: ip=%s error=%v", ip, err)
				}
			}

			// Update progress (throttled)
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

	log.Printf("Premium network discovery completed: network_id=%s found=%d duration=%d", networkID, foundCount, scan.DurationSeconds)
	return nil
}

// scanHost performs multi-stage scanning on a single host (premium version)
func (ps *PremiumScanner) scanHost(ctx context.Context, ip, networkID string, rule *model.DiscoveryRule, scanID string) (*model.DiscoveredDevice, error) {
	log.Printf("Scanning host (premium): %s", ip)

	device := &model.DiscoveredDevice{
		ID:        generateID("discovered"),
		IP:        ip,
		NetworkID: networkID,
		Status:    "unknown",
		LastScanID: scanID,
		LastSeen:  time.Now(),
	}

	// Stage 1: Ping check (ICMP)
	var alive bool
	if ps.pingScanner != nil {
		alive, _ = ps.pingScanner.Ping(ctx, ip)
		if alive {
			device.Status = "online"
		}
	}

	// For quick scans, only report hosts that respond to ping
	if rule.ScanType == "quick" && !alive {
		return nil, nil // Host is down or unreachable
	}

	// Stage 2: MAC address and hostname (if alive or doing full/deep scan)
	if alive || rule.ScanType != "quick" {
		// ARP scan for MAC address
		if ps.options.ARPScan && ps.arpScanner != nil {
			mac, _ := ps.arpScanner.GetMAC(ctx, ip)
			if mac != "" {
				device.MACAddress = mac
			}
		}

		// Hostname lookup
		if hostname, err := ps.getHostname(ip); err == nil {
			device.Hostname = hostname
		}
	}

	// Stage 3: Port scanning (for full/deep scans)
	if rule.ScanPorts && rule.ScanType != "quick" && ps.portScanner != nil {
		ports, err := ps.portScanner.ScanPorts(ctx, ip, rule)
		if err == nil && len(ports) > 0 {
			device.OpenPorts = ports
			if device.Status == "unknown" {
				device.Status = "online"
			}
		}
	}

	// Stage 4: Service fingerprinting (if enabled and ports found)
	if ps.options.ServiceDetection && rule.ServiceDetection && len(device.OpenPorts) > 0 && ps.serviceScanner != nil {
		services := ps.serviceScanner.DetectServices(ctx, ip, device.OpenPorts)
		device.Services = services
	}

	// Stage 5: OS fingerprinting (if enabled)
	if ps.options.OSDetection && rule.OSDetection {
		osGuess := ps.guessOS(device)
		device.OSGuess = osGuess.OS
		device.OSFamily = osGuess.Family
	}

	// Calculate confidence score
	device.Confidence = ps.calculateConfidence(device)

	return device, nil
}

// generateIPList generates all IPs in a CIDR range
func (ps *PremiumScanner) generateIPList(cidr string) ([]string, error) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	var ips []string
	for ip := ipNet.IP.Mask(ipNet.Mask); ipNet.Contains(ip); inc(ip) {
		// Skip network and broadcast addresses for /30 and smaller
		ones, _ := ipNet.Mask.Size()
		if ones <= 30 {
			if ip.Equal(ipNet.IP) {
				continue
			}
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
func (ps *PremiumScanner) isExcluded(ip string, excludeList []string) bool {
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
func (ps *PremiumScanner) getHostname(ip string) (string, error) {
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
func (ps *PremiumScanner) guessOS(device *model.DiscoveredDevice) *OSGuess {
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
func (ps *PremiumScanner) calculateConfidence(device *model.DiscoveredDevice) int {
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
func (ps *PremiumScanner) getDepthFromType(scanType string) int {
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
