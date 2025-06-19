package plugins

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ========================================================================
// Core Transcoder Interface - Clean SDK Design
// ========================================================================

// Transcoder interface defines the core transcoding functionality
// that all transcoding plugins must implement
type Transcoder interface {
	// Name returns the name of this transcoder (e.g., "FFmpeg Software", "NVENC Hardware")
	Name() string

	// Transcode initiates a transcoding operation and returns the result
	Transcode(ctx context.Context, req *TranscodeRequest) (*TranscodeResult, error)

	// Supports determines if this transcoder can handle the given device profile
	Supports(profile *DeviceProfile) bool
}

// TranscodeRequest represents a transcoding request
type TranscodeRequest struct {
	InputPath     string            `json:"input_path"`
	OutputPath    string            `json:"output_path"`
	Seek          time.Duration     `json:"seek"`
	Duration      time.Duration     `json:"duration"`
	CodecOpts     *CodecOptions     `json:"codec_opts"`
	DeviceProfile *DeviceProfile    `json:"device_profile"`
	SessionID     string            `json:"session_id"`
	Environment   map[string]string `json:"environment,omitempty"`
}

// TranscodeResult contains the result of a transcoding operation
type TranscodeResult struct {
	SegmentPath string        `json:"segment_path"`
	Duration    time.Duration `json:"duration"`
	SessionID   string        `json:"session_id"`
	OutputURL   string        `json:"output_url"`
	ManifestURL string        `json:"manifest_url,omitempty"` // For DASH/HLS
	Cleanup     func() error  `json:"-"`                      // Cleanup function
}

// CodecOptions defines codec and container configuration
type CodecOptions struct {
	Video     string   `json:"video"`             // Video codec (h264, h265, etc.)
	Audio     string   `json:"audio"`             // Audio codec (aac, ac3, etc.)
	Container string   `json:"container"`         // Container format (mp4, dash, hls)
	Bitrate   string   `json:"bitrate"`           // Video bitrate (e.g., "1000k")
	Extra     []string `json:"extra,omitempty"`   // Additional FFmpeg arguments
	Quality   int      `json:"quality,omitempty"` // CRF/CQ value
	Preset    string   `json:"preset,omitempty"`  // Encoding preset
}

// DeviceProfile captures client capabilities
type DeviceProfile struct {
	UserAgent               string   `json:"user_agent"`
	SupportedCodecs         []string `json:"supported_codecs"`
	MaxResolution           string   `json:"max_resolution"`
	MaxBitrate              int      `json:"max_bitrate"`
	SupportsHEVC            bool     `json:"supports_hevc"`
	SupportsAV1             bool     `json:"supports_av1"`
	SupportsHDR             bool     `json:"supports_hdr"`
	AllowsSoftwareTranscode bool     `json:"allows_software_transcode"`
	SupportsHardwareAccel   bool     `json:"supports_hardware_accel"`
	PreferredCodecs         []string `json:"preferred_codecs"`
	ClientIP                string   `json:"client_ip"`
	Platform                string   `json:"platform"`
	Browser                 string   `json:"browser"`
}

// CommandRunner interface for command execution (enables mocking in tests)
type CommandRunner interface {
	Run(ctx context.Context, cmd string, args ...string) ([]byte, error)
}

// TranscoderRegistry manages multiple transcoding implementations
type TranscoderRegistry struct {
	transcoders []Transcoder
}

// NewTranscoderRegistry creates a new transcoder registry
func NewTranscoderRegistry() *TranscoderRegistry {
	return &TranscoderRegistry{
		transcoders: make([]Transcoder, 0),
	}
}

// Register adds a transcoder to the registry
func (r *TranscoderRegistry) Register(transcoder Transcoder) {
	r.transcoders = append(r.transcoders, transcoder)
}

// GetBestTranscoder returns the best transcoder for the given device profile
func (r *TranscoderRegistry) GetBestTranscoder(profile *DeviceProfile) Transcoder {
	for _, transcoder := range r.transcoders {
		if transcoder.Supports(profile) {
			return transcoder
		}
	}
	return nil // No suitable transcoder found
}

// ListTranscoders returns all registered transcoders
func (r *TranscoderRegistry) ListTranscoders() []Transcoder {
	return r.transcoders
}

// Global registry instance
var GlobalTranscoderRegistry = NewTranscoderRegistry()

// RegisterTranscoder is a convenience function to register a transcoder globally
func RegisterTranscoder(transcoder Transcoder) {
	GlobalTranscoderRegistry.Register(transcoder)
}

