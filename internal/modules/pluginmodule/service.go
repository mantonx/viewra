package pluginmodule

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/services"
	"github.com/mantonx/viewra/internal/types"
	plugins "github.com/mantonx/viewra/sdk"
)

// Ensure PluginModule implements PluginService
var _ services.PluginService = (*ServiceAdapter)(nil)

// ServiceAdapter adapts the PluginModule to implement the PluginService interface
type ServiceAdapter struct {
	module *PluginModule
}

// NewServiceAdapter creates a new service adapter for the plugin module
func NewServiceAdapter(module *PluginModule) services.PluginService {
	return &ServiceAdapter{module: module}
}

// ListPlugins returns all available plugins
func (s *ServiceAdapter) ListPlugins(ctx context.Context) ([]plugins.PluginInfo, error) {
	if s.module.externalManager == nil {
		return nil, fmt.Errorf("external plugin manager not initialized")
	}

	// Get external plugins
	externalPlugins := s.module.externalManager.ListPlugins()

	// Convert to SDK PluginInfo
	infos := make([]plugins.PluginInfo, 0, len(externalPlugins))
	for _, p := range externalPlugins {
		infos = append(infos, plugins.PluginInfo{
			ID:      p.ID,
			Name:    p.Name,
			Version: p.Version,
			Type:    p.Type,
		})
	}

	return infos, nil
}

// GetPlugin retrieves a specific plugin by ID
func (s *ServiceAdapter) GetPlugin(ctx context.Context, pluginID string) (plugins.Plugin, error) {
	if s.module.externalManager == nil {
		return nil, fmt.Errorf("external plugin manager not initialized")
	}

	// Get the plugin interface
	pluginInterface, found := s.module.externalManager.GetRunningPluginInterface(pluginID)
	if !found {
		return nil, fmt.Errorf("plugin %s not found", pluginID)
	}

	// Try to cast to plugins.Plugin
	if plugin, ok := pluginInterface.(plugins.Plugin); ok {
		return plugin, nil
	}

	return nil, fmt.Errorf("plugin %s does not implement Plugin interface", pluginID)
}

// EnablePlugin enables a plugin
func (s *ServiceAdapter) EnablePlugin(ctx context.Context, pluginID string) error {
	// This would need to be implemented based on plugin configuration
	return fmt.Errorf("not implemented")
}

// DisablePlugin disables a plugin
func (s *ServiceAdapter) DisablePlugin(ctx context.Context, pluginID string) error {
	// This would need to be implemented based on plugin configuration
	return fmt.Errorf("not implemented")
}

// GetPluginStatus returns the status of a plugin
func (s *ServiceAdapter) GetPluginStatus(ctx context.Context, pluginID string) (*types.PluginStatus, error) {
	// Get plugin info from external manager
	externalPlugin, exists := s.module.externalManager.GetPlugin(pluginID)
	if !exists {
		return nil, fmt.Errorf("plugin %s not found", pluginID)
	}

	// Get plugin info from ListPlugins to check if enabled
	allPlugins := s.module.externalManager.ListPlugins()
	var enabled bool
	for _, p := range allPlugins {
		if p.ID == pluginID {
			enabled = p.Enabled
			break
		}
	}

	return &types.PluginStatus{
		ID:      pluginID,
		Name:    externalPlugin.Name,
		Version: externalPlugin.Version,
		Type:    externalPlugin.Type,
		Enabled: enabled,
		Running: externalPlugin.Running,
		Healthy: true, // Would need health check implementation
	}, nil
}

// GetMetadataScrapers returns all metadata scraper plugins
func (s *ServiceAdapter) GetMetadataScrapers() []plugins.MetadataScraperService {
	if s.module.externalManager == nil {
		return nil
	}

	scrapers := []plugins.MetadataScraperService{}

	// Get all plugins and check which ones are metadata scrapers
	allPlugins := s.module.externalManager.ListPlugins()
	for _, p := range allPlugins {
		if p.Type == "metadata_scraper" && p.Enabled {
			// Check if it's actually running
			if extPlugin, exists := s.module.externalManager.GetPlugin(p.ID); exists && extPlugin.Running {
				// Get the actual plugin interface
				if pluginInterface, found := s.module.externalManager.GetRunningPluginInterface(p.ID); found {
					if scraper, ok := pluginInterface.(plugins.MetadataScraperService); ok {
						scrapers = append(scrapers, scraper)
					}
				}
			}
		}
	}

	return scrapers
}

// GetEnrichmentServices returns all enrichment service plugins
func (s *ServiceAdapter) GetEnrichmentServices() []plugins.EnrichmentService {
	if s.module.externalManager == nil {
		return nil
	}

	enrichers := []plugins.EnrichmentService{}

	// Get all plugins and check which ones are enrichment services
	allPlugins := s.module.externalManager.ListPlugins()
	for _, p := range allPlugins {
		if (p.Type == "enrichment" || p.Type == "enricher") && p.Enabled {
			// Check if it's actually running
			if extPlugin, exists := s.module.externalManager.GetPlugin(p.ID); exists && extPlugin.Running {
				// Get the actual plugin interface
				if pluginInterface, found := s.module.externalManager.GetRunningPluginInterface(p.ID); found {
					if enricher, ok := pluginInterface.(plugins.EnrichmentService); ok {
						enrichers = append(enrichers, enricher)
					}
				}
			}
		}
	}

	return enrichers
}

// GetTranscodingProviders returns all transcoding provider plugins
func (s *ServiceAdapter) GetTranscodingProviders() []plugins.TranscodingProvider {
	if s.module.externalManager == nil {
		return nil
	}

	providers := []plugins.TranscodingProvider{}

	// Get all plugins and check which ones are transcoding providers
	allPlugins := s.module.externalManager.ListPlugins()
	for _, p := range allPlugins {
		if (p.Type == "transcoding" || p.Type == "transcoder") && p.Enabled {
			// Check if it's actually running
			if extPlugin, exists := s.module.externalManager.GetPlugin(p.ID); exists && extPlugin.Running {
				// Get the actual plugin interface
				if pluginInterface, found := s.module.externalManager.GetRunningPluginInterface(p.ID); found {
					if provider, ok := pluginInterface.(plugins.TranscodingProvider); ok {
						providers = append(providers, provider)
					}
				}
			}
		}
	}

	return providers
}

// UpdatePluginConfig updates a plugin's configuration
func (s *ServiceAdapter) UpdatePluginConfig(ctx context.Context, pluginID string, config map[string]interface{}) error {
	// This would need to be implemented based on plugin configuration
	return fmt.Errorf("not implemented")
}

// GetPluginConfig returns a plugin's configuration
func (s *ServiceAdapter) GetPluginConfig(ctx context.Context, pluginID string) (map[string]interface{}, error) {
	// This would need to be implemented based on plugin configuration
	return nil, fmt.Errorf("not implemented")
}
