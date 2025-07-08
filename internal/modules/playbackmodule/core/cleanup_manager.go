// Package core provides the core functionality for the playback module.
package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/modules/playbackmodule/models"
	"github.com/mantonx/viewra/internal/services"
	"github.com/mantonx/viewra/internal/utils"
	"gorm.io/gorm"
)

// CleanupConfig holds configuration for cleanup operations
type CleanupConfig struct {
	// Session cleanup
	SessionInactiveThreshold time.Duration // Default: 30 minutes
	SessionMaxAge            time.Duration // Default: 24 hours

	// Transcode cleanup
	TranscodeUnusedThreshold time.Duration // Default: 2 hours
	TranscodeMaxAge          time.Duration // Default: 7 days
	TranscodeCacheSizeLimit  int64         // Default: 100GB

	// Cleanup intervals
	CleanupInterval     time.Duration // Default: 15 minutes
	DeepCleanupInterval time.Duration // Default: 1 hour
}

// DefaultCleanupConfig returns default cleanup configuration
func DefaultCleanupConfig() *CleanupConfig {
	return &CleanupConfig{
		SessionInactiveThreshold: 30 * time.Minute,
		SessionMaxAge:            24 * time.Hour,
		TranscodeUnusedThreshold: 2 * time.Hour,
		TranscodeMaxAge:          7 * 24 * time.Hour,
		TranscodeCacheSizeLimit:  100 * 1024 * 1024 * 1024, // 100GB
		CleanupInterval:          15 * time.Minute,
		DeepCleanupInterval:      1 * time.Hour,
	}
}

// CleanupManager handles cleanup of orphaned sessions and transcodes
type CleanupManager struct {
	logger             hclog.Logger
	db                 *gorm.DB
	config             *CleanupConfig
	transcodingService services.TranscodingService

	mu              sync.RWMutex
	stopCh          chan struct{}
	wg              sync.WaitGroup
	isRunning       bool
	lastCleanup     time.Time
	lastDeepCleanup time.Time
}

// NewCleanupManager creates a new cleanup manager
func NewCleanupManager(logger hclog.Logger, db *gorm.DB, config *CleanupConfig, transcodingService services.TranscodingService) *CleanupManager {
	if config == nil {
		config = DefaultCleanupConfig()
	}

	return &CleanupManager{
		logger:             logger,
		db:                 db,
		config:             config,
		transcodingService: transcodingService,
		stopCh:             make(chan struct{}),
	}
}

// Start begins the cleanup routine
func (cm *CleanupManager) Start() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.isRunning {
		return fmt.Errorf("cleanup manager already running")
	}

	cm.isRunning = true
	cm.wg.Add(1)
	go cm.cleanupRoutine()

	cm.logger.Info("Cleanup manager started",
		"cleanupInterval", cm.config.CleanupInterval,
		"deepCleanupInterval", cm.config.DeepCleanupInterval)

	return nil
}

// Stop gracefully stops the cleanup routine
func (cm *CleanupManager) Stop() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if !cm.isRunning {
		return nil
	}

	close(cm.stopCh)
	cm.wg.Wait()
	cm.isRunning = false

	cm.logger.Info("Cleanup manager stopped")
	return nil
}

// cleanupRoutine runs periodic cleanup tasks
func (cm *CleanupManager) cleanupRoutine() {
	defer cm.wg.Done()

	// Run initial cleanup
	cm.runCleanup(false)

	cleanupTicker := time.NewTicker(cm.config.CleanupInterval)
	defer cleanupTicker.Stop()

	deepCleanupTicker := time.NewTicker(cm.config.DeepCleanupInterval)
	defer deepCleanupTicker.Stop()

	for {
		select {
		case <-cm.stopCh:
			return
		case <-cleanupTicker.C:
			cm.runCleanup(false)
		case <-deepCleanupTicker.C:
			cm.runCleanup(true)
		}
	}
}

// runCleanup performs cleanup tasks
func (cm *CleanupManager) runCleanup(deep bool) {
	ctx := context.Background()

	cm.logger.Debug("Running cleanup", "deep", deep)

	// Clean up inactive sessions
	if err := cm.cleanupInactiveSessions(ctx); err != nil {
		cm.logger.Error("Failed to cleanup inactive sessions", "error", err)
	}

	// Clean up old sessions
	if err := cm.cleanupOldSessions(ctx); err != nil {
		cm.logger.Error("Failed to cleanup old sessions", "error", err)
	}

	// Clean up orphaned transcodes
	if err := cm.cleanupOrphanedTranscodes(ctx); err != nil {
		cm.logger.Error("Failed to cleanup orphaned transcodes", "error", err)
	}

	if deep {
		// Deep cleanup tasks
		if err := cm.cleanupUnusedTranscodes(ctx); err != nil {
			cm.logger.Error("Failed to cleanup unused transcodes", "error", err)
		}

		if err := cm.enforceTranscodeCacheLimit(ctx); err != nil {
			cm.logger.Error("Failed to enforce transcode cache limit", "error", err)
		}

		if err := cm.verifyFileSystemIntegrity(ctx); err != nil {
			cm.logger.Error("Failed to verify filesystem integrity", "error", err)
		}
	}

	cm.mu.Lock()
	if deep {
		cm.lastDeepCleanup = time.Now()
	} else {
		cm.lastCleanup = time.Now()
	}
	cm.mu.Unlock()
}

