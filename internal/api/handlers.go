package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/martinsuchenak/rackd/internal/log"
	"github.com/martinsuchenak/rackd/internal/model"
	"github.com/martinsuchenak/rackd/internal/storage"
)

// Handler handles HTTP requests
type Handler struct {
	storage storage.Storage
}

// NewHandler creates a new API handler
func NewHandler(s storage.Storage) *Handler {
	return &Handler{storage: s}
}

// RegisterRoutes registers all API routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Datacenter CRUD
	mux.HandleFunc("GET /api/datacenters", h.listDatacenters)
	mux.HandleFunc("POST /api/datacenters", h.createDatacenter)
	mux.HandleFunc("GET /api/datacenters/{id}", h.getDatacenter)
	mux.HandleFunc("PUT /api/datacenters/{id}", h.updateDatacenter)
	mux.HandleFunc("DELETE /api/datacenters/{id}", h.deleteDatacenter)
	mux.HandleFunc("GET /api/datacenters/{id}/devices", h.getDatacenterDevices)

	// Network CRUD
	mux.HandleFunc("GET /api/networks", h.listNetworks)
	mux.HandleFunc("POST /api/networks", h.createNetwork)
	mux.HandleFunc("GET /api/networks/{id}", h.getNetwork)
	mux.HandleFunc("PUT /api/networks/{id}", h.updateNetwork)
	mux.HandleFunc("DELETE /api/networks/{id}", h.deleteNetwork)
	mux.HandleFunc("GET /api/networks/{id}/devices", h.getNetworkDevices)

	// Device CRUD
	mux.HandleFunc("GET /api/devices", h.listDevices)
	mux.HandleFunc("POST /api/devices", h.createDevice)
	mux.HandleFunc("GET /api/devices/{id}", h.getDevice)
	mux.HandleFunc("PUT /api/devices/{id}", h.updateDevice)
	mux.HandleFunc("DELETE /api/devices/{id}", h.deleteDevice)

	// Search
	mux.HandleFunc("GET /api/search", h.searchDevices)

	// Relationships
	mux.HandleFunc("POST /api/devices/{id}/relationships", h.addRelationship)
	mux.HandleFunc("GET /api/devices/{id}/relationships", h.getRelationships)
	mux.HandleFunc("GET /api/devices/{id}/related", h.getRelatedDevices)
	mux.HandleFunc("DELETE /api/devices/{id}/relationships/{child_id}/{type}", h.removeRelationship)

	// Network Pools
	mux.HandleFunc("GET /api/networks/{id}/pools", h.listNetworkPools)
	mux.HandleFunc("POST /api/networks/{id}/pools", h.createNetworkPool)
	mux.HandleFunc("GET /api/pools/{id}", h.getNetworkPool)
	mux.HandleFunc("PUT /api/pools/{id}", h.updateNetworkPool)
	mux.HandleFunc("DELETE /api/pools/{id}", h.deleteNetworkPool)
	mux.HandleFunc("GET /api/pools/{id}/next-ip", h.getNextIP)
}

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

// writeJSON writes a JSON response
func (h *Handler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes an error response
func (h *Handler) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// internalError logs the error and writes a generic 500 response
func (h *Handler) internalError(w http.ResponseWriter, err error) {
	log.Error("Internal server error", "error", err)
	h.writeError(w, http.StatusInternalServerError, "Internal Server Error")
}

// generateID generates a UUIDv7 for a device
func generateID(name string) string {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.New().String()
	}
	return id.String()
}

// getDefaultDatacenter returns the default datacenter if it exists and is the only one
func (h *Handler) getDefaultDatacenter() *model.Datacenter {
	dcStorage, ok := h.storage.(storage.DatacenterStorage)
	if !ok {
		return nil
	}
	datacenters, err := dcStorage.ListDatacenters(nil)
	if err != nil || len(datacenters) != 1 {
		return nil
	}
	// Return the single existing datacenter as default, regardless of its ID
	return &datacenters[0]
}

