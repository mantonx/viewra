// Package transcoding provides FFmpeg-based video transcoding capabilities.
//
// The SessionManager is split across multiple files for better organization:
//   - session_manager.go: Core struct, constructor, session lifecycle (this file)
//   - session_manager_cleanup.go: Cleanup operations (idle sessions, old transcodes)
package transcoding

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// ConfigProvider provides transcoding configuration.
// This minimal interface allows the SessionManager to work with dynamic config
// from the application layer without creating import cycles.
type ConfigProvider interface {
	GetConfig(ctx context.Context) *TranscodeConfig
}

// SessionManager manages active transcode sessions.
// Handles session lifecycle, cleanup, and restart logic for seeking.
type SessionManager struct {
	sessions        sync.Map // map[string]*TranscodeSession (key: "mediaID_quality")
	sessionMu       sync.Map // map[string]*sync.Mutex - per-session creation locks to prevent race conditions
	logger          *slog.Logger
	config          *TranscodeConfig
	configProvider  ConfigProvider // Optional: for dynamic config from settings
	fallbackManager *HardwareFallbackManager
	logStore        *FFmpegLogStore // Optional: for persistent FFmpeg log capture
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

	// Log FFmpeg configuration at startup (uses Paths.Version() for consistency)
	if config.FFmpegPaths != nil {
		version := config.FFmpegPaths.Version()
		if config.FFmpegPaths.IsCustomBuild() {
			logger.Info("FFmpeg configured for transcoding",
				"path", config.FFmpegPaths.FFmpeg,
				"version", version,
				"source", "custom (VIEWRA_FFMPEG_PATH)",
				"lib_path", config.FFmpegPaths.LibPath,
			)
		} else {
			logger.Info("FFmpeg configured for transcoding",
				"path", config.FFmpegPaths.FFmpeg,
				"version", version,
				"source", "system PATH",
			)
		}
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

// SetLogStore sets the FFmpeg log store for persistent log capture.
// When set, all new transcode sessions will have their FFmpeg output captured.
func (m *SessionManager) SetLogStore(store *FFmpegLogStore) {
	m.logStore = store
}

// GetLogStore returns the FFmpeg log store (may be nil if not configured).
func (m *SessionManager) GetLogStore() *FFmpegLogStore {
	return m.logStore
}

// getEffectiveConfig returns the current configuration, using provider if available.
func (m *SessionManager) getEffectiveConfig(ctx context.Context) *TranscodeConfig {
	if m.configProvider != nil {
		return m.configProvider.GetConfig(ctx)
	}
	return m.config
}

// getSessionMutex returns a mutex for the given session key.
// Creates a new mutex if one doesn't exist. This ensures only one goroutine
// can create a session for a given key at a time, preventing duplicate FFmpeg processes.
func (m *SessionManager) getSessionMutex(key string) *sync.Mutex {
	mu := &sync.Mutex{}
	actual, _ := m.sessionMu.LoadOrStore(key, mu)
	return actual.(*sync.Mutex)
}

// GetOrCreateSession returns an existing session or creates a new one.
// Sessions are reused when possible to avoid re-transcoding segments that already exist.
// Multiple quality sessions can coexist to support ABR (Adaptive Bitrate) streaming.
// clientSupportedCodecs is a list of video codecs the client can decode (e.g., ["h264", "h265", "vp9"]).
// This is used to select the best codec from the profile's preferred/fallback list.
// audioTrackIndex specifies which audio track to mux (-1 means use default/first track).
func (m *SessionManager) GetOrCreateSession(
	mediaID int64,
	quality string,
	startPosition float64,
	audioTrackIndex int,
	inputPath string,
	profile *AdaptiveProfile,
	strategy StreamStrategy,
	outputDir string,
	videoInfo *VideoInfo,
	clientSupportedCodecs []string,
) (*TranscodeSession, error) {
	key := sessionKey(mediaID, quality, audioTrackIndex)

	// Stop all sessions for OTHER media to prevent resource hogging
	// When user switches to a different video, clean up the old one immediately
	m.stopOtherMediaSessions(mediaID)

	// NOTE: We intentionally do NOT stop other quality sessions for the same media.
	// This allows ABR to work properly - HLS.js can switch between qualities without
	// causing transcode sessions to be killed and restarted.

	// Acquire per-key mutex to prevent race conditions where multiple requests
	// concurrently create sessions for the same key. Without this lock, each request
	// would see no existing session, create its own FFmpeg process, and the last one
	// to call Store() would win - leaving orphaned FFmpeg processes.
	mu := m.getSessionMutex(key)
	mu.Lock()
	defer mu.Unlock()

	// Check for existing session (now safe from race conditions)
	// Reuse if start position is close enough to the session's start position.
	// This balances segment reuse with proper seek support.
	if existing, ok := m.sessions.Load(key); ok {
		session := existing.(*TranscodeSession)

		// Calculate if the requested position is reachable from the current session
		// A session starting at position S can serve content from S onwards
		// If requested position < session start, we need a new session (can't go backwards)
		positionDiff := startPosition - session.StartPosition

		// Reuse session if:
		// 1. Requested position is within the session's range:
		//    - Position must be >= session start (can't go backwards from where FFmpeg started)
		//    - Small seeks forward are fine (within 30 seconds ahead of session start)
		// 2. Special case: startPosition=0 and session also started from 0 (beginning)
		//
		// NOTE: We no longer treat startPosition=0 as "use any session". If user wants to
		// start from beginning, we should only reuse a session that also started from beginning.
		// This fixes the bug where restarting a video would incorrectly reuse a session that
		// was seeked to a later position.
		canReuse := (positionDiff >= 0 && positionDiff <= 30) // Forward seek within reasonable range

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

		// Stop the old session's FFmpeg process
		session.Stop()
		m.sessions.Delete(key)

		// DON'T cleanup segments here - they may still be useful:
		// 1. HLS.js may have pending requests for old segments from cached manifest
		// 2. User might seek back to a position we already transcoded
		// 3. Segments are numbered by time position, so new seek creates different segment numbers
		//
		// Cleanup happens when:
		// - User switches to a different media (stopOtherMediaSessions)
		// - Periodic LRU cleanup (CleanupOldTranscodes)
		// - Session idle timeout (CleanupIdleSessions)
	}

	// Create new session (without log writer initially)
	session := NewTranscodeSession(mediaID, quality, startPosition, audioTrackIndex, outputDir, m.logger, nil)

	// Create log writer for this session (if log store is configured)
	// We do this after session creation so we can use the actual session ID
	if m.logStore != nil {
		logWriter, err := m.logStore.CreateLogWriter(session.ID, mediaID, quality)
		if err != nil {
			m.logger.Warn("Failed to create FFmpeg log writer",
				"session_id", session.ID,
				"error", err)
			// Continue without logging - not a fatal error
		} else {
			session.SetLogWriter(logWriter)
		}
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

// GetSession retrieves an existing session by media ID, quality, and audio track.
// audioTrackIndex specifies which audio track (-1 or 0 = default).
func (m *SessionManager) GetSession(mediaID int64, quality string, audioTrackIndex int) (*TranscodeSession, error) {
	key := sessionKey(mediaID, quality, audioTrackIndex)

	if existing, ok := m.sessions.Load(key); ok {
		session := existing.(*TranscodeSession)
		session.UpdateLastAccessed()
		return session, nil
	}

	// If audio track not specified, also try to find a session with default audio
	if audioTrackIndex < 0 {
		// Try with audioTrackIndex = 0 (default track)
		defaultKey := sessionKey(mediaID, quality, 0)
		if existing, ok := m.sessions.Load(defaultKey); ok {
			session := existing.(*TranscodeSession)
			session.UpdateLastAccessed()
			return session, nil
		}
	}

	return nil, fmt.Errorf("no active transcode session for media %d quality %s", mediaID, quality)
}

// StopSession stops a specific session and removes it from the manager.
// audioTrackIndex specifies which audio track (-1 or 0 = default).
func (m *SessionManager) StopSession(mediaID int64, quality string, audioTrackIndex int) error {
	key := sessionKey(mediaID, quality, audioTrackIndex)

	if existing, ok := m.sessions.Load(key); ok {
		session := existing.(*TranscodeSession)
		session.Stop()
		m.sessions.Delete(key)

		// Close log writer if present
		if m.logStore != nil {
			m.logStore.CloseLogWriter(session.ID)
		}

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

	m.sessions.Range(func(key, value any) bool {
		session := value.(*TranscodeSession)
		session.Stop()

		// Close log writer if present
		if m.logStore != nil {
			m.logStore.CloseLogWriter(session.ID)
		}

		// Clean up output directory
		m.cleanupOutputDir(session.OutputDir, session.ID)

		return true
	})

	// Clear all sessions
	m.sessions = sync.Map{}
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
// audioTrackIndex specifies which audio track (-1 or 0 = default).
func (m *SessionManager) GetSessionOutputPath(mediaID int64, quality string, audioTrackIndex int) (string, error) {
	key := sessionKey(mediaID, quality, audioTrackIndex)

	if existing, ok := m.sessions.Load(key); ok {
		session := existing.(*TranscodeSession)
		return session.OutputDir, nil
	}

	// If audio track not specified, also try with default
	if audioTrackIndex < 0 {
		defaultKey := sessionKey(mediaID, quality, 0)
		if existing, ok := m.sessions.Load(defaultKey); ok {
			session := existing.(*TranscodeSession)
			return session.OutputDir, nil
		}
	}

	return "", fmt.Errorf("no active session for media %d quality %s", mediaID, quality)
}

// sessionKey generates a unique key for a media/quality/audio combination.
// audioTrackIndex of -1 or 0 uses the default audio track (no suffix).
func sessionKey(mediaID int64, quality string, audioTrackIndex int) string {
	if audioTrackIndex > 0 {
		return fmt.Sprintf("%d:%s:audio%d", mediaID, quality, audioTrackIndex)
	}
	return fmt.Sprintf("%d:%s", mediaID, quality)
}
