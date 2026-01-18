package model

import "time"

// DeviceRelationship represents a relationship between two devices
type DeviceRelationship struct {
	ParentID  string    `json:"parent_id"`
	ChildID   string    `json:"child_id"`
	Type      string    `json:"type"` // "contains", "connected_to", "depends_on"
	CreatedAt time.Time `json:"created_at"`
}