// GetTranscoder retrieves a registered transcoder by name
func GetTranscoder(name string) (Transcoder, bool) {
	for _, transcoder := range GlobalTranscoderRegistry.transcoders {
		if transcoder.Name() == name {
			return transcoder, true
		}
	}
	return nil, false
}

// GetSupportedTranscoders returns all transcoders that support the given profile
func GetSupportedTranscoders(profile *DeviceProfile) []Transcoder {
	var supported []Transcoder
	for _, transcoder := range GlobalTranscoderRegistry.transcoders {
		if transcoder.Supports(profile) {
			supported = append(supported, transcoder)
		}
	}
	return supported
}

// ========================================================================
// Modern TranscodingService Interface (Primary)
// ========================================================================

// TranscodingService interface using the new simplified design
type TranscodingService interface {
	// GetCapabilities returns what codecs and resolutions this transcoder supports
	GetCapabilities(ctx context.Context) (*TranscodingCapabilities, error)

	// StartTranscode initiates a transcoding session using the new request format
	StartTranscode(ctx context.Context, req *TranscodeRequest) (*TranscodeSession, error)

	// GetTranscodeSession retrieves information about an active session
	GetTranscodeSession(ctx context.Context, sessionID string) (*TranscodeSession, error)

	// StopTranscode terminates a transcoding session
	StopTranscode(ctx context.Context, sessionID string) error

	// ListActiveSessions returns all currently active transcoding sessions
	ListActiveSessions(ctx context.Context) ([]*TranscodeSession, error)

	// GetTranscodeStream returns the output stream for a transcoding session
	GetTranscodeStream(ctx context.Context, sessionID string) (io.ReadCloser, error)
}

// TranscodingCapabilities describes what a transcoder can handle
type TranscodingCapabilities struct {
	Name                  string              `json:"name"`
	SupportedCodecs       []string            `json:"supported_codecs"`
	SupportedResolutions  []string            `json:"supported_resolutions"`
	SupportedContainers   []string            `json:"supported_containers"`
	HardwareAcceleration  bool                `json:"hardware_acceleration"`
	MaxConcurrentSessions int                 `json:"max_concurrent_sessions"`
	Features              TranscodingFeatures `json:"features"`
	Priority              int                 `json:"priority"` // Higher = preferred
}

// TranscodingFeatures describes advanced transcoding features
type TranscodingFeatures struct {
	SubtitleBurnIn      bool `json:"subtitle_burn_in"`
	SubtitlePassthrough bool `json:"subtitle_passthrough"`
	MultiAudioTracks    bool `json:"multi_audio_tracks"`
	HDRSupport          bool `json:"hdr_support"`
	ToneMapping         bool `json:"tone_mapping"`
	StreamingOutput     bool `json:"streaming_output"`
	SegmentedOutput     bool `json:"segmented_output"`
}

// LegacyTranscodeRequest represents the old complex transcoding request
type LegacyTranscodeRequest struct {
	// Input
	InputPath string `json:"input_path"`
	StartTime int    `json:"start_time,omitempty"` // Start time in seconds for seek-ahead

	// Output format
	TargetCodec     string `json:"target_codec"`
	TargetContainer string `json:"target_container"`
	Resolution      string `json:"resolution"`
	Bitrate         int    `json:"bitrate"`

	// Audio settings
	AudioCodec   string `json:"audio_codec"`
	AudioBitrate int    `json:"audio_bitrate"`
	AudioStream  int    `json:"audio_stream"`

	// Subtitle settings
	Subtitles *SubtitleConfig `json:"subtitles,omitempty"`

	// Advanced settings
	Quality int               `json:"quality"` // CRF or quality level
	Preset  string            `json:"preset"`  // Encoding preset (fast, medium, slow, etc.)
	Options map[string]string `json:"options"` // Additional encoder options

	// Client information
	DeviceProfile *DeviceProfile `json:"device_profile,omitempty"`

	// Session settings
	Priority int `json:"priority"` // Session priority (1-10)
}

// SubtitleConfig defines subtitle handling
type SubtitleConfig struct {
	Enabled   bool   `json:"enabled"`
	BurnIn    bool   `json:"burn_in"`
	StreamIdx int    `json:"stream_idx"`
	FontSize  int    `json:"font_size,omitempty"`
	FontColor string `json:"font_color,omitempty"`
}

