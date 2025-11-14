package transcode

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/viewra/viewra/internal/domain/transcode"
	"golang.org/x/sys/unix"
)

// CleanupSchedulerConfig configures the automated cleanup scheduler.
type CleanupSchedulerConfig struct {
	// Enabled determines if automated cleanup is active
	Enabled bool

	// Interval is how often to run cleanup checks
	Interval time.Duration

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
		Interval:             6 * time.Hour,  // Run every 6 hours
		DiskThresholdPercent: 85,              // Cleanup at 85% disk usage
		DiskWarningPercent:   80,              // Warn at 80% disk usage
		MinFreeSpaceGB:       10,              // Require 10GB free
		MaxAgeHours:          720,             // Delete after 30 days (30 * 24 hours)
		MaxIdleHours:         168,             // Delete if not accessed in 7 days (7 * 24 hours)
		MaxStorageGB:         0,               // Unlimited by default
		CleanupBatchSize:     10,              // Delete up to 10 transcodes per run
		KeepFailedHours:      24,              // Keep failed jobs for 24 hours
	}
}

// CleanupScheduler runs automated cleanup jobs on a schedule.
type CleanupScheduler struct {
	config         *CleanupSchedulerConfig
	cleanupService *CleanupService
	repo           transcode.Repository
	outputDir      string
	logger         *slog.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
}

// NewCleanupScheduler creates a new automated cleanup scheduler.
func NewCleanupScheduler(
	config *CleanupSchedulerConfig,
	cleanupService *CleanupService,
	repo transcode.Repository,
	outputDir string,
	logger *slog.Logger,
) *CleanupScheduler {
	if config == nil {
		config = DefaultCleanupSchedulerConfig()
	}

	return &CleanupScheduler{
		config:         config,
		cleanupService: cleanupService,
		repo:           repo,
		outputDir:      outputDir,
		logger:         logger,
	}
}

// Start begins the automated cleanup scheduler.
func (s *CleanupScheduler) Start(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.config.Enabled {
		s.logger.Info("automated cleanup scheduler is disabled")
		return
	}

	s.ctx, s.cancel = context.WithCancel(ctx)
	s.wg.Add(1)

	go s.run()

	s.logger.Info("automated cleanup scheduler started",
		slog.Duration("interval", s.config.Interval),
		slog.Int("disk_threshold", s.config.DiskThresholdPercent),
	)
}

// Stop gracefully stops the cleanup scheduler.
func (s *CleanupScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
		s.wg.Wait()
		s.logger.Info("automated cleanup scheduler stopped")
	}
}

// run is the main scheduler loop.
func (s *CleanupScheduler) run() {
	defer s.wg.Done()

	// Run immediately on start
	s.performCleanup()

	ticker := time.NewTicker(s.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.performCleanup()
		}
	}
}

// performCleanup executes the cleanup logic.
func (s *CleanupScheduler) performCleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	s.logger.Info("running automated cleanup check")

	// 1. Check disk usage
	diskUsage, err := s.getDiskUsage()
	if err != nil {
		s.logger.Error("failed to check disk usage", slog.String("error", err.Error()))
		return
	}

	usagePercent := int(diskUsage.UsedPercent)
	freeSpaceGB := diskUsage.FreeBytes / (1024 * 1024 * 1024)

	s.logger.Info("disk usage check",
		slog.Int("used_percent", usagePercent),
		slog.Int64("free_gb", freeSpaceGB),
		slog.Int64("total_gb", diskUsage.TotalBytes/(1024*1024*1024)),
	)

	// 2. Log warning if approaching threshold
	if usagePercent >= s.config.DiskWarningPercent {
		s.logger.Warn("disk usage approaching threshold",
			slog.Int("used_percent", usagePercent),
			slog.Int("warning_threshold", s.config.DiskWarningPercent),
		)
	}

	// 3. Determine if cleanup is needed
	needsCleanup := false
	reason := ""

	if usagePercent >= s.config.DiskThresholdPercent {
		needsCleanup = true
		reason = fmt.Sprintf("disk usage %d%% exceeds threshold %d%%", usagePercent, s.config.DiskThresholdPercent)
	} else if freeSpaceGB < s.config.MinFreeSpaceGB {
		needsCleanup = true
		reason = fmt.Sprintf("free space %dGB below minimum %dGB", freeSpaceGB, s.config.MinFreeSpaceGB)
	} else if s.config.MaxStorageGB > 0 {
		// Check total transcode storage
		totalSize, err := s.repo.GetTotalSize(ctx)
		if err == nil {
			totalSizeGB := totalSize / (1024 * 1024 * 1024)
			if totalSizeGB > s.config.MaxStorageGB {
				needsCleanup = true
				reason = fmt.Sprintf("transcode storage %dGB exceeds limit %dGB", totalSizeGB, s.config.MaxStorageGB)
			}
		}
	}

	// 4. Always clean old/idle/failed transcodes regardless of disk usage
	s.cleanupByPolicy(ctx)

	// 5. If disk threshold exceeded, perform aggressive LRU cleanup
	if needsCleanup {
		s.logger.Warn("disk threshold exceeded, performing aggressive cleanup", slog.String("reason", reason))
		s.cleanupByLRU(ctx)
	}
}

