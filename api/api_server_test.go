package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/martinsuchenak/rackd/internal/api"
	"github.com/martinsuchenak/rackd/internal/storage"
)

// TestServer is a helper for integration tests
type TestServer struct {
	server   *httptest.Server
	handler  *api.Handler
	storage  storage.Storage
	stopChan chan struct{}
}

// NewTestServer creates a new test server
func NewTestServer(t *testing.T) *TestServer {
	t.Helper()

	tmpDir := t.TempDir()

	store, err := storage.NewSQLiteStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	handler := api.NewHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	server := httptest.NewServer(mux)

	return &TestServer{
		server:  server,
		handler: handler,
		storage: store,
	}
}

// Close stops the test server
func (ts *TestServer) Close() {
	if ts.server != nil {
		ts.server.Close()
	}
}

// URL returns the base URL of the test server
func (ts *TestServer) URL() string {
	return ts.server.URL
}

// DeviceJSON is a helper for creating device JSON
func DeviceJSON(name string, extra map[string]interface{}) []byte {
	device := map[string]interface{}{
		"name": name,
	}
	for k, v := range extra {
		device[k] = v
	}
	data, _ := json.Marshal(device)
	return data
}
