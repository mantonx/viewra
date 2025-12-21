// Package sdk provides utilities for building ViewRA plugins.
package sdk

import (
	"context"
	"log/slog"
	"os"

	"github.com/hashicorp/go-hclog"
)

// NewLogger creates a new logger for use in ViewRA plugins.
// It returns both an hclog.Logger (for go-plugin compatibility) and
// an slog.Logger (for use in plugin code).
//
// The hclog.Logger should be passed to plugin.Serve() so go-plugin
// can capture and forward logs to the host. The slog.Logger wraps
// hclog and should be used throughout your plugin code.
//
// Example:
//
//	func main() {
//	    hclogger, logger := sdk.NewLogger("my-plugin")
//	    myPlugin := NewMyPlugin(logger)
//	    plugin.Serve(&plugin.ServeConfig{
//	        // ... other config ...
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

// NewLoggerWithLevel creates a logger with a specific log level.
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
