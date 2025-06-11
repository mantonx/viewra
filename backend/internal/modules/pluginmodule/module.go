package pluginmodule

import (
	"context"
	"fmt"
	"os"

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
	// Create a temporary plugin module for registration - it will be properly initialized later
	// Use the configuration system to get the correct plugin directory
	cfg := config.Get()
	pluginDir := cfg.Plugins.PluginDir

	// Manual fallback: check environment variable if config is empty or default
	if pluginDir == "" || pluginDir == "./data/plugins" {
		if envPluginDir := os.Getenv("PLUGIN_DIR"); envPluginDir != "" {
			pluginDir = envPluginDir
			fmt.Printf("Using PLUGIN_DIR environment variable: %s\n", pluginDir)
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
	// Database migrations for plugin-related tables would go here
	// For now, plugin tables are defined elsewhere
	return nil
}

func (pm *PluginModule) Init() error {
	pm.logger.Info("initializing plugin module")
	// The actual initialization happens in Initialize() method
	return nil
}

// Initialize initializes the plugin module and all sub-managers
func (pm *PluginModule) Initialize(ctx context.Context) error {
	pm.logger.Info("initializing plugin module")

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

	// Initialize library manager
	if err := pm.libraryManager.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize library plugin manager: %w", err)
	}

	// Initialize media manager
	if err := pm.mediaManager.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize media plugin manager: %w", err)
	}

	pm.logger.Info("plugin module initialized successfully")
	return nil
}

// Shutdown gracefully shuts down the plugin module
func (pm *PluginModule) Shutdown(ctx context.Context) error {
	pm.logger.Info("shutting down plugin module")

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
	return pm.externalManager.EnablePlugin(pluginID)
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

// RegisterRoutes registers HTTP routes for the plugin module
func (pm *PluginModule) RegisterRoutes(router *gin.Engine) {
	pm.logger.Info("registering plugin module HTTP routes")

	// Register plugin management routes
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
	c.JSON(StatusOK, gin.H{
		"plugins": plugins,
		"count":   len(plugins),
	})
}

// listCorePluginsHandler returns all core plugins
func (pm *PluginModule) listCorePluginsHandler(c *gin.Context) {
	plugins := pm.coreManager.ListCorePluginInfo()
	c.JSON(StatusOK, gin.H{
		"plugins": plugins,
		"count":   len(plugins),
	})
}

// listExternalPluginsHandler returns all external plugins
func (pm *PluginModule) listExternalPluginsHandler(c *gin.Context) {
	plugins := pm.externalManager.ListPlugins()
	c.JSON(StatusOK, gin.H{
		"plugins": plugins,
		"count":   len(plugins),
	})
}

// getExternalPluginHandler returns a specific external plugin
func (pm *PluginModule) getExternalPluginHandler(c *gin.Context) {
	pluginID := c.Param("plugin_id")
	if pluginID == "" {
		c.JSON(StatusBadRequest, gin.H{"error": ErrPluginIDRequired})
		return
	}

	plugin, exists := pm.externalManager.GetPlugin(pluginID)
	if !exists {
		c.JSON(StatusNotFound, gin.H{"error": ErrPluginNotFound})
		return
	}

	c.JSON(StatusOK, gin.H{"plugin": plugin})
}

// enableExternalPluginHandler enables an external plugin
func (pm *PluginModule) enableExternalPluginHandler(c *gin.Context) {
	pluginID := c.Param("plugin_id")
	if pluginID == "" {
		c.JSON(StatusBadRequest, gin.H{"error": ErrPluginIDRequired})
		return
	}

	if err := pm.EnableExternalPlugin(pluginID); err != nil {
		c.JSON(StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(StatusOK, gin.H{"message": "Plugin enabled successfully", "plugin_id": pluginID})
}

// disableExternalPluginHandler disables an external plugin
func (pm *PluginModule) disableExternalPluginHandler(c *gin.Context) {
	pluginID := c.Param("plugin_id")
	if pluginID == "" {
		c.JSON(400, gin.H{"error": "Plugin ID is required"})
		return
	}

	if err := pm.DisableExternalPlugin(pluginID); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Plugin disabled successfully", "plugin_id": pluginID})
}

// loadExternalPluginHandler loads an external plugin
func (pm *PluginModule) loadExternalPluginHandler(c *gin.Context) {
	pluginID := c.Param("plugin_id")
	if pluginID == "" {
		c.JSON(400, gin.H{"error": "Plugin ID is required"})
		return
	}

	ctx := c.Request.Context()
	if err := pm.LoadExternalPlugin(ctx, pluginID); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Plugin loaded successfully", "plugin_id": pluginID})
}

// unloadExternalPluginHandler unloads an external plugin
func (pm *PluginModule) unloadExternalPluginHandler(c *gin.Context) {
	pluginID := c.Param("plugin_id")
	if pluginID == "" {
		c.JSON(400, gin.H{"error": "Plugin ID is required"})
		return
	}

	ctx := c.Request.Context()
	if err := pm.UnloadExternalPlugin(ctx, pluginID); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Plugin unloaded successfully", "plugin_id": pluginID})
}

// refreshExternalPluginsHandler re-discovers external plugins
func (pm *PluginModule) refreshExternalPluginsHandler(c *gin.Context) {
	if err := pm.RefreshExternalPlugins(); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "External plugins refreshed successfully"})
}

// enableCorePluginHandler enables a core plugin
func (pm *PluginModule) enableCorePluginHandler(c *gin.Context) {
	pluginName := c.Param("plugin_name")
	if pluginName == "" {
		c.JSON(400, gin.H{"error": "Plugin name is required"})
		return
	}

	if err := pm.EnableCorePlugin(pluginName); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Core plugin enabled successfully", "plugin_name": pluginName})
}

// disableCorePluginHandler disables a core plugin
func (pm *PluginModule) disableCorePluginHandler(c *gin.Context) {
	pluginName := c.Param("plugin_name")
	if pluginName == "" {
		c.JSON(400, gin.H{"error": "Plugin name is required"})
		return
	}

	if err := pm.DisableCorePlugin(pluginName); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Core plugin disabled successfully", "plugin_name": pluginName})
}

// NewPluginModule creates a new plugin module instance
func NewPluginModule(db *gorm.DB, config *PluginModuleConfig) *PluginModule {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "plugin-module",
		Level: hclog.Debug,
	})

	logger.Info("Creating new plugin module")
	logger.Info("Creating external plugin manager")
	externalManager := NewExternalPluginManager(db, logger)
	logger.Info("External plugin manager created", "manager", externalManager != nil)

	pm := &PluginModule{
		db:              db,
		config:          config,
		logger:          logger,
		coreManager:     NewCorePluginManager(db),
		externalManager: externalManager,
		libraryManager:  NewLibraryPluginManager(db),
		mediaManager:    NewMediaPluginManager(db, logger),
	}

	logger.Info("Plugin module created", "external_manager_stored", pm.externalManager != nil)
	return pm
}
