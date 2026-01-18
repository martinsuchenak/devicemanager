package model

import "time"

// Datacenter represents a data center location
type Datacenter struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Location    string    `json:"location,omitempty"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// DatacenterFilter holds filter criteria for listing datacenters
type DatacenterFilter struct {
	Name string // Filter by name (partial match)
}
