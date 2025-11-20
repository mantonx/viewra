package transcode

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/viewra/viewra/internal/domain/transcode"
	"github.com/viewra/viewra/internal/infrastructure/filesystem"
	"github.com/viewra/viewra/internal/infrastructure/transcoding"
)

// CleanupFilter defines criteria for selecting transcodes to clean
type CleanupFilter struct {
	// Status filters
	IncludeCompleted bool
	IncludeFailed    bool
	IncludeQueued    bool

	// Age filters
	OlderThan *time.Duration // Delete if older than this duration

	// Media filters
	MediaID *int64  // Clean transcodes for specific media
	Quality *string // Clean specific quality only

	// Size filters
	MinSizeBytes *int64 // Only clean transcodes larger than this

	// Limit
	Limit *int // Maximum number to clean (for safety)

	// Dry run mode
	DryRun bool // Don't actually delete, just report what would be deleted
}

// NewFailedJobsFilter creates a filter for cleaning failed jobs older than the specified duration.
func NewFailedJobsFilter(olderThan time.Duration, dryRun bool) CleanupFilter {
	return CleanupFilter{
		IncludeFailed: true,
		OlderThan:     &olderThan,
		DryRun:        dryRun,
	}
}

// NewSingleJobFilter creates a filter for cleaning a specific job by media ID and quality.
func NewSingleJobFilter(mediaID int64, quality string, dryRun bool) CleanupFilter {
	return CleanupFilter{
		MediaID: &mediaID,
		Quality: &quality,
		DryRun:  dryRun,
	}
}

// NewOldJobsFilter creates a filter for cleaning jobs older than the specified duration.
func NewOldJobsFilter(olderThan time.Duration, dryRun bool) CleanupFilter {
	return CleanupFilter{
		IncludeCompleted: true,
		IncludeFailed:    true,
		IncludeQueued:    true,
		OlderThan:        &olderThan,
		DryRun:           dryRun,
	}
}

// CleanupResult contains information about cleanup operation
type CleanupResult struct {
	DeletedCount     int
	DeletedSizeBytes int64
	FailedCount      int
	Errors           []error
	DeletedItems     []CleanupItem
}

// CleanupItem represents a single cleaned transcode
type CleanupItem struct {
	JobID       int64
	MediaID     int64
	Quality     string
	Status      string
	SizeBytes   int64
	Path        string
	CreatedAt   time.Time
	CompletedAt *time.Time
}

// CleanupService handles transcode cleanup operations
type CleanupService struct {
	repo      transcode.Repository
	outputDir string
}

// NewCleanupService creates a new cleanup service
func NewCleanupService(repo transcode.Repository, outputDir string) *CleanupService {
	return &CleanupService{
		repo:      repo,
		outputDir: outputDir,
	}
}

// Clean performs cleanup based on the provided filter
func (s *CleanupService) Clean(ctx context.Context, filter CleanupFilter) (*CleanupResult, error) {
	result := &CleanupResult{
		DeletedItems: make([]CleanupItem, 0),
		Errors:       make([]error, 0),
	}

	// Get all transcode jobs
	jobs, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list transcode jobs: %w", err)
	}

	// Convert pointers to values for filtering
	jobValues := make([]transcode.TranscodeJob, 0, len(jobs))
	for _, job := range jobs {
		if job != nil {
			jobValues = append(jobValues, *job)
		}
	}

	// Apply filters and collect candidates
	candidates := s.filterJobs(jobValues, filter)

	// Apply limit if specified
	if filter.Limit != nil && len(candidates) > *filter.Limit {
		candidates = candidates[:*filter.Limit]
	}

	// Delete each candidate
	for _, job := range candidates {
		item := CleanupItem{
			JobID:     job.ID,
			MediaID:   job.MediaID,
			Quality:   job.Quality,
			Status:    job.Status,
			CreatedAt: job.CreatedAt,
		}
		if !job.CompletedAt.IsZero() {
			item.CompletedAt = &job.CompletedAt
		}

		// Calculate output path
		outputPath := s.getOutputPath(job.MediaID, job.Quality)
		item.Path = outputPath

		// Calculate size
		size, err := filesystem.CalculateDirSize(outputPath)
		if err == nil {
			item.SizeBytes = size
		}

		// Delete files (if not dry run)
		if !filter.DryRun {
			if err := s.deleteTranscodeFiles(outputPath); err != nil {
				result.FailedCount++
				result.Errors = append(result.Errors, fmt.Errorf("failed to delete files for job %d: %w", job.ID, err))
				continue
			}

			// Delete database record
			if err := s.repo.Delete(ctx, job.ID); err != nil {
				result.FailedCount++
				result.Errors = append(result.Errors, fmt.Errorf("failed to delete DB record for job %d: %w", job.ID, err))
				continue
			}
		}

		result.DeletedCount++
		result.DeletedSizeBytes += item.SizeBytes
		result.DeletedItems = append(result.DeletedItems, item)
	}

	return result, nil
}

