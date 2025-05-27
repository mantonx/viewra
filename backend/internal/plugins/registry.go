package plugins

import (
	"fmt"
	"sync"
)

// PluginFactory is a function that creates a new plugin instance
type PluginFactory func() Plugin

// PluginRegistry manages plugin factories for self-registration
type PluginRegistry struct {
	mu        sync.RWMutex
	factories map[string]PluginFactory
}

// Global plugin registry
var globalRegistry = &PluginRegistry{
	factories: make(map[string]PluginFactory),
}

// RegisterPlugin allows plugins to register themselves
func RegisterPlugin(pluginID string, factory PluginFactory) {
	globalRegistry.Register(pluginID, factory)
}

// Register adds a plugin factory to the registry
func (r *PluginRegistry) Register(pluginID string, factory PluginFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.factories[pluginID] = factory
}

// CreatePlugin creates a plugin instance using the registered factory
func (r *PluginRegistry) CreatePlugin(pluginID string) (Plugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	factory, exists := r.factories[pluginID]
	if !exists {
		return nil, fmt.Errorf("no factory registered for plugin: %s", pluginID)
	}
	
	return factory(), nil
}

// IsRegistered checks if a plugin is registered
func (r *PluginRegistry) IsRegistered(pluginID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	_, exists := r.factories[pluginID]
	return exists
}

// ListRegistered returns all registered plugin IDs
func (r *PluginRegistry) ListRegistered() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	plugins := make([]string, 0, len(r.factories))
	for pluginID := range r.factories {
		plugins = append(plugins, pluginID)
	}
	return plugins
}

// GetGlobalRegistry returns the global plugin registry
func GetGlobalRegistry() *PluginRegistry {
	return globalRegistry
} 