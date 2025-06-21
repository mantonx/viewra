// Package events provides a comprehensive event bus system for Viewra.
// This enables real-time notifications, plugin interactions, auditing, and analytics.
package events

import (
	"time"
)

// EventType represents the type of event
type EventType string

// System-wide event types
const (
	// Media events
	EventMediaLibraryScanned   EventType = "media.library.scanned"
	EventMediaFileFound        EventType = "media.file.found"
	EventMediaMetadataEnriched EventType = "media.metadata.enriched"
	EventMediaFileDeleted      EventType = "media.file.deleted"
	// EventMediaFileUploaded event type removed as app won't support uploads

	// Media Asset events
	EventAssetCreated   EventType = "asset.created"
	EventAssetUpdated   EventType = "asset.updated"
	EventAssetRemoved   EventType = "asset.removed"
	EventAssetPreferred EventType = "asset.preferred"

	// User events
	EventUserCreated          EventType = "user.created"
	EventUserLoggedIn         EventType = "user.logged_in"
	EventUserDeviceRegistered EventType = "user.device.registered"

	// Playback events
	EventPlaybackStarted  EventType = "playback.started"
	EventPlaybackFinished EventType = "playback.finished"
	EventPlaybackProgress EventType = "playback.progress"

	// System events
	EventSystemStarted EventType = "system.started"
	EventSystemStopped EventType = "system.stopped"

	// Plugin events
	EventPluginLoaded    EventType = "plugin.loaded"
	EventPluginUnloaded  EventType = "plugin.unloaded"
	EventPluginEnabled   EventType = "plugin.enabled"
	EventPluginDisabled  EventType = "plugin.disabled"
	EventPluginInstalled EventType = "plugin.installed"
	EventPluginError     EventType = "plugin.error"

	// Scan events
	EventScanStarted   EventType = "scan.started"
	EventScanProgress  EventType = "scan.progress"
	EventScanCompleted EventType = "scan.completed"
	EventScanFailed    EventType = "scan.failed"
	EventScanResumed   EventType = "scan.resumed"
	EventScanPaused    EventType = "scan.paused"

	// General events
	EventError   EventType = "error"
	EventWarning EventType = "warning"
	EventInfo    EventType = "info"
	EventDebug   EventType = "debug"
)

// EventPriority represents the priority level of an event
type EventPriority int

const (
	PriorityLow      EventPriority = 1
	PriorityNormal   EventPriority = 5
	PriorityHigh     EventPriority = 10
	PriorityCritical EventPriority = 20
)

// Event represents a system event
type Event struct {
	ID        string                 `json:"id"`
	Type      EventType              `json:"type"`
	Source    string                 `json:"source"` // system, plugin:id, user:id, etc.
	Target    string                 `json:"target"` // specific target if applicable
	Title     string                 `json:"title"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data"`
	Priority  EventPriority          `json:"priority"`
	Tags      []string               `json:"tags"`
	Timestamp time.Time              `json:"timestamp"`
	TTL       *time.Duration         `json:"ttl,omitempty"` // Time to live
}

// EventHandler represents a function that handles events
type EventHandler func(event Event) error

// EventFilter represents filters for event subscriptions
type EventFilter struct {
	Types    []EventType    `json:"types,omitempty"`
	Sources  []string       `json:"sources,omitempty"`
	Tags     []string       `json:"tags,omitempty"`
	Priority *EventPriority `json:"priority,omitempty"`
}

// Subscription represents an event subscription
type Subscription struct {
	ID            string       `json:"id"`
	Filter        EventFilter  `json:"filter"`
	Handler       EventHandler `json:"-"`
	Subscriber    string       `json:"subscriber"` // plugin:id, system, user:id
	Created       time.Time    `json:"created"`
	LastTriggered *time.Time   `json:"last_triggered,omitempty"`
	TriggerCount  int64        `json:"trigger_count"`
}

// EventStats represents statistics about events
type EventStats struct {
	TotalEvents         int64            `json:"total_events"`
	EventsByType        map[string]int64 `json:"events_by_type"`
	EventsBySource      map[string]int64 `json:"events_by_source"`
	EventsByPriority    map[string]int64 `json:"events_by_priority"`
	RecentEvents        []Event          `json:"recent_events"`
	ActiveSubscriptions int              `json:"active_subscriptions"`
}

// EventBusConfig represents configuration for the event bus
type EventBusConfig struct {
	BufferSize        int           `json:"buffer_size"`
	MaxEventAge       time.Duration `json:"max_event_age"`
	MaxStoredEvents   int           `json:"max_stored_events"`
	EnablePersistence bool          `json:"enable_persistence"`
	EnableMetrics     bool          `json:"enable_metrics"`
	LogLevel          string        `json:"log_level"`
}

