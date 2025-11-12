package scanner

import (
	"context"
	"os"
)

// WalkFunc is called for each file discovered during directory traversal
type WalkFunc func(info FileInfo) error

// FileWalker discovers files in a directory recursively
type FileWalker interface {
	// Walk traverses a directory tree, calling walkFn for each file
	Walk(ctx context.Context, root string, walkFn WalkFunc) error
}

// FileFilter determines if a file should be processed
type FileFilter interface {
	// ShouldProcess returns true if the file should be processed as media
	ShouldProcess(path string, info os.FileInfo) bool

	// IsMediaFile returns true if the file extension indicates media content
	IsMediaFile(path string) bool

	// GetMediaType determines the media type based on file extension
	GetMediaType(extension string) MediaType
}

// ScanJobRepository handles persistence of scan jobs
type ScanJobRepository interface {
	// Create creates a new scan job
	Create(ctx context.Context, job *ScanJob) error

	// GetByID retrieves a scan job by its ID
	GetByID(ctx context.Context, id int64) (*ScanJob, error)

	// GetLatestByLibrary retrieves the most recent scan job for a library
	GetLatestByLibrary(ctx context.Context, libraryID int64) (*ScanJob, error)

	// ListByLibrary retrieves scan jobs for a library, ordered by creation date (newest first)
	ListByLibrary(ctx context.Context, libraryID int64, limit int32) ([]*ScanJob, error)

	// ListRunning retrieves all currently running scan jobs
	ListRunning(ctx context.Context) ([]*ScanJob, error)

	// UpdateProgress updates the progress counters for a scan job
	UpdateProgress(ctx context.Context, id int64, progress *Progress) error

	// UpdateStatus updates the status of a scan job
	UpdateStatus(ctx context.Context, id int64, status ScanStatus) error

	// Complete marks a scan job as completed or failed
	Complete(ctx context.Context, job *ScanJob) error

	// Delete deletes a scan job
	Delete(ctx context.Context, id int64) error

	// DeleteOld deletes old completed/failed scan jobs for a library
	DeleteOld(ctx context.Context, libraryID int64, retentionDays int) error
}
