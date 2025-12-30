package grpc

import (
	"context"
	"log/slog"

	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/host"
)

// PluginCorePlugin is the go-plugin implementation for the PluginCore service.
// This is used by the host to communicate with plugins.
type PluginCorePlugin struct {
	plugin.Plugin
	// Impl is only used when serving (plugin side)
	Impl pluginv1.PluginCoreServer
}

func (p *PluginCorePlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	pluginv1.RegisterPluginCoreServer(s, p.Impl)
	return nil
}

func (p *PluginCorePlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return pluginv1.NewPluginCoreClient(c), nil
}

// EnricherPlugin is the go-plugin implementation for the Enricher service.
type EnricherPlugin struct {
	plugin.Plugin
	// Impl is only used when serving (plugin side)
	Impl pluginv1.EnricherServer
}

func (p *EnricherPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	pluginv1.RegisterEnricherServer(s, p.Impl)
	return nil
}

func (p *EnricherPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return pluginv1.NewEnricherClient(c), nil
}

// HostDataPlugin is the go-plugin implementation for the HostData service.
// On the host side, this starts a gRPC server on a broker ID that the plugin can connect to.
type HostDataPlugin struct {
	plugin.Plugin
	Impl   pluginv1.HostDataServer
	Logger *slog.Logger
}

func (p *HostDataPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	// Plugin side - we don't serve, the host does
	return nil
}

func (p *HostDataPlugin) GRPCClient(
	ctx context.Context,
	broker *plugin.GRPCBroker,
	c *grpc.ClientConn,
) (interface{}, error) {
	// Host side - start a server on a broker ID that the plugin can connect to
	if p.Impl == nil {
		// No implementation provided - return nil (data service not available)
		return (*HostDataBrokerInfo)(nil), nil
	}

	// Get a unique broker ID
	brokerID := broker.NextId()

	// Start the data server on this broker ID with logging interceptor
	go broker.AcceptAndServe(brokerID, func(opts []grpc.ServerOption) *grpc.Server {
		if p.Logger != nil {
			opts = append(opts,
				grpc.UnaryInterceptor(LoggingInterceptor(p.Logger)),
				grpc.StreamInterceptor(LoggingStreamInterceptor(p.Logger)),
			)
		}
		s := grpc.NewServer(opts...)
		pluginv1.RegisterHostDataServer(s, p.Impl)
		return s
	})

	return &HostDataBrokerInfo{BrokerID: brokerID}, nil
}

// HostDataBrokerInfo contains the broker ID for connecting to the host data service.
type HostDataBrokerInfo struct {
	BrokerID uint32
}

// HostStoragePlugin is the go-plugin implementation for the HostStorage service.
// On the host side, this starts a gRPC server on a broker ID that the plugin can connect to.
type HostStoragePlugin struct {
	plugin.Plugin
	Impl     pluginv1.HostStorageServer
	PluginID string // The ID of the plugin that this storage is for
	Logger   *slog.Logger
}

func (p *HostStoragePlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	// Plugin side - we don't serve, the host does
	return nil
}

func (p *HostStoragePlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	// Host side - start a server on a broker ID that the plugin can connect to
	if p.Impl == nil {
		// No implementation provided - return nil (storage not available)
		return (*HostStorageBrokerInfo)(nil), nil
	}

	// Get a unique broker ID
	brokerID := broker.NextId()

	// Create a wrapper that injects the plugin ID into the context
	wrapper := &hostStorageContextWrapper{
		impl:     p.Impl,
		pluginID: p.PluginID,
	}

	// Start the storage server on this broker ID with logging interceptor
	go broker.AcceptAndServe(brokerID, func(opts []grpc.ServerOption) *grpc.Server {
		if p.Logger != nil {
			opts = append(opts,
				grpc.UnaryInterceptor(LoggingInterceptor(p.Logger)),
				grpc.StreamInterceptor(LoggingStreamInterceptor(p.Logger)),
			)
		}
		s := grpc.NewServer(opts...)
		pluginv1.RegisterHostStorageServer(s, wrapper)
		return s
	})

	return &HostStorageBrokerInfo{BrokerID: brokerID}, nil
}

// HostStorageBrokerInfo contains the broker ID for connecting to the host storage service.
type HostStorageBrokerInfo struct {
	BrokerID uint32
}

