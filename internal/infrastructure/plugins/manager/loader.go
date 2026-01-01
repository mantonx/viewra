package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-plugin"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	domainevents "github.com/mantonx/viewra/internal/domain/events"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/logging"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/manifest"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/types"
	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

// LoadPlugin loads a single plugin from the given path.
func (m *Manager) LoadPlugin(ctx context.Context, path string) (*types.Instance, error) {
	if m.pluginFactory == nil {
		return nil, fmt.Errorf("plugin factory not set")
	}

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
	hostServiceLogger := m.logger.With("plugin", mf.ID, "component", "host-service")
	pluginMap := m.buildPluginMap(mf.ID, hostServiceLogger)

	// Create the go-plugin client
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  types.Handshake,
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

	// Create plugin instance
	instance, err := m.createPluginInstance(mf, path, buildTime, client, rpcClient)
	if err != nil {
		client.Kill()
		return nil, err
	}

	// Initialize the plugin
	if err := m.initializePlugin(ctx, instance, mf, pluginDir, rpcClient, client); err != nil {
		client.Kill()
		return nil, err
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

	// Post-load registration
	m.registerPluginServices(ctx, instance, mf)

	return instance, nil
}

// buildPluginMap creates the plugin map with core interfaces and host services.
func (m *Manager) buildPluginMap(pluginID string, hostServiceLogger *slog.Logger) map[string]plugin.Plugin {
	pluginMap := map[string]plugin.Plugin{
		"core":     m.pluginFactory.NewPluginCoreGRPCPlugin(),
		"enricher": m.pluginFactory.NewEnricherGRPCPlugin(),
		"provider": m.pluginFactory.NewPluginProviderGRPCPlugin(),
	}

	// Add host data service if available
	if m.hostDataServer != nil {
		if p := m.pluginFactory.NewHostDataGRPCPlugin(m.hostDataServer, hostServiceLogger); p != nil {
			pluginMap["host_data"] = p
		}
	}

	// Add host storage service if available (with plugin ID for context injection)
	if m.hostStorageServer != nil {
		if p := m.pluginFactory.NewHostStorageGRPCPlugin(m.hostStorageServer, pluginID, hostServiceLogger); p != nil {
			pluginMap["host_storage"] = p
		}
	}

	// Add host weather service if available
	if m.hostWeatherServer != nil {
		if p := m.pluginFactory.NewHostWeatherGRPCPlugin(m.hostWeatherServer, hostServiceLogger); p != nil {
			pluginMap["host_weather"] = p
		}
	}

	// Add host plugins service (always available - for capability-based plugin discovery)
	if m.hostPluginsServer != nil {
		if p := m.pluginFactory.NewHostPluginsGRPCPlugin(m.hostPluginsServer, hostServiceLogger); p != nil {
			pluginMap["host_plugins"] = p
		}
	}

	return pluginMap
}

// createPluginInstance creates the Instance struct and dispenses core interfaces.
func (m *Manager) createPluginInstance(
	mf *manifest.Manifest,
	path string,
	buildTime time.Time,
	client *plugin.Client,
	rpcClient plugin.ClientProtocol,
) (*types.Instance, error) {
	// Get the core interface
	raw, err := rpcClient.Dispense("core")
	if err != nil {
		return nil, fmt.Errorf("failed to dispense core interface: %w", err)
	}

	coreClient, ok := raw.(pluginv1.PluginCoreClient)
	if !ok {
		return nil, fmt.Errorf("unexpected core interface type: %T", raw)
	}

	// Create the plugin instance using manifest data
	instance := &types.Instance{
		ID:         mf.ID,
		Manifest:   mf,
		Path:       path,
		BuildTime:  buildTime,
		Client:     client,
		CoreClient: coreClient,
		Health: types.Health{
			Status:        pluginv1.HealthStatus_UNKNOWN,
			LastHeartbeat: time.Now(),
		},
		Categories: types.ParseCategories(mf.Categories),
	}

	// If it's an enricher, get the enricher client
	if types.HasCategoryIn(instance.Categories, types.CategoryEnricher) {
		enricherRaw, err := rpcClient.Dispense("enricher")
		if err != nil {
			m.logger.Warn("plugin declares enricher category but dispense failed",
				"plugin", mf.ID, "error", err)
		} else if enricherClient, ok := enricherRaw.(pluginv1.EnricherClient); ok {
			instance.EnricherClient = enricherClient
		}
	}

	// If it's a provider plugin, get the provider client
	if types.HasCategoryIn(instance.Categories, types.CategoryProvider) {
		providerRaw, err := rpcClient.Dispense("provider")
		if err != nil {
			m.logger.Warn("plugin declares provider category but dispense failed",
				"plugin", mf.ID, "error", err)
		} else if providerClient, ok := providerRaw.(pluginv1.PluginProviderClient); ok {
			instance.ProviderClient = providerClient
			m.logger.Debug("provider client available", "plugin", mf.ID)
		}
	}

	return instance, nil
}

// initializePlugin prepares and calls the plugin's Initialize method.
func (m *Manager) initializePlugin(
	ctx context.Context,
	instance *types.Instance,
	mf *manifest.Manifest,
	pluginDir string,
	rpcClient plugin.ClientProtocol,
	client *plugin.Client,
) error {
	// Prepare data directory
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
			return fmt.Errorf("failed to read plugin config: %w", err)
		}
	} else {
		m.logger.Debug("loaded plugin config", "plugin", mf.ID, "config_path", configPath)
	}

	// Dispense host services to get broker IDs
	brokerIDs := m.dispenseHostServices(mf.ID, rpcClient)

	// Call Initialize on the plugin
	initResp, err := instance.CoreClient.Initialize(ctx, &pluginv1.InitRequest{
		HostVersion:         m.hostVersion,
		DataDir:             dataDir,
		Config:              configBytes,
		HostStorageBrokerId: brokerIDs.storage,
		HostDataBrokerId:    brokerIDs.data,
		HostWeatherBrokerId: brokerIDs.weather,
		HostPluginsBrokerId: brokerIDs.plugins,
		SystemInfo:          m.systemInfo,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize plugin: %w", err)
	}
	if !initResp.Success {
		return fmt.Errorf("plugin initialization failed: %s", initResp.Error)
	}

	return nil
}

