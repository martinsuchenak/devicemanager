package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/martinsuchenak/devicemanager/internal/api"
	"github.com/martinsuchenak/devicemanager/internal/config"
	"github.com/martinsuchenak/devicemanager/internal/mcp"
	"github.com/martinsuchenak/devicemanager/internal/storage"
	"github.com/martinsuchenak/devicemanager/internal/ui"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize storage
	store, err := storage.NewFileStorage(cfg.DataDir, cfg.StorageFormat)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	log.Printf("Storage initialized: %s (%s format)", cfg.DataDir, cfg.StorageFormat)

	// Create API handler
	apiHandler := api.NewHandler(store)

	// Create MCP server
	mcpServer := mcp.NewServer(store, cfg.BearerToken)

	// Setup HTTP routes
	mux := http.NewServeMux()

	// API routes
	apiHandler.RegisterRoutes(mux)

	// MCP endpoint
	mux.HandleFunc("/mcp", mcpServer.GetHTTPHandler())

	// Serve web UI at root (handles all / and /assets/* requests)
	mux.Handle("/", ui.AssetHandler())

	// Start server
	server := &http.Server{
		Addr: cfg.ListenAddr,
		Handler: mux,
	}

	// Handle shutdown gracefully
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down server...")
		server.Close()
	}()

	// Log startup info
	log.Printf("Starting Device Manager server on %s", cfg.ListenAddr)
	log.Printf("Web UI: http://localhost%s", cfg.ListenAddr)
	log.Printf("API: http://localhost%s/api/", cfg.ListenAddr)
	log.Printf("MCP: http://localhost%s/mcp", cfg.ListenAddr)
	mcpServer.LogStartup()

	// Start serving
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}

	log.Println("Server stopped")
}
