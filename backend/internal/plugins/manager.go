package plugins

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
	"github.com/mantonx/viewra/internal/apiroutes"
	"github.com/mantonx/viewra/internal/config"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/modules/assetmodule"
	"github.com/mantonx/viewra/internal/plugins/proto"
	"google.golang.org/grpc"
	"gorm.io/gorm"
)

// manager implements the Manager interface
type manager struct {
	// Configuration
	pluginDir string
	db        *gorm.DB
	logger    hclog.Logger
	
	// State management
	mu      sync.RWMutex
	plugins map[string]*Plugin
	registry Registry
	
	// Hot reload
	watcher *fsnotify.Watcher
	ctx     context.Context
	cancel  context.CancelFunc
	
	// Host services for plugins
	hostGRPCServer   *grpc.Server
	hostGRPCListener net.Listener
	hostAssetService *HostAssetService
	hostServiceAddr  string
	corePluginManager *CorePluginManager
}

// NewManager creates a new plugin manager
func NewManager(pluginDir string, db *gorm.DB, logger hclog.Logger) Manager {
	ctx, cancel := context.WithCancel(context.Background())
	
	m := &manager{
		pluginDir: pluginDir,
		db:        db,
		logger:    logger.Named("plugin-manager"),
		plugins:   make(map[string]*Plugin),
		registry:  Registry{},
		ctx:       ctx,
		cancel:    cancel,
		corePluginManager: NewCorePluginManager(db),
	}
	
	return m
}

// Initialize starts the plugin manager
func (m *manager) Initialize(ctx context.Context) error {
	m.logger.Info("initializing plugin manager", "plugin_dir", m.pluginDir)
	
	// Create plugin directory if it doesn't exist
	if err := os.MkdirAll(m.pluginDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}
	
	// Initialize file watcher for hot reload
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	m.watcher = watcher
	
	// Start file watching
	if err := m.startFileWatcher(); err != nil {
		return fmt.Errorf("failed to start file watcher: %w", err)
	}
	
	// Discover plugins
	if err := m.DiscoverPlugins(); err != nil {
		return fmt.Errorf("failed to discover plugins: %w", err)
	}
	
	// Start host services for bidirectional communication BEFORE loading plugins
	if err := m.startHostServices(); err != nil {
		return fmt.Errorf("failed to start host services: %w", err)
	}
	
	// Load enabled plugins (now with host service address available)
	if err := m.loadEnabledPlugins(ctx); err != nil {
		return fmt.Errorf("failed to load enabled plugins: %w", err)
	}
	
	// Initialize core plugins
	if m.corePluginManager != nil {
		if err := m.corePluginManager.InitializeAllPlugins(); err != nil {
			m.logger.Error("Failed to initialize core plugins", "error", err)
		}
	}
	
	m.logger.Info("plugin manager initialized", "plugins_discovered", len(m.plugins), "host_service_addr", m.hostServiceAddr)
	return nil
}

// Shutdown gracefully stops all plugins and cleans up resources
func (m *manager) Shutdown(ctx context.Context) error {
	m.logger.Info("shutting down plugin manager")
	
	// Stop host services first
	m.stopHostServices()
	
	// Stop file watcher
	if m.watcher != nil {
		m.watcher.Close()
	}
	
	// Unload all plugins
	m.mu.RLock()
	pluginIDs := make([]string, 0, len(m.plugins))
	for id := range m.plugins {
		pluginIDs = append(pluginIDs, id)
	}
	m.mu.RUnlock()
	
	for _, id := range pluginIDs {
		if err := m.UnloadPlugin(ctx, id); err != nil {
			m.logger.Error("failed to unload plugin during shutdown", "plugin", id, "error", err)
		}
	}
	
	// Cancel context
	m.cancel()
	
	// Shutdown core plugins
	if m.corePluginManager != nil {
		if err := m.corePluginManager.ShutdownAllPlugins(); err != nil {
			m.logger.Error("Failed to shutdown core plugins", "error", err)
		}
	}
	
	m.logger.Info("plugin manager shutdown complete")
	return nil
}

// DiscoverPlugins scans the plugin directory for plugin configurations
func (m *manager) DiscoverPlugins() error {
	m.logger.Debug("discovering plugins", "dir", m.pluginDir)
	
	return filepath.Walk(m.pluginDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Look for plugin.cue files
		if info.Name() == "plugin.cue" {
			if err := m.loadPluginConfig(path); err != nil {
				m.logger.Error("failed to load plugin config", "path", path, "error", err)
				return nil // Continue with other plugins
			}
		}
		
		return nil
	})
}

