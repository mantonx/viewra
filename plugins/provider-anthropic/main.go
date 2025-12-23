// Package main implements the Anthropic provider plugin for ViewRA.
// This plugin provides AI inference capabilities via Anthropic's Claude models.
package main

import (
	"context"

	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/pkg/plugin/sdk"
	"github.com/mantonx/viewra/plugins/provider-anthropic/internal"
)

func main() {
	// Use SDK logger for go-plugin compatibility
	hclogger, logger := sdk.NewLogger("provider-anthropic")

	anthropicPlugin := internal.NewPlugin(logger)

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: plugin.HandshakeConfig{
			ProtocolVersion:  1,
			MagicCookieKey:   "VIEWRA_PLUGIN",
			MagicCookieValue: "viewra-plugin-v1",
		},
		Plugins: map[string]plugin.Plugin{
			"core":     &PluginCoreGRPCPlugin{Impl: anthropicPlugin},
			"provider": &PluginProviderGRPCPlugin{Impl: anthropicPlugin.Provider()},
		},
		GRPCServer: plugin.DefaultGRPCServer,
		Logger:     hclogger,
	})
}

// PluginCoreGRPCPlugin implements plugin.GRPCPlugin for PluginCore service.
type PluginCoreGRPCPlugin struct {
	plugin.Plugin
	Impl pluginv1.PluginCoreServer
}

func (p *PluginCoreGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	pluginv1.RegisterPluginCoreServer(s, p.Impl)
	return nil
}

func (p *PluginCoreGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (any, error) {
	return pluginv1.NewPluginCoreClient(c), nil
}

// PluginProviderGRPCPlugin implements plugin.GRPCPlugin for PluginProvider service.
type PluginProviderGRPCPlugin struct {
	plugin.Plugin
	Impl pluginv1.PluginProviderServer
}

func (p *PluginProviderGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	pluginv1.RegisterPluginProviderServer(s, p.Impl)
	return nil
}

func (p *PluginProviderGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (any, error) {
	return pluginv1.NewPluginProviderClient(c), nil
}
