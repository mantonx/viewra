// Widget plugin support for ViewRA widget plugins.
//
// This file provides the WidgetPlugin interface and ServeWidget() helper
// for building widget plugins that provide HTTP routes and home screen widgets
// but don't enrich media items (like recommendations, continue watching, etc.).
//
// # Quick Start
//
// Create a widget plugin that implements the WidgetPlugin interface:
//
//	type MyWidget struct {
//	    sdk.Base
//	    storage *sdk.StorageClient
//	}
//
//	func (w *MyWidget) Initialize(ctx context.Context, dataDir string, config []byte, services *sdk.HostServices) error {
//	    w.storage = services.Storage
//	    return nil
//	}
//
//	func (w *MyWidget) Shutdown(ctx context.Context) error {
//	    return nil
//	}
//
//	func (w *MyWidget) GetSettingsSchema() ([]byte, error) {
//	    return sdk.NewSchema().
//	        Widgets([]sdk.Widget{...}).
//	        Build()
//	}
//
//	func (w *MyWidget) Configure(settings []byte) error {
//	    return nil
//	}
//
//	func (w *MyWidget) GetRoutes() []sdk.Route {
//	    return []sdk.Route{...}
//	}
//
//	func (w *MyWidget) HandleHTTP(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
//	    // Handle requests
//	}
//
//	func main() {
//	    hclogger, logger := sdk.NewLogger("my-widget")
//	    plugin := &MyWidget{}
//	    plugin.SetLogger(logger)
//	    sdk.ServeWidget(plugin, hclogger)
//	}
package sdk

