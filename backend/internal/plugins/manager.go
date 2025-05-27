// Package plugins provides the core plugin management system for Viewra.
package plugins

import (
	"context"
	"crypto/md5"
	"encoding/json" // Still needed for event data
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

// getDB is a helper function to get the GORM DB instance from the interface
func (m *Manager) getDB() *gorm.DB {
	if m.db == nil {
		return nil
	}
	dbInterface := m.db.GetDB()
	if dbInterface == nil {
		return nil
	}
	return dbInterface.(*gorm.DB)
}

// Manager handles plugin lifecycle and management
type Manager struct {
	mu                sync.RWMutex
	plugins           map[string]Plugin           // pluginID -> Plugin instance
	pluginInfos       map[string]*PluginInfo      // pluginID -> PluginInfo
	metadataScrapers  []MetadataScraperPlugin
	adminPagePlugins  []AdminPagePlugin
	uiComponentPlugins []UIComponentPlugin
	scannerPlugins    []ScannerPlugin
	scannerHookPlugins []ScannerHookPlugin
	analyzerPlugins   []AnalyzerPlugin
	notificationPlugins []NotificationPlugin
	hooks             map[string][]HookHandler
	events            chan PluginEventData
	eventBus          events.EventBus             // New system-wide event bus
	db                Database
	pluginDir         string
	devMode           bool
	logger            PluginLogger
	ctx               context.Context             // Manager context
	cancel            context.CancelFunc          // Cancel function for manager context
}

// HookHandler represents a registered hook handler
type HookHandler struct {
	PluginID string
	Handler  func(data interface{}) interface{}
	Priority int
}

// PluginEventData represents an event in the plugin system
type PluginEventData struct {
	PluginID  string
	EventType string
	Message   string
	Data      interface{}
	Timestamp time.Time
}

// NewManager creates a new plugin manager
func NewManager(db Database, pluginDir string, logger PluginLogger) *Manager {
	return &Manager{
		plugins:             make(map[string]Plugin),
		pluginInfos:         make(map[string]*PluginInfo),
		metadataScrapers:    make([]MetadataScraperPlugin, 0),
		adminPagePlugins:    make([]AdminPagePlugin, 0),
		uiComponentPlugins:  make([]UIComponentPlugin, 0),
		scannerPlugins:      make([]ScannerPlugin, 0),
		scannerHookPlugins:  make([]ScannerHookPlugin, 0),
		analyzerPlugins:     make([]AnalyzerPlugin, 0),
		notificationPlugins: make([]NotificationPlugin, 0),
		hooks:               make(map[string][]HookHandler),
		events:              make(chan PluginEventData, 100),
		eventBus:            nil, // Will be set via SetEventBus method
		db:                  db,
		pluginDir:           pluginDir,
		logger:              logger,
	}
}

// SetEventBus sets the system-wide event bus
func (m *Manager) SetEventBus(eventBus events.EventBus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.eventBus = eventBus
}

// Initialize initializes the plugin manager
func (m *Manager) Initialize(ctx context.Context) error {
	m.logger.Info("Initializing plugin manager")
	
	// Create manager context that can be cancelled during shutdown
	m.ctx, m.cancel = context.WithCancel(context.Background())
	
	// Create plugin directory if it doesn't exist
	if err := os.MkdirAll(m.pluginDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}
	
	// Start event processor with manager context
	go m.processEvents(m.ctx)
	
	// Discover and load plugins
	if err := m.DiscoverPlugins(ctx); err != nil {
		return fmt.Errorf("failed to discover plugins: %w", err)
	}
	
	// Load enabled plugins from database
	if err := m.LoadEnabledPlugins(ctx); err != nil {
		return fmt.Errorf("failed to load enabled plugins: %w", err)
	}
	
	m.logger.Info("Plugin manager initialized", "plugins", len(m.plugins))
	return nil
}

// DiscoverPlugins scans the plugin directory for available plugins
func (m *Manager) DiscoverPlugins(ctx context.Context) error {
	m.logger.Info("Discovering plugins", "dir", m.pluginDir)
	
	return filepath.WalkDir(m.pluginDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		
		// Look for plugin manifest files (plugin.yaml, plugin.yml)
		if d.Name() == "plugin.yaml" || d.Name() == "plugin.yml" {
			manifestPath := path
			pluginDir := filepath.Dir(path)
			
			// Read and parse manifest using our helper function
			manifest, err := ReadPluginManifestFile(manifestPath)
			if err != nil {
				m.logger.Error("Failed to read plugin manifest", "path", manifestPath, "error", err)
				return nil // Continue with other plugins
			}
			
			// Create plugin info
			info := &PluginInfo{
				ID:          manifest.ID,
				Name:        manifest.Name,
				Version:     manifest.Version,
				Description: manifest.Description,
				Author:      manifest.Author,
				Website:     manifest.Website,
				Repository:  manifest.Repository,
				License:     manifest.License,
				Type:        manifest.Type,
				Tags:        manifest.Tags,
				Manifest:    manifest,
				InstallPath: pluginDir,
				Status:      PluginStatusDisabled,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}
			
			// Check if plugin exists in database
			var dbPlugin database.Plugin
			db := m.getDB()
			if db != nil {
				if err := db.Where("plugin_id = ?", manifest.ID).First(&dbPlugin).Error; err == nil {
					// Plugin exists in database, update info
					info.Status = PluginStatus(dbPlugin.Status)
					info.CreatedAt = dbPlugin.CreatedAt
					info.UpdatedAt = dbPlugin.UpdatedAt
					
					// Parse config if exists
					if dbPlugin.ConfigData != "" {
						var config map[string]interface{}
						if err := yaml.Unmarshal([]byte(dbPlugin.ConfigData), &config); err == nil {
							info.Config = config
						}
					}
				}
			}
			
			m.mu.Lock()
			m.pluginInfos[manifest.ID] = info
			m.mu.Unlock()
			
			m.logger.Info("Discovered plugin", "id", manifest.ID, "name", manifest.Name, "version", manifest.Version)
		}
		
		return nil
	})
}

