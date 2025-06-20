package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/pkg/plugins"
)

// CleanupService provides centralized cleanup for all transcoding providers
type CleanupService struct {
	baseDir     string
	store       *SessionStore
	fileManager *FileManager
	logger      hclog.Logger
	policies    map[string]RetentionPolicy
	config      CleanupConfig
}

// CleanupConfig contains cleanup configuration
type CleanupConfig struct {
	BaseDirectory      string
	RetentionHours     int
	ExtendedHours      int
	MaxTotalSizeGB     int64
	CleanupInterval    time.Duration
	LargeFileThreshold int64
	ProviderOverrides  map[string]ProviderCleanupConfig
}

// ProviderCleanupConfig contains provider-specific cleanup settings
type ProviderCleanupConfig struct {
	RetentionHours int
	MaxSessions    int
	MaxSizeGB      int64
}

// NewCleanupService creates a new cleanup service
func NewCleanupService(config CleanupConfig, store *SessionStore, fileManager *FileManager, logger hclog.Logger) *CleanupService {
	// Get base directory from environment or config
	baseDir := config.BaseDirectory
	if envDir := os.Getenv("VIEWRA_TRANSCODING_DIR"); envDir != "" {
		baseDir = envDir
	}

	return &CleanupService{
		baseDir:     baseDir,
		store:       store,
		fileManager: fileManager,
		logger:      logger.Named("cleanup-service"),
		config:      config,
		policies:    make(map[string]RetentionPolicy),
	}
}

// Run starts the cleanup service
func (cs *CleanupService) Run(ctx context.Context) {
	cs.logger.Info("starting cleanup service",
		"interval", cs.config.CleanupInterval,
		"base_dir", cs.baseDir)

	ticker := time.NewTicker(cs.config.CleanupInterval)
	defer ticker.Stop()

	// Run initial cleanup
	cs.cleanupAllProviders()

	for {
		select {
		case <-ticker.C:
			cs.cleanupAllProviders()
		case <-ctx.Done():
			cs.logger.Info("cleanup service stopped")
			return
		}
	}
}

// cleanupAllProviders runs cleanup for all providers
func (cs *CleanupService) cleanupAllProviders() {
	cs.logger.Debug("running cleanup cycle")

	// Get cleanup statistics
	stats, err := cs.GetCleanupStats()
	if err != nil {
		cs.logger.Error("failed to get cleanup stats", "error", err)
		return
	}

	cs.logger.Info("cleanup stats",
		"total_sessions", stats.TotalSessions,
		"total_size", plugins.FormatBytes(stats.TotalSize),
		"oldest_session", stats.OldestSession)

	// Check if we need emergency cleanup due to size
	if stats.TotalSize > cs.config.MaxTotalSizeGB*1024*1024*1024 {
		cs.logger.Warn("total size exceeds limit, running emergency cleanup",
			"current_size", plugins.FormatBytes(stats.TotalSize),
			"limit", fmt.Sprintf("%dGB", cs.config.MaxTotalSizeGB))
		cs.runEmergencyCleanup(stats.TotalSize)
	}

	// Run standard cleanup based on retention policy
	policy := RetentionPolicy{
		RetentionHours:     cs.config.RetentionHours,
		ExtendedHours:      cs.config.ExtendedHours,
		MaxTotalSizeGB:     cs.config.MaxTotalSizeGB,
		LargeFileThreshold: cs.config.LargeFileThreshold,
	}

	// Clean up expired sessions from database
	dbCount, err := cs.store.CleanupExpiredSessions(policy)
	if err != nil {
		cs.logger.Error("failed to cleanup database sessions", "error", err)
	} else if dbCount > 0 {
		cs.logger.Info("cleaned up database sessions", "count", dbCount)
	}

	// Clean up orphaned directories
	orphanCount, err := cs.cleanupOrphanedDirectories()
	if err != nil {
		cs.logger.Error("failed to cleanup orphaned directories", "error", err)
	} else if orphanCount > 0 {
		cs.logger.Info("cleaned up orphaned directories", "count", orphanCount)
	}
}

