package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/martinsuchenak/rackd/internal/model"
	"github.com/martinsuchenak/rackd/internal/storage"
)

func TestHandler_DatacenterCRUD(t *testing.T) {
	handler := setupTestHandler()
	store := handler.storage.(*mockStorage)

	var dcID string

	// 1. Create Datacenter
	t.Run("CreateDatacenter", func(t *testing.T) {
		payload := `{"name": "Test DC", "location": "NYC"}`
		req := httptest.NewRequest("POST", "/api/datacenters", bytes.NewReader([]byte(payload)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.createDatacenter(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", resp.StatusCode)
		}

		var dc model.Datacenter
		json.NewDecoder(resp.Body).Decode(&dc)
		dcID = dc.ID
		if dcID == "" {
			t.Error("Expected ID to be returned")
		}
	})

	// 2. Get Datacenter
	t.Run("GetDatacenter", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/datacenters/"+dcID, nil)
		req.SetPathValue("id", dcID)
		w := httptest.NewRecorder()

		handler.getDatacenter(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var dc model.Datacenter
		json.NewDecoder(resp.Body).Decode(&dc)
		if dc.ID != dcID {
			t.Errorf("Expected ID %s, got %s", dcID, dc.ID)
		}
	})

	// 3. Update Datacenter
	t.Run("UpdateDatacenter", func(t *testing.T) {
		payload := `{"name": "Updated DC", "location": "NYC"}`
		req := httptest.NewRequest("PUT", "/api/datacenters/"+dcID, bytes.NewReader([]byte(payload)))
		req.SetPathValue("id", dcID)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.updateDatacenter(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		// Verify update in mock storage
		dc, _ := store.GetDatacenter(dcID)
		if dc.Name != "Updated DC" {
			t.Errorf("Expected name 'Updated DC', got %s", dc.Name)
		}
	})

	// 4. List Datacenters
	t.Run("ListDatacenters", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/datacenters", nil)
		w := httptest.NewRecorder()

		handler.listDatacenters(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var dcs []model.Datacenter
		json.NewDecoder(resp.Body).Decode(&dcs)
		if len(dcs) != 1 {
			t.Errorf("Expected 1 datacenter, got %d", len(dcs))
		}
	})

	// 5. Delete Datacenter
	t.Run("DeleteDatacenter", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/datacenters/"+dcID, nil)
		req.SetPathValue("id", dcID)
		w := httptest.NewRecorder()

		handler.deleteDatacenter(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("Expected status 204, got %d", resp.StatusCode)
		}

		_, err := store.GetDatacenter(dcID)
		if err != storage.ErrDatacenterNotFound {
			t.Error("Datacenter should be deleted")
		}
	})
}

func TestHandler_GetDatacenterDevices(t *testing.T) {
	handler := setupTestHandler()
	store := handler.storage.(*mockStorage)

	// Setup
	dc := &model.Datacenter{ID: "dc-1", Name: "Test DC"}
	store.CreateDatacenter(dc)

	d1 := &model.Device{ID: "dev-1", Name: "Device 1", DatacenterID: "dc-1", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	d2 := &model.Device{ID: "dev-2", Name: "Device 2", DatacenterID: "dc-2", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	store.CreateDevice(d1)
	store.CreateDevice(d2)

	req := httptest.NewRequest("GET", "/api/datacenters/dc-1/devices", nil)
	req.SetPathValue("id", "dc-1")
	w := httptest.NewRecorder()

	handler.getDatacenterDevices(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var devices []model.Device
	json.NewDecoder(resp.Body).Decode(&devices)

	if len(devices) != 1 {
		t.Errorf("Expected 1 device, got %d", len(devices))
	}
	if devices[0].ID != "dev-1" {
		t.Errorf("Expected device dev-1, got %s", devices[0].ID)
	}
}