// loadPluginConfig loads a plugin configuration from a CueLang file
func (m *manager) loadPluginConfig(configPath string) error {
	// Read the CueLang file
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}
	
	// Parse with CueLang
	cueCtx := cuecontext.New()
	value := cueCtx.CompileBytes(configData)
	if err := value.Err(); err != nil {
		return fmt.Errorf("failed to parse CueLang config: %w", err)
	}
	
	// CueLang uses definitions, so we need to look up the #Plugin value
	pluginValue := value.LookupPath(cue.ParsePath("#Plugin"))
	if !pluginValue.Exists() {
		// Fallback to direct decoding if no #Plugin definition
		pluginValue = value
	}

	// Decode into Config struct
	var config Config
	if err := pluginValue.Decode(&config); err != nil {
		return fmt.Errorf("failed to decode config: %w", err)
	}
	
	// Workaround for CUE decoding issue with nested EntryPoints.Main
	if config.EntryPoints.Main == "" { // If Decode didn't populate it
		epMainValue := pluginValue.LookupPath(cue.ParsePath("entry_points.main"))
		if epMainValue.Exists() {
			epMainStr, strErr := epMainValue.String()
			if strErr == nil && epMainStr != "" {
				config.EntryPoints.Main = epMainStr
				m.logger.Debug("Populated EntryPoints.Main via LookupPath workaround", "plugin_id", config.ID, "value", epMainStr)
			} else if strErr != nil {
				m.logger.Warn("EntryPoints.Main found by LookupPath but not a string", "plugin_id", config.ID, "error", strErr)
			}
		}
	}
	
	// Workaround for CUE decoding issue with enabled_by_default boolean
	enabledByDefaultValue := pluginValue.LookupPath(cue.ParsePath("enabled_by_default"))
	if enabledByDefaultValue.Exists() {
		if enabledBool, err := enabledByDefaultValue.Bool(); err == nil {
			config.EnabledByDefault = enabledBool
			m.logger.Debug("Populated EnabledByDefault via LookupPath workaround", "plugin_id", config.ID, "value", enabledBool)
		} else {
			m.logger.Warn("enabled_by_default found by LookupPath but not a boolean", "plugin_id", config.ID, "error", err)
		}
	}
	
	// Validate required fields
	if config.ID == "" || config.Name == "" || config.Version == "" {
		return fmt.Errorf("missing required fields: id, name, or version")
	}
	
	// Validate entry point
	if config.EntryPoints.Main == "" {
		return fmt.Errorf("plugin %s config is missing entry_points.main or it decoded as empty", config.ID)
	}
	
	pluginDir := filepath.Dir(configPath)
	binaryPath := filepath.Join(pluginDir, config.EntryPoints.Main)

	// Check for binary more strictly
	// Use os.Stat and check if err is nil. Any error means we can't use the binary.
	if _, err := os.Stat(binaryPath); err != nil {
		return fmt.Errorf("plugin binary for %s not found or inaccessible at %s: %w", config.ID, binaryPath, err)
	}
	
	// Create plugin instance
	plugin := &Plugin{
		ID:          config.ID,
		Name:        config.Name,
		Version:     config.Version,
		Type:        config.Type,
		Description: config.Description,
		Author:      config.Author,
		BinaryPath:  binaryPath,
		ConfigPath:  configPath,
		BasePath:    pluginDir,
		Running:     false,
	}
	
	// Store plugin
	m.mu.Lock()
	m.plugins[config.ID] = plugin
	m.mu.Unlock()
	
	// Register plugin in database if not already registered
	m.registerPluginInDatabase(plugin, config.EnabledByDefault)
	
	m.logger.Info("discovered plugin", 
		"id", config.ID,
		"name", config.Name,
		"version", config.Version,
		"type", config.Type,
		"enabled_by_default", config.EnabledByDefault)
	
	return nil
}

// LoadPlugin loads and starts a plugin
func (m *manager) LoadPlugin(ctx context.Context, pluginID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	plugin, exists := m.plugins[pluginID]
	if !exists {
		return fmt.Errorf("plugin not found: %s", pluginID)
	}
	
	if plugin.Running {
		m.logger.Debug("plugin already running", "plugin", pluginID)
		return nil
	}
	
	m.logger.Info("loading plugin", "plugin", pluginID, "binary", plugin.BinaryPath)
	
	// Create plugin client
	pluginMap := map[string]goplugin.Plugin{
		"plugin": &GRPCPlugin{},
	}
	
	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig:  Handshake,
		Plugins:          pluginMap,
		Cmd:              exec.Command(plugin.BinaryPath),
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
		Logger:           m.logger.Named(pluginID),
	})
	
	// Start the plugin
	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return fmt.Errorf("failed to start plugin client: %w", err)
	}
	
	// Dispense the plugin
	rawPlugin, err := rpcClient.Dispense("plugin")
	if err != nil {
		client.Kill()
		return fmt.Errorf("failed to dispense plugin: %w", err)
	}
	
	// Type assert to gRPC client
	grpcClient, ok := rawPlugin.(*GRPCClient)
	if !ok {
		client.Kill()
		return fmt.Errorf("plugin does not implement gRPC interface")
	}
	
	// Store client references
	plugin.Client = client
	plugin.GRPCClient = grpcClient
	plugin.PluginService = grpcClient.PluginServiceClient
	
	// Initialize optional service clients
	m.initializeServiceClients(plugin, grpcClient)
	
	// Initialize the plugin
	m.logger.Info("Initializing plugin", "plugin_id", pluginID)
	dbURL := m.getDatabaseURL()
	_, err = plugin.GRPCClient.Initialize(ctx, &proto.InitializeRequest{
		Context: &proto.PluginContext{
			DatabaseUrl:     dbURL,
			PluginId:        pluginID,
			BasePath:        plugin.BasePath,
			HostServiceAddr: m.hostServiceAddr,
		},
	})
	if err != nil {
		m.logger.Error("Failed to initialize plugin", "plugin_id", pluginID, "error", err)
		plugin.Client.Kill() // Ensure process is killed on init failure
		plugin.Running = false
		return fmt.Errorf("failed to initialize plugin %s: %w", pluginID, err)
	}
	m.logger.Info("Plugin initialized successfully", "plugin_id", pluginID)

	plugin.Running = true

	// Check for APIRegistrationService and register routes
	if plugin.GRPCClient.APIRegistrationServiceClient != nil {
		m.logger.Info("Plugin implements APIRegistrationService, fetching routes...", "plugin_id", pluginID)
		resp, err := plugin.GRPCClient.APIRegistrationServiceClient.GetRegisteredRoutes(context.Background(), &proto.GetRegisteredRoutesRequest{})
		if err != nil {
			m.logger.Error("Failed to get registered routes from plugin", "plugin_id", pluginID, "error", err)
		} else {
			m.logger.Info("Received routes from plugin", "plugin_id", pluginID, "count", len(resp.GetRoutes()))
			for _, route := range resp.GetRoutes() {
				fullPath := path.Join("/api/plugins/", pluginID, route.Path)
				m.logger.Info("Registering route from plugin", "plugin_id", pluginID, "path", fullPath, "method", route.Method, "description", route.Description)
				apiroutes.Register(fullPath, route.Method, route.Description)
			}
		}
	} else {
		m.logger.Info("Plugin does not implement APIRegistrationService or client is nil", "plugin_id", pluginID)
	}

	// Register plugin in service registries for all services it implements
	m.registerPlugin(plugin)

	m.plugins[pluginID] = plugin
	
	m.logger.Info("plugin loaded successfully", "plugin", pluginID)
	return nil
}

