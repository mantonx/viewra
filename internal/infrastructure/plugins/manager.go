// Package plugins provides infrastructure for loading and managing ViewRA plugins.
// It uses Hashicorp's go-plugin library for process isolation and gRPC communication.
package plugins

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-plugin"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	domainevents "github.com/mantonx/viewra/internal/domain/events"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/logging"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/manifest"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/registry"
)

// PluginCategory defines the type of functionality a plugin provides.
type PluginCategory string

const (
	CategoryEnricher         PluginCategory = "enricher"
	CategoryNotificationSink PluginCategory = "notification_sink"
	CategoryAI               PluginCategory = "ai"
	CategoryProvider         PluginCategory = "provider"
)

// Handshake is the shared handshake config for all plugins.
// Both host and plugin must agree on these values.
var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "VIEWRA_PLUGIN",
	MagicCookieValue: "viewra-plugin-v1",
}

// PluginHealth represents the current health status of a plugin.
type PluginHealth struct {
	Status        pluginv1.HealthStatus_Status
	Message       string
	LastHeartbeat time.Time
	ErrorRate     float64       // Errors per minute (rolling window)
	AvgLatency    time.Duration // Average response latency
	Restarts      int           // Number of times this plugin has been restarted
}

// PluginInstance represents a loaded and running plugin.
type PluginInstance struct {
	// ID is the unique identifier for this plugin.
	ID string

	// Manifest contains the plugin's static metadata from plugin.yml.
	Manifest *manifest.Manifest

	// Path is the filesystem path to the plugin binary.
	Path string

	// BuildTime is when the plugin binary was last modified (used as build time proxy).
	BuildTime time.Time

	// Client is the go-plugin client managing the plugin process.
	Client *plugin.Client

	// CoreClient provides access to the plugin's core RPC methods.
	CoreClient pluginv1.PluginCoreClient

	// EnricherClient provides access to enricher-specific methods (if applicable).
	EnricherClient pluginv1.EnricherClient

	// ProviderClient provides access to AI provider methods (if applicable).
	// This is set for plugins with category "provider" (e.g., provider-ollama, provider-openai).
	ProviderClient pluginv1.PluginProviderClient

	// Health tracks the plugin's current health status.
	Health PluginHealth

	// Categories lists which interfaces this plugin implements.
	Categories []PluginCategory

	// mu protects health updates.
	mu sync.RWMutex
}

// UpdateHealth updates the plugin's health status.
func (p *PluginInstance) UpdateHealth(status pluginv1.HealthStatus_Status, message string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Health.Status = status
	p.Health.Message = message
	p.Health.LastHeartbeat = time.Now()
}

// IsHealthy returns true if the plugin is in a healthy state.
func (p *PluginInstance) IsHealthy() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Health.Status == pluginv1.HealthStatus_HEALTHY
}

// Manager manages plugin lifecycle and provides access to plugin services.
type Manager struct {
	// plugins maps plugin ID to instance.
	plugins map[string]*PluginInstance

	// pluginDir is the directory containing plugin binaries.
	pluginDir string

	// storageDir is the base directory for plugin data storage.
	storageDir string

	// logger is used for logging plugin operations.
	logger *slog.Logger

	// hostVersion is the ViewRA version for compatibility checking.
	hostVersion string

	// mu protects the plugins map.
	mu sync.RWMutex

	// healthCheckInterval is how often to check plugin health.
	healthCheckInterval time.Duration

	// maxRestarts is the maximum number of automatic restarts for a crashed plugin.
	maxRestarts int

	// hostDataServer provides media data access for plugins.
	hostDataServer *HostDataServer

	// hostStorageServer provides KV storage for plugins.
	hostStorageServer *HostStorageServer

	// hostWeatherServer provides weather context for AI plugins.
	hostWeatherServer *HostWeatherServer

	// hostPluginsServer provides capability-based plugin discovery.
	hostPluginsServer *HostPluginsServer

	// publisher publishes plugin lifecycle events (optional).
	publisher domainevents.Publisher

	// routeRegistry tracks routes registered by plugins.
	routeRegistry *registry.RouteRegistry

	// capabilityRegistry tracks which plugins provide which capabilities.
	capabilityRegistry *registry.CapabilityRegistry

	// rateLimiter provides rate limiting for plugin routes.
	rateLimiter *registry.RouteRateLimiter

	// httpProxy proxies HTTP requests to plugins.
	httpProxy *HTTPProxy

	// providerRegistry tracks AI provider plugins.
	providerRegistry *registry.ProviderRegistry

	// systemInfo contains host system resource information to pass to plugins.
	systemInfo *pluginv1.SystemInfo
}

