package plugins

import (
	"encoding/json"
	"time"
)

// ========================================================================
// Core Transcoding Types - Clean, No Legacy
// ========================================================================

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

	// Provider-specific overrides
	ProviderSettings json.RawMessage `json:"provider_settings,omitempty"`
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

// HardwareInfo represents hardware acceleration information
type HardwareInfo struct {
	Type       string `json:"type"`        // "nvidia", "intel", "amd", "apple", etc.
	Device     string `json:"device"`      // Device identifier
	DeviceName string `json:"device_name"` // Human-readable name
	Available  bool   `json:"available"`   // Whether hardware is available
	InUse      bool   `json:"in_use"`      // Whether currently in use
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