// UnloadPlugin stops and unloads a plugin
func (m *manager) UnloadPlugin(ctx context.Context, pluginID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	plugin, exists := m.plugins[pluginID]
	if !exists {
		return fmt.Errorf("plugin not found: %s", pluginID)
	}
	
	if !plugin.Running {
		return nil
	}
	
	m.logger.Info("unloading plugin", "plugin", pluginID)
	
	// Stop the plugin gracefully
	if plugin.PluginService != nil {
		_, err := plugin.PluginService.Stop(ctx, &proto.StopRequest{})
		if err != nil {
			m.logger.Error("failed to stop plugin gracefully", "plugin", pluginID, "error", err)
		}
	}
	
	// Kill the plugin process
	if plugin.Client != nil {
		plugin.Client.Kill()
	}
	
	// Update state
	plugin.Running = false
	plugin.LastStopped = time.Now()
	plugin.Client = nil
	plugin.GRPCClient = nil
	plugin.PluginService = nil
	plugin.MetadataScraperService = nil
	plugin.ScannerHookService = nil
	plugin.DatabaseService = nil
	plugin.AdminPageService = nil
	plugin.APIRegistrationService = nil
	plugin.SearchService = nil
	
	// Unregister from type-specific registries
	m.unregisterPlugin(plugin)
	
	m.logger.Info("plugin unloaded", "plugin", pluginID)
	return nil
}

// RestartPlugin restarts a plugin
func (m *manager) RestartPlugin(ctx context.Context, pluginID string) error {
	m.logger.Info("restarting plugin", "plugin", pluginID)
	
	if err := m.UnloadPlugin(ctx, pluginID); err != nil {
		m.logger.Error("failed to unload plugin for restart", "plugin", pluginID, "error", err)
	}
	
	// Wait for cleanup
	time.Sleep(100 * time.Millisecond)
	
	return m.LoadPlugin(ctx, pluginID)
}

// ListPlugins returns all discovered plugins
func (m *manager) ListPlugins() []PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var infos []PluginInfo
	for _, plugin := range m.plugins {
		info := PluginInfo{
			Name:        plugin.Name,
			Type:        plugin.Type,
			Version:     plugin.Version,
			Description: plugin.Description,
			Enabled:     plugin.Running,
			IsCore:      false, // External plugins are never core
		}
		infos = append(infos, info)
	}
	return infos
}

// GetPlugin returns a specific plugin
func (m *manager) GetPlugin(pluginID string) (*Plugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	plugin, exists := m.plugins[pluginID]
	return plugin, exists
}

// GetMetadataScrapers returns all registered metadata scraper service clients
func (m *manager) GetMetadataScrapers() []proto.MetadataScraperServiceClient {
	m.mu.RLock()
	defer m.mu.RUnlock()
	clients := make([]proto.MetadataScraperServiceClient, 0, len(m.registry.MetadataScrapers))
	for _, pluginID := range m.registry.MetadataScrapers {
		if plugin, exists := m.plugins[pluginID]; exists && plugin.Running && plugin.MetadataScraperService != nil {
			clients = append(clients, plugin.MetadataScraperService)
		}
	}
	return clients // Ensures a non-nil slice is returned
}

