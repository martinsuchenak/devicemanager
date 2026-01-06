package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// TestAPI_DeviceRelationships tests device relationship management
func TestAPI_DeviceRelationships(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	// Create two devices
	d1Resp, _ := http.Post(ts.URL()+"/api/devices", "application/json", bytes.NewReader(DeviceJSON("Parent Device", nil)))
	var d1 map[string]interface{}
	json.NewDecoder(d1Resp.Body).Decode(&d1)
	parentID := d1["id"].(string)

	d2Resp, _ := http.Post(ts.URL()+"/api/devices", "application/json", bytes.NewReader(DeviceJSON("Child Device", nil)))
	var d2 map[string]interface{}
	json.NewDecoder(d2Resp.Body).Decode(&d2)
	childID := d2["id"].(string)

	// 1. Add Relationship
	t.Run("AddRelationship", func(t *testing.T) {
		payload := map[string]string{
			"child_id":          childID,
			"relationship_type": "contains",
		}
		data, _ := json.Marshal(payload)

		resp, err := http.Post(ts.URL()+"/api/devices/"+parentID+"/relationships", "application/json", bytes.NewReader(data))
		if err != nil {
			t.Fatalf("Failed to add relationship: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("Expected status 201, got %d: %s", resp.StatusCode, string(body))
		}
	})

	// 2. Get Related Devices
	t.Run("GetRelated", func(t *testing.T) {
		resp, err := http.Get(ts.URL() + "/api/devices/" + parentID + "/related")
		if err != nil {
			t.Fatalf("Failed to get related devices: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var related []map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&related)

		if len(related) != 1 {
			t.Errorf("Expected 1 related device, got %d", len(related))
		} else if related[0]["id"] != childID {
			t.Errorf("Expected related device %s, got %v", childID, related[0]["id"])
		}
	})

	// 3. Remove Relationship
	t.Run("RemoveRelationship", func(t *testing.T) {
		req, err := http.NewRequest("DELETE", ts.URL()+"/api/devices/"+parentID+"/relationships/"+childID+"/contains", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to remove relationship: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("Expected status 204, got %d", resp.StatusCode)
		}

		// Verify removal
		checkResp, _ := http.Get(ts.URL() + "/api/devices/" + parentID + "/related")
		var related []map[string]interface{}
		json.NewDecoder(checkResp.Body).Decode(&related)
		checkResp.Body.Close()

		if len(related) != 0 {
			t.Errorf("Expected 0 related devices, got %d", len(related))
		}
	})
}