// ManagerConfig configures the plugin manager.
type ManagerConfig struct {
	// PluginDir is the directory containing plugin binaries.
	PluginDir string

	// StorageDir is the base directory for plugin data storage.
	StorageDir string

	// HostVersion is the ViewRA version for compatibility checking.
	HostVersion string

	// HealthCheckInterval is how often to check plugin health.
	// Defaults to 30 seconds if not set.
	HealthCheckInterval time.Duration

	// MaxRestarts is the maximum number of automatic restarts for a crashed plugin.
	// Defaults to 3 if not set.
	MaxRestarts int

	// MediaQuerier provides media data access for plugins.
	// If nil, plugins will not be able to query media data.
	MediaQuerier MediaQuerier

	// HostStorageServer provides KV storage for plugins.
	// If nil, plugins will not be able to use host storage.
	HostStorageServer *HostStorageServer

	// HostWeatherServer provides weather context for AI plugins.
	// If nil, AI plugins will not receive weather-based context enrichment.
	HostWeatherServer *HostWeatherServer
}

// NewManager creates a new plugin manager.
func NewManager(cfg ManagerConfig, logger *slog.Logger) (*Manager, error) {
	if cfg.PluginDir == "" {
		return nil, errors.New("plugin directory is required")
	}

	if cfg.HealthCheckInterval == 0 {
		cfg.HealthCheckInterval = 30 * time.Second
	}
	if cfg.MaxRestarts == 0 {
		cfg.MaxRestarts = 3
	}

	// Ensure directories exist
	if err := os.MkdirAll(cfg.PluginDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create plugin directory: %w", err)
	}
	if cfg.StorageDir != "" {
		if err := os.MkdirAll(cfg.StorageDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create storage directory: %w", err)
		}
	}

	// Create HostDataServer if MediaQuerier is provided
	var hostDataServer *HostDataServer
	if cfg.MediaQuerier != nil {
		hostDataServer = NewHostDataServer(cfg.MediaQuerier, logger.With("component", "host-data-server"))
	}

	// Create registries and rate limiter
	routeRegistry := registry.NewRouteRegistry()
	capabilityRegistry := registry.NewCapabilityRegistry()
	providerRegistry := registry.NewProviderRegistry()
	rateLimiter := registry.NewRouteRateLimiter()

	m := &Manager{
		plugins:             make(map[string]*PluginInstance),
		pluginDir:           cfg.PluginDir,
		storageDir:          cfg.StorageDir,
		logger:              logger,
		hostVersion:         cfg.HostVersion,
		healthCheckInterval: cfg.HealthCheckInterval,
		maxRestarts:         cfg.MaxRestarts,
		hostDataServer:      hostDataServer,
		hostStorageServer:   cfg.HostStorageServer,
		hostWeatherServer:   cfg.HostWeatherServer,
		routeRegistry:       routeRegistry,
		capabilityRegistry:  capabilityRegistry,
		providerRegistry:    providerRegistry,
		rateLimiter:         rateLimiter,
	}

	// Create host plugins server for capability-based plugin discovery
	m.hostPluginsServer = NewHostPluginsServer(m, logger.With("component", "host-plugins-server"))

	// Create HTTP proxy (needs manager reference)
	m.httpProxy = NewHTTPProxy(m, routeRegistry, capabilityRegistry, rateLimiter, logger.With("component", "http-proxy"))

	return m, nil
}

// SetPublisher sets the event publisher for plugin lifecycle events.
func (m *Manager) SetPublisher(pub domainevents.Publisher) {
	m.publisher = pub
}

// SetSystemInfo sets the system information to pass to plugins during initialization.
// This should be called before loading plugins so they can receive hardware/system details.
func (m *Manager) SetSystemInfo(info *pluginv1.SystemInfo) {
	m.systemInfo = info
}

// DiscoverPlugins scans the plugin directory for plugin binaries.
// It returns a list of paths to discovered plugins.
func (m *Manager) DiscoverPlugins() ([]string, error) {
	var plugins []string

	entries, err := os.ReadDir(m.pluginDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// Check for a binary with the same name inside the directory
			binaryPath := filepath.Join(m.pluginDir, entry.Name(), entry.Name())
			if info, err := os.Stat(binaryPath); err == nil && !info.IsDir() {
				plugins = append(plugins, binaryPath)
			}
		} else {
			// Direct binary in plugin directory
			path := filepath.Join(m.pluginDir, entry.Name())
			if info, err := os.Stat(path); err == nil && !info.IsDir() && isExecutable(info) {
				plugins = append(plugins, path)
			}
		}
	}

	return plugins, nil
}

