package pluginmodule

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/database"
	"gorm.io/gorm"
)

// CorePluginManager manages built-in core plugins only
// This is separate from external plugin management and focuses solely on CorePlugin interfaces
type CorePluginManager struct {
	db      *gorm.DB
	logger  hclog.Logger
	mu      sync.RWMutex
	plugins map[string]CorePlugin
}

// NewCorePluginManager creates a new core plugin manager
func NewCorePluginManager(db *gorm.DB) *CorePluginManager {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "core-plugin-manager",
		Level: hclog.Debug,
	})

	return &CorePluginManager{
		db:      db,
		logger:  logger,
		plugins: make(map[string]CorePlugin),
	}
}

// LoadCorePluginsFromRegistry loads all core plugins from the global registry
func (m *CorePluginManager) LoadCorePluginsFromRegistry() error {
	m.logger.Info("loading core plugins from global registry")

	// Get all registered factories from the global registry
	factories := GetGlobalRegistry().GetCorePluginFactories()

	for name, factory := range factories {
		m.logger.Info("creating core plugin", "name", name)

		// Create plugin instance from factory
		plugin := factory()

		// Register the plugin (this also ensures it's in the database)
		if err := m.RegisterCorePlugin(plugin); err != nil {
			m.logger.Error("failed to register core plugin", "name", name, "error", err)
			continue
		}

		m.logger.Info("loaded core plugin from registry", "name", name, "type", plugin.GetPluginType())
	}

	m.logger.Info("core plugin loading completed", "count", len(factories))
	return nil
}

// RegisterCorePlugin registers a core plugin
func (m *CorePluginManager) RegisterCorePlugin(plugin CorePlugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := plugin.GetName()
	m.plugins[name] = plugin

	// Ensure plugin exists in database with "core" type
	if err := m.ensurePluginInDatabase(plugin); err != nil {
		log.Printf("WARNING: Failed to register core plugin %s in database: %v", name, err)
	}

	log.Printf("✅ Registered core plugin: %s (type: %s)", name, plugin.GetPluginType())
	return nil
}

// GetCorePlugin returns a core plugin by name
func (m *CorePluginManager) GetCorePlugin(name string) (CorePlugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	plugin, exists := m.plugins[name]
	return plugin, exists
}

// ListCorePlugins returns all registered core plugins
func (m *CorePluginManager) ListCorePlugins() map[string]CorePlugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]CorePlugin)
	for name, plugin := range m.plugins {
		result[name] = plugin
	}
	return result
}

// ListCorePluginInfo returns info about all core plugins
func (m *CorePluginManager) ListCorePluginInfo() []PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var infos []PluginInfo
	for _, plugin := range m.plugins {
		info := PluginInfo{
			Name:          plugin.GetDisplayName(),
			Type:          plugin.GetPluginType(),
			SupportedExts: plugin.GetSupportedExtensions(),
			Enabled:       plugin.IsEnabled(),
			IsCore:        true,
			Version:       "1.0.0", // Core plugins version
			Description:   fmt.Sprintf("Built-in %s metadata extraction handler", plugin.GetPluginType()),
			Category:      fmt.Sprintf("core_%s", plugin.GetPluginType()),
		}
		infos = append(infos, info)
	}
	return infos
}

// InitializeAllPlugins initializes all core plugins
func (m *CorePluginManager) InitializeAllPlugins() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, plugin := range m.plugins {
		// Check database status first
		status, err := m.getPluginStatusFromDB(name)
		if err != nil {
			log.Printf("WARNING: Failed to get plugin status for %s: %v", name, err)
			status = "enabled" // Default to enabled for core plugins
		}

		if status == "enabled" && plugin.IsEnabled() {
			if err := plugin.Initialize(); err != nil {
				log.Printf("❌ Failed to initialize core plugin %s: %v", name, err)
				continue
			}
			log.Printf("✅ Initialized core plugin: %s", name)
		} else {
			log.Printf("⏸️ Core plugin %s is disabled (status: %s, enabled: %v)", name, status, plugin.IsEnabled())
		}
	}

	return nil
}

// ShutdownAllPlugins shuts down all core plugins
func (m *CorePluginManager) ShutdownAllPlugins() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, plugin := range m.plugins {
		if err := plugin.Shutdown(); err != nil {
			log.Printf("WARNING: Failed to shutdown core plugin %s: %v", name, err)
		}
	}

	return nil
}

