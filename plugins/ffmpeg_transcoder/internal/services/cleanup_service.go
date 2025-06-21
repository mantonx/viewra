package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mantonx/viewra/plugins/ffmpeg_transcoder/internal/config"
	"github.com/mantonx/viewra/plugins/ffmpeg_transcoder/internal/types"
	plugins "github.com/mantonx/viewra/sdk"
)

// cleanupService handles file and session cleanup
type cleanupService struct {
	logger plugins.Logger
	config *config.Config
}

// NewCleanupService creates a new cleanup service
func NewCleanupService(logger plugins.Logger, cfg *config.Config) CleanupService {
	return &cleanupService{
		logger: logger,
		config: cfg,
	}
}

// CleanupExpiredSessions removes expired transcoding files
func (s *cleanupService) CleanupExpiredSessions() (*types.CleanupInfo, error) {
	s.logger.Info("starting cleanup of expired sessions")

	info := &types.CleanupInfo{
		LastCleanup: time.Now(),
	}

	// Get transcoding directory
	transcodingDir := s.config.Core.OutputDirectory

	// List all directories in the transcoding directory
	entries, err := os.ReadDir(transcodingDir)
	if err != nil {
		if os.IsNotExist(err) {
			// Directory doesn't exist, nothing to clean
			return info, nil
		}
		return nil, fmt.Errorf("failed to read transcoding directory: %w", err)
	}

	// Process each session directory
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Skip if not a session directory
		if !s.isSessionDirectory(entry.Name()) {
			continue
		}

		dirPath := filepath.Join(transcodingDir, entry.Name())

		// Get directory stats
		dirInfo, err := os.Stat(dirPath)
		if err != nil {
			s.logger.Warn("failed to stat directory",
				"dir", dirPath,
				"error", err,
			)
			continue
		}

		info.TotalDirectories++
		dirSize := s.getDirectorySize(dirPath)
		info.TotalSize += dirSize

		// Check if directory should be removed
		if s.shouldRemove(dirInfo) {
			s.logger.Debug("removing expired session directory",
				"dir", entry.Name(),
				"age", time.Since(dirInfo.ModTime()),
				"size_mb", dirSize/(1024*1024),
			)

			if err := os.RemoveAll(dirPath); err != nil {
				s.logger.Error("failed to remove directory",
					"dir", dirPath,
					"error", err,
				)
			} else {
				info.DirectoriesRemoved++
				info.SizeFreed += dirSize
			}
		}
	}

	// Set next cleanup time
	info.NextCleanup = time.Now().Add(s.config.Cleanup.GetCleanupInterval())

	s.logger.Info("cleanup completed",
		"total_dirs", info.TotalDirectories,
		"removed", info.DirectoriesRemoved,
		"size_freed_mb", info.SizeFreed/(1024*1024),
		"total_size_mb", info.TotalSize/(1024*1024),
	)

	return info, nil
}

// GetCleanupStats returns cleanup statistics
func (s *cleanupService) GetCleanupStats() (*types.CleanupInfo, error) {
	info := &types.CleanupInfo{
		LastCleanup: time.Now(),
	}

	transcodingDir := s.config.Core.OutputDirectory

	entries, err := os.ReadDir(transcodingDir)
	if err != nil {
		if os.IsNotExist(err) {
			return info, nil
		}
		return nil, fmt.Errorf("failed to read transcoding directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() || !s.isSessionDirectory(entry.Name()) {
			continue
		}

		dirPath := filepath.Join(transcodingDir, entry.Name())
		info.TotalDirectories++
		info.TotalSize += s.getDirectorySize(dirPath)
	}

	return info, nil
}

// CleanupSession removes a specific session's files
func (s *cleanupService) CleanupSession(sessionID string) error {
	transcodingDir := s.config.Core.OutputDirectory

	// Find and remove all directories that contain this session ID
	entries, err := os.ReadDir(transcodingDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read transcoding directory: %w", err)
	}

	removed := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if directory belongs to this session
		if strings.Contains(entry.Name(), sessionID) {
			dirPath := filepath.Join(transcodingDir, entry.Name())
			if err := os.RemoveAll(dirPath); err != nil {
				s.logger.Error("failed to remove session directory",
					"dir", dirPath,
					"error", err,
				)
			} else {
				removed++
				s.logger.Debug("removed session directory",
					"dir", entry.Name(),
					"session_id", sessionID,
				)
			}
		}
	}

	if removed == 0 {
		s.logger.Debug("no directories found for session", "session_id", sessionID)
	}

	return nil
}

// isSessionDirectory checks if a directory name looks like a session directory
func (s *cleanupService) isSessionDirectory(name string) bool {
	// Session directories typically have patterns like:
	// - dash_ffmpeg_<sessionid>
	// - hls_ffmpeg_<sessionid>
	// - ffmpeg_<sessionid>
	return strings.Contains(name, "ffmpeg_") ||
		strings.HasPrefix(name, "dash_") ||
		strings.HasPrefix(name, "hls_") ||
		strings.HasPrefix(name, "mp4_")
}

// shouldRemove determines if a directory should be removed based on age
func (s *cleanupService) shouldRemove(info os.FileInfo) bool {
	age := time.Since(info.ModTime())

	// Use retention hours from config
	retentionDuration := time.Duration(s.config.Cleanup.RetentionHours) * time.Hour

	// Remove if older than retention period
	return age > retentionDuration
}

// getDirectorySize calculates the total size of a directory
func (s *cleanupService) getDirectorySize(path string) int64 {
	var size int64

	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	if err != nil {
		s.logger.Warn("error calculating directory size",
			"path", path,
			"error", err,
		)
	}

	return size
}
