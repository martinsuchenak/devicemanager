package storage

import "github.com/martinsuchenak/rackd/internal/model"

// Storage is the main storage interface for all data operations
// This is a public interface for premium/enterprise extensions
type Storage interface {
	// Device operations
	ListDevices(filter *model.DeviceFilter) ([]model.Device, error)
	GetDevice(id string) (*model.Device, error)
	CreateDevice(device *model.Device) error
	UpdateDevice(device *model.Device) error
	DeleteDevice(id string) error
	SearchDevices(query string) ([]model.Device, error)

	// Datacenter operations
	GetDatacenter(id string) (*model.Datacenter, error)
	ListDatacenters() ([]*model.Datacenter, error)
	CreateDatacenter(dc *model.Datacenter) error
	UpdateDatacenter(dc *model.Datacenter) error
	DeleteDatacenter(id string) error
}

// DiscoveryStorage defines the interface for device discovery operations
// This is a public interface for premium/enterprise extensions
type DiscoveryStorage interface {
	// GetNetwork retrieves a network by ID
	GetNetwork(id string) (*model.Network, error)

	// CreateOrUpdateDiscoveredDevice saves or updates a discovered device
	CreateOrUpdateDiscoveredDevice(device *model.DiscoveredDevice) error

	// UpdateDiscoveryScan updates a discovery scan record
	UpdateDiscoveryScan(scan *model.DiscoveryScan) error
}