// DefaultEventBusConfig returns default configuration
func DefaultEventBusConfig() EventBusConfig {
	return EventBusConfig{
		BufferSize:        1000,
		MaxEventAge:       24 * time.Hour,
		MaxStoredEvents:   10000,
		EnablePersistence: true,
		EnableMetrics:     true,
		LogLevel:          "info",
	}
}

// =============================================================================
// PREDEFINED EVENT DATA STRUCTURES
// =============================================================================

// MediaLibraryScannedData represents data for media.library.scanned event
type MediaLibraryScannedData struct {
	LibraryID    uint   `json:"library_id"`
	Path         string `json:"path"`
	Type         string `json:"type"`
	DurationMs   int64  `json:"duration_ms"`
	FileCount    int    `json:"file_count"`
	BytesScanned int64  `json:"bytes_scanned"`
	ErrorCount   int    `json:"error_count,omitempty"`
}

// MediaFileFoundData represents data for media.file.found event
type MediaFileFoundData struct {
	Path      string    `json:"path"`
	LibraryID uint      `json:"library_id"`
	Size      int64     `json:"size"`
	Hash      string    `json:"hash"`
	MimeType  string    `json:"mime_type"`
	ModTime   time.Time `json:"mod_time"`
}

// MediaMetadataEnrichedData represents data for media.metadata.enriched event
type MediaMetadataEnrichedData struct {
	MediaID   uint                   `json:"media_id"`
	Title     string                 `json:"title"`
	Source    string                 `json:"source"`
	PosterURL string                 `json:"poster_url,omitempty"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// MediaFileDeletedData represents data for media.file.deleted event
type MediaFileDeletedData struct {
	Path    string `json:"path"`
	MediaID uint   `json:"media_id"`
	Reason  string `json:"reason,omitempty"`
}

// UserCreatedData represents data for user.created event
type UserCreatedData struct {
	UserID    uint      `json:"user_id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// UserLoggedInData represents data for user.logged_in event
type UserLoggedInData struct {
	UserID    uint      `json:"user_id"`
	Username  string    `json:"username"`
	DeviceID  string    `json:"device_id"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// UserDeviceRegisteredData represents data for user.device.registered event
type UserDeviceRegisteredData struct {
	UserID     uint   `json:"user_id"`
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	DeviceType string `json:"device_type"`
	OS         string `json:"os"`
	Browser    string `json:"browser"`
}

// PlaybackStartedData represents data for playback.started event
type PlaybackStartedData struct {
	UserID    uint      `json:"user_id"`
	MediaID   uint      `json:"media_id"`
	DeviceID  string    `json:"device_id"`
	SessionID string    `json:"session_id"`
	Timestamp time.Time `json:"timestamp"`
}

// PlaybackFinishedData represents data for playback.finished event
type PlaybackFinishedData struct {
	UserID          uint    `json:"user_id"`
	MediaID         uint    `json:"media_id"`
	DeviceID        string  `json:"device_id"`
	SessionID       string  `json:"session_id"`
	ProgressSeconds int     `json:"progress_seconds"`
	DurationSeconds int     `json:"duration_seconds"`
	Finished        bool    `json:"finished"`
	WatchPercentage float64 `json:"watch_percentage"`
}

// PlaybackProgressData represents data for playback.progress event
type PlaybackProgressData struct {
	UserID       uint    `json:"user_id"`
	MediaID      uint    `json:"media_id"`
	DeviceID     string  `json:"device_id"`
	SessionID    string  `json:"session_id"`
	CurrentTime  int     `json:"current_time"`
	Duration     int     `json:"duration"`
	BufferedTime int     `json:"buffered_time,omitempty"`
	PlaybackRate float64 `json:"playback_rate"`
	Volume       float64 `json:"volume"`
}

// SystemStartedData represents data for system.started event
type SystemStartedData struct {
	Version      string `json:"version"`
	UptimeMs     int64  `json:"uptime_ms"`
	PluginCount  int    `json:"plugin_count"`
	LibraryCount int    `json:"library_count"`
	UserCount    int    `json:"user_count"`
	MediaCount   int64  `json:"media_count"`
}

// PluginLoadedData represents data for plugin.loaded event
type PluginLoadedData struct {
	PluginID string `json:"plugin_id"`
	Name     string `json:"name"`
	Version  string `json:"version"`
	Type     string `json:"type"`
}

// ScanProgressData represents data for scan progress events
type ScanProgressData struct {
	ScanID         string  `json:"scan_id"`
	LibraryID      uint    `json:"library_id"`
	Progress       float64 `json:"progress"`
	FilesFound     int     `json:"files_found"`
	FilesProcessed int     `json:"files_processed"`
	BytesProcessed int64   `json:"bytes_processed"`
	ErrorCount     int     `json:"error_count,omitempty"`
	CurrentFile    string  `json:"current_file,omitempty"`
}
