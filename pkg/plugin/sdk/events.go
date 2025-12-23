// Event handling helpers for ViewRA plugins.
//
// Plugins can subscribe to events from the host to react to changes
// in the media library, playback state, and more.
//
// # Available Events
//
// Media events:
//   - media.added: New media item added to library
//   - media.updated: Media metadata updated
//   - media.deleted: Media item removed
//
// Playback events:
//   - playback.started: User started playback
//   - playback.stopped: User stopped playback
//   - playback.completed: Playback reached end
//
// Library events:
//   - library.scan.started: Library scan started
//   - library.scan.completed: Library scan finished
//
// # Usage
//
// Implement the EventHandler interface and return subscriptions:
//
//	func (p *MyPlugin) GetSubscriptions() []string {
//	    return []string{"media.added", "media.updated"}
//	}
//
//	func (p *MyPlugin) OnEvent(ctx context.Context, event sdk.Event) bool {
//	    switch event.Type {
//	    case "media.added":
//	        var payload MediaAddedPayload
//	        if err := event.Unmarshal(&payload); err != nil {
//	            return false
//	        }
//	        // Handle new media
//	        return true
//	    }
//	    return false
//	}
package sdk

import (
	"encoding/json"
	"time"
)

// Event types
const (
	// Media events
	EventMediaAdded   = "media.added"
	EventMediaUpdated = "media.updated"
	EventMediaDeleted = "media.deleted"

	// Playback events
	EventPlaybackStarted   = "playback.started"
	EventPlaybackStopped   = "playback.stopped"
	EventPlaybackCompleted = "playback.completed"

	// Library events
	EventLibraryScanStarted   = "library.scan.started"
	EventLibraryScanCompleted = "library.scan.completed"
)

// Event represents an event from the host.
type Event struct {
	// Type is the event type, e.g., "media.added"
	Type string

	// Source identifies who emitted the event
	Source string

	// Timestamp is when the event occurred
	Timestamp time.Time

	// Payload is the JSON-encoded event data
	Payload []byte

	// CorrelationID links related events
	CorrelationID string
}

// Unmarshal decodes the event payload into the given struct.
//
// Example:
//
//	var payload MediaAddedPayload
//	if err := event.Unmarshal(&payload); err != nil {
//	    return false
//	}
func (e *Event) Unmarshal(v any) error {
	return json.Unmarshal(e.Payload, v)
}

// EventHandler is an optional interface for plugins that handle events.
type EventHandler interface {
	// GetSubscriptions returns event types this plugin wants to receive.
	GetSubscriptions() []string

	// OnEvent handles an incoming event.
	// Returns true if the event was handled.
	OnEvent(event Event) bool
}

// Common event payloads

// MediaAddedPayload is the payload for media.added events.
type MediaAddedPayload struct {
	MediaID   int64  `json:"media_id"`
	MediaType string `json:"media_type"`
	Title     string `json:"title"`
	Year      int    `json:"year"`
	LibraryID int64  `json:"library_id"`
}

// MediaUpdatedPayload is the payload for media.updated events.
type MediaUpdatedPayload struct {
	MediaID      int64    `json:"media_id"`
	MediaType    string   `json:"media_type"`
	UpdatedField []string `json:"updated_fields"`
}

// MediaDeletedPayload is the payload for media.deleted events.
type MediaDeletedPayload struct {
	MediaID   int64  `json:"media_id"`
	MediaType string `json:"media_type"`
}

// PlaybackStartedPayload is the payload for playback.started events.
type PlaybackStartedPayload struct {
	SessionID string `json:"session_id"`
	UserID    string `json:"user_id"`
	MediaID   int64  `json:"media_id"`
	MediaType string `json:"media_type"`
}

// PlaybackStoppedPayload is the payload for playback.stopped events.
type PlaybackStoppedPayload struct {
	SessionID    string  `json:"session_id"`
	UserID       string  `json:"user_id"`
	MediaID      int64   `json:"media_id"`
	PositionSecs float64 `json:"position_secs"`
	DurationSecs float64 `json:"duration_secs"`
}

// LibraryScanStartedPayload is the payload for library.scan.started events.
type LibraryScanStartedPayload struct {
	LibraryID int64  `json:"library_id"`
	ScanType  string `json:"scan_type"` // "full" or "incremental"
}

// LibraryScanCompletedPayload is the payload for library.scan.completed events.
type LibraryScanCompletedPayload struct {
	LibraryID   int64 `json:"library_id"`
	ItemsAdded  int   `json:"items_added"`
	ItemsUpdate int   `json:"items_updated"`
	ItemsRemove int   `json:"items_removed"`
	DurationMs  int64 `json:"duration_ms"`
}