// GetScannerHooks returns all registered scanner hook service clients
func (m *manager) GetScannerHooks() []proto.ScannerHookServiceClient {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Debug logging
	m.logger.Debug("GetScannerHooks called", "registry_count", len(m.registry.ScannerHooks), "plugins_count", len(m.plugins))
	for _, pluginID := range m.registry.ScannerHooks {
		m.logger.Debug("Checking scanner hook plugin", "plugin_id", pluginID)
		if plugin, exists := m.plugins[pluginID]; exists {
			m.logger.Debug("Plugin found", "plugin_id", pluginID, "running", plugin.Running, "has_scanner_service", plugin.ScannerHookService != nil)
		} else {
			m.logger.Debug("Plugin not found in plugins map", "plugin_id", pluginID)
		}
	}
	
	clients := make([]proto.ScannerHookServiceClient, 0, len(m.registry.ScannerHooks))
	for _, pluginID := range m.registry.ScannerHooks {
		if plugin, exists := m.plugins[pluginID]; exists && plugin.Running && plugin.ScannerHookService != nil {
			clients = append(clients, plugin.ScannerHookService)
		}
	}
	
	m.logger.Debug("GetScannerHooks returning", "client_count", len(clients))
	return clients // Ensures a non-nil slice is returned
}

// GetDatabases returns all registered database service clients
func (m *manager) GetDatabases() []proto.DatabaseServiceClient {
	m.mu.RLock()
	defer m.mu.RUnlock()
	clients := make([]proto.DatabaseServiceClient, 0, len(m.registry.Databases))
	for _, pluginID := range m.registry.Databases {
		if plugin, exists := m.plugins[pluginID]; exists && plugin.Running && plugin.DatabaseService != nil {
			clients = append(clients, plugin.DatabaseService)
		}
	}
	return clients // Ensures a non-nil slice is returned
}

// GetAdminPages returns all registered admin page service clients
func (m *manager) GetAdminPages() []proto.AdminPageServiceClient {
	m.mu.RLock()
	defer m.mu.RUnlock()
	clients := make([]proto.AdminPageServiceClient, 0, len(m.registry.AdminPages))
	for _, pluginID := range m.registry.AdminPages {
		if plugin, exists := m.plugins[pluginID]; exists && plugin.Running && plugin.AdminPageService != nil {
			clients = append(clients, plugin.AdminPageService)
		}
	}
	return clients // Ensures a non-nil slice is returned
}

// Helper methods

func (m *manager) loadEnabledPlugins(ctx context.Context) error {
	var enabledPlugins []struct {
		PluginID    string
		Status      string
		Type        string
		InstallPath string
	}
	
	if err := m.db.Table("plugins").
		Select("plugin_id, status, type, install_path").
		Where("status = ?", "enabled").
		Find(&enabledPlugins).Error; err != nil {
		return fmt.Errorf("failed to query enabled plugins: %w", err)
	}

	// Separate core and external plugins
	var externalPlugins []string
	var corePlugins []string
	
	for _, dbPlugin := range enabledPlugins {
		if dbPlugin.Type == "core" || dbPlugin.InstallPath == "core" {
			corePlugins = append(corePlugins, dbPlugin.PluginID)
		} else {
			externalPlugins = append(externalPlugins, dbPlugin.PluginID)
		}
	}
	
	// Initialize core plugins via core plugin manager
	if len(corePlugins) > 0 {
		m.logger.Info("initializing enabled core plugins", "count", len(corePlugins), "plugins", corePlugins)
		if m.corePluginManager != nil {
			if err := m.corePluginManager.InitializeAllPlugins(); err != nil {
				m.logger.Error("failed to initialize core plugins", "error", err)
			}
		}
	}
	
	// Load external plugins normally
	for _, pluginID := range externalPlugins {
		if err := m.LoadPlugin(ctx, pluginID); err != nil {
			m.logger.Error("failed to load enabled external plugin", "plugin", pluginID, "error", err)
		}
	}
	
	return nil
}

func (m *manager) initializeServiceClients(plugin *Plugin, grpcClient *GRPCClient) {
	plugin.MetadataScraperService = grpcClient.MetadataScraperServiceClient
	plugin.ScannerHookService = grpcClient.ScannerHookServiceClient
	plugin.DatabaseService = grpcClient.DatabaseServiceClient
	plugin.AdminPageService = grpcClient.AdminPageServiceClient
	plugin.APIRegistrationService = grpcClient.APIRegistrationServiceClient
	plugin.SearchService = grpcClient.SearchServiceClient
}

func (m *manager) registerPlugin(plugin *Plugin) {
	// Register plugin for all services it implements, not just its primary type
	m.logger.Debug("Registering plugin services", "plugin_id", plugin.ID)
	
	if plugin.MetadataScraperService != nil {
		m.registry.MetadataScrapers = append(m.registry.MetadataScrapers, plugin.ID)
		m.logger.Debug("Registered plugin as metadata scraper", "plugin_id", plugin.ID)
	}
	if plugin.ScannerHookService != nil {
		m.registry.ScannerHooks = append(m.registry.ScannerHooks, plugin.ID)
		m.logger.Debug("Registered plugin as scanner hook", "plugin_id", plugin.ID)
	}
	if plugin.DatabaseService != nil {
		m.registry.Databases = append(m.registry.Databases, plugin.ID)
		m.logger.Debug("Registered plugin as database service", "plugin_id", plugin.ID)
	}
	if plugin.AdminPageService != nil {
		m.registry.AdminPages = append(m.registry.AdminPages, plugin.ID)
		m.logger.Debug("Registered plugin as admin page", "plugin_id", plugin.ID)
	}
	
	m.logger.Debug("Plugin registration complete", "plugin_id", plugin.ID, 
		"scanner_hooks_count", len(m.registry.ScannerHooks),
		"metadata_scrapers_count", len(m.registry.MetadataScrapers))
}

