package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/martinsuchenak/rackd/internal/model"
)

func TestHandler_Relationships(t *testing.T) {
	handler := setupTestHandler()
	store := handler.storage.(*mockStorage)

	// Setup devices
	d1 := &model.Device{ID: "d1", Name: "Parent", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	d2 := &model.Device{ID: "d2", Name: "Child", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	store.CreateDevice(d1)
	store.CreateDevice(d2)

	// 1. Add Relationship
	t.Run("AddRelationship", func(t *testing.T) {
		payload := `{"child_id": "d2", "relationship_type": "contains"}`
		req := httptest.NewRequest("POST", "/api/devices/d1/relationships", bytes.NewReader([]byte(payload)))
		req.SetPathValue("id", "d1")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.addRelationship(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", resp.StatusCode)
		}
	})

	// 2. Get Relationships
	t.Run("GetRelationships", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/devices/d1/relationships", nil)
		req.SetPathValue("id", "d1")
		w := httptest.NewRecorder()

		handler.getRelationships(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var rels []model.DeviceRelationship
		json.NewDecoder(resp.Body).Decode(&rels)
		if len(rels) != 1 {
			t.Errorf("Expected 1 relationship, got %d", len(rels))
		}
		if rels[0].ChildID != "d2" {
			t.Errorf("Expected ChildID d2, got %s", rels[0].ChildID)
		}
	})

	// 3. Get Related Devices
	t.Run("GetRelatedDevices", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/devices/d1/related", nil)
		req.SetPathValue("id", "d1")
		w := httptest.NewRecorder()

		handler.getRelatedDevices(w, req)

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
		if devices[0].ID != "d2" {
			t.Errorf("Expected device d2, got %s", devices[0].ID)
		}
	})

	// 4. Remove Relationship
	t.Run("RemoveRelationship", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/devices/d1/relationships/d2/contains", nil)
		req.SetPathValue("id", "d1")
		req.SetPathValue("child_id", "d2")
		req.SetPathValue("type", "contains")
		w := httptest.NewRecorder()

		handler.removeRelationship(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("Expected status 204, got %d", resp.StatusCode)
		}

		// Verify removal
		rels, _ := store.GetRelationships("d1")
		if len(rels) != 0 {
			t.Errorf("Expected 0 relationships, got %d", len(rels))
		}
	})
}
