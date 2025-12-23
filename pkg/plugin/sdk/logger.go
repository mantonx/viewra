// Logging utilities for ViewRA plugins.
//
// ViewRA plugins use go-plugin for process isolation, which requires special
// handling of logs. This file provides logger adapters that ensure plugin logs
// are properly captured and forwarded to the host application.
//
// # Why Special Logging?
//
// go-plugin runs plugins in separate processes. For logs to appear in the host's
// log output, they must be written in a format that go-plugin can parse and forward.
// The NewLogger function creates loggers configured for this.
//
// # Usage
//
// In your plugin's main() function:
//
//	func main() {
//	    // Create both hclog (for go-plugin) and slog (for your code) loggers
//	    hclogger, logger := sdk.NewLogger("my-plugin")
//
//	    // Create your plugin with the slog logger
//	    plugin := internal.NewPlugin(logger)
//
//	    // Start the plugin server with the hclog logger
//	    go_plugin.Serve(&go_plugin.ServeConfig{
//	        HandshakeConfig: sdk.Handshake,
//	        Plugins: map[string]go_plugin.Plugin{
//	            "core": &MyGRPCPlugin{Impl: plugin},
//	        },
//	        GRPCServer: go_plugin.DefaultGRPCServer,
//	        Logger:     hclogger, // Important: pass hclogger here
//	    })
//	}
//
// Then use the slog logger throughout your plugin:
//
//	func (p *Plugin) DoSomething() {
//	    p.logger.Info("doing something", "key", "value")
//	}
package sdk

import (
	"context"
	"log/slog"
	"os"

	"github.com/hashicorp/go-hclog"
)

// NewLogger creates a new logger pair for use in ViewRA plugins.
//
// Returns:
//   - hclog.Logger: Pass this to plugin.Serve() for go-plugin log forwarding
//   - *slog.Logger: Use this in your plugin code for logging
//
// The hclog.Logger outputs JSON to stderr, which go-plugin parses and forwards
// to the host. The slog.Logger wraps hclog so you can use the standard slog API.
//
// Example:
//
//	func main() {
//	    hclogger, logger := sdk.NewLogger("my-plugin")
//
//	    plugin := NewMyPlugin(logger)
//
//	    go_plugin.Serve(&go_plugin.ServeConfig{
//	        HandshakeConfig: sdk.Handshake,
//	        Plugins: map[string]go_plugin.Plugin{...},
//	        GRPCServer: go_plugin.DefaultGRPCServer,
//	        Logger: hclogger,
//	    })
//	}
func NewLogger(name string) (hclog.Logger, *slog.Logger) {
	hclogger := hclog.New(&hclog.LoggerOptions{
		Name:       name,
		Level:      hclog.Debug,
		Output:     os.Stderr,
		JSONFormat: true, // JSON format is parsed by go-plugin and forwarded to host
	})

	slogger := slog.New(newHclogHandler(hclogger))
	return hclogger, slogger
}

// NewLoggerWithLevel creates a logger pair with a specific log level.
// Use this when you want to control the verbosity of plugin logs.
//
// Example:
//
//	// Only log warnings and errors
//	hclogger, logger := sdk.NewLoggerWithLevel("my-plugin", slog.LevelWarn)
func NewLoggerWithLevel(name string, level slog.Level) (hclog.Logger, *slog.Logger) {
	hcLevel := slogLevelToHclog(level)
	hclogger := hclog.New(&hclog.LoggerOptions{
		Name:       name,
		Level:      hcLevel,
		Output:     os.Stderr,
		JSONFormat: true,
	})

	slogger := slog.New(newHclogHandler(hclogger))
	return hclogger, slogger
}

// slogLevelToHclog converts slog.Level to hclog.Level.
func slogLevelToHclog(level slog.Level) hclog.Level {
	switch {
	case level <= slog.LevelDebug:
		return hclog.Debug
	case level <= slog.LevelInfo:
		return hclog.Info
	case level <= slog.LevelWarn:
		return hclog.Warn
	default:
		return hclog.Error
	}
}

// hclogHandler adapts hclog.Logger to slog.Handler interface.
// This allows plugin code to use the standard slog API while
// outputting in hclog format that go-plugin can parse and forward.
type hclogHandler struct {
	logger hclog.Logger
	attrs  []slog.Attr
	groups []string
}

func newHclogHandler(logger hclog.Logger) *hclogHandler {
	return &hclogHandler{logger: logger}
}

func (h *hclogHandler) Enabled(_ context.Context, level slog.Level) bool {
	switch level {
	case slog.LevelDebug:
		return h.logger.IsDebug()
	case slog.LevelInfo:
		return h.logger.IsInfo()
	case slog.LevelWarn:
		return h.logger.IsWarn()
	case slog.LevelError:
		return h.logger.IsError()
	default:
		return true
	}
}

func (h *hclogHandler) Handle(_ context.Context, r slog.Record) error {
	// Convert slog attrs to hclog args
	args := make([]interface{}, 0, r.NumAttrs()*2+len(h.attrs)*2)

	// Add handler-level attrs first
	for _, a := range h.attrs {
		args = append(args, a.Key, a.Value.Any())
	}

	// Add record attrs
	r.Attrs(func(a slog.Attr) bool {
		args = append(args, a.Key, a.Value.Any())
		return true
	})

	// Log at appropriate level
	switch r.Level {
	case slog.LevelDebug:
		h.logger.Debug(r.Message, args...)
	case slog.LevelInfo:
		h.logger.Info(r.Message, args...)
	case slog.LevelWarn:
		h.logger.Warn(r.Message, args...)
	case slog.LevelError:
		h.logger.Error(r.Message, args...)
	default:
		h.logger.Info(r.Message, args...)
	}

	return nil
}

func (h *hclogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &hclogHandler{
		logger: h.logger,
		attrs:  newAttrs,
		groups: h.groups,
	}
}

func (h *hclogHandler) WithGroup(name string) slog.Handler {
	newGroups := make([]string, len(h.groups)+1)
	copy(newGroups, h.groups)
	newGroups[len(h.groups)] = name
	return &hclogHandler{
		logger: h.logger.Named(name),
		attrs:  h.attrs,
		groups: newGroups,
	}
}
