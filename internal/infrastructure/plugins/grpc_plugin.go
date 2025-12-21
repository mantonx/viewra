package plugins

import (
	"context"
	"log/slog"

	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

// PluginCoreGRPCPlugin is the go-plugin implementation for the PluginCore service.
// This is used by the host to communicate with plugins.
type PluginCoreGRPCPlugin struct {
	plugin.Plugin
	// Impl is only used when serving (plugin side)
	Impl pluginv1.PluginCoreServer
}

func (p *PluginCoreGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	pluginv1.RegisterPluginCoreServer(s, p.Impl)
	return nil
}

func (p *PluginCoreGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return pluginv1.NewPluginCoreClient(c), nil
}

// EnricherGRPCPlugin is the go-plugin implementation for the Enricher service.
type EnricherGRPCPlugin struct {
	plugin.Plugin
	// Impl is only used when serving (plugin side)
	Impl pluginv1.EnricherServer
}

func (p *EnricherGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	pluginv1.RegisterEnricherServer(s, p.Impl)
	return nil
}

func (p *EnricherGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return pluginv1.NewEnricherClient(c), nil
}

// HostDataGRPCPlugin is the go-plugin implementation for the HostData service.
// On the host side, this starts a gRPC server on a broker ID that the plugin can connect to.
type HostDataGRPCPlugin struct {
	plugin.Plugin
	Impl   pluginv1.HostDataServer
	Logger *slog.Logger
}

func (p *HostDataGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	// Plugin side - we don't serve, the host does
	return nil
}

func (p *HostDataGRPCPlugin) GRPCClient(
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

// HostStorageGRPCPlugin is the go-plugin implementation for the HostStorage service.
// On the host side, this starts a gRPC server on a broker ID that the plugin can connect to.
type HostStorageGRPCPlugin struct {
	plugin.Plugin
	Impl     pluginv1.HostStorageServer
	PluginID string // The ID of the plugin that this storage is for
	Logger   *slog.Logger
}

func (p *HostStorageGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	// Plugin side - we don't serve, the host does
	return nil
}

func (p *HostStorageGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
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
type hostStorageContextWrapper struct {
	pluginv1.UnimplementedHostStorageServer
	impl     pluginv1.HostStorageServer
	pluginID string
}

func (w *hostStorageContextWrapper) KVGet(ctx context.Context, req *pluginv1.KVKey) (*pluginv1.KVValue, error) {
	ctx = ContextWithPluginID(ctx, w.pluginID)
	return w.impl.KVGet(ctx, req)
}

func (w *hostStorageContextWrapper) KVSet(ctx context.Context, req *pluginv1.KVEntry) (*pluginv1.Empty, error) {
	ctx = ContextWithPluginID(ctx, w.pluginID)
	return w.impl.KVSet(ctx, req)
}

func (w *hostStorageContextWrapper) KVDelete(ctx context.Context, req *pluginv1.KVKey) (*pluginv1.Empty, error) {
	ctx = ContextWithPluginID(ctx, w.pluginID)
	return w.impl.KVDelete(ctx, req)
}

func (w *hostStorageContextWrapper) KVList(ctx context.Context, req *pluginv1.KVListRequest) (*pluginv1.KVKeyList, error) {
	ctx = ContextWithPluginID(ctx, w.pluginID)
	return w.impl.KVList(ctx, req)
}

func (w *hostStorageContextWrapper) GetDatabasePath(ctx context.Context, req *pluginv1.Empty) (*pluginv1.DatabasePath, error) {
	ctx = ContextWithPluginID(ctx, w.pluginID)
	return w.impl.GetDatabasePath(ctx, req)
}

func (w *hostStorageContextWrapper) RegisterSchema(ctx context.Context, req *pluginv1.SchemaVersion) (*pluginv1.Empty, error) {
	ctx = ContextWithPluginID(ctx, w.pluginID)
	return w.impl.RegisterSchema(ctx, req)
}

func (w *hostStorageContextWrapper) GetDatabaseStats(ctx context.Context, req *pluginv1.Empty) (*pluginv1.DatabaseStats, error) {
	ctx = ContextWithPluginID(ctx, w.pluginID)
	return w.impl.GetDatabaseStats(ctx, req)
}

// HostLLMGRPCPlugin is the go-plugin implementation for the HostLLM service.
// On the host side, this starts a gRPC server on a broker ID that the plugin can connect to.
type HostLLMGRPCPlugin struct {
	plugin.Plugin
	Impl   pluginv1.HostLLMServer
	Logger *slog.Logger
}

func (p *HostLLMGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	// Plugin side - we don't serve, the host does
	return nil
}

func (p *HostLLMGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	// Host side - start a server on a broker ID that the plugin can connect to
	if p.Impl == nil {
		// No implementation provided - return nil (LLM not available)
		return (*HostLLMBrokerInfo)(nil), nil
	}

	// Get a unique broker ID
	brokerID := broker.NextId()

	// Start the LLM server on this broker ID with logging interceptor
	go broker.AcceptAndServe(brokerID, func(opts []grpc.ServerOption) *grpc.Server {
		if p.Logger != nil {
			opts = append(opts,
				grpc.UnaryInterceptor(LoggingInterceptor(p.Logger)),
				grpc.StreamInterceptor(LoggingStreamInterceptor(p.Logger)),
			)
		}
		s := grpc.NewServer(opts...)
		pluginv1.RegisterHostLLMServer(s, p.Impl)
		return s
	})

	return &HostLLMBrokerInfo{BrokerID: brokerID}, nil
}

// HostLLMBrokerInfo contains the broker ID for connecting to the host LLM service.
type HostLLMBrokerInfo struct {
	BrokerID uint32
}

// HostEmbeddingsGRPCPlugin is the go-plugin implementation for the HostEmbeddings service.
// On the host side, this starts a gRPC server on a broker ID that the plugin can connect to.
type HostEmbeddingsGRPCPlugin struct {
	plugin.Plugin
	Impl   pluginv1.HostEmbeddingsServer
	Logger *slog.Logger
}

func (p *HostEmbeddingsGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	// Plugin side - we don't serve, the host does
	return nil
}

func (p *HostEmbeddingsGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	// Host side - start a server on a broker ID that the plugin can connect to
	if p.Impl == nil {
		// No implementation provided - return nil (embeddings not available)
		return (*HostEmbeddingsBrokerInfo)(nil), nil
	}

	// Get a unique broker ID
	brokerID := broker.NextId()

	// Start the embeddings server on this broker ID with logging interceptor
	go broker.AcceptAndServe(brokerID, func(opts []grpc.ServerOption) *grpc.Server {
		if p.Logger != nil {
			opts = append(opts,
				grpc.UnaryInterceptor(LoggingInterceptor(p.Logger)),
				grpc.StreamInterceptor(LoggingStreamInterceptor(p.Logger)),
			)
		}
		s := grpc.NewServer(opts...)
		pluginv1.RegisterHostEmbeddingsServer(s, p.Impl)
		return s
	})

	return &HostEmbeddingsBrokerInfo{BrokerID: brokerID}, nil
}

// HostEmbeddingsBrokerInfo contains the broker ID for connecting to the host embeddings service.
type HostEmbeddingsBrokerInfo struct {
	BrokerID uint32
}

// AISearchGRPCPlugin is the go-plugin implementation for the AISearch service.
// This allows the host to call the plugin's semantic search methods.
type AISearchGRPCPlugin struct {
	plugin.Plugin
}

func (p *AISearchGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	// Plugin serves this, host doesn't
	return nil
}

func (p *AISearchGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return pluginv1.NewAISearchClient(c), nil
}
