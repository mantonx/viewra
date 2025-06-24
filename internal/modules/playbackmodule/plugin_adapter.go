package playbackmodule

import (
	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/modules/pluginmodule"
)

// ExternalPluginManagerAdapter adapts the external plugin manager to our interface
type ExternalPluginManagerAdapter struct {
	manager *pluginmodule.ExternalPluginManager
}

// PluginModuleAdapter adapts pluginmodule.ExternalPluginManager to PluginManagerInterface
type PluginModuleAdapter struct {
	extManager *pluginmodule.ExternalPluginManager
	logger     hclog.Logger
}

// NewExternalPluginManagerAdapter creates a new adapter
func NewExternalPluginManagerAdapter(manager *pluginmodule.ExternalPluginManager) PluginManagerInterface {
	return &ExternalPluginManagerAdapter{
		manager: manager,
	}
}

// NewPluginModuleAdapter creates a new adapter
func NewPluginModuleAdapter(extManager *pluginmodule.ExternalPluginManager) PluginManagerInterface {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "plugin-adapter",
		Level: hclog.Info,
	})
	return &PluginModuleAdapter{
		extManager: extManager,
		logger:     logger,
	}
}

// GetRunningPluginInterface returns the plugin interface for a running plugin
func (a *ExternalPluginManagerAdapter) GetRunningPluginInterface(pluginID string) (interface{}, bool) {
	return a.manager.GetRunningPluginInterface(pluginID)
}

// GetRunningPluginInterface implements PluginManagerInterface
func (a *PluginModuleAdapter) GetRunningPluginInterface(pluginID string) (interface{}, bool) {
	if a.extManager == nil {
		return nil, false
	}
	return a.extManager.GetRunningPluginInterface(pluginID)
}

// ListPlugins implements PluginManagerInterface
func (a *PluginModuleAdapter) ListPlugins() []PluginInfo {
	if a.extManager == nil {
		return nil
	}

	// Convert from pluginmodule.PluginInfo to playbackmodule.PluginInfo
	plugins := a.extManager.ListPlugins()
	result := make([]PluginInfo, 0, len(plugins))

	for _, p := range plugins {
		result = append(result, PluginInfo{
			ID:          p.ID,
			Name:        p.Name,
			Version:     p.Version,
			Type:        p.Type,
			Description: p.Description,
			Author:      "", // Not available in pluginmodule.PluginInfo
			Status:      "", // Not available in pluginmodule.PluginInfo
		})
	}

	return result
}

// GetRunningPlugins implements PluginManagerInterface
func (a *PluginModuleAdapter) GetRunningPlugins() []PluginInfo {
	a.logger.Info("PluginModuleAdapter.GetRunningPlugins called", "extManager_nil", a.extManager == nil)

	if a.extManager == nil {
		a.logger.Warn("extManager is nil in PluginModuleAdapter")
		return nil
	}

	// Convert from pluginmodule.PluginInfo to playbackmodule.PluginInfo
	plugins := a.extManager.GetRunningPlugins()
	a.logger.Info("Got plugins from extManager", "count", len(plugins))
	result := make([]PluginInfo, 0, len(plugins))

	for _, p := range plugins {
		result = append(result, PluginInfo{
			ID:          p.ID,
			Name:        p.Name,
			Version:     p.Version,
			Type:        p.Type,
			Description: p.Description,
			Author:      "", // Not available in pluginmodule.PluginInfo
			Status:      "", // Not available in pluginmodule.PluginInfo
		})
	}

	return result
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
