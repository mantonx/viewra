// Package manager provides the plugin Manager for loading, managing, and monitoring plugins.
package manager

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
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
	"github.com/mantonx/viewra/internal/infrastructure/plugins/registry"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/types"
)

// HostPluginsServer is an interface for the host plugins server functionality.
// This avoids a circular dependency with the parent plugins package.
type HostPluginsServer interface {
	RegisterCapability(pluginID, pluginName, capability string)
	UnregisterPlugin(pluginID string)
	UpdatePluginStatus(pluginID string, enabled, configured bool)
	HasCapability(capability string) bool
}

// HTTPProxy is an interface for the HTTP proxy functionality.
// This avoids a circular dependency with the parent plugins package.
type HTTPProxy interface{}

// HostDataServer is an interface for the host data server functionality.
type HostDataServer interface{}

// HostStorageServer is an interface for the host storage server functionality.
type HostStorageServer interface{}

// HostWeatherServer is an interface for the host weather server functionality.
type HostWeatherServer interface{}

// HostRatingsServer is an interface for the host ratings server functionality.
type HostRatingsServer interface{}

// HostProgressServer is an interface for the host progress server functionality.
type HostProgressServer interface{}

// Manager manages plugin lifecycle and provides access to plugin services.
type Manager struct {
	// plugins maps plugin ID to instance.
	plugins map[string]*types.Instance

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
	hostDataServer HostDataServer

	// hostStorageServer provides KV storage for plugins.
	hostStorageServer HostStorageServer

	// hostWeatherServer provides weather context for plugins.
	hostWeatherServer HostWeatherServer

	// hostRatingsServer provides user ratings access for plugins.
	hostRatingsServer HostRatingsServer

	// hostProgressServer provides watch progress access for plugins.
	hostProgressServer HostProgressServer

	// hostPluginsServer provides capability-based plugin discovery.
	hostPluginsServer HostPluginsServer

	// publisher publishes plugin lifecycle events (optional).
	publisher domainevents.Publisher

	// routeRegistry tracks routes registered by plugins.
	routeRegistry *registry.RouteRegistry

	// capabilityRegistry tracks which plugins provide which capabilities.
	capabilityRegistry *registry.CapabilityRegistry

	// rateLimiter provides rate limiting for plugin routes.
	rateLimiter *registry.RouteRateLimiter

	// httpProxy proxies HTTP requests to plugins.
	httpProxy HTTPProxy

	// providerRegistry tracks provider plugins.
	providerRegistry *registry.ProviderRegistry

	// widgetRegistry tracks widgets registered by plugins.
	widgetRegistry *registry.WidgetRegistry

	// searchProviderRegistry tracks search provider plugins.
	searchProviderRegistry *registry.SearchProviderRegistry

	// trendingProviderRegistry tracks trending provider plugins.
	trendingProviderRegistry *registry.TrendingProviderRegistry

	// systemInfo contains host system resource information to pass to plugins.
	systemInfo *pluginv1.SystemInfo

	// pluginFactory creates go-plugin Plugin instances.
	pluginFactory PluginFactory
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

	// HostStorageServer provides KV storage for plugins.
	// If nil, plugins will not be able to use host storage.
	HostStorageServer HostStorageServer

	// HostWeatherServer provides weather context for plugins.
	// If nil, plugins will not receive weather-based context enrichment.
	HostWeatherServer HostWeatherServer

	// HostRatingsServer provides user ratings access for plugins.
	// If nil, plugins will not be able to access user ratings.
	HostRatingsServer HostRatingsServer

	// HostProgressServer provides watch progress access for plugins.
	// If nil, plugins will not be able to access watch history.
	HostProgressServer HostProgressServer
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

	// Create registries and rate limiter
	routeRegistry := registry.NewRouteRegistry()
	capabilityRegistry := registry.NewCapabilityRegistry()
	providerRegistry := registry.NewProviderRegistry()
	widgetRegistry := registry.NewWidgetRegistry()
	searchProviderRegistry := registry.NewSearchProviderRegistry()
	trendingProviderRegistry := registry.NewTrendingProviderRegistry()
	rateLimiter := registry.NewRouteRateLimiter()

	m := &Manager{
		plugins:                  make(map[string]*types.Instance),
		pluginDir:                cfg.PluginDir,
		storageDir:               cfg.StorageDir,
		logger:                   logger,
		hostVersion:              cfg.HostVersion,
		healthCheckInterval:      cfg.HealthCheckInterval,
		maxRestarts:              cfg.MaxRestarts,
		hostStorageServer:        cfg.HostStorageServer,
		hostWeatherServer:        cfg.HostWeatherServer,
		hostRatingsServer:        cfg.HostRatingsServer,
		hostProgressServer:       cfg.HostProgressServer,
		routeRegistry:            routeRegistry,
		capabilityRegistry:       capabilityRegistry,
		providerRegistry:         providerRegistry,
		widgetRegistry:           widgetRegistry,
		searchProviderRegistry:   searchProviderRegistry,
		trendingProviderRegistry: trendingProviderRegistry,
		rateLimiter:              rateLimiter,
	}

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
	var pluginPaths []string

	entries, err := os.ReadDir(m.pluginDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// Check for a binary with the same name inside the directory
			binaryPath := filepath.Join(m.pluginDir, entry.Name(), entry.Name())
			if info, err := os.Stat(binaryPath); err == nil && !info.IsDir() {
				pluginPaths = append(pluginPaths, binaryPath)
			}
		} else {
			// Direct binary in plugin directory
			path := filepath.Join(m.pluginDir, entry.Name())
			if info, err := os.Stat(path); err == nil && !info.IsDir() && isExecutable(info) {
				pluginPaths = append(pluginPaths, path)
			}
		}
	}

	return pluginPaths, nil
}

