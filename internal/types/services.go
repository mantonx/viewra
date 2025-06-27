// Package types provides common types used by service interfaces
package types

import (
	"time"
)

// MediaFilter defines criteria for filtering media files
type MediaFilter struct {
	LibraryID   uint32            `json:"library_id,omitempty"`
	MediaType   string            `json:"media_type,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	MinSize     int64             `json:"min_size,omitempty"`
	MaxSize     int64             `json:"max_size,omitempty"`
	AddedAfter  *time.Time        `json:"added_after,omitempty"`
	AddedBefore *time.Time        `json:"added_before,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Limit       int               `json:"limit,omitempty"`
	Offset      int               `json:"offset,omitempty"`
	SortBy      string            `json:"sort_by,omitempty"`
	SortOrder   string            `json:"sort_order,omitempty"`
}

// PluginStatus represents the current status of a plugin
type PluginStatus struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Version     string                 `json:"version"`
	Type        string                 `json:"type"`
	Enabled     bool                   `json:"enabled"`
	Running     bool                   `json:"running"`
	Healthy     bool                   `json:"healthy"`
	LastError   string                 `json:"last_error,omitempty"`
	LastChecked time.Time              `json:"last_checked"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ScanJob represents an active or completed scan job
type ScanJob struct {
	ID           string     `json:"id"`
	LibraryID    uint32     `json:"library_id"`
	Status       string     `json:"status"` // pending, running, completed, failed, cancelled
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	Progress     float64    `json:"progress"` // 0.0 to 1.0
	FilesFound   int        `json:"files_found"`
	FilesAdded   int        `json:"files_added"`
	FilesUpdated int        `json:"files_updated"`
	FilesDeleted int        `json:"files_deleted"`
	Errors       []string   `json:"errors,omitempty"`
}

// ScanProgress represents the current progress of a scan
type ScanProgress struct {
	JobID        string    `json:"job_id"`
	Progress     float64   `json:"progress"`
	CurrentPath  string    `json:"current_path"`
	FilesScanned int       `json:"files_scanned"`
	BytesScanned int64     `json:"bytes_scanned"`
	Rate         float64   `json:"rate"` // files per second
	ETA          time.Time `json:"eta"`
}

// ScanResult represents the outcome of a completed scan
type ScanResult struct {
	JobID        string        `json:"job_id"`
	LibraryID    uint32        `json:"library_id"`
	StartedAt    time.Time     `json:"started_at"`
	CompletedAt  time.Time     `json:"completed_at"`
	Duration     time.Duration `json:"duration"`
	FilesFound   int           `json:"files_found"`
	FilesAdded   int           `json:"files_added"`
	FilesUpdated int           `json:"files_updated"`
	FilesDeleted int           `json:"files_deleted"`
	BytesScanned int64         `json:"bytes_scanned"`
	Errors       []string      `json:"errors,omitempty"`
	Success      bool          `json:"success"`
}

// EnrichmentStatus represents the status of an enrichment operation
type EnrichmentStatus struct {
	JobID       string                 `json:"job_id"`
	MediaType   string                 `json:"media_type"`
	MediaIDs    []string               `json:"media_ids"`
	Status      string                 `json:"status"` // pending, running, completed, failed
	Progress    float64                `json:"progress"`
	StartedAt   time.Time              `json:"started_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Successful  int                    `json:"successful"`
	Failed      int                    `json:"failed"`
	Skipped     int                    `json:"skipped"`
	Errors      map[string]string      `json:"errors,omitempty"` // mediaID -> error
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}