// StaticFileHandler serves static files (for the web UI)
type StaticFileHandler struct {
	contentType string
	content     io.ReadSeeker
}

// NewStaticFileHandler creates a handler for serving static content
func NewStaticFileHandler(contentType string, content io.ReadSeeker) http.Handler {
	return &StaticFileHandler{
		contentType: contentType,
		content:     content,
	}
}

func (h *StaticFileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", h.contentType)
	http.ServeContent(w, r, "", time.Now(), h.content)
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
		GetRelationships(deviceID string) ([]storage.Relationship, error)
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

// Datacenter CRUD handlers

// listDatacenters handles GET /api/datacenters
func (h *Handler) listDatacenters(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	filter := &model.DatacenterFilter{Name: name}

	log.Debug("Listing datacenters", "name", name)

	// Check if storage supports datacenters
	dcStorage, ok := h.storage.(storage.DatacenterStorage)
	if !ok {
		log.Warn("Datacenters not supported by storage backend")
		h.writeError(w, http.StatusNotImplemented, "datacenters are not supported by this storage backend")
		return
	}

	datacenters, err := dcStorage.ListDatacenters(filter)
	if err != nil {
		log.Error("Failed to list datacenters", "error", err, "name", name)
		h.internalError(w, err)
		return
	}

	log.Info("Listed datacenters", "count", len(datacenters), "name", name)
	h.writeJSON(w, http.StatusOK, datacenters)
}

// getDatacenter handles GET /api/datacenters/{id}
func (h *Handler) getDatacenter(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		log.Warn("Get datacenter request missing ID")
		h.writeError(w, http.StatusBadRequest, "datacenter ID required")
		return
	}

	log.Debug("Getting datacenter", "id", id)

	dcStorage, ok := h.storage.(storage.DatacenterStorage)
	if !ok {
		log.Warn("Datacenters not supported by storage backend")
		h.writeError(w, http.StatusNotImplemented, "datacenters are not supported by this storage backend")
		return
	}

	datacenter, err := dcStorage.GetDatacenter(id)
	if err != nil {
		if errors.Is(err, storage.ErrDatacenterNotFound) {
			log.Warn("Datacenter not found", "id", id)
			h.writeError(w, http.StatusNotFound, "datacenter not found")
			return
		}
		log.Error("Failed to get datacenter", "error", err, "id", id)
		h.internalError(w, err)
		return
	}

	log.Info("Retrieved datacenter", "id", id, "name", datacenter.Name)
	h.writeJSON(w, http.StatusOK, datacenter)
}

// createDatacenter handles POST /api/datacenters
func (h *Handler) createDatacenter(w http.ResponseWriter, r *http.Request) {
	var datacenter model.Datacenter
	if err := json.NewDecoder(r.Body).Decode(&datacenter); err != nil {
		log.Warn("Invalid datacenter creation request body", "error", err)
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if datacenter.Name == "" {
		log.Warn("Datacenter creation missing required name")
		h.writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	log.Debug("Creating datacenter", "name", datacenter.Name)

	// Generate ID if not provided
	if datacenter.ID == "" {
		datacenter.ID = generateDatacenterID()
	}

	dcStorage, ok := h.storage.(storage.DatacenterStorage)
	if !ok {
		log.Warn("Datacenters not supported by storage backend")
		h.writeError(w, http.StatusNotImplemented, "datacenters are not supported by this storage backend")
		return
	}

	if err := dcStorage.CreateDatacenter(&datacenter); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			log.Warn("Datacenter creation failed - already exists", "name", datacenter.Name)
			h.writeError(w, http.StatusConflict, "datacenter with this name already exists")
			return
		}
		log.Error("Failed to create datacenter", "error", err, "name", datacenter.Name)
		h.internalError(w, err)
		return
	}

	log.Info("Datacenter created successfully", "id", datacenter.ID, "name", datacenter.Name)
	h.writeJSON(w, http.StatusCreated, datacenter)
}

