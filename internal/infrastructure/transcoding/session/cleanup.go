package session

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

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
func (m *Manager) stopOtherMediaSessions(currentMediaID int64) {
	var toStop []string

	m.sessions.Range(func(key, value interface{}) bool {
		session := value.(*TranscodeSession)
		if session.MediaID != currentMediaID {
			toStop = append(toStop, key.(string))
		}
		return true
	})

	for _, key := range toStop {
		if existing, ok := m.sessions.Load(key); ok {
			session := existing.(*TranscodeSession)

			m.logger.Info("Stopping session for different media (keeping segments)",
				"session_id", session.ID,
				"media_id", session.MediaID,
				"current_media_id", currentMediaID)

			session.Stop()
			m.sessions.Delete(key)

			if m.logStore != nil {
				m.logStore.CloseLogWriter(session.ID)
			}
		}
	}
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

// CleanupSessionOutput removes the output directory for a session.
func (m *Manager) CleanupSessionOutput(mediaID int64, quality string, outputDir string) error {
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
