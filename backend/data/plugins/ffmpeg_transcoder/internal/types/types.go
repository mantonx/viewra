package types

import (
	"context"
	"time"
)

// Session represents an active transcoding session
type Session struct {
	ID         string
	InputPath  string
	OutputPath string
	Container  string
	Status     SessionStatus
	Progress   float64
	StartTime  time.Time
	UpdatedAt  time.Time
	Context    context.Context
	Cancel     context.CancelFunc
	SessionDir string
	ProcessPID int
}

// SessionStatus represents the status of a transcoding session
type SessionStatus string

const (
	StatusPending   SessionStatus = "pending"
	StatusStarting  SessionStatus = "starting"
	StatusRunning   SessionStatus = "running"
	StatusCompleted SessionStatus = "completed"
	StatusFailed    SessionStatus = "failed"
	StatusCancelled SessionStatus = "cancelled"
	StatusTimeout   SessionStatus = "timeout"
)

// FFmpegProgress represents progress information from FFmpeg
type FFmpegProgress struct {
	Frame    int64
	FPS      float64
	Size     int64
	Time     time.Duration
	Bitrate  string
	Speed    float64
	Progress float64
}

// TranscodeOptions represents options for a transcoding operation
type TranscodeOptions struct {
	VideoCodec    string
	VideoPreset   string
	VideoCRF      int
	AudioCodec    string
	AudioBitrate  int
	AudioChannels int
	Container     string
	StartTime     time.Duration
	Duration      time.Duration
	Resolution    string
	TwoPass       bool
	HWAccel       string
}

// HardwareInfo represents detected hardware acceleration capabilities
type HardwareInfo struct {
	Type        string // nvenc, vaapi, qsv, videotoolbox
	Available   bool
	Encoders    map[string][]string // Map of codec to available hardware encoders
	MaxSessions int                 // Maximum concurrent sessions
}

// SessionStats represents statistics for a transcoding session
type SessionStats struct {
	TotalFrames     int64
	ProcessedFrames int64
	BytesProcessed  int64
	BytesGenerated  int64
	AverageFPS      float64
	CurrentFPS      float64
	CPUUsage        float64
	MemoryUsage     int64
	StartTime       time.Time
	Duration        time.Duration
}

// CleanupInfo represents information about cleanup operations
type CleanupInfo struct {
	TotalDirectories   int
	TotalSize          int64
	DirectoriesRemoved int
	SizeFreed          int64
	LastCleanup        time.Time
	NextCleanup        time.Time
}
