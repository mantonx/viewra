package playbackmodule

import (
	"github.com/mantonx/viewra/internal/modules/pluginmodule"
)

// ExternalPluginManagerAdapter adapts the external plugin manager to our interface
type ExternalPluginManagerAdapter struct {
	manager *pluginmodule.ExternalPluginManager
}

// PluginModuleAdapter adapts the PluginModule to our interface
type PluginModuleAdapter struct {
	pluginModule *pluginmodule.PluginModule
}

// NewExternalPluginManagerAdapter creates a new adapter
func NewExternalPluginManagerAdapter(manager *pluginmodule.ExternalPluginManager) PluginManagerInterface {
	return &ExternalPluginManagerAdapter{
		manager: manager,
	}
}

// NewPluginModuleAdapter creates a new adapter for PluginModule
func NewPluginModuleAdapter(pluginModule *pluginmodule.PluginModule) PluginManagerInterface {
	return &PluginModuleAdapter{
		pluginModule: pluginModule,
	}
}

// GetRunningPluginInterface returns the plugin interface for a running plugin
func (a *ExternalPluginManagerAdapter) GetRunningPluginInterface(pluginID string) (interface{}, bool) {
	return a.manager.GetRunningPluginInterface(pluginID)
}

// GetRunningPluginInterface returns the plugin interface for a running plugin
func (a *PluginModuleAdapter) GetRunningPluginInterface(pluginID string) (interface{}, bool) {
	if a.pluginModule == nil {
		return nil, false
	}

	// Get the external plugin manager from the PluginModule
	externalManager := a.pluginModule.GetExternalManager()
	if externalManager == nil {
		return nil, false
	}

	return externalManager.GetRunningPluginInterface(pluginID)
}

// ListPlugins returns all plugins
func (a *ExternalPluginManagerAdapter) ListPlugins() []PluginInfo {
	plugins := a.manager.ListPlugins()
	result := make([]PluginInfo, len(plugins))

	for i, plugin := range plugins {
		result[i] = PluginInfo{
			ID:          plugin.ID,
			Name:        plugin.Name,
			Version:     plugin.Version,
			Type:        plugin.Type,
			Description: plugin.Description,
			Author:      "",        // Not available in pluginmodule.PluginInfo
			Status:      "unknown", // Not available in pluginmodule.PluginInfo
		}
	}

	return result
}

// ListPlugins returns all plugins
func (a *PluginModuleAdapter) ListPlugins() []PluginInfo {
	if a.pluginModule == nil {
		return []PluginInfo{}
	}

	// Get the external plugin manager from the PluginModule
	externalManager := a.pluginModule.GetExternalManager()
	if externalManager == nil {
		return []PluginInfo{}
	}

	plugins := externalManager.ListPlugins()
	result := make([]PluginInfo, len(plugins))

	for i, plugin := range plugins {
		result[i] = PluginInfo{
			ID:          plugin.ID,
			Name:        plugin.Name,
			Version:     plugin.Version,
			Type:        plugin.Type,
			Description: plugin.Description,
			Author:      "",        // Not available in pluginmodule.PluginInfo
			Status:      "unknown", // Not available in pluginmodule.PluginInfo
		}
	}

	return result
}

// GetRunningPlugins returns all running plugins
func (a *ExternalPluginManagerAdapter) GetRunningPlugins() []PluginInfo {
	plugins := a.manager.GetRunningPlugins()
	result := make([]PluginInfo, len(plugins))

	for i, plugin := range plugins {
		result[i] = PluginInfo{
			ID:          plugin.ID,
			Name:        plugin.Name,
			Version:     plugin.Version,
			Type:        plugin.Type,
			Description: plugin.Description,
			Author:      "",        // Not available in pluginmodule.PluginInfo
			Status:      "running", // These are running plugins
		}
	}

	return result
}

// GetRunningPlugins returns all running plugins
func (a *PluginModuleAdapter) GetRunningPlugins() []PluginInfo {
	if a.pluginModule == nil {
		return []PluginInfo{}
	}

	// Get the external plugin manager from the PluginModule
	externalManager := a.pluginModule.GetExternalManager()
	if externalManager == nil {
		return []PluginInfo{}
	}

	plugins := externalManager.GetRunningPlugins()
	result := make([]PluginInfo, len(plugins))

	for i, plugin := range plugins {
		result[i] = PluginInfo{
			ID:          plugin.ID,
			Name:        plugin.Name,
			Version:     plugin.Version,
			Type:        plugin.Type,
			Description: plugin.Description,
			Author:      "",        // Not available in pluginmodule.PluginInfo
			Status:      "running", // These are running plugins
		}
	}

	return result
}
