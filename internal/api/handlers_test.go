package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/martinsuchenak/rackd/internal/model"
	"github.com/martinsuchenak/rackd/internal/storage"
)

// mockStorage is a simple in-memory storage for testing
type mockStorage struct {
	devices     map[string]*model.Device
	datacenters map[string]*model.Datacenter
	networks    map[string]*model.Network
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		devices:     make(map[string]*model.Device),
		datacenters: make(map[string]*model.Datacenter),
		networks:    make(map[string]*model.Network),
	}
}

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
	return nil, nil
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
	return nil, nil
}

func setupTestHandler() *Handler {
	return NewHandler(newMockStorage())
}

func TestHandler_ListDevices(t *testing.T) {
	handler := setupTestHandler()

	req := httptest.NewRequest("GET", "/api/devices", nil)
	w := httptest.NewRecorder()

	handler.listDevices(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var devices []model.Device
	if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(devices) != 0 {
		t.Errorf("Expected 0 devices, got %d", len(devices))
	}
}

func TestHandler_CreateDevice(t *testing.T) {
	handler := setupTestHandler()

	// First create a datacenter
	datacenterJSON := `{
		"name": "Test DC",
		"location": "San Francisco",
		"description": "A test datacenter"
	}`

	req := httptest.NewRequest("POST", "/api/datacenters", bytes.NewReader([]byte(datacenterJSON)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.createDatacenter(w, req)

	var datacenter model.Datacenter
	if err := json.NewDecoder(w.Result().Body).Decode(&datacenter); err != nil {
		t.Fatalf("Failed to decode datacenter response: %v", err)
	}

	// Now create a device with the datacenter
	deviceJSON := fmt.Sprintf(`{
		"name": "Test Server",
		"description": "A test server",
		"make_model": "Dell R740",
		"os": "Ubuntu 22.04",
		"datacenter_id": "%s",
		"tags": ["server", "test"],
		"domains": ["example.com"],
		"addresses": [
			{
				"ip": "192.168.1.10",
				"port": 443,
				"type": "ipv4",
				"label": "management"
			}
		]
	}`, datacenter.ID)

	req = httptest.NewRequest("POST", "/api/devices", bytes.NewReader([]byte(deviceJSON)))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	handler.createDevice(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}

	var device model.Device
	if err := json.NewDecoder(resp.Body).Decode(&device); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if device.Name != "Test Server" {
		t.Errorf("Expected name 'Test Server', got %s", device.Name)
	}

	if device.MakeModel != "Dell R740" {
		t.Errorf("Expected make_model 'Dell R740', got %s", device.MakeModel)
	}

	if len(device.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(device.Tags))
	}

	if len(device.Addresses) != 1 {
		t.Errorf("Expected 1 address, got %d", len(device.Addresses))
	}
}

func TestHandler_CreateDevice_MissingName(t *testing.T) {
	handler := setupTestHandler()

	deviceJSON := `{
		"description": "A test server"
	}`

	req := httptest.NewRequest("POST", "/api/devices", bytes.NewReader([]byte(deviceJSON)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.createDevice(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}

	var errResp map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if errResp["error"] == "" {
		t.Error("Expected error message in response")
	}
}

func TestHandler_CreateDevice_InvalidJSON(t *testing.T) {
	handler := setupTestHandler()

	req := httptest.NewRequest("POST", "/api/devices", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.createDevice(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestHandler_GetDevice(t *testing.T) {
	handler := setupTestHandler()

	// First create a device
	storage := handler.storage.(*mockStorage)
	device := &model.Device{
		ID:          "get-test-1",
		Name:        "Get Test Device",
		Description: "Test description",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	storage.CreateDevice(device)

	// Now get it
	req := httptest.NewRequest("GET", "/api/devices/get-test-1", nil)
	req.SetPathValue("id", "get-test-1")
	w := httptest.NewRecorder()

	handler.getDevice(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var retrieved model.Device
	if err := json.NewDecoder(resp.Body).Decode(&retrieved); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if retrieved.Name != device.Name {
		t.Errorf("Expected name '%s', got %s", device.Name, retrieved.Name)
	}
}

func TestHandler_GetDevice_NotFound(t *testing.T) {
	handler := setupTestHandler()

	req := httptest.NewRequest("GET", "/api/devices/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	handler.getDevice(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

func TestHandler_UpdateDevice(t *testing.T) {
	handler := setupTestHandler()

	// First create a device
	storage := handler.storage.(*mockStorage)
	device := &model.Device{
		ID:        "update-test-1",
		Name:      "Original Name",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	storage.CreateDevice(device)

	// Update it
	updateJSON := `{
		"name": "Updated Name",
		"tags": ["updated"]
	}`

	req := httptest.NewRequest("PUT", "/api/devices/update-test-1", bytes.NewReader([]byte(updateJSON)))
	req.SetPathValue("id", "update-test-1")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.updateDevice(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	var updated model.Device
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if updated.Name != "Updated Name" {
		t.Errorf("Expected name 'Updated Name', got %s", updated.Name)
	}
}

func TestHandler_DeleteDevice(t *testing.T) {
	handler := setupTestHandler()

	// First create a device
	mockStore := handler.storage.(*mockStorage)
	device := &model.Device{
		ID:        "delete-test-1",
		Name:      "Delete Test Device",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mockStore.CreateDevice(device)

	// Delete it
	req := httptest.NewRequest("DELETE", "/api/devices/delete-test-1", nil)
	req.SetPathValue("id", "delete-test-1")
	w := httptest.NewRecorder()

	handler.deleteDevice(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", resp.StatusCode)
	}

	// Verify it's gone
	_, err := mockStore.GetDevice("delete-test-1")
	if err != storage.ErrDeviceNotFound {
		t.Error("Device should have been deleted")
	}
}

func TestHandler_DeleteDevice_NotFound(t *testing.T) {
	handler := setupTestHandler()

	req := httptest.NewRequest("DELETE", "/api/devices/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	handler.deleteDevice(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

func TestHandler_SearchDevices(t *testing.T) {
	handler := setupTestHandler()

	// Create some devices
	storage := handler.storage.(*mockStorage)
	devices := []*model.Device{
		{ID: "search-1", Name: "Web Server", Tags: []string{"web"}, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "search-2", Name: "Database Server", Tags: []string{"database"}, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	for _, d := range devices {
		storage.CreateDevice(d)
	}

	req := httptest.NewRequest("GET", "/api/search?q=server", nil)
	w := httptest.NewRecorder()

	handler.searchDevices(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var results []model.Device
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Mock search returns all devices, so we expect 2
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
}

func TestHandler_SearchDevices_MissingQuery(t *testing.T) {
	handler := setupTestHandler()

	req := httptest.NewRequest("GET", "/api/search", nil)
	w := httptest.NewRecorder()

	handler.searchDevices(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestHandler_RegisterRoutes(t *testing.T) {
	handler := setupTestHandler()

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	server := httptest.NewServer(mux)
	defer server.Close()

	// Test list devices
	resp, err := http.Get(server.URL + "/api/devices")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Test search
	resp, err = http.Get(server.URL + "/api/search?q=test")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestHandler_Integration_CreateGetUpdateDelete(t *testing.T) {
	handler := setupTestHandler()

	// Create
	deviceJSON := `{
		"name": "Integration Test Device",
		"description": "Testing full lifecycle"
	}`

	createReq := httptest.NewRequest("POST", "/api/devices", bytes.NewReader([]byte(deviceJSON)))
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()

	handler.createDevice(createW, createReq)
	createResp := createW.Result()
	defer createResp.Body.Close()

	if createResp.StatusCode != http.StatusCreated {
		t.Errorf("Create failed: status %d", createResp.StatusCode)
	}

	var created model.Device
	json.NewDecoder(createResp.Body).Decode(&created)

	// Get
	getReq := httptest.NewRequest("GET", "/api/devices/"+created.ID, nil)
	getReq.SetPathValue("id", created.ID)
	getW := httptest.NewRecorder()
	handler.getDevice(getW, getReq)

	getResp := getW.Result()
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		t.Errorf("Get failed: status %d", getResp.StatusCode)
	}

	// Update
	updateJSON := `{"name": "Updated Integration Device"}`
	updateReq := httptest.NewRequest("PUT", "/api/devices/"+created.ID, bytes.NewReader([]byte(updateJSON)))
	updateReq.SetPathValue("id", created.ID)
	updateReq.Header.Set("Content-Type", "application/json")
	updateW := httptest.NewRecorder()

	handler.updateDevice(updateW, updateReq)
	updateResp := updateW.Result()
	defer updateResp.Body.Close()

	if updateResp.StatusCode != http.StatusOK {
		t.Errorf("Update failed: status %d", updateResp.StatusCode)
	}

	// Delete
	deleteReq := httptest.NewRequest("DELETE", "/api/devices/"+created.ID, nil)
	deleteReq.SetPathValue("id", created.ID)
	deleteW := httptest.NewRecorder()

	handler.deleteDevice(deleteW, deleteReq)
	deleteResp := deleteW.Result()
	defer deleteResp.Body.Close()

	if deleteResp.StatusCode != http.StatusNoContent {
		t.Errorf("Delete failed: status %d", deleteResp.StatusCode)
	}
}
