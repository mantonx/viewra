package scanner

import (
	"time"
)

// ScanStatus represents the state of a scan job
type ScanStatus string

const (
	ScanStatusPending   ScanStatus = "pending"
	ScanStatusRunning   ScanStatus = "running"
	ScanStatusPaused    ScanStatus = "paused"
	ScanStatusCompleted ScanStatus = "completed"
	ScanStatusFailed    ScanStatus = "failed"
)

// ScanJob represents a library scanning operation
type ScanJob struct {
	ID             int64
	LibraryID      int64
	Status         ScanStatus
	Progress       float64
	FilesFound     int64
	FilesProcessed int64
	BytesProcessed int64
	ErrorCount     int64
	StartedAt      time.Time
	CompletedAt    *time.Time
	ErrorMessage   string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// FileInfo represents a discovered file during scanning
type FileInfo struct {
	Path      string
	Size      int64
	ModTime   time.Time
	Extension string
	IsDir     bool
}

// MediaType represents the type of media content
type MediaType string

const (
	MediaTypeMovie   MediaType = "movie"
	MediaTypeEpisode MediaType = "episode"
	MediaTypeTrack   MediaType = "track"
	MediaTypeUnknown MediaType = "unknown"
)

// ScanResult represents the outcome of processing a file
type ScanResult struct {
	FilePath       string
	MediaType      MediaType
	Title          string
	Year           *int
	SeasonNumber   *int // TV episodes only
	EpisodeNumber  *int // TV episodes only
	Artist         string
	Album          string
	TrackNumber    *int
	Duration       int64 // Seconds
	Hash           string
	Error          error
	BytesProcessed int64
}

// Progress tracks scanning progress with thread-safe counters
type Progress struct {
	FilesFound     int64
	FilesProcessed int64
	BytesProcessed int64
	ErrorCount     int64
	StartTime      time.Time
	LastUpdate     time.Time
}

// GetPercentage calculates the completion percentage
func (p *Progress) GetPercentage() float64 {
	if p.FilesFound == 0 {
		return 0
	}
	percentage := float64(p.FilesProcessed) / float64(p.FilesFound) * 100
	if percentage > 100 {
		return 100
	}
	return percentage
}

// HashingStrategy defines how file hashing is performed
type HashingStrategy string

const (
	// HashingStrategyAlways always computes hash for every file
	HashingStrategyAlways HashingStrategy = "always"
	// HashingStrategyOnConflict only hashes files with duplicate sizes
	HashingStrategyOnConflict HashingStrategy = "on_conflict"
	// HashingStrategyDisabled disables hashing (no duplicate detection)
	HashingStrategyDisabled HashingStrategy = "disabled"
)

// FileCacheEntry represents cached metadata for a file
type FileCacheEntry struct {
	Path      string
	Size      int64
	ModTime   time.Time
	Hash      string
	MediaType MediaType

	// Parsed metadata (avoid reparsing)
	Title         string
	Artist        string
	Album         string
	Year          *int
	SeasonNumber  *int
	EpisodeNumber *int
	TrackNumber   *int
}

// IsUnchanged returns true if the file has not been modified
func (e *FileCacheEntry) IsUnchanged(modTime time.Time, size int64) bool {
	return e.ModTime.Equal(modTime) && e.Size == size
}