// hostBrokerIDs holds broker IDs for host services.
type hostBrokerIDs struct {
	storage uint32
	data    uint32
	weather uint32
	plugins uint32
}

// dispenseHostServices dispenses all host services and returns their broker IDs.
func (m *Manager) dispenseHostServices(pluginID string, rpcClient plugin.ClientProtocol) hostBrokerIDs {
	var ids hostBrokerIDs

	// Dispense host storage
	if m.hostStorageServer != nil {
		ids.storage = m.dispenseHostService(pluginID, rpcClient, "host_storage")
	}

	// Dispense host data
	if m.hostDataServer != nil {
		ids.data = m.dispenseHostService(pluginID, rpcClient, "host_data")
	}

	// Dispense host weather
	if m.hostWeatherServer != nil {
		ids.weather = m.dispenseHostService(pluginID, rpcClient, "host_weather")
	}

	// Dispense host plugins
	if m.hostPluginsServer != nil {
		ids.plugins = m.dispenseHostService(pluginID, rpcClient, "host_plugins")
	}

	return ids
}

// dispenseHostService dispenses a single host service and returns its broker ID.
func (m *Manager) dispenseHostService(pluginID string, rpcClient plugin.ClientProtocol, serviceName string) uint32 {
	m.logger.Debug("attempting to dispense "+serviceName, "plugin", pluginID)

	raw, err := rpcClient.Dispense(serviceName)
	if err != nil {
		m.logger.Warn("failed to dispense "+serviceName, "plugin", pluginID, "error", err)
		return 0
	}

	brokerID := m.pluginFactory.GetBrokerID(raw)
	if brokerID != 0 {
		m.logger.Debug(serviceName+" available for plugin",
			"plugin", pluginID,
			"broker_id", brokerID)
	} else {
		m.logger.Warn(serviceName+" dispense returned unexpected type",
			"plugin", pluginID,
			"got_type", fmt.Sprintf("%T", raw),
			"is_nil", raw == nil)
	}

	return brokerID
}

