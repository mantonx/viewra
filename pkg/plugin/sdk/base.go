// Package sdk provides utilities for building ViewRA plugins.
//
// Plugin authors embed sdk.Base in their plugin struct to get common
// functionality like logging, metrics, and request context handling.
//
// Example:
//
//	type MyPlugin struct {
//	    sdk.Base
//	    // your fields
//	}
//
//	func (p *MyPlugin) Enrich(ctx context.Context, req *sdk.EnrichRequest) (*sdk.EnrichResponse, error) {
//	    p.Log().Info("enriching", "title", req.Title)
//	    // your implementation
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
// Plugin structs must embed this type.
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
func (Base) mustEmbedBase() {}

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
func (b *Base) InitWithLogger(dataDir string, logger *slog.Logger) {
	b.dataDir = dataDir
	b.logger = logger
}

// SetLogger sets the logger for this plugin.
func (b *Base) SetLogger(logger *slog.Logger) {
	b.logger = logger
}

// Log returns a logger with the request ID attached (if available in context).
func (b *Base) Log() *slog.Logger {
	if b.logger == nil {
		b.logger = slog.Default()
	}
	return b.logger
}

// LogWithContext returns a logger with the request ID from the context.
func (b *Base) LogWithContext(ctx context.Context) *slog.Logger {
	logger := b.Log()
	if reqID := GetRequestID(ctx); reqID != "" {
		logger = logger.With("request_id", reqID)
	}
	return logger
}

// DataDir returns the plugin's data directory path.
// Plugins can store persistent data here.
func (b *Base) DataDir() string {
	return b.dataDir
}

// RecordRequest records a successful request for metrics.
func (b *Base) RecordRequest(latency time.Duration) {
	b.requestsTotal.Add(1)
	b.totalLatency.Add(int64(latency))
	b.requestCount.Add(1)
}

// RecordError records an error for metrics.
func (b *Base) RecordError() {
	b.errorsTotal.Add(1)
}

// Metrics returns the current plugin metrics.
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
type PluginMetrics struct {
	RequestsTotal int64
	ErrorsTotal   int64
	AvgLatency    time.Duration
}

// GetRequestID extracts the request ID from the gRPC context.
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
// Use this when making calls to host services.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "x-request-id", requestID)
}
