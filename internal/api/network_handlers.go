package api

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/martinsuchenak/rackd/internal/log"
	"github.com/martinsuchenak/rackd/internal/model"
	"github.com/martinsuchenak/rackd/internal/storage"
)

// listNetworks handles GET /api/networks
func (h *Handler) listNetworks(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	datacenterID := r.URL.Query().Get("datacenter_id")
	filter := &model.NetworkFilter{Name: name, DatacenterID: datacenterID}

	log.Debug("Listing networks", "name", name, "datacenter_id", datacenterID)

	// Check if storage supports networks
	netStorage, ok := h.storage.(storage.NetworkStorage)
	if !ok {
		log.Warn("Networks not supported by storage backend")
		h.writeError(w, http.StatusNotImplemented, "networks are not supported by this storage backend")
		return
	}

	networks, err := netStorage.ListNetworks(filter)
	if err != nil {
		log.Error("Failed to list networks", "error", err, "name", name, "datacenter_id", datacenterID)
		h.internalError(w, err)
		return
	}

	log.Info("Listed networks", "count", len(networks), "name", name, "datacenter_id", datacenterID)
	h.writeJSON(w, http.StatusOK, networks)
}

// getNetwork handles GET /api/networks/{id}
func (h *Handler) getNetwork(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		log.Warn("Get network request missing ID")
		h.writeError(w, http.StatusBadRequest, "network ID required")
		return
	}

	log.Debug("Getting network", "id", id)

	netStorage, ok := h.storage.(storage.NetworkStorage)
	if !ok {
		log.Warn("Networks not supported by storage backend")
		h.writeError(w, http.StatusNotImplemented, "networks are not supported by this storage backend")
		return
	}

	network, err := netStorage.GetNetwork(id)
	if err != nil {
		if errors.Is(err, storage.ErrNetworkNotFound) {
			log.Warn("Network not found", "id", id)
			h.writeError(w, http.StatusNotFound, "network not found")
			return
		}
		log.Error("Failed to get network", "error", err, "id", id)
		h.internalError(w, err)
		return
	}

	log.Info("Retrieved network", "id", id, "name", network.Name)
	h.writeJSON(w, http.StatusOK, network)
}

// createNetwork handles POST /api/networks
func (h *Handler) createNetwork(w http.ResponseWriter, r *http.Request) {
	var network model.Network
	if err := json.NewDecoder(r.Body).Decode(&network); err != nil {
		log.Warn("Invalid network creation request body", "error", err)
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if network.Name == "" {
		log.Warn("Network creation missing required name")
		h.writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if network.Subnet == "" {
		log.Warn("Network creation missing required subnet")
		h.writeError(w, http.StatusBadRequest, "subnet is required")
		return
	}
	if _, _, err := net.ParseCIDR(network.Subnet); err != nil {
		log.Warn("Network creation invalid subnet CIDR", "subnet", network.Subnet, "error", err)
		h.writeError(w, http.StatusBadRequest, "invalid subnet CIDR: "+network.Subnet)
		return
	}

	log.Debug("Creating network", "name", network.Name, "subnet", network.Subnet, "datacenter_id", network.DatacenterID)

	// Auto-assign default datacenter if none provided
	if network.DatacenterID == "" {
		if defaultDC := h.getDefaultDatacenter(); defaultDC != nil {
			network.DatacenterID = defaultDC.ID
		} else {
			h.writeError(w, http.StatusBadRequest, "datacenter_id is required")
			return
		}
	}

	// Generate ID if not provided
	if network.ID == "" {
		network.ID = generateNetworkID()
	}

	netStorage, ok := h.storage.(storage.NetworkStorage)
	if !ok {
		h.writeError(w, http.StatusNotImplemented, "networks are not supported by this storage backend")
		return
	}

	if err := netStorage.CreateNetwork(&network); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			log.Warn("Network creation failed - already exists", "name", network.Name)
			h.writeError(w, http.StatusConflict, "network with this name already exists")
			return
		}
		if strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
			log.Warn("Network creation failed - datacenter not found", "datacenter_id", network.DatacenterID)
			h.writeError(w, http.StatusBadRequest, "datacenter not found")
			return
		}
		log.Error("Failed to create network", "error", err, "name", network.Name)
		h.internalError(w, err)
		return
	}

	log.Info("Network created successfully", "id", network.ID, "name", network.Name, "subnet", network.Subnet)
	h.writeJSON(w, http.StatusCreated, network)
}

