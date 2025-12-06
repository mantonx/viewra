package transcoding

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// cleanupOutputDir removes a session's output directory and logs any errors.
// This centralizes cleanup logic to avoid duplication across the codebase.
// sessionID is optional and used for logging context.
func (m *SessionManager) cleanupOutputDir(path string, sessionID string) {
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
// This prevents resource hogging when users switch between different videos.
// Note: We only stop the FFmpeg process and remove from session tracking.
// Segments are NOT deleted immediately - they're cleaned up by periodic LRU cleanup.
// This allows:
//   - Users to switch back to a video without re-transcoding
//   - Multiple clients/tabs to share cached segments
//   - Debugging and analysis of transcoded segments
func (m *SessionManager) stopOtherMediaSessions(currentMediaID int64) {
	var toStop []string

	// Collect sessions to stop (can't delete while iterating sync.Map)
	m.sessions.Range(func(key, value interface{}) bool {
		session := value.(*TranscodeSession)
		if session.MediaID != currentMediaID {
			toStop = append(toStop, key.(string))
		}
		return true
	})

	// Stop sessions but keep segments for potential reuse
	for _, key := range toStop {
		if existing, ok := m.sessions.Load(key); ok {
			session := existing.(*TranscodeSession)

			m.logger.Info("Stopping session for different media (keeping segments)",
				"session_id", session.ID,
				"media_id", session.MediaID,
				"current_media_id", currentMediaID)

			session.Stop()
			m.sessions.Delete(key)

			// Close log writer if present
			if m.logStore != nil {
				m.logStore.CloseLogWriter(session.ID)
			}

			// NOTE: We intentionally do NOT clean up the output directory here.
			// Segments are left for potential reuse and cleaned up by:
			// - CleanupOldTranscodes (periodic LRU cleanup based on age)
			// - CleanupIdleSessions won't find these since they're removed from sessions map
		}
	}
}

// stopOtherQualitySessions stops actively transcoding sessions for the same media but different qualities.
// This prevents resource contention when HLS.js requests multiple qualities after seeking.
// Segments are kept for potential reuse (ABR quality switching).
func (m *SessionManager) stopOtherQualitySessions(currentMediaID int64, currentQuality string) {
	var toStop []string

	// Collect ACTIVE sessions to stop (can't delete while iterating sync.Map)
	m.sessions.Range(func(key, value interface{}) bool {
		session := value.(*TranscodeSession)
		// Only stop sessions that are still actively transcoding
		// Keep completed sessions so their segments remain available
		if session.MediaID == currentMediaID && session.Quality != currentQuality {
			// Check if FFmpeg process is still running
			if session.FFmpegCmd != nil && session.FFmpegCmd.ProcessState == nil {
				toStop = append(toStop, key.(string))
			}
		}
		return true
	})

	// Stop FFmpeg processes but keep segments for ABR
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

			// Close log writer if present
			if m.logStore != nil {
				m.logStore.CloseLogWriter(session.ID)
			}

			// NOTE: Segments are kept for ABR quality switching
		}
	}
}

// CleanupIdleSessions stops sessions that haven't been accessed recently.
// Should be called periodically (e.g., every 5 minutes) to free resources.
func (m *SessionManager) CleanupIdleSessions(idleTimeout time.Duration) int {
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

			// Close log writer if present
			if m.logStore != nil {
				m.logStore.CloseLogWriter(session.ID)
			}

			// Clean up output directory
			m.cleanupOutputDir(session.OutputDir, session.ID)
		}

		return true
	})

	if cleanedCount > 0 {
		m.logger.Info("Cleaned up idle sessions", "count", cleanedCount)
	}

	return cleanedCount
}

// StartIdleCleanup starts a background goroutine that periodically cleans up idle sessions.
// Returns a cancel function that should be called to stop the cleanup goroutine.
func (m *SessionManager) StartIdleCleanup(interval time.Duration, idleTimeout time.Duration) func() {
	ticker := time.NewTicker(interval)
	done := make(chan bool)

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

	// Return cancel function
	return func() {
		done <- true
	}
}

// CleanupSessionOutput removes the output directory for a session.
// Useful for manual cleanup or when sessions fail.
func (m *SessionManager) CleanupSessionOutput(mediaID int64, quality string, outputDir string) error {
	sessionOutputDir := filepath.Join(outputDir, fmt.Sprintf("%d", mediaID), quality)

	if err := os.RemoveAll(sessionOutputDir); err != nil {
		return fmt.Errorf("failed to clean up session output: %w", err)
	}

	m.logger.Info("Cleaned up session output",
		"media_id", mediaID,
		"quality", quality,
		"path", sessionOutputDir)

	return nil
}

// CleanupOldTranscodes removes old transcode cache files based on age and LRU.
// Returns the number of directories cleaned and bytes freed.
func (m *SessionManager) CleanupOldTranscodes(outputDir string, maxAge time.Duration) (int, int64, error) {
	type dirInfo struct {
		path     string
		modTime  time.Time
		size     int64
		mediaID  int64
		isActive bool
	}

	var dirs []dirInfo
	activeMediaIDs := make(map[int64]bool)

	// Collect active media IDs from current sessions
	m.sessions.Range(func(key, value interface{}) bool {
		session := value.(*TranscodeSession)
		activeMediaIDs[session.MediaID] = true
		return true
	})

	// Scan transcode directory for media subdirectories
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read transcode directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Parse media ID from directory name
		var mediaID int64
		if _, err := fmt.Sscanf(entry.Name(), "%d", &mediaID); err != nil {
			continue // Skip non-numeric directories
		}

		mediaPath := filepath.Join(outputDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Calculate directory size
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

	// Clean up old directories (not active and older than maxAge)
	cleanedCount := 0
	var bytesFreed int64
	cutoff := time.Now().Add(-maxAge)

	for _, dir := range dirs {
		// Skip active sessions
		if dir.isActive {
			continue
		}

		// Skip recent transcodes
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
// Returns a cancel function that should be called to stop the cleanup goroutine.
func (m *SessionManager) StartPeriodicCleanup(
	sessionCheckInterval time.Duration,
	sessionIdleTimeout time.Duration,
	transcodeCheckInterval time.Duration,
	transcodeMaxAge time.Duration,
	outputDir string,
) func() {
	done := make(chan bool)

	// Session cleanup ticker
	sessionTicker := time.NewTicker(sessionCheckInterval)

	// Transcode cache cleanup ticker (less frequent)
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

				// Also cleanup old FFmpeg logs
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

	// Return cancel function
	return func() {
		done <- true
	}
}
