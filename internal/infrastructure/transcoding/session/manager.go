package session

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/mantonx/viewra/internal/infrastructure/ffmpeg/hls"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/logging"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/videoinfo"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/profile"
)

// ConfigProvider provides transcoding configuration.
// This minimal interface allows the SessionManager to work with dynamic config
// from the application layer without creating import cycles.
type ConfigProvider interface {
	GetConfig(ctx context.Context) *Config
}

// Manager manages active transcode sessions.
// Handles session lifecycle, cleanup, and restart logic for seeking.
type Manager struct {
	sessions        sync.Map // map[string]*TranscodeSession (key: "mediaID_quality")
	sessionMu       sync.Map // map[string]*sync.Mutex - per-session creation locks
	logger          *slog.Logger
	config          *Config
	hwAccel         string // Default hardware acceleration (from ManagerConfig)
	hwDevice        string // Default hardware device (from ManagerConfig)
	configProvider  ConfigProvider
	fallbackManager *hls.HardwareFallbackManager
	logStore        *logging.FFmpegLogStore
}

// ManagerConfig contains all configuration needed for the session manager.
type ManagerConfig struct {
	FFmpegPaths                *hls.Paths
	HardwareAccel              string // "none", "vaapi", "nvenc", "qsv", "videotoolbox"
	HardwareDevice             string
	OutputBaseDir              string
	MinFreeDiskGB              int64
	MaxCPUPercent              int
	MaxMemoryMB                int
	ProcessGroupKill           bool
	ToneMappingEnabled         bool
	ToneMappingAlgorithm       string
	ToneMappingBackend         string
	LibPlaceboPeakDetect       bool
	LibPlaceboContrastRecovery float64
}

// NewManager creates a new session manager.
func NewManager(config *ManagerConfig, logger *slog.Logger) *Manager {
	if config == nil {
		config = &ManagerConfig{
			HardwareAccel:      "none",
			ToneMappingEnabled: true,
		}
	}

	sessionConfig := &Config{
		FFmpegPaths:                config.FFmpegPaths,
		MaxMemoryMB:                config.MaxMemoryMB,
		ToneMappingEnabled:         config.ToneMappingEnabled,
		ToneMappingAlgorithm:       config.ToneMappingAlgorithm,
		ToneMappingBackend:         config.ToneMappingBackend,
		LibPlaceboPeakDetect:       config.LibPlaceboPeakDetect,
		LibPlaceboContrastRecovery: config.LibPlaceboContrastRecovery,
	}

	// Convert to hls config for fallback manager
	hlsConfig := &hls.Config{
		HardwareAccel: hls.HardwareAccel(config.HardwareAccel),
	}
	if config.FFmpegPaths != nil {
		hlsConfig.FFmpegPath = config.FFmpegPaths.FFmpeg
		hlsConfig.FFmpegLibPath = config.FFmpegPaths.LibPath
	}

	mgr := &Manager{
		logger:          logger,
		config:          sessionConfig,
		hwAccel:         config.HardwareAccel,
		hwDevice:        config.HardwareDevice,
		fallbackManager: hls.NewHardwareFallbackManager(hlsConfig, logger),
	}

	// Verify hardware acceleration is available
	hwAccel := hls.HardwareAccel(config.HardwareAccel)
	if hwAccel != hls.AccelNone {
		if err := mgr.fallbackManager.VerifyHardwareAvailability(hwAccel); err != nil {
			logger.Warn("Hardware acceleration verification failed, falling back to software",
				"hardware", config.HardwareAccel,
				"error", err,
			)
		}
	}

	return mgr
}

// SetConfigProvider sets a dynamic config provider for runtime settings.
func (m *Manager) SetConfigProvider(provider ConfigProvider) {
	m.configProvider = provider
}

// SetLogStore sets the FFmpeg log store for persistent log capture.
func (m *Manager) SetLogStore(store *logging.FFmpegLogStore) {
	m.logStore = store
}

// GetLogStore returns the FFmpeg log store (may be nil if not configured).
func (m *Manager) GetLogStore() *logging.FFmpegLogStore {
	return m.logStore
}

// getEffectiveConfig returns the current configuration, using provider if available.
func (m *Manager) getEffectiveConfig(ctx context.Context) *Config {
	if m.configProvider != nil {
		return m.configProvider.GetConfig(ctx)
	}
	return m.config
}

