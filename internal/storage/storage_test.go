package storage

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/martinsuchenak/rackd/internal/model"
)

// setupTestStorage creates a temporary storage instance for testing
func setupTestStorage(t *testing.T, format string) *FileStorage {
	t.Helper()

	tmpDir := t.TempDir()
	storage, err := NewFileStorage(tmpDir, format)
	if err != nil {
		t.Fatalf("Failed to create test storage: %v", err)
	}

	return storage
}

func TestNewFileStorage(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		wantErr bool
	}{
		{"JSON format", "json", false},
		{"TOML format", "toml", false},
		{"Invalid format defaults to JSON", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			storage, err := NewFileStorage(tmpDir, tt.format)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewFileStorage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if storage == nil {
				t.Error("Expected storage to be created")
			}

			if storage.format != "json" && storage.format != "toml" {
				t.Errorf("Expected format to be json or toml, got %s", storage.format)
			}
		})
	}
}

func TestFileStorage_CreateDevice(t *testing.T) {
	formats := []string{"json", "toml"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			storage := setupTestStorage(t, format)

			device := &model.Device{
				ID:          "test-device-1",
				Name:        "Test Device",
				Description: "Test Description",
				MakeModel:   "Test Model",
				OS:          "Test OS",
				Location:    "Test Location",
				Tags:        []string{"test", "unit"},
				Domains:     []string{"example.com"},
				Addresses: []model.Address{
					{IP: "192.168.1.1", Port: 8080, Type: "ipv4", Label: "eth0"},
				},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			err := storage.CreateDevice(device)
			if err != nil {
				t.Fatalf("CreateDevice() error = %v", err)
			}

			// Verify device was stored
			retrieved, err := storage.GetDevice("test-device-1")
			if err != nil {
				t.Fatalf("GetDevice() error = %v", err)
			}

			if retrieved.Name != device.Name {
				t.Errorf("Expected name %s, got %s", device.Name, retrieved.Name)
			}

			if retrieved.Description != device.Description {
				t.Errorf("Expected description %s, got %s", device.Description, retrieved.Description)
			}

			if len(retrieved.Tags) != 2 {
				t.Errorf("Expected 2 tags, got %d", len(retrieved.Tags))
			}

			if len(retrieved.Addresses) != 1 {
				t.Errorf("Expected 1 address, got %d", len(retrieved.Addresses))
			}

			// Verify file was created
			devicePath := storage.devicePath("test-device-1")
			if _, err := os.Stat(devicePath); os.IsNotExist(err) {
				t.Errorf("Device file was not created at %s", devicePath)
			}
		})
	}
}

func TestFileStorage_CreateDevice_Duplicate(t *testing.T) {
	storage := setupTestStorage(t, "json")

	device := &model.Device{
		ID:        "test-device-1",
		Name:      "Test Device",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// First creation should succeed
	err := storage.CreateDevice(device)
	if err != nil {
		t.Fatalf("First CreateDevice() error = %v", err)
	}

	// Second creation with same ID should fail
	err = storage.CreateDevice(device)
	if err == nil {
		t.Error("Expected error when creating duplicate device, got nil")
	}
}

func TestFileStorage_GetDevice(t *testing.T) {
	storage := setupTestStorage(t, "json")

	device := &model.Device{
		ID:        "get-test-1",
		Name:      "Get Test Device",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	storage.CreateDevice(device)

	tests := []struct {
		name    string
		id      string
		wantErr bool
		errMsg  string
	}{
		{"Existing device by ID", "get-test-1", false, ""},
		{"Non-existent device", "non-existent", true, "device not found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retrieved, err := storage.GetDevice(tt.id)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetDevice() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && retrieved.Name != device.Name {
				t.Errorf("Expected name %s, got %s", device.Name, retrieved.Name)
			}
		})
	}
}

