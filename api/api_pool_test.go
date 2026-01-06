package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// TestAPI_NetworkPools tests network pool management
func TestAPI_NetworkPools(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	// Setup: Create Datacenter and Network
	dcResp, _ := http.Post(ts.URL()+"/api/datacenters", "application/json", bytes.NewReader(DeviceJSON("Pool DC", nil)))
	var dc map[string]interface{}
	json.NewDecoder(dcResp.Body).Decode(&dc)
	dcID := dc["id"].(string)

	netPayload := map[string]interface{}{
		"name":          "Pool Network",
		"subnet":        "10.0.0.0/24",
		"datacenter_id": dcID,
	}
	netData, _ := json.Marshal(netPayload)
	netResp, _ := http.Post(ts.URL()+"/api/networks", "application/json", bytes.NewReader(netData))
	var net map[string]interface{}
	json.NewDecoder(netResp.Body).Decode(&net)
	netID := net["id"].(string)

	var poolID string

	// 1. Create Pool
	t.Run("CreatePool", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":     "Test Pool",
			"start_ip": "10.0.0.100",
			"end_ip":   "10.0.0.200",
		}
		data, _ := json.Marshal(payload)

		resp, err := http.Post(ts.URL()+"/api/networks/"+netID+"/pools", "application/json", bytes.NewReader(data))
		if err != nil {
			t.Fatalf("Failed to create pool: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 201, got %d: %s", resp.StatusCode, string(body))
		}

		var pool map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&pool); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		poolID = pool["id"].(string)
		if poolID == "" {
			t.Error("Expected pool ID to be set")
		}
	})

	// 2. Get Next IP
	t.Run("GetNextIP", func(t *testing.T) {
		resp, err := http.Get(ts.URL() + "/api/pools/" + poolID + "/next-ip")
		if err != nil {
			t.Fatalf("Failed to get next IP: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		if result["ip"] != "10.0.0.100" {
			t.Errorf("Expected first IP 10.0.0.100, got %v", result["ip"])
		}
	})

	// 3. List Pools
	t.Run("ListPools", func(t *testing.T) {
		resp, err := http.Get(ts.URL() + "/api/networks/" + netID + "/pools")
		if err != nil {
			t.Fatalf("Failed to list pools: %v", err)
		}
		defer resp.Body.Close()

		var pools []map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&pools)

		if len(pools) != 1 {
			t.Errorf("Expected 1 pool, got %d", len(pools))
		}
	})

	// 4. Delete Pool
	t.Run("DeletePool", func(t *testing.T) {
		req, err := http.NewRequest("DELETE", ts.URL()+"/api/pools/"+poolID, nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to delete pool: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})
}
