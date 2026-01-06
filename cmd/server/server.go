package server

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/martinsuchenak/rackd/internal/api"
	"github.com/martinsuchenak/rackd/internal/config"
	"github.com/martinsuchenak/rackd/internal/log"
	"github.com/martinsuchenak/rackd/internal/mcp"
	"github.com/martinsuchenak/rackd/internal/storage"
	"github.com/martinsuchenak/rackd/internal/ui"
	"github.com/paularlott/cli"
)

func Command() *cli.Command {
	return &cli.Command{
		Name:        "server",
		Usage:       "Start the Rackd server",
		Description: "Start the HTTP server with web UI, API, and MCP endpoints",
		Flags:       config.GetFlags(),
		Run: func(ctx context.Context, cmd *cli.Command) error {
			cfg := config.Load()

			log.Info("Configuration loaded", "data_dir", cfg.DataDir, "listen_addr", cfg.ListenAddr)

			// Initialize storage (SQLite only)
			store, err := storage.NewStorage(cfg.DataDir, "", "")
			if err != nil {
				log.Error("Failed to initialize storage", "error", err)
				return err
			}
			log.Info("Storage initialized", "backend", "SQLite", "path", cfg.DataDir)

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
				log.Info("Shutting down server...")
				server.Close()
			}()

			// Log startup info
			log.Info("Starting Rackd server", "addr", cfg.ListenAddr)
			log.Info("Web UI available", "url", "http://localhost"+cfg.ListenAddr)
			log.Info("API available", "url", "http://localhost"+cfg.ListenAddr+"/api/")
			log.Info("MCP available", "url", "http://localhost"+cfg.ListenAddr+"/mcp")
			if cfg.IsMCPEnabled() {
				log.Info("MCP authentication enabled")
			}
			if cfg.IsAPIAuthEnabled() {
				log.Info("API authentication enabled")
			}
			mcpServer.LogStartup()

			// Start serving
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error("Server error", "error", err)
				return err
			}

			log.Info("Server stopped")
			return nil
		},
	}
}