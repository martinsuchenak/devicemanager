package api

import (
	"time"

	"github.com/martinsuchenak/rackd/internal/model"
	"github.com/martinsuchenak/rackd/internal/storage"
)

// mockStorage is a simple in-memory storage for testing
type mockStorage struct {
	devices       map[string]*model.Device
	datacenters   map[string]*model.Datacenter
	networks      map[string]*model.Network
	relationships map[string][]model.DeviceRelationship
	pools         map[string]*model.NetworkPool
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		devices:       make(map[string]*model.Device),
		datacenters:   make(map[string]*model.Datacenter),
		networks:      make(map[string]*model.Network),
		relationships: make(map[string][]model.DeviceRelationship),
		pools:         make(map[string]*model.NetworkPool),
	}
}

// Device Storage
func (m *mockStorage) ListDevices(filter *model.DeviceFilter) ([]model.Device, error) {
	result := make([]model.Device, 0, len(m.devices))
	for _, d := range m.devices {
		result = append(result, *d)
	}
	return result, nil
}

func (m *mockStorage) GetDevice(id string) (*model.Device, error) {
	if d, ok := m.devices[id]; ok {
		clone := *d
		return &clone, nil
	}
	return nil, storage.ErrDeviceNotFound
}

func (m *mockStorage) CreateDevice(device *model.Device) error {
	if device.ID == "" {
		device.ID = "generated-" + time.Now().Format("20060102150405")
	}
	if device.CreatedAt.IsZero() {
		device.CreatedAt = time.Now()
	}
	if device.UpdatedAt.IsZero() {
		device.UpdatedAt = time.Now()
	}
	m.devices[device.ID] = device
	return nil
}

func (m *mockStorage) UpdateDevice(device *model.Device) error {
	if _, ok := m.devices[device.ID]; !ok {
		return storage.ErrDeviceNotFound
	}
	device.UpdatedAt = time.Now()
	m.devices[device.ID] = device
	return nil
}

func (m *mockStorage) DeleteDevice(id string) error {
	if _, ok := m.devices[id]; !ok {
		return storage.ErrDeviceNotFound
	}
	delete(m.devices, id)
	return nil
}

func (m *mockStorage) SearchDevices(query string) ([]model.Device, error) {
	result := make([]model.Device, 0)
	for _, d := range m.devices {
		result = append(result, *d)
	}
	return result, nil
}

// DatacenterStorage implementation

func (m *mockStorage) ListDatacenters(filter *model.DatacenterFilter) ([]model.Datacenter, error) {
	result := make([]model.Datacenter, 0, len(m.datacenters))
	for _, dc := range m.datacenters {
		result = append(result, *dc)
	}
	return result, nil
}

func (m *mockStorage) GetDatacenter(id string) (*model.Datacenter, error) {
	if dc, ok := m.datacenters[id]; ok {
		clone := *dc
		return &clone, nil
	}
	return nil, storage.ErrDatacenterNotFound
}

func (m *mockStorage) CreateDatacenter(dc *model.Datacenter) error {
	if dc.ID == "" {
		dc.ID = "dc-" + time.Now().Format("20060102150405")
	}
	m.datacenters[dc.ID] = dc
	return nil
}

func (m *mockStorage) UpdateDatacenter(dc *model.Datacenter) error {
	if _, ok := m.datacenters[dc.ID]; !ok {
		return storage.ErrDatacenterNotFound
	}
	m.datacenters[dc.ID] = dc
	return nil
}

func (m *mockStorage) DeleteDatacenter(id string) error {
	if _, ok := m.datacenters[id]; !ok {
		return storage.ErrDatacenterNotFound
	}
	delete(m.datacenters, id)
	return nil
}

func (m *mockStorage) GetDatacenterDevices(datacenterID string) ([]model.Device, error) {
	var result []model.Device
	for _, d := range m.devices {
		if d.DatacenterID == datacenterID {
			result = append(result, *d)
		}
	}
	return result, nil
}

// NetworkStorage implementation

func (m *mockStorage) ListNetworks(filter *model.NetworkFilter) ([]model.Network, error) {
	result := make([]model.Network, 0, len(m.networks))
	for _, n := range m.networks {
		result = append(result, *n)
	}
	return result, nil
}

