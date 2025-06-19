package pluginmodule

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/pkg/plugins"
)

// DashboardManager orchestrates plugin-defined dashboard sections
type DashboardManager struct {
	mu               sync.RWMutex
	sections         map[string]*plugins.DashboardSection
	dataProviders    map[string]plugins.DashboardDataProvider
	sectionProviders map[string]plugins.DashboardSectionProvider
	logger           hclog.Logger
}

// NewDashboardManager creates a new dashboard manager
func NewDashboardManager(logger hclog.Logger) *DashboardManager {
	return &DashboardManager{
		sections:         make(map[string]*plugins.DashboardSection),
		dataProviders:    make(map[string]plugins.DashboardDataProvider),
		sectionProviders: make(map[string]plugins.DashboardSectionProvider),
		logger:           logger.Named("dashboard-manager"),
	}
}

// RegisterPlugin registers a plugin that can provide dashboard sections
func (dm *DashboardManager) RegisterPlugin(pluginID string, plugin interface{}) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// Check if plugin provides dashboard sections
	if sectionProvider, ok := plugin.(plugins.DashboardSectionProvider); ok {
		dm.sectionProviders[pluginID] = sectionProvider
		dm.logger.Info("Registered dashboard section provider", "pluginID", pluginID)

		// Immediately discover sections from this plugin
		go dm.discoverSectionsFromPlugin(pluginID, sectionProvider)
	}

	// Check if plugin provides dashboard data
	if dataProvider, ok := plugin.(plugins.DashboardDataProvider); ok {
		dm.dataProviders[pluginID] = dataProvider
		dm.logger.Info("Registered dashboard data provider", "pluginID", pluginID)
	}

	return nil
}

// UnregisterPlugin removes a plugin's dashboard sections
func (dm *DashboardManager) UnregisterPlugin(pluginID string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// Remove all sections for this plugin
	for sectionID := range dm.sections {
		if dm.sections[sectionID].PluginID == pluginID {
			delete(dm.sections, sectionID)
		}
	}

	delete(dm.sectionProviders, pluginID)
	delete(dm.dataProviders, pluginID)

	dm.logger.Info("Unregistered plugin from dashboard", "pluginID", pluginID)
	return nil
}

// discoverSectionsFromPlugin discovers dashboard sections from a plugin
func (dm *DashboardManager) discoverSectionsFromPlugin(pluginID string, provider plugins.DashboardSectionProvider) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sections, err := provider.GetDashboardSections(ctx)
	if err != nil {
		dm.logger.Error("Failed to discover sections from plugin", "pluginID", pluginID, "error", err)
		return
	}

	dm.mu.Lock()
	defer dm.mu.Unlock()

	for _, section := range sections {
		section.PluginID = pluginID // Ensure plugin ID is set
		dm.sections[section.ID] = &section
		dm.logger.Info("Discovered dashboard section", "sectionID", section.ID, "pluginID", pluginID)
	}
}

// GetAllSections returns all registered dashboard sections
func (dm *DashboardManager) GetAllSections() []plugins.DashboardSection {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	sections := make([]plugins.DashboardSection, 0, len(dm.sections))
	for _, section := range dm.sections {
		sections = append(sections, *section)
	}

	// Sort by priority (higher priority first)
	sort.Slice(sections, func(i, j int) bool {
		return sections[i].Priority > sections[j].Priority
	})

	return sections
}

// GetSectionsByType returns dashboard sections filtered by type
func (dm *DashboardManager) GetSectionsByType(sectionType string) []plugins.DashboardSection {
	sections := dm.GetAllSections()
	filtered := make([]plugins.DashboardSection, 0)

	for _, section := range sections {
		if section.Type == sectionType {
			filtered = append(filtered, section)
		}
	}

	return filtered
}

