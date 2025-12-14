// Package events defines domain types for the ViewRA event system.
// Events enable pub/sub communication across all system components:
// scanning, enrichment, transcoding, plugins, and more.
//
// This package contains only types and interfaces - no implementation.
// For the concrete event bus, see infrastructure/events.
package events

import (
	"time"
)

// EventType categorizes events for filtering and handling.
type EventType string

// Domain Events - semantic events about system state changes.
const (
	// Media lifecycle events
	EventMediaDiscovered EventType = "media.discovered"
	EventMediaUpdated    EventType = "media.updated"
	EventMediaRemoved    EventType = "media.removed"

	// Enrichment lifecycle events
	EventEnrichmentQueued        EventType = "enrichment.queued"
	EventEnrichmentStarted       EventType = "enrichment.started"
	EventEnrichmentStageComplete EventType = "enrichment.stage_complete"
	EventEnrichmentComplete      EventType = "enrichment.complete"
	EventEnrichmentFailed        EventType = "enrichment.failed"
	EventEnrichmentSkipped       EventType = "enrichment.skipped"

	// Plugin lifecycle events
	EventPluginLoaded       EventType = "plugin.loaded"
	EventPluginUnloaded     EventType = "plugin.unloaded"
	EventPluginCrashed      EventType = "plugin.crashed"
	EventPluginHealthUpdate EventType = "plugin.health_update"

	// Transcode events
	EventTranscodeStarted   EventType = "transcode.started"
	EventTranscodeProgress  EventType = "transcode.progress"
	EventTranscodeCompleted EventType = "transcode.completed"
	EventTranscodeFailed    EventType = "transcode.failed"

	// Scan events
	EventScanStarted   EventType = "scan.started"
	EventScanProgress  EventType = "scan.progress"
	EventScanCompleted EventType = "scan.completed"
	EventScanFailed    EventType = "scan.failed"

	// Log events (from slog integration)
	EventLog EventType = "log"
)

// Event represents an event in the system.
// Events are immutable once created and contain all relevant context.
type Event struct {
	// Type categorizes the event for filtering
	Type EventType `json:"type"`

	// Timestamp when the event occurred
	Timestamp time.Time `json:"timestamp"`

	// Source identifies the component or plugin that generated the event
	Source string `json:"source"`

	// RequestID enables correlation across components (optional)
	RequestID string `json:"request_id,omitempty"`

	// Data contains event-specific payload
	Data map[string]any `json:"data,omitempty"`
}

// NewEvent creates a new event with the current timestamp.
func NewEvent(eventType EventType, source string) *Event {
	return &Event{
		Type:      eventType,
		Timestamp: time.Now(),
		Source:    source,
		Data:      make(map[string]any),
	}
}

// WithRequestID sets the correlation ID for tracing.
func (e *Event) WithRequestID(requestID string) *Event {
	e.RequestID = requestID
	return e
}

// WithData adds a key-value pair to the event data.
func (e *Event) WithData(key string, value any) *Event {
	if e.Data == nil {
		e.Data = make(map[string]any)
	}
	e.Data[key] = value
	return e
}

// WithMediaID is a convenience method for media-related events.
func (e *Event) WithMediaID(mediaID int64) *Event {
	return e.WithData("media_id", mediaID)
}

// WithLibraryID is a convenience method for library-related events.
func (e *Event) WithLibraryID(libraryID int64) *Event {
	return e.WithData("library_id", libraryID)
}

// WithStage is a convenience method for enrichment events.
func (e *Event) WithStage(stage string) *Event {
	return e.WithData("stage", stage)
}

// WithError is a convenience method for error events.
func (e *Event) WithError(err error) *Event {
	if err != nil {
		return e.WithData("error", err.Error())
	}
	return e
}

// WithProgress is a convenience method for progress events.
func (e *Event) WithProgress(current, total int64) *Event {
	return e.WithData("current", current).WithData("total", total)
}

// Build returns the event value (for chaining convenience).
func (e *Event) Build() Event {
	return *e
}

// LogLevel represents severity for log events.
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)