// cleanupInactiveSessions cleans up sessions that have been inactive
func (cm *CleanupManager) cleanupInactiveSessions(ctx context.Context) error {
	threshold := time.Now().Add(-cm.config.SessionInactiveThreshold)

	var sessions []models.PlaybackSession
	err := cm.db.Where("last_activity < ? AND state IN ?", threshold, []string{"playing", "paused"}).
		Find(&sessions).Error
	if err != nil {
		return fmt.Errorf("failed to find inactive sessions: %w", err)
	}

	for _, session := range sessions {
		// Mark session as stopped
		now := time.Now()
		session.State = "stopped"
		session.EndTime = &now

		if err := cm.db.Save(&session).Error; err != nil {
			cm.logger.Error("Failed to update inactive session",
				"sessionID", session.ID,
				"error", err)
			continue
		}

		cm.logger.Info("Cleaned up inactive session",
			"sessionID", session.ID,
			"lastActivity", session.LastActivity)
	}

	return nil
}

// cleanupOldSessions removes sessions older than max age
func (cm *CleanupManager) cleanupOldSessions(ctx context.Context) error {
	threshold := time.Now().Add(-cm.config.SessionMaxAge)

	result := cm.db.Where("created_at < ?", threshold).Delete(&models.PlaybackSession{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete old sessions: %w", result.Error)
	}

	if result.RowsAffected > 0 {
		cm.logger.Info("Deleted old sessions",
			"count", result.RowsAffected,
			"threshold", threshold)
	}

	return nil
}

// cleanupOrphanedTranscodes removes transcodes without active sessions
func (cm *CleanupManager) cleanupOrphanedTranscodes(ctx context.Context) error {
	// Find all active transcode IDs from sessions
	var activeTranscodeIDs []string
	err := cm.db.Model(&models.PlaybackSession{}).
		Where("transcode_id IS NOT NULL AND transcode_id != ''").
		Where("state IN ?", []string{"playing", "paused"}).
		Pluck("transcode_id", &activeTranscodeIDs).Error
	if err != nil {
		return fmt.Errorf("failed to get active transcode IDs: %w", err)
	}

	// Find cleanup tasks for orphaned transcodes
	var tasks []models.TranscodeCleanupTask
	query := cm.db.Where("status = ?", "pending")
	if len(activeTranscodeIDs) > 0 {
		query = query.Where("transcode_id NOT IN ?", activeTranscodeIDs)
	}

	err = query.Find(&tasks).Error
	if err != nil {
		return fmt.Errorf("failed to find orphaned transcodes: %w", err)
	}

	for _, task := range tasks {
		if err := cm.cleanupTranscodeTask(ctx, &task); err != nil {
			cm.logger.Error("Failed to cleanup transcode",
				"transcodeID", task.TranscodeID,
				"error", err)
		}
	}

	return nil
}

// cleanupUnusedTranscodes removes transcodes that haven't been used recently
func (cm *CleanupManager) cleanupUnusedTranscodes(ctx context.Context) error {
	threshold := time.Now().Add(-cm.config.TranscodeUnusedThreshold)

	var tasks []models.TranscodeCleanupTask
	err := cm.db.Where("last_used < ? AND status = ?", threshold, "pending").
		Find(&tasks).Error
	if err != nil {
		return fmt.Errorf("failed to find unused transcodes: %w", err)
	}

	for _, task := range tasks {
		if err := cm.cleanupTranscodeTask(ctx, &task); err != nil {
			cm.logger.Error("Failed to cleanup unused transcode",
				"transcodeID", task.TranscodeID,
				"error", err)
		}
	}

	return nil
}

// cleanupTranscodeTask removes a transcode file and updates the database
func (cm *CleanupManager) cleanupTranscodeTask(ctx context.Context, task *models.TranscodeCleanupTask) error {
	// Remove the file
	if err := os.Remove(task.FilePath); err != nil && !os.IsNotExist(err) {
		task.Status = "failed"
		cm.db.Save(task)
		return fmt.Errorf("failed to remove transcode file: %w", err)
	}

	// Mark as cleaned
	task.Status = "cleaned"
	task.CleanupAt = time.Now()

	if err := cm.db.Save(task).Error; err != nil {
		return fmt.Errorf("failed to update cleanup task: %w", err)
	}

	cm.logger.Info("Cleaned up transcode",
		"transcodeID", task.TranscodeID,
		"filePath", task.FilePath,
		"fileSize", task.FileSize)

	return nil
}

// enforceTranscodeCacheLimit ensures transcode cache doesn't exceed size limit
func (cm *CleanupManager) enforceTranscodeCacheLimit(ctx context.Context) error {
	// Calculate total size of pending transcodes
	var totalSize int64
	err := cm.db.Model(&models.TranscodeCleanupTask{}).
		Where("status = ?", "pending").
		Select("COALESCE(SUM(file_size), 0)").
		Scan(&totalSize).Error
	if err != nil {
		return fmt.Errorf("failed to calculate transcode cache size: %w", err)
	}

	if totalSize <= cm.config.TranscodeCacheSizeLimit {
		return nil
	}

	// Remove oldest transcodes until under limit
	excessSize := totalSize - cm.config.TranscodeCacheSizeLimit

	var tasks []models.TranscodeCleanupTask
	err = cm.db.Where("status = ?", "pending").
		Order("last_used ASC").
		Find(&tasks).Error
	if err != nil {
		return fmt.Errorf("failed to find transcodes for cleanup: %w", err)
	}

	var cleanedSize int64
	for _, task := range tasks {
		if cleanedSize >= excessSize {
			break
		}

		if err := cm.cleanupTranscodeTask(ctx, &task); err != nil {
			cm.logger.Error("Failed to cleanup transcode for cache limit",
				"transcodeID", task.TranscodeID,
				"error", err)
			continue
		}

		cleanedSize += task.FileSize
	}

	cm.logger.Info("Enforced transcode cache limit",
		"totalSize", totalSize,
		"limit", cm.config.TranscodeCacheSizeLimit,
		"cleanedSize", cleanedSize)

	return nil
}

// verifyFileSystemIntegrity checks for orphaned files not in database
func (cm *CleanupManager) verifyFileSystemIntegrity(ctx context.Context) error {
	// Get transcode directory from transcoding service
	// This is a simplified version - in reality we'd need to get the actual directory
	transcodeDir := "/app/viewra-data/transcodes"

	// Walk through transcode directory
	orphanedFiles := 0
	orphanedSize := int64(0)

	err := filepath.Walk(transcodeDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		if info.IsDir() {
			return nil
		}

		// Check if file is tracked in database
		var count int64
		cm.db.Model(&models.TranscodeCleanupTask{}).
			Where("file_path = ?", path).
			Count(&count)

		if count == 0 {
			// Orphaned file - create cleanup task
			task := models.TranscodeCleanupTask{
				ID:          utils.GenerateUUID(),
				TranscodeID: filepath.Base(path),
				FilePath:    path,
				FileSize:    info.Size(),
				CreatedAt:   info.ModTime(),
				LastUsed:    info.ModTime(),
				CleanupAt:   time.Now(),
				Status:      "pending",
			}

			if err := cm.db.Create(&task).Error; err != nil {
				cm.logger.Error("Failed to create cleanup task for orphaned file",
					"path", path,
					"error", err)
			} else {
				orphanedFiles++
				orphanedSize += info.Size()
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk transcode directory: %w", err)
	}

	if orphanedFiles > 0 {
		cm.logger.Info("Found orphaned transcode files",
			"count", orphanedFiles,
			"totalSize", orphanedSize)
	}

	return nil
}

// RegisterTranscode registers a new transcode for cleanup tracking
func (cm *CleanupManager) RegisterTranscode(transcodeID, filePath string, fileSize int64) error {
	task := models.TranscodeCleanupTask{
		ID:          utils.GenerateUUID(),
		TranscodeID: transcodeID,
		FilePath:    filePath,
		FileSize:    fileSize,
		CreatedAt:   time.Now(),
		LastUsed:    time.Now(),
		Status:      "pending",
	}

	return cm.db.Create(&task).Error
}

// UpdateTranscodeUsage updates the last used time for a transcode
func (cm *CleanupManager) UpdateTranscodeUsage(transcodeID string) error {
	return cm.db.Model(&models.TranscodeCleanupTask{}).
		Where("transcode_id = ?", transcodeID).
		Update("last_used", time.Now()).Error
}

// GetCleanupStats returns cleanup statistics
func (cm *CleanupManager) GetCleanupStats() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	stats := map[string]interface{}{
		"is_running":        cm.isRunning,
		"last_cleanup":      cm.lastCleanup,
		"last_deep_cleanup": cm.lastDeepCleanup,
	}

	// Get database stats
	var sessionCount int64
	cm.db.Model(&models.PlaybackSession{}).Count(&sessionCount)
	stats["total_sessions"] = sessionCount

	var activeSessionCount int64
	cm.db.Model(&models.PlaybackSession{}).
		Where("state IN ?", []string{"playing", "paused"}).
		Count(&activeSessionCount)
	stats["active_sessions"] = activeSessionCount

	var pendingCleanupCount int64
	cm.db.Model(&models.TranscodeCleanupTask{}).
		Where("status = ?", "pending").
		Count(&pendingCleanupCount)
	stats["pending_cleanups"] = pendingCleanupCount

	var totalTranscodeSize int64
	cm.db.Model(&models.TranscodeCleanupTask{}).
		Where("status = ?", "pending").
		Select("COALESCE(SUM(file_size), 0)").
		Scan(&totalTranscodeSize)
	stats["transcode_cache_size"] = totalTranscodeSize

	return stats
}
