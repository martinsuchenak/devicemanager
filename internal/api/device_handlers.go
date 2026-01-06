package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/martinsuchenak/rackd/internal/log"
	"github.com/martinsuchenak/rackd/internal/model"
	"github.com/martinsuchenak/rackd/internal/storage"
)

// listDevices handles GET /api/devices
func (h *Handler) listDevices(w http.ResponseWriter, r *http.Request) {
	tags := r.URL.Query()["tag"]
	filter := &model.DeviceFilter{Tags: tags}

	log.Debug("Listing devices", "tags", tags)
	devices, err := h.storage.ListDevices(filter)
	if err != nil {
		log.Error("Failed to list devices", "error", err, "tags", tags)
		h.internalError(w, err)
		return
	}

	log.Info("Listed devices", "count", len(devices), "tags", tags)
	h.writeJSON(w, http.StatusOK, devices)
}

// getDevice handles GET /api/devices/{id}
func (h *Handler) getDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		log.Warn("Get device request missing ID")
		h.writeError(w, http.StatusBadRequest, "device ID required")
		return
	}

	log.Debug("Getting device", "id", id)
	device, err := h.storage.GetDevice(id)
	if err != nil {
		if errors.Is(err, storage.ErrDeviceNotFound) {
			log.Warn("Device not found", "id", id)
			h.writeError(w, http.StatusNotFound, "device not found")
			return
		}
		log.Error("Failed to get device", "error", err, "id", id)
		h.internalError(w, err)
		return
	}

	log.Info("Retrieved device", "id", id, "name", device.Name)
	h.writeJSON(w, http.StatusOK, device)
}

// createDevice handles POST /api/devices
func (h *Handler) createDevice(w http.ResponseWriter, r *http.Request) {
	var device model.Device
	if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
		log.Warn("Invalid device creation request body", "error", err)
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if device.Name == "" {
		log.Warn("Device creation missing required name")
		h.writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	log.Debug("Creating device", "name", device.Name, "datacenter_id", device.DatacenterID)

	// Validate IP addresses and Pools
	for _, addr := range device.Addresses {
		if net.ParseIP(addr.IP) == nil {
			h.writeError(w, http.StatusBadRequest, "invalid IP address: "+addr.IP)
			return
		}

		if addr.PoolID != "" {
			poolStorage, ok := h.storage.(storage.NetworkPoolStorage)
			if ok {
				valid, err := poolStorage.ValidateIPInPool(addr.PoolID, addr.IP)
				if err != nil {
					h.writeError(w, http.StatusBadRequest, "validating pool IP: "+err.Error())
					return
				}
				if !valid {
					h.writeError(w, http.StatusBadRequest, fmt.Sprintf("IP %s is not valid for pool %s", addr.IP, addr.PoolID))
					return
				}
			}
		}
	}

	// Generate ID if not provided
	if device.ID == "" {
		device.ID = generateID(device.Name)
	}

	// Set timestamps
	now := time.Now()
	device.CreatedAt = now
	device.UpdatedAt = now

	// Auto-assign default datacenter if none provided
	if device.DatacenterID == "" {
		if defaultDC := h.getDefaultDatacenter(); defaultDC != nil {
			device.DatacenterID = defaultDC.ID
		}
	}

	if err := h.storage.CreateDevice(&device); err != nil {
		if err == storage.ErrInvalidID {
			log.Warn("Device creation failed - invalid ID", "id", device.ID, "name", device.Name)
			h.writeError(w, http.StatusBadRequest, "invalid device ID")
			return
		}
		if strings.Contains(err.Error(), "already exists") {
			log.Warn("Device creation failed - already exists", "id", device.ID, "name", device.Name)
			h.writeError(w, http.StatusConflict, "device already exists")
			return
		}
		log.Error("Failed to create device", "error", err, "name", device.Name)
		h.internalError(w, err)
		return
	}

	log.Info("Device created successfully", "id", device.ID, "name", device.Name, "datacenter_id", device.DatacenterID)
	h.writeJSON(w, http.StatusCreated, device)
}

// updateDevice handles PUT /api/devices/{id}
func (h *Handler) updateDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		log.Warn("Update device request missing ID")
		h.writeError(w, http.StatusBadRequest, "device ID required")
		return
	}

	var device model.Device
	if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
		log.Warn("Invalid device update request body", "error", err, "id", id)
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	log.Debug("Updating device", "id", id, "name", device.Name)

	// Ensure ID matches URL
	device.ID = id
	device.UpdatedAt = time.Now()

	// Validate IP addresses and Pools
	for _, addr := range device.Addresses {
		if net.ParseIP(addr.IP) == nil {
			h.writeError(w, http.StatusBadRequest, "invalid IP address: "+addr.IP)
			return
		}

		if addr.PoolID != "" {
			poolStorage, ok := h.storage.(storage.NetworkPoolStorage)
			if ok {
				valid, err := poolStorage.ValidateIPInPool(addr.PoolID, addr.IP)
				if err != nil {
					h.writeError(w, http.StatusBadRequest, "validating pool IP: "+err.Error())
					return
				}
				if !valid {
					h.writeError(w, http.StatusBadRequest, fmt.Sprintf("IP %s is not valid for pool %s", addr.IP, addr.PoolID))
					return
				}
			}
		}
	}

	if err := h.storage.UpdateDevice(&device); err != nil {
		if errors.Is(err, storage.ErrDeviceNotFound) {
			log.Warn("Device update failed - not found", "id", id)
			h.writeError(w, http.StatusNotFound, "device not found")
			return
		}
		log.Error("Failed to update device", "error", err, "id", id)
		h.internalError(w, err)
		return
	}

	log.Info("Device updated successfully", "id", id, "name", device.Name)
	h.writeJSON(w, http.StatusOK, device)
}

