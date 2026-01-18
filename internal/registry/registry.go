package registry

import (
	"sync"
)

// Registry manages premium extensions and features
type Registry struct {
	mu sync.RWMutex

	// Provider factories
	storageProviders map[string]StorageProviderFactory
	scannerProviders map[string]ScannerProviderFactory
	workerProviders  map[string]WorkerProviderFactory

	// Feature flags
	features map[string]bool
}

// StorageProviderFactory creates storage provider instances
type StorageProviderFactory func(config map[string]interface{}) (interface{}, error)

// ScannerProviderFactory creates scanner provider instances
type ScannerProviderFactory func(config map[string]interface{}) (interface{}, error)

// WorkerProviderFactory creates worker provider instances
type WorkerProviderFactory func(config map[string]interface{}) (interface{}, error)

var (
	registryInstance *Registry
	registryOnce     sync.Once
)

// GetRegistry returns the singleton registry instance
func GetRegistry() *Registry {
	registryOnce.Do(func() {
		registryInstance = &Registry{
			storageProviders: make(map[string]StorageProviderFactory),
			scannerProviders: make(map[string]ScannerProviderFactory),
			workerProviders:  make(map[string]WorkerProviderFactory),
			features:         make(map[string]bool),
		}
	})
	return registryInstance
}

// RegisterStorageProvider registers a storage provider factory
func (r *Registry) RegisterStorageProvider(name string, factory StorageProviderFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.storageProviders[name] = factory
}

// RegisterScannerProvider registers a scanner provider factory
func (r *Registry) RegisterScannerProvider(name string, factory ScannerProviderFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scannerProviders[name] = factory
}

// RegisterWorkerProvider registers a worker provider factory
func (r *Registry) RegisterWorkerProvider(name string, factory WorkerProviderFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workerProviders[name] = factory
}

// EnableFeature enables a feature flag
func (r *Registry) EnableFeature(feature string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.features[feature] = true
}

// IsFeatureEnabled checks if a feature is enabled
func (r *Registry) IsFeatureEnabled(feature string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.features[feature]
}

// GetStorageProvider returns a storage provider factory by name
func (r *Registry) GetStorageProvider(name string) (StorageProviderFactory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	factory, exists := r.storageProviders[name]
	return factory, exists
}

// GetScannerProvider returns a scanner provider factory by name
func (r *Registry) GetScannerProvider(name string) (ScannerProviderFactory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	factory, exists := r.scannerProviders[name]
	return factory, exists
}

// GetWorkerProvider returns a worker provider factory by name
func (r *Registry) GetWorkerProvider(name string) (WorkerProviderFactory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	factory, exists := r.workerProviders[name]
	return factory, exists
}

// RegisterFeature registers a feature implementation
func (r *Registry) RegisterFeature(name string, feature interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.features[name] = true
	return nil
}

// GetFeature returns a registered feature
func (r *Registry) GetFeature(name string) (interface{}, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	feature, exists := r.features[name]
	return feature, exists
}
