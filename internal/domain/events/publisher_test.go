package events

import (
	"testing"
)

func TestWithBufferSize(t *testing.T) {
	cfg := &SubscribeConfig{}
	opt := WithBufferSize(200)
	opt(cfg)

	if cfg.BufferSize != 200 {
		t.Errorf("expected BufferSize=200, got %d", cfg.BufferSize)
	}
}

func TestWithFilter(t *testing.T) {
	filterCalled := false
	filter := func(e Event) bool {
		filterCalled = true
		return e.Type == EventScanStarted
	}

	cfg := &SubscribeConfig{}
	opt := WithFilter(filter)
	opt(cfg)

	if cfg.Filter == nil {
		t.Fatal("expected Filter to be set")
	}

	// Test the filter function
	result := cfg.Filter(Event{Type: EventScanStarted})
	if !filterCalled {
		t.Error("filter function was not called")
	}
	if !result {
		t.Error("expected filter to return true for EventScanStarted")
	}

	result = cfg.Filter(Event{Type: EventMediaDiscovered})
	if result {
		t.Error("expected filter to return false for EventMediaDiscovered")
	}
}

func TestWithReplayLast(t *testing.T) {
	cfg := &SubscribeConfig{}
	opt := WithReplayLast(50)
	opt(cfg)

	if cfg.ReplayLast != 50 {
		t.Errorf("expected ReplayLast=50, got %d", cfg.ReplayLast)
	}
}

func TestWithEventTypes(t *testing.T) {
	cfg := &SubscribeConfig{}
	opt := WithEventTypes(EventScanStarted, EventScanCompleted, EventScanFailed)
	opt(cfg)

	if cfg.Filter == nil {
		t.Fatal("expected Filter to be set")
	}

	tests := []struct {
		eventType EventType
		expected  bool
	}{
		{EventScanStarted, true},
		{EventScanCompleted, true},
		{EventScanFailed, true},
		{EventScanProgress, false},
		{EventMediaDiscovered, false},
		{EventEnrichmentQueued, false},
	}

	for _, tc := range tests {
		result := cfg.Filter(Event{Type: tc.eventType})
		if result != tc.expected {
			t.Errorf("WithEventTypes filter for %s: expected %v, got %v", tc.eventType, tc.expected, result)
		}
	}
}

func TestWithEventTypes_Empty(t *testing.T) {
	cfg := &SubscribeConfig{}
	opt := WithEventTypes()
	opt(cfg)

	if cfg.Filter == nil {
		t.Fatal("expected Filter to be set")
	}

	// Empty type set should match nothing
	result := cfg.Filter(Event{Type: EventScanStarted})
	if result {
		t.Error("expected empty type set to match nothing")
	}
}

func TestWithEventPrefix(t *testing.T) {
	cfg := &SubscribeConfig{}
	opt := WithEventPrefix("scan.")
	opt(cfg)

	if cfg.Filter == nil {
		t.Fatal("expected Filter to be set")
	}

	tests := []struct {
		eventType EventType
		expected  bool
	}{
		{EventScanStarted, true},
		{EventScanProgress, true},
		{EventScanCompleted, true},
		{EventScanFailed, true},
		{EventMediaDiscovered, false},
		{EventEnrichmentQueued, false},
		{EventTranscodeStarted, false},
	}

	for _, tc := range tests {
		result := cfg.Filter(Event{Type: tc.eventType})
		if result != tc.expected {
			t.Errorf("WithEventPrefix filter for %s: expected %v, got %v", tc.eventType, tc.expected, result)
		}
	}
}

func TestWithEventPrefix_Enrichment(t *testing.T) {
	cfg := &SubscribeConfig{}
	opt := WithEventPrefix("enrichment.")
	opt(cfg)

	tests := []struct {
		eventType EventType
		expected  bool
	}{
		{EventEnrichmentQueued, true},
		{EventEnrichmentStarted, true},
		{EventEnrichmentStageComplete, true},
		{EventEnrichmentComplete, true},
		{EventEnrichmentFailed, true},
		{EventEnrichmentSkipped, true},
		{EventScanStarted, false},
		{EventMediaDiscovered, false},
	}

	for _, tc := range tests {
		result := cfg.Filter(Event{Type: tc.eventType})
		if result != tc.expected {
			t.Errorf("WithEventPrefix(enrichment.) filter for %s: expected %v, got %v", tc.eventType, tc.expected, result)
		}
	}
}

func TestWithEventPrefix_EmptyPrefix(t *testing.T) {
	cfg := &SubscribeConfig{}
	opt := WithEventPrefix("")
	opt(cfg)

	// Empty prefix should match everything
	result := cfg.Filter(Event{Type: EventScanStarted})
	if !result {
		t.Error("expected empty prefix to match everything")
	}
}

func TestSubscribeOptions_Chaining(t *testing.T) {
	cfg := &SubscribeConfig{}

	// Apply multiple options
	opts := []SubscribeOption{
		WithBufferSize(500),
		WithReplayLast(100),
		WithEventPrefix("scan."),
	}

	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.BufferSize != 500 {
		t.Errorf("expected BufferSize=500, got %d", cfg.BufferSize)
	}
	if cfg.ReplayLast != 100 {
		t.Errorf("expected ReplayLast=100, got %d", cfg.ReplayLast)
	}
	if cfg.Filter == nil {
		t.Error("expected Filter to be set")
	}
}

func TestSubscribeConfig_ZeroValue(t *testing.T) {
	cfg := SubscribeConfig{}

	if cfg.BufferSize != 0 {
		t.Errorf("expected default BufferSize=0, got %d", cfg.BufferSize)
	}
	if cfg.ReplayLast != 0 {
		t.Errorf("expected default ReplayLast=0, got %d", cfg.ReplayLast)
	}
	if cfg.Filter != nil {
		t.Error("expected default Filter=nil")
	}
}
