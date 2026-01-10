package discovery

import (
	"context"
	"fmt"
	"time"

	"github.com/martinsuchenak/rackd/internal/config"
	"github.com/martinsuchenak/rackd/internal/model"
	"github.com/martinsuchenak/rackd/internal/scanner"
	"github.com/paularlott/cli"
)

// TestScanCommand runs a test scan on a network
func TestScanCommand() *cli.Command {
	return &cli.Command{
		Name:        "test-scan",
		Usage:       "Test the discovery scanner on a network",
		Description: "Run a discovery scan on a network to test the scanner functionality",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "network",
				Usage:    "Network subnet to scan (e.g., 192.168.1.0/24)",
				Required: true,
			},
			&cli.StringFlag{
				Name:         "scan-type",
				Usage:        "Scan type: quick, full, or deep",
				DefaultValue: "quick",
			},
			&cli.IntFlag{
				Name:         "timeout",
				Usage:        "Per-host timeout in seconds",
				DefaultValue: 2,
			},
			&cli.BoolFlag{
				Name:         "scan-ports",
				Usage:        "Enable port scanning",
				DefaultValue: false,
			},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			cfg := config.Load()

			// Get network subnet
			subnet := cmd.GetString("network")
			scanType := cmd.GetString("scan-type")
			timeout := cmd.GetInt("timeout")
			scanPorts := cmd.GetBool("scan-ports")

			fmt.Println("=== Discovery Scanner Test ===")
			fmt.Printf("Network: %s\n", subnet)
			fmt.Printf("Scan Type: %s\n", scanType)
			fmt.Printf("Timeout: %ds\n", timeout)
			fmt.Printf("Port Scanning: %v\n", scanPorts)
			fmt.Printf("Data Dir: %s\n", cfg.DataDir)
			fmt.Println()

			// Create test storage
			testStorage := &testStorageWrapper{subnet: subnet}

			// Create scanner
			s := scanner.NewDiscoveryScanner(testStorage)

			// Create discovery rule
			rule := &model.DiscoveryRule{
				NetworkID:        "test-network",
				ScanType:         scanType,
				TimeoutSeconds:   timeout,
				ScanPorts:        scanPorts,
				ServiceDetection: scanPorts,
				OSDetection:      true,
				ExcludeIPs:       []string{},
			}

			// Run scan
			fmt.Println("Starting scan...")
			startTime := time.Now()

			updateCount := 0
			err := s.ScanNetwork(ctx, "test-network", rule, func(scan *model.DiscoveryScan) {
				if updateCount%10 == 0 || updateCount == 0 {
					fmt.Printf("Progress: %d/%d hosts (%.1f%%)\n",
						scan.ScannedHosts, scan.TotalHosts, scan.ProgressPercent)
				}
				updateCount++
			})

			duration := time.Since(startTime)
			fmt.Println()
			fmt.Println("=== Scan Complete ===")
			fmt.Printf("Duration: %v\n", duration)
			fmt.Printf("Hosts Scanned: %d\n", updateCount)
			fmt.Printf("Devices Found: %d\n", len(testStorage.devices))

			if err != nil {
				return fmt.Errorf("scan failed: %w", err)
			}

			// Display discovered devices
			if len(testStorage.devices) > 0 {
				fmt.Println("\n=== Discovered Devices ===")
				for _, device := range testStorage.devices {
					fmt.Printf("\nIP: %s\n", device.IP)
					if device.MACAddress != "" {
						fmt.Printf("  MAC: %s\n", device.MACAddress)
					}
					if device.Hostname != "" {
						fmt.Printf("  Hostname: %s\n", device.Hostname)
					}
					if device.OSGuess != "" {
						fmt.Printf("  OS: %s (%s)\n", device.OSGuess, device.OSFamily)
					}
					if len(device.OpenPorts) > 0 {
						fmt.Printf("  Open Ports: %v\n", device.OpenPorts)
					}
					if len(device.Services) > 0 {
						fmt.Printf("  Services:\n")
						for _, svc := range device.Services {
							fmt.Printf("    - Port %d: %s", svc.Port, svc.Service)
							if svc.Version != "" {
								fmt.Printf(" (%s)", svc.Version)
							}
							fmt.Println()
						}
					}
					fmt.Printf("  Confidence: %d%%\n", device.Confidence)
				}
			} else {
				fmt.Println("\nNo devices found.")
			}

			return nil
		},
	}
}

// testStorageWrapper is a mock storage for testing
type testStorageWrapper struct {
	subnet  string
	devices []*model.DiscoveredDevice
}

func (s *testStorageWrapper) GetNetwork(id string) (*model.Network, error) {
	return &model.Network{Subnet: s.subnet}, nil
}

func (s *testStorageWrapper) CreateOrUpdateDiscoveredDevice(device *model.DiscoveredDevice) error {
	s.devices = append(s.devices, device)
	fmt.Printf("  [+] Device found: %s", device.IP)
	if device.MACAddress != "" {
		fmt.Printf(" (MAC: %s)", device.MACAddress)
	}
	if device.Hostname != "" {
		fmt.Printf(" (Hostname: %s)", device.Hostname)
	}
	fmt.Println()
	return nil
}

func (s *testStorageWrapper) UpdateDiscoveryScan(scan *model.DiscoveryScan) error {
	// Ignore for testing
	return nil
}

