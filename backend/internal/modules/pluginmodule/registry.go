package pluginmodule

import (
	"sync"
)

// Global core plugin registry to manage core plugin factories
// This allows core plugins to register themselves via init() functions
type PluginRegistry struct {
	factories map[string]CorePluginFactory
	mu        sync.RWMutex
}

var globalRegistry = &PluginRegistry{
	factories: make(map[string]CorePluginFactory),
}

// RegisterCorePluginFactory registers a core plugin factory globally
// This should be called from plugin packages' init() functions
func RegisterCorePluginFactory(name string, factory CorePluginFactory) {
	globalRegistry.RegisterFactory(name, factory)
}

// RegisterFactory registers a core plugin factory
func (r *PluginRegistry) RegisterFactory(name string, factory CorePluginFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[name] = factory
}

// GetFactory returns a core plugin factory by name
func (r *PluginRegistry) GetFactory(name string) (CorePluginFactory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	factory, exists := r.factories[name]
	return factory, exists
}

// GetCorePluginFactories returns all registered core plugin factories
func (r *PluginRegistry) GetCorePluginFactories() map[string]CorePluginFactory {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]CorePluginFactory)
	for name, factory := range r.factories {
		result[name] = factory
	}
	return result
}

// ListFactoryNames returns all registered factory names
func (r *PluginRegistry) ListFactoryNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	return names
}

// GetGlobalRegistry returns the global plugin registry
func GetGlobalRegistry() *PluginRegistry {
	return globalRegistry
}