func TestFileStorage_GetDevice_ByName(t *testing.T) {
	storage := setupTestStorage(t, "json")

	device := &model.Device{
		ID:        "get-by-name-1",
		Name:      "Unique Device Name",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	storage.CreateDevice(device)

	// Get by name (case-insensitive)
	retrieved, err := storage.GetDevice("unique device name")
	if err != nil {
		t.Fatalf("GetDevice() by name error = %v", err)
	}

	if retrieved.ID != device.ID {
		t.Errorf("Expected ID %s, got %s", device.ID, retrieved.ID)
	}
}

func TestFileStorage_UpdateDevice(t *testing.T) {
	storage := setupTestStorage(t, "json")

	device := &model.Device{
		ID:          "update-test-1",
		Name:        "Original Name",
		Description: "Original Description",
		OS:          "Original OS",
		Tags:        []string{"original"},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	storage.CreateDevice(device)

	// Get the device to get a fresh copy with the stored timestamp
	retrievedBefore, _ := storage.GetDevice("update-test-1")
	originalUpdatedAt := retrievedBefore.UpdatedAt

	time.Sleep(10 * time.Millisecond) // Ensure timestamp difference

	// Update the device
	device.Name = "Updated Name"
	device.Description = "Updated Description"
	device.Tags = []string{"updated", "device"}

	err := storage.UpdateDevice(device)
	if err != nil {
		t.Fatalf("UpdateDevice() error = %v", err)
	}

	// Verify update
	retrieved, _ := storage.GetDevice("update-test-1")

	if retrieved.Name != "Updated Name" {
		t.Errorf("Expected name 'Updated Name', got %s", retrieved.Name)
	}

	if retrieved.Description != "Updated Description" {
		t.Errorf("Expected description 'Updated Description', got %s", retrieved.Description)
	}

	if len(retrieved.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(retrieved.Tags))
	}

	// Verify UpdatedAt changed
	if !retrieved.UpdatedAt.After(originalUpdatedAt) {
		t.Error("UpdatedAt should have been updated")
	}
}

func TestFileStorage_DeleteDevice(t *testing.T) {
	storage := setupTestStorage(t, "json")

	device := &model.Device{
		ID:        "delete-test-1",
		Name:      "Delete Test Device",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	storage.CreateDevice(device)

	// Delete the device
	err := storage.DeleteDevice("delete-test-1")
	if err != nil {
		t.Fatalf("DeleteDevice() error = %v", err)
	}

	// Verify it's gone
	_, err = storage.GetDevice("delete-test-1")
	if err != ErrDeviceNotFound {
		t.Errorf("Expected ErrDeviceNotFound, got %v", err)
	}

	// Verify file was removed
	devicePath := storage.devicePath("delete-test-1")
	if _, err := os.Stat(devicePath); !os.IsNotExist(err) {
		t.Error("Device file should have been deleted")
	}
}

func TestFileStorage_ListDevices(t *testing.T) {
	storage := setupTestStorage(t, "json")

	devices := []*model.Device{
		{
			ID:        "list-1",
			Name:      "Device 1",
			Tags:      []string{"server", "production"},
			Location:  "Rack A1",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "list-2",
			Name:      "Device 2",
			Tags:      []string{"server", "development"},
			Location:  "Rack A2",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "list-3",
			Name:      "Device 3",
			Tags:      []string{"workstation"},
			Location:  "Rack B1",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	for _, d := range devices {
		storage.CreateDevice(d)
	}

	// List all devices
	allDevices, err := storage.ListDevices(nil)
	if err != nil {
		t.Fatalf("ListDevices() error = %v", err)
	}

	if len(allDevices) != 3 {
		t.Errorf("Expected 3 devices, got %d", len(allDevices))
	}

	// Filter by tag
	filter := &model.DeviceFilter{Tags: []string{"server"}}
	filtered, err := storage.ListDevices(filter)
	if err != nil {
		t.Fatalf("ListDevices() with filter error = %v", err)
	}

	if len(filtered) != 2 {
		t.Errorf("Expected 2 devices with 'server' tag, got %d", len(filtered))
	}

	// Filter by multiple tags (OR logic)
	filter = &model.DeviceFilter{Tags: []string{"production", "workstation"}}
	filtered, err = storage.ListDevices(filter)
	if err != nil {
		t.Fatalf("ListDevices() with multiple tags error = %v", err)
	}

	if len(filtered) != 2 {
		t.Errorf("Expected 2 devices with 'production' or 'workstation' tag, got %d", len(filtered))
	}

	// Filter with non-matching tag
	filter = &model.DeviceFilter{Tags: []string{"nonexistent"}}
	filtered, err = storage.ListDevices(filter)
	if err != nil {
		t.Fatalf("ListDevices() with non-matching tag error = %v", err)
	}

	if len(filtered) != 0 {
		t.Errorf("Expected 0 devices with 'nonexistent' tag, got %d", len(filtered))
	}
}

func TestFileStorage_SearchDevices(t *testing.T) {
	storage := setupTestStorage(t, "json")

	devices := []*model.Device{
		{
			ID:          "search-1",
			Name:        "Web Server",
			Description: "Main web server",
			MakeModel:   "Dell R740",
			OS:          "Ubuntu 22.04",
			Location:    "Rack A1",
			Tags:        []string{"server", "production"},
			Domains:     []string{"example.com"},
			Addresses: []model.Address{
				{IP: "192.168.1.10", Port: 443, Type: "ipv4"},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "search-2",
			Name:        "Database Server",
			Description: "PostgreSQL database",
			MakeModel:   "HP DL380",
			OS:          "CentOS 8",
			Location:    "Rack A2",
			Tags:        []string{"server", "database"},
			Domains:     []string{"db.example.com"},
			Addresses: []model.Address{
				{IP: "192.168.1.20", Port: 5432, Type: "ipv4"},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "search-3",
			Name:        "Developer Workstation",
			Description: "Development machine",
			MakeModel:   "MacBook Pro",
			OS:          "macOS",
			Location:    "Office",
			Tags:        []string{"workstation"},
			Addresses: []model.Address{
				{IP: "192.168.2.50", Port: 22, Type: "ipv4"},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	for _, d := range devices {
		storage.CreateDevice(d)
	}

	tests := []struct {
		name        string
		query       string
		wantCount   int
		wantIDs     []string
		description string
	}{
		{"Search by name - partial", "web", 1, []string{"search-1"}, "Find devices with 'web' in name"},
		{"Search by name - Server", "Server", 2, []string{"search-1", "search-2"}, "Find devices with 'Server' in name"},
		{"Search by IP", "192.168.1", 2, []string{"search-1", "search-2"}, "Find devices by IP prefix"},
		{"Search by tag", "production", 1, []string{"search-1"}, "Find devices by tag"},
		{"Search by make/model", "Dell", 1, []string{"search-1"}, "Find devices by make/model"},
		{"Search by OS", "Ubuntu", 1, []string{"search-1"}, "Find devices by OS"},
		{"Search by location", "Rack", 2, []string{"search-1", "search-2"}, "Find devices by location"},
		{"Search by domain", "example.com", 2, []string{"search-1", "search-2"}, "Find devices by domain"},
		{"Search by description", "database", 1, []string{"search-2"}, "Find devices by description"},
		{"Search - no results", "nonexistent", 0, nil, "No matches found"},
		{"Search - case insensitive", "SERVER", 2, []string{"search-1", "search-2"}, "Case insensitive search"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := storage.SearchDevices(tt.query)
			if err != nil {
				t.Fatalf("SearchDevices() error = %v", err)
			}

			if len(results) != tt.wantCount {
				t.Errorf("%s: expected %d results, got %d", tt.description, tt.wantCount, len(results))
			}

			if tt.wantIDs != nil {
				foundIDs := make(map[string]bool)
				for _, r := range results {
					foundIDs[r.ID] = true
				}
				for _, wantID := range tt.wantIDs {
					if !foundIDs[wantID] {
						t.Errorf("%s: expected to find device %s", tt.description, wantID)
					}
				}
			}
		})
	}
}

func TestFileStorage_Persistence(t *testing.T) {
	formats := []string{"json", "toml"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Create first storage instance and add a device
			storage1, err := NewFileStorage(tmpDir, format)
			if err != nil {
				t.Fatalf("Failed to create storage: %v", err)
			}

			device := &model.Device{
				ID:          "persist-1",
				Name:        "Persistent Device",
				Description: "This should persist",
				Tags:        []string{"persistent"},
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}

			err = storage1.CreateDevice(device)
			if err != nil {
				t.Fatalf("CreateDevice() error = %v", err)
			}

			// Create new storage instance (simulating restart)
			storage2, err := NewFileStorage(tmpDir, format)
			if err != nil {
				t.Fatalf("Failed to create second storage: %v", err)
			}

			// Verify device was loaded
			retrieved, err := storage2.GetDevice("persist-1")
			if err != nil {
				t.Fatalf("GetDevice() from new storage error = %v", err)
			}

			if retrieved.Name != device.Name {
				t.Errorf("Expected name %s, got %s", device.Name, retrieved.Name)
			}

			if retrieved.Description != device.Description {
				t.Errorf("Expected description %s, got %s", device.Description, retrieved.Description)
			}

			if len(retrieved.Tags) != 1 {
				t.Errorf("Expected 1 tag, got %d", len(retrieved.Tags))
			}
		})
	}
}

func TestFileStorage_Backup(t *testing.T) {
	storage := setupTestStorage(t, "json")

	device := &model.Device{
		ID:        "backup-test-1",
		Name:      "Backup Test",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	storage.CreateDevice(device)

	// Update the device
	device.Name = "Updated Name"
	err := storage.UpdateDevice(device)
	if err != nil {
		t.Fatalf("UpdateDevice() error = %v", err)
	}

	// Check if backup file exists (if implemented)
	// This test verifies the saveFile temp file behavior
	devicePath := storage.devicePath("backup-test-1")
	tmpPath := devicePath + ".tmp"

	// Temp file should be cleaned up after successful save
	if _, err := os.Stat(tmpPath); err == nil {
		t.Error("Temp file should be cleaned up after successful save")
	}
}

func TestFileStorage_EmptyStorage(t *testing.T) {
	storage := setupTestStorage(t, "json")

	// List on empty storage
	devices, err := storage.ListDevices(nil)
	if err != nil {
		t.Fatalf("ListDevices() on empty storage error = %v", err)
	}

	if len(devices) != 0 {
		t.Errorf("Expected 0 devices, got %d", len(devices))
	}

	// Search on empty storage
	results, err := storage.SearchDevices("test")
	if err != nil {
		t.Fatalf("SearchDevices() on empty storage error = %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}
}

func TestFileStorage_InvalidID(t *testing.T) {
	storage := setupTestStorage(t, "json")

	device := &model.Device{
		ID:        "",
		Name:      "Invalid Device",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := storage.CreateDevice(device)
	if err != ErrInvalidID {
		t.Errorf("Expected ErrInvalidID for empty ID, got %v", err)
	}
}

func TestFileStorage_ConcurrentAccess(t *testing.T) {
	storage := setupTestStorage(t, "json")

	done := make(chan bool)

	// Concurrent creates
	for i := 0; i < 10; i++ {
		go func(idx int) {
			device := &model.Device{
				ID:        fmt.Sprintf("concurrent-%d", idx),
				Name:      fmt.Sprintf("Concurrent Device %d", idx),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			storage.CreateDevice(device)
			done <- true
		}(i)
	}

	// Wait for all creates
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all devices were created
	devices, _ := storage.ListDevices(nil)
	if len(devices) != 10 {
		t.Errorf("Expected 10 devices, got %d", len(devices))
	}
}
