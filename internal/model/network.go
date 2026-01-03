package model

import "time"

// Network represents a network subnet in a data center
type Network struct {
	ID          string    `json:"id" toml:"id"`
	Name        string    `json:"name" toml:"name"`
	Subnet      string    `json:"subnet" toml:"subnet"` // CIDR notation, e.g., "192.168.1.0/24"
	DatacenterID string   `json:"datacenter_id" toml:"datacenter_id"`
	Description string    `json:"description,omitempty" toml:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at" toml:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" toml:"updated_at"`
}

// NetworkFilter holds filter criteria for listing networks
type NetworkFilter struct {
	Name        string // Filter by name (partial match)
	DatacenterID string // Filter by datacenter
}