func (m *manager) unregisterPlugin(plugin *Plugin) {
	m.registry.MetadataScrapers = removeFromSlice(m.registry.MetadataScrapers, plugin.ID)
	m.registry.ScannerHooks = removeFromSlice(m.registry.ScannerHooks, plugin.ID)
	m.registry.Databases = removeFromSlice(m.registry.Databases, plugin.ID)
	m.registry.AdminPages = removeFromSlice(m.registry.AdminPages, plugin.ID)
}

func (m *manager) startFileWatcher() error {
	if err := m.watcher.Add(m.pluginDir); err != nil {
		return fmt.Errorf("failed to watch plugin directory: %w", err)
	}
	
	go m.watchFiles()
	return nil
}

func (m *manager) watchFiles() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case event, ok := <-m.watcher.Events:
			if !ok {
				return
			}
			
			if event.Op&fsnotify.Write == fsnotify.Write {
				m.handleFileUpdate(event.Name)
			}
		case err, ok := <-m.watcher.Errors:
			if !ok {
				return
			}
			m.logger.Error("file watcher error", "error", err)
		}
	}
}

func (m *manager) handleFileUpdate(filePath string) {
	if filepath.Base(filePath) == "plugin.cue" {
		m.logger.Info("plugin config updated, rediscovering", "file", filePath)
		if err := m.DiscoverPlugins(); err != nil {
			m.logger.Error("failed to rediscover plugins", "error", err)
		}
	}
}

func (m *manager) getDatabaseURL() string {
	// Use centralized database configuration directly
	cfg := config.Get().Database
	
	// Generate URL if not explicitly set
	if cfg.URL != "" {
		return cfg.URL
	}
	
	switch cfg.Type {
	case "sqlite":
		return "sqlite://" + cfg.DatabasePath
	case "postgres":
		url := "postgres://"
		if cfg.Username != "" {
			url += cfg.Username
			if cfg.Password != "" {
				url += ":" + cfg.Password
			}
			url += "@"
		}
		url += cfg.Host
		if cfg.Port != 5432 {
			url += fmt.Sprintf(":%d", cfg.Port)
		}
		url += "/" + cfg.Database
		return url
	default:
		return "sqlite://" + cfg.DatabasePath
	}
}

