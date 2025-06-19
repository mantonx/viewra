package pluginmodule

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/config"
	"github.com/mantonx/viewra/internal/modules/modulemanager"
	"gorm.io/gorm"
)

// Auto-register the module when imported
func init() {
	Register()
}

const (
	ModuleID   = "system.plugins"
	ModuleName = "Plugin Manager"
)

// Register registers this module with the module system
func Register() {
	// Try to get plugin directory from configuration during registration
	cfg := config.Get()
	pluginDir := cfg.Plugins.PluginDir

	// Manual fallback: check environment variable if config is empty or default
	if pluginDir == "" || pluginDir == "./data/plugins" {
		if envPluginDir := os.Getenv("VIEWRA_PLUGIN_DIR"); envPluginDir != "" {
			pluginDir = envPluginDir
		} else if envPluginDir := os.Getenv("PLUGIN_DIR"); envPluginDir != "" {
			pluginDir = envPluginDir
		}
	}

	pluginConfig := &PluginModuleConfig{
		PluginDir: pluginDir, // Use resolved plugin directory
	}
	pluginModule := NewPluginModule(nil, pluginConfig) // DB will be set during initialization
	modulemanager.Register(pluginModule)
}

// PluginModule coordinates all plugin management functionality
// It serves as the main entry point for plugin operations and coordinates
// between core plugins, external plugins, library configurations, and media handling
type PluginModule struct {
	// Core dependencies
	db     *gorm.DB
	logger hclog.Logger
	config *PluginModuleConfig

	// Plugin sub-managers
	coreManager     *CorePluginManager
	externalManager *ExternalPluginManager
	libraryManager  *LibraryPluginManager
	mediaManager    *MediaPluginManager

	// Hot reload manager
	hotReloadManager *HotReloadManager

	// Configuration management
	configManager *PluginConfigManager

	// API handlers
	apiHandlers *PluginAPIHandlers

	// Dashboard manager
	dashboardManager *DashboardManager
}

// Module interface implementation
func (pm *PluginModule) ID() string {
	return ModuleID
}

func (pm *PluginModule) Name() string {
	return ModuleName
}

func (pm *PluginModule) Core() bool {
	return true // Plugin module is a core module
}

func (pm *PluginModule) Migrate(db *gorm.DB) error {
	pm.logger.Info("running plugin module database migrations")

	// Set the database connection if it's not already set
	if pm.db == nil {
		pm.db = db
		pm.logger.Info("Database connection set during migration")
	}

	// Database migrations for plugin-related tables would go here
	// For now, plugin tables are defined elsewhere
	return nil
}

func (pm *PluginModule) Init() error {
	pm.logger.Info("initializing plugin module")

	// Call the full initialization method with a background context
	ctx := context.Background()
	if err := pm.Initialize(ctx, pm.db); err != nil {
		return fmt.Errorf("failed to initialize plugin module: %w", err)
	}

	return nil
}