// hostStorageContextWrapper wraps HostStorageServer to inject plugin ID into context.
// This is necessary because each plugin connection needs its own context with the
// plugin ID so that storage operations are namespaced correctly.
type hostStorageContextWrapper struct {
	pluginv1.UnimplementedHostStorageServer
	impl     pluginv1.HostStorageServer
	pluginID string
}

func (w *hostStorageContextWrapper) KVGet(ctx context.Context, req *pluginv1.KVKey) (*pluginv1.KVValue, error) {
	ctx = host.ContextWithPluginID(ctx, w.pluginID)
	return w.impl.KVGet(ctx, req)
}

func (w *hostStorageContextWrapper) KVSet(ctx context.Context, req *pluginv1.KVEntry) (*pluginv1.Empty, error) {
	ctx = host.ContextWithPluginID(ctx, w.pluginID)
	return w.impl.KVSet(ctx, req)
}

func (w *hostStorageContextWrapper) KVDelete(ctx context.Context, req *pluginv1.KVKey) (*pluginv1.Empty, error) {
	ctx = host.ContextWithPluginID(ctx, w.pluginID)
	return w.impl.KVDelete(ctx, req)
}

func (w *hostStorageContextWrapper) KVList(ctx context.Context, req *pluginv1.KVListRequest) (*pluginv1.KVKeyList, error) {
	ctx = host.ContextWithPluginID(ctx, w.pluginID)
	return w.impl.KVList(ctx, req)
}

func (w *hostStorageContextWrapper) GetDatabasePath(ctx context.Context, req *pluginv1.Empty) (*pluginv1.DatabasePath, error) {
	ctx = host.ContextWithPluginID(ctx, w.pluginID)
	return w.impl.GetDatabasePath(ctx, req)
}

func (w *hostStorageContextWrapper) RegisterSchema(ctx context.Context, req *pluginv1.SchemaVersion) (*pluginv1.Empty, error) {
	ctx = host.ContextWithPluginID(ctx, w.pluginID)
	return w.impl.RegisterSchema(ctx, req)
}

func (w *hostStorageContextWrapper) GetDatabaseStats(ctx context.Context, req *pluginv1.Empty) (*pluginv1.DatabaseStats, error) {
	ctx = host.ContextWithPluginID(ctx, w.pluginID)
	return w.impl.GetDatabaseStats(ctx, req)
}

// SQL storage wrapper methods

func (w *hostStorageContextWrapper) ExecuteSQL(ctx context.Context, req *pluginv1.SQLRequest) (*pluginv1.SQLExecResult, error) {
	ctx = host.ContextWithPluginID(ctx, w.pluginID)
	return w.impl.ExecuteSQL(ctx, req)
}

func (w *hostStorageContextWrapper) QuerySQL(ctx context.Context, req *pluginv1.SQLRequest) (*pluginv1.SQLQueryResult, error) {
	ctx = host.ContextWithPluginID(ctx, w.pluginID)
	return w.impl.QuerySQL(ctx, req)
}

// Vector storage wrapper methods

func (w *hostStorageContextWrapper) VectorStoreEmbedding(ctx context.Context, req *pluginv1.VectorStoreRequest) (*pluginv1.Empty, error) {
	ctx = host.ContextWithPluginID(ctx, w.pluginID)
	return w.impl.VectorStoreEmbedding(ctx, req)
}

func (w *hostStorageContextWrapper) VectorStoreBatch(ctx context.Context, req *pluginv1.VectorStoreBatchRequest) (*pluginv1.Empty, error) {
	ctx = host.ContextWithPluginID(ctx, w.pluginID)
	return w.impl.VectorStoreBatch(ctx, req)
}

func (w *hostStorageContextWrapper) VectorSearch(ctx context.Context, req *pluginv1.VectorSearchRequest) (*pluginv1.VectorSearchResponse, error) {
	ctx = host.ContextWithPluginID(ctx, w.pluginID)
	return w.impl.VectorSearch(ctx, req)
}

func (w *hostStorageContextWrapper) VectorSearchText(ctx context.Context, req *pluginv1.VectorTextSearchRequest) (*pluginv1.VectorSearchResponse, error) {
	ctx = host.ContextWithPluginID(ctx, w.pluginID)
	return w.impl.VectorSearchText(ctx, req)
}

func (w *hostStorageContextWrapper) VectorGet(ctx context.Context, req *pluginv1.VectorQuery) (*pluginv1.VectorGetResponse, error) {
	ctx = host.ContextWithPluginID(ctx, w.pluginID)
	return w.impl.VectorGet(ctx, req)
}

