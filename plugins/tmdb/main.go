// Package main implements the TMDb enricher plugin for ViewRA.
// This plugin fetches movie and TV metadata from The Movie Database (TMDb) API.
package main

import (
	"context"
	"log/slog"

	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/pkg/plugin/sdk"
	"github.com/mantonx/viewra/plugins/tmdb/internal"
)

func main() {
	// Use SDK logger for go-plugin compatibility - logs will be forwarded to host
	hclogger, logger := sdk.NewLogger("tmdb")

	tmdbPlugin := internal.NewTMDbPlugin(logger)

	// Create the plugin wrapper that will capture the broker for host services
	corePlugin := &PluginCoreGRPCPlugin{
		Impl:   tmdbPlugin,
		plugin: tmdbPlugin,
		logger: logger,
	}

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: plugin.HandshakeConfig{
			ProtocolVersion:  1,
			MagicCookieKey:   "VIEWRA_PLUGIN",
			MagicCookieValue: "viewra-plugin-v1",
		},
		Plugins: map[string]plugin.Plugin{
			"core":         corePlugin,
			"enricher":     &EnricherGRPCPlugin{Impl: tmdbPlugin},
			"host_storage": &HostStorageGRPCPlugin{},
		},
		GRPCServer: plugin.DefaultGRPCServer,
		Logger:     hclogger, // Pass hclog to go-plugin for log forwarding
	})
}

// PluginCoreGRPCPlugin implements plugin.GRPCPlugin for PluginCore service.
// It captures the broker to enable host service connections.
type PluginCoreGRPCPlugin struct {
	plugin.Plugin
	Impl   pluginv1.PluginCoreServer
	plugin *internal.TMDbPlugin
	logger *slog.Logger
	broker *plugin.GRPCBroker
}

func (p *PluginCoreGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	// Store the broker for later use when connecting to host services
	p.broker = broker

	// Wrap the implementation to intercept Initialize and set up host services
	wrapper := &pluginCoreWrapper{
		PluginCoreServer: p.Impl,
		plugin:           p.plugin,
		broker:           broker,
		logger:           p.logger,
	}
	pluginv1.RegisterPluginCoreServer(s, wrapper)
	return nil
}

func (p *PluginCoreGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return pluginv1.NewPluginCoreClient(c), nil
}

// pluginCoreWrapper wraps PluginCoreServer to intercept Initialize and set up host services.
type pluginCoreWrapper struct {
	pluginv1.PluginCoreServer
	plugin *internal.TMDbPlugin
	broker *plugin.GRPCBroker
	logger *slog.Logger
}

func (w *pluginCoreWrapper) Initialize(ctx context.Context, req *pluginv1.InitRequest) (*pluginv1.InitResponse, error) {
	// Connect to host storage service if broker ID was provided
	if req.HostStorageBrokerId > 0 {
		w.logger.Debug("connecting to host storage service", "broker_id", req.HostStorageBrokerId)

		conn, err := w.broker.Dial(req.HostStorageBrokerId)
		if err != nil {
			w.logger.Error("failed to dial host storage", "error", err)
		} else {
			storageClient := pluginv1.NewHostStorageClient(conn)
			w.plugin.SetStorageClient(storageClient)
			w.logger.Info("connected to host storage service")
		}
	} else {
		w.logger.Debug("host storage not available (no broker ID)")
	}

	// Forward to the actual implementation
	return w.PluginCoreServer.Initialize(ctx, req)
}

// HostStorageGRPCPlugin implements plugin.GRPCPlugin for HostStorage service.
// This allows the plugin to access host-provided storage.
type HostStorageGRPCPlugin struct {
	plugin.Plugin
}

func (p *HostStorageGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	// We don't serve this - the host does
	return nil
}

func (p *HostStorageGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return pluginv1.NewHostStorageClient(c), nil
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

func (p *EnricherGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return pluginv1.NewEnricherClient(c), nil
}