// LoadEnabledPlugins loads and starts all enabled plugins
func (m *Manager) LoadEnabledPlugins(ctx context.Context) error {
	db := m.getDB()
	if db == nil {
		// No database available, skip loading enabled plugins
		m.logger.Info("No database available, skipping enabled plugins loading")
		return nil
	}
	
	var enabledPlugins []database.Plugin
	if err := db.Where("status = ?", "enabled").Find(&enabledPlugins).Error; err != nil {
		return fmt.Errorf("failed to query enabled plugins: %w", err)
	}
	
	for _, dbPlugin := range enabledPlugins {
		if err := m.LoadPlugin(ctx, dbPlugin.PluginID); err != nil {
			m.logger.Error("Failed to load enabled plugin", "plugin", dbPlugin.PluginID, "error", err)
			// Update status to error in database
			db.Model(&dbPlugin).Update("status", "error")
			continue
		}
	}
	
	return nil
}

// MigratePluginDatabase handles database migration for a plugin
func (m *Manager) MigratePluginDatabase(plugin Plugin) error {
	dbPlugin, ok := plugin.(DatabasePlugin)
	if !ok {
		// Plugin doesn't implement DatabasePlugin interface, no migration needed
		return nil
	}
	
	db := m.getDB()
	if db == nil {
		return fmt.Errorf("database not available for plugin migration")
	}
	
	m.logger.Info("Migrating database for plugin", "plugin", plugin.Info().ID)
	
	// Get models from plugin
	models := dbPlugin.GetModels()
	if len(models) > 0 {
		// Auto-migrate the plugin's models
		if err := db.AutoMigrate(models...); err != nil {
			return fmt.Errorf("failed to auto-migrate plugin models: %w", err)
		}
		
		// Call plugin's custom migration if needed
		if err := dbPlugin.Migrate(db); err != nil {
			return fmt.Errorf("failed to run plugin migration: %w", err)
		}
		
		m.logger.Info("Database migration completed for plugin", "plugin", plugin.Info().ID, "models", len(models))
	}
	
	return nil
}

// LoadPlugin loads and initializes a specific plugin
func (m *Manager) LoadPlugin(ctx context.Context, pluginID string) error {
	// First, get plugin info and check if already loaded (with lock)
	m.mu.Lock()
	info, exists := m.pluginInfos[pluginID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("plugin not found: %s", pluginID)
	}
	
	// Check if already loaded
	if _, loaded := m.plugins[pluginID]; loaded {
		m.mu.Unlock()
		return fmt.Errorf("plugin already loaded: %s", pluginID)
	}
	m.mu.Unlock()
	
	// Create plugin instance based on type (without lock)
	var plugin Plugin
	var err error
	
	switch info.Type {
	case PluginTypeMetadataScraper:
		plugin, err = m.createMetadataScraperPlugin(info)
	case PluginTypeAdminPage:
		plugin, err = m.createAdminPagePlugin(info)
	case PluginTypeUIComponent:
		plugin, err = m.createUIComponentPlugin(info)
	case PluginTypeScanner:
		plugin, err = m.createScannerPlugin(info)
	case PluginTypeAnalyzer:
		plugin, err = m.createAnalyzerPlugin(info)
	case PluginTypeNotification:
		plugin, err = m.createNotificationPlugin(info)
	default:
		return fmt.Errorf("unsupported plugin type: %s", info.Type)
	}
	
	if err != nil {
		return fmt.Errorf("failed to create plugin instance: %w", err)
	}
	
	// Create plugin context
	pluginCtx := &PluginContext{
		PluginID:   pluginID,
		Logger:     m.createPluginLogger(pluginID),
		Database:   m.db,
		Config:     m.createPluginConfig(pluginID),
		HTTPClient: m.createHTTPClient(),
		FileSystem: m.createFileSystemAccess(info.InstallPath),
		Events:     m.createEventBus(pluginID),
		Hooks:      m.createHookRegistry(pluginID),
	}
	
	// Initialize plugin (without lock)
	if err := plugin.Initialize(pluginCtx); err != nil {
		return fmt.Errorf("failed to initialize plugin: %w", err)
	}
	
	// Migrate plugin database if needed (without lock)
	if err := m.MigratePluginDatabase(plugin); err != nil {
		return fmt.Errorf("failed to migrate plugin database: %w", err)
	}
	
	// Start plugin (without lock - this may call RegisterHook which needs the lock)
	if err := plugin.Start(ctx); err != nil {
		return fmt.Errorf("failed to start plugin: %w", err)
	}
	
	// Now acquire lock to update internal state
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Double-check that plugin wasn't loaded by another goroutine
	if _, loaded := m.plugins[pluginID]; loaded {
		// Plugin was loaded by another goroutine, stop this instance
		plugin.Stop(ctx)
		return fmt.Errorf("plugin already loaded: %s", pluginID)
	}
	
	// Store plugin instance
	m.plugins[pluginID] = plugin
	
	// Add to type-specific lists
	m.addToTypeSpecificList(plugin)
	
	// Update status
	info.Status = PluginStatusEnabled
	info.UpdatedAt = time.Now()
	
	// Update database
	var dbPlugin database.Plugin
	db := m.getDB()
	if db != nil {
		if err := db.Where("plugin_id = ?", pluginID).First(&dbPlugin).Error; err == nil {
			db.Model(&dbPlugin).Updates(map[string]interface{}{
				"status":     "enabled",
				"enabled_at": time.Now(),
			})
		}
	}
	
	// Emit event
	m.emitEvent(PluginEventData{
		PluginID:  pluginID,
		EventType: "plugin_enabled",
		Message:   fmt.Sprintf("Plugin %s enabled successfully", info.Name),
		Timestamp: time.Now(),
	})
	
	m.logger.Info("Plugin loaded successfully", "plugin", pluginID)
	return nil
}

