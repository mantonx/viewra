package scanner

import (
	"context"
	"fmt"
	"time"
)

// ScanState tracks the last-known state of a file for incremental scanning.
// Used to detect new, modified, and deleted files without reprocessing unchanged files.
// Also tracks persistent warnings and errors that remain until the file is successfully re-scanned.
type ScanState struct {
	ID              int64
	LibraryID       int64
	FilePath        string
	FileSize        int64
	FileMTime       time.Time
	FileHash        string // Optional: NULL for mtime+size-only comparison (faster for large files)
	MediaID         *int64
	LastScannedAt   time.Time
	ScanJobID       int64
	CreatedAt       time.Time
	HasWarning      bool
	WarningMessage  string
	WarningCategory string
	HasError        bool
	ErrorMessage    string
	ErrorCategory   string
}

// ScanDiff represents the differences between current filesystem state and last scan
type ScanDiff struct {
	NewFiles       []FileInfo // Files that didn't exist in previous scan
	ModifiedFiles  []FileInfo // Files with changed mtime or size
	DeletedFiles   []string   // Files that existed before but are now missing
	UnchangedFiles []string   // Files with identical mtime and size
}

// NeedsProcessing returns true if there are any changes requiring scan work
func (d *ScanDiff) NeedsProcessing() bool {
	return len(d.NewFiles) > 0 || len(d.ModifiedFiles) > 0 || len(d.DeletedFiles) > 0
}

// Summary returns a human-readable summary of the scan diff
func (d *ScanDiff) Summary() string {
	return fmt.Sprintf("new=%d, modified=%d, deleted=%d, unchanged=%d",
		len(d.NewFiles), len(d.ModifiedFiles), len(d.DeletedFiles), len(d.UnchangedFiles))
}

// TotalChanges returns the count of files needing processing
func (d *ScanDiff) TotalChanges() int {
	return len(d.NewFiles) + len(d.ModifiedFiles) + len(d.DeletedFiles)
}

// LibraryIssueCounts holds counts of warnings and errors for a library
type LibraryIssueCounts struct {
	ErrorCount   int64
	WarningCount int64
}

// ScanStateRepository defines operations for managing scan state
type ScanStateRepository interface {
	// GetLibraryState retrieves all scan state for a library
	GetLibraryState(ctx context.Context, libraryID int64) ([]*ScanState, error)

	// Upsert creates or updates scan state for a file
	Upsert(ctx context.Context, state *ScanState) error

	// UpsertBatch creates or updates scan state for multiple files
	UpsertBatch(ctx context.Context, states []*ScanState) error

	// DeleteByPath removes scan state for a specific file
	DeleteByPath(ctx context.Context, libraryID int64, filePath string) error

	// DeleteByPaths removes scan state for multiple files (for deleted files)
	DeleteByPaths(ctx context.Context, libraryID int64, filePaths []string) error

	// DeleteByLibrary removes all scan state for a library
	DeleteByLibrary(ctx context.Context, libraryID int64) error

	// GetByPath retrieves scan state for a specific file
	GetByPath(ctx context.Context, libraryID int64, filePath string) (*ScanState, error)

	// CountByLibrary returns the number of files tracked for a library
	CountByLibrary(ctx context.Context, libraryID int64) (int64, error)

	// GetLibraryWarnings retrieves all files with warnings for a library
	GetLibraryWarnings(ctx context.Context, libraryID int64) ([]*ScanState, error)

	// GetLibraryErrors retrieves all files with errors for a library
	GetLibraryErrors(ctx context.Context, libraryID int64) ([]*ScanState, error)

	// GetLibraryIssues retrieves all files with either warnings or errors for a library
	GetLibraryIssues(ctx context.Context, libraryID int64) ([]*ScanState, error)

	// CountLibraryIssues returns counts of errors and warnings for a library
	CountLibraryIssues(ctx context.Context, libraryID int64) (*LibraryIssueCounts, error)

	// SetWarning sets a warning for a file
	SetWarning(ctx context.Context, libraryID int64, filePath, message, category string) error

	// SetError sets an error for a file
	SetError(ctx context.Context, libraryID int64, filePath, message, category string) error

	// ClearWarning clears the warning for a file
	ClearWarning(ctx context.Context, libraryID int64, filePath string) error

	// ClearError clears the error for a file
	ClearError(ctx context.Context, libraryID int64, filePath string) error
}
