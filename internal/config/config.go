package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Config holds the application configuration
type Config struct {
	DataDir      string
	ListenAddr   string
	MCPAuthToken string
	APIAuthToken string
	ConfigFile   string // Path to .env file (if loaded)
}

// Load loads configuration with the following priority (highest to lowest):
// 1. Command-line parameters (passed as opts)
// 2. .env file (if exists)
// 3. Environment variables
// 4. Default values
//
// If opts is provided, it overrides all other sources.
// Otherwise, .env file overrides environment variables.
func Load(opts *Config) *Config {
	cfg := &Config{
		DataDir:      "./data",
		ListenAddr:   ":8080",
		MCPAuthToken: "",
		APIAuthToken: "",
	}

	// First, try to load from .env file
	envFile := ".env"
	if _, err := os.Stat(envFile); err == nil {
		if err := loadFromEnvFile(cfg, envFile); err != nil {
			log := &logger{}
			log.Printf("Warning: Failed to load .env file: %v", err)
		} else {
			cfg.ConfigFile = envFile
		}
	}

	// Then load environment variables (only if not already set by .env)
	cfg.DataDir = coalesce(cfg.DataDir, os.Getenv("RACKD_DATA_DIR"), "./data")
	cfg.ListenAddr = coalesce(cfg.ListenAddr, os.Getenv("RACKD_LISTEN_ADDR"), ":8080")
	cfg.MCPAuthToken = coalesce(cfg.MCPAuthToken, os.Getenv("RACKD_BEARER_TOKEN"), "")
	cfg.APIAuthToken = coalesce(cfg.APIAuthToken, os.Getenv("RACKD_API_TOKEN"), "")

	// Finally, apply CLI opts if provided (highest priority)
	if opts != nil {
		if opts.DataDir != "" {
			cfg.DataDir = opts.DataDir
		}
		if opts.ListenAddr != "" {
			cfg.ListenAddr = opts.ListenAddr
		}
		if opts.MCPAuthToken != "" {
			cfg.MCPAuthToken = opts.MCPAuthToken
		}
		if opts.APIAuthToken != "" {
			cfg.APIAuthToken = opts.APIAuthToken
		}
	}

	return cfg
}

// loadFromEnvFile loads configuration from a .env file
func loadFromEnvFile(cfg *Config, filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE or KEY="VALUE"
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), "\"")

		// Map .env keys to config fields
		switch key {
		case "RACKD_DATA_DIR":
			cfg.DataDir = value
		case "RACKD_LISTEN_ADDR":
			cfg.ListenAddr = value
		case "RACKD_BEARER_TOKEN":
			cfg.MCPAuthToken = value
		case "RACKD_API_TOKEN":
			cfg.APIAuthToken = value
		}
	}

	return scanner.Err()
}

// IsMCPEnabled checks if MCP authentication is configured
func (c *Config) IsMCPEnabled() bool {
	return c.MCPAuthToken != ""
}

// IsAPIAuthEnabled checks if API authentication is configured
func (c *Config) IsAPIAuthEnabled() bool {
	return c.APIAuthToken != ""
}

// String returns a string representation of the config source
func (c *Config) String() string {
	if c.ConfigFile != "" {
		return fmt.Sprintf(".env file (%s)", c.ConfigFile)
	}
	return "environment variables"
}

// coalesce returns the first non-empty string value
func coalesce(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// Simple logger type to avoid import cycle
type logger struct{}

func (l *logger) Printf(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

// Legacy Load function for backward compatibility (ENV vars only)
func LoadFromEnv() *Config {
	return Load(nil)
}
