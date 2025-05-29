package plugins

import (
	"fmt"
)

// CorePluginFactory defines a function that creates core plugins
type CorePluginFactory func() CoreMediaPlugin

// CorePluginsManager manages all core media plugins
type CorePluginsManager struct {
	registry  *MediaPluginRegistry
	plugins   []CoreMediaPlugin
	factories map[string]CorePluginFactory
}

// NewCorePluginsManager creates a new core plugins manager
func NewCorePluginsManager() *CorePluginsManager {
	return &CorePluginsManager{
		registry:  NewMediaPluginRegistry(),
		plugins:   make([]CoreMediaPlugin, 0),
		factories: make(map[string]CorePluginFactory),
	}
}

// RegisterPluginFactory registers a factory function for creating core plugins
func (cpm *CorePluginsManager) RegisterPluginFactory(name string, factory CorePluginFactory) {
	cpm.factories[name] = factory
	fmt.Printf("DEBUG: Registered core plugin factory: %s\n", name)
}

// InitializeCorePlugins creates and initializes all registered core media plugins
func (cpm *CorePluginsManager) InitializeCorePlugins() error {
	fmt.Printf("DEBUG: Initializing %d core media plugins...\n", len(cpm.factories))
	
	for name, factory := range cpm.factories {
		plugin := factory()
		if err := cpm.registry.RegisterCorePlugin(plugin); err != nil {
			return fmt.Errorf("failed to register core plugin %s: %w", name, err)
		}
		cpm.plugins = append(cpm.plugins, plugin)
		fmt.Printf("DEBUG: Initialized core plugin: %s\n", name)
	}
	
	fmt.Printf("DEBUG: Successfully initialized %d core media plugins\n", len(cpm.plugins))
	return nil
}

// GetRegistry returns the media plugin registry
func (cpm *CorePluginsManager) GetRegistry() *MediaPluginRegistry {
	return cpm.registry
}

// GetPlugins returns all registered core plugins
func (cpm *CorePluginsManager) GetPlugins() []CoreMediaPlugin {
	return cpm.plugins
}

// Shutdown gracefully shuts down all core plugins
func (cpm *CorePluginsManager) Shutdown() error {
	fmt.Printf("DEBUG: Shutting down core media plugins...\n")
	
	var lastErr error
	for _, plugin := range cpm.plugins {
		if err := plugin.Shutdown(); err != nil {
			fmt.Printf("WARNING: Error shutting down plugin %s: %v\n", plugin.GetName(), err)
			lastErr = err
		}
	}
	
	// Shutdown the registry
	if err := cpm.registry.Shutdown(); err != nil {
		fmt.Printf("WARNING: Error shutting down plugin registry: %v\n", err)
		lastErr = err
	}
	
	fmt.Printf("DEBUG: Core media plugins shutdown complete\n")
	return lastErr
}

// EnablePlugin enables a specific core plugin
func (cpm *CorePluginsManager) EnablePlugin(pluginName string) error {
	for _, plugin := range cpm.plugins {
		if plugin.GetName() == pluginName {
			return plugin.Enable()
		}
	}
	return fmt.Errorf("core plugin %s not found", pluginName)
}

// DisablePlugin disables a specific core plugin  
func (cpm *CorePluginsManager) DisablePlugin(pluginName string) error {
	for _, plugin := range cpm.plugins {
		if plugin.GetName() == pluginName {
			return plugin.Disable()
		}
	}
	return fmt.Errorf("core plugin %s not found", pluginName)
}

// GetPluginInfo returns information about all core plugins
func (cpm *CorePluginsManager) GetPluginInfo() []MediaPluginInfo {
	return cpm.registry.GetPluginInfo()
}

// GetAvailableFactories returns the names of all registered plugin factories
func (cpm *CorePluginsManager) GetAvailableFactories() []string {
	names := make([]string, 0, len(cpm.factories))
	for name := range cpm.factories {
		names = append(names, name)
	}
	return names
} 