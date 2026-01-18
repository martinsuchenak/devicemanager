package model

import "time"

// Network represents a network subnet in a data center
type Network struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Subnet       string    `json:"subnet"` // CIDR notation, e.g., "192.168.1.0/24"
	DatacenterID string    `json:"datacenter_id"`
	Description  string    `json:"description,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// NetworkFilter holds filter criteria for listing networks
type NetworkFilter struct {
	Name         string // Filter by name (partial match)
	DatacenterID string // Filter by datacenter
}

// NetworkPool represents a range of IPs within a network
type NetworkPool struct {
	ID          string    `json:"id"`
	NetworkID   string    `json:"network_id"`
	Name        string    `json:"name"`
	StartIP     string    `json:"start_ip"`
	EndIP       string    `json:"end_ip"`
	Tags        []string  `json:"tags"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// NetworkPoolFilter holds filter criteria for listing network pools
type NetworkPoolFilter struct {
	NetworkID string
}
