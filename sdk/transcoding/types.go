// Package transcoding provides the plugin SDK for Viewra transcoding providers.
// This is a minimal interface-only SDK that plugins should implement.
//
// Plugins should implement the TranscodingProvider interface to provide
// custom transcoding functionality. The actual implementation logic
// is handled by the Viewra transcoding module.
package transcoding

import (
	"context"
	"io"
	"time"
)

// Logger interface for plugin logging
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// TranscodeRequest contains the parameters for a transcoding request
type TranscodeRequest struct {
	MediaID          string
	SessionID        string
	InputPath        string
	OutputPath       string
	Container        string
	VideoCodec       string
	AudioCodec       string
	Resolution       *Resolution
	Quality          int
	SpeedPriority    SpeedPriority
	Seek             time.Duration
	Duration         time.Duration
	EnableABR        bool
	PreferHardware   bool
	HardwareType     HardwareType
	ProviderSettings []byte
	VideoBitrate     int
	AudioBitrate     int
}

// Resolution represents video dimensions
type Resolution struct {
	Width  int
	Height int
}

// SpeedPriority represents encoding speed vs quality tradeoff
type SpeedPriority int

const (
	SpeedPriorityBalanced SpeedPriority = iota
	SpeedPriorityQuality
	SpeedPriorityFastest
)

// TranscodeStatus represents the status of a transcoding operation
type TranscodeStatus string

const (
	TranscodeStatusStarting  TranscodeStatus = "starting"
	TranscodeStatusRunning   TranscodeStatus = "running"
	TranscodeStatusCompleted TranscodeStatus = "completed"
	TranscodeStatusFailed    TranscodeStatus = "failed"
	TranscodeStatusCancelled TranscodeStatus = "cancelled"
)

// TranscodeHandle represents a handle to a transcoding session
type TranscodeHandle struct {
	SessionID   string
	Provider    string
	StartTime   time.Time
	Directory   string
	Context     context.Context
	CancelFunc  context.CancelFunc
	PrivateData interface{}
	Status      TranscodeStatus
	Error       string
}

// TranscodingProgress represents the progress of a transcoding operation
type TranscodingProgress struct {
	PercentComplete float64
	TimeElapsed     time.Duration
	TimeRemaining   time.Duration
	CurrentSpeed    float64
	AverageSpeed    float64
	EstimatedTime   time.Duration
	CurrentFrame    int64
	TotalFrames     int64
	BytesRead       int64
	BytesWritten    int64
}

// ProviderInfo contains information about a transcoding provider
type ProviderInfo struct {
	ID          string
	Name        string
	Description string
	Version     string
	Author      string
	Priority    int
}

// ContainerFormat represents a supported container format
type ContainerFormat struct {
	Format      string
	MimeType    string
	Extensions  []string
	Description string
	Adaptive    bool
}

// StreamHandle represents a handle to a streaming session
type StreamHandle struct {
	SessionID   string
	Provider    string
	StartTime   time.Time
	Context     context.Context
	CancelFunc  context.CancelFunc
	PrivateData interface{}
	Status      TranscodeStatus
	Error       string
}

// DashboardSection represents a section in the provider dashboard
type DashboardSection struct {
	ID          string
	Title       string
	Type        string
	Description string
}

// HardwareType represents a type of hardware acceleration
type HardwareType string

const (
	HardwareTypeNone        HardwareType = "none"
	HardwareTypeNVIDIA      HardwareType = "nvidia"
	HardwareTypeVAAPI       HardwareType = "vaapi"
	HardwareTypeQSV         HardwareType = "qsv"
	HardwareTypeVideoToolbox HardwareType = "videotoolbox"
)

// HardwareInfo contains information about available hardware acceleration
type HardwareInfo struct {
	Available bool                `json:"available"`
	Type      string              `json:"type"`
	Encoders  map[string][]string `json:"encoders"`
}

// VideoInfo contains information about video streams
type VideoInfo struct {
	Codec     string  `json:"codec"`
	Width     int     `json:"width"`
	Height    int     `json:"height"`
	Bitrate   int64   `json:"bitrate"`
	FrameRate float64 `json:"frame_rate"`
	Duration  float64 `json:"duration"`
}

// AudioInfo contains information about audio streams
type AudioInfo struct {
	Codec      string  `json:"codec"`
	Channels   int     `json:"channels"`
	SampleRate int     `json:"sample_rate"`
	Bitrate    int64   `json:"bitrate"`
	Duration   float64 `json:"duration"`
}

// TranscodingProvider interface defines the contract for transcoding providers.
// This is the main interface that all transcoding providers must implement.
type TranscodingProvider interface {
	// GetInfo returns provider information
	GetInfo() ProviderInfo

	// GetSupportedFormats returns supported container formats
	GetSupportedFormats() []ContainerFormat

	// StartTranscode starts a new transcoding operation
	StartTranscode(ctx context.Context, req TranscodeRequest) (*TranscodeHandle, error)

	// GetProgress returns the progress of a transcoding operation
	GetProgress(handle *TranscodeHandle) (*TranscodingProgress, error)

	// StopTranscode stops a transcoding operation
	StopTranscode(handle *TranscodeHandle) error

	// StartStream starts a streaming session (for DASH/HLS)
	StartStream(ctx context.Context, req TranscodeRequest) (*StreamHandle, error)

	// GetStream returns a stream reader
	GetStream(handle *StreamHandle) (io.ReadCloser, error)

	// StopStream stops a streaming session
	StopStream(handle *StreamHandle) error

	// GetDashboardSections returns dashboard sections for this provider
	GetDashboardSections() []DashboardSection

	// GetDashboardData returns data for a dashboard section
	GetDashboardData(sectionID string) (interface{}, error)

	// ExecuteDashboardAction executes a dashboard action
	ExecuteDashboardAction(actionID string, params map[string]interface{}) error
}