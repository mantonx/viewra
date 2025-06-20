package plugins

import (
	"context"
	"io"
	"time"
)

// TranscodingProvider is the ONLY interface transcoding plugins need to implement
type TranscodingProvider interface {
	// GetInfo returns provider information
	GetInfo() ProviderInfo

	// Capabilities
	GetSupportedFormats() []ContainerFormat
	GetHardwareAccelerators() []HardwareAccelerator
	GetQualityPresets() []QualityPreset

	// File-based transcoding
	StartTranscode(ctx context.Context, req TranscodeRequest) (*TranscodeHandle, error)
	GetProgress(handle *TranscodeHandle) (*TranscodingProgress, error)
	StopTranscode(handle *TranscodeHandle) error

	// Streaming transcoding
	StartStream(ctx context.Context, req TranscodeRequest) (*StreamHandle, error)
	GetStream(handle *StreamHandle) (io.ReadCloser, error)
	StopStream(handle *StreamHandle) error

	// Dashboard integration
	GetDashboardSections() []DashboardSection
	GetDashboardData(sectionID string) (interface{}, error)
	ExecuteDashboardAction(actionID string, params map[string]interface{}) error
}

// ProviderInfo contains information about a transcoding provider
type ProviderInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Author      string `json:"author"`
	Priority    int    `json:"priority"` // Higher priority providers are preferred
}

// ContainerFormat represents a supported output format
type ContainerFormat struct {
	Format      string   `json:"format"`     // "mp4", "webm", "dash", "hls"
	MimeType    string   `json:"mime_type"`  // "video/mp4", "application/dash+xml"
	Extensions  []string `json:"extensions"` // [".mp4"], [".mpd", ".m4s"]
	Description string   `json:"description"`
	Adaptive    bool     `json:"adaptive"` // true for DASH/HLS
}

// HardwareAccelerator represents a hardware acceleration option
type HardwareAccelerator struct {
	Type        string `json:"type"` // "nvidia", "intel", "amd", "apple"
	ID          string `json:"id"`   // "nvenc", "vaapi", "qsv", "videotoolbox"
	Name        string `json:"name"` // "NVIDIA NVENC"
	Available   bool   `json:"available"`
	DeviceCount int    `json:"device_count"`
}

// QualityPreset represents a quality setting
type QualityPreset struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Quality     int    `json:"quality"`      // 0-100
	SpeedRating int    `json:"speed_rating"` // 1-10 (10 = fastest)
	SizeRating  int    `json:"size_rating"`  // 1-10 (10 = largest)
}

// TranscodeHandle represents an active transcoding operation
type TranscodeHandle struct {
	SessionID   string             `json:"session_id"`
	Provider    string             `json:"provider"`
	StartTime   time.Time          `json:"start_time"`
	Directory   string             `json:"directory"`
	ProcessID   int                `json:"process_id,omitempty"`
	Context     context.Context    `json:"-"`
	CancelFunc  context.CancelFunc `json:"-"`
	PrivateData interface{}        `json:"-"` // Provider-specific data
}

// StreamHandle represents an active streaming operation
type StreamHandle struct {
	SessionID   string             `json:"session_id"`
	Provider    string             `json:"provider"`
	StartTime   time.Time          `json:"start_time"`
	ProcessID   int                `json:"process_id,omitempty"`
	Context     context.Context    `json:"-"`
	CancelFunc  context.CancelFunc `json:"-"`
	PrivateData interface{}        `json:"-"` // Provider-specific data (e.g., FFmpeg cmd)
}
