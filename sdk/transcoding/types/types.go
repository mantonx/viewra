// Package types provides common types and interfaces used across the transcoding SDK.
// This package serves as the foundation for all other packages, preventing circular
// dependencies and providing a consistent API. All shared data structures, interfaces,
// and constants are defined here to ensure type safety and maintainability.
//
// The types are organized into several categories:
// - Request/Response types for transcoding operations
// - Provider interfaces and information structures
// - Progress tracking and status types
// - Container format definitions
// - Common interfaces like Logger
//
// This centralized approach ensures:
// - No circular dependencies between packages
// - Consistent type definitions across the SDK
// - Easy refactoring and extension
// - Clear API boundaries between modules
package types

import (
	"context"
	"time"
)

// Logger interface used across all transcoding packages
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
	Debug(msg string, keysAndValues ...interface{})
}

// TranscodeRequest contains the parameters for a transcoding request
type TranscodeRequest struct {
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
	EnableABR        bool
	PreferHardware   bool          // Whether to prefer hardware acceleration
	HardwareType     HardwareType  // Specific hardware type to use
	ProviderSettings []byte        // Provider-specific settings as JSON
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

// ProcessEntry represents a registered process
type ProcessEntry struct {
	PID       int
	SessionID string
	Provider  string
	StartTime time.Time
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
	Available bool                       `json:"available"`
	Type      string                     `json:"type"`
	Encoders  map[string][]string        `json:"encoders"`  // codec -> encoder list
}

// TranscodeResult represents the result of a completed transcoding operation
type TranscodeResult struct {
	Success      bool                   `json:"success"`
	OutputPath   string                 `json:"output_path"`
	ManifestURL  string                 `json:"manifest_url,omitempty"`  // URL for streaming manifest (DASH/HLS)
	Duration     time.Duration          `json:"duration"`
	FileSize     int64                  `json:"file_size"`
	BytesWritten int64                  `json:"bytes_written"`            // Total bytes written (alias for FileSize)
	VideoInfo    *VideoInfo             `json:"video_info,omitempty"`
	AudioInfo    *AudioInfo             `json:"audio_info,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Warnings     []string               `json:"warnings,omitempty"`
	Metadata     map[string]string      `json:"metadata,omitempty"`
}

// VideoInfo contains information about video streams
type VideoInfo struct {
	Codec      string  `json:"codec"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	Bitrate    int64   `json:"bitrate"`
	FrameRate  float64 `json:"frame_rate"`
	Duration   float64 `json:"duration"`
}

// AudioInfo contains information about audio streams
type AudioInfo struct {
	Codec      string  `json:"codec"`
	Channels   int     `json:"channels"`
	SampleRate int     `json:"sample_rate"`
	Bitrate    int64   `json:"bitrate"`
	Duration   float64 `json:"duration"`
}