package storage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/martinsuchenak/rackd/internal/model"
)

var (
	ErrDeviceNotFound = errors.New("device not found")
	ErrInvalidID      = errors.New("invalid device ID")
)

// NewStorage creates a storage backend based on the storage type
// Valid storage types: "file", "sqlite"
func NewStorage(dataDir, storageType, format string) (Storage, error) {
	switch storageType {
	case "sqlite", "":
		return NewSQLiteStorage(dataDir)
	case "file":
		return NewFileStorage(dataDir, format)
	default:
		return NewFileStorage(dataDir, format)
	}
}

// NewExtendedStorage creates an extended storage with relationship support
func NewExtendedStorage(dataDir, storageType, format string) (ExtendedStorage, error) {
	switch storageType {
	case "sqlite", "":
		return NewSQLiteStorage(dataDir)
	case "file":
		// FileStorage doesn't support relationships, return as regular Storage
		return NewFileStorage(dataDir, format)
	default:
		return NewSQLiteStorage(dataDir)
	}
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

// FileStorage implements Storage with file-based persistence
type FileStorage struct {
	mu       sync.RWMutex
	dataDir  string
	format   string // "json" or "toml"
	devices  map[string]*model.Device
	index    map[string]*string // name/id mapping for quick lookup
}

// NewFileStorage creates a new file-based storage
func NewFileStorage(dataDir, format string) (*FileStorage, error) {
	if format != "json" && format != "toml" {
		format = "json"
	}

	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	fs := &FileStorage{
		dataDir: dataDir,
		format:  format,
		devices: make(map[string]*model.Device),
		index:   make(map[string]*string),
	}

	// Load existing devices
	if err := fs.loadAll(); err != nil {
		return nil, err
	}

	return fs, nil
}

// ListDevices returns all devices, optionally filtered
func (fs *FileStorage) ListDevices(filter *model.DeviceFilter) ([]model.Device, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	devices := make([]model.Device, 0, len(fs.devices))

	for _, device := range fs.devices {
		if fs.matchesFilter(device, filter) {
			devices = append(devices, *device)
		}
	}

	return devices, nil
}

// GetDevice retrieves a device by ID or name
func (fs *FileStorage) GetDevice(id string) (*model.Device, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	// Try direct ID lookup first
	if device, ok := fs.devices[id]; ok {
		clone := *device
		return &clone, nil
	}

	// Try name lookup via index
	if deviceID, ok := fs.index[strings.ToLower(id)]; ok && *deviceID != "" {
		if device, ok := fs.devices[*deviceID]; ok {
			clone := *device
			return &clone, nil
		}
	}

	return nil, ErrDeviceNotFound
}

// CreateDevice adds a new device
func (fs *FileStorage) CreateDevice(device *model.Device) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if device.ID == "" {
		return ErrInvalidID
	}

	if _, exists := fs.devices[device.ID]; exists {
		return errors.New("device already exists")
	}

	// Store device
	fs.devices[device.ID] = device
	fs.index[strings.ToLower(device.Name)] = &device.ID

	return fs.saveDevice(device)
}

// UpdateDevice updates an existing device
func (fs *FileStorage) UpdateDevice(device *model.Device) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if device.ID == "" {
		return ErrInvalidID
	}

	if _, exists := fs.devices[device.ID]; !exists {
		return ErrDeviceNotFound
	}

	// Update index if name changed
	oldDevice := fs.devices[device.ID]
	if oldDevice.Name != device.Name {
		delete(fs.index, strings.ToLower(oldDevice.Name))
		fs.index[strings.ToLower(device.Name)] = &device.ID
	}

	// Automatically update the UpdatedAt timestamp
	device.UpdatedAt = time.Now()

	fs.devices[device.ID] = device
	return fs.saveDevice(device)
}

// DeleteDevice removes a device
func (fs *FileStorage) DeleteDevice(id string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	device, ok := fs.devices[id]
	if !ok {
		return ErrDeviceNotFound
	}

	delete(fs.index, strings.ToLower(device.Name))
	delete(fs.devices, id)

	// Remove the device file
	filePath := fs.devicePath(id)
	return os.Remove(filePath)
}

