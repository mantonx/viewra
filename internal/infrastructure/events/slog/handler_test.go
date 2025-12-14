package slog

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/domain/events"
)

// mockPublisher captures published events for testing.
type mockPublisher struct {
	mu     sync.Mutex
	events []events.Event
}

func (m *mockPublisher) Publish(e events.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, e)
}

func (m *mockPublisher) getEvents() []events.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]events.Event, len(m.events))
	copy(result, m.events)
	return result
}

func TestNewBusHandler(t *testing.T) {
	pub := &mockPublisher{}
	h := NewBusHandler(pub, "test-source")

	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if h.bus != pub {
		t.Error("bus not set correctly")
	}
	if h.source != "test-source" {
		t.Errorf("expected source 'test-source', got %s", h.source)
	}
	if h.level != slog.LevelDebug {
		t.Errorf("expected default level Debug, got %v", h.level)
	}
}

func TestBusHandler_Handle_Info(t *testing.T) {
	pub := &mockPublisher{}
	h := NewBusHandler(pub, "test")

	ctx := context.Background()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test message", 0)
	record.AddAttrs(slog.String("key1", "value1"))

	err := h.Handle(ctx, record)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	publishedEvents := pub.getEvents()
	if len(publishedEvents) != 1 {
		t.Fatalf("expected 1 event, got %d", len(publishedEvents))
	}

	e := publishedEvents[0]
	if e.Type != "log" {
		t.Errorf("expected type 'log', got %s", e.Type)
	}
	if e.Source != "test" {
		t.Errorf("expected source 'test', got %s", e.Source)
	}

	data := e.Data
	if data["level"] != events.LogLevelInfo {
		t.Errorf("expected level 'info', got %v", data["level"])
	}
	if data["message"] != "test message" {
		t.Errorf("expected message 'test message', got %v", data["message"])
	}

	attrs := data["attrs"].(map[string]any)
	if attrs["key1"] != "value1" {
		t.Errorf("expected key1='value1', got %v", attrs["key1"])
	}
}

func TestBusHandler_Handle_Levels(t *testing.T) {
	tests := []struct {
		level    slog.Level
		expected events.LogLevel
	}{
		{slog.LevelDebug, events.LogLevelDebug},
		{slog.LevelInfo, events.LogLevelInfo},
		{slog.LevelWarn, events.LogLevelWarn},
		{slog.LevelError, events.LogLevelError},
		{slog.Level(-10), events.LogLevelDebug},   // Below debug
		{slog.Level(100), events.LogLevelError},   // Above error
	}

	for _, tc := range tests {
		pub := &mockPublisher{}
		h := NewBusHandler(pub, "test")

		ctx := context.Background()
		record := slog.NewRecord(time.Now(), tc.level, "test", 0)

		err := h.Handle(ctx, record)
		if err != nil {
			t.Errorf("Handle failed for level %v: %v", tc.level, err)
			continue
		}

		publishedEvents := pub.getEvents()
		if len(publishedEvents) != 1 {
			t.Errorf("expected 1 event for level %v, got %d", tc.level, len(publishedEvents))
			continue
		}

		level := publishedEvents[0].Data["level"].(events.LogLevel)
		if level != tc.expected {
			t.Errorf("level %v: expected %s, got %s", tc.level, tc.expected, level)
		}
	}
}

func TestBusHandler_Handle_RequestID(t *testing.T) {
	pub := &mockPublisher{}
	h := NewBusHandler(pub, "test")

	ctx := context.Background()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0)
	record.AddAttrs(slog.String("request_id", "req-123"))

	err := h.Handle(ctx, record)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	publishedEvents := pub.getEvents()
	e := publishedEvents[0]

	if e.RequestID != "req-123" {
		t.Errorf("expected request_id 'req-123', got %s", e.RequestID)
	}

	// request_id should be removed from attrs
	attrs := e.Data["attrs"].(map[string]any)
	if _, ok := attrs["request_id"]; ok {
		t.Error("request_id should be removed from attrs")
	}
}

