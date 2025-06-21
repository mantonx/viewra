package transcoding

import (
	"context"
	"encoding/json"
	"time"
)

// ========================================================================
// Core Transcoding Types - Clean, No Legacy
// ========================================================================

// Logger interface for transcoding operations
type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

// SpeedPriority represents encoding speed/quality tradeoff
type SpeedPriority string

const (
	SpeedPriorityFastest  SpeedPriority = "fastest"  // Max speed, lower quality
	SpeedPriorityBalanced SpeedPriority = "balanced" // Balanced
	SpeedPriorityQuality  SpeedPriority = "quality"  // Max quality, slower
)

// HardwareType identifies hardware acceleration types
type HardwareType string

const (
	HardwareTypeNone         HardwareType = "none"         // CPU only
	HardwareTypeCUDA         HardwareType = "cuda"         // NVIDIA
	HardwareTypeVAAPI        HardwareType = "vaapi"        // Intel/AMD Linux
	HardwareTypeQSV          HardwareType = "qsv"          // Intel Quick Sync
	HardwareTypeVideoToolbox HardwareType = "videotoolbox" // macOS
	HardwareTypeAMF          HardwareType = "amf"          // AMD Windows
)

// VideoResolution represents video resolution
type VideoResolution struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Common resolutions
var (
	Resolution480p  = VideoResolution{854, 480}
	Resolution720p  = VideoResolution{1280, 720}
	Resolution1080p = VideoResolution{1920, 1080}
	Resolution1440p = VideoResolution{2560, 1440}
	Resolution4K    = VideoResolution{3840, 2160}
	Resolution8K    = VideoResolution{7680, 4320}
)

// HardwareInfo represents detected hardware acceleration capabilities
type HardwareInfo struct {
	Type        string              // nvenc, vaapi, qsv, videotoolbox
	Available   bool
	Encoders    map[string][]string // Map of codec to available hardware encoders
	MaxSessions int                 // Maximum concurrent sessions
}

// TranscodingProgress represents standardized progress information
type TranscodingProgress struct {
	// Core progress (required)
	PercentComplete float64       `json:"percent_complete"` // 0-100
	TimeElapsed     time.Duration `json:"time_elapsed"`
	TimeRemaining   time.Duration `json:"time_remaining"`

	// Data metrics
	BytesRead    int64 `json:"bytes_read"`
	BytesWritten int64 `json:"bytes_written"`

	// Performance
	CurrentSpeed float64 `json:"current_speed"` // 1.0 = realtime
	AverageSpeed float64 `json:"average_speed"`

	// Resource usage (optional)
	CPUPercent  float64 `json:"cpu_percent,omitempty"`
	MemoryBytes int64   `json:"memory_bytes,omitempty"`
	GPUPercent  float64 `json:"gpu_percent,omitempty"`
}

// TranscodeRequest represents a transcoding request
type TranscodeRequest struct {
	// Core fields
	InputPath  string `json:"input_path"`
	OutputPath string `json:"output_path"`
	SessionID  string `json:"session_id"`

	// Generic quality/speed settings
	Quality       int           `json:"quality"`        // 0-100
	SpeedPriority SpeedPriority `json:"speed_priority"` // "fastest", "balanced", "quality"

	// Format settings
	Container  string `json:"container"`   // mp4, mkv, dash, hls
	VideoCodec string `json:"video_codec"` // h264, h265, vp9
	AudioCodec string `json:"audio_codec"` // aac, opus, mp3

	// Optional transforms
	Resolution *VideoResolution `json:"resolution,omitempty"` // nil = keep original
	FrameRate  *float64         `json:"frame_rate,omitempty"` // nil = keep original
	Seek       time.Duration    `json:"seek,omitempty"`       // Start position
	Duration   time.Duration    `json:"duration,omitempty"`   // Encode duration

	// Hardware preferences
	PreferHardware bool         `json:"prefer_hardware"`
	HardwareType   HardwareType `json:"hardware_type,omitempty"`

	// Adaptive Bitrate Streaming
	EnableABR bool `json:"enable_abr,omitempty"` // Enable multi-bitrate encoding

	// Provider-specific overrides
	ProviderSettings json.RawMessage `json:"provider_settings,omitempty"`
}