// GetPlugin returns a plugin instance by ID.
func (m *Manager) GetPlugin(pluginID string) (*types.Instance, bool) {
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
func (m *Manager) ListPlugins() []*types.Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*types.Instance, 0, len(m.plugins))
	for _, p := range m.plugins {
		result = append(result, p)
	}
	return result
}

// GetEnrichers returns all plugins that implement the Enricher interface.
func (m *Manager) GetEnrichers() []*types.Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var enrichers []*types.Instance
	for _, p := range m.plugins {
		if types.HasCategoryIn(p.Categories, types.CategoryEnricher) && p.EnricherClient != nil {
			enrichers = append(enrichers, p)
		}
	}
	return enrichers
}

// GetAllPlugins returns all loaded plugins.
func (m *Manager) GetAllPlugins() []*types.Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*types.Instance, 0, len(m.plugins))
	for _, p := range m.plugins {
		result = append(result, p)
	}
	return result
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
	pluginList := make([]*types.Instance, 0, len(m.plugins))
	for _, p := range m.plugins {
		pluginList = append(pluginList, p)
	}
	sort.Slice(pluginList, func(i, j int) bool {
		return pluginList[i].Manifest.Name < pluginList[j].Manifest.Name
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

	for _, p := range pluginList {
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
		status := p.Health.Status.String()

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

// Shutdown gracefully shuts down all plugins.
func (m *Manager) Shutdown(ctx context.Context) {
	m.mu.Lock()
	pluginList := make([]*types.Instance, 0, len(m.plugins))
	for _, p := range m.plugins {
		pluginList = append(pluginList, p)
	}
	m.plugins = make(map[string]*types.Instance)
	m.mu.Unlock()

	for _, p := range pluginList {
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

// GetHTTPProxy returns the HTTP proxy for plugin routes.
func (m *Manager) GetHTTPProxy() HTTPProxy {
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

// GetProviderRegistry returns the provider registry for provider plugins.
func (m *Manager) GetProviderRegistry() *registry.ProviderRegistry {
	return m.providerRegistry
}

// GetWidgetRegistry returns the widget registry for home screen widgets.
func (m *Manager) GetWidgetRegistry() *registry.WidgetRegistry {
	return m.widgetRegistry
}

// GetSearchProviderRegistry returns the search provider registry.
func (m *Manager) GetSearchProviderRegistry() *registry.SearchProviderRegistry {
	return m.searchProviderRegistry
}

// GetTrendingProviderRegistry returns the trending provider registry.
func (m *Manager) GetTrendingProviderRegistry() *registry.TrendingProviderRegistry {
	return m.trendingProviderRegistry
}

// GetHostPluginsServer returns the host plugins server for capability-based plugin discovery.
func (m *Manager) GetHostPluginsServer() HostPluginsServer {
	return m.hostPluginsServer
}

// Helper functions

func isExecutable(info os.FileInfo) bool {
	return info.Mode()&0111 != 0
}

// Handshake returns the plugin handshake configuration.
func Handshake() plugin.HandshakeConfig {
	return types.Handshake
}

// SetHostDataServer sets the host data server.
func (m *Manager) SetHostDataServer(server HostDataServer) {
	m.hostDataServer = server
	if server != nil {
		m.logger.Info("HostDataServer set successfully", "type", fmt.Sprintf("%T", server))
	} else {
		m.logger.Warn("SetHostDataServer called with nil server")
	}
}

// SetHostPluginsServer sets the host plugins server.
func (m *Manager) SetHostPluginsServer(server HostPluginsServer) {
	m.hostPluginsServer = server
}

// SetHTTPProxy sets the HTTP proxy.
func (m *Manager) SetHTTPProxy(proxy HTTPProxy) {
	m.httpProxy = proxy
}

// SetPluginFactory sets the plugin factory.
func (m *Manager) SetPluginFactory(factory PluginFactory) {
	m.pluginFactory = factory
}

// GetPluginDir returns the plugin directory path.
func (m *Manager) GetPluginDir() string {
	return m.pluginDir
}

// GetStorageDir returns the storage directory path.
func (m *Manager) GetStorageDir() string {
	return m.storageDir
}

// GetLogger returns the manager's logger.
func (m *Manager) GetLogger() *slog.Logger {
	return m.logger
}

// GetHostVersion returns the host version string.
func (m *Manager) GetHostVersion() string {
	return m.hostVersion
}

// GetHealthCheckInterval returns the health check interval.
func (m *Manager) GetHealthCheckInterval() time.Duration {
	return m.healthCheckInterval
}

// GetMaxRestarts returns the maximum number of restarts.
func (m *Manager) GetMaxRestarts() int {
	return m.maxRestarts
}

// GetHostDataServer returns the host data server.
func (m *Manager) GetHostDataServer() HostDataServer {
	return m.hostDataServer
}

// GetHostStorageServer returns the host storage server.
func (m *Manager) GetHostStorageServer() HostStorageServer {
	return m.hostStorageServer
}

// GetHostWeatherServer returns the host weather server.
func (m *Manager) GetHostWeatherServer() HostWeatherServer {
	return m.hostWeatherServer
}

// GetHostRatingsServer returns the host ratings server.
func (m *Manager) GetHostRatingsServer() HostRatingsServer {
	return m.hostRatingsServer
}

// GetHostProgressServer returns the host progress server.
func (m *Manager) GetHostProgressServer() HostProgressServer {
	return m.hostProgressServer
}

// GetRateLimiter returns the rate limiter.
func (m *Manager) GetRateLimiter() *registry.RouteRateLimiter {
	return m.rateLimiter
}

// GetPublisher returns the event publisher.
func (m *Manager) GetPublisher() domainevents.Publisher {
	return m.publisher
}

// GetSystemInfo returns the system info.
func (m *Manager) GetSystemInfo() *pluginv1.SystemInfo {
	return m.systemInfo
}

// RegisterPlugin adds a plugin instance to the manager.
func (m *Manager) RegisterPlugin(id string, instance *types.Instance) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.plugins[id] = instance
}

// UnregisterPlugin removes a plugin instance from the manager.
func (m *Manager) UnregisterPlugin(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.plugins, id)
}
