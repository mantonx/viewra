// Package main implements the AI Search plugin for ViewRA.
// This plugin provides semantic search and media indexing capabilities.
package main

import (
	"context"
	"log/slog"

	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/pkg/plugin/sdk"
	"github.com/mantonx/viewra/plugins/ai-search/internal"
)

func main() {
	// Use SDK logger for go-plugin compatibility - logs will be forwarded to host
	hclogger, logger := sdk.NewLogger("ai-search")

	aiSearchPlugin := internal.NewAISearchPlugin(logger)

	// Create the plugin wrapper that will capture the broker for host services
	corePlugin := &PluginCoreGRPCPlugin{
		Impl:   aiSearchPlugin,
		plugin: aiSearchPlugin,
		logger: logger,
	}

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: plugin.HandshakeConfig{
			ProtocolVersion:  1,
			MagicCookieKey:   "VIEWRA_PLUGIN",
			MagicCookieValue: "viewra-plugin-v1",
		},
		Plugins: map[string]plugin.Plugin{
			"core":            corePlugin,
			"enricher":        &EnricherGRPCPlugin{Impl: aiSearchPlugin},
			"ai_search":       &AISearchGRPCPlugin{Impl: aiSearchPlugin},
			"host_llm":        &HostLLMGRPCPlugin{},
			"host_embeddings": &HostEmbeddingsGRPCPlugin{},
			"host_data":       &HostDataGRPCPlugin{},
			"host_weather":    &HostWeatherGRPCPlugin{},
		},
		GRPCServer: plugin.DefaultGRPCServer,
		Logger:     hclogger, // Pass hclog to go-plugin for proper log forwarding
	})
}

// PluginCoreGRPCPlugin implements plugin.GRPCPlugin for PluginCore service.
type PluginCoreGRPCPlugin struct {
	plugin.Plugin
	Impl   pluginv1.PluginCoreServer
	plugin *internal.AISearchPlugin
	logger *slog.Logger
	broker *plugin.GRPCBroker
}

func (p *PluginCoreGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	p.broker = broker
	wrapper := &pluginCoreWrapper{
		PluginCoreServer: p.Impl,
		plugin:           p.plugin,
		broker:           broker,
		logger:           p.logger,
	}
	pluginv1.RegisterPluginCoreServer(s, wrapper)
	return nil
}

func (p *PluginCoreGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (any, error) {
	return pluginv1.NewPluginCoreClient(c), nil
}

// pluginCoreWrapper wraps PluginCoreServer to intercept Initialize.
type pluginCoreWrapper struct {
	pluginv1.PluginCoreServer
	plugin *internal.AISearchPlugin
	broker *plugin.GRPCBroker
	logger *slog.Logger
}

func (w *pluginCoreWrapper) Initialize(ctx context.Context, req *pluginv1.InitRequest) (*pluginv1.InitResponse, error) {
	// Connect to host LLM service
	if req.HostLlmBrokerId > 0 {
		w.logger.Debug("connecting to host LLM service", "broker_id", req.HostLlmBrokerId)
		conn, err := w.broker.Dial(req.HostLlmBrokerId)
		if err != nil {
			w.logger.Error("failed to dial host LLM", "error", err)
		} else {
			w.plugin.SetLLMClient(pluginv1.NewHostLLMClient(conn))
			w.logger.Info("connected to host LLM service")
		}
	}

	// Connect to host embeddings service
	if req.HostEmbeddingsBrokerId > 0 {
		w.logger.Debug("connecting to host embeddings service", "broker_id", req.HostEmbeddingsBrokerId)
		conn, err := w.broker.Dial(req.HostEmbeddingsBrokerId)
		if err != nil {
			w.logger.Error("failed to dial host embeddings", "error", err)
		} else {
			w.plugin.SetEmbeddingsClient(pluginv1.NewHostEmbeddingsClient(conn))
			w.logger.Info("connected to host embeddings service")
		}
	}

	// Connect to host data service
	if req.HostDataBrokerId > 0 {
		w.logger.Debug("connecting to host data service", "broker_id", req.HostDataBrokerId)
		conn, err := w.broker.Dial(req.HostDataBrokerId)
		if err != nil {
			w.logger.Error("failed to dial host data", "error", err)
		} else {
			w.plugin.SetDataClient(pluginv1.NewHostDataClient(conn))
			w.logger.Info("connected to host data service")
		}
	}

	// Connect to host weather service (for location-based context enrichment)
	if req.HostWeatherBrokerId > 0 {
		w.logger.Debug("connecting to host weather service", "broker_id", req.HostWeatherBrokerId)
		conn, err := w.broker.Dial(req.HostWeatherBrokerId)
		if err != nil {
			w.logger.Error("failed to dial host weather", "error", err)
		} else {
			w.plugin.SetWeatherClient(pluginv1.NewHostWeatherClient(conn))
			w.logger.Info("connected to host weather service")
		}
	}

	return w.PluginCoreServer.Initialize(ctx, req)
}

// EnricherGRPCPlugin implements plugin.GRPCPlugin for Enricher service.
type EnricherGRPCPlugin struct {
	plugin.Plugin
	Impl pluginv1.EnricherServer
}

func (p *EnricherGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	pluginv1.RegisterEnricherServer(s, p.Impl)
	return nil
}

func (p *EnricherGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (any, error) {
	return pluginv1.NewEnricherClient(c), nil
}

// HostLLMGRPCPlugin implements plugin.GRPCPlugin for HostLLM service.
type HostLLMGRPCPlugin struct {
	plugin.Plugin
}

func (p *HostLLMGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	return nil // Host serves this
}

func (p *HostLLMGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (any, error) {
	return pluginv1.NewHostLLMClient(c), nil
}

// HostEmbeddingsGRPCPlugin implements plugin.GRPCPlugin for HostEmbeddings service.
type HostEmbeddingsGRPCPlugin struct {
	plugin.Plugin
}

func (p *HostEmbeddingsGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	return nil // Host serves this
}

func (p *HostEmbeddingsGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (any, error) {
	return pluginv1.NewHostEmbeddingsClient(c), nil
}

// HostDataGRPCPlugin implements plugin.GRPCPlugin for HostData service.
type HostDataGRPCPlugin struct {
	plugin.Plugin
}

func (p *HostDataGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	return nil // Host serves this
}

func (p *HostDataGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (any, error) {
	return pluginv1.NewHostDataClient(c), nil
}

// AISearchGRPCPlugin implements plugin.GRPCPlugin for AISearch service.
type AISearchGRPCPlugin struct {
	plugin.Plugin
	Impl pluginv1.AISearchServer
}

func (p *AISearchGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	pluginv1.RegisterAISearchServer(s, p.Impl)
	return nil
}

func (p *AISearchGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (any, error) {
	return pluginv1.NewAISearchClient(c), nil
}

// HostWeatherGRPCPlugin implements plugin.GRPCPlugin for HostWeather service.
type HostWeatherGRPCPlugin struct {
	plugin.Plugin
}

func (p *HostWeatherGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	return nil // Host serves this
}

func (p *HostWeatherGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (any, error) {
	return pluginv1.NewHostWeatherClient(c), nil
}