// runEmergencyCleanup removes files to get under size limit
func (cs *CleanupService) runEmergencyCleanup(currentSize int64) {
	targetSize := cs.config.MaxTotalSizeGB * 1024 * 1024 * 1024 * 90 / 100 // Target 90% of limit

	// Get sessions sorted by last accessed (oldest first)
	sessions, err := cs.fileManager.GetOldestSessions(100)
	if err != nil {
		cs.logger.Error("failed to get oldest sessions", "error", err)
		return
	}

	freedSize := int64(0)
	removedCount := 0

	for _, sessionDir := range sessions {
		if currentSize-freedSize <= targetSize {
			break
		}

		// Skip very recent sessions (less than 1 hour old)
		if time.Since(sessionDir.LastModified) < time.Hour {
			continue
		}

		// Get size of this session
		size, err := cs.fileManager.GetDirectorySize(sessionDir.Path)
		if err != nil {
			cs.logger.Warn("failed to get directory size", "path", sessionDir.Path, "error", err)
			continue
		}

		// Remove the directory
		if err := os.RemoveAll(sessionDir.Path); err != nil {
			cs.logger.Error("failed to remove directory", "path", sessionDir.Path, "error", err)
			continue
		}

		freedSize += size
		removedCount++
		cs.logger.Info("removed session for emergency cleanup",
			"path", sessionDir.Path,
			"size", plugins.FormatBytes(size),
			"age", time.Since(sessionDir.LastModified))
	}

	cs.logger.Info("emergency cleanup completed",
		"removed_count", removedCount,
		"freed_size", plugins.FormatBytes(freedSize))
}

// cleanupOrphanedDirectories removes directories without database records
func (cs *CleanupService) cleanupOrphanedDirectories() (int, error) {
	entries, err := os.ReadDir(cs.baseDir)
	if err != nil {
		return 0, fmt.Errorf("failed to read base directory: %w", err)
	}

	orphanCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Extract session ID from directory name
		// Format: container_provider_sessionid
		dirName := entry.Name()
		sessionID := cs.extractSessionID(dirName)
		if sessionID == "" {
			continue
		}

		// Check if session exists in database
		_, err := cs.store.GetSession(sessionID)
		if err != nil {
			// Session not found, this is an orphan
			dirPath := filepath.Join(cs.baseDir, dirName)

			// Check age before removing
			info, err := entry.Info()
			if err != nil {
				continue
			}

			// Only remove if older than 1 hour
			if time.Since(info.ModTime()) > time.Hour {
				if err := os.RemoveAll(dirPath); err != nil {
					cs.logger.Error("failed to remove orphaned directory", "path", dirPath, "error", err)
				} else {
					orphanCount++
					cs.logger.Debug("removed orphaned directory", "path", dirPath)
				}
			}
		}
	}

	return orphanCount, nil
}

// extractSessionID extracts session ID from directory name
func (cs *CleanupService) extractSessionID(dirName string) string {
	// Directory format: container_provider_sessionid
	// Example: dash_ffmpeg_1234567890
	parts := filepath.Base(dirName)

	// For now, use the full directory name as session ID
	// In the future, we might parse it differently
	return parts
}

// GetCleanupStats returns cleanup statistics
func (cs *CleanupService) GetCleanupStats() (*CleanupStats, error) {
	stats := &CleanupStats{
		Timestamp: time.Now(),
	}

	// Get total size and count
	entries, err := os.ReadDir(cs.baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var oldestTime time.Time
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		stats.TotalSessions++

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Track oldest session
		if oldestTime.IsZero() || info.ModTime().Before(oldestTime) {
			oldestTime = info.ModTime()
		}

		// Get directory size
		dirPath := filepath.Join(cs.baseDir, entry.Name())
		size, err := cs.fileManager.GetDirectorySize(dirPath)
		if err != nil {
			cs.logger.Warn("failed to get directory size", "path", dirPath, "error", err)
			continue
		}
		stats.TotalSize += size

		// Count by status (would need to query DB for accurate status)
		if time.Since(info.ModTime()) < 5*time.Minute {
			stats.ActiveSessions++
		}
	}

	if !oldestTime.IsZero() {
		stats.OldestSession = time.Since(oldestTime)
	}

	// Set policy info
	stats.RetentionHours = cs.config.RetentionHours
	stats.MaxSizeGB = cs.config.MaxTotalSizeGB

	return stats, nil
}

// CleanupSession removes a specific session's files
func (cs *CleanupService) CleanupSession(sessionID string) error {
	// Get session from store to find directory
	session, err := cs.store.GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// Remove directory if it exists
	if session.DirectoryPath != "" && session.DirectoryPath != "/" {
		if err := os.RemoveAll(session.DirectoryPath); err != nil {
			return fmt.Errorf("failed to remove directory: %w", err)
		}
		cs.logger.Info("cleaned up session directory", "session_id", sessionID, "path", session.DirectoryPath)
	}

	return nil
}

// CleanupStats contains cleanup statistics
type CleanupStats struct {
	TotalSessions  int
	ActiveSessions int
	TotalSize      int64
	OldestSession  time.Duration
	RetentionHours int
	MaxSizeGB      int64
	Timestamp      time.Time
}
