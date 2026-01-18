package premium

import (
	"fmt"
	"plugin"

	"github.com/martinsuchenak/rackd/internal/registry"
)

// Loader handles loading premium features from plugins or built-in sources
type Loader struct {
	registry *registry.Registry
}

// NewLoader creates a new premium feature loader
func NewLoader() *Loader {
	return &Loader{
		registry: registry.GetRegistry(),
	}
}

// RegisterPremiumFeaturesFunc is the function signature expected from plugins
// Plugins must export a function with this signature named "RegisterPremiumFeatures"
type RegisterPremiumFeaturesFunc func(reg *registry.Registry) error

// LoadFromPlugin loads premium features from a Go plugin file
// The plugin must export a function named "RegisterPremiumFeatures" with signature:
// func RegisterPremiumFeatures(reg *registry.Registry) error
func (l *Loader) LoadFromPlugin(pluginPath string) error {
	// Open the plugin
	plug, err := plugin.Open(pluginPath)
	if err != nil {
		return fmt.Errorf("failed to open plugin %s: %w", pluginPath, err)
	}

	// Look up the RegisterPremiumFeatures symbol
	symbol, err := plug.Lookup("RegisterPremiumFeatures")
	if err != nil {
		return fmt.Errorf("plugin %s does not export RegisterPremiumFeatures function: %w", pluginPath, err)
	}

	// Type check the symbol to ensure it's a function with the correct signature
	registerFunc, ok := symbol.(RegisterPremiumFeaturesFunc)
	if !ok {
		return fmt.Errorf("plugin %s RegisterPremiumFeatures has invalid signature, expected func(reg *registry.Registry) error", pluginPath)
	}

	// Call the registration function
	if err := registerFunc(l.registry); err != nil {
		return fmt.Errorf("failed to register premium features from plugin %s: %w", pluginPath, err)
	}

	return nil
}

// LoadBuiltIn loads premium features from a monolithic build
// The registerFunc parameter should be a function that registers built-in premium features
func (l *Loader) LoadBuiltIn(registerFunc RegisterPremiumFeaturesFunc) error {
	if registerFunc == nil {
		return fmt.Errorf("register function cannot be nil")
	}

	if err := registerFunc(l.registry); err != nil {
		return fmt.Errorf("failed to register built-in premium features: %w", err)
	}

	return nil
}
