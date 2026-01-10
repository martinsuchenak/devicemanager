package scanner

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/martinsuchenak/rackd/internal/model"
)

// ServiceScanner detects services on open ports
type ServiceScanner struct{}

// NewServiceScanner creates a new service scanner
func NewServiceScanner() *ServiceScanner {
	return &ServiceScanner{}
}

// ServiceProbes defines probe patterns for common services
var ServiceProbes = map[int]string{
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
	5900:  "VNC",
	6379:  "Redis",
	8080:  "HTTP-Alt",
	27017: "MongoDB",
}

// DetectServices attempts to identify services on open ports
func (ss *ServiceScanner) DetectServices(ctx context.Context, ip string, ports []int) []model.ServiceInfo {
	var services []model.ServiceInfo

	for _, port := range ports {
		service := ss.probeService(ctx, ip, port)
		if service != nil {
			services = append(services, *service)
		}
	}

	return services
}

// probeService probes a single port to identify the service
func (ss *ServiceScanner) probeService(ctx context.Context, ip string, port int) *model.ServiceInfo {
	address := fmt.Sprintf("%s:%d", ip, port)

	// Connect with timeout
	conn, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		return nil
	}
	defer conn.Close()

	service := &model.ServiceInfo{
		Port:     port,
		Protocol: "tcp",
	}

	// Try to get banner
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	reader := bufio.NewReader(conn)

	// Some servers require a probe
	if port == 80 || port == 8080 {
		fmt.Fprintf(conn, "GET / HTTP/1.0\r\n\r\n")
	} else {
		// Generic probe - just wait for banner
	}

	banner, err := reader.ReadString('\n')
	if err == nil {
		banner = strings.TrimSpace(banner)
		service.Banner = banner
		service.Service = ss.parseService(banner, port)
	} else {
		// Use default service mapping
		if defaultService, ok := ServiceProbes[port]; ok {
			service.Service = defaultService
		}
	}

	// Try to parse version from banner
	if service.Banner != "" {
		service.Version = ss.parseVersion(service.Banner)
	}

	return service
}

// parseService attempts to identify service from banner
func (ss *ServiceScanner) parseService(banner string, port int) string {
	bannerUpper := strings.ToUpper(banner)

	if strings.Contains(bannerUpper, "SSH") {
		return "SSH"
	}
	if strings.Contains(bannerUpper, "FTP") {
		return "FTP"
	}
	if strings.Contains(bannerUpper, "HTTP") {
		return "HTTP"
	}
	if strings.Contains(bannerUpper, "SMTP") {
		return "SMTP"
	}
	if strings.Contains(bannerUpper, "MYSQL") {
		return "MySQL"
	}
	if strings.Contains(bannerUpper, "POSTGRESQL") {
		return "PostgreSQL"
	}

	// Fallback to port-based mapping
	if defaultService, ok := ServiceProbes[port]; ok {
		return defaultService
	}

	return "unknown"
}

// parseVersion attempts to extract version from banner
func (ss *ServiceScanner) parseVersion(banner string) string {
	// Simple version extraction - can be enhanced
	parts := strings.Fields(banner)
	for i, part := range parts {
		if strings.Contains(part, "v") || strings.Contains(part, "Version") {
			if i+1 < len(parts) {
				return parts[i+1]
			}
		}
	}
	return ""
}