// Initialize initializes the plugin module and all sub-managers
func (pm *PluginModule) Initialize(ctx context.Context, db *gorm.DB) error {
	pm.db = db

	pm.logger.Info("initializing plugin module")

	// Ensure database connection is set for sub-managers
	if pm.db != nil {
		// Update sub-managers with database connection if they were created without it
		if pm.coreManager != nil && pm.coreManager.db == nil {
			pm.coreManager.db = pm.db
		}
		if pm.externalManager != nil && pm.externalManager.db == nil {
			pm.externalManager.db = pm.db
		}
		if pm.libraryManager != nil && pm.libraryManager.db == nil {
			pm.libraryManager.db = pm.db
		}
		if pm.mediaManager != nil && pm.mediaManager.db == nil {
			pm.mediaManager.db = pm.db
		}
	}

	// Resolve plugin directory if not set during registration
	if pm.config.PluginDir == "" {
		// Use the configuration system to get the correct plugin directory
		cfg := config.Get()
		pluginDir := cfg.Plugins.PluginDir

		// Manual fallback: check environment variable if config is empty or default
		if pluginDir == "" || pluginDir == "./data/plugins" {
			if envPluginDir := os.Getenv("VIEWRA_PLUGIN_DIR"); envPluginDir != "" {
				pluginDir = envPluginDir
				pm.logger.Info("Using VIEWRA_PLUGIN_DIR environment variable", "plugin_dir", pluginDir)
			} else if envPluginDir := os.Getenv("PLUGIN_DIR"); envPluginDir != "" {
				pluginDir = envPluginDir
				pm.logger.Info("Using PLUGIN_DIR environment variable", "plugin_dir", pluginDir)
			}
		}

		pm.config.PluginDir = pluginDir
		pm.logger.Info("Resolved plugin directory", "plugin_dir", pluginDir)
	}

	// Load core plugins from the global registry first
	if err := pm.coreManager.LoadCorePluginsFromRegistry(); err != nil {
		return fmt.Errorf("failed to load core plugins from registry: %w", err)
	}

	// Initialize core plugin manager
	if err := pm.coreManager.InitializeAllPlugins(); err != nil {
		return fmt.Errorf("failed to initialize core plugins: %w", err)
	}

	// Initialize external plugin manager
	hostServices := &HostServices{} // Placeholder for future gRPC services
	if err := pm.externalManager.Initialize(ctx, pm.config.PluginDir, hostServices); err != nil {
		return fmt.Errorf("failed to initialize external plugin manager: %w", err)
	}

	// Initialize hot reload manager if enabled in configuration
	if pm.config.EnableHotReload {
		// Convert module config to hot reload config
		hotReloadConfig := &HotReloadConfig{
			Enabled:         pm.config.HotReload.Enabled,
			DebounceDelay:   time.Duration(pm.config.HotReload.DebounceDelayMs) * time.Millisecond,
			WatchPatterns:   pm.config.HotReload.WatchPatterns,
			ExcludePatterns: pm.config.HotReload.ExcludePatterns,
			PreserveState:   pm.config.HotReload.PreserveState,
			MaxRetries:      pm.config.HotReload.MaxRetries,
			RetryDelay:      time.Duration(pm.config.HotReload.RetryDelayMs) * time.Millisecond,
		}
		var err error
		pm.hotReloadManager, err = NewHotReloadManager(pm.db, pm.logger, pm.externalManager, pm.config.PluginDir, hotReloadConfig)
		if err != nil {
			pm.logger.Error("failed to create hot reload manager", "error", err)
		} else {
			// Set up reload callbacks for logging and notifications
			pm.hotReloadManager.SetReloadCallbacks(
				func(pluginID string) {
					pm.logger.Info("🔄 Hot reload started", "plugin_id", pluginID)
				},
				func(pluginID string, oldVersion, newVersion string) {
					pm.logger.Info("✅ Hot reload completed successfully",
						"plugin_id", pluginID,
						"old_version", oldVersion,
						"new_version", newVersion)
				},
				func(pluginID string, err error) {
					pm.logger.Error("❌ Hot reload failed", "plugin_id", pluginID, "error", err)
				},
			)

			// Start hot reload monitoring
			if err := pm.hotReloadManager.Start(); err != nil {
				pm.logger.Error("failed to start hot reload manager", "error", err)
			} else {
				pm.logger.Info("✅ Hot reload system initialized and watching for plugin changes")
			}
		}
	} else {
		pm.logger.Info("Hot reload is disabled in configuration")
	}

	// Initialize library manager
	if err := pm.libraryManager.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize library plugin manager: %w", err)
	}

	// Initialize media manager
	if err := pm.mediaManager.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize media plugin manager: %w", err)
	}

	// Initialize configuration manager
	pm.configManager = NewPluginConfigManager(pm.db, pm.logger)
	pm.logger.Info("plugin configuration manager initialized")

	// Initialize API handlers
	pm.apiHandlers = NewPluginAPIHandlers(pm, pm.db, pm.logger)
	pm.logger.Info("plugin API handlers initialized")

	// Initialize dashboard manager
	pm.dashboardManager = NewDashboardManager(pm.logger)
	pm.logger.Info("dashboard manager initialized")

	// Connect external plugin manager to dashboard manager
	if pm.externalManager != nil {
		pm.externalManager.SetDashboardManager(pm.dashboardManager)
		pm.logger.Info("external plugin manager connected to dashboard manager")
	}

	pm.logger.Info("plugin module initialized successfully")
	return nil
}

