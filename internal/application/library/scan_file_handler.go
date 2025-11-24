package library

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/scanner"
)

// processFileWithCheckpoint processes a single file based on library type
// Returns (hasWarning bool, error)
func (uc *ScanLibraryUseCase) processFileWithCheckpoint(ctx context.Context, lib *library.Library, checkpoint *scanner.ScanCheckpoint, existingMediaCache *sync.Map) (bool, error) {
	// Re-extract metadata for this file (checkpoint only stores the path)
	// We need full metadata to create the media entry
	// CRITICAL: os.Stat can hang indefinitely on CIFS/NFS, so we wrap it with a timeout
	fileInfo, err := uc.statWithTimeout(ctx, checkpoint.FilePath, 30*time.Second)
	if err != nil {
		return false, fmt.Errorf("failed to stat file: %w", err)
	}

	// Create a scanner.FileInfo for the coordinator to process
	scanFileInfo := scanner.FileInfo{
		Path:      checkpoint.FilePath,
		Size:      fileInfo.Size(),
		ModTime:   fileInfo.ModTime(),
		IsDir:     false,
		Extension: strings.ToLower(filepath.Ext(checkpoint.FilePath)), // Keep the dot for GetMediaType
	}

	// Add timeout protection to prevent worker deadlocks on slow network storage
	// FFprobe can hang indefinitely on large files over CIFS/NFS
	// Timeout varies based on storage type and file size
	timeout := uc.calculateProcessingTimeout(fileInfo.Size())
	processCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Use the reused coordinator instance (no longer creating new one per file!)
	result := uc.coordinator.ProcessFile(processCtx, scanFileInfo)

	// Check if metadata extraction had an error
	if result.Error != nil {
		// Check if it was a timeout
		if processCtx.Err() == context.DeadlineExceeded {
			uc.logger.Warn("file processing timeout exceeded",
				"file_path", checkpoint.FilePath,
				"file_size", fileInfo.Size(),
				"timeout", timeout)
			return false, fmt.Errorf("processing timeout after %v: %w", timeout, processCtx.Err())
		}
		return false, result.Error
	}

	// Process based on library type and capture the returned media ID
	var mediaID *int64
	var processErr error
	switch lib.Type {
	case library.LibraryTypeMovies:
		mediaID, processErr = uc.processMovie(ctx, lib.ID, &result, checkpoint, existingMediaCache)
	case library.LibraryTypeTV:
		mediaID, processErr = uc.processTVEpisode(ctx, lib.ID, &result, checkpoint, existingMediaCache)
	case library.LibraryTypeMusic:
		mediaID, processErr = uc.processMusicTrack(ctx, lib.ID, &result, checkpoint, existingMediaCache)
	default:
		return false, fmt.Errorf("unknown library type: %s", lib.Type)
	}

	// If media creation/update failed, return the error
	if processErr != nil {
		return false, processErr
	}

	// Update scan state after successful processing
	// This enables incremental scanning on the next scan
	// Use checkpoint hash (computed upfront during checkpoint creation)
	scanState := &scanner.ScanState{
		LibraryID:     lib.ID,
		FilePath:      checkpoint.FilePath,
		FileSize:      fileInfo.Size(),
		FileMTime:     fileInfo.ModTime(),
		FileHash:      checkpoint.FileHash, // Already computed during checkpoint creation
		MediaID:       mediaID,              // Link to created/updated media record
		LastScannedAt: time.Now(),
		ScanJobID:     checkpoint.ScanJobID,
	}

	// Check if file processing had warnings (e.g., FFmpeg metadata extraction failure)
	hasWarning := result.Warning != nil
	if hasWarning {
		// Set persistent warning in scan_state
		scanState.HasWarning = true
		scanState.WarningMessage = result.Warning.Error()
		scanState.WarningCategory = result.WarningCategory
	}
	// If no warning, clear any previous warning (file successfully re-scanned)
	// HasWarning, WarningMessage, WarningCategory default to false/empty

	if err := uc.scanRepos.ScanState.Upsert(ctx, scanState); err != nil {
		// Log error but don't fail the scan - scan state is for optimization, not critical
		uc.logger.Warn("failed to update scan state",
			"file_path", checkpoint.FilePath,
			"error", err)
	}

	return hasWarning, nil
}