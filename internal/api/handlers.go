package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
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

	devices, err := h.storage.ListDevices(filter)
	if err != nil {
		h.internalError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, devices)
}

// getDevice handles GET /api/devices/{id}
func (h *Handler) getDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "device ID required")
		return
	}

	device, err := h.storage.GetDevice(id)
	if err != nil {
		if errors.Is(err, storage.ErrDeviceNotFound) {
			h.writeError(w, http.StatusNotFound, "device not found")
			return
		}
		h.internalError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, device)
}

// createDevice handles POST /api/devices
func (h *Handler) createDevice(w http.ResponseWriter, r *http.Request) {
	var device model.Device
	if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if device.Name == "" {
		h.writeError(w, http.StatusBadRequest, "name is required")
		return
	}

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
			h.writeError(w, http.StatusBadRequest, "invalid device ID")
			return
		}
		if strings.Contains(err.Error(), "already exists") {
			h.writeError(w, http.StatusConflict, "device already exists")
			return
		}
		h.internalError(w, err)
		return
	}

	h.writeJSON(w, http.StatusCreated, device)
}

// updateDevice handles PUT /api/devices/{id}
func (h *Handler) updateDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "device ID required")
		return
	}

	var device model.Device
	if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

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
			h.writeError(w, http.StatusNotFound, "device not found")
			return
		}
		h.internalError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, device)
}

