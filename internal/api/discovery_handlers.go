package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/martinsuchenak/rackd/internal/discovery"
	"github.com/martinsuchenak/rackd/internal/log"
	"github.com/martinsuchenak/rackd/internal/model"
	"github.com/martinsuchenak/rackd/internal/storage"
)

// DiscoveryHandler handles discovery-related HTTP requests
type DiscoveryHandler struct {
	storage storage.DiscoveryStorage
	scanner discovery.Scanner
}

// NewDiscoveryHandler creates a new discovery API handler
func NewDiscoveryHandler(s storage.DiscoveryStorage, sc discovery.Scanner) *DiscoveryHandler {
	return &DiscoveryHandler{storage: s, scanner: sc}
}

// RegisterDiscoveryRoutes registers all discovery API routes
func (h *DiscoveryHandler) RegisterRoutes(mux *http.ServeMux) {
	// Discovered Devices
	mux.HandleFunc("GET /api/discovered", h.listDiscoveredDevices)
	mux.HandleFunc("GET /api/discovered/{id}", h.getDiscoveredDevice)
	mux.HandleFunc("POST /api/discovered/{id}/promote", h.promoteDevice)
	mux.HandleFunc("POST /api/discovered/bulk-promote", h.bulkPromoteDevices)
	mux.HandleFunc("DELETE /api/discovered/{id}", h.deleteDiscoveredDevice)

	// Discovery Scans
	mux.HandleFunc("GET /api/discovery/scans", h.listDiscoveryScans)
	mux.HandleFunc("POST /api/discovery/scans", h.startDiscoveryScan)
	mux.HandleFunc("GET /api/discovery/scans/{id}", h.getDiscoveryScan)
	mux.HandleFunc("DELETE /api/discovery/scans/{id}", h.deleteDiscoveryScan)

	// Discovery Rules
	mux.HandleFunc("GET /api/discovery/rules", h.listDiscoveryRules)
	mux.HandleFunc("POST /api/discovery/rules", h.createDiscoveryRule)
	mux.HandleFunc("GET /api/discovery/rules/{id}", h.getDiscoveryRule)
	mux.HandleFunc("PUT /api/discovery/rules/{id}", h.updateDiscoveryRule)
	mux.HandleFunc("DELETE /api/discovery/rules/{id}", h.deleteDiscoveryRule)
}

// listDiscoveredDevices handles GET /api/discovered
func (h *DiscoveryHandler) listDiscoveredDevices(w http.ResponseWriter, r *http.Request) {
	filter := &model.DiscoveredDeviceFilter{
		NetworkID: r.URL.Query().Get("network_id"),
		Status:    r.URL.Query().Get("status"),
	}

	if r.URL.Query().Get("promoted") == "true" {
		promoted := true
		filter.Promoted = &promoted
	} else if r.URL.Query().Get("promoted") == "false" {
		notPromoted := false
		filter.Promoted = &notPromoted
	}

	devices, err := h.storage.ListDiscoveredDevices(filter)
	if err != nil {
		h.internalError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, devices)
}

// getDiscoveredDevice handles GET /api/discovered/{id}
func (h *DiscoveryHandler) getDiscoveredDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	device, err := h.storage.GetDiscoveredDevice(id)
	if err != nil {
		if errors.Is(err, storage.ErrDiscoveredDeviceNotFound) {
			h.writeError(w, http.StatusNotFound, "discovered device not found")
			return
		}
		h.internalError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, device)
}

// promoteDevice handles POST /api/discovered/{id}/promote
func (h *DiscoveryHandler) promoteDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req model.PromoteDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		h.writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	device, err := h.storage.PromoteDevice(id, &req)
	if err != nil {
		if errors.Is(err, storage.ErrDiscoveredDeviceNotFound) {
			h.writeError(w, http.StatusNotFound, "discovered device not found")
			return
		}
		if strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
			h.writeError(w, http.StatusBadRequest, "invalid datacenter_id or network_id: referenced entity does not exist")
			return
		}
		h.internalError(w, err)
		return
	}

	log.Info("Device promoted", "discovered_id", id, "device_id", device.ID)
	h.writeJSON(w, http.StatusCreated, device)
}

