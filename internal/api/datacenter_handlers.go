package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/martinsuchenak/rackd/internal/log"
	"github.com/martinsuchenak/rackd/internal/model"
	"github.com/martinsuchenak/rackd/internal/storage"
)

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