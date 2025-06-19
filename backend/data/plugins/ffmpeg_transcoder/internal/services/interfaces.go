package services

import (
	"context"
	"time"
)

// TranscodingService defines the core transcoding functionality
type TranscodingService interface {
	// Start a new transcoding job
	StartJob(ctx context.Context, request *TranscodingRequest) (*TranscodingResponse, error)

	// Stop a running transcoding job
	StopJob(ctx context.Context, jobID string) error

	// Get job status and progress
	GetJobStatus(ctx context.Context, jobID string) (*TranscodingJob, error)

	// Get system statistics
	GetSystemStats(ctx context.Context) (*SystemStats, error)

	// Clean up old/completed jobs
	CleanupJobs(ctx context.Context, olderThan int) (int, error)
}

// FFmpegExecutor defines the interface for executing FFmpeg commands
type FFmpegExecutor interface {
	// Execute FFmpeg command with progress monitoring
	Execute(ctx context.Context, args []string, progressCallback ProgressCallback) error

	// Get FFmpeg version and capabilities
	GetVersion(ctx context.Context) (string, error)

	// Probe media file for format information
	ProbeFile(ctx context.Context, filename string) (*FormatInfo, error)

	// Validate FFmpeg installation and codecs
	ValidateInstallation(ctx context.Context) error
}

// ProgressCallback is called when progress updates are available
type ProgressCallback func(jobID string, progress *Progress)

// Basic types used by the interfaces
type TranscodingRequest struct {
	InputFile  string            `json:"input_file"`
	OutputFile string            `json:"output_file"`
	Settings   JobSettings       `json:"settings"`
	Priority   int               `json:"priority,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type TranscodingResponse struct {
	JobID     string            `json:"job_id"`
	Status    TranscodingStatus `json:"status"`
	Message   string            `json:"message,omitempty"`
	QueueSize int               `json:"queue_size,omitempty"`
}

type TranscodingJob struct {
	ID         string             `json:"id"`
	Status     TranscodingStatus  `json:"status"`
	InputFile  string             `json:"input_file"`
	OutputFile string             `json:"output_file"`
	Settings   JobSettings        `json:"settings"`
	Progress   Progress           `json:"progress"`
	StartTime  time.Time          `json:"start_time"`
	EndTime    *time.Time         `json:"end_time,omitempty"`
	Error      string             `json:"error,omitempty"`
	CancelFunc context.CancelFunc `json:"-"` // Function to cancel the job context
}

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

type JobSettings struct {
	VideoCodec   string `json:"video_codec"`
	AudioCodec   string `json:"audio_codec"`
	Container    string `json:"container"`
	Quality      int    `json:"quality"`       // CRF value
	Preset       string `json:"preset"`        // Encoding preset
	AudioBitrate int    `json:"audio_bitrate"` // kbps
}

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

type FormatInfo struct {
	Container  string            `json:"container"`
	VideoCodec string            `json:"video_codec,omitempty"`
	AudioCodec string            `json:"audio_codec,omitempty"`
	Duration   time.Duration     `json:"duration"`
	Bitrate    int               `json:"bitrate"` // Total bitrate in kbps
	FileSize   int64             `json:"file_size"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type SystemStats struct {
	ActiveJobs    int     `json:"active_jobs"`
	QueuedJobs    int     `json:"queued_jobs"`
	CompletedJobs int     `json:"completed_jobs"`
	FailedJobs    int     `json:"failed_jobs"`
	TotalJobs     int     `json:"total_jobs"`
	AverageCPU    float64 `json:"average_cpu"`
	AverageMemory int64   `json:"average_memory"`
	UptimeSeconds int64   `json:"uptime_seconds"`
}