// Shutdown gracefully shuts down the plugin module
func (pm *PluginModule) Shutdown(ctx context.Context) error {
	pm.logger.Info("shutting down plugin module")

	// Shutdown hot reload manager first
	if pm.hotReloadManager != nil {
		if err := pm.hotReloadManager.Stop(); err != nil {
			pm.logger.Error("failed to shutdown hot reload manager", "error", err)
		}
	}

	// Shutdown in reverse order
	if err := pm.externalManager.Shutdown(ctx); err != nil {
		pm.logger.Error("failed to shutdown external plugin manager", "error", err)
	}

	if err := pm.coreManager.ShutdownAllPlugins(); err != nil {
		pm.logger.Error("failed to shutdown core plugins", "error", err)
	}

	pm.logger.Info("plugin module shutdown complete")
	return nil
}

// Core Plugin Management

// RegisterCorePlugin registers a core plugin
func (pm *PluginModule) RegisterCorePlugin(plugin CorePlugin) error {
	return pm.coreManager.RegisterCorePlugin(plugin)
}

// GetCorePlugin returns a core plugin by name
func (pm *PluginModule) GetCorePlugin(name string) (CorePlugin, bool) {
	return pm.coreManager.GetCorePlugin(name)
}

// EnableCorePlugin enables a core plugin
func (pm *PluginModule) EnableCorePlugin(name string) error {
	return pm.coreManager.EnablePlugin(name)
}

// DisableCorePlugin disables a core plugin
func (pm *PluginModule) DisableCorePlugin(name string) error {
	return pm.coreManager.DisablePlugin(name)
}

// External Plugin Management

// LoadExternalPlugin loads an external plugin
func (pm *PluginModule) LoadExternalPlugin(ctx context.Context, pluginID string) error {
	return pm.externalManager.LoadPlugin(ctx, pluginID)
}

// UnloadExternalPlugin unloads an external plugin
func (pm *PluginModule) UnloadExternalPlugin(ctx context.Context, pluginID string) error {
	return pm.externalManager.UnloadPlugin(ctx, pluginID)
}

// GetExternalPlugin returns an external plugin by ID
func (pm *PluginModule) GetExternalPlugin(pluginID string) (*ExternalPlugin, bool) {
	return pm.externalManager.GetPlugin(pluginID)
}

// EnableExternalPlugin enables an external plugin
func (pm *PluginModule) EnableExternalPlugin(pluginID string) error {
	pm.logger.Info("enabling external plugin", "plugin_id", pluginID)

	// Enable the plugin in the external manager
	if err := pm.externalManager.EnablePlugin(pluginID); err != nil {
		return fmt.Errorf("failed to enable plugin: %w", err)
	}

	// Ensure the plugin has default configuration
	if pm.configManager != nil {
		pm.logger.Info("ensuring configuration exists for enabled plugin", "plugin_id", pluginID)
		_, err := pm.configManager.EnsureConfigurationExists(pluginID)
		if err != nil {
			pm.logger.Warn("failed to create default configuration for plugin",
				"plugin_id", pluginID,
				"error", err)
			// Don't fail the enable operation, just log the warning
		} else {
			pm.logger.Info("configuration ensured for plugin", "plugin_id", pluginID)
		}
	}

	return nil
}

// DisableExternalPlugin disables an external plugin
func (pm *PluginModule) DisableExternalPlugin(pluginID string) error {
	return pm.externalManager.DisablePlugin(pluginID)
}

// RefreshExternalPlugins re-discovers external plugins
func (pm *PluginModule) RefreshExternalPlugins() error {
	return pm.externalManager.RefreshPlugins()
}

// Plugin Information

// ListAllPlugins returns information about all plugins (core and external)
func (pm *PluginModule) ListAllPlugins() []PluginInfo {
	var allPlugins []PluginInfo

	// Add core plugins
	corePlugins := pm.coreManager.ListCorePluginInfo()
	allPlugins = append(allPlugins, corePlugins...)

	// Add external plugins
	externalPlugins := pm.externalManager.ListPlugins()
	allPlugins = append(allPlugins, externalPlugins...)

	return allPlugins
}

