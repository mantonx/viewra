package playbackmodule

import (
	"context"
	"io"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/modules/playbackmodule/core"
	"github.com/mantonx/viewra/pkg/plugins"
)

// DeviceProfile captures client playback capabilities
// This is used for decision-making, not transcoding parameters
type DeviceProfile struct {
	UserAgent       string   `json:"user_agent"`
	SupportedCodecs []string `json:"supported_codecs"`
	MaxResolution   string   `json:"max_resolution"`
	MaxBitrate      int      `json:"max_bitrate"`
	SupportsHEVC    bool     `json:"supports_hevc"`
	SupportsAV1     bool     `json:"supports_av1"`
	SupportsHDR     bool     `json:"supports_hdr"`
	ClientIP        string   `json:"client_ip"`
}

// PlaybackDecision represents the decision made by the planner
type PlaybackDecision struct {
	ShouldTranscode bool                      `json:"should_transcode"`
	DirectPlayURL   string                    `json:"direct_play_url,omitempty"`
	TranscodeParams *plugins.TranscodeRequest `json:"transcode_params,omitempty"`
	Reason          string                    `json:"reason"`
}

// PlaybackPlanner interface for making playback decisions
type PlaybackPlanner interface {
	// DecidePlayback determines whether to direct play or transcode
	DecidePlayback(mediaPath string, deviceProfile *DeviceProfile) (*PlaybackDecision, error)
}

// PluginInfo represents plugin information
type PluginInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Author      string `json:"author"`
	Status      string `json:"status"`
}

// PluginManagerInterface defines the interface for plugin management
type PluginManagerInterface interface {
	GetRunningPluginInterface(pluginID string) (interface{}, bool)
	ListPlugins() []PluginInfo
	GetRunningPlugins() []PluginInfo
}

// TranscodeManager interface for managing transcoding sessions
type TranscodeManager interface {
	// RegisterProvider registers a transcoding provider from a plugin
	RegisterProvider(pluginID string, provider plugins.TranscodingProvider) error

	// DiscoverTranscodingPlugins discovers and registers all available transcoding plugins
	DiscoverTranscodingPlugins() error

	// CanTranscode checks if transcoding is available for given parameters without starting a session
	CanTranscode(req *plugins.TranscodeRequest) error

	// StartTranscode starts a new transcoding session using the service
	StartTranscode(req *plugins.TranscodeRequest) (*database.TranscodeSession, error)

	// GetSession retrieves a transcoding session
	GetSession(sessionID string) (*database.TranscodeSession, error)

	// StopSession stops a transcoding session
	StopSession(sessionID string) error

	// ListSessions lists all active sessions
	ListSessions() ([]*database.TranscodeSession, error)

	// GetStats returns transcoding statistics
	GetStats() (*TranscodingStats, error)

	// GetTranscodeService returns the core transcode service
	GetTranscodeService() *core.TranscodeService

	// Cleanup performs cleanup of expired sessions
	Cleanup()

	// GetCleanupStats returns cleanup-related statistics
	GetCleanupStats() (*CleanupStats, error)

	// Streaming methods
	StartStreamingTranscode(req *plugins.TranscodeRequest) (*plugins.StreamHandle, error)
	GetStream(sessionID string) (io.ReadCloser, error)
	StopStreamingTranscode(sessionID string) error
}

// TranscodingStats represents overall transcoding statistics
type TranscodingStats struct {
	ActiveSessions    int                          `json:"active_sessions"`
	TotalSessions     int64                        `json:"total_sessions"`
	CompletedSessions int64                        `json:"completed_sessions"`
	FailedSessions    int64                        `json:"failed_sessions"`
	TotalBytesOut     int64                        `json:"total_bytes_out"`
	AverageSpeed      float64                      `json:"average_speed"`
	Backends          map[string]*BackendStats     `json:"backends"`
	RecentSessions    []*database.TranscodeSession `json:"recent_sessions"`
}

// BackendStats represents statistics for a specific transcoding backend
type BackendStats struct {
	Name           string                 `json:"name"`
	Priority       int                    `json:"priority"`
	ActiveSessions int                    `json:"active_sessions"`
	TotalSessions  int64                  `json:"total_sessions"`
	SuccessRate    float64                `json:"success_rate"`
	AverageSpeed   float64                `json:"average_speed"`
	Capabilities   map[string]interface{} `json:"capabilities"`
	LastUsed       *time.Time             `json:"last_used,omitempty"`
}

// CleanupStats represents statistics about file cleanup operations
type CleanupStats struct {
	TotalDirectories       int       `json:"total_directories"`
	TotalSizeGB            float64   `json:"total_size_gb"`
	DirectoriesRemoved     int       `json:"directories_removed"`
	SizeFreedGB            float64   `json:"size_freed_gb"`
	LastCleanupTime        time.Time `json:"last_cleanup_time"`
	NextCleanupTime        time.Time `json:"next_cleanup_time"`
	RetentionHours         int       `json:"retention_hours"`
	ExtendedRetentionHours int       `json:"extended_retention_hours"`
	MaxSizeLimitGB         int       `json:"max_size_limit_gb"`
}

// MediaInfo represents file metadata
type MediaInfo struct {
	Container    string `json:"container"`
	VideoCodec   string `json:"video_codec"`
	AudioCodec   string `json:"audio_codec"`
	Resolution   string `json:"resolution"`
	Bitrate      int64  `json:"bitrate"`
	Duration     int64  `json:"duration"`
	HasHDR       bool   `json:"has_hdr"`
	HasSubtitles bool   `json:"has_subtitles"`
}

// TranscodingJob represents a running transcoding process
type TranscodingJob struct {
	SessionID string
	Process   interface{} // Platform-specific process handle
	Output    io.ReadCloser
	Cancel    context.CancelFunc
}

// PlaybackModuleConfig represents configuration for the playback module
type PlaybackModuleConfig struct {
	MaxConcurrentSessions int               `json:"max_concurrent_sessions"`
	EnableHardwareAccel   bool              `json:"enable_hardware_accel"`
	DefaultQuality        int               `json:"default_quality"`
	TranscodingTimeout    time.Duration     `json:"transcoding_timeout"`
	BufferSize            int               `json:"buffer_size"`
	EnableLogging         bool              `json:"enable_logging"`
	LogLevel              string            `json:"log_level"`
	CustomSettings        map[string]string `json:"custom_settings"`
}