func (m *mockStorage) GetNetwork(id string) (*model.Network, error) {
	if n, ok := m.networks[id]; ok {
		clone := *n
		return &clone, nil
	}
	return nil, storage.ErrNetworkNotFound
}

func (m *mockStorage) CreateNetwork(network *model.Network) error {
	if network.ID == "" {
		network.ID = "net-" + time.Now().Format("20060102150405")
	}
	m.networks[network.ID] = network
	return nil
}

func (m *mockStorage) UpdateNetwork(network *model.Network) error {
	if _, ok := m.networks[network.ID]; !ok {
		return storage.ErrNetworkNotFound
	}
	m.networks[network.ID] = network
	return nil
}

func (m *mockStorage) DeleteNetwork(id string) error {
	if _, ok := m.networks[id]; !ok {
		return storage.ErrNetworkNotFound
	}
	delete(m.networks, id)
	return nil
}

func (m *mockStorage) GetNetworkDevices(networkID string) ([]model.Device, error) {
	// Simple mock implementation
	return []model.Device{}, nil
}

// RelationshipStorage implementation

func (m *mockStorage) AddRelationship(parentID, childID, relationshipType string) error {
	rel := model.DeviceRelationship{
		ParentID:  parentID,
		ChildID:   childID,
		Type:      relationshipType,
		CreatedAt: time.Now(),
	}
	m.relationships[parentID] = append(m.relationships[parentID], rel)
	return nil
}

func (m *mockStorage) GetRelationships(deviceID string) ([]model.DeviceRelationship, error) {
	return m.relationships[deviceID], nil
}

func (m *mockStorage) RemoveRelationship(parentID, childID, relType string) error {
	rels := m.relationships[parentID]
	newRels := make([]model.DeviceRelationship, 0)
	for _, r := range rels {
		if r.ChildID != childID || r.Type != relType {
			newRels = append(newRels, r)
		}
	}
	m.relationships[parentID] = newRels
	return nil
}

func (m *mockStorage) GetRelatedDevices(deviceID string, relType string) ([]model.Device, error) {
	rels := m.relationships[deviceID]
	result := make([]model.Device, 0)
	for _, r := range rels {
		if relType == "" || r.Type == relType {
			if d, ok := m.devices[r.ChildID]; ok {
				result = append(result, *d)
			}
		}
	}
	return result, nil
}

func (m *mockStorage) GetReverseRelationships(deviceID string) ([]model.DeviceRelationship, error) {
	return nil, nil // Not needed for current tests
}

// NetworkPoolStorage implementation

func (m *mockStorage) ListNetworkPools(filter *model.NetworkPoolFilter) ([]model.NetworkPool, error) {
	result := make([]model.NetworkPool, 0)
	for _, p := range m.pools {
		if filter != nil && filter.NetworkID != "" && p.NetworkID != filter.NetworkID {
			continue
		}
		result = append(result, *p)
	}
	return result, nil
}

func (m *mockStorage) GetNetworkPool(id string) (*model.NetworkPool, error) {
	if p, ok := m.pools[id]; ok {
		return p, nil
	}
	return nil, storage.ErrPoolNotFound
}

func (m *mockStorage) CreateNetworkPool(pool *model.NetworkPool) error {
	if pool.ID == "" {
		pool.ID = "pool-" + time.Now().Format("20060102150405")
	}
	m.pools[pool.ID] = pool
	return nil
}

func (m *mockStorage) UpdateNetworkPool(pool *model.NetworkPool) error {
	existing, ok := m.pools[pool.ID]
	if !ok {
		return storage.ErrPoolNotFound
	}
	// Preserve NetworkID if not provided
	if pool.NetworkID == "" {
		pool.NetworkID = existing.NetworkID
	}
	m.pools[pool.ID] = pool
	return nil
}

func (m *mockStorage) DeleteNetworkPool(id string) error {
	delete(m.pools, id)
	return nil
}

func (m *mockStorage) GetNextAvailableIP(poolID string) (string, error) {
	if p, ok := m.pools[poolID]; ok {
		return p.StartIP, nil // Simple mock implementation using StartIP
	}
	return "", storage.ErrPoolNotFound
}

func (m *mockStorage) ValidateIPInPool(poolID, ip string) (bool, error) {
	return true, nil // Mock validation
}