import (
	"context"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

// WidgetPlugin is the interface that widget plugins must implement.
// Widget plugins provide HTTP routes and home screen widgets but don't enrich media.
//
// Plugin identity comes from plugin.yml manifest file, not code.
type WidgetPlugin interface {
	// mustEmbedBase ensures plugins embed sdk.Base
	mustEmbedBase()

	// Initialize is called when the plugin is loaded.
	// Config is the contents of config.yml passed by the host.
	// Services provides access to host services (storage, data, etc.) - may have nil fields.
	Initialize(ctx context.Context, dataDir string, config []byte, services *HostServices) error

	// Shutdown is called before the plugin is unloaded.
	// Use this to clean up any resources.
	Shutdown(ctx context.Context) error

	// GetSettingsSchema returns a JSON Schema describing the plugin's configurable settings.
	// Use Schema.Build() to generate this from your schema definition.
	// Include Widgets() to register home screen widgets.
	GetSettingsSchema() ([]byte, error)

	// Configure applies new settings to the plugin.
	Configure(settings []byte) error

	// GetRoutes returns HTTP routes this plugin exposes.
	GetRoutes() []Route

	// HandleHTTP handles a non-streaming HTTP request.
	HandleHTTP(ctx context.Context, req *HTTPRequest) (*HTTPResponse, error)
}

// --- gRPC Server Implementation ---

// widgetGRPCServer wraps a WidgetPlugin to implement the gRPC PluginCoreServer.
type widgetGRPCServer struct {
	pluginv1.UnimplementedPluginCoreServer
	impl     WidgetPlugin
	base     *Base
	broker   *plugin.GRPCBroker
	services *HostServices
}

func (s *widgetGRPCServer) Initialize(ctx context.Context, req *pluginv1.InitRequest) (*pluginv1.InitResponse, error) {
	s.base.Init(req.DataDir)

	// Connect to host services
	s.services = &HostServices{}
	s.connectHostServices(req)

	// Initialize the plugin
	if err := s.impl.Initialize(ctx, req.DataDir, req.Config, s.services); err != nil {
		return &pluginv1.InitResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &pluginv1.InitResponse{Success: true}, nil
}

func (s *widgetGRPCServer) connectHostServices(req *pluginv1.InitRequest) {
	logger := s.base.Log()

	// Storage service
	if req.HostStorageBrokerId > 0 {
		conn, err := s.broker.Dial(req.HostStorageBrokerId)
		if err != nil {
			logger.Error("failed to dial host storage", "error", err)
		} else {
			s.services.Storage = &StorageClient{client: pluginv1.NewHostStorageClient(conn)}
			logger.Debug("connected to host storage service")
		}
	}

	// Data service
	if req.HostDataBrokerId > 0 {
		conn, err := s.broker.Dial(req.HostDataBrokerId)
		if err != nil {
			logger.Error("failed to dial host data", "error", err)
		} else {
			s.services.Data = &DataClient{client: pluginv1.NewHostDataClient(conn)}
			logger.Debug("connected to host data service")
		}
	}

	// Weather service
	if req.HostWeatherBrokerId > 0 {
		conn, err := s.broker.Dial(req.HostWeatherBrokerId)
		if err != nil {
			logger.Error("failed to dial host weather", "error", err)
		} else {
			s.services.Weather = &WeatherClient{client: pluginv1.NewHostWeatherClient(conn)}
			logger.Debug("connected to host weather service")
		}
	}

	// Plugins service (for capability-based plugin discovery)
	if req.HostPluginsBrokerId > 0 {
		conn, err := s.broker.Dial(req.HostPluginsBrokerId)
		if err != nil {
			logger.Error("failed to dial host plugins", "error", err)
		} else {
			s.services.Plugins = NewPluginsClient(conn)
			logger.Debug("connected to host plugins service")
		}
	}

	// Ratings service (for user ratings access)
	if req.HostRatingsBrokerId > 0 {
		conn, err := s.broker.Dial(req.HostRatingsBrokerId)
		if err != nil {
			logger.Error("failed to dial host ratings", "error", err)
		} else {
			s.services.Ratings = NewRatingsClient(conn)
			logger.Debug("connected to host ratings service")
		}
	}
}

func (s *widgetGRPCServer) Shutdown(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.Empty, error) {
	if err := s.impl.Shutdown(ctx); err != nil {
		s.base.Log().Error("shutdown error", "error", err)
	}
	return &pluginv1.Empty{}, nil
}

func (s *widgetGRPCServer) HealthCheck(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.HealthStatus, error) {
	metrics := s.base.Metrics()
	return &pluginv1.HealthStatus{
		Status:        pluginv1.HealthStatus_HEALTHY,
		RequestsTotal: metrics.RequestsTotal,
		ErrorsTotal:   metrics.ErrorsTotal,
		AvgLatencyMs:  float64(metrics.AvgLatency.Milliseconds()),
	}, nil
}

func (s *widgetGRPCServer) GetSettingsSchema(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.SettingsSchema, error) {
	schema, err := s.impl.GetSettingsSchema()
	if err != nil {
		return nil, err
	}
	return &pluginv1.SettingsSchema{JsonSchema: schema}, nil
}

func (s *widgetGRPCServer) Configure(ctx context.Context, settings *pluginv1.Settings) (*pluginv1.ConfigureResponse, error) {
	if err := s.impl.Configure(settings.Json); err != nil {
		return &pluginv1.ConfigureResponse{Success: false, Error: err.Error()}, nil
	}
	return &pluginv1.ConfigureResponse{Success: true}, nil
}

func (s *widgetGRPCServer) GetSubscriptions(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.EventSubscriptions, error) {
	// Widget plugins don't subscribe to events by default
	return &pluginv1.EventSubscriptions{}, nil
}

func (s *widgetGRPCServer) OnEvent(ctx context.Context, event *pluginv1.Event) (*pluginv1.EventResponse, error) {
	// Widget plugins don't handle events by default
	return &pluginv1.EventResponse{Handled: false}, nil
}

func (s *widgetGRPCServer) GetRoutes(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.PluginRoutes, error) {
	routes := s.impl.GetRoutes()
	protoRoutes := make([]*pluginv1.PluginRoute, len(routes))
	for i, r := range routes {
		protoRoutes[i] = &pluginv1.PluginRoute{
			Path:        r.Path,
			Methods:     r.Methods,
			AdminOnly:   r.AdminOnly,
			Description: r.Description,
			AliasPath:   r.AliasPath,
		}
	}
	return &pluginv1.PluginRoutes{Routes: protoRoutes}, nil
}

func (s *widgetGRPCServer) HandleHTTP(ctx context.Context, req *pluginv1.PluginHTTPRequest) (*pluginv1.PluginHTTPResponse, error) {
	sdkReq := &HTTPRequest{
		Method:  req.Method,
		Path:    req.Path,
		Query:   req.Query,
		Headers: req.Headers,
		Body:    req.Body,
		UserID:  req.UserId,
	}

	resp, err := s.impl.HandleHTTP(ctx, sdkReq)
	if err != nil {
		return &pluginv1.PluginHTTPResponse{
			StatusCode:  500,
			ContentType: "application/json",
			Body:        []byte(`{"error":"` + err.Error() + `"}`),
		}, nil
	}

	return &pluginv1.PluginHTTPResponse{
		StatusCode:  int32(resp.StatusCode),
		ContentType: resp.ContentType,
		Headers:     resp.Headers,
		Body:        resp.Body,
	}, nil
}

func (s *widgetGRPCServer) HandleHTTPStream(stream pluginv1.PluginCore_HandleHTTPStreamServer) error {
	// Widget plugins don't support streaming by default
	return nil
}

// --- go-plugin integration ---

// WidgetCoreGRPCPlugin implements plugin.GRPCPlugin for WidgetPlugin.
type WidgetCoreGRPCPlugin struct {
	plugin.Plugin
	Impl   WidgetPlugin
	base   *Base
	broker *plugin.GRPCBroker
}

func (p *WidgetCoreGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	server := &widgetGRPCServer{
		impl:   p.Impl,
		base:   p.base,
		broker: broker,
	}
	pluginv1.RegisterPluginCoreServer(s, server)
	return nil
}

func (p *WidgetCoreGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return nil, nil // Client-side not needed for plugins
}

// --- ServeWidget ---

// ServeWidget starts a widget plugin server.
// Call this from your plugin's main() function.
//
// Example:
//
//	func main() {
//	    hclogger, logger := sdk.NewLogger("my-widget")
//	    p := NewMyWidget()
//	    p.SetLogger(logger)
//	    sdk.ServeWidget(p, hclogger)
//	}
func ServeWidget(impl WidgetPlugin, logger hclog.Logger) {
	base := &Base{}
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins: map[string]plugin.Plugin{
			"core":         &WidgetCoreGRPCPlugin{Impl: impl, base: base},
			"host_storage": &HostStorageGRPCPlugin{},
			"host_data":    &HostDataGRPCPlugin{},
			"host_weather": &HostWeatherGRPCPlugin{},
			"host_plugins": &HostPluginsGRPCPlugin{},
		},
		GRPCServer: plugin.DefaultGRPCServer,
		Logger:     logger,
	})
}