func removeFromSlice(slice []string, item string) []string {
	for i, v := range slice {
		if v == item {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

func (m *manager) registerPluginInDatabase(plugin *Plugin, enabledByDefault bool) {
	if m.db == nil {
		m.logger.Warn("database not available, skipping plugin registration", "plugin", plugin.ID)
		return
	}

	// Import the database models package
	// We need to check if plugin already exists in database
	var existingPlugin struct {
		ID       uint32 `gorm:"primaryKey"`
		PluginID string `gorm:"uniqueIndex;not null"`
		Status   string `gorm:"not null;default:'disabled'"`
	}
	
	err := m.db.Table("plugins").Where("plugin_id = ?", plugin.ID).First(&existingPlugin).Error
	if err == nil {
		// Plugin already exists in database, don't overwrite
		m.logger.Debug("plugin already registered in database", "plugin", plugin.ID, "status", existingPlugin.Status)
		return
	}
	
	// Plugin doesn't exist, create new database entry
	now := time.Now()
	
	// Use the plugin's enabled_by_default setting
	defaultStatus := "disabled"
	var enabledAt *time.Time
	if enabledByDefault {
		defaultStatus = "enabled"
		enabledAt = &now
		m.logger.Info("plugin enabled by default", "plugin", plugin.ID)
	}
	
	dbPlugin := map[string]interface{}{
		"plugin_id":    plugin.ID,
		"name":         plugin.Name,
		"version":      plugin.Version,
		"description":  plugin.Description,
		"author":       plugin.Author,
		"type":         plugin.Type,
		"status":       defaultStatus,
		"install_path": plugin.BasePath,
		"installed_at": now,
		"created_at":   now,
		"updated_at":   now,
	}
	
	// Add enabled_at if plugin is enabled by default
	if enabledAt != nil {
		dbPlugin["enabled_at"] = *enabledAt
	}
	
	if err := m.db.Table("plugins").Create(dbPlugin).Error; err != nil {
		m.logger.Error("failed to register plugin in database", "plugin", plugin.ID, "error", err)
		return
	}
	
	m.logger.Info("plugin registered in database", "plugin", plugin.ID, "status", defaultStatus)
}

// HostAssetService implements AssetService for the host side
// This allows plugins to save assets through the host's media asset module
type HostAssetService struct {
	logger hclog.Logger
}

// NewHostAssetService creates a new host asset service
func NewHostAssetService(logger hclog.Logger) *HostAssetService {
	return &HostAssetService{
		logger: logger.Named("host-asset-service"),
	}
}

// SaveAsset implements the AssetService interface for the host
func (h *HostAssetService) SaveAsset(mediaFileID string, assetType, category, subtype string, data []byte, mimeType, sourceURL, pluginID string, metadata map[string]string) (uint32, string, string, error) {
	h.logger.Debug("saving asset via plugin interface", "media_file_id", mediaFileID, "type", assetType, "category", category, "subtype", subtype, "mime_type", mimeType, "plugin_id", pluginID, "size", len(data))
	
	// Convert plugin asset format to new entity-based asset system
	h.logger.Info("Converting plugin asset to new entity-based asset system", "media_file_id", mediaFileID, "plugin_id", pluginID)
	
	// Get database connection
	db := database.GetDB()
	if db == nil {
		return 0, "", "", fmt.Errorf("database not available")
	}
	
	// Get the MediaFile using UUID string directly
	var mediaFile database.MediaFile
	if err := db.Where("id = ?", mediaFileID).First(&mediaFile).Error; err != nil {
		return 0, "", "", fmt.Errorf("failed to find media file %s: %w", mediaFileID, err)
	}
	
	// Only handle track media types for now
	if mediaFile.MediaType != database.MediaTypeTrack {
		return 0, "", "", fmt.Errorf("album artwork only supported for music tracks")
	}
	
	// Get the track to find the album
	var track database.Track
	if err := db.Preload("Album").Where("id = ?", mediaFile.MediaID).First(&track).Error; err != nil {
		return 0, "", "", fmt.Errorf("failed to find track for media file %s: %w", mediaFileID, err)
	}
	
	// Parse Album.ID as UUID
	albumUUID, err := uuid.Parse(track.Album.ID)
	if err != nil {
		return 0, "", "", fmt.Errorf("invalid album ID format for track %s: %w", track.ID, err)
	}
	
	h.logger.Info("Saving plugin asset for real album", "media_file_id", mediaFileID, "album_id", track.Album.ID, "album_title", track.Album.Title, "plugin_id", pluginID)
	
	// Detect proper MIME type from data if the provided one is generic
	if mimeType == "" || mimeType == "application/octet-stream" || mimeType == "binary/octet-stream" {
		mimeType = h.detectImageMimeType(data)
		h.logger.Info("Detected MIME type", "mime_type", mimeType, "media_file_id", mediaFileID)
	}
	
	// Determine the source based on plugin information
	var source assetmodule.AssetSource
	if pluginID == "" {
		// If no plugin ID is provided, try to infer from metadata
		source = assetmodule.SourcePlugin // Default for unknown plugins
	} else {
		// Determine if this is a core plugin or external plugin
		// Core plugins typically have names like "core_ffmpeg", "core_enrichment"
		if strings.HasPrefix(pluginID, "core_") {
			source = assetmodule.SourceCore
		} else {
			source = assetmodule.SourcePlugin
		}
	}
	
	h.logger.Info("Determined asset source", "source", source, "plugin_id", pluginID, "media_file_id", mediaFileID, "source_url", sourceURL)
	
	// Map old plugin asset types to new system
	var entityType assetmodule.EntityType
	var assetTypeNew assetmodule.AssetType
	
	switch assetType {
	case "music":
		entityType = assetmodule.EntityTypeAlbum
		switch subtype {
		case "artwork", "cover":
			assetTypeNew = assetmodule.AssetTypeCover
		default:
			assetTypeNew = assetmodule.AssetTypeCover // Default to cover
		}
	default:
		// Default fallback
		entityType = assetmodule.EntityTypeAlbum
		assetTypeNew = assetmodule.AssetTypeCover
	}
	
	// Create asset request for new system using real Album.ID
	request := &assetmodule.AssetRequest{
		EntityType: entityType,
		EntityID:   albumUUID,
		Type:       assetTypeNew,
		Source:     source,
		PluginID:   pluginID, // Set the plugin ID for tracking
		Data:       data,
		Format:     mimeType,
		Preferred:  true, // Mark plugin assets as preferred
	}

	// Save using the new asset manager
	response, err := assetmodule.SaveMediaAsset(request)
	if err != nil {
		h.logger.Error("Failed to save asset with new system", "error", err)
		return 0, "", "", fmt.Errorf("failed to save asset with new system: %w", err)
	}

	h.logger.Info("Successfully saved asset with new system", "asset_id", response.ID, "path", response.Path, "source", source, "plugin_id", pluginID, "album_title", track.Album.Title)
	
	// Return values compatible with old interface (using dummy values since new system uses UUIDs)
	return 1, response.ID.String(), response.Path, nil
}

// detectImageMimeType detects MIME type from image data
func (h *HostAssetService) detectImageMimeType(data []byte) string {
	if len(data) < 16 {
		return "image/jpeg" // Default fallback
	}
	
	// Check for common image signatures
	switch {
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}): // JPEG
		return "image/jpeg"
	case bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47}): // PNG
		return "image/png"
	case bytes.HasPrefix(data, []byte{0x47, 0x49, 0x46}): // GIF
		return "image/gif"
	case bytes.HasPrefix(data, []byte{0x52, 0x49, 0x46, 0x46}) && bytes.Contains(data[8:12], []byte("WEBP")): // WebP
		return "image/webp"
	case bytes.HasPrefix(data, []byte{0x42, 0x4D}): // BMP
		return "image/bmp"
	default:
		return "image/jpeg" // Default fallback
	}
}

