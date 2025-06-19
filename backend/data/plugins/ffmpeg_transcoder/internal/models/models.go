package models

import (
	"time"
)

// TranscodingJob represents a single transcoding job/session
type TranscodingJob struct {
	ID            string            `json:"id"`
	Status        TranscodingStatus `json:"status"`
	InputFile     string            `json:"input_file"`
	OutputFile    string            `json:"output_file"`
	Settings      JobSettings       `json:"settings"`
	Progress      Progress          `json:"progress"`
	StartTime     time.Time         `json:"start_time"`
	EndTime       *time.Time        `json:"end_time,omitempty"`
	Error         string            `json:"error,omitempty"`
	FFmpegCommand string            `json:"ffmpeg_command,omitempty"`
	Metrics       JobMetrics        `json:"metrics"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// TranscodingStatus represents the current status of a transcoding job
type TranscodingStatus string

const (
	StatusPending    TranscodingStatus = "pending"
	StatusQueued     TranscodingStatus = "queued"
	StatusProcessing TranscodingStatus = "processing"
	StatusCompleted  TranscodingStatus = "completed"
	StatusFailed     TranscodingStatus = "failed"
	StatusCancelled  TranscodingStatus = "cancelled"
	StatusTimeout    TranscodingStatus = "timeout"
)

// JobSettings contains the transcoding settings for a specific job
type JobSettings struct {
	VideoCodec            string            `json:"video_codec"`
	AudioCodec            string            `json:"audio_codec"`
	Container             string            `json:"container"`
	Quality               int               `json:"quality"` // CRF value
	Preset                string            `json:"preset"`  // Encoding preset
	Resolution            *Resolution       `json:"resolution,omitempty"`
	AudioBitrate          int               `json:"audio_bitrate"`                     // kbps
	AudioChannels         int               `json:"audio_channels,omitempty"`          // Number of audio channels
	PreserveSurroundSound bool              `json:"preserve_surround_sound,omitempty"` // Whether to preserve surround sound
	Filters               []string          `json:"filters,omitempty"`
	Custom                map[string]string `json:"custom,omitempty"` // Custom FFmpeg options
}

// Resolution represents video resolution
type Resolution struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Progress represents the current progress of a transcoding job
type Progress struct {
	Percentage    float64       `json:"percentage"` // 0.0 to 100.0
	FramesTotal   int64         `json:"frames_total"`
	FramesCurrent int64         `json:"frames_current"`
	TimeTotal     time.Duration `json:"time_total"`   // Total duration of input
	TimeCurrent   time.Duration `json:"time_current"` // Current position
	Speed         float64       `json:"speed"`        // Processing speed multiplier
	Bitrate       string        `json:"bitrate"`      // Current bitrate
	LastUpdate    time.Time     `json:"last_update"`
}

// JobMetrics contains performance metrics for a transcoding job
type JobMetrics struct {
	InputFileSize    int64         `json:"input_file_size"`   // Bytes
	OutputFileSize   int64         `json:"output_file_size"`  // Bytes
	ProcessingTime   time.Duration `json:"processing_time"`   // Total processing time
	AverageSpeed     float64       `json:"average_speed"`     // Average processing speed
	PeakCPUUsage     float64       `json:"peak_cpu_usage"`    // Peak CPU usage percentage
	PeakMemoryUsage  int64         `json:"peak_memory_usage"` // Peak memory usage in bytes
	CompressionRatio float64       `json:"compression_ratio"` // Output size / input size
	Quality          float64       `json:"quality,omitempty"` // Quality metric if available
}

// SessionInfo represents information about an active transcoding session
type SessionInfo struct {
	ID            string            `json:"id"`
	Status        TranscodingStatus `json:"status"`
	InputFile     string            `json:"input_file"`
	OutputFile    string            `json:"output_file"`
	Progress      Progress          `json:"progress"`
	StartTime     time.Time         `json:"start_time"`
	EstimatedEnd  *time.Time        `json:"estimated_end,omitempty"`
	CurrentCPU    float64           `json:"current_cpu"`
	CurrentMemory int64             `json:"current_memory"`
}

// TranscodingCapabilities represents the capabilities of the transcoder
type TranscodingCapabilities struct {
	SupportedFormats     []string            `json:"supported_formats"`
	SupportedCodecs      map[string][]string `json:"supported_codecs"` // format -> codecs
	MaxConcurrentJobs    int                 `json:"max_concurrent_jobs"`
	HardwareAcceleration []string            `json:"hardware_acceleration,omitempty"`
	Features             []string            `json:"features"`
}

// FormatInfo represents information about a media format
type FormatInfo struct {
	Container    string            `json:"container"`
	VideoCodec   string            `json:"video_codec,omitempty"`
	AudioCodec   string            `json:"audio_codec,omitempty"`
	Duration     time.Duration     `json:"duration"`
	Resolution   *Resolution       `json:"resolution,omitempty"`
	Bitrate      int               `json:"bitrate"` // Total bitrate in kbps
	AudioBitrate int               `json:"audio_bitrate,omitempty"`
	Framerate    float64           `json:"framerate,omitempty"`
	FileSize     int64             `json:"file_size"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// TranscodingRequest represents a request to start a new transcoding job
type TranscodingRequest struct {
	InputFile   string            `json:"input_file"`
	OutputFile  string            `json:"output_file"`
	Settings    JobSettings       `json:"settings"`
	Priority    int               `json:"priority,omitempty"` // 1-10, higher = more priority
	Metadata    map[string]string `json:"metadata,omitempty"`
	CallbackURL string            `json:"callback_url,omitempty"`
}

// TranscodingResponse represents the response from starting a transcoding job
type TranscodingResponse struct {
	JobID     string            `json:"job_id"`
	Status    TranscodingStatus `json:"status"`
	Message   string            `json:"message,omitempty"`
	QueueSize int               `json:"queue_size,omitempty"`
}

// SystemStats represents current system statistics
type SystemStats struct {
	ActiveJobs    int        `json:"active_jobs"`
	QueuedJobs    int        `json:"queued_jobs"`
	CompletedJobs int        `json:"completed_jobs"`
	FailedJobs    int        `json:"failed_jobs"`
	TotalJobs     int        `json:"total_jobs"`
	AverageCPU    float64    `json:"average_cpu"`
	AverageMemory int64      `json:"average_memory"`
	UptimeSeconds int64      `json:"uptime_seconds"`
	LastJobTime   *time.Time `json:"last_job_time,omitempty"`
}

// IsCompleted returns true if the job has finished (success or failure)
func (j *TranscodingJob) IsCompleted() bool {
	return j.Status == StatusCompleted || j.Status == StatusFailed ||
		j.Status == StatusCancelled || j.Status == StatusTimeout
}

// IsActive returns true if the job is currently running
func (j *TranscodingJob) IsActive() bool {
	return j.Status == StatusProcessing
}

// Duration returns the total duration of the job (if completed)
func (j *TranscodingJob) Duration() time.Duration {
	if j.EndTime != nil {
		return j.EndTime.Sub(j.StartTime)
	}
	if j.IsActive() {
		return time.Since(j.StartTime)
	}
	return 0
}

// EstimatedTimeRemaining estimates how much time is left based on current progress
func (p *Progress) EstimatedTimeRemaining() time.Duration {
	if p.Percentage <= 0 || p.Speed <= 0 {
		return 0
	}

	remaining := p.TimeTotal - p.TimeCurrent
	if remaining <= 0 {
		return 0
	}

	return time.Duration(float64(remaining) / p.Speed)
}

// Update updates the progress with new values
func (p *Progress) Update(framesCurrent int64, timeCurrent time.Duration, speed float64, bitrate string) {
	p.FramesCurrent = framesCurrent
	p.TimeCurrent = timeCurrent
	p.Speed = speed
	p.Bitrate = bitrate
	p.LastUpdate = time.Now()

	// Calculate percentage
	if p.FramesTotal > 0 {
		p.Percentage = float64(framesCurrent) / float64(p.FramesTotal) * 100.0
	} else if p.TimeTotal > 0 {
		p.Percentage = float64(timeCurrent) / float64(p.TimeTotal) * 100.0
	}

	// Ensure percentage doesn't exceed 100%
	if p.Percentage > 100.0 {
		p.Percentage = 100.0
	}
}
