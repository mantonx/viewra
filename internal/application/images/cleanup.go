package images

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/viewra/viewra/internal/domain/images"
)

// CleanupUseCase handles cleanup of orphaned image cache files
type CleanupUseCase struct {
	repo     images.Repository
	cacheDir string
	logger   *slog.Logger
}

// NewCleanupUseCase creates a new cleanup use case
func NewCleanupUseCase(repo images.Repository, cacheDir string, logger *slog.Logger) *CleanupUseCase {
	return &CleanupUseCase{
		repo:     repo,
		cacheDir: cacheDir,
		logger:   logger,
	}
}

// CleanupStats holds statistics about cleanup operation
type CleanupStats struct {
	OrphanedFiles int
	DeletedFiles  int
	Errors        int
	BytesFreed    int64
}

// CleanOrphanedImages removes cache files that are no longer referenced in the database
func (uc *CleanupUseCase) CleanOrphanedImages(ctx context.Context) (*CleanupStats, error) {
	stats := &CleanupStats{}

	// 1. Get all hashes from database
	dbHashes, err := uc.repo.GetAllFileHashes(ctx)
	if err != nil {
		return stats, fmt.Errorf("failed to get database hashes: %w", err)
	}

	// Convert to set for O(1) lookup
	hashSet := make(map[string]bool, len(dbHashes))
	for _, hash := range dbHashes {
		hashSet[hash] = true
	}

	uc.logger.Info("Starting image cache cleanup",
		"cache_dir", uc.cacheDir,
		"db_hashes", len(dbHashes))

	// 2. Scan cache directory
	if _, err := os.Stat(uc.cacheDir); os.IsNotExist(err) {
		uc.logger.Info("Cache directory does not exist, nothing to clean",
			"cache_dir", uc.cacheDir)
		return stats, nil
	}

	cacheFiles, err := filepath.Glob(filepath.Join(uc.cacheDir, "*"))
	if err != nil {
		return stats, fmt.Errorf("failed to scan cache directory: %w", err)
	}

	// 3. Check each cache file
	for _, file := range cacheFiles {
		// Skip directories
		info, err := os.Stat(file)
		if err != nil {
			uc.logger.Warn("Failed to stat cache file", "path", file, "error", err)
			stats.Errors++
			continue
		}
		if info.IsDir() {
			continue
		}

		// Extract hash from filename: {hash}_original.jpg or {hash}_300x450.jpg
		hash := extractHashFromFilename(filepath.Base(file))
		if hash == "" {
			uc.logger.Warn("Could not extract hash from filename", "path", file)
			continue
		}

		// Check if hash is still referenced in database
		if !hashSet[hash] {
			// Orphaned cache file - delete it
			stats.OrphanedFiles++

			fileSize := info.Size()
			if err := os.Remove(file); err != nil {
				uc.logger.Warn("Failed to delete orphaned cache file",
					"path", file,
					"hash", hash,
					"error", err)
				stats.Errors++
				continue
			}

			stats.DeletedFiles++
			stats.BytesFreed += fileSize

			uc.logger.Debug("Deleted orphaned cache file",
				"path", filepath.Base(file),
				"hash", hash,
				"size_bytes", fileSize)
		}
	}

	uc.logger.Info("Image cache cleanup completed",
		"orphaned_files", stats.OrphanedFiles,
		"deleted_files", stats.DeletedFiles,
		"errors", stats.Errors,
		"bytes_freed", stats.BytesFreed)

	return stats, nil
}

// CleanCacheForHash removes all cache variants for a specific hash
// This is called after deleting media to clean up cache files
func (uc *CleanupUseCase) CleanCacheForHash(ctx context.Context, hash string) error {
	if hash == "" {
		return nil
	}

	// Check if hash is still used by other images
	remainingImages, err := uc.repo.GetByHash(ctx, hash)
	if err != nil {
		return fmt.Errorf("failed to check hash usage: %w", err)
	}

	if len(remainingImages) > 0 {
		// Hash still in use by other images - don't delete
		uc.logger.Debug("Hash still in use, skipping cache cleanup",
			"hash", hash,
			"remaining_references", len(remainingImages))
		return nil
	}

	// Hash no longer used - delete all cache variants
	pattern := filepath.Join(uc.cacheDir, hash+"_*")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to glob cache files: %w", err)
	}

	var deletedCount int
	var totalBytes int64

	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			uc.logger.Warn("Failed to stat cache file", "path", file, "error", err)
			continue
		}

		if err := os.Remove(file); err != nil {
			uc.logger.Warn("Failed to delete cache file",
				"path", file,
				"hash", hash,
				"error", err)
			continue
		}

		deletedCount++
		totalBytes += info.Size()
	}

	if deletedCount > 0 {
		uc.logger.Info("Cleaned cache files for hash",
			"hash", hash,
			"deleted_files", deletedCount,
			"bytes_freed", totalBytes)
	}

	return nil
}

// CleanCacheForHashes removes cache files for multiple hashes
func (uc *CleanupUseCase) CleanCacheForHashes(ctx context.Context, hashes []string) error {
	for _, hash := range hashes {
		if err := uc.CleanCacheForHash(ctx, hash); err != nil {
			uc.logger.Error("Failed to clean cache for hash",
				"hash", hash,
				"error", err)
			// Continue with other hashes
		}
	}
	return nil
}

// extractHashFromFilename extracts the hash from a cache filename
// Examples:
//   - "abc123_original.jpg" -> "abc123"
//   - "abc123_300x450.jpg" -> "abc123"
//   - "abc123.jpg" -> "abc123"
func extractHashFromFilename(filename string) string {
	// Remove extension
	nameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Split by underscore
	parts := strings.Split(nameWithoutExt, "_")
	if len(parts) == 0 {
		return ""
	}

	// First part is the hash
	return parts[0]
}
