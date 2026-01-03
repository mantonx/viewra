package grpc

import (
	"log/slog"

	"github.com/hashicorp/go-plugin"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/manager"
)

// Factory implements the manager.PluginFactory interface.
// It creates go-plugin Plugin instances for the plugin manager.
type Factory struct{}

// NewFactory creates a new plugin factory.
func NewFactory() *Factory {
	return &Factory{}
}

// NewPluginCoreGRPCPlugin creates a new PluginCorePlugin.
func (f *Factory) NewPluginCoreGRPCPlugin() plugin.Plugin {
	return &PluginCorePlugin{}
}

// NewEnricherGRPCPlugin creates a new EnricherPlugin.
func (f *Factory) NewEnricherGRPCPlugin() plugin.Plugin {
	return &EnricherPlugin{}
}

// NewPluginProviderGRPCPlugin creates a new PluginProviderPlugin.
func (f *Factory) NewPluginProviderGRPCPlugin() plugin.Plugin {
	return &PluginProviderPlugin{}
}

// NewHostDataGRPCPlugin creates a new HostDataPlugin.
// The impl parameter is a marker interface from manager package.
func (f *Factory) NewHostDataGRPCPlugin(impl manager.HostDataServer, logger *slog.Logger) plugin.Plugin {
	if impl == nil {
		return nil
	}
	server, ok := impl.(pluginv1.HostDataServer)
	if !ok {
		return nil
	}
	return &HostDataPlugin{
		Impl:   server,
		Logger: logger,
	}
}

// NewHostStorageGRPCPlugin creates a new HostStoragePlugin.
// The impl parameter is a marker interface from manager package.
func (f *Factory) NewHostStorageGRPCPlugin(impl manager.HostStorageServer, pluginID string, logger *slog.Logger) plugin.Plugin {
	if impl == nil {
		return nil
	}
	server, ok := impl.(pluginv1.HostStorageServer)
	if !ok {
		return nil
	}
	return &HostStoragePlugin{
		Impl:     server,
		PluginID: pluginID,
		Logger:   logger,
	}
}

// NewHostWeatherGRPCPlugin creates a new HostWeatherPlugin.
// The impl parameter is a marker interface from manager package.
func (f *Factory) NewHostWeatherGRPCPlugin(impl manager.HostWeatherServer, logger *slog.Logger) plugin.Plugin {
	if impl == nil {
		return nil
	}
	server, ok := impl.(pluginv1.HostWeatherServer)
	if !ok {
		return nil
	}
	return &HostWeatherPlugin{
		Impl:   server,
		Logger: logger,
	}
}

// NewHostPluginsGRPCPlugin creates a new HostPluginsPlugin.
// The impl parameter is a marker interface from manager package.
func (f *Factory) NewHostPluginsGRPCPlugin(impl manager.HostPluginsServer, logger *slog.Logger) plugin.Plugin {
	if impl == nil {
		return nil
	}
	server, ok := impl.(pluginv1.HostPluginsServer)
	if !ok {
		return nil
	}
	return &HostPluginsPlugin{
		Impl:   server,
		Logger: logger,
	}
}

// NewHostRatingsGRPCPlugin creates a new HostRatingsPlugin.
// The impl parameter is a marker interface from manager package.
func (f *Factory) NewHostRatingsGRPCPlugin(impl manager.HostRatingsServer, logger *slog.Logger) plugin.Plugin {
	if impl == nil {
		return nil
	}
	server, ok := impl.(pluginv1.HostRatingsServer)
	if !ok {
		return nil
	}
	return &HostRatingsPlugin{
		Impl:   server,
		Logger: logger,
	}
}

// NewVectorSearchGRPCPlugin creates a new VectorSearchPlugin for client-side dispensing.
// This is a client-only plugin - the plugin side serves VectorSearch, host side gets client.
func (f *Factory) NewVectorSearchGRPCPlugin() plugin.Plugin {
	return &VectorSearchPlugin{}
}

// NewTrendingProviderGRPCPlugin creates a new TrendingProviderPlugin for client-side dispensing.
// This is a client-only plugin - the plugin side serves TrendingProviderService, host side gets client.
func (f *Factory) NewTrendingProviderGRPCPlugin() plugin.Plugin {
	return &TrendingProviderPlugin{}
}

// GetBrokerID extracts the broker ID from a dispensed broker info struct.
func (f *Factory) GetBrokerID(brokerInfo interface{}) uint32 {
	if brokerInfo == nil {
		return 0
	}

	switch info := brokerInfo.(type) {
	case *HostDataBrokerInfo:
		if info != nil {
			return info.BrokerID
		}
	case *HostStorageBrokerInfo:
		if info != nil {
			return info.BrokerID
		}
	case *HostWeatherBrokerInfo:
		if info != nil {
			return info.BrokerID
		}
	case *HostPluginsBrokerInfo:
		if info != nil {
			return info.BrokerID
		}
	case *HostRatingsBrokerInfo:
		if info != nil {
			return info.BrokerID
		}
	}

	return 0
}
