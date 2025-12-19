package session

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// tryRemoveEmptyDir attempts to remove a directory if it's empty.
// This is used to clean up parent directories after removing their contents.
// Errors are silently ignored since the directory may not be empty or may not exist.
func (m *Manager) tryRemoveEmptyDir(path string) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return // Directory doesn't exist or can't be read
	}
	if len(entries) == 0 {
		if err := os.Remove(path); err == nil {
			m.logger.Debug("Removed empty media directory", "path", path)
		}
	}
}

// cleanupOutputDir removes a session's output directory and logs any errors.
func (m *Manager) cleanupOutputDir(path string, sessionID string) {
	if err := os.RemoveAll(path); err != nil {
		if sessionID != "" {
			m.logger.Warn("Failed to clean up output directory",
				"path", path,
				"session_id", sessionID,
				"error", err)
		} else {
			m.logger.Warn("Failed to clean up output directory",
				"path", path,
				"error", err)
		}
	}
}

// stopOtherMediaSessions stops all sessions for media IDs other than the specified one.
// This runs asynchronously to avoid blocking the new video from starting.
func (m *Manager) stopOtherMediaSessions(currentMediaID int64) {
	var toStop []*TranscodeSession

	m.sessions.Range(func(key, value interface{}) bool {
		session := value.(*TranscodeSession)
		if session.MediaID != currentMediaID {
			toStop = append(toStop, session)
			// Delete from map immediately so the session isn't found by other requests
			m.sessions.Delete(key)
		}
		return true
	})

	if len(toStop) == 0 {
		return
	}

	// Stop sessions asynchronously to avoid blocking the new video request.
	// We're not cleaning up output dirs here (keeping segments for potential reuse),
	// so there's no race condition with file access.
	go func() {
		for _, session := range toStop {
			m.logger.Info("Stopping session for different media (keeping segments)",
				"session_id", session.ID,
				"media_id", session.MediaID,
				"current_media_id", currentMediaID)

			session.Stop()

			if m.logStore != nil {
				m.logStore.CloseLogWriter(session.ID)
			}
		}
	}()
}

// StopOtherQualitySessions stops actively transcoding sessions for the same media but different qualities.
func (m *Manager) StopOtherQualitySessions(currentMediaID int64, currentQuality string) {
	var toStop []string

	m.sessions.Range(func(key, value interface{}) bool {
		session := value.(*TranscodeSession)
		if session.MediaID == currentMediaID && session.Quality != currentQuality {
			if session.FFmpegCmd != nil && session.FFmpegCmd.ProcessState == nil {
				toStop = append(toStop, key.(string))
			}
		}
		return true
	})

	for _, key := range toStop {
		if existing, ok := m.sessions.Load(key); ok {
			session := existing.(*TranscodeSession)

			m.logger.Info("Stopping active quality session (keeping segments for ABR)",
				"stopped_session", session.ID,
				"stopped_quality", session.Quality,
				"current_quality", currentQuality,
				"media_id", currentMediaID)

			session.Stop()
			m.sessions.Delete(key)

			if m.logStore != nil {
				m.logStore.CloseLogWriter(session.ID)
			}
		}
	}
}

// CleanupIdleSessions stops sessions that haven't been accessed recently.
func (m *Manager) CleanupIdleSessions(idleTimeout time.Duration) int {
	cleanedCount := 0

	m.sessions.Range(func(key, value any) bool {
		session := value.(*TranscodeSession)

		if time.Since(session.LastAccessed) > idleTimeout {
			m.logger.Info("Stopping idle session",
				"session_id", session.ID,
				"idle_duration", time.Since(session.LastAccessed))

			session.Stop()
			m.sessions.Delete(key)
			cleanedCount++

			if m.logStore != nil {
				m.logStore.CloseLogWriter(session.ID)
			}

			m.cleanupOutputDir(session.OutputDir, session.ID)

			// Try to remove the parent media directory if it's now empty
			mediaDir := filepath.Dir(session.OutputDir)
			m.tryRemoveEmptyDir(mediaDir)
		}

		return true
	})

	if cleanedCount > 0 {
		m.logger.Info("Cleaned up idle sessions", "count", cleanedCount)
	}

	return cleanedCount
}

