package transcode

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	domain "github.com/mantonx/viewra/internal/domain/transcode"
	"golang.org/x/sys/unix"
)

// CleanupSchedulerConfig configures the automated cleanup scheduler.
type CleanupSchedulerConfig struct {
	// Enabled determines if automated cleanup is active
	Enabled bool

	// DiskThresholdPercent triggers cleanup when disk usage exceeds this percentage (e.g., 85)
	DiskThresholdPercent int

	// DiskWarningPercent logs warnings when disk usage exceeds this percentage (e.g., 80)
	DiskWarningPercent int

	// MinFreeSpaceGB is the minimum free space required in GB
	MinFreeSpaceGB int64

	// MaxAgeHours deletes completed transcodes older than this (0 = disabled)
	MaxAgeHours int

	// MaxIdleHours deletes transcodes not accessed in this many hours (0 = disabled)
	MaxIdleHours int

	// MaxStorageGB is the maximum total storage for all transcodes (0 = unlimited)
	MaxStorageGB int64

	// CleanupBatchSize is the maximum number of transcodes to delete per run
	CleanupBatchSize int

	// KeepFailedHours is how long to keep failed jobs before cleanup
	KeepFailedHours int
}

// DefaultCleanupSchedulerConfig returns sensible defaults for automated cleanup.
func DefaultCleanupSchedulerConfig() *CleanupSchedulerConfig {
	return &CleanupSchedulerConfig{
		Enabled:              true,
		DiskThresholdPercent: 85,             // Cleanup at 85% disk usage
		DiskWarningPercent:   80,             // Warn at 80% disk usage
		MinFreeSpaceGB:       10,             // Require 10GB free
		MaxAgeHours:          720,            // Delete after 30 days (30 * 24 hours)
		MaxIdleHours:         168,            // Delete if not accessed in 7 days (7 * 24 hours)
		MaxStorageGB:         0,              // Unlimited by default
		CleanupBatchSize:     10,             // Delete up to 10 transcodes per run
		KeepFailedHours:      24,             // Keep failed jobs for 24 hours
	}
}

// PerformPolicyCleanup executes all policy-based cleanup rules.
// This runs on a schedule (e.g., every 6 hours) and cleans:
// - Failed jobs older than KeepFailedHours
// - Completed jobs older than MaxAgeHours
// - Idle jobs not accessed in MaxIdleHours
// - Orphaned files with no database record
func PerformPolicyCleanup(
	ctx context.Context,
	svc *CleanupService,
	config *CleanupSchedulerConfig,
) error {
	logger := slog.Default().With("task", "transcode-policy-cleanup")
	logger.Info("running policy-based cleanup")

	// 1. Clean failed jobs older than configured hours
	if config.KeepFailedHours > 0 {
		olderThan := time.Duration(config.KeepFailedHours) * time.Hour
		result, err := svc.CleanFailed(ctx, olderThan, false)
		if err != nil {
			logger.Error("failed to clean failed jobs", "error", err)
		} else if result.DeletedCount > 0 {
			logger.Info("cleaned failed transcode jobs",
				"count", result.DeletedCount,
				"size_bytes", result.DeletedSizeBytes)
		}
	}

	// 2. Clean old completed transcodes if max age is set
	if config.MaxAgeHours > 0 {
		olderThan := time.Duration(config.MaxAgeHours) * time.Hour
		result, err := svc.CleanOld(ctx, olderThan, false)
		if err != nil {
			logger.Error("failed to clean old transcodes", "error", err)
		} else if result.DeletedCount > 0 {
			logger.Info("cleaned old transcodes",
				"count", result.DeletedCount,
				"size_bytes", result.DeletedSizeBytes,
				"max_age_hours", config.MaxAgeHours)
		}
	}

	// 3. Clean idle transcodes if max idle is set
	if config.MaxIdleHours > 0 {
		idleSince := time.Now().Add(-time.Duration(config.MaxIdleHours) * time.Hour)
		result, err := cleanIdleTranscodes(ctx, svc, idleSince)
		if err != nil {
			logger.Error("failed to clean idle transcodes", "error", err)
		} else if result.DeletedCount > 0 {
			logger.Info("cleaned idle transcodes",
				"count", result.DeletedCount,
				"size_bytes", result.DeletedSizeBytes,
				"max_idle_hours", config.MaxIdleHours)
		}
	}

	// 4. Clean orphaned files
	result, err := svc.CleanOrphans(ctx, false)
	if err != nil {
		logger.Error("failed to clean orphans", "error", err)
	} else if result.DeletedCount > 0 {
		logger.Info("cleaned orphaned files",
			"count", result.DeletedCount,
			"size_bytes", result.DeletedSizeBytes)
	}

	logger.Info("policy cleanup completed")
	return nil // Don't fail the task for individual cleanup errors
}