// AssetExists implements the AssetService interface for the host
func (h *HostAssetService) AssetExists(mediaFileID string, assetType, category, subtype, hash string) (bool, uint32, string, error) {
	h.logger.Debug("checking asset existence via plugin interface", "media_file_id", mediaFileID, "type", assetType, "category", category, "hash", hash)
	
	// TODO: This is a compatibility stub for the old plugin asset system
	// Plugins will need to be updated to use the new entity-based asset system
	// For now, we'll disable plugin asset functionality to avoid breaking the build
	
	h.logger.Warn("plugin asset interface is deprecated - plugins need to be updated for new entity-based asset system")
	return false, 0, "", fmt.Errorf("plugin asset interface is deprecated - please update plugin to use new entity-based asset system")
}

// RemoveAsset implements the AssetService interface for the host
func (h *HostAssetService) RemoveAsset(assetID uint32) error {
	h.logger.Debug("removing asset via plugin interface", "asset_id", assetID)
	
	// TODO: This is a compatibility stub for the old plugin asset system
	// Plugins will need to be updated to use the new entity-based asset system
	// For now, we'll disable plugin asset functionality to avoid breaking the build
	
	h.logger.Warn("plugin asset interface is deprecated - plugins need to be updated for new entity-based asset system")
	return fmt.Errorf("plugin asset interface is deprecated - please update plugin to use new entity-based asset system")
}

// startHostServices starts the host-side gRPC server for plugin communication
func (m *manager) startHostServices() error {
	// Create listener on available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	
	m.hostGRPCListener = listener
	m.hostServiceAddr = listener.Addr().String()
	
	// Create host asset service
	m.hostAssetService = NewHostAssetService(m.logger)
	
	// Create gRPC server with increased message size limits for artwork uploads
	maxMsgSize := 50 * 1024 * 1024 // 50MB for high-quality artwork
	m.hostGRPCServer = grpc.NewServer(
		grpc.MaxRecvMsgSize(maxMsgSize),
		grpc.MaxSendMsgSize(maxMsgSize),
	)
	
	// Register AssetService on the host server
	proto.RegisterAssetServiceServer(m.hostGRPCServer, &AssetServer{Impl: m.hostAssetService})
	
	// Start server in background
	go func() {
		m.logger.Info("starting host gRPC server", "addr", m.hostServiceAddr)
		if err := m.hostGRPCServer.Serve(m.hostGRPCListener); err != nil {
			m.logger.Error("host gRPC server error", "error", err)
		}
	}()
	
	m.logger.Info("host services started", "addr", m.hostServiceAddr)
	return nil
}

// stopHostServices stops the host-side gRPC server
func (m *manager) stopHostServices() {
	if m.hostGRPCServer != nil {
		m.logger.Info("stopping host gRPC server")
		m.hostGRPCServer.GracefulStop()
		m.hostGRPCServer = nil
	}
	
	if m.hostGRPCListener != nil {
		m.hostGRPCListener.Close()
		m.hostGRPCListener = nil
	}
	
	m.hostServiceAddr = ""
	m.hostAssetService = nil
}

// ListCorePlugins returns information about all core plugins
func (m *manager) ListCorePlugins() []PluginInfo {
	if m.corePluginManager == nil {
		return []PluginInfo{}
	}
	return m.corePluginManager.ListCorePluginInfo()
}

// EnableCorePlugin enables a core plugin
func (m *manager) EnableCorePlugin(name string) error {
	if m.corePluginManager == nil {
		return fmt.Errorf("core plugin manager not initialized")
	}
	return m.corePluginManager.EnablePlugin(name)
}

// DisableCorePlugin disables a core plugin
func (m *manager) DisableCorePlugin(name string) error {
	if m.corePluginManager == nil {
		return fmt.Errorf("core plugin manager not initialized")
	}
	return m.corePluginManager.DisablePlugin(name)
}

// GetEnabledFileHandlers returns all enabled core file handlers
func (m *manager) GetEnabledFileHandlers() []FileHandlerPlugin {
	if m.corePluginManager == nil {
		return []FileHandlerPlugin{}
	}
	return m.corePluginManager.GetEnabledFileHandlers()
}

// RegisterCorePlugin allows external registration of core plugins
func (m *manager) RegisterCorePlugin(plugin CorePlugin) error {
	if m.corePluginManager == nil {
		return fmt.Errorf("core plugin manager not initialized")
	}
	return m.corePluginManager.RegisterCorePlugin(plugin)
}

