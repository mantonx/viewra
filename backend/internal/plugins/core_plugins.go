package plugins

import (
	"fmt"
	"log"
	"sync"

	"github.com/mantonx/viewra/internal/database"
	"gorm.io/gorm"
)

// CorePluginManager manages built-in core plugins
type CorePluginManager struct {
	db      *gorm.DB
	mu      sync.RWMutex
	plugins map[string]CorePlugin
}

// NewCorePluginManager creates a new core plugin manager
func NewCorePluginManager(db *gorm.DB) *CorePluginManager {
	return &CorePluginManager{
		db:      db,
		plugins: make(map[string]CorePlugin),
	}
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
			Name:          fmt.Sprintf("Core %s Plugin", plugin.GetPluginType()),
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
	
	// Initialize the plugin
	if err := plugin.Initialize(); err != nil {
		log.Printf("WARNING: Failed to initialize enabled plugin %s: %v", name, err)
		return fmt.Errorf("failed to initialize plugin: %w", err)
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
	
	// Shutdown first
	if err := plugin.Shutdown(); err != nil {
		log.Printf("WARNING: Failed to shutdown plugin during disable: %v", err)
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
			// CorePlugin extends FileHandlerPlugin, so this is safe
			handlers = append(handlers, plugin)
		}
	}
	return handlers
}

// Database operations

func (m *CorePluginManager) ensurePluginInDatabase(plugin CorePlugin) error {
	if m.db == nil {
		return fmt.Errorf("database not available")
	}
	
	var existingPlugin database.Plugin
	err := m.db.Where("plugin_id = ?", plugin.GetName()).First(&existingPlugin).Error
	
	if err == gorm.ErrRecordNotFound {
		// Create new plugin record
		dbPlugin := database.Plugin{
			PluginID:    plugin.GetName(),
			Name:        plugin.GetName(),
			Version:     "1.0.0",
			Description: fmt.Sprintf("Built-in %s handler", plugin.GetPluginType()),
			Type:        "core",
			Status:      "enabled", // Core plugins are enabled by default
			InstallPath: "core",    // Mark as core plugin
		}
		
		return m.db.Create(&dbPlugin).Error
	}
	
	return err
}

func (m *CorePluginManager) updatePluginStatus(pluginID, status string) error {
	if m.db == nil {
		return fmt.Errorf("database not available")
	}
	
	return m.db.Model(&database.Plugin{}).
		Where("plugin_id = ?", pluginID).
		Update("status", status).Error
}

func (m *CorePluginManager) getPluginStatusFromDB(pluginID string) (string, error) {
	if m.db == nil {
		return "", fmt.Errorf("database not available")
	}
	
	var plugin database.Plugin
	err := m.db.Where("plugin_id = ?", pluginID).First(&plugin).Error
	if err != nil {
		return "", err
	}
	
	return plugin.Status, nil
} 