package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/martinsuchenak/rackd/internal/api"
	"github.com/martinsuchenak/rackd/internal/config"
	"github.com/martinsuchenak/rackd/internal/mcp"
	"github.com/martinsuchenak/rackd/internal/storage"
	"github.com/martinsuchenak/rackd/internal/ui"
)

func main() {
	// Command-line flags (highest priority)
	dataDir := flag.String("data-dir", "", "Data directory path")
	listenAddr := flag.String("addr", "", "Server listen address (e.g., :8080)")
	bearerToken := flag.String("token", "", "MCP bearer token for authentication")
	apiToken := flag.String("api-token", "", "API bearer token for authentication")
	showVersion := flag.Bool("version", false, "Show version information")
	showHelp := flag.Bool("help", false, "Show help information")

	flag.Parse()

	// Show help if requested
	if *showHelp {
		flag.Usage()
		os.Exit(0)
	}

	// Show version if requested
	if *showVersion {
		println("Rackd v1.0.0")
		os.Exit(0)
	}

	// Load configuration with priority: CLI flags > .env file > ENV vars > defaults
	cfg := &config.Config{}

	// Apply CLI flags if provided
	cliOpts := &config.Config{}
	if *dataDir != "" {
		cliOpts.DataDir = *dataDir
	}
	if *listenAddr != "" {
		cliOpts.ListenAddr = *listenAddr
	}
	if *bearerToken != "" {
		cliOpts.MCPAuthToken = *bearerToken
	}
	if *apiToken != "" {
		cliOpts.APIAuthToken = *apiToken
	}

	// If any CLI flag was set, use it to override all other sources
	if *dataDir != "" || *listenAddr != "" || *bearerToken != "" || *apiToken != "" {
		cfg = config.Load(cliOpts)
	} else {
		// No CLI flags, load from .env file or ENV vars
		cfg = config.Load(nil)
	}

	// Log config source
	log.Printf("Configuration loaded from: %s", cfg)

	// Initialize storage (SQLite only)
	store, err := storage.NewStorage(cfg.DataDir, "", "")
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	log.Printf("Storage initialized: %s (SQLite backend)", cfg.DataDir)

	// Create API handler
	apiHandler := api.NewHandler(store)

	// Create MCP server
	mcpServer := mcp.NewServer(store, cfg.MCPAuthToken)

	// Setup HTTP routes
	mux := http.NewServeMux()

	// API routes
	apiHandler.RegisterRoutes(mux)

	// MCP endpoint
	mux.HandleFunc("/mcp", mcpServer.GetHTTPHandler())

	// Serve web UI at root (handles all / and /assets/* requests)
	mux.Handle("/", ui.AssetHandler())

	// Apply middleware
	var handler http.Handler = mux
	if cfg.IsAPIAuthEnabled() {
		handler = api.AuthMiddleware(cfg.APIAuthToken, handler)
	}
	handler = api.SecurityHeadersMiddleware(handler)

	// Start server
	server := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: handler,
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
	log.Printf("Starting Rackd server on %s", cfg.ListenAddr)
	log.Printf("Web UI: http://localhost%s", cfg.ListenAddr)
	log.Printf("API: http://localhost%s/api/", cfg.ListenAddr)
	log.Printf("MCP: http://localhost%s/mcp", cfg.ListenAddr)
	if cfg.IsMCPEnabled() {
		log.Printf("MCP authentication: Enabled (bearer token required)")
	}
	if cfg.IsAPIAuthEnabled() {
		log.Printf("API authentication: Enabled")
	}
	mcpServer.LogStartup()

	// Start serving
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}

	log.Println("Server stopped")
}