// deleteDevice handles DELETE /api/devices/{id}
func (h *Handler) deleteDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		log.Warn("Delete device request missing ID")
		h.writeError(w, http.StatusBadRequest, "device ID required")
		return
	}

	log.Debug("Deleting device", "id", id)
	if err := h.storage.DeleteDevice(id); err != nil {
		if errors.Is(err, storage.ErrDeviceNotFound) {
			log.Warn("Device deletion failed - not found", "id", id)
			h.writeError(w, http.StatusNotFound, "device not found")
			return
		}
		log.Error("Failed to delete device", "error", err, "id", id)
		h.internalError(w, err)
		return
	}

	log.Info("Device deleted successfully", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

// searchDevices handles GET /api/search?q=
func (h *Handler) searchDevices(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		log.Warn("Search devices request missing query")
		h.writeError(w, http.StatusBadRequest, "search query required")
		return
	}

	log.Debug("Searching devices", "query", query)
	devices, err := h.storage.SearchDevices(query)
	if err != nil {
		log.Error("Failed to search devices", "error", err, "query", query)
		h.internalError(w, err)
		return
	}

	log.Info("Search devices completed", "query", query, "results", len(devices))
	h.writeJSON(w, http.StatusOK, devices)
}

// Relationship handlers (SQLite only)

