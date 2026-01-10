package config

import (
	"path/filepath"
	"strconv"
	"time"

	"github.com/paularlott/cli"
)

type Config struct {
	DataDir      string
	ListenAddr   string
	MCPAuthToken string
	APIAuthToken string

	// Discovery settings
	DiscoveryEnabled          bool
	DiscoveryInterval         time.Duration
	DiscoveryMaxConcurrent    int
	DiscoveryTimeout          time.Duration
	DiscoveryDefaultScanType  string
	DiscoveryCleanupDays      int
}

var (
	dataDir      string
	listenAddr   string
	mcpAuthToken string
	apiAuthToken string

	// Discovery flag variables
	discoveryEnabled          bool
	discoveryInterval         string
	discoveryMaxConcurrent    string
	discoveryTimeout          string
	discoveryDefaultScanType  string
	discoveryCleanupDays      string
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
		// Discovery flags
		&cli.BoolFlag{
			Name:         "discovery-enabled",
			Usage:        "Enable automatic device discovery",
			EnvVars:      []string{"RACKD_DISCOVERY_ENABLED"},
			DefaultValue: false,
			AssignTo:     &discoveryEnabled,
		},
		&cli.StringFlag{
			Name:         "discovery-interval",
			Usage:        "Discovery scan interval (e.g., 24h, 12h)",
			EnvVars:      []string{"RACKD_DISCOVERY_INTERVAL"},
			DefaultValue: "24h",
			AssignTo:     &discoveryInterval,
		},
		&cli.StringFlag{
			Name:         "discovery-max-concurrent",
			Usage:        "Maximum concurrent discovery scans",
			EnvVars:      []string{"RACKD_DISCOVERY_MAX_CONCURRENT"},
			DefaultValue: "10",
			AssignTo:     &discoveryMaxConcurrent,
		},
		&cli.StringFlag{
			Name:         "discovery-timeout",
			Usage:        "Per-host discovery timeout (e.g., 5s, 10s)",
			EnvVars:      []string{"RACKD_DISCOVERY_TIMEOUT"},
			DefaultValue: "5s",
			AssignTo:     &discoveryTimeout,
		},
		&cli.StringFlag{
			Name:         "discovery-default-scan-type",
			Usage:        "Default scan type: quick, full, or deep",
			EnvVars:      []string{"RACKD_DISCOVERY_DEFAULT_SCAN_TYPE"},
			DefaultValue: "full",
			AssignTo:     &discoveryDefaultScanType,
		},
		&cli.StringFlag{
			Name:         "discovery-cleanup-days",
			Usage:        "Days before cleanup of old discovered devices",
			EnvVars:      []string{"RACKD_DISCOVERY_CLEANUP_DAYS"},
			DefaultValue: "30",
			AssignTo:     &discoveryCleanupDays,
		},
	}
}

func Load() *Config {
	// Parse discovery interval
	discoveryIntervalDur, _ := time.ParseDuration(discoveryInterval)
	if discoveryIntervalDur == 0 {
		discoveryIntervalDur = 24 * time.Hour
	}

	// Parse discovery max concurrent
	discoveryMaxConcurrentInt, _ := strconv.Atoi(discoveryMaxConcurrent)
	if discoveryMaxConcurrentInt == 0 {
		discoveryMaxConcurrentInt = 10
	}

	// Parse discovery timeout
	discoveryTimeoutDur, _ := time.ParseDuration(discoveryTimeout)
	if discoveryTimeoutDur == 0 {
		discoveryTimeoutDur = 5 * time.Second
	}

	// Parse discovery cleanup days
	discoveryCleanupDaysInt, _ := strconv.Atoi(discoveryCleanupDays)
	if discoveryCleanupDaysInt == 0 {
		discoveryCleanupDaysInt = 30
	}

	// Validate default scan type
	scanType := discoveryDefaultScanType
	if scanType == "" {
		scanType = "full"
	}
	if scanType != "quick" && scanType != "full" && scanType != "deep" {
		scanType = "full"
	}

	return &Config{
		DataDir:      dataDir,
		ListenAddr:   listenAddr,
		MCPAuthToken: mcpAuthToken,
		APIAuthToken: apiAuthToken,

		// Discovery settings
		DiscoveryEnabled:          discoveryEnabled,
		DiscoveryInterval:         discoveryIntervalDur,
		DiscoveryMaxConcurrent:    discoveryMaxConcurrentInt,
		DiscoveryTimeout:          discoveryTimeoutDur,
		DiscoveryDefaultScanType:  scanType,
		DiscoveryCleanupDays:      discoveryCleanupDaysInt,
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