// GetMainData fetches main data for a dashboard section
func (dm *DashboardManager) GetMainData(ctx context.Context, sectionID string) (interface{}, error) {
	dm.logger.Info("GetMainData called in dashboard manager", "sectionID", sectionID)

	dm.mu.RLock()
	section, exists := dm.sections[sectionID]
	if !exists {
		dm.mu.RUnlock()
		dm.logger.Error("Section not found", "sectionID", sectionID)
		return nil, fmt.Errorf("section not found: %s", sectionID)
	}

	provider, exists := dm.dataProviders[section.PluginID]
	dm.mu.RUnlock()

	if !exists {
		dm.logger.Error("Data provider not found for plugin", "pluginID", section.PluginID, "sectionID", sectionID)
		return nil, fmt.Errorf("data provider not found for plugin: %s", section.PluginID)
	}

	dm.logger.Info("Calling plugin GetMainData", "pluginID", section.PluginID, "sectionID", sectionID)
	return provider.GetMainData(ctx, sectionID)
}

// GetNerdData fetches advanced/detailed data for a dashboard section
func (dm *DashboardManager) GetNerdData(ctx context.Context, sectionID string) (interface{}, error) {
	dm.mu.RLock()
	section, exists := dm.sections[sectionID]
	if !exists {
		dm.mu.RUnlock()
		return nil, fmt.Errorf("section not found: %s", sectionID)
	}

	provider, exists := dm.dataProviders[section.PluginID]
	dm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("data provider not found for plugin: %s", section.PluginID)
	}

	return provider.GetNerdData(ctx, sectionID)
}

// GetMetrics fetches time-series metrics for a dashboard section
func (dm *DashboardManager) GetMetrics(ctx context.Context, sectionID string, timeRange plugins.TimeRange) ([]plugins.MetricPoint, error) {
	dm.mu.RLock()
	section, exists := dm.sections[sectionID]
	if !exists {
		dm.mu.RUnlock()
		return nil, fmt.Errorf("section not found: %s", sectionID)
	}

	provider, exists := dm.dataProviders[section.PluginID]
	dm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("data provider not found for plugin: %s", section.PluginID)
	}

	return provider.GetMetrics(ctx, sectionID, timeRange)
}

// ExecuteAction executes a dashboard action
func (dm *DashboardManager) ExecuteAction(ctx context.Context, sectionID, actionID string, payload map[string]interface{}) (interface{}, error) {
	dm.mu.RLock()
	section, exists := dm.sections[sectionID]
	dm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("section not found: %s", sectionID)
	}

	// Find the action in the section manifest
	var action *plugins.DashboardAction
	for _, a := range section.Manifest.Actions {
		if a.ID == actionID {
			action = &a
			break
		}
	}

	if action == nil {
		return nil, fmt.Errorf("action not found: %s", actionID)
	}

	// Execute action through plugin (this could be extended with action executors)
	dm.logger.Info("Executing action", "actionID", actionID, "sectionID", sectionID)

	// For now, return success - plugins could implement ActionExecutor interface
	return map[string]interface{}{
		"success": true,
		"action":  actionID,
		"section": sectionID,
	}, nil
}

// RefreshSection forces a refresh of a dashboard section
func (dm *DashboardManager) RefreshSection(ctx context.Context, sectionID string) error {
	dm.mu.RLock()
	section, exists := dm.sections[sectionID]
	if !exists {
		dm.mu.RUnlock()
		return fmt.Errorf("section not found: %s", sectionID)
	}

	provider, exists := dm.sectionProviders[section.PluginID]
	dm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("section provider not found for plugin: %s", section.PluginID)
	}

	// Re-discover sections from this plugin
	go dm.discoverSectionsFromPlugin(section.PluginID, provider)

	return nil
}

// GetSectionTypes returns all unique section types
func (dm *DashboardManager) GetSectionTypes() []string {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	typeSet := make(map[string]bool)
	for _, section := range dm.sections {
		typeSet[section.Type] = true
	}

	types := make([]string, 0, len(typeSet))
	for t := range typeSet {
		types = append(types, t)
	}

	sort.Strings(types)
	return types
}