func TestBusHandler_Enabled(t *testing.T) {
	pub := &mockPublisher{}
	h := NewBusHandler(pub, "test")
	ctx := context.Background()

	// Default level is Debug, so all levels should be enabled
	if !h.Enabled(ctx, slog.LevelDebug) {
		t.Error("expected Debug to be enabled")
	}
	if !h.Enabled(ctx, slog.LevelInfo) {
		t.Error("expected Info to be enabled")
	}
	if !h.Enabled(ctx, slog.LevelWarn) {
		t.Error("expected Warn to be enabled")
	}
	if !h.Enabled(ctx, slog.LevelError) {
		t.Error("expected Error to be enabled")
	}
}

func TestBusHandler_WithLevel(t *testing.T) {
	pub := &mockPublisher{}
	h := NewBusHandler(pub, "test").WithLevel(slog.LevelWarn)
	ctx := context.Background()

	if h.Enabled(ctx, slog.LevelDebug) {
		t.Error("expected Debug to be disabled")
	}
	if h.Enabled(ctx, slog.LevelInfo) {
		t.Error("expected Info to be disabled")
	}
	if !h.Enabled(ctx, slog.LevelWarn) {
		t.Error("expected Warn to be enabled")
	}
	if !h.Enabled(ctx, slog.LevelError) {
		t.Error("expected Error to be enabled")
	}
}

func TestBusHandler_WithAttrs(t *testing.T) {
	pub := &mockPublisher{}
	h := NewBusHandler(pub, "test")

	// Add pre-attached attributes
	h2 := h.WithAttrs([]slog.Attr{
		slog.String("component", "scanner"),
		slog.Int("version", 1),
	}).(*BusHandler)

	ctx := context.Background()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0)
	record.AddAttrs(slog.String("key", "value"))

	err := h2.Handle(ctx, record)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	publishedEvents := pub.getEvents()
	attrs := publishedEvents[0].Data["attrs"].(map[string]any)

	if attrs["component"] != "scanner" {
		t.Errorf("expected component='scanner', got %v", attrs["component"])
	}
	if attrs["version"] != int64(1) {
		t.Errorf("expected version=1, got %v", attrs["version"])
	}
	if attrs["key"] != "value" {
		t.Errorf("expected key='value', got %v", attrs["key"])
	}
}

func TestBusHandler_WithGroup(t *testing.T) {
	pub := &mockPublisher{}
	h := NewBusHandler(pub, "test")

	h2 := h.WithGroup("mygroup").(*BusHandler)

	if len(h2.groups) != 1 || h2.groups[0] != "mygroup" {
		t.Errorf("expected groups=['mygroup'], got %v", h2.groups)
	}

	h3 := h2.WithGroup("nested").(*BusHandler)
	if len(h3.groups) != 2 || h3.groups[1] != "nested" {
		t.Errorf("expected groups=['mygroup', 'nested'], got %v", h3.groups)
	}
}

func TestBusHandler_Integration(t *testing.T) {
	pub := &mockPublisher{}
	h := NewBusHandler(pub, "scanner")

	logger := slog.New(h)
	logger.Info("scan started",
		slog.Int64("library_id", 5),
		slog.String("path", "/media/movies"))

	publishedEvents := pub.getEvents()
	if len(publishedEvents) != 1 {
		t.Fatalf("expected 1 event, got %d", len(publishedEvents))
	}

	e := publishedEvents[0]
	if e.Source != "scanner" {
		t.Errorf("expected source 'scanner', got %s", e.Source)
	}
	if e.Data["message"] != "scan started" {
		t.Errorf("expected message 'scan started', got %v", e.Data["message"])
	}
}

func TestNewMultiHandler(t *testing.T) {
	h := NewMultiHandler()
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if len(h.handlers) != 0 {
		t.Errorf("expected 0 handlers, got %d", len(h.handlers))
	}
}

func TestMultiHandler_Handle(t *testing.T) {
	pub1 := &mockPublisher{}
	pub2 := &mockPublisher{}
	h1 := NewBusHandler(pub1, "source1")
	h2 := NewBusHandler(pub2, "source2")

	multi := NewMultiHandler(h1, h2)

	ctx := context.Background()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test message", 0)

	err := multi.Handle(ctx, record)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	// Both handlers should have received the event
	events1 := pub1.getEvents()
	events2 := pub2.getEvents()

	if len(events1) != 1 {
		t.Errorf("expected 1 event in pub1, got %d", len(events1))
	}
	if len(events2) != 1 {
		t.Errorf("expected 1 event in pub2, got %d", len(events2))
	}
}

