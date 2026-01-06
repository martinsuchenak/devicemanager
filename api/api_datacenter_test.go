package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// TestAPI_DatacenterCRUD tests the full datacenter lifecycle
func TestAPI_DatacenterCRUD(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	var dcID string

	// 1. Create Datacenter
	t.Run("CreateDatacenter", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":        "Primary DC",
			"location":    "New York",
			"description": "Main datacenter",
		}
		data, _ := json.Marshal(payload)

		resp, err := http.Post(ts.URL()+"/api/datacenters", "application/json", bytes.NewReader(data))
		if err != nil {
			t.Fatalf("Failed to create datacenter: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 201, got %d: %s", resp.StatusCode, string(body))
		}

		var dc map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&dc); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		dcID = dc["id"].(string)
		if dcID == "" {
			t.Error("Expected datacenter ID to be set")
		}
	})

	// 2. Read Datacenter
	t.Run("GetDatacenter", func(t *testing.T) {
		resp, err := http.Get(ts.URL() + "/api/datacenters/" + dcID)
		if err != nil {
			t.Fatalf("Failed to get datacenter: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var dc map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&dc); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if dc["id"] != dcID {
			t.Errorf("Expected ID %s, got %v", dcID, dc["id"])
		}
	})

	// 3. Update Datacenter
	t.Run("UpdateDatacenter", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":        "Primary DC Updated",
			"location":    "New York",
			"description": "Updated description",
		}
		data, _ := json.Marshal(payload)

		req, err := http.NewRequest("PUT", ts.URL()+"/api/datacenters/"+dcID, bytes.NewReader(data))
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to update datacenter: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var dc map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&dc)

		if dc["name"] != "Primary DC Updated" {
			t.Errorf("Expected updated name, got %v", dc["name"])
		}
	})

	// 4. List Datacenters
	t.Run("ListDatacenters", func(t *testing.T) {
		resp, err := http.Get(ts.URL() + "/api/datacenters")
		if err != nil {
			t.Fatalf("Failed to list datacenters: %v", err)
		}
		defer resp.Body.Close()

		var result []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// We expect 2 datacenters: the default one and the one we created
		if len(result) != 2 {
			t.Errorf("Expected 2 datacenters, got %d", len(result))
		}
	})

	// 5. Delete Datacenter
	t.Run("DeleteDatacenter", func(t *testing.T) {
		req, err := http.NewRequest("DELETE", ts.URL()+"/api/datacenters/"+dcID, nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to delete datacenter: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("Expected status 204, got %d", resp.StatusCode)
		}
	})
}