// CleanByMediaID removes all transcodes for a specific media item
func (s *CleanupService) CleanByMediaID(ctx context.Context, mediaID int64, dryRun bool) (*CleanupResult, error) {
	filter := CleanupFilter{
		MediaID:          &mediaID,
		IncludeCompleted: true,
		IncludeFailed:    true,
		IncludeQueued:    true,
		DryRun:           dryRun,
	}
	return s.Clean(ctx, filter)
}

// CleanFailed removes all failed transcode jobs
func (s *CleanupService) CleanFailed(ctx context.Context, olderThan time.Duration, dryRun bool) (*CleanupResult, error) {
	filter := CleanupFilter{
		IncludeFailed: true,
		OlderThan:     &olderThan,
		DryRun:        dryRun,
	}
	return s.Clean(ctx, filter)
}

// CleanOld removes transcodes older than specified duration
func (s *CleanupService) CleanOld(ctx context.Context, olderThan time.Duration, dryRun bool) (*CleanupResult, error) {
	filter := CleanupFilter{
		IncludeCompleted: true,
		OlderThan:        &olderThan,
		DryRun:           dryRun,
	}
	return s.Clean(ctx, filter)
}

// CleanAll removes all transcodes (dangerous!)
func (s *CleanupService) CleanAll(ctx context.Context, dryRun bool) (*CleanupResult, error) {
	filter := CleanupFilter{
		IncludeCompleted: true,
		IncludeFailed:    true,
		IncludeQueued:    true,
		DryRun:           dryRun,
	}
	return s.Clean(ctx, filter)
}

