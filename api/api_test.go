package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/martinsuchenak/rackd/internal/api"
	"github.com/martinsuchenak/rackd/internal/storage"
)

// TestServer is a helper for integration tests
type TestServer struct {
	server   *httptest.Server
	handler  *api.Handler
	storage  storage.Storage
	stopChan chan struct{}
}

// NewTestServer creates a new test server
func NewTestServer(t *testing.T) *TestServer {
	t.Helper()

	tmpDir := t.TempDir()

	store, err := storage.NewSQLiteStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	handler := api.NewHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	server := httptest.NewServer(mux)

	return &TestServer{
		server:  server,
		handler: handler,
		storage: store,
	}
}

// Close stops the test server
func (ts *TestServer) Close() {
	if ts.server != nil {
		ts.server.Close()
	}
}

// URL returns the base URL of the test server
func (ts *TestServer) URL() string {
	return ts.server.URL
}

// DeviceJSON is a helper for creating device JSON
func DeviceJSON(name string, extra map[string]interface{}) []byte {
	device := map[string]interface{}{
		"name": name,
	}
	for k, v := range extra {
		device[k] = v
	}
	data, _ := json.Marshal(device)
	return data
}

// TestAPI_Integration_CreateReadUpdateDelete tests the full device lifecycle
func TestAPI_Integration_CreateReadUpdateDelete(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	var deviceID string

	// 1. Create a device
	t.Run("CreateDevice", func(t *testing.T) {
		// First create a datacenter
		dcPayload := DeviceJSON("Test DC", map[string]interface{}{
			"location":    "San Francisco",
			"description": "A test datacenter",
		})
		dcResp, err := http.Post(ts.URL()+"/api/datacenters", "application/json", bytes.NewReader(dcPayload))
		if err != nil {
			t.Fatalf("Failed to create datacenter: %v", err)
		}
		defer dcResp.Body.Close()

		var datacenter map[string]interface{}
		if err := json.NewDecoder(dcResp.Body).Decode(&datacenter); err != nil {
			t.Fatalf("Failed to decode datacenter response: %v", err)
		}

		payload := DeviceJSON("Integration Test Server", map[string]interface{}{
			"description":   "Test server for integration testing",
			"make_model":    "Dell R740",
			"os":            "Ubuntu 22.04",
			"datacenter_id": datacenter["id"].(string),
			"tags":          []string{"test", "integration"},
			"addresses": []map[string]interface{}{
				{
					"ip":    "192.168.1.100",
					"port":  443,
					"type":  "ipv4",
					"label": "management",
				},
			},
		})

		resp, err := http.Post(ts.URL()+"/api/devices", "application/json", bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("Failed to create device: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 201, got %d: %s", resp.StatusCode, string(body))
		}

		var device map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&device); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		deviceID = device["id"].(string)
		if deviceID == "" {
			t.Error("Expected device ID to be set")
		}

		if device["name"] != "Integration Test Server" {
			t.Errorf("Expected name 'Integration Test Server', got %v", device["name"])
		}
	})

	// 2. Read the device
	t.Run("GetDevice", func(t *testing.T) {
		resp, err := http.Get(ts.URL() + "/api/devices/" + deviceID)
		if err != nil {
			t.Fatalf("Failed to get device: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var device map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&device); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if device["id"] != deviceID {
			t.Errorf("Expected ID %s, got %v", deviceID, device["id"])
		}
	})

	// 3. Update the device
	t.Run("UpdateDevice", func(t *testing.T) {
		payload := DeviceJSON("Updated Server Name", map[string]interface{}{
			"tags": []string{"test", "integration", "updated"},
		})

		req, err := http.NewRequest("PUT", ts.URL()+"/api/devices/"+deviceID, bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to update device: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
		}

		var device map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&device)

		if device["name"] != "Updated Server Name" {
			t.Errorf("Expected updated name, got %v", device["name"])
		}
	})

	// 4. Delete the device
	t.Run("DeleteDevice", func(t *testing.T) {
		req, err := http.NewRequest("DELETE", ts.URL()+"/api/devices/"+deviceID, nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to delete device: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("Expected status 204, got %d", resp.StatusCode)
		}
	})

	// 5. Verify device is gone
	t.Run("VerifyDeleted", func(t *testing.T) {
		resp, err := http.Get(ts.URL() + "/api/devices/" + deviceID)
		if err != nil {
			t.Fatalf("Failed to get device: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", resp.StatusCode)
		}
	})
}