// UnloadPlugin stops and unloads a plugin
func (m *Manager) UnloadPlugin(ctx context.Context, pluginID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	plugin, exists := m.plugins[pluginID]
	if !exists {
		return fmt.Errorf("plugin not loaded: %s", pluginID)
	}
	
	// Stop plugin
	if err := plugin.Stop(ctx); err != nil {
		m.logger.Error("Error stopping plugin", "plugin", pluginID, "error", err)
	}
	
	// Remove from type-specific lists
	m.removeFromTypeSpecificList(plugin)
	
	// Remove from plugins map
	delete(m.plugins, pluginID)
	
	// Update plugin info status
	if info, exists := m.pluginInfos[pluginID]; exists {
		info.Status = PluginStatusDisabled
		info.UpdatedAt = time.Now()
	}
	
	// Update database
	var dbPlugin database.Plugin
	db := m.getDB()
	if db != nil {
		if err := db.Where("plugin_id = ?", pluginID).First(&dbPlugin).Error; err == nil {
			db.Model(&dbPlugin).Update("status", "disabled")
		}
	}
	
	// Emit event
	m.emitEvent(PluginEventData{
		PluginID:  pluginID,
		EventType: "plugin_disabled",
		Message:   fmt.Sprintf("Plugin %s disabled", pluginID),
		Timestamp: time.Now(),
	})
	
	m.logger.Info("Plugin unloaded successfully", "plugin", pluginID)
	return nil
}

// EnablePlugin enables a plugin
func (m *Manager) EnablePlugin(ctx context.Context, pluginID string) error {
	return m.LoadPlugin(ctx, pluginID)
}

// DisablePlugin disables a plugin
func (m *Manager) DisablePlugin(ctx context.Context, pluginID string) error {
	return m.UnloadPlugin(ctx, pluginID)
}

// GetPlugin returns a loaded plugin instance
func (m *Manager) GetPlugin(pluginID string) (Plugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	plugin, exists := m.plugins[pluginID]
	return plugin, exists
}

// GetPluginInfo returns plugin information
func (m *Manager) GetPluginInfo(pluginID string) (*PluginInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	info, exists := m.pluginInfos[pluginID]
	return info, exists
}

// ListPlugins returns all discovered plugins
func (m *Manager) ListPlugins() []*PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	plugins := make([]*PluginInfo, 0, len(m.pluginInfos))
	for _, info := range m.pluginInfos {
		plugins = append(plugins, info)
	}
	
	// Sort by name
	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].Name < plugins[j].Name
	})
	
	return plugins
}

// GetMetadataScrapers returns all loaded metadata scraper plugins
func (m *Manager) GetMetadataScrapers() []MetadataScraperPlugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return append([]MetadataScraperPlugin(nil), m.metadataScrapers...)
}

// GetAdminPagePlugins returns all loaded admin page plugins
func (m *Manager) GetAdminPagePlugins() []AdminPagePlugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return append([]AdminPagePlugin(nil), m.adminPagePlugins...)
}

// GetUIComponentPlugins returns all loaded UI component plugins
func (m *Manager) GetUIComponentPlugins() []UIComponentPlugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return append([]UIComponentPlugin(nil), m.uiComponentPlugins...)
}

// GetScannerHookPlugins returns all loaded scanner hook plugins
func (m *Manager) GetScannerHookPlugins() []ScannerHookPlugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return append([]ScannerHookPlugin(nil), m.scannerHookPlugins...)
}

// RegisterHook registers a hook handler
func (m *Manager) RegisterHook(hookName string, handler HookHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.hooks[hookName] == nil {
		m.hooks[hookName] = make([]HookHandler, 0)
	}
	
	m.hooks[hookName] = append(m.hooks[hookName], handler)
	
	// Sort by priority
	sort.Slice(m.hooks[hookName], func(i, j int) bool {
		return m.hooks[hookName][i].Priority < m.hooks[hookName][j].Priority
	})
}

