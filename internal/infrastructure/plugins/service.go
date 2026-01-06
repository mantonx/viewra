// Package plugins provides the plugin infrastructure for ViewRA.
// It supports loading and managing external plugins that extend the server's functionality.
//
// The package is organized into sub-packages:
//   - grpc/     - gRPC plugin implementations for go-plugin framework
//   - host/     - Host services exposed to plugins (data, storage, weather, plugins)
//   - logging/  - slog to hclog adapter for plugin logging
//   - manager/  - Plugin lifecycle management (load, unload, restart, health)
//   - manifest/ - Plugin manifest (plugin.yml) parsing
//   - pool/     - Warm pool for plugin pre-loading
//   - proxy/    - HTTP proxy for plugin routes
//   - querier/  - Media query interface for plugins
//   - registry/ - Registries for routes, capabilities, providers, rate limits
//   - types/    - Shared types (Instance, Health, Category, Handshake)
//
// This file re-exports the main types for backward compatibility.
// Callers can import the sub-packages directly for more specific access.
package plugins

import (
	pluginsgrpc "github.com/mantonx/viewra/internal/infrastructure/plugins/grpc"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/host"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/manager"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/proxy"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/types"
)

// Type aliases for backward compatibility.
// New code should import the sub-packages directly.

// Manager manages plugin lifecycle and provides access to plugin services.
type Manager = manager.Manager

// ManagerConfig contains configuration for the plugin manager.
type ManagerConfig = manager.ManagerConfig

// NewManager creates a new plugin manager.
var NewManager = manager.NewManager

// HTTPProxy handles proxying HTTP requests to plugin gRPC handlers.
type HTTPProxy = proxy.HTTPProxy

// NewHTTPProxy creates a new HTTP proxy for plugin routes.
var NewHTTPProxy = proxy.NewHTTPProxy

// PluginInstance represents a loaded and running plugin.
// Deprecated: Use types.Instance instead.
type PluginInstance = types.Instance

// HostStorageServer provides storage services to plugins.
type HostStorageServer = host.StorageServer

// HostStorageConfig contains configuration for the host storage server.
type HostStorageConfig = host.StorageConfig

// NewHostStorageServer creates a new host storage server.
var NewHostStorageServer = host.NewStorageServer

// HostWeatherServer provides weather context to plugins.
type HostWeatherServer = host.WeatherServer

// NewHostWeatherServer creates a new host weather server.
var NewHostWeatherServer = host.NewWeatherServer

// HostDataServer provides media data access to plugins.
type HostDataServer = host.DataServer

// NewHostDataServer creates a new host data server.
var NewHostDataServer = host.NewDataServer

// HostPluginsServer provides capability-based plugin discovery.
type HostPluginsServer = host.PluginsServer

// NewHostPluginsServer creates a new host plugins server.
var NewHostPluginsServer = host.NewPluginsServer

// HostRatingsServer provides user ratings access to plugins.
type HostRatingsServer = host.RatingsServer

// NewHostRatingsServer creates a new host ratings server.
var NewHostRatingsServer = host.NewRatingsServer

// HostProgressServer provides watch progress access to plugins.
type HostProgressServer = host.ProgressServer

// NewHostProgressServer creates a new host progress server.
var NewHostProgressServer = host.NewProgressServer

// GRPCEnricher wraps a gRPC EnricherClient to implement the application Enricher interface.
type GRPCEnricher = pluginsgrpc.Enricher

// NewGRPCEnricher creates a new GRPCEnricher wrapping the given client.
var NewGRPCEnricher = pluginsgrpc.NewEnricher

// MediaQuerier provides media query operations for plugins.
type MediaQuerier = host.MediaQuerier

// Context helpers re-exported from host package.
var (
	// ContextWithPluginID adds a plugin ID to a context.
	ContextWithPluginID = host.ContextWithPluginID
	// GetPluginIDFromContext retrieves the plugin ID from a context.
	GetPluginIDFromContext = host.GetPluginIDFromContext
)

// gRPC plugin types re-exported from grpc package.
type (
	// PluginCoreGRPCPlugin is the go-plugin implementation for the PluginCore service.
	PluginCoreGRPCPlugin = pluginsgrpc.PluginCorePlugin
	// EnricherGRPCPlugin is the go-plugin implementation for the Enricher service.
	EnricherGRPCPlugin = pluginsgrpc.EnricherPlugin
	// HostDataGRPCPlugin is the go-plugin implementation for the HostData service.
	HostDataGRPCPlugin = pluginsgrpc.HostDataPlugin
	// HostStorageGRPCPlugin is the go-plugin implementation for the HostStorage service.
	HostStorageGRPCPlugin = pluginsgrpc.HostStoragePlugin
	// HostWeatherGRPCPlugin is the go-plugin implementation for the HostWeather service.
	HostWeatherGRPCPlugin = pluginsgrpc.HostWeatherPlugin
	// HostPluginsGRPCPlugin is the go-plugin implementation for the HostPlugins service.
	HostPluginsGRPCPlugin = pluginsgrpc.HostPluginsPlugin
	// HostRatingsGRPCPlugin is the go-plugin implementation for the HostRatings service.
	HostRatingsGRPCPlugin = pluginsgrpc.HostRatingsPlugin
	// HostProgressGRPCPlugin is the go-plugin implementation for the HostProgress service.
	HostProgressGRPCPlugin = pluginsgrpc.HostProgressPlugin
	// PluginProviderGRPCPlugin is the go-plugin implementation for the PluginProvider service.
	PluginProviderGRPCPlugin = pluginsgrpc.PluginProviderPlugin

	// Broker info types
	HostDataBrokerInfo     = pluginsgrpc.HostDataBrokerInfo
	HostStorageBrokerInfo  = pluginsgrpc.HostStorageBrokerInfo
	HostWeatherBrokerInfo  = pluginsgrpc.HostWeatherBrokerInfo
	HostPluginsBrokerInfo  = pluginsgrpc.HostPluginsBrokerInfo
	HostRatingsBrokerInfo  = pluginsgrpc.HostRatingsBrokerInfo
	HostProgressBrokerInfo = pluginsgrpc.HostProgressBrokerInfo

	// Factory creates go-plugin Plugin instances for the plugin manager.
	Factory = pluginsgrpc.Factory
)

// NewFactory creates a new plugin factory.
var NewFactory = pluginsgrpc.NewFactory

// Logging interceptors re-exported from grpc package.
var (
	LoggingInterceptor       = pluginsgrpc.LoggingInterceptor
	LoggingStreamInterceptor = pluginsgrpc.LoggingStreamInterceptor
)

// Types re-exported from types package.
type (
	// Category defines the type of functionality a plugin provides.
	Category = types.Category
	// Health represents the current health status of a plugin.
	Health = types.Health
)

// Category constants.
const (
	CategoryEnricher         = types.CategoryEnricher
	CategoryNotificationSink = types.CategoryNotificationSink
	CategoryProvider         = types.CategoryProvider
)

// Handshake is the shared handshake config for all plugins.
var Handshake = types.Handshake
