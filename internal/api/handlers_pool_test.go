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

func TestHandler_NetworkPoolCRUD(t *testing.T) {
	handler := setupTestHandler()
	store := handler.storage.(*mockStorage)

	var poolID string

	// 1. Create Pool
	t.Run("CreatePool", func(t *testing.T) {
		payload := `{"name": "Test Pool", "start_ip": "10.0.0.100", "end_ip": "10.0.0.200"}`
		req := httptest.NewRequest("POST", "/api/networks/net-1/pools", bytes.NewReader([]byte(payload)))
		req.SetPathValue("id", "net-1")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.createNetworkPool(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", resp.StatusCode)
		}

		var pool model.NetworkPool
		json.NewDecoder(resp.Body).Decode(&pool)
		poolID = pool.ID
		if poolID == "" {
			t.Error("Expected ID to be returned")
		}
		if pool.NetworkID != "net-1" {
			t.Errorf("Expected NetworkID net-1, got %s", pool.NetworkID)
		}
	})

	if poolID == "" {
		t.Fatal("Pool creation failed, skipping remaining tests")
	}

	// 2. Get Pool
	t.Run("GetPool", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/pools/"+poolID, nil)
		req.SetPathValue("id", poolID)
		w := httptest.NewRecorder()

		handler.getNetworkPool(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	// 3. Update Pool
	t.Run("UpdatePool", func(t *testing.T) {
		payload := `{"name": "Test Pool", "start_ip": "10.0.0.100", "end_ip": "10.0.0.200", "description": "Updated Pool"}`
		req := httptest.NewRequest("PUT", "/api/pools/"+poolID, bytes.NewReader([]byte(payload)))
		req.SetPathValue("id", poolID)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.updateNetworkPool(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		pool, err := store.GetNetworkPool(poolID)
		if err != nil {
			t.Fatalf("Failed to get pool from store: %v", err)
		}
		if pool.Description != "Updated Pool" {
			t.Error("Description should be updated")
		}
	})

	// 4. List Pools
	t.Run("ListPools", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/networks/net-1/pools", nil)
		req.SetPathValue("id", "net-1")
		w := httptest.NewRecorder()

		handler.listNetworkPools(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var list []model.NetworkPool
		json.NewDecoder(resp.Body).Decode(&list)
		if len(list) != 1 {
			t.Errorf("Expected 1 pool, got %d", len(list))
		}
	})

	// 5. Get Next IP
	t.Run("GetNextIP", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/pools/"+poolID+"/next-ip", nil)
		req.SetPathValue("id", poolID)
		w := httptest.NewRecorder()

		handler.getNextIP(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var result map[string]string
		json.NewDecoder(resp.Body).Decode(&result)
		if result["ip"] != "10.0.0.100" { // Mock returns StartIP
			t.Errorf("Expected 10.0.0.100, got %s", result["ip"])
		}
	})

	// 6. Delete Pool
	t.Run("DeletePool", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/pools/"+poolID, nil)
		req.SetPathValue("id", poolID)
		w := httptest.NewRecorder()

		handler.deleteNetworkPool(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		_, err := store.GetNetworkPool(poolID)
		if err != storage.ErrPoolNotFound {
			t.Error("Pool should be deleted")
		}
	})
}
