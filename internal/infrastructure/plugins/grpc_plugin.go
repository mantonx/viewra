package plugins

import (
	"context"

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
// This allows plugins to access host data.
type HostDataGRPCPlugin struct {
	plugin.Plugin
	Impl pluginv1.HostDataServer
}

func (p *HostDataGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	pluginv1.RegisterHostDataServer(s, p.Impl)
	return nil
}

func (p *HostDataGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return pluginv1.NewHostDataClient(c), nil
}

// HostStorageGRPCPlugin is the go-plugin implementation for the HostStorage service.
type HostStorageGRPCPlugin struct {
	plugin.Plugin
	Impl pluginv1.HostStorageServer
}

func (p *HostStorageGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	pluginv1.RegisterHostStorageServer(s, p.Impl)
	return nil
}

func (p *HostStorageGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return pluginv1.NewHostStorageClient(c), nil
}
