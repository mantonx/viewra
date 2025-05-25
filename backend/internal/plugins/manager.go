// Package plugins provides the core plugin management system for Viewra.
package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"gorm.io/gorm"
)

// getDB is a helper function to get the GORM DB instance from the interface
func (m *Manager) getDB() *gorm.DB {
	return m.db.GetDB().(*gorm.DB)
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
	analyzerPlugins   []AnalyzerPlugin
	notificationPlugins []NotificationPlugin
	hooks             map[string][]HookHandler
	events            chan PluginEventData
	eventBus          events.EventBus             // New system-wide event bus
	db                Database
	pluginDir         string
	devMode           bool
	logger            PluginLogger
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
	
	// Create plugin directory if it doesn't exist
	if err := os.MkdirAll(m.pluginDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}
	
	// Start event processor
	go m.processEvents(ctx)
	
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
		
		// Look for plugin.json files
		if d.Name() == "plugin.json" {
			manifestPath := path
			pluginDir := filepath.Dir(path)
			
			// Read and parse manifest
			manifest, err := m.readPluginManifest(manifestPath)
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
			if err := m.getDB().Where("plugin_id = ?", manifest.ID).First(&dbPlugin).Error; err == nil {
				// Plugin exists in database, update info
				info.Status = PluginStatus(dbPlugin.Status)
				info.CreatedAt = dbPlugin.CreatedAt
				info.UpdatedAt = dbPlugin.UpdatedAt
				
				// Parse config if exists
				if dbPlugin.ConfigData != "" {
					var config map[string]interface{}
					if err := json.Unmarshal([]byte(dbPlugin.ConfigData), &config); err == nil {
						info.Config = config
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
	var enabledPlugins []database.Plugin
	if err := m.getDB().Where("status = ?", "enabled").Find(&enabledPlugins).Error; err != nil {
		return fmt.Errorf("failed to query enabled plugins: %w", err)
	}
	
	for _, dbPlugin := range enabledPlugins {
		if err := m.LoadPlugin(ctx, dbPlugin.PluginID); err != nil {
			m.logger.Error("Failed to load enabled plugin", "plugin", dbPlugin.PluginID, "error", err)
			// Update status to error in database
			m.getDB().Model(&dbPlugin).Update("status", "error")
			continue
		}
	}
	
	return nil
}

// LoadPlugin loads and initializes a specific plugin
func (m *Manager) LoadPlugin(ctx context.Context, pluginID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	info, exists := m.pluginInfos[pluginID]
	if !exists {
		return fmt.Errorf("plugin not found: %s", pluginID)
	}
	
	// Check if already loaded
	if _, loaded := m.plugins[pluginID]; loaded {
		return fmt.Errorf("plugin already loaded: %s", pluginID)
	}
	
	// Create plugin instance based on type
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
	
	// Initialize plugin
	if err := plugin.Initialize(pluginCtx); err != nil {
		return fmt.Errorf("failed to initialize plugin: %w", err)
	}
	
	// Start plugin
	if err := plugin.Start(ctx); err != nil {
		return fmt.Errorf("failed to start plugin: %w", err)
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
	if err := m.getDB().Where("plugin_id = ?", pluginID).First(&dbPlugin).Error; err == nil {
		m.getDB().Model(&dbPlugin).Updates(map[string]interface{}{
			"status":     "enabled",
			"enabled_at": time.Now(),
		})
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
	if err := m.getDB().Where("plugin_id = ?", pluginID).First(&dbPlugin).Error; err == nil {
		m.getDB().Model(&dbPlugin).Update("status", "disabled")
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
	
	// Close events channel
	close(m.events)
	
	m.logger.Info("Plugin manager shutdown complete")
	return nil
}

// Helper methods

func (m *Manager) readPluginManifest(path string) (*PluginManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	
	var manifest PluginManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	
	// Validate required fields
	if manifest.ID == "" || manifest.Name == "" || manifest.Version == "" {
		return nil, fmt.Errorf("invalid manifest: missing required fields")
	}
	
	return &manifest, nil
}

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
	// Get plugin database ID
	var dbPlugin database.Plugin
	if err := m.getDB().Where("plugin_id = ?", event.PluginID).First(&dbPlugin).Error; err != nil {
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
	
	m.getDB().Create(&pluginEvent)
}

// Plugin factory methods (to be implemented based on plugin types)
func (m *Manager) createMetadataScraperPlugin(info *PluginInfo) (Plugin, error) {
	// For now, return a basic implementation
	// In a real implementation, this would load the actual plugin executable/script
	return NewBasicPlugin(info), nil
}

func (m *Manager) createAdminPagePlugin(info *PluginInfo) (Plugin, error) {
	return NewBasicPlugin(info), nil
}

func (m *Manager) createUIComponentPlugin(info *PluginInfo) (Plugin, error) {
	return NewBasicPlugin(info), nil
}

func (m *Manager) createScannerPlugin(info *PluginInfo) (Plugin, error) {
	return NewBasicPlugin(info), nil
}

func (m *Manager) createAnalyzerPlugin(info *PluginInfo) (Plugin, error) {
	return NewBasicPlugin(info), nil
}

func (m *Manager) createNotificationPlugin(info *PluginInfo) (Plugin, error) {
	return NewBasicPlugin(info), nil
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
