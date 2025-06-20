package services

import (
	"context"
	"time"

	"github.com/mantonx/viewra/data/plugins/ffmpeg_transcoder/internal/types"
	"github.com/mantonx/viewra/pkg/plugins"
)

// TranscodingService handles FFmpeg transcoding operations
type TranscodingService interface {
	// StartTranscode starts a new transcoding session
	StartTranscode(ctx context.Context, req *plugins.TranscodeRequest) (*types.Session, error)

	// GetSession retrieves session information
	GetSession(sessionID string) (*types.Session, error)

	// StopSession stops a transcoding session
	StopSession(sessionID string) error

	// ListSessions returns all active sessions
	ListSessions() ([]*types.Session, error)
}

// SessionManager manages transcoding sessions
type SessionManager interface {
	// CreateSession creates a new session
	CreateSession(id string, inputPath string, container string) (*types.Session, error)

	// GetSession retrieves a session
	GetSession(sessionID string) (*types.Session, error)

	// UpdateSession updates session information
	UpdateSession(id string, update func(*types.Session) error) error

	// RemoveSession removes a session
	RemoveSession(sessionID string) error

	// ListActiveSessions returns all active sessions
	ListActiveSessions() ([]*types.Session, error)

	// ListAllSessions returns all sessions
	ListAllSessions() ([]*types.Session, error)

	// CleanupStaleSessions removes stale sessions
	CleanupStaleSessions(maxAge time.Duration) error
}

// HardwareDetector detects available hardware acceleration
type HardwareDetector interface {
	// DetectHardware detects available hardware acceleration
	DetectHardware() (*types.HardwareInfo, error)

	// GetBestEncoder returns the best encoder for given codec
	GetBestEncoder(codec string) string

	// IsEncoderAvailable checks if an encoder is available
	IsEncoderAvailable(encoder string) bool
}

// CleanupService handles file and session cleanup
type CleanupService interface {
	// CleanupExpiredSessions removes expired transcoding files
	CleanupExpiredSessions() (*types.CleanupInfo, error)

	// GetCleanupStats returns cleanup statistics
	GetCleanupStats() (*types.CleanupInfo, error)

	// CleanupSession removes a specific session's files
	CleanupSession(sessionID string) error
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
	InputFile   string            `json:"input_file"`
	OutputFile  string            `json:"output_file"`
	Settings    JobSettings       `json:"settings"`
	Priority    int               `json:"priority,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
}

type TranscodingResponse struct {
	JobID     string            `json:"job_id"`
	Status    TranscodingStatus `json:"status"`
	Message   string            `json:"message,omitempty"`
	QueueSize int               `json:"queue_size,omitempty"`
}

type TranscodingJob struct {
	ID         string              `json:"id"`
	Status     TranscodingStatus   `json:"status"`
	InputFile  string              `json:"input_file"`
	OutputFile string              `json:"output_file"`
	Settings   JobSettings         `json:"settings"`
	Progress   Progress            `json:"progress"`
	StartTime  time.Time           `json:"start_time"`
	EndTime    *time.Time          `json:"end_time,omitempty"`
	Error      string              `json:"error,omitempty"`
	CancelFunc context.CancelFunc  `json:"-"`                 // Function to cancel the job context
	Request    *TranscodingRequest `json:"request,omitempty"` // Store the original request
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