// updateDatacenter handles PUT /api/datacenters/{id}
func (h *Handler) updateDatacenter(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		log.Warn("Update datacenter request missing ID")
		h.writeError(w, http.StatusBadRequest, "datacenter ID required")
		return
	}

	var datacenter model.Datacenter
	if err := json.NewDecoder(r.Body).Decode(&datacenter); err != nil {
		log.Warn("Invalid datacenter update request body", "error", err, "id", id)
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	log.Debug("Updating datacenter", "id", id, "name", datacenter.Name)

	// Ensure ID matches URL
	datacenter.ID = id

	dcStorage, ok := h.storage.(storage.DatacenterStorage)
	if !ok {
		log.Warn("Datacenters not supported by storage backend")
		h.writeError(w, http.StatusNotImplemented, "datacenters are not supported by this storage backend")
		return
	}

	if err := dcStorage.UpdateDatacenter(&datacenter); err != nil {
		if errors.Is(err, storage.ErrDatacenterNotFound) {
			log.Warn("Datacenter update failed - not found", "id", id)
			h.writeError(w, http.StatusNotFound, "datacenter not found")
			return
		}
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			log.Warn("Datacenter update failed - name already exists", "id", id, "name", datacenter.Name)
			h.writeError(w, http.StatusConflict, "datacenter with this name already exists")
			return
		}
		log.Error("Failed to update datacenter", "error", err, "id", id)
		h.internalError(w, err)
		return
	}

	log.Info("Datacenter updated successfully", "id", id, "name", datacenter.Name)
	h.writeJSON(w, http.StatusOK, datacenter)
}

// deleteDatacenter handles DELETE /api/datacenters/{id}
func (h *Handler) deleteDatacenter(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		log.Warn("Delete datacenter request missing ID")
		h.writeError(w, http.StatusBadRequest, "datacenter ID required")
		return
	}

	log.Debug("Deleting datacenter", "id", id)

	dcStorage, ok := h.storage.(storage.DatacenterStorage)
	if !ok {
		log.Warn("Datacenters not supported by storage backend")
		h.writeError(w, http.StatusNotImplemented, "datacenters are not supported by this storage backend")
		return
	}

	if err := dcStorage.DeleteDatacenter(id); err != nil {
		if errors.Is(err, storage.ErrDatacenterNotFound) {
			log.Warn("Datacenter deletion failed - not found", "id", id)
			h.writeError(w, http.StatusNotFound, "datacenter not found")
			return
		}
		log.Error("Failed to delete datacenter", "error", err, "id", id)
		h.internalError(w, err)
		return
	}

	log.Info("Datacenter deleted successfully", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

// getDatacenterDevices handles GET /api/datacenters/{id}/devices
func (h *Handler) getDatacenterDevices(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		log.Warn("Get datacenter devices request missing ID")
		h.writeError(w, http.StatusBadRequest, "datacenter ID required")
		return
	}

	log.Debug("Getting datacenter devices", "datacenter_id", id)

	dcStorage, ok := h.storage.(storage.DatacenterStorage)
	if !ok {
		log.Warn("Datacenters not supported by storage backend")
		h.writeError(w, http.StatusNotImplemented, "datacenters are not supported by this storage backend")
		return
	}

	devices, err := dcStorage.GetDatacenterDevices(id)
	if err != nil {
		log.Error("Failed to get datacenter devices", "error", err, "datacenter_id", id)
		h.internalError(w, err)
		return
	}

	log.Info("Retrieved datacenter devices", "datacenter_id", id, "count", len(devices))
	h.writeJSON(w, http.StatusOK, devices)
}

// generateDatacenterID generates a UUIDv7 for a datacenter
func generateDatacenterID() string {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.New().String()
	}
	return id.String()
}

// Network CRUD handlers

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

// Network Pool Handlers

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
