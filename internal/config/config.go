package config

import (
	"path/filepath"
	"github.com/paularlott/cli"
)

type Config struct {
	DataDir      string
	ListenAddr   string
	MCPAuthToken string
	APIAuthToken string
}

var (
	dataDir      string
	listenAddr   string
	mcpAuthToken string
	apiAuthToken string
)

func GetFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:         "data-dir",
			Usage:        "Data directory path",
			EnvVars:      []string{"RACKD_DATA_DIR"},
			DefaultValue: filepath.Join(".", "data"),
			AssignTo:     &dataDir,
		},
		&cli.StringFlag{
			Name:         "addr",
			Usage:        "Server listen address",
			EnvVars:      []string{"RACKD_LISTEN_ADDR"},
			DefaultValue: ":8080",
			AssignTo:     &listenAddr,
		},
		&cli.StringFlag{
			Name:     "mcp-token",
			Usage:    "MCP bearer token",
			EnvVars:  []string{"RACKD_BEARER_TOKEN"},
			AssignTo: &mcpAuthToken,
		},
		&cli.StringFlag{
			Name:     "api-token",
			Usage:    "API bearer token",
			EnvVars:  []string{"RACKD_API_TOKEN"},
			AssignTo: &apiAuthToken,
		},
	}
}

func Load() *Config {
	return &Config{
		DataDir:      dataDir,
		ListenAddr:   listenAddr,
		MCPAuthToken: mcpAuthToken,
		APIAuthToken: apiAuthToken,
	}
}



// IsMCPEnabled checks if MCP authentication is configured
func (c *Config) IsMCPEnabled() bool {
	return c.MCPAuthToken != ""
}

// IsAPIAuthEnabled checks if API authentication is configured
func (c *Config) IsAPIAuthEnabled() bool {
	return c.APIAuthToken != ""
}


