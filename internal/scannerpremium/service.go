package scannerpremium

import (
	"bufio"
	"context"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/martinsuchenak/rackd/internal/model"
)

// ServiceScanner performs service fingerprinting
type ServiceScanner struct {
	timeout time.Duration
}

// NewServiceScanner creates a new service scanner
func NewServiceScanner() *ServiceScanner {
	return &ServiceScanner{
		timeout: 5 * time.Second,
	}
}

// DetectServices attempts to identify services on open ports
func (ss *ServiceScanner) DetectServices(ctx context.Context, ip string, ports []int) []model.ServiceInfo {
	results := make([]model.ServiceInfo, 0, len(ports))

	for _, port := range ports {
		select {
		case <-ctx.Done():
			return results
		default:
		}

		service := ss.probeService(ctx, ip, port)
		if service != nil {
			results = append(results, *service)
		}
	}

	return results
}

// probeService attempts to identify a service by connecting and grabbing banners
func (ss *ServiceScanner) probeService(ctx context.Context, ip string, port int) *model.ServiceInfo {
	address := net.JoinHostPort(ip, strconv.Itoa(port))

	conn, err := net.DialTimeout("tcp", address, ss.timeout)
	if err != nil {
		return nil
	}
	defer conn.Close()

	// Set read deadline
	conn.SetReadDeadline(time.Now().Add(ss.timeout))

	// Try to get a banner
	banner := ss.grabBanner(conn)

	// Identify service
	service := ss.guessService(port)
	product := ""
	version := ""

	if banner != "" {
		service, product, version = ss.parseBanner(port, banner)
	}

	return &model.ServiceInfo{
		Port:     port,
		Protocol: "tcp",
		Service:  service,
		Product:  product,
		Version:  version,
		Banner:   banner,
	}
}

// grabBanner attempts to grab a service banner
func (ss *ServiceScanner) grabBanner(conn net.Conn) string {
	// Send a simple request to trigger a response
	conn.Write([]byte("\r\n"))

	// Read response
	scanner := bufio.NewScanner(conn)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}

	return ""
}

// parseBanner attempts to parse service information from a banner
func (ss *ServiceScanner) parseBanner(port int, banner string) (service, product, version string) {
	banner = strings.ToLower(banner)

	// SSH
	if strings.Contains(banner, "ssh") || strings.Contains(banner, "openssh") {
		service = "ssh"
		if idx := strings.Index(banner, "openssh"); idx >= 0 {
			product = "OpenSSH"
			// Try to extract version
			rest := banner[idx+8:]
			if len(rest) > 0 && rest[0] == '_' {
				parts := strings.Fields(rest)
				if len(parts) > 0 {
					version = strings.Trim(parts[0], "_")
				}
			}
		}
		return
	}

	// HTTP
	if port == 80 || port == 8080 || strings.Contains(banner, "http") {
		service = "http"
		if strings.Contains(banner, "nginx") {
			product = "nginx"
		} else if strings.Contains(banner, "apache") {
			product = "Apache"
		}
		return
	}

	// HTTPS
	if port == 443 {
		service = "https"
		return
	}

	// FTP
	if port == 21 || strings.Contains(banner, "ftp") || strings.Contains(banner, "vsftpd") {
		service = "ftp"
		if strings.Contains(banner, "vsftpd") {
			product = "vsftpd"
		}
		return
	}

	// SMTP
	if port == 25 || strings.Contains(banner, "smtp") || strings.Contains(banner, "postfix") {
		service = "smtp"
		if strings.Contains(banner, "postfix") {
			product = "Postfix"
		}
		return
	}

	// MySQL
	if port == 3306 || strings.Contains(banner, "mysql") {
		service = "mysql"
		return
	}

	// PostgreSQL
	if port == 5432 || strings.Contains(banner, "postgresql") {
		service = "postgresql"
		return
	}

	// Redis
	if port == 6379 {
		service = "redis"
		return
	}

	// MongoDB
	if port == 27017 || strings.Contains(banner, "mongodb") {
		service = "mongodb"
		return
	}

	return
}

// guessService guesses the service based on port number
func (ss *ServiceScanner) guessService(port int) string {
	services := map[int]string{
		21:    "ftp",
		22:    "ssh",
		23:    "telnet",
		25:    "smtp",
		53:    "dns",
		80:    "http",
		110:   "pop3",
		111:   "rpcbind",
		135:   "msrpc",
		139:   "netbios",
		143:   "imap",
		443:   "https",
		445:   "smb",
		993:   "imaps",
		995:   "pop3s",
		1723:  "pptp",
		3306:  "mysql",
		3389:  "rdp",
		5432:  "postgresql",
		5900:  "vnc",
		6379:  "redis",
		8080:  "http-proxy",
		27017: "mongodb",
	}

	if svc, ok := services[port]; ok {
		return svc
	}
	return "unknown"
}

// SetTimeout updates the service detection timeout
func (ss *ServiceScanner) SetTimeout(timeout time.Duration) {
	ss.timeout = timeout
}
