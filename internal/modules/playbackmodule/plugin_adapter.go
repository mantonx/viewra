package playbackmodule

import (
	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/modules/pluginmodule"
)

// PluginManagerAdapter adapts pluginmodule.ExternalPluginManager to PluginManagerInterface
type PluginManagerAdapter struct {
	extManager *pluginmodule.ExternalPluginManager
	logger     hclog.Logger
}

// NewPluginModuleAdapter creates a new plugin manager adapter
func NewPluginModuleAdapter(extManager *pluginmodule.ExternalPluginManager) PluginManagerInterface {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "plugin-adapter",
		Level: hclog.Info,
	})
	return &PluginManagerAdapter{
		extManager: extManager,
		logger:     logger,
	}
}

// GetRunningPluginInterface implements PluginManagerInterface
func (a *PluginManagerAdapter) GetRunningPluginInterface(pluginID string) (interface{}, bool) {
	if a.extManager == nil {
		return nil, false
	}
	return a.extManager.GetRunningPluginInterface(pluginID)
}

// ListPlugins implements PluginManagerInterface
func (a *PluginManagerAdapter) ListPlugins() []PluginInfo {
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
func (a *PluginManagerAdapter) GetRunningPlugins() []PluginInfo {
	a.logger.Info("PluginManagerAdapter.GetRunningPlugins called", "extManager_nil", a.extManager == nil)

	if a.extManager == nil {
		a.logger.Warn("extManager is nil in PluginManagerAdapter")
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
			Author:      "",        // Not available in pluginmodule.PluginInfo
			Status:      "running", // These are running plugins
		})
	}

	return result
}
