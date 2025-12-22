// Package plugins provides infrastructure for loading and managing ViewRA plugins.
// It uses Hashicorp's go-plugin library for process isolation and gRPC communication.
package plugins

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/go-plugin"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	domainevents "github.com/mantonx/viewra/internal/domain/events"
)

// PluginCategory defines the type of functionality a plugin provides.
type PluginCategory string

const (
	CategoryEnricher         PluginCategory = "enricher"
	CategoryNotificationSink PluginCategory = "notification_sink"
	CategoryAI               PluginCategory = "ai"
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
	Manifest *Manifest

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

	// AISearchClient provides access to AI search methods (if applicable).
	AISearchClient pluginv1.AISearchClient

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

	// hostLLMServer provides LLM access for AI plugins.
	hostLLMServer *HostLLMServer

	// hostEmbeddingsServer provides embeddings storage for AI plugins.
	hostEmbeddingsServer *HostEmbeddingsServer

	// hostWeatherServer provides weather context for AI plugins.
	hostWeatherServer *HostWeatherServer

	// publisher publishes plugin lifecycle events (optional).
	publisher domainevents.Publisher

	// routeRegistry tracks routes registered by plugins.
	routeRegistry *RouteRegistry

	// capabilityRegistry tracks which plugins provide which capabilities.
	capabilityRegistry *CapabilityRegistry

	// rateLimiter provides rate limiting for plugin routes.
	rateLimiter *RouteRateLimiter

	// httpProxy proxies HTTP requests to plugins.
	httpProxy *HTTPProxy
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

	// HostLLMServer provides LLM access for AI plugins.
	// If nil, AI plugins will not be able to generate embeddings or chat.
	HostLLMServer *HostLLMServer

	// HostEmbeddingsServer provides embeddings storage for AI plugins.
	// If nil, AI plugins will not be able to store/retrieve embeddings.
	HostEmbeddingsServer *HostEmbeddingsServer

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
	routeRegistry := NewRouteRegistry()
	capabilityRegistry := NewCapabilityRegistry()
	rateLimiter := NewRouteRateLimiter()

	m := &Manager{
		plugins:              make(map[string]*PluginInstance),
		pluginDir:            cfg.PluginDir,
		storageDir:           cfg.StorageDir,
		logger:               logger,
		hostVersion:          cfg.HostVersion,
		healthCheckInterval:  cfg.HealthCheckInterval,
		maxRestarts:          cfg.MaxRestarts,
		hostDataServer:       hostDataServer,
		hostStorageServer:    cfg.HostStorageServer,
		hostLLMServer:        cfg.HostLLMServer,
		hostEmbeddingsServer: cfg.HostEmbeddingsServer,
		hostWeatherServer:    cfg.HostWeatherServer,
		routeRegistry:        routeRegistry,
		capabilityRegistry:   capabilityRegistry,
		rateLimiter:          rateLimiter,
	}

	// Create HTTP proxy (needs manager reference)
	m.httpProxy = NewHTTPProxy(m, routeRegistry, capabilityRegistry, rateLimiter, logger.With("component", "http-proxy"))

	return m, nil
}

