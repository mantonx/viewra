// Package types provides types and interfaces for the transcoding module.
// This includes all types needed for file-based transcoding operations.
// Package types defines common types and interfaces used throughout the transcoding module.
// It provides configuration structures, session information, and plugin interfaces.
package types

import (
	"context"
	"time"

	plugins "github.com/mantonx/viewra/sdk"
)

// SpeedPriority represents encoding speed vs quality tradeoff
type SpeedPriority int

const (
	SpeedPriorityBalanced SpeedPriority = iota
	SpeedPriorityQuality
	SpeedPriorityFastest
)

// Resolution represents video dimensions
type Resolution struct {
	Width  int
	Height int
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
	ProviderSettings []byte
	VideoBitrate     int
	AudioBitrate     int
}

// EncodeRequest represents a request to encode video to intermediate format
type EncodeRequest struct {
	InputPath    string
	OutputPath   string
	VideoCodec   string
	AudioCodec   string
	Quality      int
	Resolution   *Resolution
	Container    string
	VideoBitrate int
	AudioBitrate int
}

// EncodedFile represents an encoded output file
type EncodedFile struct {
	Path     string
	Profile  EncodingProfile
	Size     int64
	Duration time.Duration
}

// EncodingResult represents the result of encoding stage
type EncodingResult struct {
	OutputFiles    []EncodedFile
	ProcessingTime time.Duration
}

// PipelineResult represents the final result of the transcoding pipeline
type PipelineResult struct {
	SessionID    string
	ContentHash  string
	ManifestURL  string
	StreamURL    string
	Duration     time.Duration
	EncodedFiles []EncodedFile
	PackagedDir  string
	Metadata     PipelineMetadata
}

// PipelineMetadata contains metadata about the pipeline execution
type PipelineMetadata struct {
	ProcessingTime time.Duration
	EncodingTime   time.Duration
	PackagingTime  time.Duration
	TotalSize      int64
}

// PipelineConfig represents configuration for the pipeline
type PipelineConfig struct {
	BaseDir                string
	MaxRetries             int
	RetryDelay             time.Duration
	MaxConcurrentEncoding  int
	MaxConcurrentPackaging int
}



// ValidationError represents a validation error with field context
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// NewValidationError creates a new validation error
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

// EncodingProfile represents an encoding configuration
type EncodingProfile struct {
	Name         string `json:"name"`
	VideoCodec   string `json:"video_codec"`
	AudioCodec   string `json:"audio_codec"`
	VideoBitrate int    `json:"video_bitrate"` // in kbps
	AudioBitrate int    `json:"audio_bitrate"` // in kbps
	Width        int    `json:"width"`
	Height       int    `json:"height"`
}

// PluginManagerInterface defines the interface for plugin management
// This allows the transcoding module to interact with the plugin system
type PluginManagerInterface interface {
	// GetTranscodingProviders returns all available transcoding providers
	GetTranscodingProviders() []plugins.TranscodingProvider

	// GetTranscodingProvider returns a specific provider by ID
	GetTranscodingProvider(id string) (plugins.TranscodingProvider, error)
}

// TranscodingService defines the public interface for transcoding operations.
// This interface is registered with the service registry and used by other modules.
type TranscodingService interface {
	// StartTranscode initiates a new transcoding session
	StartTranscode(ctx context.Context, req plugins.TranscodeRequest) (*plugins.TranscodeHandle, error)

	// StopTranscode stops an active transcoding session
	StopTranscode(sessionID string) error

	// GetProgress returns the progress of a transcoding session
	GetProgress(sessionID string) (*plugins.TranscodingProgress, error)

	// GetProviders returns all available transcoding providers
	GetProviders() []plugins.ProviderInfo

	// GetSession returns details of a specific session
	GetSession(sessionID string) (*SessionInfo, error)
}

// SessionInfo represents information about a transcoding session
type SessionInfo struct {
	SessionID   string                  `json:"sessionId"`
	MediaID     string                  `json:"mediaId"`
	Provider    string                  `json:"provider"`
	Container   string                  `json:"container"`
	Status      plugins.TranscodeStatus `json:"status"`
	Progress    float64                 `json:"progress"`
	StartTime   time.Time               `json:"startTime"`
	Directory   string                  `json:"directory"`
	ContentHash string                  `json:"contentHash,omitempty"`
	Error       string                  `json:"error,omitempty"`
}

// Config holds configuration for the transcoding module
type Config struct {
	// TranscodingDir is the base directory for transcoding operations
	TranscodingDir string

	// MaxConcurrentSessions limits concurrent transcoding sessions
	MaxConcurrentSessions int

	// SessionTimeout is the maximum duration for a transcoding session
	SessionTimeout time.Duration

	// CleanupInterval is how often to run cleanup
	CleanupInterval time.Duration

	// RetentionPeriod is how long to keep completed sessions
	RetentionPeriod time.Duration
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		TranscodingDir:        "/app/viewra-data/transcoding",
		MaxConcurrentSessions: 5,
		SessionTimeout:        2 * time.Hour,
		CleanupInterval:       30 * time.Minute,
		RetentionPeriod:       24 * time.Hour,
	}
}

// PipelineStatus represents the status of the pipeline provider
type PipelineStatus struct {
	Available        bool     `json:"available"`
	ActiveJobs       int      `json:"activeJobs"`
	CompletedJobs    int      `json:"completedJobs"`
	FailedJobs       int      `json:"failedJobs"`
	FFmpegVersion    string   `json:"ffmpegVersion"`
	SupportedFormats []string `json:"supportedFormats"`
}
