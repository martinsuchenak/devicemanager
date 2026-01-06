package api

import (
	"encoding/json"
	"io"
	"net/http"
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
	mux.HandleFunc("GET /api/devices/search", h.searchDevices)

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