// TestAPI_ListDevices tests listing devices with filtering
func TestAPI_ListDevices(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	// Create multiple devices
	devices := []map[string]interface{}{
		{"name": "Server 1", "tags": []string{"server", "production"}},
		{"name": "Server 2", "tags": []string{"server", "development"}},
		{"name": "Workstation 1", "tags": []string{"workstation"}},
	}

	for _, d := range devices {
		payload, _ := json.Marshal(d)
		resp, err := http.Post(ts.URL()+"/api/devices", "application/json", bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("Failed to create device: %v", err)
		}
		resp.Body.Close()
	}

	// List all devices
	t.Run("ListAll", func(t *testing.T) {
		resp, err := http.Get(ts.URL() + "/api/devices")
		if err != nil {
			t.Fatalf("Failed to list devices: %v", err)
		}
		defer resp.Body.Close()

		var result []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(result) != 3 {
			t.Errorf("Expected 3 devices, got %d", len(result))
		}
	})

	// Filter by tag
	t.Run("FilterByTag", func(t *testing.T) {
		resp, err := http.Get(ts.URL() + "/api/devices?tag=server")
		if err != nil {
			t.Fatalf("Failed to list devices: %v", err)
		}
		defer resp.Body.Close()

		var result []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(result) != 2 {
			t.Errorf("Expected 2 devices with 'server' tag, got %d", len(result))
		}
	})
}

// TestAPI_SearchDevices tests the search functionality
func TestAPI_SearchDevices(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	// Create devices with searchable content
	devices := []map[string]interface{}{
		{"name": "Web Server", "description": "Apache web server"},
		{"name": "Database Server", "description": "PostgreSQL database"},
		{"name": "Mail Server", "description": "Postfix mail server"},
	}

	for _, d := range devices {
		payload, _ := json.Marshal(d)
		resp, err := http.Post(ts.URL()+"/api/devices", "application/json", bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("Failed to create device: %v", err)
		}
		resp.Body.Close()
	}

	tests := []struct {
		name      string
		query     string
		minCount  int
		maxCount  int
	}{
		{"Search by name", "server", 2, 3},
		{"Search by description", "postgresql", 1, 1},
		{"Search no results", "nonexistent", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Get(ts.URL() + "/api/search?q=" + tt.query)
			if err != nil {
				t.Fatalf("Failed to search: %v", err)
			}
			defer resp.Body.Close()

			var result []map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if len(result) < tt.minCount || len(result) > tt.maxCount {
				t.Errorf("Expected %d-%d results, got %d", tt.minCount, tt.maxCount, len(result))
			}
		})
	}
}