// bulkPromoteDevices handles POST /api/discovered/bulk-promote
func (h *DiscoveryHandler) bulkPromoteDevices(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs     []string                      `json:"ids"`
		Devices []model.PromoteDeviceRequest `json:"devices"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.IDs) == 0 {
		h.writeError(w, http.StatusBadRequest, "ids are required")
		return
	}

	devices, errs := h.storage.BulkPromoteDevices(req.IDs, req.Devices)

	response := map[string]interface{}{
		"promoted": devices,
		"errors":   errs,
	}

	log.Info("Bulk device promotion", "count", len(devices), "errors", len(errs))
	h.writeJSON(w, http.StatusCreated, response)
}

// deleteDiscoveredDevice handles DELETE /api/discovered/{id}
func (h *DiscoveryHandler) deleteDiscoveredDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := h.storage.DeleteDiscoveredDevice(id); err != nil {
		if errors.Is(err, storage.ErrDiscoveredDeviceNotFound) {
			h.writeError(w, http.StatusNotFound, "discovered device not found")
			return
		}
		h.internalError(w, err)
		return
	}

	log.Info("Discovered device deleted", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

// listDiscoveryScans handles GET /api/discovery/scans
func (h *DiscoveryHandler) listDiscoveryScans(w http.ResponseWriter, r *http.Request) {
	networkID := r.URL.Query().Get("network_id")

	scans, err := h.storage.ListDiscoveryScans(networkID)
	if err != nil {
		h.internalError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, scans)
}

// startDiscoveryScan handles POST /api/discovery/scans
func (h *DiscoveryHandler) startDiscoveryScan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NetworkID string `json:"network_id"`
		ScanType  string `json:"scan_type"` // quick, full, deep
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.NetworkID == "" {
		h.writeError(w, http.StatusBadRequest, "network_id is required")
		return
	}

	if req.ScanType == "" {
		req.ScanType = "full"
	}

	// Create scan record
	scan := &model.DiscoveryScan{
		ID:        generateID("discovery_scan"),
		NetworkID: req.NetworkID,
		Status:    "pending",
		ScanType:  req.ScanType,
	}

	if err := h.storage.CreateDiscoveryScan(scan); err != nil {
		h.internalError(w, err)
		return
	}

	// Execute scan in background
	go func() {
		ctx := context.Background()
		// Create a discovery rule for this one-time scan
		rule := &model.DiscoveryRule{
			NetworkID:         req.NetworkID,
			ScanType:          req.ScanType,
			TimeoutSeconds:    5,
			ScanPorts:         req.ScanType != "quick",
			ServiceDetection:  req.ScanType != "quick",
			OSDetection:       req.ScanType == "deep",
		}

		// Run the scan
		if err := h.scanner.ScanNetwork(ctx, req.NetworkID, rule, func(update *model.DiscoveryScan) {
			update.ID = scan.ID
			h.storage.UpdateDiscoveryScan(update)
		}); err != nil {
			log.Error("Discovery scan failed", "scan_id", scan.ID, "error", err)
			scan.Status = "failed"
			scan.ErrorMessage = err.Error()
			now := time.Now()
			scan.CompletedAt = &now
			h.storage.UpdateDiscoveryScan(scan)
		}
	}()

	log.Info("Discovery scan started", "scan_id", scan.ID, "network_id", req.NetworkID)
	h.writeJSON(w, http.StatusCreated, scan)
}

// getDiscoveryScan handles GET /api/discovery/scans/{id}
func (h *DiscoveryHandler) getDiscoveryScan(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	scan, err := h.storage.GetDiscoveryScan(id)
	if err != nil {
		if errors.Is(err, storage.ErrDiscoveryScanNotFound) {
			h.writeError(w, http.StatusNotFound, "scan not found")
			return
		}
		h.internalError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, scan)
}

// deleteDiscoveryScan handles DELETE /api/discovery/scans/{id}
func (h *DiscoveryHandler) deleteDiscoveryScan(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := h.storage.DeleteDiscoveryScan(id); err != nil {
		if errors.Is(err, storage.ErrDiscoveryScanNotFound) {
			h.writeError(w, http.StatusNotFound, "scan not found")
			return
		}
		h.internalError(w, err)
		return
	}

	log.Info("Discovery scan deleted", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

// listDiscoveryRules handles GET /api/discovery/rules
func (h *DiscoveryHandler) listDiscoveryRules(w http.ResponseWriter, r *http.Request) {
	networkID := r.URL.Query().Get("network_id")

	rules, err := h.storage.ListDiscoveryRules(networkID)
	if err != nil {
		h.internalError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, rules)
}

// createDiscoveryRule handles POST /api/discovery/rules
func (h *DiscoveryHandler) createDiscoveryRule(w http.ResponseWriter, r *http.Request) {
	var rule model.DiscoveryRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if rule.NetworkID == "" {
		h.writeError(w, http.StatusBadRequest, "network_id is required")
		return
	}

	rule.ID = generateID("discovery_rule")
	now := time.Now()
	rule.CreatedAt = now
	rule.UpdatedAt = now

	if err := h.storage.CreateDiscoveryRule(&rule); err != nil {
		h.internalError(w, err)
		return
	}

	log.Info("Discovery rule created", "id", rule.ID, "network_id", rule.NetworkID)
	h.writeJSON(w, http.StatusCreated, rule)
}

// getDiscoveryRule handles GET /api/discovery/rules/{id}
func (h *DiscoveryHandler) getDiscoveryRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	rule, err := h.storage.GetDiscoveryRule(id)
	if err != nil {
		if errors.Is(err, storage.ErrDiscoveryRuleNotFound) {
			h.writeError(w, http.StatusNotFound, "rule not found")
			return
		}
		h.internalError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, rule)
}

// updateDiscoveryRule handles PUT /api/discovery/rules/{id}
func (h *DiscoveryHandler) updateDiscoveryRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var rule model.DiscoveryRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	rule.ID = id
	rule.UpdatedAt = time.Now()

	if err := h.storage.UpdateDiscoveryRule(&rule); err != nil {
		if errors.Is(err, storage.ErrDiscoveryRuleNotFound) {
			h.writeError(w, http.StatusNotFound, "rule not found")
			return
		}
		h.internalError(w, err)
		return
	}

	log.Info("Discovery rule updated", "id", id)
	h.writeJSON(w, http.StatusOK, rule)
}

// deleteDiscoveryRule handles DELETE /api/discovery/rules/{id}
func (h *DiscoveryHandler) deleteDiscoveryRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := h.storage.DeleteDiscoveryRule(id); err != nil {
		if errors.Is(err, storage.ErrDiscoveryRuleNotFound) {
			h.writeError(w, http.StatusNotFound, "rule not found")
			return
		}
		h.internalError(w, err)
		return
	}

	log.Info("Discovery rule deleted", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

// Helper methods

func (h *DiscoveryHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *DiscoveryHandler) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func (h *DiscoveryHandler) internalError(w http.ResponseWriter, err error) {
	log.Error("Internal server error", "error", err)
	h.writeError(w, http.StatusInternalServerError, "internal server error")
}
