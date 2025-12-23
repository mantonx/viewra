// Package sdk provides utilities for building ViewRA plugins.
//
// The SDK provides common functionality that all plugins need:
//   - Logging with request ID tracking
//   - Metrics collection
//   - Data directory access
//   - Settings schema building
//
// # Plugin Structure
//
// All plugins embed sdk.Base to get common functionality:
//
//	type MyPlugin struct {
//	    sdk.Base
//	    // your fields
//	}
//
// # Logging
//
// Use sdk.NewLogger in your main() to create properly configured loggers:
//
//	func main() {
//	    hclogger, logger := sdk.NewLogger("my-plugin")
//	    plugin := NewMyPlugin(logger)
//	    // ... serve plugin
//	}
//
// Then use p.Log() or p.LogWithContext(ctx) in your plugin code:
//
//	func (p *MyPlugin) DoSomething(ctx context.Context) {
//	    p.LogWithContext(ctx).Info("doing something", "key", "value")
//	}
//
// # Data Directory
//
// Plugins can store persistent data in their data directory:
//
//	func (p *MyPlugin) Initialize(ctx context.Context, dataDir string, config []byte) error {
//	    cachePath := filepath.Join(p.DataDir(), "cache.db")
//	    // ... use cache
//	}
//
// # Metrics
//
// Track request metrics for health monitoring:
//
//	func (p *MyPlugin) HandleRequest(ctx context.Context) error {
//	    start := time.Now()
//	    defer func() {
//	        p.RecordRequest(time.Since(start))
//	    }()
//	    // ... handle request
//	}
package sdk

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"google.golang.org/grpc/metadata"
)

// Base provides common functionality for all plugins.
// Plugin structs must embed this type to get logging, metrics, and data directory access.
//
// Example:
//
//	type MyPlugin struct {
//	    sdk.Base
//	    apiKey string
//	}
//
//	func NewMyPlugin(logger *slog.Logger) *MyPlugin {
//	    p := &MyPlugin{}
//	    p.SetLogger(logger)
//	    return p
//	}
type Base struct {
	logger  *slog.Logger
	dataDir string

	// Metrics
	requestsTotal atomic.Int64
	errorsTotal   atomic.Int64
	totalLatency  atomic.Int64 // Nanoseconds
	requestCount  atomic.Int64 // For avg latency calculation
}

// mustEmbedBase forces plugins to embed Base.
// This is unexported, so plugins must embed Base to compile.
func (*Base) mustEmbedBase() {}

// Init initializes the base plugin functionality.
// Called automatically by the SDK during plugin initialization.
// Use InitWithLogger for proper go-plugin log forwarding.
func (b *Base) Init(dataDir string) {
	b.dataDir = dataDir
	// Use default slog logger - prefer InitWithLogger for proper go-plugin integration
	if b.logger == nil {
		b.logger = slog.Default()
	}
}

// InitWithLogger initializes the base plugin with a specific logger.
// Use sdk.NewLogger() to create a logger that properly forwards to the host.
//
// Example:
//
//	func (p *MyPlugin) Initialize(ctx context.Context, dataDir string, config []byte) error {
//	    _, logger := sdk.NewLogger("my-plugin")
//	    p.InitWithLogger(dataDir, logger)
//	    return nil
//	}
func (b *Base) InitWithLogger(dataDir string, logger *slog.Logger) {
	b.dataDir = dataDir
	b.logger = logger
}

// SetLogger sets the logger for this plugin.
// Call this early in your plugin initialization.
//
// Example:
//
//	func NewMyPlugin(logger *slog.Logger) *MyPlugin {
//	    p := &MyPlugin{}
//	    p.SetLogger(logger)
//	    return p
//	}
func (b *Base) SetLogger(logger *slog.Logger) {
	b.logger = logger
}

// Log returns the plugin's logger.
// Use this for logging without request context.
//
// Example:
//
//	p.Log().Info("plugin started", "version", "1.0.0")
func (b *Base) Log() *slog.Logger {
	if b.logger == nil {
		b.logger = slog.Default()
	}
	return b.logger
}

// LogWithContext returns a logger with the request ID from the context.
// Use this when handling requests to enable request tracing.
//
// Example:
//
//	func (p *MyPlugin) HandleRequest(ctx context.Context, req *Request) error {
//	    p.LogWithContext(ctx).Info("handling request", "type", req.Type)
//	    // ...
//	}
func (b *Base) LogWithContext(ctx context.Context) *slog.Logger {
	logger := b.Log()
	if reqID := GetRequestID(ctx); reqID != "" {
		logger = logger.With("request_id", reqID)
	}
	return logger
}

// DataDir returns the plugin's data directory path.
// Plugins can store persistent data here (caches, databases, etc.).
// The directory is created by the host and is unique to each plugin.
//
// Example:
//
//	func (p *MyPlugin) getCachePath() string {
//	    return filepath.Join(p.DataDir(), "cache.json")
//	}
func (b *Base) DataDir() string {
	return b.dataDir
}

// RecordRequest records a successful request for metrics.
// Call this after handling each request to track performance.
//
// Example:
//
//	func (p *MyPlugin) HandleRequest(ctx context.Context) error {
//	    start := time.Now()
//	    defer p.RecordRequest(time.Since(start))
//	    // ... handle request
//	}
func (b *Base) RecordRequest(latency time.Duration) {
	b.requestsTotal.Add(1)
	b.totalLatency.Add(int64(latency))
	b.requestCount.Add(1)
}

// RecordError records an error for metrics.
// Call this when a request fails to track error rates.
//
// Example:
//
//	if err != nil {
//	    p.RecordError()
//	    return err
//	}
func (b *Base) RecordError() {
	b.errorsTotal.Add(1)
}

// Metrics returns the current plugin metrics.
// These metrics are reported to the host for health monitoring.
func (b *Base) Metrics() PluginMetrics {
	requestCount := b.requestCount.Load()
	var avgLatency time.Duration
	if requestCount > 0 {
		avgLatency = time.Duration(b.totalLatency.Load() / requestCount)
	}

	return PluginMetrics{
		RequestsTotal: b.requestsTotal.Load(),
		ErrorsTotal:   b.errorsTotal.Load(),
		AvgLatency:    avgLatency,
	}
}

// PluginMetrics contains plugin performance metrics.
// These are reported to the host for health monitoring and displayed in the UI.
type PluginMetrics struct {
	// RequestsTotal is the total number of requests handled.
	RequestsTotal int64

	// ErrorsTotal is the total number of errors encountered.
	ErrorsTotal int64

	// AvgLatency is the average request latency.
	AvgLatency time.Duration
}

// GetRequestID extracts the request ID from the gRPC context.
// Returns empty string if no request ID is present.
//
// Example:
//
//	reqID := sdk.GetRequestID(ctx)
//	if reqID != "" {
//	    log.Info("handling request", "request_id", reqID)
//	}
func GetRequestID(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	values := md.Get("x-request-id")
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

// WithRequestID returns a context with the request ID in outgoing metadata.
// Use this when making calls to host services to propagate request tracing.
//
// Example:
//
//	ctx = sdk.WithRequestID(ctx, requestID)
//	result, err := hostClient.DoSomething(ctx, req)
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "x-request-id", requestID)
}
