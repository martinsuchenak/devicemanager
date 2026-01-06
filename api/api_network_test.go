package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// TestAPI_NetworkCRUD tests the full network lifecycle
func TestAPI_NetworkCRUD(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	var dcID string
	var netID string

	// Create a datacenter first
	dcPayload := map[string]interface{}{"name": "Network DC"}
	dcData, _ := json.Marshal(dcPayload)
	resp, _ := http.Post(ts.URL()+"/api/datacenters", "application/json", bytes.NewReader(dcData))
	var dc map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&dc)
	resp.Body.Close()
	dcID = dc["id"].(string)

	// 1. Create Network
	t.Run("CreateNetwork", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":          "LAN",
			"subnet":        "192.168.1.0/24",
			"gateway":       "192.168.1.1",
			"vlan":          10,
			"datacenter_id": dcID,
		}
		data, _ := json.Marshal(payload)

		resp, err := http.Post(ts.URL()+"/api/networks", "application/json", bytes.NewReader(data))
		if err != nil {
			t.Fatalf("Failed to create network: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 201, got %d: %s", resp.StatusCode, string(body))
		}

		var network map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&network); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		netID = network["id"].(string)
		if netID == "" {
			t.Error("Expected network ID to be set")
		}
	})

	// 2. Read Network
	t.Run("GetNetwork", func(t *testing.T) {
		resp, err := http.Get(ts.URL() + "/api/networks/" + netID)
		if err != nil {
			t.Fatalf("Failed to get network: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var network map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&network); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if network["id"] != netID {
			t.Errorf("Expected ID %s, got %v", netID, network["id"])
		}
	})

	// 3. Update Network
	t.Run("UpdateNetwork", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":          "LAN Updated",
			"subnet":        "192.168.1.0/24",
			"datacenter_id": dcID,
		}
		data, _ := json.Marshal(payload)

		req, err := http.NewRequest("PUT", ts.URL()+"/api/networks/"+netID, bytes.NewReader(data))
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to update network: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var network map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&network)

		if network["name"] != "LAN Updated" {
			t.Errorf("Expected updated name, got %v", network["name"])
		}
	})

	// 4. List Networks
	t.Run("ListNetworks", func(t *testing.T) {
		resp, err := http.Get(ts.URL() + "/api/networks")
		if err != nil {
			t.Fatalf("Failed to list networks: %v", err)
		}
		defer resp.Body.Close()

		var result []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(result) != 1 {
			t.Errorf("Expected 1 network, got %d", len(result))
		}
	})

	// 5. Delete Network
	t.Run("DeleteNetwork", func(t *testing.T) {
		req, err := http.NewRequest("DELETE", ts.URL()+"/api/networks/"+netID, nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to delete network: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("Expected status 204, got %d", resp.StatusCode)
		}
	})
}