// GetEnabledFileHandlers returns all enabled file handlers from both core and external plugins
func (pm *PluginModule) GetEnabledFileHandlers() []FileHandlerPlugin {
	var handlers []FileHandlerPlugin

	// Add core file handlers
	coreHandlers := pm.coreManager.GetEnabledFileHandlers()
	handlers = append(handlers, coreHandlers...)

	// Add external file handlers
	externalHandlers := pm.externalManager.GetEnabledFileHandlers()
	handlers = append(handlers, externalHandlers...)

	return handlers
}

// Sub-manager access for advanced operations

// GetCoreManager returns the core plugin manager
func (pm *PluginModule) GetCoreManager() *CorePluginManager {
	return pm.coreManager
}

// GetExternalManager returns the external plugin manager
func (pm *PluginModule) GetExternalManager() *ExternalPluginManager {
	pm.logger.Info("GetExternalManager called", "manager_exists", pm.externalManager != nil)
	if pm.externalManager != nil {
		pm.logger.Info("Returning external manager", "type", fmt.Sprintf("%T", pm.externalManager))
	} else {
		pm.logger.Warn("External manager is nil!")
	}
	return pm.externalManager
}

// GetLibraryManager returns the library plugin manager
func (pm *PluginModule) GetLibraryManager() *LibraryPluginManager {
	return pm.libraryManager
}

// GetMediaManager returns the media plugin manager
func (pm *PluginModule) GetMediaManager() *MediaPluginManager {
	return pm.mediaManager
}

// GetDashboardManager returns the dashboard manager
func (pm *PluginModule) GetDashboardManager() *DashboardManager {
	return pm.dashboardManager
}

// Hot Reload Management

// GetHotReloadManager returns the hot reload manager
func (pm *PluginModule) GetHotReloadManager() *HotReloadManager {
	return pm.hotReloadManager
}

// GetConfigManager returns the plugin configuration manager
func (pm *PluginModule) GetConfigManager() *PluginConfigManager {
	return pm.configManager
}

// GetAPIHandlers returns the plugin API handlers
func (pm *PluginModule) GetAPIHandlers() *PluginAPIHandlers {
	return pm.apiHandlers
}

// TriggerPluginReload manually triggers a hot reload for a specific plugin
func (pm *PluginModule) TriggerPluginReload(pluginID string) error {
	if pm.hotReloadManager == nil {
		return fmt.Errorf("hot reload manager not available")
	}
	return pm.hotReloadManager.TriggerManualReload(pluginID)
}

// GetHotReloadStatus returns the current hot reload status
func (pm *PluginModule) GetHotReloadStatus() map[string]interface{} {
	if pm.hotReloadManager == nil {
		return map[string]interface{}{
			"enabled": false,
			"error":   "hot reload manager not available",
		}
	}
	return pm.hotReloadManager.GetReloadStatus()
}

// SetHotReloadEnabled enables or disables hot reload
func (pm *PluginModule) SetHotReloadEnabled(enabled bool) error {
	if pm.hotReloadManager == nil {
		return fmt.Errorf("hot reload manager not available")
	}
	return pm.hotReloadManager.SetEnabled(enabled)
}

