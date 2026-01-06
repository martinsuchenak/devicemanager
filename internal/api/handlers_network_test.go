package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/martinsuchenak/rackd/internal/model"
	"github.com/martinsuchenak/rackd/internal/storage"
)

func TestHandler_NetworkCRUD(t *testing.T) {
	handler := setupTestHandler()
	store := handler.storage.(*mockStorage)

	// Setup DC
	store.CreateDatacenter(&model.Datacenter{ID: "dc-1", Name: "DC"})

	var netID string

	// 1. Create Network
	t.Run("CreateNetwork", func(t *testing.T) {
		payload := `{"name": "LAN", "subnet": "192.168.1.0/24", "datacenter_id": "dc-1"}`
		req := httptest.NewRequest("POST", "/api/networks", bytes.NewReader([]byte(payload)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.createNetwork(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", resp.StatusCode)
		}

		var net model.Network
		json.NewDecoder(resp.Body).Decode(&net)
		netID = net.ID
		if netID == "" {
			t.Error("Expected ID to be returned")
		}
	})

	// 2. Get Network
	t.Run("GetNetwork", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/networks/"+netID, nil)
		req.SetPathValue("id", netID)
		w := httptest.NewRecorder()

		handler.getNetwork(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	// 3. Update Network
	t.Run("UpdateNetwork", func(t *testing.T) {
		payload := `{"name": "Updated LAN", "subnet": "192.168.1.0/24", "datacenter_id": "dc-1"}`
		req := httptest.NewRequest("PUT", "/api/networks/"+netID, bytes.NewReader([]byte(payload)))
		req.SetPathValue("id", netID)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.updateNetwork(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		net, _ := store.GetNetwork(netID)
		if net.Name != "Updated LAN" {
			t.Errorf("Expected name 'Updated LAN', got %s", net.Name)
		}
	})

	// 4. List Networks
	t.Run("ListNetworks", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/networks", nil)
		w := httptest.NewRecorder()

		handler.listNetworks(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var list []model.Network
		json.NewDecoder(resp.Body).Decode(&list)
		if len(list) != 1 {
			t.Errorf("Expected 1 network, got %d", len(list))
		}
	})

	// 5. Delete Network
	t.Run("DeleteNetwork", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/networks/"+netID, nil)
		req.SetPathValue("id", netID)
		w := httptest.NewRecorder()

		handler.deleteNetwork(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("Expected status 204, got %d", resp.StatusCode)
		}

		_, err := store.GetNetwork(netID)
		if err != storage.ErrNetworkNotFound {
			t.Error("Network should be deleted")
		}
	})
}

func TestHandler_GetNetworkDevices(t *testing.T) {
	handler := setupTestHandler()

	req := httptest.NewRequest("GET", "/api/networks/net-1/devices", nil)
	req.SetPathValue("id", "net-1")
	w := httptest.NewRecorder()

	handler.getNetworkDevices(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Currently mock returns empty list, just verify response format
	var devices []model.Device
	if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}
}
