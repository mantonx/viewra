package transcoding

import (
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SessionManager manages active transcode sessions.
// Handles session lifecycle, cleanup, and restart logic for seeking.
type SessionManager struct {
	sessions        sync.Map // map[string]*TranscodeSession (key: "mediaID_quality")
	logger          *slog.Logger
	config          *TranscodeConfig
	fallbackManager *HardwareFallbackManager
}

// NewSessionManager creates a new session manager.
func NewSessionManager(config *TranscodeConfig, logger *slog.Logger) *SessionManager {
	if config == nil {
		config = DefaultTranscodeConfig()
	}

	mgr := &SessionManager{
		logger:          logger,
		config:          config,
		fallbackManager: NewHardwareFallbackManager(config, logger),
	}

	// Verify hardware acceleration is available
	if config.HardwareAccel != AccelNone {
		if err := mgr.fallbackManager.VerifyHardwareAvailability(config.HardwareAccel); err != nil {
			logger.Warn("Hardware acceleration verification failed, falling back to software",
				"hardware", config.HardwareAccel,
				"error", err,
			)
			config.HardwareAccel = AccelNone
		}
	}

	return mgr
}

// GetOrCreateSession returns an existing session or creates a new one.
// If a session exists but the seek position is significantly different (>30s),
// the old session is stopped and a new one is created from the new position.
func (m *SessionManager) GetOrCreateSession(
	mediaID int64,
	quality string,
	startPosition float64,
	inputPath string,
	profile *QualityProfile,
	strategy StreamStrategy,
	outputDir string,
	videoInfo *VideoInfo,
) (*TranscodeSession, error) {
	key := sessionKey(mediaID, quality)

	// Check for existing session
	if existing, ok := m.sessions.Load(key); ok {
		session := existing.(*TranscodeSession)

		// If seek position is far from current position, restart session
		seekThreshold := 30.0 // seconds
		if math.Abs(startPosition-session.StartPosition) > seekThreshold {
			m.logger.Info("Restarting session for seek",
				"media_id", mediaID,
				"quality", quality,
				"old_position", session.StartPosition,
				"new_position", startPosition)

			// Stop old session
			session.Stop()
			m.sessions.Delete(key)

			// Fall through to create new session
		} else {
			// Reuse existing session
			session.UpdateLastAccessed()
			m.logger.Debug("Reusing existing session",
				"session_id", session.ID,
				"media_id", mediaID,
				"quality", quality)
			return session, nil
		}
	}

	// Create new session
	session := NewTranscodeSession(mediaID, quality, startPosition, outputDir, m.logger)

	// Start FFmpeg process with hardware acceleration and HDR tone mapping
	hwAccel := m.config.HardwareAccel
	hwDevice := m.config.HardwareDevice
	toneMappingEnabled := m.config.ToneMappingEnabled
	if err := session.Start(inputPath, profile, strategy, hwAccel, hwDevice, videoInfo, toneMappingEnabled); err != nil {
		// Check if this is a hardware error and fallback if needed
		if m.fallbackManager.RecordFailure(hwAccel, err) {
			m.logger.Info("Retrying with fallback acceleration",
				"from", hwAccel,
				"to", m.config.HardwareAccel,
			)
			// Retry with new hardware acceleration setting
			if err := session.Start(inputPath, profile, strategy, m.config.HardwareAccel, hwDevice, videoInfo, toneMappingEnabled); err != nil {
				return nil, fmt.Errorf("failed to start transcode session after fallback: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to start transcode session: %w", err)
		}
	}

	// Store session
	m.sessions.Store(key, session)

	m.logger.Info("Created new transcode session",
		"session_id", session.ID,
		"media_id", mediaID,
		"quality", quality,
		"start_position", startPosition)

	return session, nil
}

// GetSession retrieves an existing session by media ID and quality.
func (m *SessionManager) GetSession(mediaID int64, quality string) (*TranscodeSession, error) {
	key := sessionKey(mediaID, quality)

	if existing, ok := m.sessions.Load(key); ok {
		session := existing.(*TranscodeSession)
		session.UpdateLastAccessed()
		return session, nil
	}

	return nil, fmt.Errorf("no active transcode session for media %d quality %s", mediaID, quality)
}

// StopSession stops a specific session and removes it from the manager.
func (m *SessionManager) StopSession(mediaID int64, quality string) error {
	key := sessionKey(mediaID, quality)

	if existing, ok := m.sessions.Load(key); ok {
		session := existing.(*TranscodeSession)
		session.Stop()
		m.sessions.Delete(key)

		// Clean up output directory
		if err := os.RemoveAll(session.OutputDir); err != nil {
			m.logger.Warn("Failed to clean up session output directory",
				"session_id", session.ID,
				"error", err)
		}

		m.logger.Info("Stopped transcode session", "session_id", session.ID)
		return nil
	}

	return fmt.Errorf("session not found")
}

// StopAllSessions stops all active sessions.
func (m *SessionManager) StopAllSessions() {
	m.logger.Info("Stopping all transcode sessions")

	m.sessions.Range(func(key, value interface{}) bool {
		session := value.(*TranscodeSession)
		session.Stop()

		// Clean up output directory
		if err := os.RemoveAll(session.OutputDir); err != nil {
			m.logger.Warn("Failed to clean up session output directory",
				"session_id", session.ID,
				"error", err)
		}

		return true
	})

	// Clear all sessions
	m.sessions = sync.Map{}
}

// CleanupIdleSessions stops sessions that haven't been accessed recently.
// Should be called periodically (e.g., every 5 minutes) to free resources.
func (m *SessionManager) CleanupIdleSessions(idleTimeout time.Duration) int {
	cleanedCount := 0

	m.sessions.Range(func(key, value interface{}) bool {
		session := value.(*TranscodeSession)

		if time.Since(session.LastAccessed) > idleTimeout {
			m.logger.Info("Stopping idle session",
				"session_id", session.ID,
				"idle_duration", time.Since(session.LastAccessed))

			session.Stop()
			m.sessions.Delete(key)
			cleanedCount++

			// Clean up output directory
			if err := os.RemoveAll(session.OutputDir); err != nil {
				m.logger.Warn("Failed to clean up session output directory",
					"session_id", session.ID,
					"error", err)
			}
		}

		return true
	})

	if cleanedCount > 0 {
		m.logger.Info("Cleaned up idle sessions", "count", cleanedCount)
	}

	return cleanedCount
}

// GetStats returns statistics about active sessions.
func (m *SessionManager) GetStats() map[string]interface{} {
	activeCount := 0
	m.sessions.Range(func(key, value interface{}) bool {
		activeCount++
		return true
	})

	return map[string]interface{}{
		"active_sessions": activeCount,
	}
}

// GetSessionOutputPath returns the path where a session's segments are stored.
// Useful for serving segments without needing the full session object.
func (m *SessionManager) GetSessionOutputPath(mediaID int64, quality string) (string, error) {
	key := sessionKey(mediaID, quality)

	if existing, ok := m.sessions.Load(key); ok {
		session := existing.(*TranscodeSession)
		return session.OutputDir, nil
	}

	return "", fmt.Errorf("no active session for media %d quality %s", mediaID, quality)
}

// sessionKey generates a unique key for a media/quality combination.
func sessionKey(mediaID int64, quality string) string {
	return fmt.Sprintf("%d:%s", mediaID, quality)
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