// ExecuteHook executes all handlers for a specific hook
func (m *Manager) ExecuteHook(hookName string, data interface{}) interface{} {
	m.mu.RLock()
	handlers := m.hooks[hookName]
	m.mu.RUnlock()
	
	result := data
	for _, handler := range handlers {
		result = handler.Handler(result)
	}
	
	return result
}

// RegisterAdminRoutes registers admin routes from all admin page plugins
func (m *Manager) RegisterAdminRoutes(router *gin.RouterGroup) error {
	plugins := m.GetAdminPagePlugins()
	
	for _, plugin := range plugins {
		if err := plugin.RegisterRoutes(router); err != nil {
			m.logger.Error("Failed to register admin routes", "plugin", plugin.Info().ID, "error", err)
			continue
		}
	}
	
	return nil
}

// Shutdown shuts down the plugin manager and all loaded plugins
func (m *Manager) Shutdown(ctx context.Context) error {
	m.logger.Info("Shutting down plugin manager")
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Stop all plugins
	for pluginID, plugin := range m.plugins {
		if err := plugin.Stop(ctx); err != nil {
			m.logger.Error("Error stopping plugin during shutdown", "plugin", pluginID, "error", err)
		}
	}
	
	// Cancel manager context to stop event processor
	if m.cancel != nil {
		m.cancel()
	}
	
	// Close events channel
	close(m.events)
	
	m.logger.Info("Plugin manager shutdown complete")
	return nil
}

// Helper methods

// readPluginManifest was moved to manifest.go as ReadPluginManifestFile

func (m *Manager) addToTypeSpecificList(plugin Plugin) {
	if scraper, ok := plugin.(MetadataScraperPlugin); ok {
		m.metadataScrapers = append(m.metadataScrapers, scraper)
	}
	if adminPage, ok := plugin.(AdminPagePlugin); ok {
		m.adminPagePlugins = append(m.adminPagePlugins, adminPage)
	}
	if uiComponent, ok := plugin.(UIComponentPlugin); ok {
		m.uiComponentPlugins = append(m.uiComponentPlugins, uiComponent)
	}
	if scanner, ok := plugin.(ScannerPlugin); ok {
		m.scannerPlugins = append(m.scannerPlugins, scanner)
	}
	if scannerHook, ok := plugin.(ScannerHookPlugin); ok {
		m.scannerHookPlugins = append(m.scannerHookPlugins, scannerHook)
	}
	if analyzer, ok := plugin.(AnalyzerPlugin); ok {
		m.analyzerPlugins = append(m.analyzerPlugins, analyzer)
	}
	if notification, ok := plugin.(NotificationPlugin); ok {
		m.notificationPlugins = append(m.notificationPlugins, notification)
	}
}

func (m *Manager) removeFromTypeSpecificList(plugin Plugin) {
	pluginID := plugin.Info().ID
	
	// Remove from metadata scrapers
	for i, scraper := range m.metadataScrapers {
		if scraper.Info().ID == pluginID {
			m.metadataScrapers = append(m.metadataScrapers[:i], m.metadataScrapers[i+1:]...)
			break
		}
	}
	
	// Remove from admin page plugins
	for i, adminPage := range m.adminPagePlugins {
		if adminPage.Info().ID == pluginID {
			m.adminPagePlugins = append(m.adminPagePlugins[:i], m.adminPagePlugins[i+1:]...)
			break
		}
	}
	
	// Remove from UI component plugins
	for i, uiComponent := range m.uiComponentPlugins {
		if uiComponent.Info().ID == pluginID {
			m.uiComponentPlugins = append(m.uiComponentPlugins[:i], m.uiComponentPlugins[i+1:]...)
			break
		}
	}
	
	// Remove from scanner plugins
	for i, scanner := range m.scannerPlugins {
		if scanner.Info().ID == pluginID {
			m.scannerPlugins = append(m.scannerPlugins[:i], m.scannerPlugins[i+1:]...)
			break
		}
	}
	
	// Remove from scanner hook plugins
	for i, scannerHook := range m.scannerHookPlugins {
		if scannerHook.Info().ID == pluginID {
			m.scannerHookPlugins = append(m.scannerHookPlugins[:i], m.scannerHookPlugins[i+1:]...)
			break
		}
	}
	
	// Remove from analyzer plugins
	for i, analyzer := range m.analyzerPlugins {
		if analyzer.Info().ID == pluginID {
			m.analyzerPlugins = append(m.analyzerPlugins[:i], m.analyzerPlugins[i+1:]...)
			break
		}
	}
	
	// Remove from notification plugins
	for i, notification := range m.notificationPlugins {
		if notification.Info().ID == pluginID {
			m.notificationPlugins = append(m.notificationPlugins[:i], m.notificationPlugins[i+1:]...)
			break
		}
	}
}