// PerformDiskMonitoring checks disk usage and performs LRU cleanup if needed.
// This runs more frequently (e.g., every 30 minutes) than policy cleanup.
// It only performs aggressive cleanup when disk thresholds are exceeded.
func PerformDiskMonitoring(
	ctx context.Context,
	svc *CleanupService,
	repo interface {
		GetTotalSize(ctx context.Context) (int64, error)
		ListByLRU(ctx context.Context, limit int) ([]*domain.TranscodeJob, error)
	},
	config *CleanupSchedulerConfig,
	outputDir string,
) error {
	logger := slog.Default().With("task", "transcode-disk-monitor")

	// Get disk usage
	diskUsage, err := getDiskUsage(outputDir)
	if err != nil {
		return fmt.Errorf("failed to check disk usage: %w", err)
	}

	usagePercent := int(diskUsage.UsedPercent)
	freeSpaceGB := diskUsage.FreeBytes / (1024 * 1024 * 1024)

	logger.Info("disk usage check",
		"used_percent", usagePercent,
		"free_gb", freeSpaceGB,
		"total_gb", diskUsage.TotalBytes/(1024*1024*1024),
		"threshold_percent", config.DiskThresholdPercent)

	// Log warning if approaching threshold
	if usagePercent >= config.DiskWarningPercent {
		logger.Warn("disk usage approaching threshold",
			"used_percent", usagePercent,
			"warning_threshold", config.DiskWarningPercent)
	}

	// Determine if cleanup is needed
	needsCleanup := false
	reason := ""

	if usagePercent >= config.DiskThresholdPercent {
		needsCleanup = true
		reason = fmt.Sprintf("disk usage %d%% exceeds threshold %d%%",
			usagePercent, config.DiskThresholdPercent)
	} else if freeSpaceGB < config.MinFreeSpaceGB {
		needsCleanup = true
		reason = fmt.Sprintf("free space %dGB below minimum %dGB",
			freeSpaceGB, config.MinFreeSpaceGB)
	} else if config.MaxStorageGB > 0 {
		// Check total transcode storage
		totalSize, err := repo.GetTotalSize(ctx)
		if err == nil {
			totalSizeGB := totalSize / (1024 * 1024 * 1024)
			if totalSizeGB > config.MaxStorageGB {
				needsCleanup = true
				reason = fmt.Sprintf("transcode storage %dGB exceeds limit %dGB",
					totalSizeGB, config.MaxStorageGB)
			}
		}
	}

	// Perform LRU cleanup if needed
	if needsCleanup {
		logger.Warn("disk threshold exceeded, performing LRU cleanup",
			"reason", reason)
		return performLRUCleanup(ctx, svc, repo, config)
	}

	return nil
}

