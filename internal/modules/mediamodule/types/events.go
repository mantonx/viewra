// Package types - Event types
package types

import (
	"time"
)

// MediaEventType represents the type of media event
type MediaEventType string

const (
	// File events
	EventFileAdded    MediaEventType = "media.file.added"
	EventFileUpdated  MediaEventType = "media.file.updated"
	EventFileDeleted  MediaEventType = "media.file.deleted"
	EventFileScanned  MediaEventType = "media.file.scanned"
	
	// Library events
	EventLibraryCreated MediaEventType = "media.library.created"
	EventLibraryUpdated MediaEventType = "media.library.updated"
	EventLibraryDeleted MediaEventType = "media.library.deleted"
	EventLibraryScanStarted MediaEventType = "media.library.scan.started"
	EventLibraryScanCompleted MediaEventType = "media.library.scan.completed"
	EventLibraryScanFailed MediaEventType = "media.library.scan.failed"
	
	// Metadata events
	EventMetadataUpdated MediaEventType = "media.metadata.updated"
	EventMetadataEnriched MediaEventType = "media.metadata.enriched"
	EventMetadataFailed MediaEventType = "media.metadata.failed"
)

// MediaEvent represents a media-related event
type MediaEvent struct {
	Type      MediaEventType         `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// FileAddedEvent represents a file addition event
type FileAddedEvent struct {
	FileID    string `json:"file_id"`
	LibraryID uint32 `json:"library_id"`
	Path      string `json:"path"`
	MediaType string `json:"media_type"`
}

// FileUpdatedEvent represents a file update event
type FileUpdatedEvent struct {
	FileID  string                 `json:"file_id"`
	Changes map[string]interface{} `json:"changes"`
}

// FileDeletedEvent represents a file deletion event
type FileDeletedEvent struct {
	FileID    string `json:"file_id"`
	LibraryID uint32 `json:"library_id"`
	Path      string `json:"path"`
}

// LibraryScanStartedEvent represents a library scan start event
type LibraryScanStartedEvent struct {
	LibraryID uint32 `json:"library_id"`
	ScanType  string `json:"scan_type"` // full, incremental
}

// LibraryScanCompletedEvent represents a library scan completion event
type LibraryScanCompletedEvent struct {
	LibraryID    uint32        `json:"library_id"`
	FilesScanned int           `json:"files_scanned"`
	FilesAdded   int           `json:"files_added"`
	FilesUpdated int           `json:"files_updated"`
	FilesDeleted int           `json:"files_deleted"`
	Duration     time.Duration `json:"duration"`
}

// MetadataEnrichedEvent represents a metadata enrichment event
type MetadataEnrichedEvent struct {
	MediaType string                 `json:"media_type"`
	MediaID   string                 `json:"media_id"`
	Provider  string                 `json:"provider"`
	Metadata  map[string]interface{} `json:"metadata"`
}