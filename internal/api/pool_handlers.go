package api

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/martinsuchenak/rackd/internal/log"
	"github.com/martinsuchenak/rackd/internal/model"
	"github.com/martinsuchenak/rackd/internal/storage"
)

// listNetworkPools handles GET /api/networks/{id}/pools
func (h *Handler) listNetworkPools(w http.ResponseWriter, r *http.Request) {
	networkID := r.PathValue("id")
	if networkID == "" {
		log.Warn("List network pools request missing network ID")
		h.writeError(w, http.StatusBadRequest, "network ID is required")
		return
	}

	log.Debug("Listing network pools", "network_id", networkID)

	poolStorage, ok := h.storage.(storage.NetworkPoolStorage)
	if !ok {
		log.Warn("Network pools not supported by storage backend")
		h.writeError(w, http.StatusNotImplemented, "network pools not supported by storage backend")
		return
	}

	pools, err := poolStorage.ListNetworkPools(&model.NetworkPoolFilter{NetworkID: networkID})
	if err != nil {
		log.Error("Failed to list network pools", "error", err, "network_id", networkID)
		h.internalError(w, err)
		return
	}

	log.Info("Listed network pools", "network_id", networkID, "count", len(pools))
	h.writeJSON(w, http.StatusOK, pools)
}

// getNetworkPool handles GET /api/pools/{id}
func (h *Handler) getNetworkPool(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		log.Warn("Get network pool request missing ID")
		h.writeError(w, http.StatusBadRequest, "pool ID is required")
		return
	}

	log.Debug("Getting network pool", "id", id)

	poolStorage, ok := h.storage.(storage.NetworkPoolStorage)
	if !ok {
		log.Warn("Network pools not supported by storage backend")
		h.writeError(w, http.StatusNotImplemented, "network pools not supported by storage backend")
		return
	}

	pool, err := poolStorage.GetNetworkPool(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Warn("Network pool not found", "id", id)
			h.writeError(w, http.StatusNotFound, "network pool not found")
			return
		}
		log.Error("Failed to get network pool", "error", err, "id", id)
		h.internalError(w, err)
		return
	}

	log.Info("Retrieved network pool", "id", id, "name", pool.Name)
	h.writeJSON(w, http.StatusOK, pool)
}