// registerPluginServices handles post-load registration of routes, capabilities, and events.
func (m *Manager) registerPluginServices(ctx context.Context, instance *types.Instance, mf *manifest.Manifest) {
	// Register plugin routes
	if err := m.registerPluginRoutes(ctx, instance); err != nil {
		m.logger.Warn("failed to register plugin routes",
			"plugin", mf.ID,
			"error", err)
		// Continue - plugin is usable but routes won't work
	}

	// Register provider plugin with provider registry
	if instance.ProviderClient != nil {
		caps, err := m.providerRegistry.Register(ctx, mf.ID, instance.ProviderClient, instance.CoreClient)
		if err != nil {
			m.logger.Warn("failed to register provider plugin",
				"plugin", mf.ID,
				"error", err)
		} else {
			m.logger.Debug("registered provider",
				"plugin", mf.ID,
				"provider_id", caps.ProviderId,
				"supports_chat", caps.SupportsChat,
				"supports_embeddings", caps.SupportsEmbeddings)
		}
	}

	// Register capabilities from manifest with the HostPluginsServer
	if m.hostPluginsServer != nil && len(mf.Provides) > 0 {
		for _, capability := range mf.Provides {
			m.hostPluginsServer.RegisterCapability(mf.ID, mf.Name, capability)
		}
		m.logger.Debug("registered plugin capabilities",
			"plugin", mf.ID,
			"capabilities", mf.Provides)
	}

	// Register widgets from plugin's settings schema
	m.registerPluginWidgets(ctx, instance, mf.ID)

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

	// Unregister routes, capabilities, providers, and widgets
	m.routeRegistry.UnregisterRoutes(pluginID)
	m.capabilityRegistry.Unregister(pluginID)
	m.providerRegistry.Unregister(pluginID)
	if m.widgetRegistry != nil {
		m.widgetRegistry.UnregisterPlugin(pluginID)
	}
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

// registerPluginWidgets extracts widgets from a plugin's settings schema and registers them.
func (m *Manager) registerPluginWidgets(ctx context.Context, instance *types.Instance, pluginID string) {
	if instance.CoreClient == nil || m.widgetRegistry == nil {
		return
	}

	// Fetch settings schema with timeout
	schemaCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	schemaResp, err := instance.CoreClient.GetSettingsSchema(schemaCtx, &pluginv1.Empty{})
	if err != nil {
		m.logger.Debug("failed to get settings schema for widget registration",
			"plugin", pluginID,
			"error", err)
		return
	}

	if len(schemaResp.JsonSchema) == 0 {
		return
	}

	// Parse the schema to extract x-viewra-widgets
	var schema map[string]any
	if err := json.Unmarshal(schemaResp.JsonSchema, &schema); err != nil {
		m.logger.Debug("failed to parse settings schema",
			"plugin", pluginID,
			"error", err)
		return
	}

	widgetsRaw, ok := schema["x-viewra-widgets"]
	if !ok {
		return
	}

	widgetsList, ok := widgetsRaw.([]any)
	if !ok {
		m.logger.Debug("x-viewra-widgets is not an array",
			"plugin", pluginID)
		return
	}

	// Convert to SDK Widget types and register
	var widgets []sdk.Widget
	for _, w := range widgetsList {
		widgetMap, ok := w.(map[string]any)
		if !ok {
			continue
		}

		widget := sdk.Widget{
			ID:                 getStringFromMap(widgetMap, "id"),
			Type:               getStringFromMap(widgetMap, "type"),
			Location:           getStringFromMap(widgetMap, "location"),
			Priority:           getIntFromMap(widgetMap, "priority"),
			CacheTTLSeconds:    getIntFromMap(widgetMap, "cache_ttl_seconds"),
			RequiredCapability: getStringFromMap(widgetMap, "required_capability"),
			SettingsKey:        getStringFromMap(widgetMap, "settings_key"),
			ClientTypes:        getStringSliceFromMap(widgetMap, "client_types"),
		}

		// Parse config map
		if configRaw, ok := widgetMap["config"].(map[string]any); ok {
			widget.Config = configRaw
		}

		if widget.ID != "" && widget.Type != "" {
			widgets = append(widgets, widget)
		}
	}

	if len(widgets) > 0 {
		m.widgetRegistry.RegisterAll(pluginID, widgets)
		m.logger.Debug("registered plugin widgets",
			"plugin", pluginID,
			"count", len(widgets))
	}
}

// getStringFromMap extracts a string value from a map.
func getStringFromMap(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// getIntFromMap extracts an int value from a map.
func getIntFromMap(m map[string]any, key string) int {
	switch v := m[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	case int64:
		return int(v)
	}
	return 0
}

// getStringSliceFromMap extracts a string slice from a map.
func getStringSliceFromMap(m map[string]any, key string) []string {
	raw, ok := m[key].([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}
