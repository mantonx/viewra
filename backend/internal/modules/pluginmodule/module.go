package pluginmodule

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-hclog"
	"gorm.io/gorm"
)

// PluginModule coordinates all plugin management functionality
// It serves as the main entry point for plugin operations and coordinates
// between core plugins, external plugins, library configurations, and media handling
type PluginModule struct {
	db *gorm.DB

	// Sub-managers
	coreManager     *CorePluginManager
	externalManager *ExternalPluginManager
	libraryManager  *LibraryPluginManager
	mediaManager    *MediaPluginManager

	// Configuration
	config *PluginModuleConfig
	logger hclog.Logger
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
		mediaManager:    NewMediaPluginManager(db),
	}
	
	logger.Info("Plugin module created", "external_manager_stored", pm.externalManager != nil)
	return pm
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