// MarshalJSON implements custom JSON marshaling to handle time.Duration
func (r TranscodeRequest) MarshalJSON() ([]byte, error) {
	type Alias TranscodeRequest
	return json.Marshal(&struct {
		Seek     int64 `json:"seek,omitempty"`     // Duration as nanoseconds
		Duration int64 `json:"duration,omitempty"` // Duration as nanoseconds
		*Alias
	}{
		Seek:     int64(r.Seek),
		Duration: int64(r.Duration),
		Alias:    (*Alias)(&r),
	})
}

// UnmarshalJSON implements custom JSON unmarshaling to handle time.Duration
func (r *TranscodeRequest) UnmarshalJSON(data []byte) error {
	type Alias TranscodeRequest
	aux := &struct {
		Seek     int64 `json:"seek,omitempty"`
		Duration int64 `json:"duration,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	r.Seek = time.Duration(aux.Seek)
	r.Duration = time.Duration(aux.Duration)
	return nil
}

// TranscodeResult contains the result of a transcoding operation
type TranscodeResult struct {
	Success      bool   `json:"success"`
	OutputPath   string `json:"output_path,omitempty"`
	ManifestURL  string `json:"manifest_url,omitempty"`
	BytesWritten int64  `json:"bytes_written,omitempty"`
	Duration     int64  `json:"duration,omitempty"` // Duration in seconds
	Error        string `json:"error,omitempty"`
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

// QualityMapper maps generic quality settings to codec-specific parameters
type QualityMapper interface {
	// MapQuality converts 0-100 quality percentage to codec-specific parameters
	MapQuality(percent int, codec string) map[string]interface{}
	
	// GetSpeedPreset maps generic speed priority to codec-specific preset
	GetSpeedPreset(priority SpeedPriority) string
	
	// GetDefaultQuality returns the default quality percentage for a codec
	GetDefaultQuality(codec string) int
}

// ContainerFormat represents a supported container format
type ContainerFormat struct {
	Format      string   `json:"format"`      // "mp4", "webm", "dash", etc.
	MimeType    string   `json:"mime_type"`   // MIME type
	Extensions  []string `json:"extensions"`  // File extensions
	Description string   `json:"description"` // Human-readable description
	Adaptive    bool     `json:"adaptive"`    // Whether it's adaptive streaming
}

// TranscodeHandle represents a handle to control an active transcoding session
type TranscodeHandle struct {
	SessionID   string               `json:"session_id"`
	Status      TranscodeStatus      `json:"status"`
	Progress    *TranscodingProgress `json:"progress,omitempty"`
	Error       string               `json:"error,omitempty"`
	Provider    string               `json:"provider"`     // Plugin name that created the session
	StartTime   time.Time            `json:"start_time"`
	Directory   string               `json:"directory"`    // Output directory
	Context     context.Context      `json:"-"`            // Context for cancellation
	CancelFunc  context.CancelFunc   `json:"-"`            // Cancel function
	PrivateData interface{}          `json:"-"`            // Private plugin data
}

// ProviderInfo represents information about a transcoding provider
type ProviderInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Author      string   `json:"author"`
	Priority    int      `json:"priority"`
	Capabilities []string `json:"capabilities"`
}

// StreamHandle represents a handle to a streaming transcoding session
type StreamHandle struct {
	SessionID   string            `json:"session_id"`
	Status      TranscodeStatus   `json:"status"`
	Error       string            `json:"error,omitempty"`
	Context     context.Context   `json:"-"`
	CancelFunc  context.CancelFunc `json:"-"`
	PrivateData interface{}       `json:"-"`
}

// DashboardSection represents a section in the plugin dashboard
type DashboardSection struct {
	ID      string      `json:"id"`
	Title   string      `json:"title"`
	Type    string      `json:"type"`
	Content interface{} `json:"content"`
	Order   int         `json:"order"`
}