// StartIdleCleanup starts a background goroutine that periodically cleans up idle sessions.
func (m *Manager) StartIdleCleanup(interval time.Duration, idleTimeout time.Duration) func() {
	ticker := time.NewTicker(interval)
	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-done:
				ticker.Stop()
				return
			case <-ticker.C:
				m.CleanupIdleSessions(idleTimeout)
			}
		}
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			close(done)
		})
	}
}

// CleanupOrphanedOutputDirs removes transcode output directories that don't have
// an associated session. This should be called on startup to clean up after crashes
// or restarts that left stale data.
func (m *Manager) CleanupOrphanedOutputDirs(outputDir string) (int, int64, error) {
	if outputDir == "" {
		return 0, 0, nil
	}

	// Get all tracked media IDs from active sessions
	activeMediaIDs := make(map[int64]map[string]bool) // mediaID -> set of quality dirs
	m.sessions.Range(func(key, value interface{}) bool {
		session := value.(*TranscodeSession)
		if activeMediaIDs[session.MediaID] == nil {
			activeMediaIDs[session.MediaID] = make(map[string]bool)
		}
		// Extract quality from output dir (e.g., ".../366808/4k-40m" -> "4k-40m")
		activeMediaIDs[session.MediaID][filepath.Base(session.OutputDir)] = true
		return true
	})

	var cleanedCount int
	var freedBytes int64

	// Scan output directory for media directories
	mediaDirs, err := os.ReadDir(outputDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil // No output dir yet, nothing to clean
		}
		return 0, 0, fmt.Errorf("failed to read output directory: %w", err)
	}

	for _, mediaDir := range mediaDirs {
		if !mediaDir.IsDir() {
			continue
		}

		// Skip non-numeric directories (not media ID directories) and logs directory
		if mediaDir.Name() == "logs" {
			continue
		}
		var mediaID int64
		if _, err := fmt.Sscanf(mediaDir.Name(), "%d", &mediaID); err != nil {
			continue
		}

		mediaPath := filepath.Join(outputDir, mediaDir.Name())

		// If this media has no active sessions, clean up the entire directory
		if _, hasActiveSessions := activeMediaIDs[mediaID]; !hasActiveSessions {
			size, err := getDirSize(mediaPath)
			if err == nil {
				freedBytes += size
			}

			if err := os.RemoveAll(mediaPath); err != nil {
				m.logger.Warn("Failed to clean orphaned media directory",
					"path", mediaPath,
					"error", err)
			} else {
				cleanedCount++
				m.logger.Info("Cleaned orphaned media directory",
					"path", mediaPath,
					"freed_bytes", size)
			}
			continue
		}

		// Media has some active sessions - check individual quality directories
		qualityDirs, err := os.ReadDir(mediaPath)
		if err != nil {
			continue
		}

		for _, qualityDir := range qualityDirs {
			if !qualityDir.IsDir() {
				continue
			}

			qualityPath := filepath.Join(mediaPath, qualityDir.Name())

			// If this quality dir is not in active sessions, clean it up
			if !activeMediaIDs[mediaID][qualityDir.Name()] {
				size, err := getDirSize(qualityPath)
				if err == nil {
					freedBytes += size
				}

				if err := os.RemoveAll(qualityPath); err != nil {
					m.logger.Warn("Failed to clean orphaned quality directory",
						"path", qualityPath,
						"error", err)
				} else {
					cleanedCount++
					m.logger.Info("Cleaned orphaned quality directory",
						"path", qualityPath,
						"freed_bytes", size)
				}
			}
		}

		// Try to remove parent if now empty
		m.tryRemoveEmptyDir(mediaPath)
	}

	if cleanedCount > 0 {
		m.logger.Info("Startup cleanup complete",
			"directories_cleaned", cleanedCount,
			"bytes_freed", freedBytes)
	}

	return cleanedCount, freedBytes, nil
}

// getDirSize calculates the total size of a directory.
func getDirSize(path string) (int64, error) {
	var size int64
	err := filepath.WalkDir(path, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err == nil {
				size += info.Size()
			}
		}
		return nil
	})
	return size, err
}