// getSessionMutex returns a mutex for the given session key.
func (m *Manager) getSessionMutex(key string) *sync.Mutex {
	mu := &sync.Mutex{}
	actual, _ := m.sessionMu.LoadOrStore(key, mu)
	return actual.(*sync.Mutex)
}

// GetOrCreateSessionParams contains parameters for GetOrCreateSession.
type GetOrCreateSessionParams struct {
	MediaID               int64
	Quality               string
	StartPosition         float64
	AudioTrackIndex       int
	InputPath             string
	Profile               *profile.AdaptiveProfile
	Strategy              string
	OutputDir             string
	VideoInfo             *videoinfo.VideoInfo
	ClientSupportedCodecs []string
	HWAccel               string
	HWDevice              string
}

// GetOrCreateSession returns an existing session or creates a new one.
func (m *Manager) GetOrCreateSession(params GetOrCreateSessionParams) (*TranscodeSession, error) {
	// Use manager's default HW settings if not specified in params
	hwAccel := params.HWAccel
	if hwAccel == "" {
		hwAccel = m.hwAccel
	}
	hwDevice := params.HWDevice
	if hwDevice == "" {
		hwDevice = m.hwDevice
	}

	key := sessionKey(params.MediaID, params.Quality, params.AudioTrackIndex)

	// Stop all sessions for OTHER media to prevent resource hogging
	m.stopOtherMediaSessions(params.MediaID)

	// Acquire per-key mutex to prevent race conditions
	mu := m.getSessionMutex(key)
	mu.Lock()
	defer mu.Unlock()

	// Check for existing session
	if existing, ok := m.sessions.Load(key); ok {
		session := existing.(*TranscodeSession)

		// Determine reuse window based on strategy.
		// Remux strategies have fast startup (~1-3s), so we can afford a larger window
		// to reduce unnecessary session restarts during typical navigation patterns.
		// Transcode has slow startup (~10-30s), so use a smaller window to avoid
		// long waits when the user seeks far ahead.
		reuseWindowSec := 30.0 // Default for transcode
		isRemuxStrategy := params.Strategy == "remux" || params.Strategy == "remux_audio" || params.Strategy == "remux_hevc"
		if isRemuxStrategy {
			reuseWindowSec = 60.0 // Larger window for fast-startup remux
		}

		positionDiff := params.StartPosition - session.StartPosition
		canReuse := (positionDiff >= 0 && positionDiff <= reuseWindowSec)

		if canReuse {
			session.UpdateLastAccessed()

			m.logger.Debug("Reusing existing transcode session",
				"session_id", session.ID,
				"media_id", params.MediaID,
				"quality", params.Quality,
				"session_start", session.StartPosition,
				"requested_start", params.StartPosition,
				"position_diff", positionDiff,
				"reuse_window_sec", reuseWindowSec)

			return session, nil
		}

		// Position is outside reusable range - need to restart session
		m.logger.Info("Restarting session for seek operation",
			"session_id", session.ID,
			"media_id", params.MediaID,
			"quality", params.Quality,
			"session_start", session.StartPosition,
			"requested_start", params.StartPosition,
			"position_diff", positionDiff,
			"reuse_window_sec", reuseWindowSec)

		session.Stop()
		m.sessions.Delete(key)

		// Close log writer for old session
		if m.logStore != nil {
			m.logStore.CloseLogWriter(session.ID)
		}

		// Clean up old output directory before creating new session
		// This prevents stale segments from interfering with the new session
		m.cleanupOutputDir(session.OutputDir, session.ID)
	}

	// Create new session
	session := NewTranscodeSession(
		params.MediaID,
		params.Quality,
		params.StartPosition,
		params.AudioTrackIndex,
		params.OutputDir,
		m.logger,
		nil,
	)

	// Create log writer for this session
	if m.logStore != nil {
		logWriter, err := m.logStore.CreateLogWriter(session.ID, params.MediaID, params.Quality)
		if err != nil {
			m.logger.Warn("Failed to create FFmpeg log writer",
				"session_id", session.ID,
				"error", err)
		} else {
			session.SetLogWriter(logWriter)
		}
	}

	// Get effective config
	effectiveConfig := m.getEffectiveConfig(context.Background())

	// Convert videoinfo.VideoInfo to hls.VideoInfo
	var videoInfo *hls.VideoInfo
	if params.VideoInfo != nil {
		videoInfo = convertProbeToHLSVideoInfo(params.VideoInfo)
	}

	// Start FFmpeg process
	startParams := StartParams{
		InputPath:             params.InputPath,
		Profile:               params.Profile,
		Strategy:              params.Strategy,
		HWAccel:               hwAccel,
		HWDevice:              hwDevice,
		VideoInfo:             videoInfo,
		Config:                effectiveConfig,
		ClientSupportedCodecs: params.ClientSupportedCodecs,
	}

	if err := session.Start(startParams); err != nil {
		// Check if this is a hardware error and fallback if needed
		hwAccelType := hls.HardwareAccel(hwAccel)
		if m.fallbackManager.RecordFailure(hwAccelType, err) {
			m.logger.Info("Retrying with fallback acceleration",
				"from", hwAccel,
				"to", m.fallbackManager.GetCurrentAccel(),
			)
			startParams.HWAccel = string(m.fallbackManager.GetCurrentAccel())
			if err := session.Start(startParams); err != nil {
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
		"media_id", params.MediaID,
		"quality", params.Quality,
		"start_position", params.StartPosition)

	return session, nil
}

// GetSession retrieves an existing session by media ID, quality, and audio track.
func (m *Manager) GetSession(mediaID int64, quality string, audioTrackIndex int) (*TranscodeSession, error) {
	key := sessionKey(mediaID, quality, audioTrackIndex)

	if existing, ok := m.sessions.Load(key); ok {
		session := existing.(*TranscodeSession)
		session.UpdateLastAccessed()
		return session, nil
	}

	// If audio track not specified, also try to find a session with default audio
	if audioTrackIndex < 0 {
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
func (m *Manager) StopSession(mediaID int64, quality string, audioTrackIndex int) error {
	key := sessionKey(mediaID, quality, audioTrackIndex)

	if existing, ok := m.sessions.Load(key); ok {
		session := existing.(*TranscodeSession)
		session.Stop()
		m.sessions.Delete(key)

		if m.logStore != nil {
			m.logStore.CloseLogWriter(session.ID)
		}

		m.cleanupOutputDir(session.OutputDir, session.ID)

		m.logger.Info("Stopped transcode session", "session_id", session.ID)
		return nil
	}

	return fmt.Errorf("session not found")
}

// StopAllSessions stops all active sessions.
func (m *Manager) StopAllSessions() {
	m.logger.Info("Stopping all transcode sessions")

	m.sessions.Range(func(key, value any) bool {
		session := value.(*TranscodeSession)
		session.Stop()

		if m.logStore != nil {
			m.logStore.CloseLogWriter(session.ID)
		}

		m.cleanupOutputDir(session.OutputDir, session.ID)

		return true
	})

	m.sessions = sync.Map{}
}

// GetStats returns statistics about active sessions.
func (m *Manager) GetStats() map[string]interface{} {
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
func (m *Manager) GetSessionOutputPath(mediaID int64, quality string, audioTrackIndex int) (string, error) {
	key := sessionKey(mediaID, quality, audioTrackIndex)

	if existing, ok := m.sessions.Load(key); ok {
		session := existing.(*TranscodeSession)
		return session.OutputDir, nil
	}

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
func sessionKey(mediaID int64, quality string, audioTrackIndex int) string {
	if audioTrackIndex > 0 {
		return fmt.Sprintf("%d:%s:audio%d", mediaID, quality, audioTrackIndex)
	}
	return fmt.Sprintf("%d:%s", mediaID, quality)
}

// convertProbeToHLSVideoInfo converts videoinfo.VideoInfo to hls.VideoInfo
func convertProbeToHLSVideoInfo(v *videoinfo.VideoInfo) *hls.VideoInfo {
	if v == nil {
		return nil
	}
	return &hls.VideoInfo{
		Codec:           v.Codec,
		Width:           v.Width,
		Height:          v.Height,
		Bitrate:         v.Bitrate,
		Duration:        v.Duration,
		AudioCodec:      v.AudioCodec,
		AudioBitrate:    v.AudioBitrate,
		AudioChannels:   v.AudioChannels,
		ContainerFormat: v.ContainerFormat,
		PixelFormat:     v.PixelFormat,
		ColorSpace:      v.ColorSpace,
		ColorPrimaries:  v.ColorPrimaries,
		ColorTransfer:   v.ColorTransfer,
		BitDepth:        v.BitDepth,
		IsHDR:           v.IsHDR,
	}
}