// RegisterRoutes registers HTTP routes for the plugin module
func (pm *PluginModule) RegisterRoutes(router *gin.Engine) {
	pm.logger.Info("registering plugin module HTTP routes")

	// Register comprehensive API handlers (new unified system)
	if pm.apiHandlers != nil {
		pm.apiHandlers.RegisterRoutes(router)
		pm.logger.Info("registered comprehensive plugin API routes under /api/v1/plugins")
	}

	// Register dashboard API routes
	if pm.dashboardManager != nil {
		dashboardAPI := NewDashboardAPIHandlers(pm.dashboardManager)
		apiGroup := router.Group("/api/v1")
		dashboardAPI.RegisterRoutes(apiGroup)
		pm.logger.Info("registered dashboard API routes under /api/v1/dashboard")
	}

	// Register legacy plugin management routes for backward compatibility
	api := router.Group("/api/plugin-manager")
	{
		// Plugin listing and management
		api.GET("", pm.listAllPluginsHandler)
		api.GET("/core", pm.listCorePluginsHandler)
		api.GET("/external", pm.listExternalPluginsHandler)
		api.GET("/external/:plugin_id", pm.getExternalPluginHandler)

		// Plugin operations
		api.POST("/external/:plugin_id/enable", pm.enableExternalPluginHandler)
		api.POST("/external/:plugin_id/disable", pm.disableExternalPluginHandler)
		api.POST("/external/:plugin_id/load", pm.loadExternalPluginHandler)
		api.POST("/external/:plugin_id/unload", pm.unloadExternalPluginHandler)
		api.POST("/external/refresh", pm.refreshExternalPluginsHandler)

		// Core plugin operations
		api.POST("/core/:plugin_name/enable", pm.enableCorePluginHandler)
		api.POST("/core/:plugin_name/disable", pm.disableCorePluginHandler)

		// Hot reload routes
		api.GET("/hot-reload/status", pm.getHotReloadStatusHandler)
		api.POST("/hot-reload/enable", pm.enableHotReloadHandler)
		api.POST("/hot-reload/disable", pm.disableHotReloadHandler)
		api.POST("/hot-reload/trigger/:plugin_id", pm.triggerHotReloadHandler)
	}

	// Register health monitoring routes through external manager
	if pm.externalManager != nil {
		pm.externalManager.RegisterHealthRoutes(router)
		pm.logger.Info("registered external plugin health monitoring routes")
	}

	pm.logger.Info("plugin module HTTP routes registered successfully")
}

// HTTP Handlers

// listAllPluginsHandler returns all plugins (core and external)
func (pm *PluginModule) listAllPluginsHandler(c *gin.Context) {
	plugins := pm.ListAllPlugins()
	c.JSON(http.StatusOK, gin.H{
		"plugins": plugins,
		"count":   len(plugins),
	})
}

// listCorePluginsHandler returns all core plugins
func (pm *PluginModule) listCorePluginsHandler(c *gin.Context) {
	plugins := pm.coreManager.ListCorePluginInfo()
	c.JSON(http.StatusOK, gin.H{
		"plugins": plugins,
		"count":   len(plugins),
	})
}

// listExternalPluginsHandler returns all external plugins
func (pm *PluginModule) listExternalPluginsHandler(c *gin.Context) {
	plugins := pm.externalManager.ListPlugins()
	c.JSON(http.StatusOK, gin.H{
		"plugins": plugins,
		"count":   len(plugins),
	})
}

// getExternalPluginHandler returns a specific external plugin
func (pm *PluginModule) getExternalPluginHandler(c *gin.Context) {
	pluginID := c.Param("plugin_id")
	if pluginID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Plugin ID is required"})
		return
	}

	plugin, exists := pm.externalManager.GetPlugin(pluginID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plugin not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"plugin": plugin})
}

// enableExternalPluginHandler enables an external plugin
func (pm *PluginModule) enableExternalPluginHandler(c *gin.Context) {
	pluginID := c.Param("plugin_id")
	if pluginID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Plugin ID is required"})
		return
	}

	if err := pm.EnableExternalPlugin(pluginID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Plugin enabled successfully", "plugin_id": pluginID})
}

// disableExternalPluginHandler disables an external plugin
func (pm *PluginModule) disableExternalPluginHandler(c *gin.Context) {
	pluginID := c.Param("plugin_id")
	if pluginID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Plugin ID is required"})
		return
	}

	if err := pm.DisableExternalPlugin(pluginID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Plugin disabled successfully", "plugin_id": pluginID})
}

// loadExternalPluginHandler loads an external plugin
func (pm *PluginModule) loadExternalPluginHandler(c *gin.Context) {
	pluginID := c.Param("plugin_id")
	if pluginID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Plugin ID is required"})
		return
	}

	ctx := c.Request.Context()
	if err := pm.LoadExternalPlugin(ctx, pluginID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Plugin loaded successfully", "plugin_id": pluginID})
}

// unloadExternalPluginHandler unloads an external plugin
func (pm *PluginModule) unloadExternalPluginHandler(c *gin.Context) {
	pluginID := c.Param("plugin_id")
	if pluginID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Plugin ID is required"})
		return
	}

	ctx := c.Request.Context()
	if err := pm.UnloadExternalPlugin(ctx, pluginID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Plugin unloaded successfully", "plugin_id": pluginID})
}

