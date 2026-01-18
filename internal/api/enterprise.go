package api

import (
	"encoding/json"
	"net/http"

	"github.com/martinsuchenak/rackd/pkg/storage"
)

// EnterpriseHandler handles enterprise-specific HTTP requests
type EnterpriseHandler struct {
	storage storage.Storage
}

// NewEnterpriseHandler creates a new enterprise API handler
func NewEnterpriseHandler(s storage.Storage) *EnterpriseHandler {
	return &EnterpriseHandler{storage: s}
}

// RegisterRoutes registers all enterprise API routes
func (h *EnterpriseHandler) RegisterRoutes(mux *http.ServeMux) {
	// Enterprise features
	mux.HandleFunc("GET /api/enterprise/features", h.listFeatures)
	mux.HandleFunc("GET /api/enterprise/license", h.getLicense)
	mux.HandleFunc("GET /api/enterprise/assets", h.getAssets)

	// Enterprise reports
	mux.HandleFunc("POST /api/enterprise/reports/network", h.generateNetworkReport)
	mux.HandleFunc("POST /api/enterprise/reports/compliance", h.generateComplianceReport)

	// Enterprise device management
	mux.HandleFunc("POST /api/enterprise/devices/bulk-update", h.bulkUpdateDevices)
	mux.HandleFunc("POST /api/enterprise/devices/sync", h.syncDevices)
}

// listFeatures returns available enterprise features
func (h *EnterpriseHandler) listFeatures(w http.ResponseWriter, r *http.Request) {
	features := map[string]interface{}{
		"features": []map[string]string{
			{"name": "premium-scanner", "description": "Advanced network scanning with ping, port, ARP, and service detection"},
			{"name": "scheduled-discovery", "description": "Automated scheduled network discovery"},
			{"name": "bulk-operations", "description": "Bulk device update and sync operations"},
			{"name": "reporting", "description": "Network and compliance reports"},
			{"name": "enterprise-api", "description": "Enterprise-specific API endpoints"},
		},
	}
	h.writeJSON(w, http.StatusOK, features)
}

// getAssets returns the list of enterprise UI assets to load dynamically
func (h *EnterpriseHandler) getAssets(w http.ResponseWriter, r *http.Request) {
	assets := map[string]interface{}{
		"css": []string{"/assets/enterprise.css"},
		"js":  []string{"/assets/enterprise.js"},
		"features": []string{
			"premium-scanner",
			"scheduled-discovery",
			"bulk-operations",
			"reporting",
		},
	}
	h.writeJSON(w, http.StatusOK, assets)
}

// getLicense returns enterprise license information
func (h *EnterpriseHandler) getLicense(w http.ResponseWriter, r *http.Request) {
	license := map[string]interface{}{
		"type":        "enterprise",
		"status":      "active",
		"valid_until": "2026-12-31",
		"features":    []string{"premium-scanner", "scheduled-discovery", "bulk-operations", "reporting"},
	}
	h.writeJSON(w, http.StatusOK, license)
}

// generateNetworkReport generates a network report
func (h *EnterpriseHandler) generateNetworkReport(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusAccepted, map[string]string{
		"message":  "Network report generation started",
		"reportId": "pending",
	})
}

// generateComplianceReport generates a compliance report
func (h *EnterpriseHandler) generateComplianceReport(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusAccepted, map[string]string{
		"message":  "Compliance report generation started",
		"reportId": "pending",
	})
}

// bulkUpdateDevices performs bulk updates on devices
func (h *EnterpriseHandler) bulkUpdateDevices(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]string{
		"message": "Bulk update completed",
		"updated": "0",
	})
}

// syncDevices synchronizes devices with external sources
func (h *EnterpriseHandler) syncDevices(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]string{
		"message": "Device sync completed",
		"synced":  "0",
	})
}

// writeJSON writes a JSON response
func (h *EnterpriseHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes an error response
func (h *EnterpriseHandler) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