// updateNetwork handles PUT /api/networks/{id}
func (h *Handler) updateNetwork(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		log.Warn("Update network request missing ID")
		h.writeError(w, http.StatusBadRequest, "network ID required")
		return
	}

	var network model.Network
	if err := json.NewDecoder(r.Body).Decode(&network); err != nil {
		log.Warn("Invalid network update request body", "error", err, "id", id)
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	log.Debug("Updating network", "id", id, "name", network.Name)

	// Ensure ID matches URL
	network.ID = id

	// Validate subnet if provided (though it's required in model, JSON decode might leave it empty or partially filled)
	if network.Subnet != "" {
		if _, _, err := net.ParseCIDR(network.Subnet); err != nil {
			log.Warn("Network update invalid subnet CIDR", "subnet", network.Subnet, "error", err, "id", id)
			h.writeError(w, http.StatusBadRequest, "invalid subnet CIDR: "+network.Subnet)
			return
		}
	}

	netStorage, ok := h.storage.(storage.NetworkStorage)
	if !ok {
		log.Warn("Networks not supported by storage backend")
		h.writeError(w, http.StatusNotImplemented, "networks are not supported by this storage backend")
		return
	}

	if err := netStorage.UpdateNetwork(&network); err != nil {
		if errors.Is(err, storage.ErrNetworkNotFound) {
			log.Warn("Network update failed - not found", "id", id)
			h.writeError(w, http.StatusNotFound, "network not found")
			return
		}
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			log.Warn("Network update failed - name already exists", "id", id, "name", network.Name)
			h.writeError(w, http.StatusConflict, "network with this name already exists")
			return
		}
		if strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
			log.Warn("Network update failed - datacenter not found", "id", id, "datacenter_id", network.DatacenterID)
			h.writeError(w, http.StatusBadRequest, "datacenter not found")
			return
		}
		log.Error("Failed to update network", "error", err, "id", id)
		h.internalError(w, err)
		return
	}

	log.Info("Network updated successfully", "id", id, "name", network.Name)
	h.writeJSON(w, http.StatusOK, network)
}

// deleteNetwork handles DELETE /api/networks/{id}
func (h *Handler) deleteNetwork(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		log.Warn("Delete network request missing ID")
		h.writeError(w, http.StatusBadRequest, "network ID required")
		return
	}

	log.Debug("Deleting network", "id", id)

	netStorage, ok := h.storage.(storage.NetworkStorage)
	if !ok {
		log.Warn("Networks not supported by storage backend")
		h.writeError(w, http.StatusNotImplemented, "networks are not supported by this storage backend")
		return
	}

	if err := netStorage.DeleteNetwork(id); err != nil {
		if errors.Is(err, storage.ErrNetworkNotFound) {
			log.Warn("Network deletion failed - not found", "id", id)
			h.writeError(w, http.StatusNotFound, "network not found")
			return
		}
		log.Error("Failed to delete network", "error", err, "id", id)
		h.internalError(w, err)
		return
	}

	log.Info("Network deleted successfully", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

// getNetworkDevices handles GET /api/networks/{id}/devices
func (h *Handler) getNetworkDevices(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		log.Warn("Get network devices request missing ID")
		h.writeError(w, http.StatusBadRequest, "network ID required")
		return
	}

	log.Debug("Getting network devices", "network_id", id)

	netStorage, ok := h.storage.(storage.NetworkStorage)
	if !ok {
		log.Warn("Networks not supported by storage backend")
		h.writeError(w, http.StatusNotImplemented, "networks are not supported by this storage backend")
		return
	}

	devices, err := netStorage.GetNetworkDevices(id)
	if err != nil {
		log.Error("Failed to get network devices", "error", err, "network_id", id)
		h.internalError(w, err)
		return
	}

	log.Info("Retrieved network devices", "network_id", id, "count", len(devices))
	h.writeJSON(w, http.StatusOK, devices)
}

// generateNetworkID generates a UUIDv7 for a network
func generateNetworkID() string {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.New().String()
	}
	return id.String()
}