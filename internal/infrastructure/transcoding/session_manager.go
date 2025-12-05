package transcoding

import (
	"context"
	"fmt"
	"log/slog"
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
	configProvider  ConfigProvider // Optional: for dynamic config from settings
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

// SetConfigProvider sets a dynamic config provider for runtime settings.
// When set, GetOrCreateSession will fetch fresh config from the provider
// for settings like tone mapping that can change without restart.
func (m *SessionManager) SetConfigProvider(provider ConfigProvider) {
	m.configProvider = provider
}

// getEffectiveConfig returns the current configuration, using provider if available.
func (m *SessionManager) getEffectiveConfig(ctx context.Context) *TranscodeConfig {
	if m.configProvider != nil {
		return m.configProvider.GetConfig(ctx)
	}
	return m.config
}

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

// SubtitleBurnInOptions contains options for subtitle burn-in during transcoding.
type SubtitleBurnInOptions struct {
	Enabled     bool // Whether to burn in subtitles
	StreamIndex int  // Relative index among subtitle streams (0-based)
}

// GetOrCreateSession returns an existing session or creates a new one.
// Sessions are reused when possible to avoid re-transcoding segments that already exist.
// Multiple quality sessions can coexist to support ABR (Adaptive Bitrate) streaming.
// clientSupportedCodecs is a list of video codecs the client can decode (e.g., ["h264", "h265", "vp9"]).
// This is used to select the best codec from the profile's preferred/fallback list.
// subtitleOpts specifies optional subtitle burn-in settings for PGS/bitmap subtitles.
func (m *SessionManager) GetOrCreateSession(
	mediaID int64,
	quality string,
	startPosition float64,
	inputPath string,
	profile *AdaptiveProfile,
	strategy StreamStrategy,
	outputDir string,
	videoInfo *VideoInfo,
	clientSupportedCodecs []string,
	subtitleOpts *SubtitleBurnInOptions,
) (*TranscodeSession, error) {
	// Include subtitle info in session key so changing subtitles triggers new session
	subtitleKey := ""
	if subtitleOpts != nil && subtitleOpts.Enabled {
		subtitleKey = fmt.Sprintf("_sub%d", subtitleOpts.StreamIndex)
	}
	key := sessionKey(mediaID, quality) + subtitleKey

	// Stop all sessions for OTHER media to prevent resource hogging
	// When user switches to a different video, clean up the old one immediately
	m.stopOtherMediaSessions(mediaID)

	// NOTE: We intentionally do NOT stop other quality sessions for the same media.
	// This allows ABR to work properly - HLS.js can switch between qualities without
	// causing transcode sessions to be killed and restarted.

	// Check for existing session
	// Reuse if start position is close enough or if no position specified (ABR quality switch)
	// This balances ABR functionality with proper seek support.
	if existing, ok := m.sessions.Load(key); ok {
		session := existing.(*TranscodeSession)

		// Calculate if the requested position is reachable from the current session
		// A session starting at position S can serve content from S onwards
		// If requested position < session start, we need a new session (can't go backwards)
		positionDiff := startPosition - session.StartPosition

		// Reuse session if:
		// 1. No specific start position requested (ABR quality switch, position=0 means "use existing")
		// 2. Requested position is within the session's range:
		//    - Position must be >= session start (can't go backwards from where FFmpeg started)
		//    - Small seeks forward are fine (within 30 seconds ahead of session start)
		canReuse := startPosition == 0 || // No specific position requested (ABR switch)
			(positionDiff >= 0 && positionDiff <= 30) // Forward seek within reasonable range

		if canReuse {
			session.UpdateLastAccessed()

			m.logger.Debug("Reusing existing transcode session",
				"session_id", session.ID,
				"media_id", mediaID,
				"quality", quality,
				"session_start", session.StartPosition,
				"requested_start", startPosition,
				"position_diff", positionDiff)

			return session, nil
		}

		// Position is outside reusable range - need to restart session
		m.logger.Info("Restarting session for seek operation",
			"session_id", session.ID,
			"media_id", mediaID,
			"quality", quality,
			"session_start", session.StartPosition,
			"requested_start", startPosition,
			"position_diff", positionDiff)

		// Stop the old session
		session.Stop()
		m.sessions.Delete(key)

		// Clean up output directory
		m.cleanupOutputDir(session.OutputDir, session.ID)
	}

	// Create new session
	session := NewTranscodeSession(mediaID, quality, startPosition, outputDir, m.logger)

	// Set subtitle burn-in options if provided
	if subtitleOpts != nil && subtitleOpts.Enabled {
		session.BurnInSubtitle = true
		session.SubtitleStreamIndex = subtitleOpts.StreamIndex
		m.logger.Info("Creating session with subtitle burn-in",
			"media_id", mediaID,
			"quality", quality,
			"subtitle_stream_index", subtitleOpts.StreamIndex,
			"session_key", key)
	}

	// Get effective config (dynamic settings like tone mapping may have changed)
	effectiveConfig := m.getEffectiveConfig(context.Background())

	// Start FFmpeg process with hardware acceleration and HDR tone mapping
	hwAccel := effectiveConfig.HardwareAccel
	hwDevice := effectiveConfig.HardwareDevice
	if err := session.Start(inputPath, profile, strategy, hwAccel, hwDevice, videoInfo, effectiveConfig, clientSupportedCodecs); err != nil {
		// Check if this is a hardware error and fallback if needed
		if m.fallbackManager.RecordFailure(hwAccel, err) {
			m.logger.Info("Retrying with fallback acceleration",
				"from", hwAccel,
				"to", m.config.HardwareAccel,
			)
			// Retry with new hardware acceleration setting (use base config for fallback)
			if err := session.Start(inputPath, profile, strategy, m.config.HardwareAccel, hwDevice, videoInfo, effectiveConfig, clientSupportedCodecs); err != nil {
				return nil, fmt.Errorf("failed to start transcode session after fallback: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to start transcode session: %w", err)
		}
	}

	// Store session
	m.sessions.Store(key, session)

	m.logger.Debug("Created new transcode session",
		"session_id", session.ID,
		"media_id", mediaID,
		"quality", quality,
		"start_position", startPosition)

	return session, nil
}

// GetSession retrieves an existing session by media ID and quality.
// If subtitleIndex is >= 0, it will look for a session with that specific subtitle burn-in.
// If subtitleIndex is < 0, it will try to find any session (first without subtitle, then with).
func (m *SessionManager) GetSession(mediaID int64, quality string, subtitleIndex int) (*TranscodeSession, error) {
	// Build the session key based on whether subtitle is specified
	var key string
	if subtitleIndex >= 0 {
		key = sessionKey(mediaID, quality) + fmt.Sprintf("_sub%d", subtitleIndex)
	} else {
		key = sessionKey(mediaID, quality)
	}

	if existing, ok := m.sessions.Load(key); ok {
		session := existing.(*TranscodeSession)
		session.UpdateLastAccessed()
		return session, nil
	}

	// If no subtitle specified, also try to find a session with any subtitle (fallback)
	if subtitleIndex < 0 {
		// Scan for any session matching the mediaID and quality prefix
		var found *TranscodeSession
		prefix := sessionKey(mediaID, quality)
		m.sessions.Range(func(k, v interface{}) bool {
			keyStr := k.(string)
			if keyStr == prefix || (len(keyStr) > len(prefix) && keyStr[:len(prefix)] == prefix && keyStr[len(prefix)] == '_') {
				found = v.(*TranscodeSession)
				found.UpdateLastAccessed()
				return false // stop iteration
			}
			return true
		})
		if found != nil {
			return found, nil
		}
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
		m.cleanupOutputDir(session.OutputDir, session.ID)

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
		m.cleanupOutputDir(session.OutputDir, session.ID)

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
			m.cleanupOutputDir(session.OutputDir, session.ID)
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

// audioSessionKey generates a unique key for an audio-only session.
func audioSessionKey(mediaID int64, audioTrackIndex int) string {
	return fmt.Sprintf("%d:audio:%d", mediaID, audioTrackIndex)
}

// GetAudioSession retrieves an existing audio-only transcode session.
func (m *SessionManager) GetAudioSession(mediaID int64, audioTrackIndex int) (*TranscodeSession, error) {
	key := audioSessionKey(mediaID, audioTrackIndex)

	if existing, ok := m.sessions.Load(key); ok {
		session := existing.(*TranscodeSession)
		return session, nil
	}

	return nil, fmt.Errorf("no active audio session for media %d track %d", mediaID, audioTrackIndex)
}

// GetOrCreateAudioSession returns an existing audio-only session or creates a new one.
// Audio-only sessions transcode a specific audio track to AAC for HLS delivery.
func (m *SessionManager) GetOrCreateAudioSession(
	mediaID int64,
	audioTrackIndex int,
	startPosition float64,
	inputPath string,
	outputDir string,
	videoInfo *VideoInfo,
) (*TranscodeSession, error) {
	key := audioSessionKey(mediaID, audioTrackIndex)

	// Check for existing session
	if existing, ok := m.sessions.Load(key); ok {
		session := existing.(*TranscodeSession)

		// Reuse if start position is compatible
		positionDiff := startPosition - session.StartPosition
		if startPosition == 0 || (positionDiff >= 0 && positionDiff <= 30) {
			session.UpdateLastAccessed()
			m.logger.Debug("Reusing existing audio session",
				"session_id", session.ID,
				"media_id", mediaID,
				"audio_track", audioTrackIndex)
			return session, nil
		}

		// Need to restart for different position
		session.Stop()
		m.sessions.Delete(key)
		m.cleanupOutputDir(session.OutputDir, session.ID)
	}

	// Create new audio-only session
	quality := fmt.Sprintf("audio_%d", audioTrackIndex)
	session := NewTranscodeSession(mediaID, quality, startPosition, outputDir, m.logger)

	// Get effective config
	effectiveConfig := m.getEffectiveConfig(context.Background())

	// Start audio-only FFmpeg process
	if err := session.StartAudioOnly(inputPath, audioTrackIndex, effectiveConfig, videoInfo); err != nil {
		return nil, fmt.Errorf("failed to start audio transcode session: %w", err)
	}

	// Store session
	m.sessions.Store(key, session)

	m.logger.Debug("Created new audio transcode session",
		"session_id", session.ID,
		"media_id", mediaID,
		"audio_track", audioTrackIndex,
		"start_position", startPosition)

	return session, nil
}


// stopOtherMediaSessions stops all sessions for media IDs other than the specified one.
// This prevents resource hogging when users switch between different videos.
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

	// Stop and clean up sessions for other media
	for _, key := range toStop {
		if existing, ok := m.sessions.Load(key); ok {
			session := existing.(*TranscodeSession)

			m.logger.Info("Stopping session for different media",
				"session_id", session.ID,
				"media_id", session.MediaID,
				"current_media_id", currentMediaID)

			session.Stop()
			m.sessions.Delete(key)

			// Clean up output directory immediately
			m.cleanupOutputDir(session.OutputDir, session.ID)
		}
	}
}

// stopOtherQualitySessions stops actively transcoding sessions for the same media but different qualities.
// This prevents resource contention when HLS.js requests multiple qualities after seeking.
// Completed sessions are kept so their segments remain available for playback.
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

	// Stop and clean up only ACTIVE sessions
	for _, key := range toStop {
		if existing, ok := m.sessions.Load(key); ok {
			session := existing.(*TranscodeSession)

			m.logger.Info("Stopping active quality session to reduce contention",
				"stopped_session", session.ID,
				"stopped_quality", session.Quality,
				"current_quality", currentQuality,
				"media_id", currentMediaID)

			session.Stop()
			m.sessions.Delete(key)

			// Clean up output directory immediately
			m.cleanupOutputDir(session.OutputDir, session.ID)
		}
	}
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
			}
		}
	}()

	// Return cancel function
	return func() {
		done <- true
	}
}
