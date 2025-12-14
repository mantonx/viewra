// Package slog provides slog.Handler implementations that bridge
// Go's structured logging with the ViewRA event bus.
package slog

import (
	"context"
	"log/slog"

	"github.com/mantonx/viewra/internal/domain/events"
)

// BusHandler is an slog.Handler that publishes log records to the Event Bus.
// This enables real-time log streaming to UI and diagnostic collection.
type BusHandler struct {
	bus    events.Publisher
	source string
	attrs  []slog.Attr
	groups []string
	level  slog.Level
}

// NewBusHandler creates an slog handler that publishes to the event bus.
// Source identifies the component generating logs (e.g., "scanner", "transcoder").
func NewBusHandler(bus events.Publisher, source string) *BusHandler {
	return &BusHandler{
		bus:    bus,
		source: source,
		level:  slog.LevelDebug, // Default: publish all levels
	}
}

// WithLevel creates a handler that only publishes logs at or above the given level.
func (h *BusHandler) WithLevel(level slog.Level) *BusHandler {
	return &BusHandler{
		bus:    h.bus,
		source: h.source,
		attrs:  h.attrs,
		groups: h.groups,
		level:  level,
	}
}

// Handle publishes a log record as an event.
func (h *BusHandler) Handle(ctx context.Context, r slog.Record) error {
	level := events.LogLevelInfo
	switch {
	case r.Level >= slog.LevelError:
		level = events.LogLevelError
	case r.Level >= slog.LevelWarn:
		level = events.LogLevelWarn
	case r.Level >= slog.LevelInfo:
		level = events.LogLevelInfo
	default:
		level = events.LogLevelDebug
	}

	attrs := make(map[string]any)

	// Add pre-attached attributes
	for _, a := range h.attrs {
		attrs[a.Key] = a.Value.Any()
	}

	// Add record attributes
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})

	// Extract request_id if present
	requestID := ""
	if rid, ok := attrs["request_id"].(string); ok {
		requestID = rid
		delete(attrs, "request_id")
	}

	h.bus.Publish(events.Event{
		Type:      events.EventLog,
		Source:    h.source,
		RequestID: requestID,
		Data: map[string]any{
			"level":   level,
			"message": r.Message,
			"attrs":   attrs,
		},
	})

	return nil
}

// Enabled returns true for levels at or above the handler's minimum level.
func (h *BusHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level
}

// WithAttrs returns a new handler with additional attributes.
func (h *BusHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)

	return &BusHandler{
		bus:    h.bus,
		source: h.source,
		attrs:  newAttrs,
		groups: h.groups,
		level:  h.level,
	}
}

// WithGroup returns a new handler with an additional group prefix.
func (h *BusHandler) WithGroup(name string) slog.Handler {
	newGroups := make([]string, len(h.groups)+1)
	copy(newGroups, h.groups)
	newGroups[len(h.groups)] = name

	return &BusHandler{
		bus:    h.bus,
		source: h.source,
		attrs:  h.attrs,
		groups: newGroups,
		level:  h.level,
	}
}

// MultiHandler combines multiple slog handlers into one.
// Log records are sent to all handlers.
type MultiHandler struct {
	handlers []slog.Handler
}

// NewMultiHandler creates a handler that writes to multiple destinations.
// Common pattern: NewMultiHandler(consoleHandler, busHandler)
func NewMultiHandler(handlers ...slog.Handler) *MultiHandler {
	return &MultiHandler{handlers: handlers}
}

// Handle sends the record to all handlers.
func (m *MultiHandler) Handle(ctx context.Context, r slog.Record) error {
	var firstErr error
	for _, h := range m.handlers {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// Enabled returns true if any handler is enabled for the level.
func (m *MultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

// WithAttrs returns a new handler with attributes added to all sub-handlers.
func (m *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithAttrs(attrs)
	}
	return &MultiHandler{handlers: handlers}
}

// WithGroup returns a new handler with the group added to all sub-handlers.
func (m *MultiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithGroup(name)
	}
	return &MultiHandler{handlers: handlers}
}
