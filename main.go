package main

import (
	"context"
	"os"

	"github.com/martinsuchenak/rackd/cmd/datacenter"
	"github.com/martinsuchenak/rackd/cmd/device"
	"github.com/martinsuchenak/rackd/cmd/discovery"
	"github.com/martinsuchenak/rackd/cmd/network"
	"github.com/martinsuchenak/rackd/cmd/server"
	"github.com/martinsuchenak/rackd/internal/log"
	"github.com/paularlott/cli"
	"github.com/paularlott/cli/env"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Load .env file if it exists
	env.Load()

	// Initialize structured logging
	log.Configure("info", "console")

	rootCmd := &cli.Command{
		Name:        "rackd",
		Version:     version,
		Usage:       "Device tracking application with MCP server support",
		Description: "A Go-based device tracking application with MCP server support, web UI, and CLI",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:         "log-level",
				Usage:        "Log level (trace, debug, info, warn, error)",
				DefaultValue: "info",
				EnvVars:      []string{"RACKD_LOG_LEVEL"},
				Global:       true,
			},
			&cli.StringFlag{
				Name:         "log-format",
				Usage:        "Log format (console, json)",
				DefaultValue: "console",
				EnvVars:      []string{"RACKD_LOG_FORMAT"},
				Global:       true,
			},
		},
		PreRun: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			logLevel := cmd.GetString("log-level")
			logFormat := cmd.GetString("log-format")
			log.Configure(logLevel, logFormat)
			return ctx, nil
		},
		Commands: []*cli.Command{
			server.Command(),
			{
				Name:        "device",
				Usage:       "Device management commands",
				Description: "Manage devices in the inventory",
				Commands:    device.Commands(),
			},
			{
				Name:        "network",
				Usage:       "Network management commands",
				Description: "Manage networks in the inventory",
				Commands:    network.Commands(),
			},
			{
				Name:        "datacenter",
				Usage:       "Datacenter management commands",
				Description: "Manage datacenters in the inventory",
				Commands:    datacenter.Commands(),
			},
			{
				Name:        "discovery",
				Usage:       "Discovery commands",
				Description: "Device discovery and testing commands",
				Commands:    discovery.Commands(),
			},
		},
	}

	if err := rootCmd.Execute(context.Background()); err != nil {
		log.Error("Command execution failed", "error", err)
		os.Exit(1)
	}
}