// LoadPlugin loads a single plugin from the given path.
func (m *Manager) LoadPlugin(ctx context.Context, path string) (*PluginInstance, error) {
	pluginDir := filepath.Dir(path)

	// Read manifest first (before starting the binary)
	mf, err := manifest.Load(pluginDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load plugin manifest: %w", err)
	}

	// Get binary modification time as a proxy for build time
	var buildTime time.Time
	if info, err := os.Stat(path); err == nil {
		buildTime = info.ModTime()
	}

	m.logger.Debug("loading plugin",
		"id", mf.ID,
		"name", mf.Name,
		"version", mf.Version,
		"path", path)

	// Build plugin map with host services
	pluginMap := map[string]plugin.Plugin{
		"core":     &PluginCoreGRPCPlugin{},
		"enricher": &EnricherGRPCPlugin{},
		"provider": &PluginProviderGRPCPlugin{},
	}

	// Create a logger for host services with plugin context
	hostServiceLogger := m.logger.With("plugin", mf.ID, "component", "host-service")

	// Add host data service if available
	if m.hostDataServer != nil {
		pluginMap["host_data"] = &HostDataGRPCPlugin{
			Impl:   m.hostDataServer,
			Logger: hostServiceLogger,
		}
	}

	// Add host storage service if available (with plugin ID for context injection)
	if m.hostStorageServer != nil {
		pluginMap["host_storage"] = &HostStorageGRPCPlugin{
			Impl:     m.hostStorageServer,
			PluginID: mf.ID,
			Logger:   hostServiceLogger,
		}
	}

	// Add host weather service if available
	if m.hostWeatherServer != nil {
		pluginMap["host_weather"] = &HostWeatherGRPCPlugin{
			Impl:   m.hostWeatherServer,
			Logger: hostServiceLogger,
		}
	}

	// Add host plugins service (always available - for capability-based plugin discovery)
	if m.hostPluginsServer != nil {
		pluginMap["host_plugins"] = &HostPluginsGRPCPlugin{
			Impl:   m.hostPluginsServer,
			Logger: hostServiceLogger,
		}
	}

	// Create the go-plugin client
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  Handshake,
		Plugins:          pluginMap,
		Cmd:              exec.Command(path),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		Logger:           logging.NewHclogAdapter(m.logger),
		SyncStdout:       os.Stdout,
		SyncStderr:       os.Stderr,
	})

	// Connect to the plugin
	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("failed to connect to plugin: %w", err)
	}

	// Get the core interface
	raw, err := rpcClient.Dispense("core")
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("failed to dispense core interface: %w", err)
	}

	coreClient, ok := raw.(pluginv1.PluginCoreClient)
	if !ok {
		client.Kill()
		return nil, fmt.Errorf("unexpected core interface type: %T", raw)
	}

	// Create the plugin instance using manifest data
	instance := &PluginInstance{
		ID:         mf.ID,
		Manifest:   mf,
		Path:       path,
		BuildTime:  buildTime,
		Client:     client,
		CoreClient: coreClient,
		Health: PluginHealth{
			Status:        pluginv1.HealthStatus_UNKNOWN,
			LastHeartbeat: time.Now(),
		},
		Categories: parseCategories(mf.Categories),
	}

	// If it's an enricher, get the enricher client
	if hasCategory(instance.Categories, CategoryEnricher) {
		enricherRaw, err := rpcClient.Dispense("enricher")
		if err != nil {
			m.logger.Warn("plugin declares enricher category but dispense failed",
				"plugin", mf.ID, "error", err)
		} else if enricherClient, ok := enricherRaw.(pluginv1.EnricherClient); ok {
			instance.EnricherClient = enricherClient
		}
	}

	// If it's a provider plugin, get the provider client
	if hasCategory(instance.Categories, CategoryProvider) {
		providerRaw, err := rpcClient.Dispense("provider")
		if err != nil {
			m.logger.Warn("plugin declares provider category but dispense failed",
				"plugin", mf.ID, "error", err)
		} else if providerClient, ok := providerRaw.(pluginv1.PluginProviderClient); ok {
			instance.ProviderClient = providerClient
			m.logger.Debug("provider client available", "plugin", mf.ID)
		}
	}

	// Initialize the plugin
	dataDir := ""
	if m.storageDir != "" {
		dataDir = filepath.Join(m.storageDir, mf.ID)
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			m.logger.Warn("failed to create plugin data directory", "plugin", mf.ID, "error", err)
		}
	}

	// Read plugin config from config.yml in the plugin's directory
	configPath := filepath.Join(pluginDir, "config.yml")
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			m.logger.Warn("plugin config.yml not found, plugin may not function correctly",
				"plugin", mf.ID,
				"expected_path", configPath)
			configBytes = nil
		} else {
			client.Kill()
			return nil, fmt.Errorf("failed to read plugin config: %w", err)
		}
	} else {
		m.logger.Debug("loaded plugin config", "plugin", mf.ID, "config_path", configPath)
	}

	// Dispense host storage to get the broker ID (this starts the server on the broker)
	var hostStorageBrokerID uint32
	if m.hostStorageServer != nil {
		m.logger.Debug("attempting to dispense host_storage", "plugin", mf.ID)
		storageRaw, err := rpcClient.Dispense("host_storage")
		if err != nil {
			m.logger.Warn("failed to dispense host_storage", "plugin", mf.ID, "error", err)
		} else if brokerInfo, ok := storageRaw.(*HostStorageBrokerInfo); ok && brokerInfo != nil {
			hostStorageBrokerID = brokerInfo.BrokerID
			m.logger.Debug("host storage available for plugin",
				"plugin", mf.ID,
				"broker_id", hostStorageBrokerID)
		} else {
			m.logger.Warn("host_storage dispense returned unexpected type",
				"plugin", mf.ID,
				"got_type", fmt.Sprintf("%T", storageRaw),
				"is_nil", storageRaw == nil)
		}
	} else {
		m.logger.Debug("host storage server not configured", "plugin", mf.ID)
	}

	// Dispense host data to get the broker ID
	var hostDataBrokerID uint32
	if m.hostDataServer != nil {
		m.logger.Debug("attempting to dispense host_data", "plugin", mf.ID)
		dataRaw, err := rpcClient.Dispense("host_data")
		if err != nil {
			m.logger.Warn("failed to dispense host_data", "plugin", mf.ID, "error", err)
		} else if brokerInfo, ok := dataRaw.(*HostDataBrokerInfo); ok && brokerInfo != nil {
			hostDataBrokerID = brokerInfo.BrokerID
			m.logger.Debug("host data available for plugin",
				"plugin", mf.ID,
				"broker_id", hostDataBrokerID)
		} else {
			m.logger.Warn("host_data dispense returned unexpected type",
				"plugin", mf.ID,
				"got_type", fmt.Sprintf("%T", dataRaw),
				"is_nil", dataRaw == nil)
		}
	} else {
		m.logger.Debug("host data server not configured", "plugin", mf.ID)
	}

	// Dispense host weather to get the broker ID
	var hostWeatherBrokerID uint32
	if m.hostWeatherServer != nil {
		m.logger.Debug("attempting to dispense host_weather", "plugin", mf.ID)
		weatherRaw, err := rpcClient.Dispense("host_weather")
		if err != nil {
			m.logger.Warn("failed to dispense host_weather", "plugin", mf.ID, "error", err)
		} else if brokerInfo, ok := weatherRaw.(*HostWeatherBrokerInfo); ok && brokerInfo != nil {
			hostWeatherBrokerID = brokerInfo.BrokerID
			m.logger.Debug("host weather available for plugin",
				"plugin", mf.ID,
				"broker_id", hostWeatherBrokerID)
		} else {
			m.logger.Warn("host_weather dispense returned unexpected type",
				"plugin", mf.ID,
				"got_type", fmt.Sprintf("%T", weatherRaw),
				"is_nil", weatherRaw == nil)
		}
	} else {
		m.logger.Debug("host weather server not configured", "plugin", mf.ID)
	}

	// Dispense host plugins to get the broker ID (for capability-based discovery)
	var hostPluginsBrokerID uint32
	if m.hostPluginsServer != nil {
		m.logger.Debug("attempting to dispense host_plugins", "plugin", mf.ID)
		pluginsRaw, err := rpcClient.Dispense("host_plugins")
		if err != nil {
			m.logger.Warn("failed to dispense host_plugins", "plugin", mf.ID, "error", err)
		} else if brokerInfo, ok := pluginsRaw.(*HostPluginsBrokerInfo); ok && brokerInfo != nil {
			hostPluginsBrokerID = brokerInfo.BrokerID
			m.logger.Debug("host plugins available for plugin",
				"plugin", mf.ID,
				"broker_id", hostPluginsBrokerID)
		} else {
			m.logger.Warn("host_plugins dispense returned unexpected type",
				"plugin", mf.ID,
				"got_type", fmt.Sprintf("%T", pluginsRaw),
				"is_nil", pluginsRaw == nil)
		}
	} else {
		m.logger.Debug("host plugins server not configured", "plugin", mf.ID)
	}

	initResp, err := coreClient.Initialize(ctx, &pluginv1.InitRequest{
		HostVersion:         m.hostVersion,
		DataDir:             dataDir,
		Config:              configBytes,
		HostStorageBrokerId: hostStorageBrokerID,
		HostDataBrokerId:    hostDataBrokerID,
		HostWeatherBrokerId: hostWeatherBrokerID,
		HostPluginsBrokerId: hostPluginsBrokerID,
		SystemInfo:          m.systemInfo,
	})
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("failed to initialize plugin: %w", err)
	}
	if !initResp.Success {
		client.Kill()
		return nil, fmt.Errorf("plugin initialization failed: %s", initResp.Error)
	}

	// Register the plugin
	m.mu.Lock()
	m.plugins[mf.ID] = instance
	m.mu.Unlock()

	// Set initial health status to healthy since Initialize succeeded
	instance.UpdateHealth(pluginv1.HealthStatus_HEALTHY, "initialized")

	m.logger.Debug("plugin loaded successfully",
		"id", mf.ID,
		"name", mf.Name,
		"version", mf.Version,
		"categories", mf.Categories,
	)

	// Register plugin routes
	if err := m.registerPluginRoutes(ctx, instance); err != nil {
		m.logger.Warn("failed to register plugin routes",
			"plugin", mf.ID,
			"error", err)
		// Continue loading - plugin is usable but routes won't work
	}

	// Register provider plugin with provider registry
	if instance.ProviderClient != nil {
		caps, err := m.providerRegistry.Register(ctx, mf.ID, instance.ProviderClient, instance.CoreClient)
		if err != nil {
			m.logger.Warn("failed to register provider plugin",
				"plugin", mf.ID,
				"error", err)
		} else {
			m.logger.Debug("registered AI provider",
				"plugin", mf.ID,
				"provider_id", caps.ProviderId,
				"supports_chat", caps.SupportsChat,
				"supports_embeddings", caps.SupportsEmbeddings)
		}
	}

	// Register capabilities from manifest with the HostPluginsServer
	// This enables other plugins to discover and connect to this plugin
	if m.hostPluginsServer != nil && len(mf.Provides) > 0 {
		for _, capability := range mf.Provides {
			m.hostPluginsServer.RegisterCapability(mf.ID, mf.Name, capability)
		}
		m.logger.Debug("registered plugin capabilities",
			"plugin", mf.ID,
			"capabilities", mf.Provides)
	}

	// Publish plugin.loaded event
	if m.publisher != nil {
		m.publisher.Publish(domainevents.NewEvent(domainevents.EventPluginLoaded, "plugin-manager").
			WithData("plugin_id", mf.ID).
			WithData("name", mf.Name).
			WithData("version", mf.Version).
			WithData("categories", mf.Categories).
			WithData("is_restart", false).
			Build())
	}

	return instance, nil
}

