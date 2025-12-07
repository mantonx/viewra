package transcoding

import (
	"context"
	"log/slog"
	"time"

	"github.com/mantonx/viewra/internal/infrastructure/transcoding/logging"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/session"
)

// Re-export session package types for backward compatibility.
// New code should import the session package directly:
//   import "github.com/mantonx/viewra/internal/infrastructure/transcoding/session"

// TranscodeSession is re-exported from the session subpackage for backward compatibility.
type TranscodeSession = session.TranscodeSession

// SessionManager is an alias for session.Manager for backward compatibility.
// New code should use session.Manager directly.
type SessionManager = session.Manager

// ProgressWatchdog is re-exported from the session subpackage for backward compatibility.
type ProgressWatchdog = session.ProgressWatchdog

// SessionStartParams is re-exported from the session subpackage.
type SessionStartParams = session.StartParams

// SessionConfig is re-exported from the session subpackage.
type SessionConfig = session.Config

// SessionManagerConfig is re-exported from the session subpackage.
type SessionManagerConfig = session.ManagerConfig

// GetOrCreateSessionParams is re-exported from the session subpackage.
type GetOrCreateSessionParams = session.GetOrCreateSessionParams

// ConfigProvider is re-exported from the session subpackage.
type ConfigProvider = session.ConfigProvider

// NewTranscodeSession creates a new transcode session.
// Re-exported from session subpackage for backward compatibility.
func NewTranscodeSession(
	mediaID int64,
	quality string,
	startPosition float64,
	audioTrackIndex int,
	outputDir string,
	logger *slog.Logger,
	logWriter *logging.LogWriter,
) *TranscodeSession {
	return session.NewTranscodeSession(
		mediaID,
		quality,
		startPosition,
		audioTrackIndex,
		outputDir,
		logger,
		logWriter,
	)
}

// NewSessionManager creates a new session manager.
// Re-exported from session subpackage for backward compatibility.
func NewSessionManager(config *TranscodeConfig, logger *slog.Logger) *SessionManager {
	// Convert TranscodeConfig to session.ManagerConfig
	managerConfig := &session.ManagerConfig{
		FFmpegPaths:                config.FFmpegPaths,
		HardwareAccel:              string(config.HardwareAccel),
		HardwareDevice:             config.HardwareDevice,
		OutputBaseDir:              config.OutputBaseDir,
		MinFreeDiskGB:              config.MinFreeDiskGB,
		MaxCPUPercent:              config.MaxCPUPercent,
		MaxMemoryMB:                config.MaxMemoryMB,
		ProcessGroupKill:           config.ProcessGroupKill,
		ToneMappingEnabled:         config.ToneMappingEnabled,
		ToneMappingAlgorithm:       config.ToneMappingAlgorithm,
		ToneMappingBackend:         config.ToneMappingBackend,
		LibPlaceboPeakDetect:       config.LibPlaceboPeakDetect,
		LibPlaceboContrastRecovery: config.LibPlaceboContrastRecovery,
	}
	return session.NewManager(managerConfig, logger)
}

// NewProgressWatchdog creates a new progress watchdog.
// Re-exported from session subpackage for backward compatibility.
func NewProgressWatchdog(sess *TranscodeSession, timeout time.Duration) *ProgressWatchdog {
	return session.NewProgressWatchdog(sess, timeout)
}

// TranscodeConfigProvider is an interface for providing TranscodeConfig.
// This allows the application layer to provide config dynamically.
type TranscodeConfigProvider interface {
	GetConfig(ctx context.Context) *TranscodeConfig
}

// configProviderAdapter adapts a TranscodeConfigProvider to session.ConfigProvider.
// This bridges the gap between the infrastructure's TranscodeConfig and
// the session package's Config.
type configProviderAdapter struct {
	provider TranscodeConfigProvider
}

// GetConfig implements session.ConfigProvider by converting TranscodeConfig to session.Config.
func (a *configProviderAdapter) GetConfig(ctx context.Context) *session.Config {
	tc := a.provider.GetConfig(ctx)
	if tc == nil {
		return nil
	}
	return &session.Config{
		FFmpegPaths:                tc.FFmpegPaths,
		MaxMemoryMB:                tc.MaxMemoryMB,
		ToneMappingEnabled:         tc.ToneMappingEnabled,
		ToneMappingAlgorithm:       tc.ToneMappingAlgorithm,
		ToneMappingBackend:         tc.ToneMappingBackend,
		LibPlaceboPeakDetect:       tc.LibPlaceboPeakDetect,
		LibPlaceboContrastRecovery: tc.LibPlaceboContrastRecovery,
	}
}

// SetConfigProvider sets a dynamic config provider on the session manager.
// This wrapper converts the TranscodeConfigProvider to the session.ConfigProvider
// interface expected by the session package.
func SetConfigProvider(sm *SessionManager, provider TranscodeConfigProvider) {
	sm.SetConfigProvider(&configProviderAdapter{provider: provider})
}