// cleanupByPolicy removes transcodes based on age/idle/failed policies.
func (s *CleanupScheduler) cleanupByPolicy(ctx context.Context) {
	// Clean failed jobs older than configured hours
	if s.config.KeepFailedHours > 0 {
		olderThan := time.Duration(s.config.KeepFailedHours) * time.Hour
		result, err := s.cleanupService.CleanFailed(ctx, olderThan, false)
		if err != nil {
			s.logger.Error("failed to clean failed jobs", slog.String("error", err.Error()))
		} else if result.DeletedCount > 0 {
			s.logger.Info("cleaned failed transcode jobs",
				slog.Int("count", result.DeletedCount),
				slog.Int64("size_bytes", result.DeletedSizeBytes),
			)
		}
	}

	// Clean old transcodes if max age is set
	if s.config.MaxAgeHours > 0 {
		olderThan := time.Duration(s.config.MaxAgeHours) * time.Hour
		result, err := s.cleanupService.CleanOld(ctx, olderThan, false)
		if err != nil {
			s.logger.Error("failed to clean old transcodes", slog.String("error", err.Error()))
		} else if result.DeletedCount > 0 {
			s.logger.Info("cleaned old transcodes",
				slog.Int("count", result.DeletedCount),
				slog.Int64("size_bytes", result.DeletedSizeBytes),
				slog.Int("max_age_hours", s.config.MaxAgeHours),
			)
		}
	}

	// Clean idle transcodes if max idle is set
	if s.config.MaxIdleHours > 0 {
		idleSince := time.Now().Add(-time.Duration(s.config.MaxIdleHours) * time.Hour)
		result, err := s.cleanupIdleTranscodes(ctx, idleSince)
		if err != nil {
			s.logger.Error("failed to clean idle transcodes", slog.String("error", err.Error()))
		} else if result.DeletedCount > 0 {
			s.logger.Info("cleaned idle transcodes",
				slog.Int("count", result.DeletedCount),
				slog.Int64("size_bytes", result.DeletedSizeBytes),
				slog.Int("max_idle_hours", s.config.MaxIdleHours),
			)
		}
	}

	// Clean orphaned files
	result, err := s.cleanupService.CleanOrphans(ctx, false)
	if err != nil {
		s.logger.Error("failed to clean orphans", slog.String("error", err.Error()))
	} else if result.DeletedCount > 0 {
		s.logger.Info("cleaned orphaned files",
			slog.Int("count", result.DeletedCount),
			slog.Int64("size_bytes", result.DeletedSizeBytes),
		)
	}
}

// cleanupByLRU performs least-recently-used cleanup.
func (s *CleanupScheduler) cleanupByLRU(ctx context.Context) {
	// Get least recently used transcodes
	jobs, err := s.repo.ListByLRU(ctx, s.config.CleanupBatchSize)
	if err != nil {
		s.logger.Error("failed to list LRU transcodes", slog.String("error", err.Error()))
		return
	}

	if len(jobs) == 0 {
		s.logger.Info("no LRU transcodes to clean")
		return
	}

	deletedCount := 0
	var deletedSize int64

	for _, job := range jobs {
		// Delete the transcode
		filter := CleanupFilter{
			MediaID:          &job.MediaID,
			Quality:          &job.Quality,
			IncludeCompleted: true,
			DryRun:           false,
			Limit:            func() *int { i := 1; return &i }(),
		}

		result, err := s.cleanupService.Clean(ctx, filter)
		if err != nil {
			s.logger.Error("failed to clean LRU transcode",
				slog.Int64("job_id", job.ID),
				slog.String("error", err.Error()),
			)
			continue
		}

		deletedCount += result.DeletedCount
		deletedSize += result.DeletedSizeBytes

		s.logger.Debug("deleted LRU transcode",
			slog.Int64("job_id", job.ID),
			slog.Int64("media_id", job.MediaID),
			slog.String("quality", job.Quality),
			slog.Time("last_accessed", job.LastAccessedAt),
		)
	}

	if deletedCount > 0 {
		s.logger.Info("LRU cleanup completed",
			slog.Int("deleted_count", deletedCount),
			slog.Int64("deleted_size_bytes", deletedSize),
		)
	}
}

// cleanupIdleTranscodes removes transcodes that haven't been accessed since the given time.
func (s *CleanupScheduler) cleanupIdleTranscodes(ctx context.Context, idleSince time.Time) (*CleanupResult, error) {
	jobs, err := s.repo.ListAll(ctx)
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

		cleanResult, err := s.cleanupService.Clean(ctx, filter)
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
func (s *CleanupScheduler) getDiskUsage() (*FilesystemUsage, error) {
	var stat unix.Statfs_t
	err := unix.Statfs(s.outputDir, &stat)
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