// EnablePlugin enables a core plugin
func (m *CorePluginManager) EnablePlugin(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plugin, exists := m.plugins[name]
	if !exists {
		return fmt.Errorf("core plugin not found: %s", name)
	}

	// Update database first
	if err := m.updatePluginStatus(name, "enabled"); err != nil {
		return fmt.Errorf("failed to update plugin status in database: %w", err)
	}

	// Enable and initialize the plugin
	if err := plugin.Enable(); err != nil {
		log.Printf("WARNING: Failed to enable plugin %s: %v", name, err)
		return fmt.Errorf("failed to enable plugin: %w", err)
	}

	log.Printf("✅ Enabled core plugin: %s", name)
	return nil
}

// DisablePlugin disables a core plugin
func (m *CorePluginManager) DisablePlugin(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plugin, exists := m.plugins[name]
	if !exists {
		return fmt.Errorf("core plugin not found: %s", name)
	}

	// Disable first
	if err := plugin.Disable(); err != nil {
		log.Printf("WARNING: Failed to disable plugin during disable: %v", err)
	}

	// Update database
	if err := m.updatePluginStatus(name, "disabled"); err != nil {
		return fmt.Errorf("failed to update plugin status in database: %w", err)
	}

	log.Printf("⏸️ Disabled core plugin: %s", name)
	return nil
}

// GetEnabledFileHandlers returns all enabled core plugins that can handle files
func (m *CorePluginManager) GetEnabledFileHandlers() []FileHandlerPlugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var handlers []FileHandlerPlugin
	for _, plugin := range m.plugins {
		if plugin.IsEnabled() {
			handlers = append(handlers, plugin)
		}
	}
	return handlers
}

// ensurePluginInDatabase ensures the plugin is registered in the database
func (m *CorePluginManager) ensurePluginInDatabase(plugin CorePlugin) error {
	var dbPlugin database.Plugin

	// Check if plugin exists
	result := m.db.Where("plugin_id = ? AND type = ?", plugin.GetName(), "core").First(&dbPlugin)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			// Create new plugin record
			now := time.Now()
			newPlugin := database.Plugin{
				PluginID:    plugin.GetName(),
				Name:        plugin.GetDisplayName(),
				Type:        "core",
				Version:     "1.0.0",
				Status:      "enabled", // Core plugins default to enabled
				Description: fmt.Sprintf("Built-in %s metadata extraction handler", plugin.GetPluginType()),
				InstallPath: fmt.Sprintf("core://%s", plugin.GetName()),
				InstalledAt: now,
				CreatedAt:   now,
				UpdatedAt:   now,
			}

			if err := m.db.Create(&newPlugin).Error; err != nil {
				return fmt.Errorf("failed to create plugin record: %w", err)
			}

			m.logger.Info("registered core plugin", "plugin", plugin.GetName(), "display_name", plugin.GetDisplayName())
		} else {
			return fmt.Errorf("failed to query plugin: %w", result.Error)
		}
	} else {
		// Plugin exists - update display name if it has changed
		displayName := plugin.GetDisplayName()
		if dbPlugin.Name != displayName {
			dbPlugin.Name = displayName
			dbPlugin.UpdatedAt = time.Now()

			if err := m.db.Save(&dbPlugin).Error; err != nil {
				return fmt.Errorf("failed to update plugin name: %w", err)
			}

			m.logger.Info("updated core plugin display name",
				"plugin", plugin.GetName(),
				"old_name", dbPlugin.Name,
				"new_name", displayName)
		}
	}

	return nil
}

// updatePluginStatus updates the plugin status in the database
func (m *CorePluginManager) updatePluginStatus(pluginID, status string) error {
	return m.db.Model(&database.Plugin{}).
		Where("plugin_id = ? AND type = ?", pluginID, "core").
		Update("status", status).Error
}

// getPluginStatusFromDB gets the plugin status from the database
func (m *CorePluginManager) getPluginStatusFromDB(pluginID string) (string, error) {
	var plugin database.Plugin
	err := m.db.Where("plugin_id = ? AND type = ?", pluginID, "core").First(&plugin).Error
	if err != nil {
		return "", err
	}
	return plugin.Status, nil
}