// TranscodeSession represents an active transcoding session
type TranscodeSession struct {
	ID        string                 `json:"id"`
	Request   *TranscodeRequest      `json:"request"`
	Status    TranscodeStatus        `json:"status"`
	Progress  float64                `json:"progress"` // 0.0 to 1.0
	StartTime time.Time              `json:"start_time"`
	EndTime   *time.Time             `json:"end_time,omitempty"`
	Backend   string                 `json:"backend"`
	Error     string                 `json:"error,omitempty"`
	Stats     *TranscodeStats        `json:"stats,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// TranscodeStatus represents the status of a transcoding session
type TranscodeStatus string

const (
	StatusPending   TranscodeStatus = "pending"
	StatusStarting  TranscodeStatus = "starting"
	StatusRunning   TranscodeStatus = "running"
	StatusCompleted TranscodeStatus = "completed"
	StatusFailed    TranscodeStatus = "failed"
	StatusCancelled TranscodeStatus = "cancelled"
)

// TranscodeStats contains performance statistics for a transcoding session
type TranscodeStats struct {
	Duration        time.Duration `json:"duration"`
	BytesProcessed  int64         `json:"bytes_processed"`
	BytesGenerated  int64         `json:"bytes_generated"`
	FramesProcessed int64         `json:"frames_processed"`
	CurrentFPS      float64       `json:"current_fps"`
	AverageFPS      float64       `json:"average_fps"`
	CPUUsage        float64       `json:"cpu_usage"`
	MemoryUsage     int64         `json:"memory_usage"`
	Speed           float64       `json:"speed"` // Transcoding speed multiplier
}

// TranscodingServiceClient interface for communicating with transcoding services
type TranscodingServiceClient interface {
	GetCapabilities(ctx context.Context) (*TranscodingCapabilities, error)
	StartTranscode(ctx context.Context, req *TranscodeRequest) (*TranscodeSession, error)
	GetTranscodeSession(ctx context.Context, sessionID string) (*TranscodeSession, error)
	StopTranscode(ctx context.Context, sessionID string) error
	ListActiveSessions(ctx context.Context) ([]*TranscodeSession, error)
	GetTranscodeStream(ctx context.Context, sessionID string) (io.ReadCloser, error)
}

// ========================================================================
// Shared Transcoding Utilities
// ========================================================================

// TranscodingHelper provides shared utilities for all transcoding plugins
type TranscodingHelper struct {
	DataDir string
	Logger  Logger
}

// NewTranscodingHelper creates a new transcoding helper
func NewTranscodingHelper(logger Logger) *TranscodingHelper {
	dataDir := "/viewra-data/transcoding"
	if envDir := os.Getenv("VIEWRA_TRANSCODING_DIR"); envDir != "" {
		dataDir = envDir
	}

	return &TranscodingHelper{
		DataDir: dataDir,
		Logger:  logger,
	}
}

// GenerateSessionDirectory creates a properly structured session directory
// Format: [encode_type]_[transcoder_plugin_type]_[UUID]
func (th *TranscodingHelper) GenerateSessionDirectory(encodeType, transcoderType string, sessionID string) (string, error) {
	// Always use the provided session ID to ensure consistency with backend expectations
	var sessionUUID string
	if strings.Contains(sessionID, "-") && len(sessionID) >= 36 {
		// Use the provided session ID as-is
		sessionUUID = sessionID
	} else {
		// If sessionID is not a UUID, use it as-is but log a warning
		sessionUUID = sessionID
		if th.Logger != nil {
			th.Logger.Warn("session ID is not a UUID format", "session_id", sessionID)
		}
	}

	// Create directory name: [encode_type]_[transcoder_type]_[UUID]
	dirName := fmt.Sprintf("%s_%s_%s", encodeType, transcoderType, sessionUUID)
	sessionDir := filepath.Join(th.DataDir, dirName)

	// Create directory
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create session directory %s: %w", sessionDir, err)
	}

	if th.Logger != nil {
		th.Logger.Info("created session directory", "path", sessionDir, "session_id", sessionID)
	}

	return sessionDir, nil
}

// CleanupSession removes session files and directories
func (th *TranscodingHelper) CleanupSession(sessionID string) error {
	// Find directories that match this session ID
	entries, err := os.ReadDir(th.DataDir)
	if err != nil {
		return fmt.Errorf("failed to read transcoding directory: %w", err)
	}

	var cleanedDirs []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if directory name ends with the session ID (UUID)
		if strings.HasSuffix(entry.Name(), sessionID) {
			dirPath := filepath.Join(th.DataDir, entry.Name())
			if err := os.RemoveAll(dirPath); err != nil {
				if th.Logger != nil {
					th.Logger.Warn("failed to remove session directory", "path", dirPath, "error", err)
				}
			} else {
				cleanedDirs = append(cleanedDirs, entry.Name())
			}
		}
	}

	if th.Logger != nil && len(cleanedDirs) > 0 {
		th.Logger.Info("cleaned up session directories", "session_id", sessionID, "directories", cleanedDirs)
	}

	return nil
}

// CleanupExpiredSessions removes old session directories based on retention policy
func (th *TranscodingHelper) CleanupExpiredSessions(retentionHours int, maxSizeLimitGB int) (*CleanupStats, error) {
	stats := &CleanupStats{
		LastCleanupTime:        time.Now(),
		RetentionHours:         retentionHours,
		ExtendedRetentionHours: retentionHours * 4, // Extended retention for smaller files
		MaxSizeLimitGB:         maxSizeLimitGB,
	}

	entries, err := os.ReadDir(th.DataDir)
	if err != nil {
		return stats, fmt.Errorf("failed to read transcoding directory: %w", err)
	}

	cutoffTime := time.Now().Add(-time.Duration(retentionHours) * time.Hour)
	extendedCutoffTime := time.Now().Add(-time.Duration(retentionHours*4) * time.Hour)

	var totalSize int64
	var removedDirs []string
	var removedSize int64

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirPath := filepath.Join(th.DataDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Calculate directory size
		dirSize := th.calculateDirectorySize(dirPath)
		totalSize += dirSize
		stats.TotalDirectories++

		// Check if directory should be removed
		shouldRemove := false
		sizeMB := float64(dirSize) / (1024 * 1024)

		if info.ModTime().Before(cutoffTime) {
			// Regular retention policy
			shouldRemove = true
		} else if sizeMB < 500 && info.ModTime().Before(extendedCutoffTime) {
			// Extended retention for smaller files
			shouldRemove = true
		}

		// Emergency cleanup if total size exceeds limit
		if maxSizeLimitGB > 0 && float64(totalSize)/(1024*1024*1024) > float64(maxSizeLimitGB) {
			if info.ModTime().Before(time.Now().Add(-1 * time.Hour)) {
				shouldRemove = true
			}
		}

		if shouldRemove {
			if err := os.RemoveAll(dirPath); err != nil {
				if th.Logger != nil {
					th.Logger.Warn("failed to remove expired directory", "path", dirPath, "error", err)
				}
			} else {
				removedDirs = append(removedDirs, entry.Name())
				removedSize += dirSize
				stats.DirectoriesRemoved++
			}
		}
	}

	stats.TotalSizeGB = float64(totalSize) / (1024 * 1024 * 1024)
	stats.SizeFreedGB = float64(removedSize) / (1024 * 1024 * 1024)
	stats.NextCleanupTime = time.Now().Add(30 * time.Minute) // Next cleanup in 30 minutes

	if th.Logger != nil && len(removedDirs) > 0 {
		th.Logger.Info("completed cleanup of expired sessions",
			"removed_directories", len(removedDirs),
			"size_freed_gb", stats.SizeFreedGB,
			"total_size_gb", stats.TotalSizeGB,
		)
	}

	return stats, nil
}

// calculateDirectorySize calculates the total size of a directory
func (th *TranscodingHelper) calculateDirectorySize(dirPath string) int64 {
	var size int64

	filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Continue on errors
		}
		if !d.IsDir() {
			if info, err := d.Info(); err == nil {
				size += info.Size()
			}
		}
		return nil
	})

	return size
}

// GetOutputPath generates the appropriate output path for different container types
func (th *TranscodingHelper) GetOutputPath(sessionDir string, container string) string {
	switch container {
	case "dash":
		return filepath.Join(sessionDir, "manifest.mpd")
	case "hls":
		return filepath.Join(sessionDir, "playlist.m3u8")
	default:
		return filepath.Join(sessionDir, "output.mp4")
	}
}

// CleanupStats represents statistics about file cleanup operations
type CleanupStats struct {
	TotalDirectories       int       `json:"total_directories"`
	TotalSizeGB            float64   `json:"total_size_gb"`
	DirectoriesRemoved     int       `json:"directories_removed"`
	SizeFreedGB            float64   `json:"size_freed_gb"`
	LastCleanupTime        time.Time `json:"last_cleanup_time"`
	NextCleanupTime        time.Time `json:"next_cleanup_time"`
	RetentionHours         int       `json:"retention_hours"`
	ExtendedRetentionHours int       `json:"extended_retention_hours"`
	MaxSizeLimitGB         int       `json:"max_size_limit_gb"`
}
