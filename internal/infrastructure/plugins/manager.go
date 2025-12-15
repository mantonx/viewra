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
)

// PluginCategory defines the type of functionality a plugin provides.
type PluginCategory string

const (
	CategoryEnricher         PluginCategory = "enricher"
	CategoryNotificationSink PluginCategory = "notification_sink"
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

	// Info contains the plugin's identity and capabilities.
	Info *pluginv1.PluginInfo

	// Path is the filesystem path to the plugin binary.
	Path string

	// Client is the go-plugin client managing the plugin process.
	Client *plugin.Client

	// CoreClient provides access to the plugin's core RPC methods.
	CoreClient pluginv1.PluginCoreClient

	// EnricherClient provides access to enricher-specific methods (if applicable).
	EnricherClient pluginv1.EnricherClient

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

	return &Manager{
		plugins:             make(map[string]*PluginInstance),
		pluginDir:           cfg.PluginDir,
		storageDir:          cfg.StorageDir,
		logger:              logger,
		hostVersion:         cfg.HostVersion,
		healthCheckInterval: cfg.HealthCheckInterval,
		maxRestarts:         cfg.MaxRestarts,
	}, nil
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
	m.logger.Info("loading plugin", "path", path)

	// Create the go-plugin client
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: Handshake,
		Plugins: map[string]plugin.Plugin{
			"core":     &PluginCoreGRPCPlugin{},
			"enricher": &EnricherGRPCPlugin{},
		},
		Cmd:              exec.Command(path),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		Logger:           newHCLogAdapter(m.logger),
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

	// Get plugin info
	info, err := coreClient.GetInfo(ctx, &pluginv1.Empty{})
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("failed to get plugin info: %w", err)
	}

	// Create the plugin instance
	instance := &PluginInstance{
		ID:         info.Id,
		Info:       info,
		Path:       path,
		Client:     client,
		CoreClient: coreClient,
		Health: PluginHealth{
			Status:        pluginv1.HealthStatus_UNKNOWN,
			LastHeartbeat: time.Now(),
		},
		Categories: parseCategories(info.Categories),
	}

	// If it's an enricher, get the enricher client
	if hasCategory(instance.Categories, CategoryEnricher) {
		enricherRaw, err := rpcClient.Dispense("enricher")
		if err != nil {
			m.logger.Warn("plugin declares enricher category but dispense failed",
				"plugin", info.Id, "error", err)
		} else if enricherClient, ok := enricherRaw.(pluginv1.EnricherClient); ok {
			instance.EnricherClient = enricherClient
		}
	}

	// Initialize the plugin
	dataDir := ""
	if m.storageDir != "" {
		dataDir = filepath.Join(m.storageDir, info.Id)
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			m.logger.Warn("failed to create plugin data directory", "plugin", info.Id, "error", err)
		}
	}

	initResp, err := coreClient.Initialize(ctx, &pluginv1.InitRequest{
		HostVersion: m.hostVersion,
		DataDir:     dataDir,
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
	m.plugins[info.Id] = instance
	m.mu.Unlock()

	m.logger.Info("plugin loaded successfully",
		"id", info.Id,
		"name", info.Name,
		"version", info.Version,
		"categories", info.Categories,
	)

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

	m.logger.Info("plugin unloaded", "id", pluginID)
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

	health, err := p.CoreClient.HealthCheck(checkCtx, &pluginv1.Empty{})
	if err != nil {
		p.UpdateHealth(pluginv1.HealthStatus_UNHEALTHY, err.Error())
		m.logger.Warn("plugin health check failed", "plugin", p.ID, "error", err)

		// Check if we should restart the plugin
		if p.Health.Restarts < m.maxRestarts {
			m.restartPlugin(ctx, p)
		}
		return
	}

	p.UpdateHealth(health.Status, health.Message)
}

func (m *Manager) restartPlugin(ctx context.Context, p *PluginInstance) {
	m.logger.Info("restarting plugin", "plugin", p.ID, "restarts", p.Health.Restarts)

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
		return
	}

	newInstance.Health.Restarts = p.Health.Restarts + 1
	m.logger.Info("plugin restarted successfully", "plugin", p.ID, "restarts", newInstance.Health.Restarts)
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
