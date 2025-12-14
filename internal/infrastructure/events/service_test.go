package events

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	domainevents "github.com/mantonx/viewra/internal/domain/events"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestNewBus(t *testing.T) {
	b := NewBus(100, testLogger())

	if b == nil {
		t.Fatal("expected non-nil bus")
	}

	stats := b.Stats()
	if stats.BufferCapacity != 100 {
		t.Errorf("expected buffer capacity 100, got %d", stats.BufferCapacity)
	}
}

func TestNewBusHandler(t *testing.T) {
	pub := NewBus(100, testLogger())
	h := NewBusHandler(pub, "test")

	if h == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestNewMultiHandler(t *testing.T) {
	pub := NewBus(100, testLogger())
	h1 := NewBusHandler(pub, "source1")
	h2 := slog.NewTextHandler(os.Stderr, nil)

	multi := NewMultiHandler(h1, h2)
	if multi == nil {
		t.Fatal("expected non-nil multi handler")
	}
}

// Test that type aliases work correctly
func TestTypeAliases(t *testing.T) {
	// Bus type alias
	var bus *Bus
	bus = NewBus(100, testLogger())
	if bus == nil {
		t.Error("Bus type alias failed")
	}

	// Event type alias
	var e Event
	e = Event{
		Type:   EventMediaDiscovered,
		Source: "test",
	}
	if e.Type != domainevents.EventMediaDiscovered {
		t.Error("Event type alias failed")
	}

	// EventType type alias
	var et EventType
	et = EventScanStarted
	if et != domainevents.EventScanStarted {
		t.Error("EventType type alias failed")
	}

	// LogLevel type alias
	var ll LogLevel
	ll = LogLevelError
	if ll != domainevents.LogLevelError {
		t.Error("LogLevel type alias failed")
	}
}

// Test that re-exported constants match domain constants
func TestReExportedConstants(t *testing.T) {
	tests := []struct {
		name     string
		exported EventType
		domain   domainevents.EventType
	}{
		{"EventMediaDiscovered", EventMediaDiscovered, domainevents.EventMediaDiscovered},
		{"EventMediaUpdated", EventMediaUpdated, domainevents.EventMediaUpdated},
		{"EventMediaRemoved", EventMediaRemoved, domainevents.EventMediaRemoved},
		{"EventEnrichmentQueued", EventEnrichmentQueued, domainevents.EventEnrichmentQueued},
		{"EventEnrichmentStarted", EventEnrichmentStarted, domainevents.EventEnrichmentStarted},
		{"EventEnrichmentStageComplete", EventEnrichmentStageComplete, domainevents.EventEnrichmentStageComplete},
		{"EventEnrichmentComplete", EventEnrichmentComplete, domainevents.EventEnrichmentComplete},
		{"EventEnrichmentFailed", EventEnrichmentFailed, domainevents.EventEnrichmentFailed},
		{"EventEnrichmentSkipped", EventEnrichmentSkipped, domainevents.EventEnrichmentSkipped},
		{"EventPluginLoaded", EventPluginLoaded, domainevents.EventPluginLoaded},
		{"EventPluginUnloaded", EventPluginUnloaded, domainevents.EventPluginUnloaded},
		{"EventPluginCrashed", EventPluginCrashed, domainevents.EventPluginCrashed},
		{"EventPluginHealthUpdate", EventPluginHealthUpdate, domainevents.EventPluginHealthUpdate},
		{"EventTranscodeStarted", EventTranscodeStarted, domainevents.EventTranscodeStarted},
		{"EventTranscodeProgress", EventTranscodeProgress, domainevents.EventTranscodeProgress},
		{"EventTranscodeCompleted", EventTranscodeCompleted, domainevents.EventTranscodeCompleted},
		{"EventTranscodeFailed", EventTranscodeFailed, domainevents.EventTranscodeFailed},
		{"EventScanStarted", EventScanStarted, domainevents.EventScanStarted},
		{"EventScanProgress", EventScanProgress, domainevents.EventScanProgress},
		{"EventScanCompleted", EventScanCompleted, domainevents.EventScanCompleted},
		{"EventScanFailed", EventScanFailed, domainevents.EventScanFailed},
		{"EventLog", EventLog, domainevents.EventLog},
	}

	for _, tc := range tests {
		if tc.exported != tc.domain {
			t.Errorf("%s: exported %s != domain %s", tc.name, tc.exported, tc.domain)
		}
	}
}

func TestReExportedLogLevels(t *testing.T) {
	if LogLevelDebug != domainevents.LogLevelDebug {
		t.Error("LogLevelDebug mismatch")
	}
	if LogLevelInfo != domainevents.LogLevelInfo {
		t.Error("LogLevelInfo mismatch")
	}
	if LogLevelWarn != domainevents.LogLevelWarn {
		t.Error("LogLevelWarn mismatch")
	}
	if LogLevelError != domainevents.LogLevelError {
		t.Error("LogLevelError mismatch")
	}
}

func TestReExportedSubscribeOptions(t *testing.T) {
	cfg := &domainevents.SubscribeConfig{}

	// Test WithBufferSize
	WithBufferSize(200)(cfg)
	if cfg.BufferSize != 200 {
		t.Errorf("WithBufferSize: expected 200, got %d", cfg.BufferSize)
	}

	// Test WithReplayLast
	WithReplayLast(50)(cfg)
	if cfg.ReplayLast != 50 {
		t.Errorf("WithReplayLast: expected 50, got %d", cfg.ReplayLast)
	}

	// Test WithFilter
	called := false
	WithFilter(func(e domainevents.Event) bool {
		called = true
		return true
	})(cfg)
	cfg.Filter(domainevents.Event{})
	if !called {
		t.Error("WithFilter: filter not called")
	}
}

func TestReExportedNewEvent(t *testing.T) {
	// NewEvent should work the same as domain NewEvent
	e := NewEvent(EventScanStarted, "scanner")

	if e.Type != EventScanStarted {
		t.Errorf("expected type EventScanStarted, got %s", e.Type)
	}
	if e.Source != "scanner" {
		t.Errorf("expected source 'scanner', got %s", e.Source)
	}
	if e.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}

	// Test chaining
	event := NewEvent(EventMediaDiscovered, "test").
		WithMediaID(42).
		WithRequestID("req-123").
		Build()

	if event.Data["media_id"] != int64(42) {
		t.Errorf("expected media_id=42, got %v", event.Data["media_id"])
	}
	if event.RequestID != "req-123" {
		t.Errorf("expected request_id='req-123', got %s", event.RequestID)
	}
}