// addRelationship handles POST /api/devices/{id}/relationships
func (h *Handler) addRelationship(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")

	if deviceID == "" {
		log.Warn("Add relationship request missing device ID")
		h.writeError(w, http.StatusBadRequest, "device ID required")
		return
	}

	var req struct {
		ChildID          string `json:"child_id"`
		RelationshipType string `json:"relationship_type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("Invalid add relationship request body", "error", err, "device_id", deviceID)
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ChildID == "" {
		log.Warn("Add relationship missing child ID", "device_id", deviceID)
		h.writeError(w, http.StatusBadRequest, "child_id is required")
		return
	}

	if req.RelationshipType == "" {
		req.RelationshipType = "related"
	}

	log.Debug("Adding device relationship", "parent_id", deviceID, "child_id", req.ChildID, "type", req.RelationshipType)

	// Check if storage supports relationships
	relStorage, ok := h.storage.(interface {
		AddRelationship(parentID, childID, relationshipType string) error
	})
	if !ok {
		h.writeError(w, http.StatusNotImplemented, "relationships are not supported by this storage backend")
		return
	}

	if err := relStorage.AddRelationship(deviceID, req.ChildID, req.RelationshipType); err != nil {
		if errors.Is(err, storage.ErrDeviceNotFound) {
			log.Warn("Add relationship failed - device not found", "parent_id", deviceID, "child_id", req.ChildID)
			h.writeError(w, http.StatusNotFound, "device not found")
			return
		}
		log.Error("Failed to add relationship", "error", err, "parent_id", deviceID, "child_id", req.ChildID, "type", req.RelationshipType)
		h.internalError(w, err)
		return
	}

	log.Info("Relationship added successfully", "parent_id", deviceID, "child_id", req.ChildID, "type", req.RelationshipType)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"message":           "relationship created",
		"parent_id":         deviceID,
		"child_id":          req.ChildID,
		"relationship_type": req.RelationshipType,
	})
}

// getRelationships handles GET /api/devices/{id}/relationships
func (h *Handler) getRelationships(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")
	if deviceID == "" {
		log.Warn("Get relationships request missing device ID")
		h.writeError(w, http.StatusBadRequest, "device ID required")
		return
	}

	log.Debug("Getting device relationships", "device_id", deviceID)

	// Check if storage supports relationships
	relStorage, ok := h.storage.(interface {
		GetRelationships(deviceID string) ([]model.DeviceRelationship, error)
	})
	if !ok {
		log.Warn("Relationships not supported by storage backend")
		h.writeError(w, http.StatusNotImplemented, "relationships are not supported by this storage backend")
		return
	}

	relationships, err := relStorage.GetRelationships(deviceID)
	if err != nil {
		log.Error("Failed to get device relationships", "error", err, "device_id", deviceID)
		h.internalError(w, err)
		return
	}

	log.Info("Retrieved device relationships", "device_id", deviceID, "count", len(relationships))
	h.writeJSON(w, http.StatusOK, relationships)
}

// getRelatedDevices handles GET /api/devices/{id}/related
func (h *Handler) getRelatedDevices(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")
	if deviceID == "" {
		log.Warn("Get related devices request missing device ID")
		h.writeError(w, http.StatusBadRequest, "device ID required")
		return
	}

	// Get relationship type from query parameter
	relType := r.URL.Query().Get("type")

	log.Debug("Getting related devices", "device_id", deviceID, "type", relType)

	// Check if storage supports relationships
	relStorage, ok := h.storage.(interface {
		GetRelatedDevices(deviceID, relationshipType string) ([]model.Device, error)
	})
	if !ok {
		log.Warn("Relationships not supported by storage backend")
		h.writeError(w, http.StatusNotImplemented, "relationships are not supported by this storage backend")
		return
	}

	devices, err := relStorage.GetRelatedDevices(deviceID, relType)
	if err != nil {
		log.Error("Failed to get related devices", "error", err, "device_id", deviceID, "type", relType)
		h.internalError(w, err)
		return
	}

	log.Info("Retrieved related devices", "device_id", deviceID, "type", relType, "count", len(devices))
	h.writeJSON(w, http.StatusOK, devices)
}

// removeRelationship handles DELETE /api/devices/{id}/relationships
func (h *Handler) removeRelationship(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")
	childID := r.PathValue("child_id")
	relType := r.PathValue("type")

	if deviceID == "" || childID == "" {
		log.Warn("Remove relationship request missing required IDs", "device_id", deviceID, "child_id", childID)
		h.writeError(w, http.StatusBadRequest, "device ID and child ID required")
		return
	}

	log.Debug("Removing device relationship", "parent_id", deviceID, "child_id", childID, "type", relType)

	// Check if storage supports relationships
	relStorage, ok := h.storage.(interface {
		RemoveRelationship(parentID, childID, relationshipType string) error
	})
	if !ok {
		log.Warn("Relationships not supported by storage backend")
		h.writeError(w, http.StatusNotImplemented, "relationships are not supported by this storage backend")
		return
	}

	if err := relStorage.RemoveRelationship(deviceID, childID, relType); err != nil {
		if errors.Is(err, storage.ErrDeviceNotFound) {
			log.Warn("Remove relationship failed - device or relationship not found", "parent_id", deviceID, "child_id", childID, "type", relType)
			h.writeError(w, http.StatusNotFound, "device or relationship not found")
			return
		}
		log.Error("Failed to remove relationship", "error", err, "parent_id", deviceID, "child_id", childID, "type", relType)
		h.internalError(w, err)
		return
	}

	log.Info("Relationship removed successfully", "parent_id", deviceID, "child_id", childID, "type", relType)
	w.WriteHeader(http.StatusNoContent)
}
