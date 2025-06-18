package models

import (
	"time"

	"gorm.io/gorm"
)

// TranscodeSession represents a transcoding session in the main database
// Generic for all transcoding backends (FFmpeg, VAAPI, QSV, NVENC, etc.)
type TranscodeSession struct {
	ID         string     `gorm:"primaryKey" json:"id"`
	PluginID   string     `gorm:"not null;index;default:'ffmpeg_transcoder'" json:"plugin_id"` // Scope sessions by plugin
	Backend    string     `gorm:"not null;index" json:"backend"`                               // ffmpeg, vaapi, qsv, nvenc, etc.
	InputPath  string     `gorm:"not null;index" json:"input_path"`                            // Add index for duplicate detection
	OutputPath string     `json:"output_path"`
	Status     string     `gorm:"not null;default:'pending';index" json:"status"` // Add index for status queries
	Progress   float64    `gorm:"default:0" json:"progress"`
	StartTime  time.Time  `gorm:"not null;index" json:"start_time"` // Add index for cleanup queries
	EndTime    *time.Time `json:"end_time,omitempty"`

	// Transcoding parameters - generic for all backends
	TargetCodec     string `json:"target_codec,omitempty"`     // h264, hevc, av1, etc.
	TargetContainer string `json:"target_container,omitempty"` // mp4, webm, mkv, etc.
	Resolution      string `json:"resolution,omitempty"`       // 1080p, 720p, etc.
	Bitrate         int    `json:"bitrate,omitempty"`          // Target bitrate in kbps
	AudioCodec      string `json:"audio_codec,omitempty"`      // aac, opus, etc.
	AudioBitrate    int    `json:"audio_bitrate,omitempty"`    // Audio bitrate in kbps
	Quality         int    `json:"quality,omitempty"`          // Quality setting (0-51 for x264/x265)
	Preset          string `json:"preset,omitempty"`           // Encoding preset (fast, medium, slow, etc.)

	// Hardware acceleration settings
	HWAccel       string `json:"hw_accel,omitempty"`        // vaapi, qsv, nvenc, etc.
	HWAccelDevice string `json:"hw_accel_device,omitempty"` // GPU device path/ID

	// Session metadata
	Error     string `gorm:"type:text" json:"error,omitempty"`
	ClientIP  string `json:"client_ip,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
	Metadata  string `gorm:"type:text" json:"metadata,omitempty"` // JSON for additional backend-specific data

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName returns the table name for the TranscodeSession model
func (TranscodeSession) TableName() string {
	return "transcode_sessions"
}

// TranscodeStats represents transcoding statistics in the main database
// Generic for all transcoding backends
type TranscodeStats struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	SessionID string `gorm:"not null;index" json:"session_id"`                            // FK to transcode_sessions
	PluginID  string `gorm:"not null;index;default:'ffmpeg_transcoder'" json:"plugin_id"` // Scope stats by plugin
	Backend   string `gorm:"not null;index" json:"backend"`                               // ffmpeg, vaapi, qsv, nvenc, etc.

	// Performance metrics - generic for all backends
	Duration        int64   `json:"duration"`         // Duration in milliseconds
	BytesProcessed  int64   `json:"bytes_processed"`  // Input bytes processed
	BytesGenerated  int64   `json:"bytes_generated"`  // Output bytes generated
	FramesProcessed int64   `json:"frames_processed"` // Video frames processed
	CurrentFPS      float64 `json:"current_fps"`      // Current processing FPS
	AverageFPS      float64 `json:"average_fps"`      // Average processing FPS
	Speed           float64 `json:"speed"`            // Processing speed multiplier

	// System resource usage
	CPUUsage    float64 `json:"cpu_usage"`    // CPU usage percentage
	MemoryUsage int64   `json:"memory_usage"` // Memory usage in bytes
	GPUUsage    float64 `json:"gpu_usage"`    // GPU usage percentage (for HW accel)
	GPUMemory   int64   `json:"gpu_memory"`   // GPU memory usage in bytes

	RecordedAt time.Time `gorm:"not null;index" json:"recorded_at"` // Add index for cleanup queries
}

// TableName returns the table name for the TranscodeStats model
func (TranscodeStats) TableName() string {
	return "transcode_stats"
}
