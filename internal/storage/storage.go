package storage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/martinsuchenak/rackd/internal/model"
)

var (
	ErrDeviceNotFound     = errors.New("device not found")
	ErrInvalidID          = errors.New("invalid device ID")
	ErrDatacenterNotFound = errors.New("datacenter not found")
	ErrNetworkNotFound    = errors.New("network not found")
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

// RelationshipStorage defines the interface for device relationships
type RelationshipStorage interface {
	AddRelationship(parentID, childID, relationshipType string) error
	RemoveRelationship(parentID, childID, relationshipType string) error
	GetRelationships(deviceID string) ([]Relationship, error)
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
}

// MigrateFromFileStorage is kept for backward compatibility during migration
// It reads devices from old file-based storage and returns them
func MigrateFromFileStorage(dataDir, format string) ([]model.Device, error) {
	if format != "json" && format != "toml" {
		format = "json"
	}

	entries, err := os.ReadDir(dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []model.Device{}, nil
		}
		return nil, err
	}

	var devices []model.Device
	ext := "." + format

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if !strings.HasSuffix(entry.Name(), ext) {
			continue
		}

		var device model.Device
		filePath := filepath.Join(dataDir, entry.Name())
		file, err := os.Open(filePath)
		if err != nil {
			return nil, err
		}

		switch format {
		case "json":
			err = loadJSON(file, &device)
		case "toml":
			err = loadTOML(file, &device)
		}
		file.Close()

		if err != nil {
			return nil, err
		}

		devices = append(devices, device)
	}

	return devices, nil
}
