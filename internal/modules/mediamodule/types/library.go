// Package types - Library types
package types

import (
	"time"
)

// LibraryConfig represents configuration for a media library
type LibraryConfig struct {
	Path          string            `json:"path"`
	Type          string            `json:"type"` // movie, tv, music
	ScanEnabled   bool              `json:"scan_enabled"`
	ScanInterval  time.Duration     `json:"scan_interval"`
	FilePatterns  []string          `json:"file_patterns"`
	ExcludePatterns []string        `json:"exclude_patterns"`
	Metadata      map[string]string `json:"metadata"`
}

// LibraryStats represents statistics for a media library
type LibraryStats struct {
	LibraryID      uint32            `json:"library_id"`
	FileCount      int64             `json:"file_count"`
	TotalSize      int64             `json:"total_size"`
	TypeCounts     map[string]int64  `json:"type_counts"`
	CodecStats     []CodecStat       `json:"codec_stats"`
	ResolutionStats []ResolutionStat `json:"resolution_stats"`
	ContainerStats []ContainerStat   `json:"container_stats"`
	LastScan       *time.Time        `json:"last_scan,omitempty"`
	ScanInProgress bool              `json:"scan_in_progress"`
}

// CodecStat represents codec usage statistics
type CodecStat struct {
	VideoCodec string `json:"video_codec"`
	AudioCodec string `json:"audio_codec"`
	Count      int64  `json:"count"`
}

// ResolutionStat represents resolution distribution
type ResolutionStat struct {
	Resolution string `json:"resolution"`
	Count      int64  `json:"count"`
}

// ContainerStat represents container format distribution
type ContainerStat struct {
	Container string `json:"container"`
	Count     int64  `json:"count"`
}

// LibraryScanRequest represents a request to scan a library
type LibraryScanRequest struct {
	LibraryID    uint32 `json:"library_id"`
	FullScan     bool   `json:"full_scan"`
	ForceRefresh bool   `json:"force_refresh"`
}

// LibraryScanStatus represents the current status of a library scan
type LibraryScanStatus struct {
	LibraryID     uint32     `json:"library_id"`
	Status        string     `json:"status"` // idle, scanning, completed, failed
	Progress      float64    `json:"progress"`
	FilesScanned  int        `json:"files_scanned"`
	FilesTotal    int        `json:"files_total"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
}