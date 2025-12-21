// Package events provides the event bus infrastructure for ViewRA.
//
// Domain types (Event, EventType, Publisher, etc.) are in domain/events.
// This package provides the concrete Bus implementation.
//
// Usage:
//
//	import "github.com/mantonx/viewra/internal/infrastructure/events"
//
//	bus := events.NewBus(10000, logger)
//	bus.Publish(domainevents.Event{...})
package events

import (
	"log/slog"

	domainevents "github.com/mantonx/viewra/internal/domain/events"
	"github.com/mantonx/viewra/internal/infrastructure/events/bus"
	eventslog "github.com/mantonx/viewra/internal/infrastructure/events/slog"
)

// Type aliases for sub-package types.
// This allows consumers to import just this package.
type (
	// Bus is the central event bus for pub/sub communication.
	Bus = bus.Bus

	// Stats contains event bus statistics.
	Stats = bus.Stats

	// BusHandler is an slog.Handler that publishes to the event bus.
	BusHandler = eventslog.BusHandler

	// MultiHandler combines multiple slog handlers into one.
	MultiHandler = eventslog.MultiHandler
)

// Re-export domain types for convenience.
// This allows consumers to import just this package for everything event-related.
type (
	// Event represents an event in the system.
	Event = domainevents.Event

	// EventType categorizes events for filtering.
	EventType = domainevents.EventType

	// Publisher is the interface for publishing events.
	Publisher = domainevents.Publisher

	// Subscriber provides access to event streams.
	Subscriber = domainevents.Subscriber

	// Subscription represents an active subscription.
	Subscription = domainevents.Subscription

	// LogLevel represents severity for log events.
	LogLevel = domainevents.LogLevel
)

// Re-export domain event type constants.
const (
	// Media lifecycle events
	EventMediaDiscovered = domainevents.EventMediaDiscovered
	EventMediaUpdated    = domainevents.EventMediaUpdated
	EventMediaRemoved    = domainevents.EventMediaRemoved

	// Enrichment lifecycle events
	EventEnrichmentQueued        = domainevents.EventEnrichmentQueued
	EventEnrichmentStarted       = domainevents.EventEnrichmentStarted
	EventEnrichmentStageComplete = domainevents.EventEnrichmentStageComplete
	EventEnrichmentComplete      = domainevents.EventEnrichmentComplete
	EventEnrichmentFailed        = domainevents.EventEnrichmentFailed
	EventEnrichmentSkipped       = domainevents.EventEnrichmentSkipped

	// Plugin lifecycle events
	EventPluginLoaded       = domainevents.EventPluginLoaded
	EventPluginUnloaded     = domainevents.EventPluginUnloaded
	EventPluginCrashed      = domainevents.EventPluginCrashed
	EventPluginHealthUpdate = domainevents.EventPluginHealthUpdate

	// Transcode events
	EventTranscodeStarted   = domainevents.EventTranscodeStarted
	EventTranscodeProgress  = domainevents.EventTranscodeProgress
	EventTranscodeCompleted = domainevents.EventTranscodeCompleted
	EventTranscodeFailed    = domainevents.EventTranscodeFailed

	// Scan events
	EventScanStarted   = domainevents.EventScanStarted
	EventScanProgress  = domainevents.EventScanProgress
	EventScanCompleted = domainevents.EventScanCompleted
	EventScanFailed    = domainevents.EventScanFailed

	// Playback events
	EventPlaybackStarted  = domainevents.EventPlaybackStarted
	EventPlaybackPaused   = domainevents.EventPlaybackPaused
	EventPlaybackResumed  = domainevents.EventPlaybackResumed
	EventPlaybackProgress = domainevents.EventPlaybackProgress
	EventPlaybackStopped  = domainevents.EventPlaybackStopped

	// User events
	EventUserLogin  = domainevents.EventUserLogin
	EventUserLogout = domainevents.EventUserLogout

	// System events
	EventSystemLowDiskSpace     = domainevents.EventSystemLowDiskSpace
	EventSystemRestartRequested = domainevents.EventSystemRestartRequested
	EventSystemRestartScheduled = domainevents.EventSystemRestartScheduled
	EventSystemRestartCancelled = domainevents.EventSystemRestartCancelled
	EventSystemShuttingDown     = domainevents.EventSystemShuttingDown

	// Settings events
	EventSettingsChanged        = domainevents.EventSettingsChanged
	EventSettingsPendingRestart = domainevents.EventSettingsPendingRestart

	// Log events
	EventLog = domainevents.EventLog

	// Log levels
	LogLevelDebug = domainevents.LogLevelDebug
	LogLevelInfo  = domainevents.LogLevelInfo
	LogLevelWarn  = domainevents.LogLevelWarn
	LogLevelError = domainevents.LogLevelError
)

// Re-export domain subscribe options for convenience.
// This allows consumers to import just this package for everything event-related.
var (
	WithBufferSize  = domainevents.WithBufferSize
	WithFilter      = domainevents.WithFilter
	WithReplayLast  = domainevents.WithReplayLast
	WithEventTypes  = domainevents.WithEventTypes
	WithEventPrefix = domainevents.WithEventPrefix
)

// NewEvent creates a new event with the current timestamp.
// Re-exported from domain/events for convenience.
var NewEvent = domainevents.NewEvent

// NewBus creates a new event bus with the specified ring buffer size.
func NewBus(bufferSize int, logger *slog.Logger) *Bus {
	return bus.New(bufferSize, logger)
}

// NewBusHandler creates an slog handler that publishes to the event bus.
func NewBusHandler(pub domainevents.Publisher, source string) *BusHandler {
	return eventslog.NewBusHandler(pub, source)
}

// NewMultiHandler creates a handler that writes to multiple slog destinations.
func NewMultiHandler(handlers ...slog.Handler) *MultiHandler {
	return eventslog.NewMultiHandler(handlers...)
}
