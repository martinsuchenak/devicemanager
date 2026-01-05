package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandler_Validation_InvalidIP(t *testing.T) {
	handler := setupTestHandler()

	deviceJSON := `{
		"name": "Invalid IP Device",
		"addresses": [
			{
				"ip": "999.999.999.999",
				"type": "ipv4"
			}
		]
	}`

	req := httptest.NewRequest("POST", "/api/devices", bytes.NewReader([]byte(deviceJSON)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.createDevice(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid IP, got %d", resp.StatusCode)
	}
}

func TestHandler_Validation_InvalidSubnet(t *testing.T) {
	handler := setupTestHandler()

	// Mock storage implementation needed for networks
	// Since setupTestHandler uses mockStorage which only implemented Device methods in previous file, 
	// we might fail if we don't extend it. But let's check if the handler validates BEFORE calling storage.
	// Yes, validation happens before storage calls.

	networkJSON := `{
		"name": "Invalid Subnet Network",
		"subnet": "192.168.1.1/99",
		"datacenter_id": "dc-1"
	}`

	req := httptest.NewRequest("POST", "/api/networks", bytes.NewReader([]byte(networkJSON)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.createNetwork(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid subnet, got %d", resp.StatusCode)
	}
}

func TestMiddleware_SecurityHeaders(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := SecurityHeadersMiddleware(nextHandler)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	resp := w.Result()
	
	headers := []string{
		"Content-Security-Policy",
		"Strict-Transport-Security",
		"X-Frame-Options",
		"X-Content-Type-Options",
		"Referrer-Policy",
	}

	for _, h := range headers {
		if resp.Header.Get(h) == "" {
			t.Errorf("Expected header %s to be set", h)
		}
	}
}

func TestMiddleware_Auth(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	token := "secret-token"
	middleware := AuthMiddleware(token, nextHandler)

	tests := []struct {
		name           string
		path           string
		authHeader     string
		expectedStatus int
	}{
		{"No Auth - Non-API Path", "/", "", http.StatusOK},
		{"No Auth - API Path", "/api/devices", "", http.StatusUnauthorized},
		{"Valid Auth - API Path", "/api/devices", "Bearer secret-token", http.StatusOK},
		{"Invalid Auth - API Path", "/api/devices", "Bearer wrong-token", http.StatusUnauthorized},
		{"Query Auth - Disabled", "/api/devices?token=secret-token", "", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()

			middleware.ServeHTTP(w, req)

			if w.Result().StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Result().StatusCode)
			}
		})
	}
}