// SearchDevices searches for devices matching the query
func (fs *FileStorage) SearchDevices(query string) ([]model.Device, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	query = strings.ToLower(query)
	results := make([]model.Device, 0)

	for _, device := range fs.devices {
		if fs.matchesSearch(device, query) {
			results = append(results, *device)
		}
	}

	return results, nil
}

// matchesFilter checks if a device matches the given filter
func (fs *FileStorage) matchesFilter(device *model.Device, filter *model.DeviceFilter) bool {
	if filter == nil || len(filter.Tags) == 0 {
		return true
	}

	// Check if any tag matches (OR logic)
	for _, filterTag := range filter.Tags {
		for _, deviceTag := range device.Tags {
			if strings.EqualFold(deviceTag, filterTag) {
				return true
			}
		}
	}

	return false
}

// matchesSearch checks if a device matches the search query
func (fs *FileStorage) matchesSearch(device *model.Device, query string) bool {
	if query == "" {
		return true
	}

	// Search in name
	if strings.Contains(strings.ToLower(device.Name), query) {
		return true
	}

	// Search in description
	if strings.Contains(strings.ToLower(device.Description), query) {
		return true
	}

	// Search in make/model
	if strings.Contains(strings.ToLower(device.MakeModel), query) {
		return true
	}

	// Search in OS
	if strings.Contains(strings.ToLower(device.OS), query) {
		return true
	}

	// Search in location
	if strings.Contains(strings.ToLower(device.Location), query) {
		return true
	}

	// Search in IP addresses
	for _, addr := range device.Addresses {
		if strings.Contains(addr.IP, query) {
			return true
		}
	}

	// Search in domains
	for _, domain := range device.Domains {
		if strings.Contains(strings.ToLower(domain), query) {
			return true
		}
	}

	// Search in tags
	for _, tag := range device.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}

	return false
}

// devicePath returns the file path for a device
func (fs *FileStorage) devicePath(id string) string {
	ext := "." + fs.format
	return filepath.Join(fs.dataDir, id+ext)
}

// saveDevice saves a single device to disk
func (fs *FileStorage) saveDevice(device *model.Device) error {
	return fs.saveFile(fs.devicePath(device.ID), device)
}

// loadAll loads all devices from the data directory
func (fs *FileStorage) loadAll() error {
	entries, err := os.ReadDir(fs.dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := "." + fs.format
		if !strings.HasSuffix(entry.Name(), ext) {
			continue
		}

		device := &model.Device{}
		if err := fs.loadFile(filepath.Join(fs.dataDir, entry.Name()), device); err != nil {
			return err
		}

		fs.devices[device.ID] = device
		fs.index[strings.ToLower(device.Name)] = &device.ID
	}

	return nil
}

// saveFile saves data to a file in the configured format
func (fs *FileStorage) saveFile(path string, data interface{}) error {
	var err error

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	// Create temp file
	tmpPath := path + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer file.Close()

	switch fs.format {
	case "json":
		err = saveJSON(file, data)
	case "toml":
		err = saveTOML(file, data)
	default:
		return errors.New("unsupported storage format")
	}

	if err != nil {
		os.Remove(tmpPath)
		return err
	}

	// Atomic rename
	return os.Rename(tmpPath, path)
}

// loadFile loads data from a file
func (fs *FileStorage) loadFile(path string, data interface{}) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	switch fs.format {
	case "json":
		return loadJSON(file, data)
	case "toml":
		return loadTOML(file, data)
	default:
		return errors.New("unsupported storage format")
	}
}

// Relationship stub methods - FileStorage doesn't support relationships
// These methods return errors to indicate relationships require SQLite backend

var ErrRelationshipsNotSupported = errors.New("relationships are not supported by file-based storage; use SQLite backend")

func (fs *FileStorage) AddRelationship(parentID, childID, relationshipType string) error {
	return ErrRelationshipsNotSupported
}

func (fs *FileStorage) RemoveRelationship(parentID, childID, relationshipType string) error {
	return ErrRelationshipsNotSupported
}

func (fs *FileStorage) GetRelationships(deviceID string) ([]Relationship, error) {
	return nil, ErrRelationshipsNotSupported
}

func (fs *FileStorage) GetRelatedDevices(deviceID, relationshipType string) ([]model.Device, error) {
	return nil, ErrRelationshipsNotSupported
}
