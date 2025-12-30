package manager

import (
	"log/slog"

	"github.com/hashicorp/go-plugin"
)

// PluginFactory creates go-plugin Plugin instances.
// This interface allows the manager package to create plugins without
// importing the parent plugins package, avoiding circular dependencies.
type PluginFactory interface {
	// NewPluginCoreGRPCPlugin creates a new PluginCoreGRPCPlugin.
	NewPluginCoreGRPCPlugin() plugin.Plugin

	// NewEnricherGRPCPlugin creates a new EnricherGRPCPlugin.
	NewEnricherGRPCPlugin() plugin.Plugin

	// NewPluginProviderGRPCPlugin creates a new PluginProviderGRPCPlugin.
	NewPluginProviderGRPCPlugin() plugin.Plugin

	// NewHostDataGRPCPlugin creates a new HostDataGRPCPlugin.
	NewHostDataGRPCPlugin(impl HostDataServer, logger *slog.Logger) plugin.Plugin

	// NewHostStorageGRPCPlugin creates a new HostStorageGRPCPlugin.
	NewHostStorageGRPCPlugin(impl HostStorageServer, pluginID string, logger *slog.Logger) plugin.Plugin

	// NewHostWeatherGRPCPlugin creates a new HostWeatherGRPCPlugin.
	NewHostWeatherGRPCPlugin(impl HostWeatherServer, logger *slog.Logger) plugin.Plugin

	// NewHostPluginsGRPCPlugin creates a new HostPluginsGRPCPlugin.
	NewHostPluginsGRPCPlugin(impl HostPluginsServer, logger *slog.Logger) plugin.Plugin

	// GetBrokerID extracts the broker ID from a dispensed broker info struct.
	GetBrokerID(brokerInfo interface{}) uint32
}