func TestIntegration_PublishSubscribe(t *testing.T) {
	b := NewBus(100, testLogger())

	// Subscribe with filter
	sub := b.Subscribe(
		WithBufferSize(10),
		WithEventPrefix("scan."),
	)
	defer b.Unsubscribe(sub)

	// Publish events using re-exported types
	b.Publish(NewEvent(EventScanStarted, "scanner").
		WithLibraryID(5).
		Build())

	b.Publish(NewEvent(EventMediaDiscovered, "scanner").
		WithMediaID(100).
		Build())

	b.Publish(NewEvent(EventScanCompleted, "scanner").
		WithLibraryID(5).
		Build())

	// Should only receive scan events
	received := []EventType{}
	timeout := time.After(100 * time.Millisecond)
loop:
	for {
		select {
		case e := <-sub.Events():
			received = append(received, e.Type)
		case <-timeout:
			break loop
		}
	}

	if len(received) != 2 {
		t.Fatalf("expected 2 scan events, got %d: %v", len(received), received)
	}
	if received[0] != EventScanStarted {
		t.Errorf("expected EventScanStarted, got %s", received[0])
	}
	if received[1] != EventScanCompleted {
		t.Errorf("expected EventScanCompleted, got %s", received[1])
	}
}

func TestIntegration_SlogToBus(t *testing.T) {
	b := NewBus(100, testLogger())

	// Subscribe to log events
	sub := b.Subscribe(WithEventTypes(EventLog))
	defer b.Unsubscribe(sub)

	// Create logger that publishes to bus
	handler := NewBusHandler(b, "test-component")
	logger := slog.New(handler)

	// Log something
	logger.Info("test message", slog.String("key", "value"))

	// Should receive the log event
	select {
	case e := <-sub.Events():
		if e.Type != EventLog {
			t.Errorf("expected EventLog, got %s", e.Type)
		}
		if e.Source != "test-component" {
			t.Errorf("expected source 'test-component', got %s", e.Source)
		}
		if e.Data["message"] != "test message" {
			t.Errorf("expected message 'test message', got %v", e.Data["message"])
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timed out waiting for log event")
	}
}

func TestIntegration_ReplayWithFilter(t *testing.T) {
	b := NewBus(100, testLogger())

	// Publish various events
	b.Publish(NewEvent(EventScanStarted, "scanner").Build())
	b.Publish(NewEvent(EventMediaDiscovered, "scanner").Build())
	b.Publish(NewEvent(EventEnrichmentQueued, "enricher").Build())
	b.Publish(NewEvent(EventScanProgress, "scanner").Build())
	b.Publish(NewEvent(EventEnrichmentComplete, "enricher").Build())

	// Subscribe with filter and replay
	sub := b.Subscribe(
		WithEventPrefix("enrichment."),
		WithReplayLast(10),
	)
	defer b.Unsubscribe(sub)

	// Should only replay enrichment events
	received := []EventType{}
	timeout := time.After(100 * time.Millisecond)
loop:
	for {
		select {
		case e := <-sub.Events():
			received = append(received, e.Type)
		case <-timeout:
			break loop
		}
	}

	if len(received) != 2 {
		t.Fatalf("expected 2 enrichment events, got %d: %v", len(received), received)
	}
}

// Test that interfaces work correctly
func TestInterfaces(t *testing.T) {
	b := NewBus(100, testLogger())

	// Bus should satisfy Publisher
	var pub Publisher = b
	pub.Publish(Event{Type: EventScanStarted})

	// Bus should satisfy Subscriber
	var sub Subscriber = b
	subscription := sub.Subscribe()
	subscription.Close()
}

// Test that the facade can be used without importing sub-packages
func TestFacadeUsage(t *testing.T) {
	// This test verifies that consumers only need to import this package
	// All necessary types and functions are available

	// Create bus
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	bus := NewBus(1000, logger)

	// Create subscription with options
	sub := bus.Subscribe(
		WithBufferSize(100),
		WithEventPrefix("scan."),
		WithReplayLast(10),
	)
	defer bus.Unsubscribe(sub)

	// Create events using builder
	event := NewEvent(EventScanStarted, "scanner").
		WithLibraryID(1).
		WithRequestID("req-123").
		Build()

	// Publish event
	bus.Publish(event)

	// Create slog handler
	handler := NewBusHandler(bus, "component")
	_ = slog.New(handler)

	// Create multi handler
	consoleHandler := slog.NewTextHandler(os.Stderr, nil)
	_ = NewMultiHandler(handler, consoleHandler)

	// Get stats
	stats := bus.Stats()
	if stats.TotalPublished != 1 {
		t.Errorf("expected 1 published, got %d", stats.TotalPublished)
	}

	// Get recent events
	recent := bus.Recent(10)
	if len(recent) != 1 {
		t.Errorf("expected 1 recent event, got %d", len(recent))
	}

	// Close bus
	bus.Close()
}

// mockPublisher for testing NewBusHandler with custom publisher
type mockPublisher struct {
	events []Event
}

func (m *mockPublisher) Publish(e Event) {
	m.events = append(m.events, e)
}

func TestNewBusHandler_WithCustomPublisher(t *testing.T) {
	pub := &mockPublisher{}

	// NewBusHandler accepts domainevents.Publisher interface
	h := NewBusHandler(pub, "test")

	logger := slog.New(h)
	logger.Info("test message")

	if len(pub.events) != 1 {
		t.Errorf("expected 1 event, got %d", len(pub.events))
	}
}

func TestWithEventTypes_ViaFacade(t *testing.T) {
	b := NewBus(100, testLogger())

	sub := b.Subscribe(WithEventTypes(
		EventScanStarted,
		EventScanCompleted,
	))
	defer b.Unsubscribe(sub)

	b.Publish(Event{Type: EventScanStarted})
	b.Publish(Event{Type: EventScanProgress})
	b.Publish(Event{Type: EventScanCompleted})
	b.Publish(Event{Type: EventMediaDiscovered})

	received := 0
	timeout := time.After(100 * time.Millisecond)
loop:
	for {
		select {
		case <-sub.Events():
			received++
		case <-timeout:
			break loop
		}
	}

	if received != 2 {
		t.Errorf("expected 2 events (started + completed), got %d", received)
	}
}

func TestBusHandler_WithSlogLogger(t *testing.T) {
	b := NewBus(100, testLogger())

	sub := b.Subscribe(WithEventTypes(EventLog))
	defer b.Unsubscribe(sub)

	handler := NewBusHandler(b, "myapp")
	logger := slog.New(handler)

	ctx := context.Background()
	logger.InfoContext(ctx, "application started",
		slog.String("version", "1.0.0"),
		slog.Int("port", 8080),
	)

	select {
	case e := <-sub.Events():
		if e.Type != EventLog {
			t.Errorf("expected EventLog, got %s", e.Type)
		}
		if e.Data["level"] != LogLevelInfo {
			t.Errorf("expected level info, got %v", e.Data["level"])
		}
		attrs := e.Data["attrs"].(map[string]any)
		if attrs["version"] != "1.0.0" {
			t.Errorf("expected version '1.0.0', got %v", attrs["version"])
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timed out waiting for log event")
	}
}