// deleteDevice handles DELETE /api/devices/{id}
func (h *Handler) deleteDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "device ID required")
		return
	}

	if err := h.storage.DeleteDevice(id); err != nil {
		if errors.Is(err, storage.ErrDeviceNotFound) {
			h.writeError(w, http.StatusNotFound, "device not found")
			return
		}
		h.internalError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// searchDevices handles GET /api/search?q=
func (h *Handler) searchDevices(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		h.writeError(w, http.StatusBadRequest, "search query required")
		return
	}

	devices, err := h.storage.SearchDevices(query)
	if err != nil {
		h.internalError(w, err)
		return
	}

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
	log.Printf("Internal Server Error: %v", err)
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
		h.writeError(w, http.StatusBadRequest, "device ID required")
		return
	}

	var req struct {
		ChildID          string `json:"child_id"`
		RelationshipType string `json:"relationship_type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ChildID == "" {
		h.writeError(w, http.StatusBadRequest, "child_id is required")
		return
	}

	if req.RelationshipType == "" {
		req.RelationshipType = "related"
	}

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
			h.writeError(w, http.StatusNotFound, "device not found")
			return
		}
		h.internalError(w, err)
		return
	}

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

	// Check if storage supports relationships
	relStorage, ok := h.storage.(interface {
		GetRelationships(deviceID string) ([]storage.Relationship, error)
	})
	if !ok {
		h.writeError(w, http.StatusNotImplemented, "relationships are not supported by this storage backend")
		return
	}

	relationships, err := relStorage.GetRelationships(deviceID)
	if err != nil {
		h.internalError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, relationships)
}

// getRelatedDevices handles GET /api/devices/{id}/related
func (h *Handler) getRelatedDevices(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")

	// Get relationship type from query parameter
	relType := r.URL.Query().Get("type")

	// Check if storage supports relationships
	relStorage, ok := h.storage.(interface {
		GetRelatedDevices(deviceID, relationshipType string) ([]model.Device, error)
	})
	if !ok {
		h.writeError(w, http.StatusNotImplemented, "relationships are not supported by this storage backend")
		return
	}

	devices, err := relStorage.GetRelatedDevices(deviceID, relType)
	if err != nil {
		h.internalError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, devices)
}

// removeRelationship handles DELETE /api/devices/{id}/relationships
func (h *Handler) removeRelationship(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")
	childID := r.PathValue("child_id")
	relType := r.PathValue("type")

	if deviceID == "" || childID == "" {
		h.writeError(w, http.StatusBadRequest, "device ID and child ID required")
		return
	}

	// Check if storage supports relationships
	relStorage, ok := h.storage.(interface {
		RemoveRelationship(parentID, childID, relationshipType string) error
	})
	if !ok {
		h.writeError(w, http.StatusNotImplemented, "relationships are not supported by this storage backend")
		return
	}

	if err := relStorage.RemoveRelationship(deviceID, childID, relType); err != nil {
		if errors.Is(err, storage.ErrDeviceNotFound) {
			h.writeError(w, http.StatusNotFound, "device or relationship not found")
			return
		}
		h.internalError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Datacenter CRUD handlers

// listDatacenters handles GET /api/datacenters
func (h *Handler) listDatacenters(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	filter := &model.DatacenterFilter{Name: name}

	// Check if storage supports datacenters
	dcStorage, ok := h.storage.(storage.DatacenterStorage)
	if !ok {
		h.writeError(w, http.StatusNotImplemented, "datacenters are not supported by this storage backend")
		return
	}

	datacenters, err := dcStorage.ListDatacenters(filter)
	if err != nil {
		h.internalError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, datacenters)
}

// getDatacenter handles GET /api/datacenters/{id}
func (h *Handler) getDatacenter(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "datacenter ID required")
		return
	}

	dcStorage, ok := h.storage.(storage.DatacenterStorage)
	if !ok {
		h.writeError(w, http.StatusNotImplemented, "datacenters are not supported by this storage backend")
		return
	}

	datacenter, err := dcStorage.GetDatacenter(id)
	if err != nil {
		if errors.Is(err, storage.ErrDatacenterNotFound) {
			h.writeError(w, http.StatusNotFound, "datacenter not found")
			return
		}
		h.internalError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, datacenter)
}

// createDatacenter handles POST /api/datacenters
func (h *Handler) createDatacenter(w http.ResponseWriter, r *http.Request) {
	var datacenter model.Datacenter
	if err := json.NewDecoder(r.Body).Decode(&datacenter); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if datacenter.Name == "" {
		h.writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	// Generate ID if not provided
	if datacenter.ID == "" {
		datacenter.ID = generateDatacenterID()
	}

	dcStorage, ok := h.storage.(storage.DatacenterStorage)
	if !ok {
		h.writeError(w, http.StatusNotImplemented, "datacenters are not supported by this storage backend")
		return
	}

	if err := dcStorage.CreateDatacenter(&datacenter); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			h.writeError(w, http.StatusConflict, "datacenter with this name already exists")
			return
		}
		h.internalError(w, err)
		return
	}

	h.writeJSON(w, http.StatusCreated, datacenter)
}

// updateDatacenter handles PUT /api/datacenters/{id}
func (h *Handler) updateDatacenter(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "datacenter ID required")
		return
	}

	var datacenter model.Datacenter
	if err := json.NewDecoder(r.Body).Decode(&datacenter); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Ensure ID matches URL
	datacenter.ID = id

	dcStorage, ok := h.storage.(storage.DatacenterStorage)
	if !ok {
		h.writeError(w, http.StatusNotImplemented, "datacenters are not supported by this storage backend")
		return
	}

	if err := dcStorage.UpdateDatacenter(&datacenter); err != nil {
		if errors.Is(err, storage.ErrDatacenterNotFound) {
			h.writeError(w, http.StatusNotFound, "datacenter not found")
			return
		}
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			h.writeError(w, http.StatusConflict, "datacenter with this name already exists")
			return
		}
		h.internalError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, datacenter)
}

// deleteDatacenter handles DELETE /api/datacenters/{id}
func (h *Handler) deleteDatacenter(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "datacenter ID required")
		return
	}

	dcStorage, ok := h.storage.(storage.DatacenterStorage)
	if !ok {
		h.writeError(w, http.StatusNotImplemented, "datacenters are not supported by this storage backend")
		return
	}

	if err := dcStorage.DeleteDatacenter(id); err != nil {
		if errors.Is(err, storage.ErrDatacenterNotFound) {
			h.writeError(w, http.StatusNotFound, "datacenter not found")
			return
		}
		h.internalError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// getDatacenterDevices handles GET /api/datacenters/{id}/devices
func (h *Handler) getDatacenterDevices(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "datacenter ID required")
		return
	}

	dcStorage, ok := h.storage.(storage.DatacenterStorage)
	if !ok {
		h.writeError(w, http.StatusNotImplemented, "datacenters are not supported by this storage backend")
		return
	}

	devices, err := dcStorage.GetDatacenterDevices(id)
	if err != nil {
		h.internalError(w, err)
		return
	}

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

	// Check if storage supports networks
	netStorage, ok := h.storage.(storage.NetworkStorage)
	if !ok {
		h.writeError(w, http.StatusNotImplemented, "networks are not supported by this storage backend")
		return
	}

	networks, err := netStorage.ListNetworks(filter)
	if err != nil {
		h.internalError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, networks)
}

// getNetwork handles GET /api/networks/{id}
func (h *Handler) getNetwork(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "network ID required")
		return
	}

	netStorage, ok := h.storage.(storage.NetworkStorage)
	if !ok {
		h.writeError(w, http.StatusNotImplemented, "networks are not supported by this storage backend")
		return
	}

	network, err := netStorage.GetNetwork(id)
	if err != nil {
		if errors.Is(err, storage.ErrNetworkNotFound) {
			h.writeError(w, http.StatusNotFound, "network not found")
			return
		}
		h.internalError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, network)
}

// createNetwork handles POST /api/networks
func (h *Handler) createNetwork(w http.ResponseWriter, r *http.Request) {
	var network model.Network
	if err := json.NewDecoder(r.Body).Decode(&network); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if network.Name == "" {
		h.writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if network.Subnet == "" {
		h.writeError(w, http.StatusBadRequest, "subnet is required")
		return
	}
	if _, _, err := net.ParseCIDR(network.Subnet); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid subnet CIDR: "+network.Subnet)
		return
	}

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
			h.writeError(w, http.StatusConflict, "network with this name already exists")
			return
		}
		if strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
			h.writeError(w, http.StatusBadRequest, "datacenter not found")
			return
		}
		h.internalError(w, err)
		return
	}

	h.writeJSON(w, http.StatusCreated, network)
}

// updateNetwork handles PUT /api/networks/{id}
func (h *Handler) updateNetwork(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "network ID required")
		return
	}

	var network model.Network
	if err := json.NewDecoder(r.Body).Decode(&network); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Ensure ID matches URL
	network.ID = id

	// Validate subnet if provided (though it's required in model, JSON decode might leave it empty or partially filled)
	if network.Subnet != "" {
		if _, _, err := net.ParseCIDR(network.Subnet); err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid subnet CIDR: "+network.Subnet)
			return
		}
	}

	netStorage, ok := h.storage.(storage.NetworkStorage)
	if !ok {
		h.writeError(w, http.StatusNotImplemented, "networks are not supported by this storage backend")
		return
	}

	if err := netStorage.UpdateNetwork(&network); err != nil {
		if errors.Is(err, storage.ErrNetworkNotFound) {
			h.writeError(w, http.StatusNotFound, "network not found")
			return
		}
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			h.writeError(w, http.StatusConflict, "network with this name already exists")
			return
		}
		if strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
			h.writeError(w, http.StatusBadRequest, "datacenter not found")
			return
		}
		h.internalError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, network)
}

// deleteNetwork handles DELETE /api/networks/{id}
func (h *Handler) deleteNetwork(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "network ID required")
		return
	}

	netStorage, ok := h.storage.(storage.NetworkStorage)
	if !ok {
		h.writeError(w, http.StatusNotImplemented, "networks are not supported by this storage backend")
		return
	}

	if err := netStorage.DeleteNetwork(id); err != nil {
		if errors.Is(err, storage.ErrNetworkNotFound) {
			h.writeError(w, http.StatusNotFound, "network not found")
			return
		}
		h.internalError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// getNetworkDevices handles GET /api/networks/{id}/devices
func (h *Handler) getNetworkDevices(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "network ID required")
		return
	}

	netStorage, ok := h.storage.(storage.NetworkStorage)
	if !ok {
		h.writeError(w, http.StatusNotImplemented, "networks are not supported by this storage backend")
		return
	}

	devices, err := netStorage.GetNetworkDevices(id)
	if err != nil {
		h.internalError(w, err)
		return
	}

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
		h.writeError(w, http.StatusBadRequest, "network ID is required")
		return
	}

	poolStorage, ok := h.storage.(storage.NetworkPoolStorage)
	if !ok {
		h.writeError(w, http.StatusNotImplemented, "network pools not supported by storage backend")
		return
	}

	pools, err := poolStorage.ListNetworkPools(&model.NetworkPoolFilter{NetworkID: networkID})
	if err != nil {
		h.internalError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, pools)
}

func (h *Handler) getNetworkPool(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "pool ID is required")
		return
	}

	poolStorage, ok := h.storage.(storage.NetworkPoolStorage)
	if !ok {
		h.writeError(w, http.StatusNotImplemented, "network pools not supported by storage backend")
		return
	}

	pool, err := poolStorage.GetNetworkPool(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.writeError(w, http.StatusNotFound, "network pool not found")
			return
		}
		h.internalError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, pool)
}

func (h *Handler) createNetworkPool(w http.ResponseWriter, r *http.Request) {
	networkID := r.PathValue("id") // From /api/networks/{id}/pools
	if networkID == "" {
		h.writeError(w, http.StatusBadRequest, "network ID is required")
		return
	}

	var pool model.NetworkPool
	if err := json.NewDecoder(r.Body).Decode(&pool); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	pool.NetworkID = networkID
	if pool.Name == "" {
		h.writeError(w, http.StatusBadRequest, "pool name is required")
		return
	}
	if pool.StartIP == "" || pool.EndIP == "" {
		h.writeError(w, http.StatusBadRequest, "start_ip and end_ip are required")
		return
	}
	if net.ParseIP(pool.StartIP) == nil || net.ParseIP(pool.EndIP) == nil {
		h.writeError(w, http.StatusBadRequest, "invalid IP address format")
		return
	}

	if pool.ID == "" {
		pool.ID = generateID(pool.Name)
	}

	poolStorage, ok := h.storage.(storage.NetworkPoolStorage)
	if !ok {
		h.writeError(w, http.StatusNotImplemented, "network pools not supported by storage backend")
		return
	}

	if err := poolStorage.CreateNetworkPool(&pool); err != nil {
		if strings.Contains(err.Error(), "already exists") { // Assuming unique name/network constraint
			h.writeError(w, http.StatusConflict, "network pool already exists")
			return
		}
		h.internalError(w, err)
		return
	}

	h.writeJSON(w, http.StatusCreated, pool)
}

func (h *Handler) updateNetworkPool(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "pool ID is required")
		return
	}

	var pool model.NetworkPool
	if err := json.NewDecoder(r.Body).Decode(&pool); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	pool.ID = id

	poolStorage, ok := h.storage.(storage.NetworkPoolStorage)
	if !ok {
		h.writeError(w, http.StatusNotImplemented, "network pools not supported by storage backend")
		return
	}

	if err := poolStorage.UpdateNetworkPool(&pool); err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.writeError(w, http.StatusNotFound, "network pool not found")
			return
		}
		h.internalError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, pool)
}

func (h *Handler) deleteNetworkPool(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "pool ID is required")
		return
	}

	poolStorage, ok := h.storage.(storage.NetworkPoolStorage)
	if !ok {
		h.writeError(w, http.StatusNotImplemented, "network pools not supported by storage backend")
		return
	}

	if err := poolStorage.DeleteNetworkPool(id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.writeError(w, http.StatusNotFound, "network pool not found")
			return
		}
		h.internalError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"message": "network pool deleted"})
}

func (h *Handler) getNextIP(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "pool ID is required")
		return
	}

	poolStorage, ok := h.storage.(storage.NetworkPoolStorage)
	if !ok {
		h.writeError(w, http.StatusNotImplemented, "network pools not supported by storage backend")
		return
	}

	ip, err := poolStorage.GetNextAvailableIP(id)
	if err != nil {
		if strings.Contains(err.Error(), "no available IPs") {
			h.writeError(w, http.StatusConflict, "no available IPs in pool")
			return
		}
		h.internalError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"ip": ip})
}
