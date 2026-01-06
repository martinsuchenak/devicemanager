package storage

import (
	"errors"

	"github.com/martinsuchenak/rackd/internal/model"
)

var (
	ErrDeviceNotFound     = errors.New("device not found")
	ErrInvalidID          = errors.New("invalid device ID")
	ErrDatacenterNotFound = errors.New("datacenter not found")
	ErrNetworkNotFound    = errors.New("network not found")
	ErrPoolNotFound       = errors.New("network pool not found")
)

// NewStorage creates a SQLite storage backend
func NewStorage(dataDir, storageType, format string) (Storage, error) {
	return NewSQLiteStorage(dataDir)
}

// NewExtendedStorage creates an extended storage with relationship support
func NewExtendedStorage(dataDir, storageType, format string) (ExtendedStorage, error) {
	return NewSQLiteStorage(dataDir)
}

// DatacenterStorage defines the interface for datacenter storage
type DatacenterStorage interface {
	ListDatacenters(filter *model.DatacenterFilter) ([]model.Datacenter, error)
	GetDatacenter(id string) (*model.Datacenter, error)
	CreateDatacenter(dc *model.Datacenter) error
	UpdateDatacenter(dc *model.Datacenter) error
	DeleteDatacenter(id string) error
	GetDatacenterDevices(datacenterID string) ([]model.Device, error)
}

// NetworkStorage defines the interface for network storage
type NetworkStorage interface {
	ListNetworks(filter *model.NetworkFilter) ([]model.Network, error)
	GetNetwork(id string) (*model.Network, error)
	CreateNetwork(network *model.Network) error
	UpdateNetwork(network *model.Network) error
	DeleteNetwork(id string) error
	GetNetworkDevices(networkID string) ([]model.Device, error)
}

// NetworkPoolStorage defines the interface for network pool storage
type NetworkPoolStorage interface {
	CreateNetworkPool(pool *model.NetworkPool) error
	UpdateNetworkPool(pool *model.NetworkPool) error
	DeleteNetworkPool(id string) error
	GetNetworkPool(id string) (*model.NetworkPool, error)
	ListNetworkPools(filter *model.NetworkPoolFilter) ([]model.NetworkPool, error)
	GetNextAvailableIP(poolID string) (string, error)
	ValidateIPInPool(poolID, ip string) (bool, error)
}

// RelationshipStorage defines the interface for device relationships
type RelationshipStorage interface {
	AddRelationship(parentID, childID, relationshipType string) error
	RemoveRelationship(parentID, childID, relationshipType string) error
	GetRelationships(deviceID string) ([]model.DeviceRelationship, error)
	GetRelatedDevices(deviceID, relationshipType string) ([]model.Device, error)
}

// Storage defines the interface for device storage
type Storage interface {
	ListDevices(filter *model.DeviceFilter) ([]model.Device, error)
	GetDevice(id string) (*model.Device, error)
	CreateDevice(device *model.Device) error
	UpdateDevice(device *model.Device) error
	DeleteDevice(id string) error
	SearchDevices(query string) ([]model.Device, error)
}

// ExtendedStorage combines Storage with relationship support
type ExtendedStorage interface {
	Storage
	RelationshipStorage
	NetworkPoolStorage
}
