package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/martinsuchenak/devicemanager/internal/model"
	"github.com/martinsuchenak/devicemanager/internal/storage"
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
	mux.HandleFunc("GET /api/devices", h.listDevices)
	mux.HandleFunc("POST /api/devices", h.createDevice)
	mux.HandleFunc("GET /api/devices/", h.getDevice)
	mux.HandleFunc("PUT /api/devices/", h.updateDevice)
	mux.HandleFunc("DELETE /api/devices/", h.deleteDevice)
	mux.HandleFunc("GET /api/search", h.searchDevices)

	// Relationship endpoints (SQLite only)
	mux.HandleFunc("POST /api/devices/", h.addRelationship)
	mux.HandleFunc("GET /api/devices/", h.getRelationships)
	mux.HandleFunc("GET /api/devices/", h.getRelatedDevices)
	mux.HandleFunc("DELETE /api/devices/", h.removeRelationship)
}

// listDevices handles GET /api/devices
func (h *Handler) listDevices(w http.ResponseWriter, r *http.Request) {
	tags := r.URL.Query()["tag"]
	filter := &model.DeviceFilter{Tags: tags}

	devices, err := h.storage.ListDevices(filter)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, devices)
}

// getDevice handles GET /api/devices/{id}
func (h *Handler) getDevice(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/devices/")
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
		h.writeError(w, http.StatusInternalServerError, err.Error())
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

	// Generate ID if not provided
	if device.ID == "" {
		device.ID = generateID(device.Name)
	}

	// Set timestamps
	now := time.Now()
	device.CreatedAt = now
	device.UpdatedAt = now

	if err := h.storage.CreateDevice(&device); err != nil {
		if err == storage.ErrInvalidID {
			h.writeError(w, http.StatusBadRequest, "invalid device ID")
			return
		}
		if strings.Contains(err.Error(), "already exists") {
			h.writeError(w, http.StatusConflict, "device already exists")
			return
		}
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, device)
}

// updateDevice handles PUT /api/devices/{id}
func (h *Handler) updateDevice(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/devices/")
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

	if err := h.storage.UpdateDevice(&device); err != nil {
		if errors.Is(err, storage.ErrDeviceNotFound) {
			h.writeError(w, http.StatusNotFound, "device not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, device)
}

// deleteDevice handles DELETE /api/devices/{id}
func (h *Handler) deleteDevice(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/devices/")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "device ID required")
		return
	}

	if err := h.storage.DeleteDevice(id); err != nil {
		if errors.Is(err, storage.ErrDeviceNotFound) {
			h.writeError(w, http.StatusNotFound, "device not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, err.Error())
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
		h.writeError(w, http.StatusInternalServerError, err.Error())
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

// generateID generates a simple ID from a name
func generateID(name string) string {
	// Simple ID generation - could use UUID in production
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), " ", "-")) + "-" + time.Now().Format("20060102150405")
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
	// Extract device ID from path: /api/devices/{id}/relationships
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/devices/"), "/")
	if len(parts) < 2 || parts[1] != "relationships" {
		h.writeError(w, http.StatusBadRequest, "invalid URL format")
		return
	}
	deviceID := parts[0]

	if deviceID == "" {
		h.writeError(w, http.StatusBadRequest, "device ID required")
		return
	}

	var req struct {
		ChildID         string `json:"child_id"`
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
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"message":         "relationship created",
		"parent_id":        deviceID,
		"child_id":         req.ChildID,
		"relationship_type": req.RelationshipType,
	})
}

// getRelationships handles GET /api/devices/{id}/relationships
func (h *Handler) getRelationships(w http.ResponseWriter, r *http.Request) {
	// Extract device ID from path: /api/devices/{id}/relationships
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/devices/"), "/")
	if len(parts) < 2 || parts[1] != "relationships" {
		h.writeError(w, http.StatusBadRequest, "invalid URL format")
		return
	}
	deviceID := parts[0]

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
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, relationships)
}

// getRelatedDevices handles GET /api/devices/{id}/related
func (h *Handler) getRelatedDevices(w http.ResponseWriter, r *http.Request) {
	// Extract device ID from path: /api/devices/{id}/related
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/devices/"), "/")
	if len(parts) < 2 || parts[1] != "related" {
		h.writeError(w, http.StatusBadRequest, "invalid URL format")
		return
	}
	deviceID := parts[0]

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
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, devices)
}

// removeRelationship handles DELETE /api/devices/{id}/relationships
func (h *Handler) removeRelationship(w http.ResponseWriter, r *http.Request) {
	// Extract device ID from path: /api/devices/{id}/relationships/{child_id}/{type}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/devices/"), "/")
	if len(parts) < 4 || parts[1] != "relationships" {
		h.writeError(w, http.StatusBadRequest, "invalid URL format")
		return
	}
	deviceID := parts[0]
	childID := parts[2]
	relType := parts[3]

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
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