// TestAPI_ErrorHandling tests various error conditions
func TestAPI_ErrorHandling(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	t.Run("CreateMissingName", func(t *testing.T) {
		payload := `{"description": "No name provided"}`
		resp, err := http.Post(ts.URL()+"/api/devices", "application/json", bytes.NewReader([]byte(payload)))
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("CreateInvalidJSON", func(t *testing.T) {
		resp, err := http.Post(ts.URL()+"/api/devices", "application/json", bytes.NewReader([]byte("invalid")))
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("GetNotFound", func(t *testing.T) {
		resp, err := http.Get(ts.URL() + "/api/devices/nonexistent-id")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", resp.StatusCode)
		}
	})

	t.Run("UpdateNotFound", func(t *testing.T) {
		payload := DeviceJSON("Updated Name", nil)
		req, err := http.NewRequest("PUT", ts.URL()+"/api/devices/nonexistent-id", bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", resp.StatusCode)
		}
	})

	t.Run("DeleteNotFound", func(t *testing.T) {
		req, err := http.NewRequest("DELETE", ts.URL()+"/api/devices/nonexistent-id", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", resp.StatusCode)
		}
	})

	t.Run("SearchMissingQuery", func(t *testing.T) {
		resp, err := http.Get(ts.URL() + "/api/search")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})
}

// TestAPI_ConcurrentRequests tests concurrent API access
func TestAPI_ConcurrentRequests(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	done := make(chan bool, 10)

	// Create 10 devices concurrently
	for i := 0; i < 10; i++ {
		go func(idx int) {
			payload := DeviceJSON(fmt.Sprintf("Concurrent Device %d", idx), nil)
			resp, err := http.Post(ts.URL()+"/api/devices", "application/json", bytes.NewReader(payload))
			if err != nil {
				t.Errorf("Request failed: %v", err)
			}
			if resp != nil {
				resp.Body.Close()
			}
			done <- true
		}(i)
	}

	// Wait for all requests to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all devices were created
	resp, err := http.Get(ts.URL() + "/api/devices")
	if err != nil {
		t.Fatalf("Failed to list devices: %v", err)
	}
	defer resp.Body.Close()

	var result []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(result) != 10 {
		t.Errorf("Expected 10 devices, got %d", len(result))
	}
}

// TestAPI_LargePayload tests handling of large payloads
func TestAPI_LargePayload(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	// Create a device with many addresses
	addresses := make([]map[string]interface{}, 100)
	for i := 0; i < 100; i++ {
		addresses[i] = map[string]interface{}{
			"ip":    fmt.Sprintf("192.168.1.%d", i),
			"port":  8080 + i,
			"type":  "ipv4",
			"label": fmt.Sprintf("interface%d", i),
		}
	}

	payload := DeviceJSON("Large Payload Device", map[string]interface{}{
		"description": "Device with many addresses",
		"addresses":   addresses,
	})

	resp, err := http.Post(ts.URL()+"/api/devices", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("Failed to create device: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}

	var device map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&device)

	// Verify addresses were stored
	addrs, ok := device["addresses"].([]interface{})
	if !ok || len(addrs) != 100 {
		t.Errorf("Expected 100 addresses, got %v", len(addrs))
	}
}

// BenchmarkAPI_CreateDevice benchmarks device creation
func BenchmarkAPI_CreateDevice(b *testing.B) {
	ts := NewTestServer(&testing.T{})
	defer ts.Close()

	payload := DeviceJSON("Benchmark Device", map[string]interface{}{
		"tags": []string{"benchmark", "test"},
	})

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		resp, err := http.Post(ts.URL()+"/api/devices", "application/json", bytes.NewReader(payload))
		if err != nil {
			b.Fatalf("Request failed: %v", err)
		}
		resp.Body.Close()
	}
}

// BenchmarkAPI_ListDevices benchmarks listing devices
func BenchmarkAPI_ListDevices(b *testing.B) {
	ts := NewTestServer(&testing.T{})
	defer ts.Close()

	// Pre-create some devices
	for i := 0; i < 100; i++ {
		payload := DeviceJSON(fmt.Sprintf("Device %d", i), nil)
		http.Post(ts.URL()+"/api/devices", "application/json", bytes.NewReader(payload))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		resp, err := http.Get(ts.URL() + "/api/devices")
		if err != nil {
			b.Fatalf("Request failed: %v", err)
		}
		resp.Body.Close()
	}
}

// BenchmarkAPI_SearchDevices benchmarks search
func BenchmarkAPI_SearchDevices(b *testing.B) {
	ts := NewTestServer(&testing.T{})
	defer ts.Close()

	// Pre-create some devices
	for i := 0; i < 100; i++ {
		payload := DeviceJSON(fmt.Sprintf("Server %d", i), map[string]interface{}{
			"tags": []string{"server"},
		})
		http.Post(ts.URL()+"/api/devices", "application/json", bytes.NewReader(payload))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		resp, err := http.Get(ts.URL() + "/api/search?q=server")
		if err != nil {
			b.Fatalf("Request failed: %v", err)
		}
		resp.Body.Close()
	}
}