func (m *Manager) emitEvent(event PluginEventData) {
	// Send to legacy plugin event channel
	select {
	case m.events <- event:
	default:
		// Channel full, drop event
		m.logger.Warn("Plugin event channel full, dropping event", "event", event.EventType)
	}
	
	// Also publish to the new system-wide event bus
	if m.eventBus != nil {
		systemEvent := events.NewPluginEvent(
			events.EventType("plugin."+event.EventType), 
			event.PluginID,
			fmt.Sprintf("Plugin %s: %s", event.PluginID, event.EventType),
			event.Message,
		)
		
		// Add plugin-specific data
		if event.Data != nil {
			if dataMap, ok := event.Data.(map[string]interface{}); ok {
				systemEvent.Data = dataMap
			} else {
				systemEvent.Data["original_data"] = event.Data
			}
		}
		
		systemEvent.Data["plugin_id"] = event.PluginID
		systemEvent.Tags = []string{"plugin", event.PluginID}
		
		// Publish asynchronously to avoid blocking
		if err := m.eventBus.PublishAsync(systemEvent); err != nil {
			m.logger.Warn("Failed to publish event to system event bus", "error", err, "event_type", event.EventType)
		}
	}
}

func (m *Manager) processEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-m.events:
			if !ok {
				return
			}
			
			// Store event in database
			m.storeEvent(event)
			
			// Log event
			m.logger.Info("Plugin event", "plugin", event.PluginID, "type", event.EventType, "message", event.Message)
		}
	}
}

func (m *Manager) storeEvent(event PluginEventData) {
	// Skip storing events if no database is available
	db := m.getDB()
	if db == nil {
		return
	}
	
	// Get plugin database ID
	var dbPlugin database.Plugin
	if err := db.Where("plugin_id = ?", event.PluginID).First(&dbPlugin).Error; err != nil {
		return // Plugin not in database
	}
	
	// Serialize event data
	var dataJson string
	if event.Data != nil {
		if data, err := json.Marshal(event.Data); err == nil {
			dataJson = string(data)
		}
	}
	
	// Create event record
	pluginEvent := database.PluginEvent{
		PluginID:  dbPlugin.ID,
		EventType: event.EventType,
		Message:   event.Message,
		Data:      dataJson,
		CreatedAt: event.Timestamp,
	}
	
	db.Create(&pluginEvent)
}

// Plugin factory methods - generic plugin loading
func (m *Manager) createMetadataScraperPlugin(info *PluginInfo) (Plugin, error) {
	return m.createGenericPlugin(info)
}

func (m *Manager) createAdminPagePlugin(info *PluginInfo) (Plugin, error) {
	return m.createGenericPlugin(info)
}

func (m *Manager) createUIComponentPlugin(info *PluginInfo) (Plugin, error) {
	return m.createGenericPlugin(info)
}

func (m *Manager) createScannerPlugin(info *PluginInfo) (Plugin, error) {
	return m.createGenericPlugin(info)
}

func (m *Manager) createAnalyzerPlugin(info *PluginInfo) (Plugin, error) {
	return m.createGenericPlugin(info)
}

func (m *Manager) createNotificationPlugin(info *PluginInfo) (Plugin, error) {
	return m.createGenericPlugin(info)
}

// createGenericPlugin creates a plugin instance using the plugin registry or fallback to basic plugin
func (m *Manager) createGenericPlugin(info *PluginInfo) (Plugin, error) {
	// Try to create plugin using the registry (self-registered plugins)
	registry := GetGlobalRegistry()
	if registry.IsRegistered(info.ID) {
		plugin, err := registry.CreatePlugin(info.ID)
		if err == nil {
			m.logger.Info("Created plugin using registry", "plugin", info.ID)
			return plugin, nil
		}
		m.logger.Warn("Failed to create plugin from registry", "plugin", info.ID, "error", err)
	}
	
	// Try to load as Go plugin (.so file)
	if plugin, err := m.loadGoPlugin(info); err == nil {
		return plugin, nil
	}
	
	// Try to load using plugin factory function
	if plugin, err := m.loadPluginFactory(info); err == nil {
		return plugin, nil
	}
	
	// Fallback to basic plugin for non-Go plugins or when other loading methods fail
	m.logger.Info("Using basic plugin implementation", "plugin", info.ID)
	return NewBasicPlugin(info), nil
}

// loadGoPlugin attempts to load a Go plugin from a .so file
func (m *Manager) loadGoPlugin(info *PluginInfo) (Plugin, error) {
	// This would implement Go's plugin system (.so loading)
	// For now, return an error to indicate Go plugin loading is not available
	return nil, fmt.Errorf("Go plugin loading not implemented")
}

// loadPluginFactory attempts to load a plugin using a factory function from the plugin directory
func (m *Manager) loadPluginFactory(info *PluginInfo) (Plugin, error) {
	// For now, we'll implement a simple factory approach for known plugins
	switch info.ID {
	case "musicbrainz_enricher":
		return m.createMusicBrainzPlugin(info)
	default:
		return nil, fmt.Errorf("no factory available for plugin: %s", info.ID)
	}
}

// createMusicBrainzPlugin creates a MusicBrainz enricher plugin instance
func (m *Manager) createMusicBrainzPlugin(info *PluginInfo) (Plugin, error) {
	m.logger.Info("Creating MusicBrainz enricher plugin", "plugin", info.ID)
	
	// Return the actual MusicBrainz plugin implementation
	return &realMusicBrainzPlugin{
		info: info,
		logger: m.logger,
	}, nil
}

