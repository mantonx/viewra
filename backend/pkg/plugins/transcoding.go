package plugins

import (
	"context"
	"io"
	"time"
)

// TranscodingService interface for transcoding plugins
type TranscodingService interface {
	// GetCapabilities returns what codecs and resolutions this transcoder supports
	GetCapabilities(ctx context.Context) (*TranscodingCapabilities, error)

	// StartTranscode initiates a transcoding session
	StartTranscode(ctx context.Context, req *TranscodeRequest) (*TranscodeSession, error)

	// GetTranscodeSession retrieves information about an active session
	GetTranscodeSession(ctx context.Context, sessionID string) (*TranscodeSession, error)

	// StopTranscode terminates a transcoding session
	StopTranscode(ctx context.Context, sessionID string) error

	// ListActiveSessions returns all currently active transcoding sessions
	ListActiveSessions(ctx context.Context) ([]*TranscodeSession, error)

	// GetTranscodeStream returns the output stream for a transcoding session
	GetTranscodeStream(ctx context.Context, sessionID string) (io.ReadCloser, error)
}

// TranscodingCapabilities describes what a transcoder can handle
type TranscodingCapabilities struct {
	Name                  string              `json:"name"`
	SupportedCodecs       []string            `json:"supported_codecs"`
	SupportedResolutions  []string            `json:"supported_resolutions"`
	SupportedContainers   []string            `json:"supported_containers"`
	HardwareAcceleration  bool                `json:"hardware_acceleration"`
	MaxConcurrentSessions int                 `json:"max_concurrent_sessions"`
	Features              TranscodingFeatures `json:"features"`
	Priority              int                 `json:"priority"` // Higher = preferred
}

// TranscodingFeatures describes advanced transcoding features
type TranscodingFeatures struct {
	SubtitleBurnIn      bool `json:"subtitle_burn_in"`
	SubtitlePassthrough bool `json:"subtitle_passthrough"`
	MultiAudioTracks    bool `json:"multi_audio_tracks"`
	HDRSupport          bool `json:"hdr_support"`
	ToneMapping         bool `json:"tone_mapping"`
	StreamingOutput     bool `json:"streaming_output"`
	SegmentedOutput     bool `json:"segmented_output"`
}

// TranscodeRequest represents a transcoding request
type TranscodeRequest struct {
	// Input
	InputPath string `json:"input_path"`
	StartTime int    `json:"start_time,omitempty"` // Start time in seconds for seek-ahead

	// Output format
	TargetCodec     string `json:"target_codec"`
	TargetContainer string `json:"target_container"`
	Resolution      string `json:"resolution"`
	Bitrate         int    `json:"bitrate"`

	// Audio settings
	AudioCodec   string `json:"audio_codec"`
	AudioBitrate int    `json:"audio_bitrate"`
	AudioStream  int    `json:"audio_stream"`

	// Subtitle settings
	Subtitles *SubtitleConfig `json:"subtitles,omitempty"`

	// Advanced settings
	Quality int               `json:"quality"` // CRF or quality level
	Preset  string            `json:"preset"`  // Encoding preset (fast, medium, slow, etc.)
	Options map[string]string `json:"options"` // Additional encoder options

	// Client information
	DeviceProfile *DeviceProfile `json:"device_profile,omitempty"`

	// Session settings
	Priority int `json:"priority"` // Session priority (1-10)
}

// SubtitleConfig defines subtitle handling
type SubtitleConfig struct {
	Enabled   bool   `json:"enabled"`
	Language  string `json:"language"`
	BurnIn    bool   `json:"burn_in"`
	StreamIdx int    `json:"stream_idx"`
	FontSize  int    `json:"font_size,omitempty"`
	FontColor string `json:"font_color,omitempty"`
}

// DeviceProfile captures client capabilities
type DeviceProfile struct {
	UserAgent       string   `json:"user_agent"`
	SupportedCodecs []string `json:"supported_codecs"`
	MaxResolution   string   `json:"max_resolution"`
	MaxBitrate      int      `json:"max_bitrate"`
	SupportsHEVC    bool     `json:"supports_hevc"`
	SupportsAV1     bool     `json:"supports_av1"`
	SupportsHDR     bool     `json:"supports_hdr"`
	ClientIP        string   `json:"client_ip"`
	Platform        string   `json:"platform"`
	Browser         string   `json:"browser"`
}

// TranscodeSession represents an active transcoding session
type TranscodeSession struct {
	ID        string                 `json:"id"`
	Request   *TranscodeRequest      `json:"request"`
	Status    TranscodeStatus        `json:"status"`
	Progress  float64                `json:"progress"` // 0.0 to 1.0
	StartTime time.Time              `json:"start_time"`
	EndTime   *time.Time             `json:"end_time,omitempty"`
	Backend   string                 `json:"backend"`
	Error     string                 `json:"error,omitempty"`
	Stats     *TranscodeStats        `json:"stats,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// TranscodeStatus represents the status of a transcoding session
type TranscodeStatus string

const (
	TranscodeStatusPending   TranscodeStatus = "pending"
	TranscodeStatusStarting  TranscodeStatus = "starting"
	TranscodeStatusRunning   TranscodeStatus = "running"
	TranscodeStatusCompleted TranscodeStatus = "completed"
	TranscodeStatusFailed    TranscodeStatus = "failed"
	TranscodeStatusCancelled TranscodeStatus = "cancelled"
)

// TranscodeStats contains transcoding statistics
type TranscodeStats struct {
	Duration        time.Duration `json:"duration"`
	BytesProcessed  int64         `json:"bytes_processed"`
	BytesGenerated  int64         `json:"bytes_generated"`
	FramesProcessed int64         `json:"frames_processed"`
	CurrentFPS      float64       `json:"current_fps"`
	AverageFPS      float64       `json:"average_fps"`
	CPUUsage        float64       `json:"cpu_usage"`
	MemoryUsage     int64         `json:"memory_usage"`
	Speed           float64       `json:"speed"` // Transcoding speed multiplier
}

// TranscodingServiceClient interface for communicating with transcoding services
type TranscodingServiceClient interface {
	GetCapabilities(ctx context.Context) (*TranscodingCapabilities, error)
	StartTranscode(ctx context.Context, req *TranscodeRequest) (*TranscodeSession, error)
	GetTranscodeSession(ctx context.Context, sessionID string) (*TranscodeSession, error)
	StopTranscode(ctx context.Context, sessionID string) error
	ListActiveSessions(ctx context.Context) ([]*TranscodeSession, error)
	GetTranscodeStream(ctx context.Context, sessionID string) (io.ReadCloser, error)
}
