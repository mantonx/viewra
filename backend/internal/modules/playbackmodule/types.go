package playbackmodule

import (
	"context"
	"io"
	"time"
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
	InputPath    string          `json:"input_path"`
	TargetCodec  string          `json:"target_codec"`
	Resolution   string          `json:"resolution"`
	Bitrate      int             `json:"bitrate"`
	Subtitles    *SubtitleConfig `json:"subtitles,omitempty"`
	AudioStream  int             `json:"audio_stream"`
	DeviceHints  DeviceProfile   `json:"device_hints"`
}

// PlaybackDecision represents the planner's decision
type PlaybackDecision struct {
	ShouldTranscode bool              `json:"should_transcode"`
	DirectPath      string            `json:"direct_path,omitempty"`
	TranscodeReq    *TranscodeRequest `json:"transcode_request,omitempty"`
	Reason          string            `json:"reason"`
}

// TranscodeSession represents an active transcoding session
type TranscodeSession struct {
	ID        string            `json:"id"`
	Request   TranscodeRequest  `json:"request"`
	Status    SessionStatus     `json:"status"`
	StartTime time.Time         `json:"start_time"`
	Backend   string            `json:"backend"`
	Stream    io.ReadCloser     `json:"-"`
}

// SessionStatus represents the status of a transcoding session
type SessionStatus string

const (
	StatusPending  SessionStatus = "pending"
	StatusRunning  SessionStatus = "running"
	StatusComplete SessionStatus = "complete"
	StatusFailed   SessionStatus = "failed"
)

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

// PlaybackPlanner interface - Core Module
type PlaybackPlanner interface {
	DecidePlayback(ctx context.Context, mediaPath string, profile DeviceProfile) (*PlaybackDecision, error)
}

// TranscodeManager interface - Core Plugin
type TranscodeManager interface {
	StartTranscode(ctx context.Context, req TranscodeRequest) (*TranscodeSession, error)
	StopTranscode(sessionID string) error
	GetSession(sessionID string) (*TranscodeSession, error)
	ListActiveSessions() []*TranscodeSession
}

// Transcoder interface - External Plugin
type Transcoder interface {
	Name() string
	Supports(codec Codec, resolution Resolution) bool
	Transcode(ctx context.Context, req TranscodeRequest) (io.ReadCloser, error)
	Priority() int // Higher priority = preferred backend
}

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