// realMusicBrainzPlugin is the actual implementation of the MusicBrainz enricher plugin
type realMusicBrainzPlugin struct {
	info   *PluginInfo
	logger PluginLogger
	ctx    *PluginContext
	db     *gorm.DB
	config *MusicBrainzConfig
}

// MusicBrainzConfig represents the plugin configuration
type MusicBrainzConfig struct {
	Enabled     bool   `json:"enabled"`
	AutoEnrich  bool   `json:"auto_enrich"`
	APIBaseURL  string `json:"api_base_url"`
	UserAgent   string `json:"user_agent"`
	RateLimit   int    `json:"rate_limit"`
	CacheTTL    int    `json:"cache_ttl"`
	MaxRetries  int    `json:"max_retries"`
	Timeout     int    `json:"timeout"`
}

// MusicBrainzCache represents cached MusicBrainz API responses
type MusicBrainzCache struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	QueryHash string    `gorm:"uniqueIndex;not null" json:"query_hash"`
	QueryType string    `gorm:"not null" json:"query_type"` // "recording", "release", "artist"
	Response  string    `gorm:"type:text;not null" json:"response"`
	ExpiresAt time.Time `gorm:"not null;index" json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// MusicBrainzEnrichment represents enrichment data for media files
type MusicBrainzEnrichment struct {
	ID                     uint       `gorm:"primaryKey" json:"id"`
	MediaFileID            uint       `gorm:"uniqueIndex;not null" json:"media_file_id"`
	MusicBrainzRecordingID string     `gorm:"index" json:"musicbrainz_recording_id,omitempty"`
	MusicBrainzReleaseID   string     `gorm:"index" json:"musicbrainz_release_id,omitempty"`
	MusicBrainzArtistID    string     `gorm:"index" json:"musicbrainz_artist_id,omitempty"`
	EnrichedTitle          string     `json:"enriched_title,omitempty"`
	EnrichedArtist         string     `json:"enriched_artist,omitempty"`
	EnrichedAlbum          string     `json:"enriched_album,omitempty"`
	EnrichedAlbumArtist    string     `json:"enriched_album_artist,omitempty"`
	EnrichedYear           int        `json:"enriched_year,omitempty"`
	EnrichedGenre          string     `json:"enriched_genre,omitempty"`
	EnrichedTrackNumber    int        `json:"enriched_track_number,omitempty"`
	EnrichedDiscNumber     int        `json:"enriched_disc_number,omitempty"`
	MatchScore             float64    `json:"match_score"`
	ArtworkURL             string     `json:"artwork_url,omitempty"`
	ArtworkPath            string     `json:"artwork_path,omitempty"`
	EnrichedAt             time.Time  `gorm:"not null" json:"enriched_at"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

// MusicBrainz API types
type Recording struct {
	ID           string        `json:"id"`
	Title        string        `json:"title"`
	Score        float64       `json:"score"`
	ArtistCredit []ArtistCredit `json:"artist-credit"`
	Releases     []Release     `json:"releases"`
}

type ArtistCredit struct {
	Name   string `json:"name"`
	Artist Artist `json:"artist"`
}

type Artist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Release struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Date  string `json:"date"`
}

type SearchResponse struct {
	Recordings []Recording `json:"recordings"`
	Count      int         `json:"count"`
	Offset     int         `json:"offset"`
}

func (p *realMusicBrainzPlugin) Initialize(ctx *PluginContext) error {
	p.ctx = ctx
	
	// Get database connection
	dbInterface := ctx.Database.GetDB()
	if dbInterface == nil {
		return fmt.Errorf("database not available")
	}
	p.db = dbInterface.(*gorm.DB)
	
	// Load configuration with defaults
	p.config = &MusicBrainzConfig{
		Enabled:     true,
		AutoEnrich:  true,
		APIBaseURL:  "https://musicbrainz.org/ws/2",
		UserAgent:   "Viewra/1.0 (https://github.com/mantonx/viewra)",
		RateLimit:   1,     // 1 request per second (MusicBrainz rate limit)
		CacheTTL:    86400, // 24 hours
		MaxRetries:  3,
		Timeout:     30,
	}
	
	// Override with user configuration if available
	if ctx.Config.GetBool("enabled") {
		p.config.Enabled = ctx.Config.GetBool("enabled")
	}
	if ctx.Config.GetBool("auto_enrich") {
		p.config.AutoEnrich = ctx.Config.GetBool("auto_enrich")
	}
	
	p.logger.Info("MusicBrainz enricher plugin initialized", "plugin", p.info.ID)
	return nil
}

func (p *realMusicBrainzPlugin) Start(ctx context.Context) error {
	p.logger.Info("MusicBrainz enricher plugin started", "plugin", p.info.ID)
	
	// Register scanner hook for automatic enrichment
	if p.ctx != nil && p.ctx.Hooks != nil {
		p.ctx.Hooks.Register("media_file_scanned", func(data interface{}) interface{} {
			// Extract media file info from hook data
			hookData, ok := data.(map[string]interface{})
			if !ok {
				return nil
			}
			
			mediaFileID, ok := hookData["media_file_id"].(uint)
			if !ok {
				return nil
			}
			
			p.logger.Info("MusicBrainz enricher processing media file", "media_file_id", mediaFileID)
			
			// Enrich in background
			go func() {
				if err := p.EnrichMediaFile(context.Background(), mediaFileID); err != nil {
					p.logger.Error("Failed to enrich media file", "media_file_id", mediaFileID, "error", err)
				}
			}()
			
			return nil
		})
		
		p.logger.Info("MusicBrainz enricher registered scanner hook")
	}
	
	return nil
}

