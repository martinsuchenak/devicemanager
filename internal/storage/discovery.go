package storage

import (
	"errors"

	"github.com/martinsuchenak/rackd/internal/model"
)

var (
	// ErrDiscoveredDeviceNotFound is returned when a discovered device is not found
	ErrDiscoveredDeviceNotFound = errors.New("discovered device not found")
	// ErrDiscoveryScanNotFound is returned when a scan is not found
	ErrDiscoveryScanNotFound = errors.New("discovery scan not found")
	// ErrDiscoveryRuleNotFound is returned when a rule is not found
	ErrDiscoveryRuleNotFound = errors.New("discovery rule not found")
)

// DiscoveryStorage defines the interface for discovery-related storage operations
type DiscoveryStorage interface {
	// Networks (required by scanner to get subnet info)
	GetNetwork(id string) (*model.Network, error)

	// Discovered Devices
	ListDiscoveredDevices(filter *model.DiscoveredDeviceFilter) ([]model.DiscoveredDevice, error)
	GetDiscoveredDevice(id string) (*model.DiscoveredDevice, error)
	GetDiscoveredDeviceByIP(ip string) (*model.DiscoveredDevice, error)
	CreateOrUpdateDiscoveredDevice(device *model.DiscoveredDevice) error
	DeleteDiscoveredDevice(id string) error
	PromoteDevice(id string, promoteReq *model.PromoteDeviceRequest) (*model.Device, error)
	BulkPromoteDevices(ids []string, promoteReqs []model.PromoteDeviceRequest) ([]model.Device, []error)
	CleanupOldDevices(olderThanDays int) (int, error)

	// Discovery Scans
	ListDiscoveryScans(networkID string) ([]model.DiscoveryScan, error)
	GetDiscoveryScan(id string) (*model.DiscoveryScan, error)
	CreateDiscoveryScan(scan *model.DiscoveryScan) error
	UpdateDiscoveryScan(scan *model.DiscoveryScan) error
	DeleteDiscoveryScan(id string) error

	// Discovery Rules
	ListDiscoveryRules(networkID string) ([]model.DiscoveryRule, error)
	GetDiscoveryRule(id string) (*model.DiscoveryRule, error)
	GetDiscoveryRuleByNetwork(networkID string) (*model.DiscoveryRule, error)
	CreateDiscoveryRule(rule *model.DiscoveryRule) error
	UpdateDiscoveryRule(rule *model.DiscoveryRule) error
	DeleteDiscoveryRule(id string) error
}