// SetPublisher sets the event publisher for plugin lifecycle events.
func (m *Manager) SetPublisher(pub domainevents.Publisher) {
	m.publisher = pub
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
	manifest, err := LoadManifest(pluginDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load plugin manifest: %w", err)
	}

	// Get binary modification time as a proxy for build time
	var buildTime time.Time
	if info, err := os.Stat(path); err == nil {
		buildTime = info.ModTime()
	}

	m.logger.Info("loading plugin",
		"id", manifest.ID,
		"name", manifest.Name,
		"version", manifest.Version,
		"path", path)

	// Build plugin map with host services
	pluginMap := map[string]plugin.Plugin{
		"core":      &PluginCoreGRPCPlugin{},
		"enricher":  &EnricherGRPCPlugin{},
		"ai_search": &AISearchGRPCPlugin{},
	}

	// Create a logger for host services with plugin context
	hostServiceLogger := m.logger.With("plugin", manifest.ID, "component", "host-service")

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
			PluginID: manifest.ID,
			Logger:   hostServiceLogger,
		}
	}

	// Add host LLM service if available
	if m.hostLLMServer != nil {
		pluginMap["host_llm"] = &HostLLMGRPCPlugin{
			Impl:   m.hostLLMServer,
			Logger: hostServiceLogger,
		}
	}

	// Add host embeddings service if available
	if m.hostEmbeddingsServer != nil {
		pluginMap["host_embeddings"] = &HostEmbeddingsGRPCPlugin{
			Impl:   m.hostEmbeddingsServer,
			Logger: hostServiceLogger,
		}
	}

	// Add host weather service if available
	if m.hostWeatherServer != nil {
		pluginMap["host_weather"] = &HostWeatherGRPCPlugin{
			Impl:   m.hostWeatherServer,
			Logger: hostServiceLogger,
		}
	}

	// Create the go-plugin client
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  Handshake,
		Plugins:          pluginMap,
		Cmd:              exec.Command(path),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		Logger:           newHCLogAdapter(m.logger),
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
		ID:         manifest.ID,
		Manifest:   manifest,
		Path:       path,
		BuildTime:  buildTime,
		Client:     client,
		CoreClient: coreClient,
		Health: PluginHealth{
			Status:        pluginv1.HealthStatus_UNKNOWN,
			LastHeartbeat: time.Now(),
		},
		Categories: parseCategories(manifest.Categories),
	}

	// If it's an enricher, get the enricher client
	if hasCategory(instance.Categories, CategoryEnricher) {
		enricherRaw, err := rpcClient.Dispense("enricher")
		if err != nil {
			m.logger.Warn("plugin declares enricher category but dispense failed",
				"plugin", manifest.ID, "error", err)
		} else if enricherClient, ok := enricherRaw.(pluginv1.EnricherClient); ok {
			instance.EnricherClient = enricherClient
		}
	}

	// If it's an AI plugin, get the AI search client
	if hasCategory(instance.Categories, CategoryAI) {
		aiSearchRaw, err := rpcClient.Dispense("ai_search")
		if err != nil {
			m.logger.Warn("plugin declares ai category but dispense failed",
				"plugin", manifest.ID, "error", err)
		} else if aiSearchClient, ok := aiSearchRaw.(pluginv1.AISearchClient); ok {
			instance.AISearchClient = aiSearchClient
			m.logger.Info("AI search client available", "plugin", manifest.ID)
		}
	}

	// Initialize the plugin
	dataDir := ""
	if m.storageDir != "" {
		dataDir = filepath.Join(m.storageDir, manifest.ID)
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			m.logger.Warn("failed to create plugin data directory", "plugin", manifest.ID, "error", err)
		}
	}

	// Read plugin config from config.yml in the plugin's directory
	configPath := filepath.Join(pluginDir, "config.yml")
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			m.logger.Warn("plugin config.yml not found, plugin may not function correctly",
				"plugin", manifest.ID,
				"expected_path", configPath)
			configBytes = nil
		} else {
			client.Kill()
			return nil, fmt.Errorf("failed to read plugin config: %w", err)
		}
	} else {
		m.logger.Debug("loaded plugin config", "plugin", manifest.ID, "config_path", configPath)
	}

	// Dispense host storage to get the broker ID (this starts the server on the broker)
	var hostStorageBrokerID uint32
	if m.hostStorageServer != nil {
		m.logger.Debug("attempting to dispense host_storage", "plugin", manifest.ID)
		storageRaw, err := rpcClient.Dispense("host_storage")
		if err != nil {
			m.logger.Warn("failed to dispense host_storage", "plugin", manifest.ID, "error", err)
		} else if brokerInfo, ok := storageRaw.(*HostStorageBrokerInfo); ok && brokerInfo != nil {
			hostStorageBrokerID = brokerInfo.BrokerID
			m.logger.Info("host storage available for plugin",
				"plugin", manifest.ID,
				"broker_id", hostStorageBrokerID)
		} else {
			m.logger.Warn("host_storage dispense returned unexpected type",
				"plugin", manifest.ID,
				"got_type", fmt.Sprintf("%T", storageRaw),
				"is_nil", storageRaw == nil)
		}
	} else {
		m.logger.Debug("host storage server not configured", "plugin", manifest.ID)
	}

	// Dispense host LLM to get the broker ID
	var hostLLMBrokerID uint32
	if m.hostLLMServer != nil {
		m.logger.Debug("attempting to dispense host_llm", "plugin", manifest.ID)
		llmRaw, err := rpcClient.Dispense("host_llm")
		if err != nil {
			m.logger.Warn("failed to dispense host_llm", "plugin", manifest.ID, "error", err)
		} else if brokerInfo, ok := llmRaw.(*HostLLMBrokerInfo); ok && brokerInfo != nil {
			hostLLMBrokerID = brokerInfo.BrokerID
			m.logger.Info("host LLM available for plugin",
				"plugin", manifest.ID,
				"broker_id", hostLLMBrokerID)
		} else {
			m.logger.Warn("host_llm dispense returned unexpected type",
				"plugin", manifest.ID,
				"got_type", fmt.Sprintf("%T", llmRaw),
				"is_nil", llmRaw == nil)
		}
	} else {
		m.logger.Debug("host LLM server not configured", "plugin", manifest.ID)
	}

	// Dispense host embeddings to get the broker ID
	var hostEmbeddingsBrokerID uint32
	if m.hostEmbeddingsServer != nil {
		m.logger.Debug("attempting to dispense host_embeddings", "plugin", manifest.ID)
		embeddingsRaw, err := rpcClient.Dispense("host_embeddings")
		if err != nil {
			m.logger.Warn("failed to dispense host_embeddings", "plugin", manifest.ID, "error", err)
		} else if brokerInfo, ok := embeddingsRaw.(*HostEmbeddingsBrokerInfo); ok && brokerInfo != nil {
			hostEmbeddingsBrokerID = brokerInfo.BrokerID
			m.logger.Info("host embeddings available for plugin",
				"plugin", manifest.ID,
				"broker_id", hostEmbeddingsBrokerID)
		} else {
			m.logger.Warn("host_embeddings dispense returned unexpected type",
				"plugin", manifest.ID,
				"got_type", fmt.Sprintf("%T", embeddingsRaw),
				"is_nil", embeddingsRaw == nil)
		}
	} else {
		m.logger.Debug("host embeddings server not configured", "plugin", manifest.ID)
	}

	// Dispense host data to get the broker ID
	var hostDataBrokerID uint32
	if m.hostDataServer != nil {
		m.logger.Debug("attempting to dispense host_data", "plugin", manifest.ID)
		dataRaw, err := rpcClient.Dispense("host_data")
		if err != nil {
			m.logger.Warn("failed to dispense host_data", "plugin", manifest.ID, "error", err)
		} else if brokerInfo, ok := dataRaw.(*HostDataBrokerInfo); ok && brokerInfo != nil {
			hostDataBrokerID = brokerInfo.BrokerID
			m.logger.Info("host data available for plugin",
				"plugin", manifest.ID,
				"broker_id", hostDataBrokerID)
		} else {
			m.logger.Warn("host_data dispense returned unexpected type",
				"plugin", manifest.ID,
				"got_type", fmt.Sprintf("%T", dataRaw),
				"is_nil", dataRaw == nil)
		}
	} else {
		m.logger.Debug("host data server not configured", "plugin", manifest.ID)
	}

	// Dispense host weather to get the broker ID
	var hostWeatherBrokerID uint32
	if m.hostWeatherServer != nil {
		m.logger.Debug("attempting to dispense host_weather", "plugin", manifest.ID)
		weatherRaw, err := rpcClient.Dispense("host_weather")
		if err != nil {
			m.logger.Warn("failed to dispense host_weather", "plugin", manifest.ID, "error", err)
		} else if brokerInfo, ok := weatherRaw.(*HostWeatherBrokerInfo); ok && brokerInfo != nil {
			hostWeatherBrokerID = brokerInfo.BrokerID
			m.logger.Info("host weather available for plugin",
				"plugin", manifest.ID,
				"broker_id", hostWeatherBrokerID)
		} else {
			m.logger.Warn("host_weather dispense returned unexpected type",
				"plugin", manifest.ID,
				"got_type", fmt.Sprintf("%T", weatherRaw),
				"is_nil", weatherRaw == nil)
		}
	} else {
		m.logger.Debug("host weather server not configured", "plugin", manifest.ID)
	}

	initResp, err := coreClient.Initialize(ctx, &pluginv1.InitRequest{
		HostVersion:            m.hostVersion,
		DataDir:                dataDir,
		Config:                 configBytes,
		HostStorageBrokerId:    hostStorageBrokerID,
		HostLlmBrokerId:        hostLLMBrokerID,
		HostEmbeddingsBrokerId: hostEmbeddingsBrokerID,
		HostDataBrokerId:       hostDataBrokerID,
		HostWeatherBrokerId:    hostWeatherBrokerID,
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
	m.plugins[manifest.ID] = instance
	m.mu.Unlock()

	m.logger.Info("plugin loaded successfully",
		"id", manifest.ID,
		"name", manifest.Name,
		"version", manifest.Version,
		"categories", manifest.Categories,
	)

	// Register plugin routes
	if err := m.registerPluginRoutes(ctx, instance); err != nil {
		m.logger.Warn("failed to register plugin routes",
			"plugin", manifest.ID,
			"error", err)
		// Continue loading - plugin is usable but routes won't work
	}

	// Publish plugin.loaded event
	if m.publisher != nil {
		m.publisher.Publish(domainevents.NewEvent(domainevents.EventPluginLoaded, "plugin-manager").
			WithData("plugin_id", manifest.ID).
			WithData("name", manifest.Name).
			WithData("version", manifest.Version).
			WithData("categories", manifest.Categories).
			WithData("is_restart", false).
			Build())
	}

	return instance, nil
}

// LoadAllPlugins discovers and loads all plugins in the plugin directory.
func (m *Manager) LoadAllPlugins(ctx context.Context) error {
	paths, err := m.DiscoverPlugins()
	if err != nil {
		return err
	}

	for _, path := range paths {
		if _, err := m.LoadPlugin(ctx, path); err != nil {
			m.logger.Error("failed to load plugin", "path", path, "error", err)
			// Continue loading other plugins
		}
	}

	return nil
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

	// Unregister routes and capabilities
	m.routeRegistry.UnregisterRoutes(pluginID)
	m.capabilityRegistry.Unregister(pluginID)

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

// GetAISearchPlugin returns the first plugin that implements AI search.
// Returns nil if no AI search plugin is loaded.
func (m *Manager) GetAISearchPlugin() *PluginInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, p := range m.plugins {
		if hasCategory(p.Categories, CategoryAI) && p.AISearchClient != nil {
			return p
		}
	}
	return nil
}

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

	m.logger.Info("registered plugin routes",
		"plugin", plugin.ID,
		"count", len(routes.Routes))

	// Register capabilities
	for _, route := range routes.Routes {
		if route.Capability != "" {
			if m.capabilityRegistry.Register(plugin.ID, route.Capability, route.Path) {
				m.logger.Info("registered capability",
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
func (m *Manager) GetRouteRegistry() *RouteRegistry {
	return m.routeRegistry
}

// GetCapabilityRegistry returns the capability registry.
func (m *Manager) GetCapabilityRegistry() *CapabilityRegistry {
	return m.capabilityRegistry
}