func (p *realMusicBrainzPlugin) Stop(ctx context.Context) error {
	p.logger.Info("MusicBrainz enricher plugin stopped", "plugin", p.info.ID)
	return nil
}

func (p *realMusicBrainzPlugin) Info() *PluginInfo {
	return p.info
}

func (p *realMusicBrainzPlugin) Health() error {
	// Check database connection
	if p.db == nil {
		return fmt.Errorf("database connection not available")
	}
	
	// Check MusicBrainz API connectivity
	resp, err := http.Get(p.config.APIBaseURL + "/artist/5b11f4ce-a62d-471e-81fc-a69a8278c7da?fmt=json")
	if err != nil {
		return fmt.Errorf("MusicBrainz API not reachable: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return fmt.Errorf("MusicBrainz API returned status %d", resp.StatusCode)
	}
	
	return nil
}

// Implement DatabasePlugin interface
func (p *realMusicBrainzPlugin) GetModels() []interface{} {
	return []interface{}{
		&MusicBrainzCache{},
		&MusicBrainzEnrichment{},
	}
}

func (p *realMusicBrainzPlugin) Migrate(db interface{}) error {
	// Custom migration logic if needed
	return nil
}

func (p *realMusicBrainzPlugin) Rollback(db interface{}) error {
	gormDB := db.(*gorm.DB)
	return gormDB.Migrator().DropTable(
		&MusicBrainzCache{},
		&MusicBrainzEnrichment{},
	)
}

// Implement ScannerHookPlugin interface
func (p *realMusicBrainzPlugin) OnMediaFileScanned(mediaFileID uint, filePath string, metadata map[string]interface{}) error {
	if !p.config.AutoEnrich {
		return nil
	}
	
	return p.EnrichMediaFile(context.Background(), mediaFileID)
}

func (p *realMusicBrainzPlugin) OnScanStarted(scanJobID uint, libraryID uint, libraryPath string) error {
	p.logger.Info("MusicBrainz enricher scan started", "scan_job_id", scanJobID, "library_id", libraryID)
	return nil
}

func (p *realMusicBrainzPlugin) OnScanCompleted(scanJobID uint, libraryID uint, stats map[string]interface{}) error {
	p.logger.Info("MusicBrainz enricher scan completed", "scan_job_id", scanJobID, "library_id", libraryID)
	return nil
}

// Implement MetadataScraperPlugin interface
func (p *realMusicBrainzPlugin) CanHandle(filePath string, mimeType string) bool {
	if !p.config.Enabled {
		return false
	}
	
	// Check if it's an audio file
	audioTypes := []string{
		"audio/mpeg", "audio/mp3", "audio/flac", "audio/ogg", 
		"audio/wav", "audio/aac", "audio/m4a",
	}
	
	for _, audioType := range audioTypes {
		if strings.Contains(mimeType, audioType) {
			return true
		}
	}
	
	// Check file extension as fallback
	if strings.Contains(filePath, ".") {
		ext := strings.ToLower(filePath[strings.LastIndex(filePath, ".")+1:])
		audioExts := []string{"mp3", "flac", "ogg", "wav", "aac", "m4a", "wma"}
		
		for _, audioExt := range audioExts {
			if ext == audioExt {
				return true
			}
		}
	}
	
	return false
}

func (p *realMusicBrainzPlugin) ExtractMetadata(ctx context.Context, filePath string) (map[string]interface{}, error) {
	if !p.CanHandle(filePath, "") {
		return nil, fmt.Errorf("file type not supported: %s", filePath)
	}
	
	// This plugin enriches existing metadata rather than extracting raw metadata
	return map[string]interface{}{
		"plugin":      "musicbrainz_enricher",
		"file_path":   filePath,
		"supported":   true,
		"enrichment":  "available",
	}, nil
}

func (p *realMusicBrainzPlugin) SupportedTypes() []string {
	return []string{
		"audio/mpeg",     // MP3
		"audio/flac",     // FLAC
		"audio/ogg",      // OGG
		"audio/wav",      // WAV
		"audio/aac",      // AAC
		"audio/m4a",      // M4A
		"audio/wma",      // WMA
	}
}

// EnrichMediaFile enriches a media file with MusicBrainz data
func (p *realMusicBrainzPlugin) EnrichMediaFile(ctx context.Context, mediaFileID uint) error {
	// Check if already enriched
	var existing MusicBrainzEnrichment
	if err := p.db.Where("media_file_id = ?", mediaFileID).First(&existing).Error; err == nil {
		p.logger.Debug("Media file already enriched", "media_file_id", mediaFileID)
		return nil
	}
	
	// Get media file from main database
	var mediaFile database.MediaFile
	if err := p.db.Where("id = ?", mediaFileID).First(&mediaFile).Error; err != nil {
		return fmt.Errorf("failed to get media file: %w", err)
	}
	
	// Get music metadata separately
	var musicMetadata database.MusicMetadata
	if err := p.db.Where("media_file_id = ?", mediaFileID).First(&musicMetadata).Error; err != nil {
		p.logger.Debug("No music metadata available for enrichment", "media_file_id", mediaFileID)
		return nil
	}
	
	// Search MusicBrainz
	recording, err := p.searchRecording(musicMetadata.Title, musicMetadata.Artist, musicMetadata.Album)
	if err != nil {
		return fmt.Errorf("failed to search MusicBrainz: %w", err)
	}
	
	if recording == nil {
		p.logger.Debug("No MusicBrainz match found", "media_file_id", mediaFileID)
		return nil
	}
	
	// Create enrichment record
	enrichment := &MusicBrainzEnrichment{
		MediaFileID:            mediaFileID,
		MusicBrainzRecordingID: recording.ID,
		EnrichedTitle:          recording.Title,
		MatchScore:             recording.Score,
		EnrichedAt:             time.Now(),
	}
	
	// Add artist information
	if len(recording.ArtistCredit) > 0 {
		enrichment.EnrichedArtist = recording.ArtistCredit[0].Name
		enrichment.MusicBrainzArtistID = recording.ArtistCredit[0].Artist.ID
	}
	
	// Add release information if available
	if len(recording.Releases) > 0 {
		release := recording.Releases[0]
		enrichment.MusicBrainzReleaseID = release.ID
		enrichment.EnrichedAlbum = release.Title
		if release.Date != "" && len(release.Date) >= 4 {
			if year, err := strconv.Atoi(release.Date[:4]); err == nil {
				enrichment.EnrichedYear = year
			}
		}
	}
	
	// Save enrichment
	if err := p.db.Create(enrichment).Error; err != nil {
		return fmt.Errorf("failed to save enrichment: %w", err)
	}
	
	p.logger.Info("Media file enriched successfully", 
		"media_file_id", mediaFileID, 
		"recording_id", recording.ID,
		"match_score", recording.Score)
	
	return nil
}

// searchRecording searches MusicBrainz for a recording
func (p *realMusicBrainzPlugin) searchRecording(title, artist, album string) (*Recording, error) {
	// Build search query
	query := fmt.Sprintf("recording:\"%s\" AND artist:\"%s\"", title, artist)
	if album != "" {
		query += fmt.Sprintf(" AND release:\"%s\"", album)
	}
	
	// Generate cache key
	queryHash := p.generateQueryHash(query)
	
	// Check cache first
	if cached, err := p.getCachedResponse("recording", queryHash); err == nil {
		if len(cached) > 0 {
			return &cached[0], nil
		}
		return nil, nil
	}
	
	// Make API request
	apiURL := fmt.Sprintf("%s/recording?query=%s&fmt=json&limit=5", 
		p.config.APIBaseURL, url.QueryEscape(query))
	
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	var searchResp SearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	// Cache the response
	p.cacheRecordings("recording", queryHash, searchResp.Recordings)
	
	// Return best match (first result is usually best)
	if len(searchResp.Recordings) > 0 {
		return &searchResp.Recordings[0], nil
	}
	
	return nil, nil
}

// Helper methods
func (p *realMusicBrainzPlugin) generateQueryHash(query string) string {
	h := md5.New()
	h.Write([]byte(query))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (p *realMusicBrainzPlugin) getCachedResponse(queryType, queryHash string) ([]Recording, error) {
	var cache MusicBrainzCache
	err := p.db.Where("query_type = ? AND query_hash = ? AND expires_at > ?",
		queryType, queryHash, time.Now()).First(&cache).Error
	if err != nil {
		return nil, err
	}
	
	var recordings []Recording
	if err := json.Unmarshal([]byte(cache.Response), &recordings); err != nil {
		return nil, err
	}
	
	return recordings, nil
}

func (p *realMusicBrainzPlugin) cacheRecordings(queryType, queryHash string, recordings []Recording) {
	responseData, err := json.Marshal(recordings)
	if err != nil {
		return // Skip caching on error
	}
	
	expiresAt := time.Now().Add(time.Duration(p.config.CacheTTL) * time.Second)
	
	cache := MusicBrainzCache{
		QueryHash: queryHash,
		QueryType: queryType,
		Response:  string(responseData),
		ExpiresAt: expiresAt,
	}
	
	p.db.Save(&cache)
}

// Helper factory methods for plugin context
func (m *Manager) createPluginLogger(pluginID string) PluginLogger {
	return &basicPluginLogger{pluginID: pluginID, logger: m.logger}
}

func (m *Manager) createPluginConfig(pluginID string) PluginConfig {
	return &basicPluginConfig{pluginID: pluginID, manager: m}
}

func (m *Manager) createHTTPClient() HTTPClient {
	return &basicHTTPClient{}
}

func (m *Manager) createFileSystemAccess(basePath string) FileSystemAccess {
	return &basicFileSystemAccess{basePath: basePath}
}

func (m *Manager) createEventBus(pluginID string) EventBus {
	return &basicEventBus{pluginID: pluginID, manager: m}
}

func (m *Manager) createHookRegistry(pluginID string) HookRegistry {
	return &basicHookRegistry{pluginID: pluginID, manager: m}
}
