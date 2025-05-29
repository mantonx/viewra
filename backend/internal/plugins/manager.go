package plugins

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sync"
	"time"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/fsnotify/fsnotify"
	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
	"github.com/mantonx/viewra/internal/apiroutes"
	"github.com/mantonx/viewra/internal/plugins/proto"
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
}

// NewManager creates a new plugin manager
func NewManager(pluginDir string, db *gorm.DB, logger hclog.Logger) Manager {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &manager{
		pluginDir: pluginDir,
		db:        db,
		logger:    logger.Named("plugin-manager"),
		plugins:   make(map[string]*Plugin),
		registry:  Registry{},
		ctx:       ctx,
		cancel:    cancel,
	}
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
	
	// Load enabled plugins
	if err := m.loadEnabledPlugins(ctx); err != nil {
		return fmt.Errorf("failed to load enabled plugins: %w", err)
	}
	
	m.logger.Info("plugin manager initialized", "plugins_discovered", len(m.plugins))
	return nil
}

// Shutdown gracefully stops all plugins and cleans up resources
func (m *manager) Shutdown(ctx context.Context) error {
	m.logger.Info("shutting down plugin manager")
	
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
			DatabaseUrl: dbURL,
			PluginId:    pluginID,
			BasePath:    plugin.BasePath,
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
func (m *manager) ListPlugins() map[string]*Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make(map[string]*Plugin, len(m.plugins))
	for id, plugin := range m.plugins {
		result[id] = plugin
	}
	return result
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
		PluginID string
		Status   string
	}
	
	if err := m.db.Table("plugins").
		Select("plugin_id, status").
		Where("status = ?", "enabled").
		Find(&enabledPlugins).Error; err != nil {
		return fmt.Errorf("failed to query enabled plugins: %w", err)
	}
	
	for _, dbPlugin := range enabledPlugins {
		if err := m.LoadPlugin(ctx, dbPlugin.PluginID); err != nil {
			m.logger.Error("failed to load enabled plugin", "plugin", dbPlugin.PluginID, "error", err)
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
	// Use absolute path to ensure plugins can find the database
	// regardless of their working directory
	dbPath := os.Getenv("SQLITE_PATH")
	if dbPath == "" {
		dbPath = "/app/data/viewra.db"
	}
	return "sqlite:" + dbPath
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
		ID       uint   `gorm:"primaryKey"`
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