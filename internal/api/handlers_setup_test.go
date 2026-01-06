package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// setupTestHandler creates a new Handler with mock storage
func setupTestHandler() *Handler {
	return NewHandler(newMockStorage())
}

func TestHandler_RegisterRoutes(t *testing.T) {
	handler := setupTestHandler()

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	server := httptest.NewServer(mux)
	defer server.Close()

	// Test list devices
	resp, err := http.Get(server.URL + "/api/devices")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Test search
	resp, err = http.Get(server.URL + "/api/devices/search?q=test")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}