// createNetworkPool handles POST /api/networks/{id}/pools
func (h *Handler) createNetworkPool(w http.ResponseWriter, r *http.Request) {
	networkID := r.PathValue("id") // From /api/networks/{id}/pools
	if networkID == "" {
		log.Warn("Create network pool request missing network ID")
		h.writeError(w, http.StatusBadRequest, "network ID is required")
		return
	}

	var pool model.NetworkPool
	if err := json.NewDecoder(r.Body).Decode(&pool); err != nil {
		log.Warn("Invalid network pool creation request body", "error", err, "network_id", networkID)
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	pool.NetworkID = networkID
	if pool.Name == "" {
		log.Warn("Network pool creation missing required name", "network_id", networkID)
		h.writeError(w, http.StatusBadRequest, "pool name is required")
		return
	}
	if pool.StartIP == "" || pool.EndIP == "" {
		log.Warn("Network pool creation missing IP range", "network_id", networkID, "name", pool.Name)
		h.writeError(w, http.StatusBadRequest, "start_ip and end_ip are required")
		return
	}
	if net.ParseIP(pool.StartIP) == nil || net.ParseIP(pool.EndIP) == nil {
		log.Warn("Network pool creation invalid IP format", "start_ip", pool.StartIP, "end_ip", pool.EndIP, "network_id", networkID)
		h.writeError(w, http.StatusBadRequest, "invalid IP address format")
		return
	}

	log.Debug("Creating network pool", "name", pool.Name, "network_id", networkID, "start_ip", pool.StartIP, "end_ip", pool.EndIP)

	if pool.ID == "" {
		pool.ID = generateID(pool.Name)
	}

	poolStorage, ok := h.storage.(storage.NetworkPoolStorage)
	if !ok {
		log.Warn("Network pools not supported by storage backend")
		h.writeError(w, http.StatusNotImplemented, "network pools not supported by storage backend")
		return
	}

	if err := poolStorage.CreateNetworkPool(&pool); err != nil {
		if strings.Contains(err.Error(), "already exists") { // Assuming unique name/network constraint
			log.Warn("Network pool creation failed - already exists", "name", pool.Name, "network_id", networkID)
			h.writeError(w, http.StatusConflict, "network pool already exists")
			return
		}
		log.Error("Failed to create network pool", "error", err, "name", pool.Name, "network_id", networkID)
		h.internalError(w, err)
		return
	}

	log.Info("Network pool created successfully", "id", pool.ID, "name", pool.Name, "network_id", networkID)
	h.writeJSON(w, http.StatusCreated, pool)
}

// updateNetworkPool handles PUT /api/pools/{id}
func (h *Handler) updateNetworkPool(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		log.Warn("Update network pool request missing ID")
		h.writeError(w, http.StatusBadRequest, "pool ID is required")
		return
	}

	var pool model.NetworkPool
	if err := json.NewDecoder(r.Body).Decode(&pool); err != nil {
		log.Warn("Invalid network pool update request body", "error", err, "id", id)
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	log.Debug("Updating network pool", "id", id, "name", pool.Name)

	pool.ID = id

	poolStorage, ok := h.storage.(storage.NetworkPoolStorage)
	if !ok {
		log.Warn("Network pools not supported by storage backend")
		h.writeError(w, http.StatusNotImplemented, "network pools not supported by storage backend")
		return
	}

	if err := poolStorage.UpdateNetworkPool(&pool); err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Warn("Network pool update failed - not found", "id", id)
			h.writeError(w, http.StatusNotFound, "network pool not found")
			return
		}
		log.Error("Failed to update network pool", "error", err, "id", id)
		h.internalError(w, err)
		return
	}

	log.Info("Network pool updated successfully", "id", id, "name", pool.Name)
	h.writeJSON(w, http.StatusOK, pool)
}

// deleteNetworkPool handles DELETE /api/pools/{id}
func (h *Handler) deleteNetworkPool(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		log.Warn("Delete network pool request missing ID")
		h.writeError(w, http.StatusBadRequest, "pool ID is required")
		return
	}

	log.Debug("Deleting network pool", "id", id)

	poolStorage, ok := h.storage.(storage.NetworkPoolStorage)
	if !ok {
		log.Warn("Network pools not supported by storage backend")
		h.writeError(w, http.StatusNotImplemented, "network pools not supported by storage backend")
		return
	}

	if err := poolStorage.DeleteNetworkPool(id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Warn("Network pool deletion failed - not found", "id", id)
			h.writeError(w, http.StatusNotFound, "network pool not found")
			return
		}
		log.Error("Failed to delete network pool", "error", err, "id", id)
		h.internalError(w, err)
		return
	}

	log.Info("Network pool deleted successfully", "id", id)
	h.writeJSON(w, http.StatusOK, map[string]string{"message": "network pool deleted"})
}

// getNextIP handles GET /api/pools/{id}/next-ip
func (h *Handler) getNextIP(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		log.Warn("Get next IP request missing pool ID")
		h.writeError(w, http.StatusBadRequest, "pool ID is required")
		return
	}

	log.Debug("Getting next available IP", "pool_id", id)

	poolStorage, ok := h.storage.(storage.NetworkPoolStorage)
	if !ok {
		log.Warn("Network pools not supported by storage backend")
		h.writeError(w, http.StatusNotImplemented, "network pools not supported by storage backend")
		return
	}

	ip, err := poolStorage.GetNextAvailableIP(id)
	if err != nil {
		if strings.Contains(err.Error(), "no available IPs") {
			log.Warn("No available IPs in pool", "pool_id", id)
			h.writeError(w, http.StatusConflict, "no available IPs in pool")
			return
		}
		log.Error("Failed to get next available IP", "error", err, "pool_id", id)
		h.internalError(w, err)
		return
	}

	log.Info("Retrieved next available IP", "pool_id", id, "ip", ip)
	h.writeJSON(w, http.StatusOK, map[string]string{"ip": ip})
}