// CleanupSessionOutput removes the output directory for a session.
// If the parent media directory becomes empty after cleanup, it is also removed.
func (m *Manager) CleanupSessionOutput(mediaID int64, quality string, outputDir string) error {
	mediaDir := filepath.Join(outputDir, fmt.Sprintf("%d", mediaID))
	sessionOutputDir := filepath.Join(mediaDir, quality)

	if err := os.RemoveAll(sessionOutputDir); err != nil {
		return fmt.Errorf("failed to clean up session output: %w", err)
	}

	m.logger.Info("Cleaned up session output",
		"media_id", mediaID,
		"quality", quality,
		"path", sessionOutputDir)

	// Try to remove the parent media directory if it's now empty
	m.tryRemoveEmptyDir(mediaDir)

	return nil
}

// CleanupOldTranscodes removes old transcode cache files based on age and LRU.
func (m *Manager) CleanupOldTranscodes(outputDir string, maxAge time.Duration) (int, int64, error) {
	type dirInfo struct {
		path     string
		modTime  time.Time
		size     int64
		mediaID  int64
		isActive bool
	}

	var dirs []dirInfo
	activeMediaIDs := make(map[int64]bool)

	m.sessions.Range(func(key, value interface{}) bool {
		session := value.(*TranscodeSession)
		activeMediaIDs[session.MediaID] = true
		return true
	})

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read transcode directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		var mediaID int64
		if _, err := fmt.Sscanf(entry.Name(), "%d", &mediaID); err != nil {
			continue
		}

		mediaPath := filepath.Join(outputDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		var totalSize int64
		filepath.Walk(mediaPath, func(path string, fi os.FileInfo, err error) error {
			if err == nil && !fi.IsDir() {
				totalSize += fi.Size()
			}
			return nil
		})

		dirs = append(dirs, dirInfo{
			path:     mediaPath,
			modTime:  info.ModTime(),
			size:     totalSize,
			mediaID:  mediaID,
			isActive: activeMediaIDs[mediaID],
		})
	}

	cleanedCount := 0
	var bytesFreed int64
	cutoff := time.Now().Add(-maxAge)

	for _, dir := range dirs {
		if dir.isActive {
			continue
		}

		if dir.modTime.After(cutoff) {
			continue
		}

		m.logger.Info("Removing old transcode cache",
			"path", dir.path,
			"media_id", dir.mediaID,
			"age", time.Since(dir.modTime),
			"size_mb", dir.size/(1024*1024))

		m.cleanupOutputDir(dir.path, "")

		cleanedCount++
		bytesFreed += dir.size
	}

	if cleanedCount > 0 {
		m.logger.Info("Cleaned up old transcode caches",
			"count", cleanedCount,
			"freed_mb", bytesFreed/(1024*1024))
	}

	return cleanedCount, bytesFreed, nil
}

// StartPeriodicCleanup starts background cleanup of both idle sessions and old transcode caches.
func (m *Manager) StartPeriodicCleanup(
	sessionCheckInterval time.Duration,
	sessionIdleTimeout time.Duration,
	transcodeCheckInterval time.Duration,
	transcodeMaxAge time.Duration,
	outputDir string,
) func() {
	done := make(chan struct{})

	sessionTicker := time.NewTicker(sessionCheckInterval)
	transcodeTicker := time.NewTicker(transcodeCheckInterval)

	go func() {
		for {
			select {
			case <-done:
				sessionTicker.Stop()
				transcodeTicker.Stop()
				return
			case <-sessionTicker.C:
				m.CleanupIdleSessions(sessionIdleTimeout)
			case <-transcodeTicker.C:
				m.CleanupOldTranscodes(outputDir, transcodeMaxAge)

				if m.logStore != nil {
					deletedCount, deletedBytes, err := m.logStore.CleanupOldLogs()
					if err != nil {
						m.logger.Warn("Failed to cleanup old FFmpeg logs", "error", err)
					} else if deletedCount > 0 {
						m.logger.Info("Cleaned up old FFmpeg logs",
							"count", deletedCount,
							"freed_bytes", deletedBytes)
					}
				}
			}
		}
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			close(done)
		})
	}
}