// performLRUCleanup deletes least-recently-used transcodes.
func performLRUCleanup(
	ctx context.Context,
	svc *CleanupService,
	repo interface {
		ListByLRU(ctx context.Context, limit int) ([]*domain.TranscodeJob, error)
	},
	config *CleanupSchedulerConfig,
) error {
	logger := slog.Default().With("task", "lru-cleanup")

	// Calculate dynamic batch size based on how far over threshold we are
	// This helps handle rapid disk growth scenarios
	batchSize := config.CleanupBatchSize

	// Get current disk usage to determine pressure level
	diskUsage, err := getDiskUsage(svc.outputDir)
	if err == nil {
		usagePercent := int(diskUsage.UsedPercent)

		// Scale batch size based on how critical the situation is:
		// 85-89%: 1x (default batch size)
		// 90-94%: 2x (moderate pressure)
		// 95-97%: 4x (high pressure)
		// 98%+:   8x (critical - aggressive cleanup)
		if usagePercent >= 98 {
			batchSize = config.CleanupBatchSize * 8
			logger.Warn("critical disk pressure, using 8x batch size",
				"usage_percent", usagePercent,
				"batch_size", batchSize)
		} else if usagePercent >= 95 {
			batchSize = config.CleanupBatchSize * 4
			logger.Warn("high disk pressure, using 4x batch size",
				"usage_percent", usagePercent,
				"batch_size", batchSize)
		} else if usagePercent >= 90 {
			batchSize = config.CleanupBatchSize * 2
			logger.Warn("moderate disk pressure, using 2x batch size",
				"usage_percent", usagePercent,
				"batch_size", batchSize)
		}
	}

	// Get least recently used transcodes
	jobs, err := repo.ListByLRU(ctx, batchSize)
	if err != nil {
		return fmt.Errorf("failed to list LRU transcodes: %w", err)
	}

	if len(jobs) == 0 {
		logger.Info("no LRU transcodes to clean")
		return nil
	}

	deletedCount := 0
	var deletedSize int64

	for _, job := range jobs {
		if job == nil {
			continue
		}

		// Delete the transcode using the cleanup filter
		filter := CleanupFilter{
			MediaID:          &job.MediaID,
			Quality:          &job.Quality,
			IncludeCompleted: true,
			DryRun:           false,
			Limit:            func() *int { i := 1; return &i }(),
		}

		result, err := svc.Clean(ctx, filter)
		if err != nil {
			logger.Error("failed to clean LRU transcode",
				"job_id", job.ID,
				"media_id", job.MediaID,
				"quality", job.Quality,
				"error", err)
			continue
		}

		deletedCount += result.DeletedCount
		deletedSize += result.DeletedSizeBytes

		logger.Debug("deleted LRU transcode",
			"job_id", job.ID,
			"media_id", job.MediaID,
			"quality", job.Quality,
			"last_accessed", job.LastAccessedAt)
	}

	if deletedCount > 0 {
		logger.Info("LRU cleanup completed",
			"deleted_count", deletedCount,
			"deleted_bytes", deletedSize,
			"batch_size", batchSize)
	}

	return nil
}

// cleanIdleTranscodes removes transcodes that haven't been accessed since the given time.
func cleanIdleTranscodes(ctx context.Context, svc *CleanupService, idleSince time.Time) (*CleanupResult, error) {
	jobs, err := svc.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	result := &CleanupResult{
		DeletedItems: make([]CleanupItem, 0),
		Errors:       make([]error, 0),
	}

	for _, job := range jobs {
		if job.Status != "completed" {
			continue
		}

		// Skip if never accessed (newly created)
		if job.LastAccessedAt.IsZero() {
			continue
		}

		// Check if idle
		if job.LastAccessedAt.After(idleSince) {
			continue
		}

		// Delete this idle transcode
		filter := CleanupFilter{
			MediaID:          &job.MediaID,
			Quality:          &job.Quality,
			IncludeCompleted: true,
			DryRun:           false,
			Limit:            func() *int { i := 1; return &i }(),
		}

		cleanResult, err := svc.Clean(ctx, filter)
		if err != nil {
			result.Errors = append(result.Errors, err)
			result.FailedCount++
			continue
		}

		result.DeletedCount += cleanResult.DeletedCount
		result.DeletedSizeBytes += cleanResult.DeletedSizeBytes
		result.DeletedItems = append(result.DeletedItems, cleanResult.DeletedItems...)
	}

	return result, nil
}

// FilesystemUsage contains filesystem disk usage statistics.
type FilesystemUsage struct {
	TotalBytes  int64
	FreeBytes   int64
	UsedBytes   int64
	UsedPercent float64
}

// getDiskUsage returns disk usage statistics for the output directory.
func getDiskUsage(outputDir string) (*FilesystemUsage, error) {
	var stat unix.Statfs_t
	err := unix.Statfs(outputDir, &stat)
	if err != nil {
		return nil, fmt.Errorf("failed to get disk stats: %w", err)
	}

	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bfree * uint64(stat.Bsize)
	used := total - free
	usedPercent := float64(used) / float64(total) * 100

	return &FilesystemUsage{
		TotalBytes:  int64(total),
		FreeBytes:   int64(free),
		UsedBytes:   int64(used),
		UsedPercent: usedPercent,
	}, nil
}
