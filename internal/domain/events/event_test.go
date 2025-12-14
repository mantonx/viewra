package events

import (
	"errors"
	"testing"
	"time"
)

func TestNewEvent(t *testing.T) {
	e := NewEvent(EventMediaDiscovered, "scanner")

	if e.Type != EventMediaDiscovered {
		t.Errorf("expected type %s, got %s", EventMediaDiscovered, e.Type)
	}
	if e.Source != "scanner" {
		t.Errorf("expected source 'scanner', got %s", e.Source)
	}
	if e.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
	if e.Data == nil {
		t.Error("expected non-nil data map")
	}
}

func TestEvent_WithRequestID(t *testing.T) {
	e := NewEvent(EventScanStarted, "scanner").WithRequestID("req-123")

	if e.RequestID != "req-123" {
		t.Errorf("expected request_id 'req-123', got %s", e.RequestID)
	}
}

func TestEvent_WithData(t *testing.T) {
	e := NewEvent(EventScanProgress, "scanner").
		WithData("files_scanned", 100).
		WithData("total_files", 1000)

	if e.Data["files_scanned"] != 100 {
		t.Errorf("expected files_scanned=100, got %v", e.Data["files_scanned"])
	}
	if e.Data["total_files"] != 1000 {
		t.Errorf("expected total_files=1000, got %v", e.Data["total_files"])
	}
}

func TestEvent_WithData_NilMap(t *testing.T) {
	// Test that WithData handles nil map by creating one
	e := &Event{Type: EventScanStarted}
	e = e.WithData("key", "value")

	if e.Data == nil {
		t.Error("expected Data map to be created")
	}
	if e.Data["key"] != "value" {
		t.Errorf("expected key='value', got %v", e.Data["key"])
	}
}

func TestEvent_WithMediaID(t *testing.T) {
	e := NewEvent(EventMediaDiscovered, "scanner").WithMediaID(42)

	if e.Data["media_id"] != int64(42) {
		t.Errorf("expected media_id=42, got %v", e.Data["media_id"])
	}
}

func TestEvent_WithLibraryID(t *testing.T) {
	e := NewEvent(EventScanStarted, "scanner").WithLibraryID(5)

	if e.Data["library_id"] != int64(5) {
		t.Errorf("expected library_id=5, got %v", e.Data["library_id"])
	}
}

func TestEvent_WithStage(t *testing.T) {
	e := NewEvent(EventEnrichmentStarted, "pipeline").WithStage("tmdb")

	if e.Data["stage"] != "tmdb" {
		t.Errorf("expected stage='tmdb', got %v", e.Data["stage"])
	}
}

func TestEvent_WithError(t *testing.T) {
	testErr := errors.New("test error")
	e := NewEvent(EventScanFailed, "scanner").WithError(testErr)

	if e.Data["error"] != "test error" {
		t.Errorf("expected error='test error', got %v", e.Data["error"])
	}
}

func TestEvent_WithError_Nil(t *testing.T) {
	e := NewEvent(EventScanFailed, "scanner").WithError(nil)

	if _, ok := e.Data["error"]; ok {
		t.Error("expected no error key for nil error")
	}
}

func TestEvent_WithProgress(t *testing.T) {
	e := NewEvent(EventScanProgress, "scanner").WithProgress(50, 100)

	if e.Data["current"] != int64(50) {
		t.Errorf("expected current=50, got %v", e.Data["current"])
	}
	if e.Data["total"] != int64(100) {
		t.Errorf("expected total=100, got %v", e.Data["total"])
	}
}

func TestEvent_Build(t *testing.T) {
	e := NewEvent(EventMediaDiscovered, "scanner").
		WithMediaID(42).
		WithRequestID("req-123").
		Build()

	// Build returns a value, not a pointer
	if e.Type != EventMediaDiscovered {
		t.Errorf("expected type %s, got %s", EventMediaDiscovered, e.Type)
	}
	if e.Source != "scanner" {
		t.Errorf("expected source 'scanner', got %s", e.Source)
	}
	if e.RequestID != "req-123" {
		t.Errorf("expected request_id 'req-123', got %s", e.RequestID)
	}
	if e.Data["media_id"] != int64(42) {
		t.Errorf("expected media_id=42, got %v", e.Data["media_id"])
	}
}

func TestEvent_Chaining(t *testing.T) {
	before := time.Now()
	e := NewEvent(EventEnrichmentComplete, "enricher").
		WithRequestID("req-456").
		WithMediaID(100).
		WithStage("tmdb").
		WithData("duration_ms", 500).
		Build()
	after := time.Now()

	if e.Type != EventEnrichmentComplete {
		t.Errorf("expected type %s, got %s", EventEnrichmentComplete, e.Type)
	}
	if e.Source != "enricher" {
		t.Errorf("expected source 'enricher', got %s", e.Source)
	}
	if e.RequestID != "req-456" {
		t.Errorf("expected request_id 'req-456', got %s", e.RequestID)
	}
	if e.Timestamp.Before(before) || e.Timestamp.After(after) {
		t.Error("timestamp not in expected range")
	}
	if e.Data["media_id"] != int64(100) {
		t.Errorf("expected media_id=100, got %v", e.Data["media_id"])
	}
	if e.Data["stage"] != "tmdb" {
		t.Errorf("expected stage='tmdb', got %v", e.Data["stage"])
	}
	if e.Data["duration_ms"] != 500 {
		t.Errorf("expected duration_ms=500, got %v", e.Data["duration_ms"])
	}
}

func TestEventType_Constants(t *testing.T) {
	// Verify all event type constants are defined and have expected values
	tests := []struct {
		eventType EventType
		expected  string
	}{
		{EventMediaDiscovered, "media.discovered"},
		{EventMediaUpdated, "media.updated"},
		{EventMediaRemoved, "media.removed"},
		{EventEnrichmentQueued, "enrichment.queued"},
		{EventEnrichmentStarted, "enrichment.started"},
		{EventEnrichmentStageComplete, "enrichment.stage_complete"},
		{EventEnrichmentComplete, "enrichment.complete"},
		{EventEnrichmentFailed, "enrichment.failed"},
		{EventEnrichmentSkipped, "enrichment.skipped"},
		{EventPluginLoaded, "plugin.loaded"},
		{EventPluginUnloaded, "plugin.unloaded"},
		{EventPluginCrashed, "plugin.crashed"},
		{EventPluginHealthUpdate, "plugin.health_update"},
		{EventTranscodeStarted, "transcode.started"},
		{EventTranscodeProgress, "transcode.progress"},
		{EventTranscodeCompleted, "transcode.completed"},
		{EventTranscodeFailed, "transcode.failed"},
		{EventScanStarted, "scan.started"},
		{EventScanProgress, "scan.progress"},
		{EventScanCompleted, "scan.completed"},
		{EventScanFailed, "scan.failed"},
		{EventLog, "log"},
	}

	for _, tc := range tests {
		if string(tc.eventType) != tc.expected {
			t.Errorf("expected %s, got %s", tc.expected, tc.eventType)
		}
	}
}

func TestLogLevel_Constants(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{LogLevelDebug, "debug"},
		{LogLevelInfo, "info"},
		{LogLevelWarn, "warn"},
		{LogLevelError, "error"},
	}

	for _, tc := range tests {
		if string(tc.level) != tc.expected {
			t.Errorf("expected %s, got %s", tc.expected, tc.level)
		}
	}
}
