package enrichment

import (
	"fmt"
	"log"
	"sync"

	"github.com/mantonx/viewra/internal/config"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/modules/enrichmentmodule"
	"gorm.io/gorm"
)

// =============================================================================
// INTERNAL ENRICHMENT PLUGIN MANAGER
// =============================================================================
// This manager coordinates internal enrichment plugins and provides a unified
// interface for the scanning system to trigger enrichment operations.

// Manager coordinates internal enrichment plugins
type Manager struct {
	db               *gorm.DB
	enrichmentModule *enrichmentmodule.Module
	plugins          map[string]InternalEnrichmentPlugin
	pluginOrder      []string
	mutex            sync.RWMutex
	enabled          bool
	config           *config.Config
}

// NewManager creates a new internal enrichment plugin manager
func NewManager(db *gorm.DB, enrichmentModule *enrichmentmodule.Module, config *config.Config) *Manager {
	return &Manager{
		db:               db,
		enrichmentModule: enrichmentModule,
		plugins:          make(map[string]InternalEnrichmentPlugin),
		pluginOrder:      make([]string, 0),
		enabled:          true,
		config:           config,
	}
}

// RegisterPlugin registers an internal enrichment plugin
func (m *Manager) RegisterPlugin(plugin InternalEnrichmentPlugin) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	name := plugin.GetName()
	if _, exists := m.plugins[name]; exists {
		return fmt.Errorf("plugin %s is already registered", name)
	}

	// Initialize the plugin
	if err := plugin.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize plugin %s: %w", name, err)
	}

	m.plugins[name] = plugin
	m.pluginOrder = append(m.pluginOrder, name)

	log.Printf("INFO: Registered internal enrichment plugin: %s", name)
	return nil
}

// UnregisterPlugin removes an internal enrichment plugin
func (m *Manager) UnregisterPlugin(name string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.plugins[name]; !exists {
		return fmt.Errorf("plugin %s is not registered", name)
	}

	delete(m.plugins, name)

	// Remove from order slice
	for i, pluginName := range m.pluginOrder {
		if pluginName == name {
			m.pluginOrder = append(m.pluginOrder[:i], m.pluginOrder[i+1:]...)
			break
		}
	}

	log.Printf("INFO: Unregistered internal enrichment plugin: %s", name)
	return nil
}

// GetPlugin returns a registered plugin by name
func (m *Manager) GetPlugin(name string) (InternalEnrichmentPlugin, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	plugin, exists := m.plugins[name]
	return plugin, exists
}

// ListPlugins returns all registered plugin names in registration order
func (m *Manager) ListPlugins() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Return a copy to prevent external modification
	result := make([]string, len(m.pluginOrder))
	copy(result, m.pluginOrder)
	return result
}

