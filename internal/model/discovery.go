package model

import "time"

// DiscoveredDevice represents a device found during network discovery
type DiscoveredDevice struct {
	ID                string    `json:"id"`
	IP                string    `json:"ip"`
	MACAddress        string    `json:"mac_address,omitempty"`
	Hostname          string    `json:"hostname,omitempty"`
	NetworkID         string    `json:"network_id"`

	// Discovery metadata
	Status            string    `json:"status"` // online, offline, unknown
	Confidence        int       `json:"confidence"` // 0-100

	// Device information
	OSGuess           string    `json:"os_guess,omitempty"`
	OSFamily          string    `json:"os_family,omitempty"`
	OpenPorts         []int     `json:"open_ports,omitempty"`

	// Service fingerprinting
	Services          []ServiceInfo `json:"services,omitempty"`

	// Scan tracking
	FirstSeen         time.Time `json:"first_seen"`
	LastSeen          time.Time `json:"last_seen"`
	LastScanID        string    `json:"last_scan_id,omitempty"`

	// Promotion tracking
	PromotedToDeviceID string   `json:"promoted_to_device_id,omitempty"`
	PromotedAt        *time.Time `json:"promoted_at,omitempty"`

	// Raw data
	RawScanData       string    `json:"raw_scan_data,omitempty"`

	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// ServiceInfo represents a detected service
type ServiceInfo struct {
	Port        int    `json:"port"`
	Protocol    string `json:"protocol"` // tcp, udp
	Service     string `json:"service,omitempty"` // ssh, http, etc.
	Version     string `json:"version,omitempty"`
	Product     string `json:"product,omitempty"`
	Banner      string `json:"banner,omitempty"`
}

// DiscoveryScan represents a scan operation
type DiscoveryScan struct {
	ID              string    `json:"id"`
	NetworkID       string    `json:"network_id"`
	Status          string    `json:"status"` // pending, running, completed, failed, cancelled

	// Scan configuration
	ScanType        string    `json:"scan_type"` // quick, full, deep
	ScanDepth       int       `json:"scan_depth"` // 1-5

	// Progress tracking
	TotalHosts      int       `json:"total_hosts"`
	ScannedHosts    int       `json:"scanned_hosts"`
	FoundHosts      int       `json:"found_hosts"`
	ProgressPercent float64   `json:"progress_percent"`

	// Timing
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	DurationSeconds int       `json:"duration_seconds,omitempty"`

	// Results
	ErrorMessage    string    `json:"error_message,omitempty"`

	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// DiscoveryRule represents scan configuration for a network
type DiscoveryRule struct {
	ID                  string    `json:"id"`
	NetworkID           string    `json:"network_id"`
	Enabled             bool      `json:"enabled"`

	// Schedule
	ScanIntervalHours   int       `json:"scan_interval_hours"`
	ScanType            string    `json:"scan_type"`

	// Limits
	MaxConcurrentScans  int       `json:"max_concurrent_scans"`
	TimeoutSeconds      int       `json:"timeout_seconds"`

	// Port scanning
	ScanPorts           bool      `json:"scan_ports"`
	PortScanType        string    `json:"port_scan_type"` // common, full, custom
	CustomPorts         []int     `json:"custom_ports,omitempty"`

	// Advanced features
	ServiceDetection    bool      `json:"service_detection"`
	OSDetection         bool      `json:"os_detection"`

	// Exclusions
	ExcludeIPs          []string  `json:"exclude_ips,omitempty"`
	ExcludeHosts        []string  `json:"exclude_hosts,omitempty"`

	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// DiscoveredDeviceFilter holds filter criteria for listing discovered devices
type DiscoveredDeviceFilter struct {
	NetworkID     string
	Status        string
	Promoted      *bool // nil = all, true = promoted, false = not promoted
	MinConfidence int
}

// PromoteDeviceRequest holds data for promoting a discovered device
type PromoteDeviceRequest struct {
	DeviceID        string   `json:"device_id,omitempty"` // Optional: use this ID
	Name            string   `json:"name"`                // Required
	Description     string   `json:"description,omitempty"`
	MakeModel       string   `json:"make_model,omitempty"`
	OS              string   `json:"os,omitempty"`
	DatacenterID    string   `json:"datacenter_id,omitempty"`
	Username        string   `json:"username,omitempty"`
	Location        string   `json:"location,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	Domains         []string `json:"domains,omitempty"`
}
