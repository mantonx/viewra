package playbackmodule

import (
	"context"
	"io"
	"time"

	"github.com/mantonx/viewra/pkg/plugins"
)

// DeviceProfile captures client playback capabilities
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

// SubtitleConfig defines subtitle handling options
type SubtitleConfig struct {
	Enabled   bool   `json:"enabled"`
	Language  string `json:"language"`
	BurnIn    bool   `json:"burn_in"`
	StreamIdx int    `json:"stream_idx"`
}

// TranscodeRequest represents a transcoding request
type TranscodeRequest struct {
	InputPath   string          `json:"input_path"`
	TargetCodec string          `json:"target_codec"`
	Resolution  string          `json:"resolution"`
	Bitrate     int             `json:"bitrate"`
	Subtitles   *SubtitleConfig `json:"subtitles,omitempty"`
	AudioStream int             `json:"audio_stream"`
	DeviceHints DeviceProfile   `json:"device_hints"`
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
	DecidePlayback(mediaPath string, deviceProfile *plugins.DeviceProfile) (*PlaybackDecision, error)
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
	// RegisterTranscoder registers a transcoding plugin
	RegisterTranscoder(name string, transcoder plugins.TranscodingService) error

	// DiscoverTranscodingPlugins discovers and registers all available transcoding plugins
	DiscoverTranscodingPlugins() error

	// CanTranscode checks if transcoding is available for given parameters without starting a session
	CanTranscode(req *plugins.TranscodeRequest) error

	// StartTranscode starts a new transcoding session
	StartTranscode(req *plugins.TranscodeRequest) (*plugins.TranscodeSession, error)

	// GetSession retrieves a transcoding session
	GetSession(sessionID string) (*plugins.TranscodeSession, error)

	// StopSession stops a transcoding session
	StopSession(sessionID string) error

	// ListSessions lists all active sessions
	ListSessions() ([]*plugins.TranscodeSession, error)

	// GetStats returns transcoding statistics
	GetStats() (*TranscodingStats, error)

	// GetTranscodeStream returns the transcoding service for streaming
	GetTranscodeStream(sessionID string) (plugins.TranscodingService, error)

	// Cleanup performs cleanup of expired sessions
	Cleanup()

	// GetCleanupStats returns cleanup-related statistics
	GetCleanupStats() (*CleanupStats, error)
}

// TranscodingStats represents overall transcoding statistics
type TranscodingStats struct {
	ActiveSessions    int                         `json:"active_sessions"`
	TotalSessions     int64                       `json:"total_sessions"`
	CompletedSessions int64                       `json:"completed_sessions"`
	FailedSessions    int64                       `json:"failed_sessions"`
	TotalBytesOut     int64                       `json:"total_bytes_out"`
	AverageSpeed      float64                     `json:"average_speed"`
	Backends          map[string]*BackendStats    `json:"backends"`
	RecentSessions    []*plugins.TranscodeSession `json:"recent_sessions"`
}

// BackendStats represents statistics for a specific transcoding backend
type BackendStats struct {
	Name           string                           `json:"name"`
	Priority       int                              `json:"priority"`
	ActiveSessions int                              `json:"active_sessions"`
	TotalSessions  int64                            `json:"total_sessions"`
	SuccessRate    float64                          `json:"success_rate"`
	AverageSpeed   float64                          `json:"average_speed"`
	Capabilities   *plugins.TranscodingCapabilities `json:"capabilities"`
	LastUsed       *time.Time                       `json:"last_used,omitempty"`
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

// Codec represents a video/audio codec
type Codec string

const (
	CodecH264 Codec = "h264"
	CodecHEVC Codec = "hevc"
	CodecVP8  Codec = "vp8"
	CodecVP9  Codec = "vp9"
	CodecAV1  Codec = "av1"
)

// Resolution represents video resolution
type Resolution string

const (
	Res480p  Resolution = "480p"
	Res720p  Resolution = "720p"
	Res1080p Resolution = "1080p"
	Res1440p Resolution = "1440p"
	Res2160p Resolution = "2160p"
)

// TranscodeSession represents an active transcoding session
type TranscodeSession struct {
	ID        string           `json:"id"`
	Request   TranscodeRequest `json:"request"`
	Status    SessionStatus    `json:"status"`
	StartTime time.Time        `json:"start_time"`
	Backend   string           `json:"backend"`
	Stream    io.ReadCloser    `json:"-"`
}

// SessionStatus represents the status of a transcoding session
type SessionStatus string

const (
	StatusPending  SessionStatus = "pending"
	StatusRunning  SessionStatus = "running"
	StatusComplete SessionStatus = "complete"
	StatusFailed   SessionStatus = "failed"
)

// TranscodeProfile represents a reusable quality profile
type TranscodeProfile struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	VideoCodec  Codec             `json:"video_codec"`
	Resolution  Resolution        `json:"resolution"`
	Bitrate     int               `json:"bitrate"`
	Options     map[string]string `json:"options"`
}

// TranscodeProfileManager interface - Optional component
type TranscodeProfileManager interface {
	GetProfile(name string) (*TranscodeProfile, error)
	ListProfiles() []*TranscodeProfile
	CreateProfile(profile TranscodeProfile) error
	DeleteProfile(name string) error
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