// pluginLoadInfo holds manifest and path info for dependency resolution.
type pluginLoadInfo struct {
	manifest *manifest.Manifest
	path     string
}

// LoadAllPlugins discovers and loads all plugins in the plugin directory.
// It performs dependency resolution to ensure provider plugins load before
// plugins that depend on them.
func (m *Manager) LoadAllPlugins(ctx context.Context) error {
	paths, err := m.DiscoverPlugins()
	if err != nil {
		return err
	}

	// Phase 1: Load all manifests without starting plugins
	pluginInfos := make(map[string]*pluginLoadInfo)
	for _, path := range paths {
		pluginDir := filepath.Dir(path)
		mf, err := manifest.Load(pluginDir)
		if err != nil {
			m.logger.Error("failed to load plugin manifest", "path", path, "error", err)
			continue
		}
		pluginInfos[mf.ID] = &pluginLoadInfo{
			manifest: mf,
			path:     path,
		}
	}

	// Phase 2: Build capability -> plugin mapping
	capabilityProviders := make(map[string][]string) // capability -> []pluginID
	for pluginID, info := range pluginInfos {
		for _, capability := range info.manifest.Provides {
			capabilityProviders[capability] = append(capabilityProviders[capability], pluginID)
		}
	}

	// Phase 3: Resolve load order using topological sort
	loadOrder, skipped := m.resolveLoadOrder(pluginInfos, capabilityProviders)

	// Log skipped plugins
	for pluginID, reason := range skipped {
		m.logger.Warn("skipping plugin due to unsatisfied dependencies",
			"plugin", pluginID,
			"reason", reason)
	}

	// Phase 4: Load plugins in resolved order
	for _, pluginID := range loadOrder {
		info := pluginInfos[pluginID]
		if _, err := m.LoadPlugin(ctx, info.path); err != nil {
			m.logger.Error("failed to load plugin", "plugin", pluginID, "path", info.path, "error", err)
			// Continue loading other plugins
		}
	}

	return nil
}

