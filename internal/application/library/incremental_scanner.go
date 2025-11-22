package library

import (
	"context"
	"log/slog"

	"github.com/mantonx/viewra/internal/domain/scanner"
)

// IncrementalScanner detects changes between current filesystem state and last scan.
// Uses mtime+size comparison (no hashing) for fast change detection on large media files.
type IncrementalScanner struct {
	scanStateRepo scanner.ScanStateRepository
	logger        *slog.Logger
}

// NewIncrementalScanner creates a new incremental scanner
func NewIncrementalScanner(
	scanStateRepo scanner.ScanStateRepository,
	logger *slog.Logger,
) *IncrementalScanner {
	return &IncrementalScanner{
		scanStateRepo: scanStateRepo,
		logger:        logger,
	}
}

// DetermineChanges compares current files against previous scan state.
// Returns a diff showing new, modified, deleted, and unchanged files.
//
// Change detection strategy:
// - NEW: File doesn't exist in previous scan state
// - MODIFIED: File exists but mtime OR size has changed
// - DELETED: File in previous scan state but missing from current files
// - UNCHANGED: File exists with identical mtime AND size
//
// Note: No file hashing used - relies on mtime+size for fast comparison.
// This is appropriate for media files which don't get silently modified.
func (is *IncrementalScanner) DetermineChanges(
	ctx context.Context,
	libraryID int64,
	currentFiles []scanner.FileInfo,
) (*scanner.ScanDiff, error) {
	// Get previous scan state
	previousState, err := is.scanStateRepo.GetLibraryState(ctx, libraryID)
	if err != nil {
		return nil, err
	}

	is.logger.Info("comparing scan state",
		"library_id", libraryID,
		"current_files", len(currentFiles),
		"previous_files", len(previousState))

	// Build maps for efficient O(1) lookup
	prevFileMap := make(map[string]*scanner.ScanState)
	for _, state := range previousState {
		prevFileMap[state.FilePath] = state
	}

	currentFileMap := make(map[string]scanner.FileInfo)
	for _, file := range currentFiles {
		currentFileMap[file.Path] = file
	}

	diff := &scanner.ScanDiff{
		NewFiles:       []scanner.FileInfo{},
		ModifiedFiles:  []scanner.FileInfo{},
		DeletedFiles:   []string{},
		UnchangedFiles: []string{},
	}

	// Find new and modified files
	for path, currentFile := range currentFileMap {
		prevState, existed := prevFileMap[path]

		if !existed {
			// New file - didn't exist in previous scan
			diff.NewFiles = append(diff.NewFiles, currentFile)
		} else if is.isFileModified(prevState, currentFile) {
			// Modified file - mtime or size changed
			diff.ModifiedFiles = append(diff.ModifiedFiles, currentFile)
		} else {
			// Unchanged file - identical mtime and size
			diff.UnchangedFiles = append(diff.UnchangedFiles, path)
		}
	}

	// Find deleted files (existed before but now missing)
	for path := range prevFileMap {
		if _, exists := currentFileMap[path]; !exists {
			diff.DeletedFiles = append(diff.DeletedFiles, path)
		}
	}

	is.logger.Info("scan diff calculated", "diff", diff.Summary())

	return diff, nil
}

// isFileModified checks if a file has been modified based on mtime and size.
// Uses fast comparison for incremental scanning efficiency.
//
// Note: Hash-based comparison is intentionally NOT used here because:
// - It would require hashing ALL files just to detect changes
// - This defeats the purpose of incremental scanning (fast change detection)
// - Hashes are computed once during checkpoint creation for only changed files
// - For media files, mtime+size is sufficient (they don't get silently modified)
func (is *IncrementalScanner) isFileModified(prev *scanner.ScanState, current scanner.FileInfo) bool {
	// Quick check: modification time (nanosecond precision)
	if !current.ModTime.Equal(prev.FileMTime) {
		is.logger.Debug("file modified (mtime changed)",
			"path", current.Path,
			"prev_mtime", prev.FileMTime,
			"current_mtime", current.ModTime)
		return true
	}

	// Quick check: size
	if current.Size != prev.FileSize {
		is.logger.Debug("file modified (size changed)",
			"path", current.Path,
			"prev_size", prev.FileSize,
			"current_size", current.Size)
		return true
	}

	// Hash comparison is intentionally disabled during incremental scanning
	// Hashes are computed during checkpoint creation, NOT during change detection
	// This keeps incremental scanning fast (mtime+size check only)
	// Hash-based detection would require hashing ALL files just to detect changes,
	// defeating the purpose of incremental scanning

	return false
}