// ListExternalPlugins returns information about external (non-core) plugins only
func (m *manager) ListExternalPlugins() []PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var infos []PluginInfo
	for _, plugin := range m.plugins {
		info := PluginInfo{
			ID:          plugin.ID,
			Name:        plugin.Name,
			Type:        plugin.Type,
			Version:     plugin.Version,
			Description: plugin.Description,
			Enabled:     plugin.Running,
			IsCore:      false, // External plugins are never core
			Category:    fmt.Sprintf("external_%s", plugin.Type),
		}
		infos = append(infos, info)
	}
	return infos
}

// InstallExternalPlugin installs an external plugin from a path
func (m *manager) InstallExternalPlugin(path string) error {
	// This would typically involve copying the plugin to the plugin directory
	// and registering it. For now, we'll implement basic discovery from the path.
	return fmt.Errorf("external plugin installation not yet implemented")
}

// UninstallExternalPlugin removes an external plugin
func (m *manager) UninstallExternalPlugin(pluginID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	plugin, exists := m.plugins[pluginID]
	if !exists {
		return fmt.Errorf("plugin not found: %s", pluginID)
	}
	
	// Stop the plugin if running
	if plugin.Running {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := m.UnloadPlugin(ctx, pluginID); err != nil {
			m.logger.Error("failed to unload plugin during uninstall", "plugin", pluginID, "error", err)
		}
	}
	
	// Remove from plugins map
	delete(m.plugins, pluginID)
	
	// Remove from database
	if m.db != nil {
		if err := m.db.Where("plugin_id = ?", pluginID).Delete(&struct {
			ID       uint32 `gorm:"primaryKey"`
			PluginID string `gorm:"uniqueIndex;not null"`
		}{}).Error; err != nil {
			m.logger.Error("failed to remove plugin from database", "plugin", pluginID, "error", err)
		}
	}
	
	m.logger.Info("external plugin uninstalled", "plugin", pluginID)
	return nil
}

// GetRunningPlugins returns information about currently running plugins
func (m *manager) GetRunningPlugins() []PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var infos []PluginInfo
	for _, plugin := range m.plugins {
		if plugin.Running {
			info := PluginInfo{
				Name:        plugin.Name,
				Type:        plugin.Type,
				Version:     plugin.Version,
				Description: plugin.Description,
				Enabled:     true,
				IsCore:      false,
			}
			infos = append(infos, info)
		}
	}
	return infos
}

// CallPlugin calls a method on a specific plugin
func (m *manager) CallPlugin(ctx context.Context, pluginID string, method string, args interface{}) (interface{}, error) {
	m.mu.RLock()
	plugin, exists := m.plugins[pluginID]
	m.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("plugin not found: %s", pluginID)
	}
	
	if !plugin.Running {
		return nil, fmt.Errorf("plugin not running: %s", pluginID)
	}
	
	// This would implement gRPC calls to the plugin
	return nil, fmt.Errorf("plugin method calls not yet implemented")
}

// ProcessMediaFile processes a media file using appropriate plugins
func (m *manager) ProcessMediaFile(filePath string, mediaFile interface{}) error {
	// Get all file handlers (both core and external)
	handlers := m.GetFileHandlers()
	
	// Try each handler until one succeeds
	for _, handler := range handlers {
		// Check if this handler can process the file
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			continue
		}
		
		if handler.Match(filePath, fileInfo) {
			// This handler can process the file, but we need to convert the interface
			// to the proper MetadataContext. This is a simplified version.
			m.logger.Info("processing file with handler", "file", filePath, "handler", handler.GetName())
			return fmt.Errorf("media file processing not fully implemented")
		}
	}
	
	return fmt.Errorf("no suitable handler found for file: %s", filePath)
}

// GetFileHandlers returns all available file handlers (core and external)
func (m *manager) GetFileHandlers() []FileHandlerPlugin {
	var handlers []FileHandlerPlugin
	
	// Add core plugin handlers
	if m.corePluginManager != nil {
		coreHandlers := m.corePluginManager.GetEnabledFileHandlers()
		handlers = append(handlers, coreHandlers...)
	}
	
	// Add external plugin handlers (would need to be implemented)
	// This would iterate through running external plugins and extract their handlers
	
	return handlers
}

// RegisterHook registers a hook for a plugin
func (m *manager) RegisterHook(pluginID string, hookName string, handler interface{}) error {
	// Store hook registration in database or memory
	m.logger.Info("hook registered", "plugin", pluginID, "hook", hookName)
	return fmt.Errorf("hook registration not yet implemented")
}

// TriggerHook triggers all registered hooks for a specific event
func (m *manager) TriggerHook(hookName string, data interface{}) error {
	// Find all plugins with registered hooks for this event and trigger them
	m.logger.Debug("triggering hook", "hook", hookName)
	return fmt.Errorf("hook triggering not yet implemented")
}

// GetCorePlugin returns a specific core plugin by name
func (m *manager) GetCorePlugin(name string) (CorePlugin, bool) {
	if m.corePluginManager != nil {
		return m.corePluginManager.GetCorePlugin(name)
	}
	return nil, false
}

type PluginAssetInfo struct {
	ID       uint32 `gorm:"primaryKey"`
	PluginID string `gorm:"not null;index"`
	// ... rest of fields ...
}

type PluginPermissionInfo struct {
	ID       uint32 `gorm:"primaryKey"`
	PluginID string `gorm:"not null;index"`
	// ... rest of fields ...
} 