// resolveLoadOrder performs topological sort based on plugin dependencies.
// Returns the ordered list of plugin IDs to load and a map of skipped plugins with reasons.
func (m *Manager) resolveLoadOrder(
	plugins map[string]*pluginLoadInfo,
	capabilityProviders map[string][]string,
) ([]string, map[string]string) {
	// Build adjacency list: plugin -> plugins it depends on
	dependencies := make(map[string][]string)
	skipped := make(map[string]string)

	for pluginID, info := range plugins {
		var deps []string
		for _, dep := range info.manifest.Dependencies {
			// Find plugins that provide this capability
			providers := capabilityProviders[dep.Capability]
			if len(providers) == 0 {
				if dep.Required {
					skipped[pluginID] = fmt.Sprintf("required capability '%s' not provided by any plugin", dep.Capability)
					break
				}
				// Optional dependency not satisfied - continue without it
				m.logger.Debug("optional dependency not satisfied",
					"plugin", pluginID,
					"capability", dep.Capability)
				continue
			}
			// Add all providers as dependencies (any one of them can satisfy it)
			deps = append(deps, providers...)
		}
		if _, wasSkipped := skipped[pluginID]; !wasSkipped {
			dependencies[pluginID] = deps
		}
	}

	// Remove skipped plugins from consideration
	for pluginID := range skipped {
		delete(dependencies, pluginID)
	}

	// Kahn's algorithm for topological sort
	inDegree := make(map[string]int)
	for pluginID := range dependencies {
		if _, exists := inDegree[pluginID]; !exists {
			inDegree[pluginID] = 0
		}
		for _, dep := range dependencies[pluginID] {
			// Only count dependencies that are in our plugin set
			if _, exists := dependencies[dep]; exists {
				inDegree[pluginID]++
			}
		}
	}

	// Start with plugins that have no dependencies
	var queue []string
	for pluginID, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, pluginID)
		}
	}

	var result []string
	for len(queue) > 0 {
		// Pop from queue
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// Reduce in-degree for plugins that depend on current
		for pluginID, deps := range dependencies {
			for _, dep := range deps {
				if dep == current {
					inDegree[pluginID]--
					if inDegree[pluginID] == 0 {
						queue = append(queue, pluginID)
					}
				}
			}
		}
	}

	// Check for circular dependencies
	if len(result) != len(dependencies) {
		// Find plugins involved in cycle
		for pluginID := range dependencies {
			found := false
			for _, r := range result {
				if r == pluginID {
					found = true
					break
				}
			}
			if !found {
				skipped[pluginID] = "circular dependency detected"
			}
		}
	}

	m.logger.Debug("resolved plugin load order",
		"order", result,
		"skipped_count", len(skipped))

	return result, skipped
}

