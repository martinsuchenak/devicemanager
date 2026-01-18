package server

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/martinsuchenak/rackd/internal/api"
	"github.com/martinsuchenak/rackd/internal/config"
	"github.com/martinsuchenak/rackd/pkg/discovery"
	"github.com/martinsuchenak/rackd/internal/log"
	"github.com/martinsuchenak/rackd/internal/mcp"
	"github.com/martinsuchenak/rackd/pkg/registry"
	"github.com/martinsuchenak/rackd/internal/scanner"
	"github.com/martinsuchenak/rackd/internal/storage"
	"github.com/martinsuchenak/rackd/internal/ui"
	"github.com/martinsuchenak/rackd/internal/worker"
	"github.com/paularlott/cli"
)

// initializeDiscoveryFromRegistry attempts to initialize discovery features from the premium registry.
// Falls back to built-in implementations if premium features are not available.
// Returns (scanner, scheduler, handler, usePremium)
func initializeDiscoveryFromRegistry(
	cfg *config.Config,
	discoveryStore storage.DiscoveryStorage,
) (discovery.Scanner, *worker.Scheduler, *api.DiscoveryHandler, bool) {
	reg := registry.GetRegistry()

	var discoveryScanner discovery.Scanner
	var discoveryScheduler *worker.Scheduler
	usePremium := false

	// Try to get premium scanner from registry
	if scannerFactory, ok := reg.GetScannerProvider("discovery"); ok {
		scannerInterface, err := scannerFactory(map[string]interface{}{
			"storage": discoveryStore,
			"config":  cfg,
		})
		if err != nil {
			log.Warn("Failed to create premium scanner, falling back to built-in", "error", err)
		} else {
			var ok bool
			discoveryScanner, ok = scannerInterface.(discovery.Scanner)
			if !ok {
				log.Warn("Premium scanner does not implement discovery.Scanner interface, falling back to built-in")
			} else {
				log.Info("Using premium discovery scanner")
				usePremium = true
			}
		}
	}

	// Fall back to built-in scanner if premium wasn't loaded
	if discoveryScanner == nil {
		discoveryScanner = scanner.NewDiscoveryScanner(discoveryStore)
		log.Info("Using built-in discovery scanner")
	}

	// Create discovery handler
	discoveryHandler := api.NewDiscoveryHandler(discoveryStore, discoveryScanner)

	// Initialize scheduler if discovery is enabled
	if cfg.DiscoveryEnabled {
		log.Info("Discovery enabled, initializing scheduler",
			"interval", cfg.DiscoveryInterval,
			"max_concurrent", cfg.DiscoveryMaxConcurrent,
			"default_scan_type", cfg.DiscoveryDefaultScanType)

		// Try to get premium scheduler from registry
		if schedulerFactory, ok := reg.GetWorkerProvider("discovery-scheduler"); ok {
			schedulerInterface, err := schedulerFactory(map[string]interface{}{
				"storage": discoveryStore,
				"scanner": discoveryScanner,
				"config":  cfg,
			})
			if err != nil {
				log.Warn("Failed to create premium scheduler, falling back to built-in", "error", err)
			} else {
				var ok bool
				discoveryScheduler, ok = schedulerInterface.(*worker.Scheduler)
				if !ok {
					log.Warn("Premium scheduler is not *worker.Scheduler, falling back to built-in")
				} else {
					log.Info("Using premium discovery scheduler")
					usePremium = true
				}
			}
		}

		// Fall back to built-in scheduler if premium wasn't loaded
		if discoveryScheduler == nil {
			discoveryScheduler = worker.NewScheduler(discoveryStore, discoveryScanner)
			log.Info("Using built-in discovery scheduler")
		}

		// Start scheduler in background
		discoveryScheduler.Start()
		log.Info("Discovery scheduler started")
	} else {
		log.Info("Discovery disabled (scheduler not running). Manual scans via UI/API are still available.")
	}

	return discoveryScanner, discoveryScheduler, discoveryHandler, usePremium
}

// initializeEnterpriseRoutes registers enterprise-specific routes from the registry
func initializeEnterpriseRoutes(mux *http.ServeMux, reg *registry.Registry, store storage.Storage) {
	// Check for enterprise API handler
	if handlerFactory, ok := reg.GetAPIHandler("enterprise"); ok {
		handlerInterface := handlerFactory(map[string]interface{}{
			"storage": store,
		})
		if handlerInterface != nil {
			// Check if handler implements RegisterRoutes method
			type routeRegisterer interface {
				RegisterRoutes(*http.ServeMux)
			}
			if handler, ok := handlerInterface.(routeRegisterer); ok {
				handler.RegisterRoutes(mux)
				log.Info("Enterprise API routes registered")
			}
		}
	}
}

// initializeEnterpriseAssets registers enterprise-specific asset handlers from the registry
func initializeEnterpriseAssets(mux *http.ServeMux, reg *registry.Registry) {
	// Check for enterprise asset handler registration function
	// The registry stores functions that take a mux and register asset handlers
	type assetRegisterer func(*http.ServeMux)

	if assetHandler, exists := reg.GetFeature("enterprise-assets"); exists {
		if registerFn, ok := assetHandler.(assetRegisterer); ok {
			registerFn(mux)
			log.Info("Enterprise asset handlers registered")
		}
	}
}

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

			// Get discovery storage and create discovery handler
			discoveryStore, ok := store.(storage.DiscoveryStorage)
			var discoveryHandler *api.DiscoveryHandler
			var discoveryScheduler *worker.Scheduler

			if ok {
				log.Info("Discovery storage initialized")

				// Initialize discovery features from registry (with fallback to built-in)
				_, discoveryScheduler, discoveryHandler, _ = initializeDiscoveryFromRegistry(cfg, discoveryStore)

				// Defer stopping the scheduler if it was created
				if discoveryScheduler != nil {
					defer func() {
						log.Info("Stopping discovery scheduler...")
						discoveryScheduler.Stop()
						log.Info("Discovery scheduler stopped")
					}()
				}
			} else {
				log.Warn("Storage does not support discovery, discovery features will be unavailable")
			}

			// Create MCP server
			mcpServer := mcp.NewServer(store, cfg.MCPAuthToken)

			// Setup HTTP routes
			mux := http.NewServeMux()

			// API routes
			apiHandler.RegisterRoutes(mux)

			// Discovery API routes
			if discoveryHandler != nil {
				discoveryHandler.RegisterRoutes(mux)
			}

			// Enterprise routes (if registered)
			initializeEnterpriseRoutes(mux, registry.GetRegistry(), store)

			// Enterprise assets (if registered)
			initializeEnterpriseAssets(mux, registry.GetRegistry())

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