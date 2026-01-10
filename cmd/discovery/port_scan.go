package discovery

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/paularlott/cli"
)

// TestPortScanCommand tests port scanning without ICMP
func TestPortScanCommand() *cli.Command {
	return &cli.Command{
		Name:        "test-port-scan",
		Usage:       "Test port scanning on a host",
		Description: "Test port scanning functionality on a specific host (no ICMP required)",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "host",
				Usage:    "Host to scan (e.g., 8.8.8.8, 1.1.1.1)",
				Required: true,
			},
			&cli.IntFlag{
				Name:         "timeout",
				Usage:        "Per-port timeout in seconds",
				DefaultValue: 2,
			},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			host := cmd.GetString("host")
			timeout := cmd.GetInt("timeout")

			fmt.Println("=== Port Scan Test ===")
			fmt.Printf("Host: %s\n", host)
			fmt.Printf("Timeout: %ds\n", timeout)
			fmt.Println()

			// Common ports to scan
			ports := []int{21, 22, 23, 25, 53, 80, 110, 143, 443, 3306, 3389, 5432, 8080}

			fmt.Printf("Scanning %d common ports...\n\n", len(ports))

			var openPorts []int
			var wg sync.WaitGroup
			var mu sync.Mutex

			startTime := time.Now()

			for _, port := range ports {
				wg.Add(1)
				go func(p int) {
					defer wg.Done()

					address := fmt.Sprintf("%s:%d", host, p)
					conn, err := net.DialTimeout("tcp", address, time.Duration(timeout)*time.Second)

					if err == nil {
						conn.Close()
						mu.Lock()
						openPorts = append(openPorts, p)
						fmt.Printf("  [+] Port %d: OPEN\n", p)
						mu.Unlock()
					}
				}(port)
			}

			wg.Wait()
			duration := time.Since(startTime)

			fmt.Println()
			fmt.Println("=== Scan Complete ===")
			fmt.Printf("Duration: %v\n", duration)
			fmt.Printf("Open Ports Found: %d\n", len(openPorts))

			if len(openPorts) > 0 {
				fmt.Printf("Ports: %v\n", openPorts)

				// Try to get service banners
				fmt.Println("\n=== Service Detection ===")
				for _, port := range openPorts {
					if service := detectService(host, port); service != "" {
						fmt.Printf("  Port %d: %s\n", port, service)
					} else {
						fmt.Printf("  Port %d: Unknown service\n", port)
					}
				}
			} else {
				fmt.Println("\nNo open ports found on this host.")
				fmt.Println("\nTry scanning a host you know is up, like:")
				fmt.Println("  - 8.8.8.8 (Google DNS)")
				fmt.Println("  - 1.1.1.1 (Cloudflare DNS)")
				fmt.Println("  - 93.184.216.34 (example.com)")
			}

			return nil
		},
	}
}

// detectService tries to detect the service on a port
func detectService(host string, port int) string {
	address := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		return ""
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	// Send probe
	if port == 80 || port == 8080 {
		fmt.Fprintf(conn, "GET / HTTP/1.0\r\n\r\n")
	}

	reader := bufio.NewReader(conn)
	banner, err := reader.ReadString('\n')
	if err != nil {
		return mapPortToService(port)
	}

	banner = trimString(banner)

	// Parse banner
	if contains(banner, "SSH") {
		return "SSH"
	}
	if contains(banner, "FTP") {
		return "FTP"
	}
	if contains(banner, "HTTP") {
		return "HTTP"
	}
	if contains(banner, "SMTP") {
		return "SMTP"
	}

	return mapPortToService(port)
}

func mapPortToService(port int) string {
	services := map[int]string{
		21:    "FTP",
		22:    "SSH",
		23:    "Telnet",
		25:    "SMTP",
		53:    "DNS",
		80:    "HTTP",
		110:   "POP3",
		143:   "IMAP",
		443:   "HTTPS",
		3306:  "MySQL",
		3389:  "RDP",
		5432:  "PostgreSQL",
		8080:  "HTTP-Alt",
	}
	if s, ok := services[port]; ok {
		return s
	}
	return "Unknown"
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func trimString(s string) string {
	return strings.TrimSpace(s)
}

// Update Commands to include the new test
func Commands() []*cli.Command {
	return []*cli.Command{
		TestScanCommand(),
		TestPortScanCommand(),
	}
}