// UnloadPlugin gracefully shuts down and removes a plugin.
func (m *Manager) UnloadPlugin(ctx context.Context, pluginID string) error {
	m.mu.Lock()
	instance, ok := m.plugins[pluginID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("plugin not found: %s", pluginID)
	}
	delete(m.plugins, pluginID)
	m.mu.Unlock()

	// Gracefully shutdown the plugin
	if instance.CoreClient != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_, _ = instance.CoreClient.Shutdown(shutdownCtx, &pluginv1.Empty{})
	}

	// Kill the plugin process
	instance.Client.Kill()

	// Unregister routes, capabilities, and providers
	m.routeRegistry.UnregisterRoutes(pluginID)
	m.capabilityRegistry.Unregister(pluginID)
	m.providerRegistry.Unregister(pluginID)
	if m.hostPluginsServer != nil {
		m.hostPluginsServer.UnregisterPlugin(pluginID)
	}

	m.logger.Info("plugin unloaded", "id", pluginID)

	// Publish plugin.unloaded event
	if m.publisher != nil {
		m.publisher.Publish(domainevents.NewEvent(domainevents.EventPluginUnloaded, "plugin-manager").
			WithData("plugin_id", pluginID).
			WithData("reason", "disabled").
			Build())
	}

	return nil
}

// GetPlugin returns a plugin instance by ID.
func (m *Manager) GetPlugin(pluginID string) (*PluginInstance, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	instance, ok := m.plugins[pluginID]
	return instance, ok
}

// IsPluginEnabled returns true if the plugin exists, is loaded, and is healthy.
// Implements the PluginLookup interface for the HostPluginsServer.
func (m *Manager) IsPluginEnabled(pluginID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	instance, ok := m.plugins[pluginID]
	if !ok {
		return false
	}
	return instance.IsHealthy()
}

// ListPlugins returns all loaded plugins.
func (m *Manager) ListPlugins() []*PluginInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*PluginInstance, 0, len(m.plugins))
	for _, p := range m.plugins {
		result = append(result, p)
	}
	return result
}

// GetEnrichers returns all plugins that implement the Enricher interface.
func (m *Manager) GetEnrichers() []*PluginInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var enrichers []*PluginInstance
	for _, p := range m.plugins {
		if hasCategory(p.Categories, CategoryEnricher) && p.EnricherClient != nil {
			enrichers = append(enrichers, p)
		}
	}
	return enrichers
}

// GetAllPlugins returns all loaded plugins.
func (m *Manager) GetAllPlugins() []*PluginInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plugins := make([]*PluginInstance, 0, len(m.plugins))
	for _, p := range m.plugins {
		plugins = append(plugins, p)
	}
	return plugins
}

// PrintTable writes a formatted table of all loaded plugins to the writer.
func (m *Manager) PrintTable(w io.Writer, title string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.plugins) == 0 {
		fmt.Fprintln(w, "No plugins loaded")
		return
	}

	// Print title if provided
	if title != "" {
		fmt.Fprintf(w, "\n%s\n", title)
	}

	// Collect and sort plugins by name
	plugins := make([]*PluginInstance, 0, len(m.plugins))
	for _, p := range m.plugins {
		plugins = append(plugins, p)
	}
	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].Manifest.Name < plugins[j].Manifest.Name
	})

	// Create table with Unicode box-drawing style
	table := tablewriter.NewTable(w,
		tablewriter.WithRenderer(renderer.NewBlueprint(tw.Rendition{
			Borders: tw.Border{
				Left:   tw.On,
				Right:  tw.On,
				Top:    tw.On,
				Bottom: tw.On,
			},
			Symbols: tw.NewSymbols(tw.StyleLight),
			Settings: tw.Settings{
				Separators: tw.Separators{
					BetweenColumns: tw.On,
				},
				Lines: tw.Lines{
					ShowHeaderLine: tw.On,
				},
			},
		})),
	)

	table.Header("Name", "Version", "Categories", "Status", "Built")

	for _, p := range plugins {
		// Format categories
		categories := make([]string, 0, len(p.Categories))
		for _, c := range p.Categories {
			categories = append(categories, string(c))
		}
		catStr := strings.Join(categories, ", ")
		if catStr == "" {
			catStr = "-"
		}

		// Format status
		p.mu.RLock()
		status := p.Health.Status.String()
		p.mu.RUnlock()

		// Format build time
		buildTimeStr := "-"
		if !p.BuildTime.IsZero() {
			buildTimeStr = p.BuildTime.Format("2006-01-02 15:04")
		}

		table.Append([]string{
			p.Manifest.Name,
			p.Manifest.Version,
			catStr,
			status,
			buildTimeStr,
		})
	}

	table.Render()
}

