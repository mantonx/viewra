// Package plugins provides HashiCorp go-plugin integration helpers
package plugins

import (
	"net/rpc"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
)

// ViewraPlugin is the implementation of plugin.Plugin interface for go-plugin
type ViewraPlugin struct {
	Impl Implementation
}

// Server returns the RPC server for HashiCorp go-plugin
func (p *ViewraPlugin) Server(*plugin.MuxBroker) (interface{}, error) {
	return &GRPCServer{Impl: p.Impl}, nil
}

// Client returns the RPC client for HashiCorp go-plugin
func (p *ViewraPlugin) Client(b *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &GRPCClient{client: c}, nil
}

// GRPCServer wraps the plugin implementation for gRPC
type GRPCServer struct {
	Impl Implementation
}

// GRPCClient wraps the gRPC client
type GRPCClient struct {
	client *rpc.Client
}

// Handshake configurations for go-plugin
var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "VIEWRA_PLUGIN",
	MagicCookieValue: "viewra",
}

// PluginMap is the map of plugins we can dispense
var PluginMap = map[string]plugin.Plugin{
	"viewra": &ViewraPlugin{},
}

// StartPlugin is a helper function for plugin main() functions
func StartPlugin(impl Implementation) {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins: map[string]plugin.Plugin{
			"viewra": &ViewraPlugin{Impl: impl},
		},
	})
}

// Logger adapter to satisfy the plugin Logger interface
type HCLogAdapter struct {
	logger hclog.Logger
}

func (h *HCLogAdapter) Debug(msg string, args ...interface{}) {
	h.logger.Debug(msg, args...)
}

func (h *HCLogAdapter) Info(msg string, args ...interface{}) {
	h.logger.Info(msg, args...)
}

func (h *HCLogAdapter) Warn(msg string, args ...interface{}) {
	h.logger.Warn(msg, args...)
}

func (h *HCLogAdapter) Error(msg string, args ...interface{}) {
	h.logger.Error(msg, args...)
}

func (h *HCLogAdapter) With(args ...interface{}) hclog.Logger {
	return h.logger.With(args...)
}

// Context builder helper
func BuildPluginContext(databaseURL, hostServiceAddr, pluginBasePath, logLevel, basePath string, logger Logger) *PluginContext {
	return &PluginContext{
		DatabaseURL:     databaseURL,
		HostServiceAddr: hostServiceAddr,
		PluginBasePath:  pluginBasePath,
		LogLevel:        logLevel,
		BasePath:        basePath,
		Logger:          logger,
	}
} 