func (w *hostStorageContextWrapper) VectorDelete(ctx context.Context, req *pluginv1.VectorQuery) (*pluginv1.Empty, error) {
	ctx = host.ContextWithPluginID(ctx, w.pluginID)
	return w.impl.VectorDelete(ctx, req)
}

func (w *hostStorageContextWrapper) VectorDeleteByType(ctx context.Context, req *pluginv1.VectorTypeQuery) (*pluginv1.VectorDeleteResponse, error) {
	ctx = host.ContextWithPluginID(ctx, w.pluginID)
	return w.impl.VectorDeleteByType(ctx, req)
}

func (w *hostStorageContextWrapper) VectorCount(ctx context.Context, req *pluginv1.VectorTypeQuery) (*pluginv1.VectorCountResponse, error) {
	ctx = host.ContextWithPluginID(ctx, w.pluginID)
	return w.impl.VectorCount(ctx, req)
}

// PluginProviderPlugin is the go-plugin implementation for the PluginProvider service.
// This allows the host to call the plugin's AI provider methods (chat, embeddings, etc.).
// Provider plugins (e.g., provider-ollama, provider-openai) implement this service.
type PluginProviderPlugin struct {
	plugin.Plugin
}

func (p *PluginProviderPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	// Plugin serves this, host doesn't
	return nil
}

func (p *PluginProviderPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return pluginv1.NewPluginProviderClient(c), nil
}

// HostWeatherPlugin is the go-plugin implementation for the HostWeather service.
// On the host side, this starts a gRPC server on a broker ID that the plugin can connect to.
// This provides weather context for AI search query enrichment.
type HostWeatherPlugin struct {
	plugin.Plugin
	Impl   pluginv1.HostWeatherServer
	Logger *slog.Logger
}

func (p *HostWeatherPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	// Plugin side - we don't serve, the host does
	return nil
}

func (p *HostWeatherPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	// Host side - start a server on a broker ID that the plugin can connect to
	if p.Impl == nil {
		// No implementation provided - return nil (weather not available)
		return (*HostWeatherBrokerInfo)(nil), nil
	}

	// Get a unique broker ID
	brokerID := broker.NextId()

	// Start the weather server on this broker ID with logging interceptor
	go broker.AcceptAndServe(brokerID, func(opts []grpc.ServerOption) *grpc.Server {
		if p.Logger != nil {
			opts = append(opts,
				grpc.UnaryInterceptor(LoggingInterceptor(p.Logger)),
				grpc.StreamInterceptor(LoggingStreamInterceptor(p.Logger)),
			)
		}
		s := grpc.NewServer(opts...)
		pluginv1.RegisterHostWeatherServer(s, p.Impl)
		return s
	})

	return &HostWeatherBrokerInfo{BrokerID: brokerID}, nil
}

// HostWeatherBrokerInfo contains the broker ID for connecting to the host weather service.
type HostWeatherBrokerInfo struct {
	BrokerID uint32
}

// HostPluginsPlugin is the go-plugin implementation for the HostPlugins service.
// On the host side, this starts a gRPC server on a broker ID that the plugin can connect to.
// This provides capability-based plugin discovery for inter-plugin communication.
type HostPluginsPlugin struct {
	plugin.Plugin
	Impl   pluginv1.HostPluginsServer
	Logger *slog.Logger
}

func (p *HostPluginsPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	// Plugin side - we don't serve, the host does
	return nil
}

func (p *HostPluginsPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	// Host side - start a server on a broker ID that the plugin can connect to
	if p.Impl == nil {
		// No implementation provided - return nil (plugins service not available)
		return (*HostPluginsBrokerInfo)(nil), nil
	}

	// Get a unique broker ID
	brokerID := broker.NextId()

	// Start the plugins server on this broker ID with logging interceptor
	go broker.AcceptAndServe(brokerID, func(opts []grpc.ServerOption) *grpc.Server {
		if p.Logger != nil {
			opts = append(opts,
				grpc.UnaryInterceptor(LoggingInterceptor(p.Logger)),
				grpc.StreamInterceptor(LoggingStreamInterceptor(p.Logger)),
			)
		}
		s := grpc.NewServer(opts...)
		pluginv1.RegisterHostPluginsServer(s, p.Impl)
		return s
	})

	return &HostPluginsBrokerInfo{BrokerID: brokerID}, nil
}

// HostPluginsBrokerInfo contains the broker ID for connecting to the host plugins service.
type HostPluginsBrokerInfo struct {
	BrokerID uint32
}
