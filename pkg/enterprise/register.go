// Package enterprise provides premium/enterprise features for rackd
// This package registers premium scanner, API handlers, and MCP tools to the registry
package enterprise

import (
	"time"

	"github.com/martinsuchenak/rackd/internal/api"
	"github.com/martinsuchenak/rackd/internal/scannerpremium"
	"github.com/martinsuchenak/rackd/pkg/registry"
	"github.com/martinsuchenak/rackd/pkg/storage"
)

func init() {
	reg := registry.GetRegistry()

	// Register premium scanner provider
	reg.RegisterScannerProvider("discovery", func(config map[string]interface{}) (interface{}, error) {
		store, ok := config["storage"].(storage.DiscoveryStorage)
		if !ok {
			return nil, nil
		}

		// Create premium scanner options
		options := &scannerpremium.PremiumScanOptions{
			Privileged:       true, // Try to use raw sockets
			PingTimeout:      2 * time.Second,
			PortTimeout:      500 * time.Millisecond,
			ARPTimeout:       500 * time.Millisecond,
			PortScanType:     "common", // common ports by default
			ServiceDetection: true,
			ARPScan:          true,
			OSDetection:      true,
			MaxConcurrency:   50,
		}

		// Return premium scanner
		scanner := scannerpremium.NewPremiumScanner(store, options)
		return scanner, nil
	})

	// Register enterprise API handler
	reg.RegisterAPIHandler("enterprise", func(config map[string]interface{}) interface{} {
		store, ok := config["storage"].(storage.Storage)
		if !ok {
			return nil
		}
		return api.NewEnterpriseHandler(store)
	})

	// Register enterprise MCP tools
	reg.RegisterMCPTools("enterprise", func(config map[string]interface{}) []interface{} {
		// Return enterprise-specific MCP tools
		// Tools will be registered with the MCP server during initialization
		return []interface{}{
			map[string]string{
				"name":        "enterprise_generate_report",
				"description": "Generate enterprise network or compliance reports",
			},
			map[string]string{
				"name":        "enterprise_bulk_update",
				"description": "Perform bulk updates on devices",
			},
		}
	})

	// Note: The scheduler is created by OSS server using the registered scanner
	// We don't need to register a scheduler provider since the OSS worker package
	// already handles scheduling with the discovery.Scanner interface
}
