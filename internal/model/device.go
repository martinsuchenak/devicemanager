package model

import (
	"time"
)

// Device represents a tracked device with all its properties
type Device struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	MakeModel    string    `json:"make_model"`
	OS           string    `json:"os"`
	DatacenterID string    `json:"datacenter_id,omitempty"`
	Username     string    `json:"username,omitempty"`
	Tags         []string  `json:"tags"`
	Addresses    []Address `json:"addresses"`
	Domains      []string  `json:"domains"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Address represents a network address for a device
type Address struct {
	IP         string `json:"ip"`
	Port       int    `json:"port"`
	Type       string `json:"type"`                  // "ipv4", "ipv6"
	Label      string `json:"label"`                 // e.g., "management", "data"
	NetworkID  string `json:"network_id,omitempty"`  // Network this IP belongs to
	SwitchPort string `json:"switch_port,omitempty"` // Switch port (e.g., "eth0", "Gi1/0/1")
}

// DeviceFilter holds filter criteria for listing devices
type DeviceFilter struct {
	Tags []string // Filter by tags (OR logic)
}

// SearchQuery holds search criteria
type SearchQuery struct {
	Query string // Search in name, description, IP, domains
}