// refreshExternalPluginsHandler re-discovers external plugins
func (pm *PluginModule) refreshExternalPluginsHandler(c *gin.Context) {
	if err := pm.RefreshExternalPlugins(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "External plugins refreshed successfully"})
}

// enableCorePluginHandler enables a core plugin
func (pm *PluginModule) enableCorePluginHandler(c *gin.Context) {
	pluginName := c.Param("plugin_name")
	if pluginName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Plugin name is required"})
		return
	}

	if err := pm.EnableCorePlugin(pluginName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Core plugin enabled successfully", "plugin_name": pluginName})
}

// disableCorePluginHandler disables a core plugin
func (pm *PluginModule) disableCorePluginHandler(c *gin.Context) {
	pluginName := c.Param("plugin_name")
	if pluginName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Plugin name is required"})
		return
	}

	if err := pm.DisableCorePlugin(pluginName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Core plugin disabled successfully", "plugin_name": pluginName})
}

// Hot Reload Handlers

// getHotReloadStatusHandler returns the current hot reload status
func (pm *PluginModule) getHotReloadStatusHandler(c *gin.Context) {
	status := pm.GetHotReloadStatus()
	c.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"hot_reload": status,
	})
}

// enableHotReloadHandler enables hot reload functionality
func (pm *PluginModule) enableHotReloadHandler(c *gin.Context) {
	if err := pm.SetHotReloadEnabled(true); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Hot reload enabled successfully",
	})
}

// disableHotReloadHandler disables hot reload functionality
func (pm *PluginModule) disableHotReloadHandler(c *gin.Context) {
	if err := pm.SetHotReloadEnabled(false); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Hot reload disabled successfully",
	})
}

// triggerHotReloadHandler manually triggers a hot reload for a specific plugin
func (pm *PluginModule) triggerHotReloadHandler(c *gin.Context) {
	pluginID := c.Param("plugin_id")
	if pluginID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "plugin_id parameter is required",
		})
		return
	}

	if err := pm.TriggerPluginReload(pluginID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "success",
		"message":   fmt.Sprintf("Hot reload triggered for plugin: %s", pluginID),
		"plugin_id": pluginID,
	})
}

// NewPluginModule creates a new plugin module instance
func NewPluginModule(db *gorm.DB, config *PluginModuleConfig) *PluginModule {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "plugin-module",
		Level: hclog.Debug,
	})

	// Set default hot reload configuration if not provided
	if config.HotReload.DebounceDelayMs == 0 {
		config.HotReload.DebounceDelayMs = 500
	}
	if len(config.HotReload.WatchPatterns) == 0 {
		config.HotReload.WatchPatterns = []string{"*_transcoder", "*_enricher", "*_scanner"}
	}
	if len(config.HotReload.ExcludePatterns) == 0 {
		config.HotReload.ExcludePatterns = []string{"*.tmp", "*.log", "*.pid", ".git*", "*.swp", "*.swo", "go.mod", "go.sum", "*.go", "plugin.cue", "*.json"}
	}
	if config.HotReload.MaxRetries == 0 {
		config.HotReload.MaxRetries = 3
	}
	if config.HotReload.RetryDelayMs == 0 {
		config.HotReload.RetryDelayMs = 1000
	}

	logger.Info("Creating new plugin module")
	logger.Info("Creating external plugin manager")
	externalManager := NewExternalPluginManager(db, logger)
	logger.Info("External plugin manager created", "manager", externalManager != nil)

	// Initialize config manager
	configManager := NewPluginConfigManager(db, logger)

	pm := &PluginModule{
		db:               db,
		config:           config,
		logger:           logger,
		coreManager:      NewCorePluginManager(db),
		externalManager:  externalManager,
		libraryManager:   NewLibraryPluginManager(db),
		mediaManager:     NewMediaPluginManager(db, logger),
		configManager:    configManager,
		dashboardManager: NewDashboardManager(logger),
	}

	// Initialize API handlers with all dependencies
	pm.apiHandlers = NewPluginAPIHandlers(pm, db, logger)

	logger.Info("Plugin module created", "external_manager_stored", pm.externalManager != nil, "api_handlers_initialized", pm.apiHandlers != nil)
	return pm
}