// OnMediaFileScanned processes a scanned media file with all applicable plugins
func (m *Manager) OnMediaFileScanned(mediaFile *database.MediaFile, metadata interface{}) error {
	if !m.enabled {
		return nil
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Get library information to check restrictions
	var library database.MediaLibrary
	if err := m.db.First(&library, mediaFile.LibraryID).Error; err != nil {
		log.Printf("WARN: Failed to get library info for enrichment filtering: %v", err)
		return nil // Don't fail scanning if we can't get library info
	}

	// Get library-specific plugin restrictions from config
	librarySettings, hasRestrictions := m.config.LibraryPluginRestrictions[library.Type]
	if !hasRestrictions {
		log.Printf("DEBUG: No plugin restrictions configured for library type: %s", library.Type)
		// If no restrictions configured, use all plugins (backwards compatibility)
	} else {
		log.Printf("DEBUG: Applying plugin restrictions for library type: %s", library.Type)
	}

	// Convert metadata to map[string]string for internal plugins
	var metadataMap map[string]string
	if metadata != nil {
		switch v := metadata.(type) {
		case map[string]string:
			metadataMap = v
		case map[string]interface{}:
			// Convert map[string]interface{} to map[string]string
			metadataMap = make(map[string]string)
			for key, value := range v {
				if str, ok := value.(string); ok {
					metadataMap[key] = str
				} else {
					metadataMap[key] = fmt.Sprintf("%v", value)
				}
			}
		default:
			// For other types, create empty map and log
			metadataMap = make(map[string]string)
			log.Printf("DEBUG: Unsupported metadata type %T for file %s", metadata, mediaFile.Path)
		}
	} else {
		metadataMap = make(map[string]string)
	}

	var errors []error

	// Process with each plugin that can handle this media file
	for _, pluginName := range m.pluginOrder {
		plugin := m.plugins[pluginName]

		if !plugin.CanEnrich(mediaFile) {
			continue
		}

		// Check if plugin is allowed for this library type
		if hasRestrictions && !m.isPluginAllowedForLibrary(pluginName, librarySettings) {
			log.Printf("DEBUG: Plugin %s not allowed for library type %s, skipping", pluginName, library.Type)
			continue
		}

		log.Printf("DEBUG: Processing media file %s with plugin %s", mediaFile.ID, pluginName)

		if err := plugin.OnMediaFileScanned(mediaFile, metadataMap); err != nil {
			log.Printf("WARN: Plugin %s failed to process media file %s: %v", pluginName, mediaFile.ID, err)
			errors = append(errors, fmt.Errorf("plugin %s: %w", pluginName, err))
			// Continue with other plugins even if one fails
		}
	}

	// Return first error if any occurred
	if len(errors) > 0 {
		return errors[0]
	}

	return nil
}

// isPluginAllowedForLibrary checks if a plugin is allowed for the given library type
func (m *Manager) isPluginAllowedForLibrary(pluginName string, settings config.LibraryPluginSettings) bool {
	// Check if plugin is explicitly disallowed
	for _, disallowed := range settings.EnrichmentPlugins.DisallowedPlugins {
		if pluginName == disallowed {
			return false
		}
	}

	// If there are allowed plugins specified, check if this plugin is in the list
	if len(settings.EnrichmentPlugins.AllowedPlugins) > 0 {
		for _, allowed := range settings.EnrichmentPlugins.AllowedPlugins {
			if pluginName == allowed {
				return true
			}
		}
		// Plugin not in allowed list
		return false
	}

	// Check if it's a shared plugin that can run across library types
	for _, sharedPlugin := range settings.SharedPlugins.SharedPluginNames {
		if pluginName == sharedPlugin {
			return true
		}
	}

	// If no specific allowed plugins and not disallowed, allow by default
	return true
}

// OnScanStarted is called when a scan starts (ScannerPluginHook interface)
func (m *Manager) OnScanStarted(jobID, libraryID uint, path string) error {
	if !m.enabled {
		return nil
	}

	log.Printf("INFO: Enrichment manager notified - scan started (job: %d, library: %d, path: %s)", jobID, libraryID, path)
	return nil
}

// OnScanCompleted is called when a scan completes (ScannerPluginHook interface)
func (m *Manager) OnScanCompleted(jobID, libraryID uint, stats map[string]interface{}) error {
	if !m.enabled {
		return nil
	}

	log.Printf("INFO: Enrichment manager notified - scan completed (job: %d, library: %d)", jobID, libraryID)

	// Optionally: Trigger batch enrichment application for newly scanned files
	// This could queue enrichment jobs for all files in the library

	return nil
}

// EnrichMediaFile manually triggers enrichment for a specific media file
func (m *Manager) EnrichMediaFile(mediaFile *database.MediaFile, metadata map[string]string) error {
	if !m.enabled {
		return nil
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var errors []error

	// Process with each plugin that can handle this media file
	for _, pluginName := range m.pluginOrder {
		plugin := m.plugins[pluginName]

		if !plugin.CanEnrich(mediaFile) {
			continue
		}

		log.Printf("DEBUG: Enriching media file %s with plugin %s", mediaFile.ID, pluginName)

		if err := plugin.EnrichMediaFile(mediaFile, metadata); err != nil {
			log.Printf("WARN: Plugin %s failed to enrich media file %s: %v", pluginName, mediaFile.ID, err)
			errors = append(errors, fmt.Errorf("plugin %s: %w", pluginName, err))
			// Continue with other plugins even if one fails
		}
	}

	// Return first error if any occurred
	if len(errors) > 0 {
		return errors[0]
	}

	return nil
}

// GetPluginCount returns the number of registered plugins
func (m *Manager) GetPluginCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return len(m.plugins)
}

// IsEnabled returns whether the manager is enabled
func (m *Manager) IsEnabled() bool {
	return m.enabled
}

// SetEnabled enables or disables the manager
func (m *Manager) SetEnabled(enabled bool) {
	m.enabled = enabled

	if enabled {
		log.Printf("INFO: Internal enrichment plugin manager enabled")
	} else {
		log.Printf("INFO: Internal enrichment plugin manager disabled")
	}
}

// ClearAllCaches clears caches for all cacheable plugins
func (m *Manager) ClearAllCaches() error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var errors []error

	for name, plugin := range m.plugins {
		if cacheablePlugin, ok := plugin.(CacheablePlugin); ok {
			if err := cacheablePlugin.ClearCache(); err != nil {
				log.Printf("WARN: Failed to clear cache for plugin %s: %v", name, err)
				errors = append(errors, fmt.Errorf("plugin %s: %w", name, err))
			} else {
				log.Printf("INFO: Cleared cache for plugin %s", name)
			}
		}
	}

	if len(errors) > 0 {
		return errors[0]
	}

	return nil
}