func TestMultiHandler_Handle_RespectsLevel(t *testing.T) {
	pub1 := &mockPublisher{}
	pub2 := &mockPublisher{}
	h1 := NewBusHandler(pub1, "source1").WithLevel(slog.LevelInfo)
	h2 := NewBusHandler(pub2, "source2").WithLevel(slog.LevelError)

	multi := NewMultiHandler(h1, h2)

	ctx := context.Background()

	// Info level - only h1 should handle
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "info message", 0)
	_ = multi.Handle(ctx, record)

	if len(pub1.getEvents()) != 1 {
		t.Error("expected pub1 to receive info event")
	}
	if len(pub2.getEvents()) != 0 {
		t.Error("expected pub2 to not receive info event")
	}

	// Error level - both should handle
	record = slog.NewRecord(time.Now(), slog.LevelError, "error message", 0)
	_ = multi.Handle(ctx, record)

	if len(pub1.getEvents()) != 2 {
		t.Error("expected pub1 to receive error event")
	}
	if len(pub2.getEvents()) != 1 {
		t.Error("expected pub2 to receive error event")
	}
}

func TestMultiHandler_Enabled(t *testing.T) {
	pub1 := &mockPublisher{}
	pub2 := &mockPublisher{}
	h1 := NewBusHandler(pub1, "source1").WithLevel(slog.LevelWarn)
	h2 := NewBusHandler(pub2, "source2").WithLevel(slog.LevelError)

	multi := NewMultiHandler(h1, h2)
	ctx := context.Background()

	// Warn is enabled because h1 accepts it
	if !multi.Enabled(ctx, slog.LevelWarn) {
		t.Error("expected Warn to be enabled (h1 accepts it)")
	}

	// Info is not enabled by either
	if multi.Enabled(ctx, slog.LevelInfo) {
		t.Error("expected Info to be disabled")
	}

	// Error is enabled by both
	if !multi.Enabled(ctx, slog.LevelError) {
		t.Error("expected Error to be enabled")
	}
}

func TestMultiHandler_Enabled_Empty(t *testing.T) {
	multi := NewMultiHandler()
	ctx := context.Background()

	// Empty handler should return false for all levels
	if multi.Enabled(ctx, slog.LevelInfo) {
		t.Error("expected empty handler to return false")
	}
}

func TestMultiHandler_WithAttrs(t *testing.T) {
	pub1 := &mockPublisher{}
	pub2 := &mockPublisher{}
	h1 := NewBusHandler(pub1, "source1")
	h2 := NewBusHandler(pub2, "source2")

	multi := NewMultiHandler(h1, h2)
	multi2 := multi.WithAttrs([]slog.Attr{slog.String("key", "value")}).(*MultiHandler)

	ctx := context.Background()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0)
	_ = multi2.Handle(ctx, record)

	// Both should have the attribute
	for i, pub := range []*mockPublisher{pub1, pub2} {
		publishedEvents := pub.getEvents()
		if len(publishedEvents) != 1 {
			t.Errorf("publisher %d: expected 1 event", i)
			continue
		}
		attrs := publishedEvents[0].Data["attrs"].(map[string]any)
		if attrs["key"] != "value" {
			t.Errorf("publisher %d: expected key='value', got %v", i, attrs["key"])
		}
	}
}

func TestMultiHandler_WithGroup(t *testing.T) {
	pub1 := &mockPublisher{}
	pub2 := &mockPublisher{}
	h1 := NewBusHandler(pub1, "source1")
	h2 := NewBusHandler(pub2, "source2")

	multi := NewMultiHandler(h1, h2)
	multi2 := multi.WithGroup("mygroup").(*MultiHandler)

	if len(multi2.handlers) != 2 {
		t.Errorf("expected 2 handlers after WithGroup, got %d", len(multi2.handlers))
	}
}

func TestMultiHandler_Integration(t *testing.T) {
	pub := &mockPublisher{}
	busHandler := NewBusHandler(pub, "app")
	consoleHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})

	multi := NewMultiHandler(busHandler, consoleHandler)
	logger := slog.New(multi)

	logger.Info("test message", slog.String("key", "value"))

	// BusHandler should have received the event
	publishedEvents := pub.getEvents()
	if len(publishedEvents) != 1 {
		t.Errorf("expected 1 event, got %d", len(publishedEvents))
	}
}