// GetVectorSearchPlugin returns the first plugin that implements vector search.
// Returns nil if no vector search plugin is loaded.
// Shutdown gracefully shuts down all plugins.
func (m *Manager) Shutdown(ctx context.Context) {
	m.mu.Lock()
	plugins := make([]*PluginInstance, 0, len(m.plugins))
	for _, p := range m.plugins {
		plugins = append(plugins, p)
	}
	m.plugins = make(map[string]*PluginInstance)
	m.mu.Unlock()

	for _, p := range plugins {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		if p.CoreClient != nil {
			_, _ = p.CoreClient.Shutdown(shutdownCtx, &pluginv1.Empty{})
		}
		p.Client.Kill()
		cancel()

		// Publish plugin.unloaded event for each plugin
		if m.publisher != nil {
			m.publisher.Publish(domainevents.NewEvent(domainevents.EventPluginUnloaded, "plugin-manager").
				WithData("plugin_id", p.ID).
				WithData("reason", "shutdown").
				Build())
		}
	}

	// Stop rate limiter background tasks
	if m.rateLimiter != nil {
		m.rateLimiter.Stop()
	}

	m.logger.Info("all plugins shut down")
}

// StartHealthMonitor starts a background goroutine that monitors plugin health.
func (m *Manager) StartHealthMonitor(ctx context.Context) {
	ticker := time.NewTicker(m.healthCheckInterval)
	go func() {
		for {
			select {
			case <-ticker.C:
				m.checkAllHealth(ctx)
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

func (m *Manager) checkAllHealth(ctx context.Context) {
	plugins := m.ListPlugins()
	for _, p := range plugins {
		m.checkPluginHealth(ctx, p)
	}
}

func (m *Manager) checkPluginHealth(ctx context.Context, p *PluginInstance) {
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Capture previous status for change detection
	p.mu.RLock()
	previousStatus := p.Health.Status
	p.mu.RUnlock()

	health, err := p.CoreClient.HealthCheck(checkCtx, &pluginv1.Empty{})
	if err != nil {
		p.UpdateHealth(pluginv1.HealthStatus_UNHEALTHY, err.Error())
		m.logger.Warn("plugin health check failed", "plugin", p.ID, "error", err)

		// Publish health update event (status changed to unhealthy)
		if m.publisher != nil && previousStatus != pluginv1.HealthStatus_UNHEALTHY {
			m.publisher.Publish(domainevents.NewEvent(domainevents.EventPluginHealthUpdate, "plugin-manager").
				WithData("plugin_id", p.ID).
				WithData("status", "unhealthy").
				WithData("previous_status", previousStatus.String()).
				WithData("message", err.Error()).
				Build())
		}

		// Check if we should restart the plugin
		if p.Health.Restarts < m.maxRestarts {
			m.restartPlugin(ctx, p)
		}
		return
	}

	// Publish health update event only if status changed
	if m.publisher != nil && health.Status != previousStatus {
		m.publisher.Publish(domainevents.NewEvent(domainevents.EventPluginHealthUpdate, "plugin-manager").
			WithData("plugin_id", p.ID).
			WithData("status", health.Status.String()).
			WithData("previous_status", previousStatus.String()).
			WithData("message", health.Message).
			Build())
	}

	p.UpdateHealth(health.Status, health.Message)
}

func (m *Manager) restartPlugin(ctx context.Context, p *PluginInstance) {
	m.logger.Info("restarting plugin", "plugin", p.ID, "restarts", p.Health.Restarts)

	willRestart := p.Health.Restarts < m.maxRestarts
	restartCount := p.Health.Restarts + 1

	// Publish plugin.crashed event before restart attempt
	if m.publisher != nil {
		m.publisher.Publish(domainevents.NewEvent(domainevents.EventPluginCrashed, "plugin-manager").
			WithData("plugin_id", p.ID).
			WithData("restart_count", restartCount).
			WithData("will_restart", willRestart).
			WithData("message", "health check failed").
			Build())
	}

	// Kill the old process
	p.Client.Kill()

	// Remove from registry
	m.mu.Lock()
	delete(m.plugins, p.ID)
	m.mu.Unlock()

	// Try to reload
	newInstance, err := m.LoadPlugin(ctx, p.Path)
	if err != nil {
		m.logger.Error("failed to restart plugin", "plugin", p.ID, "error", err)
		// Publish crash event for failed restart
		if m.publisher != nil {
			m.publisher.Publish(domainevents.NewEvent(domainevents.EventPluginCrashed, "plugin-manager").
				WithData("plugin_id", p.ID).
				WithData("restart_count", restartCount).
				WithData("will_restart", false).
				WithData("message", err.Error()).
				Build())
		}
		return
	}

	newInstance.Health.Restarts = restartCount

	// Publish plugin.loaded with is_restart=true (overrides the one from LoadPlugin)
	if m.publisher != nil {
		m.publisher.Publish(domainevents.NewEvent(domainevents.EventPluginLoaded, "plugin-manager").
			WithData("plugin_id", newInstance.ID).
			WithData("name", newInstance.Manifest.Name).
			WithData("version", newInstance.Manifest.Version).
			WithData("categories", newInstance.Manifest.Categories).
			WithData("is_restart", true).
			WithData("restart_count", restartCount).
			Build())
	}

	m.logger.Info("plugin restarted successfully", "plugin", p.ID, "restarts", newInstance.Health.Restarts)
}

// RestartPlugin restarts a plugin by ID. This is useful when the plugin binary
// has been rebuilt and we need to reload it without a full server restart.
func (m *Manager) RestartPlugin(ctx context.Context, pluginID string) error {
	m.mu.RLock()
	p, ok := m.plugins[pluginID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("plugin not found: %s", pluginID)
	}

	m.logger.Info("restarting plugin by request", "plugin", pluginID)

	// Kill the old process
	p.Client.Kill()

	// Remove from registry
	m.mu.Lock()
	delete(m.plugins, pluginID)
	m.mu.Unlock()

	// Try to reload
	newInstance, err := m.LoadPlugin(ctx, p.Path)
	if err != nil {
		return fmt.Errorf("failed to restart plugin %s: %w", pluginID, err)
	}

	// Publish plugin.loaded event
	if m.publisher != nil {
		m.publisher.Publish(domainevents.NewEvent(domainevents.EventPluginLoaded, "plugin-manager").
			WithData("plugin_id", newInstance.ID).
			WithData("name", newInstance.Manifest.Name).
			WithData("version", newInstance.Manifest.Version).
			WithData("categories", newInstance.Manifest.Categories).
			WithData("is_restart", true).
			Build())
	}

	m.logger.Info("plugin restarted successfully by request", "plugin", pluginID)
	return nil
}

// Helper functions

func isExecutable(info os.FileInfo) bool {
	return info.Mode()&0111 != 0
}

func parseCategories(categories []string) []PluginCategory {
	result := make([]PluginCategory, 0, len(categories))
	for _, c := range categories {
		result = append(result, PluginCategory(c))
	}
	return result
}

func hasCategory(categories []PluginCategory, target PluginCategory) bool {
	for _, c := range categories {
		if c == target {
			return true
		}
	}
	return false
}

// registerPluginRoutes fetches and registers routes for a plugin.
func (m *Manager) registerPluginRoutes(ctx context.Context, plugin *PluginInstance) error {
	if plugin.CoreClient == nil {
		return nil
	}

	// Get routes from plugin
	routes, err := plugin.CoreClient.GetRoutes(ctx, &pluginv1.Empty{})
	if err != nil {
		return err
	}

	if routes == nil || len(routes.Routes) == 0 {
		m.logger.Debug("plugin has no routes", "plugin", plugin.ID)
		return nil
	}

	// Register routes
	m.routeRegistry.RegisterRoutes(plugin.ID, routes.Routes)

	m.logger.Debug("registered plugin routes",
		"plugin", plugin.ID,
		"count", len(routes.Routes))

	// Register capabilities
	for _, route := range routes.Routes {
		if route.Capability != "" {
			if m.capabilityRegistry.Register(plugin.ID, route.Capability, route.Path) {
				m.logger.Debug("registered capability",
					"plugin", plugin.ID,
					"capability", route.Capability,
					"path", route.Path)
			} else {
				m.logger.Warn("capability already registered by another plugin",
					"capability", route.Capability,
					"plugin", plugin.ID)
			}
		}
	}

	return nil
}

// GetHTTPProxy returns the HTTP proxy for plugin routes.
func (m *Manager) GetHTTPProxy() *HTTPProxy {
	return m.httpProxy
}

// GetRouteRegistry returns the route registry.
func (m *Manager) GetRouteRegistry() *registry.RouteRegistry {
	return m.routeRegistry
}

// GetCapabilityRegistry returns the capability registry.
func (m *Manager) GetCapabilityRegistry() *registry.CapabilityRegistry {
	return m.capabilityRegistry
}

// GetProviderRegistry returns the provider registry for AI provider plugins.
func (m *Manager) GetProviderRegistry() *registry.ProviderRegistry {
	return m.providerRegistry
}

// GetHostPluginsServer returns the host plugins server for capability-based plugin discovery.
func (m *Manager) GetHostPluginsServer() *HostPluginsServer {
	return m.hostPluginsServer
}