// GetDiskUsage returns current disk usage statistics
func (s *CleanupService) GetDiskUsage(ctx context.Context) (*DiskUsage, error) {
	usage := &DiskUsage{
		OutputDir: s.outputDir,
	}

	// Calculate total size of transcode directory
	totalSize, err := filesystem.CalculateDirSize(s.outputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate directory size: %w", err)
	}
	usage.TotalSizeBytes = totalSize

	// Count files
	fileCount := 0
	err = filepath.Walk(s.outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if !info.IsDir() {
			fileCount++
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}
	usage.FileCount = fileCount

	// Get transcode job counts by status
	jobs, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	for _, job := range jobs {
		switch job.Status {
		case "completed":
			usage.CompletedCount++
		case "failed":
			usage.FailedCount++
		case "queued":
			usage.QueuedCount++
		case "processing":
			usage.ProcessingCount++
		}
	}
	usage.TotalJobs = len(jobs)

	return usage, nil
}

// FindOrphans finds files on disk without corresponding database records
func (s *CleanupService) FindOrphans(ctx context.Context) ([]OrphanedFile, error) {
	orphans := make([]OrphanedFile, 0)

	// Get all jobs from database
	jobs, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	// Build map of valid paths
	validPaths := make(map[string]bool)
	for _, job := range jobs {
		path := s.getOutputPath(job.MediaID, job.Quality)
		validPaths[path] = true
	}

	// Walk the output directory and find orphans
	hlsDir := filepath.Join(s.outputDir, "hls")
	if _, err := os.Stat(hlsDir); os.IsNotExist(err) {
		return orphans, nil // No HLS directory yet
	}

	// Check each media directory
	mediaDirs, err := os.ReadDir(hlsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read HLS directory: %w", err)
	}

	for _, mediaDir := range mediaDirs {
		if !mediaDir.IsDir() {
			continue
		}

		mediaPath := filepath.Join(hlsDir, mediaDir.Name())
		qualityDirs, err := os.ReadDir(mediaPath)
		if err != nil {
			continue // Skip on error
		}

		for _, qualityDir := range qualityDirs {
			if !qualityDir.IsDir() {
				continue
			}

			fullPath := filepath.Join(mediaPath, qualityDir.Name())
			if !validPaths[fullPath] {
				// This is an orphan!
				size, _ := filesystem.CalculateDirSize(fullPath)
				orphans = append(orphans, OrphanedFile{
					Path:      fullPath,
					SizeBytes: size,
				})
			}
		}
	}

	return orphans, nil
}

// CleanOrphans removes orphaned files
func (s *CleanupService) CleanOrphans(ctx context.Context, dryRun bool) (*CleanupResult, error) {
	result := &CleanupResult{
		DeletedItems: make([]CleanupItem, 0),
		Errors:       make([]error, 0),
	}

	orphans, err := s.FindOrphans(ctx)
	if err != nil {
		return nil, err
	}

	for _, orphan := range orphans {
		item := CleanupItem{
			Path:      orphan.Path,
			SizeBytes: orphan.SizeBytes,
		}

		if !dryRun {
			if err := os.RemoveAll(orphan.Path); err != nil {
				result.FailedCount++
				result.Errors = append(result.Errors, fmt.Errorf("failed to delete orphan %s: %w", orphan.Path, err))
				continue
			}

			// Try to remove the parent media ID directory if it's now empty
			parentDir := filepath.Dir(orphan.Path)
			s.removeIfEmpty(parentDir)
		}

		result.DeletedCount++
		result.DeletedSizeBytes += orphan.SizeBytes
		result.DeletedItems = append(result.DeletedItems, item)
	}

	return result, nil
}

// Helper functions
func (s *CleanupService) filterJobs(jobs []transcode.TranscodeJob, filter CleanupFilter) []transcode.TranscodeJob {
	result := make([]transcode.TranscodeJob, 0)

	for _, job := range jobs {
		// Status filter
		if job.Status == "completed" && !filter.IncludeCompleted {
			continue
		}
		if job.Status == "failed" && !filter.IncludeFailed {
			continue
		}
		if job.Status == "queued" && !filter.IncludeQueued {
			continue
		}

		// Media ID filter
		if filter.MediaID != nil && job.MediaID != *filter.MediaID {
			continue
		}

		// Quality filter
		if filter.Quality != nil && job.Quality != *filter.Quality {
			continue
		}

		// Age filter
		if filter.OlderThan != nil {
			age := time.Since(job.CreatedAt)
			if age < *filter.OlderThan {
				continue
			}
		}

		// Size filter (if specified)
		if filter.MinSizeBytes != nil {
			path := s.getOutputPath(job.MediaID, job.Quality)
			size, err := filesystem.CalculateDirSize(path)
			if err != nil || size < *filter.MinSizeBytes {
				continue
			}
		}

		result = append(result, job)
	}

	return result
}

func (s *CleanupService) getOutputPath(mediaID int64, quality string) string {
	return transcoding.GetHLSOutputPath(s.outputDir, mediaID, quality)
}
func (s *CleanupService) deleteTranscodeFiles(outputPath string) error {
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		return nil // Already gone
	}

	// Remove the quality directory (e.g., data/transcode/hls/123/720p/)
	if err := os.RemoveAll(outputPath); err != nil {
		return err
	}

	// Try to remove the parent media ID directory if it's now empty
	// (e.g., data/transcode/hls/123/)
	parentDir := filepath.Dir(outputPath)
	s.removeIfEmpty(parentDir)

	return nil
}

// removeIfEmpty removes a directory if it's empty (best-effort, ignores errors)
func (s *CleanupService) removeIfEmpty(dir string) {
	// Don't try to remove the base HLS directory
	hlsDir := filepath.Join(s.outputDir, "hls")
	if dir == hlsDir || dir == s.outputDir {
		return
	}

	// Check if directory is empty
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) > 0 {
		return // Not empty or error reading
	}

	// Remove if empty (ignore errors - this is cleanup)
	_ = os.Remove(dir)
}

// DiskUsage contains disk usage statistics
type DiskUsage struct {
	OutputDir       string
	TotalSizeBytes  int64
	FileCount       int
	TotalJobs       int
	CompletedCount  int
	FailedCount     int
	QueuedCount     int
	ProcessingCount int
}

// OrphanedFile represents a file on disk without a DB record
type OrphanedFile struct {
	Path      string
	SizeBytes int64
}
