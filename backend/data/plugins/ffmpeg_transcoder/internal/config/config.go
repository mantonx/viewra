package config

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mantonx/viewra/pkg/plugins"
)

// Config represents the complete FFmpeg transcoder configuration
// This mirrors the CUE schema defined in plugin.cue
type Config struct {
	Enabled     bool              `json:"enabled"`
	Priority    int               `json:"priority"`
	FFmpeg      FFmpegConfig      `json:"ffmpeg"`
	Transcoding TranscodingConfig `json:"transcoding"`
	Hardware    HardwareConfig    `json:"hardware"`
	Sessions    SessionConfig     `json:"sessions"`
	Performance PerformanceConfig `json:"performance"`
	Cleanup     CleanupConfig     `json:"cleanup"`
	Debug       DebugConfig       `json:"debug"`
}

// FFmpegConfig contains FFmpeg binary settings
type FFmpegConfig struct {
	Path    string `json:"path"`    // Path to FFmpeg binary
	Threads int    `json:"threads"` // Number of threads (0 = auto)
}

// TranscodingConfig contains transcoding defaults
type TranscodingConfig struct {
	// Video settings
	VideoCodec  string `json:"video_codec"`  // Default video codec
	VideoPreset string `json:"video_preset"` // Encoding preset
	VideoCRF    int    `json:"video_crf"`    // CRF value (0-51)

	// Audio settings
	AudioCodec    string `json:"audio_codec"`    // Default audio codec
	AudioBitrate  int    `json:"audio_bitrate"`  // Audio bitrate in kbps
	AudioChannels int    `json:"audio_channels"` // Audio channels (2=stereo, 6=5.1)

	// Container settings
	DefaultContainer string `json:"default_container"` // Default output container

	// Output settings
	OutputDir string `json:"output_dir"` // Output directory for transcoded files
}

// HardwareConfig contains hardware acceleration settings
type HardwareConfig struct {
	Acceleration bool   `json:"acceleration"` // Enable hardware acceleration
	Type         string `json:"type"`         // Hardware type (auto, nvenc, vaapi, etc.)
	Fallback     bool   `json:"fallback"`     // Fallback to software on failure
}

// SessionConfig contains session management settings
type SessionConfig struct {
	MaxConcurrent  int `json:"max_concurrent"`  // Maximum concurrent sessions
	TimeoutMinutes int `json:"timeout_minutes"` // Session timeout in minutes
	IdleMinutes    int `json:"idle_minutes"`    // Idle timeout in minutes
}

// PerformanceConfig contains performance settings
type PerformanceConfig struct {
	BufferSize       int  `json:"buffer_size"`       // Streaming buffer size
	ProgressInterval int  `json:"progress_interval"` // Progress update interval in seconds
	TwoPass          bool `json:"two_pass"`          // Enable two-pass encoding
}

// CleanupConfig contains file cleanup settings
type CleanupConfig struct {
	RetentionHours  int `json:"retention_hours"`  // Keep files for N hours
	ExtendedHours   int `json:"extended_hours"`   // Extended retention for small files
	MaxSizeGB       int `json:"max_size_gb"`      // Maximum total size in GB
	IntervalMinutes int `json:"interval_minutes"` // Cleanup interval in minutes
}

// DebugConfig contains debugging settings
type DebugConfig struct {
	Enabled         bool   `json:"enabled"`    // Enable debug logging
	LogFFmpegOutput bool   `json:"log_ffmpeg"` // Log FFmpeg output
	LogLevel        string `json:"log_level"`  // Log level
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Enabled:  true,
		Priority: 50,
		FFmpeg: FFmpegConfig{
			Path:    "ffmpeg",
			Threads: 0, // Auto-detect
		},
		Transcoding: TranscodingConfig{
			VideoCodec:       "h264",
			VideoPreset:      "fast",
			VideoCRF:         23,
			AudioCodec:       "aac",
			AudioBitrate:     128,
			AudioChannels:    2,
			DefaultContainer: "mp4",
			OutputDir:        "/viewra-data/transcoding",
		},
		Hardware: HardwareConfig{
			Acceleration: true,
			Type:         "auto",
			Fallback:     true,
		},
		Sessions: SessionConfig{
			MaxConcurrent:  10,
			TimeoutMinutes: 120,
			IdleMinutes:    10,
		},
		Performance: PerformanceConfig{
			BufferSize:       32768,
			ProgressInterval: 5,
			TwoPass:          false,
		},
		Cleanup: CleanupConfig{
			RetentionHours:  2,
			ExtendedHours:   8,
			MaxSizeGB:       10,
			IntervalMinutes: 30,
		},
		Debug: DebugConfig{
			Enabled:         false,
			LogFFmpegOutput: false,
			LogLevel:        "info",
		},
	}
}

// GetSessionTimeout returns the session timeout duration
func (c *SessionConfig) GetSessionTimeout() time.Duration {
	return time.Duration(c.TimeoutMinutes) * time.Minute
}

// GetIdleTimeout returns the idle timeout duration
func (c *SessionConfig) GetIdleTimeout() time.Duration {
	return time.Duration(c.IdleMinutes) * time.Minute
}

// GetCleanupInterval returns the cleanup interval duration
func (c *CleanupConfig) GetCleanupInterval() time.Duration {
	return time.Duration(c.IntervalMinutes) * time.Minute
}

// GetRetentionDuration returns the retention duration
func (c *CleanupConfig) GetRetentionDuration() time.Duration {
	return time.Duration(c.RetentionHours) * time.Hour
}

// GetExtendedRetentionDuration returns the extended retention duration
func (c *CleanupConfig) GetExtendedRetentionDuration() time.Duration {
	return time.Duration(c.ExtendedHours) * time.Hour
}

// GetMaxConcurrentSessions returns the maximum concurrent sessions allowed
func (c *Config) GetMaxConcurrentSessions() int {
	return c.Sessions.MaxConcurrent
}

// GetFFmpegPath returns the path to the FFmpeg executable
func (c *Config) GetFFmpegPath() string {
	return c.FFmpeg.Path
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate FFmpeg settings
	if c.FFmpeg.Path == "" {
		return fmt.Errorf("FFmpeg path cannot be empty")
	}

	if c.FFmpeg.Threads < 0 {
		return fmt.Errorf("thread count must be non-negative")
	}

	// Validate transcoding settings
	if c.Transcoding.VideoCRF < 0 || c.Transcoding.VideoCRF > 51 {
		return fmt.Errorf("video CRF must be between 0 and 51")
	}

	if c.Transcoding.AudioBitrate <= 0 {
		return fmt.Errorf("audio bitrate must be positive")
	}

	// Validate session settings
	if c.Sessions.MaxConcurrent <= 0 {
		return fmt.Errorf("max concurrent sessions must be positive")
	}

	if c.Sessions.TimeoutMinutes <= 0 {
		return fmt.Errorf("session timeout must be positive")
	}

	// Validate cleanup settings
	if c.Cleanup.RetentionHours <= 0 {
		return fmt.Errorf("retention hours must be positive")
	}

	if c.Cleanup.MaxSizeGB <= 0 {
		return fmt.Errorf("max size must be positive")
	}

	return nil
}

// FFmpegConfigurationService extends the base configuration service with FFmpeg-specific functionality
type FFmpegConfigurationService struct {
	*plugins.BaseConfigurationService
	config *Config
}

// NewFFmpegConfigurationService creates a new FFmpeg-specific configuration service
func NewFFmpegConfigurationService(configPath string) *FFmpegConfigurationService {
	baseService := plugins.NewBaseConfigurationService("ffmpeg_transcoder", configPath)

	service := &FFmpegConfigurationService{
		BaseConfigurationService: baseService,
		config:                   DefaultConfig(),
	}

	// Set FFmpeg-specific schema
	service.SetConfigurationSchema(service.createFFmpegSchema())

	// Add FFmpeg-specific configuration callback
	service.AddConfigurationCallback(service.onConfigurationChanged)

	return service
}

// GetFFmpegConfig returns the current FFmpeg configuration
func (c *FFmpegConfigurationService) GetFFmpegConfig() *Config {
	return c.config
}

// UpdateFFmpegConfig updates the FFmpeg configuration
func (c *FFmpegConfigurationService) UpdateFFmpegConfig(newConfig *Config) error {
	// Validate FFmpeg-specific configuration
	if err := newConfig.Validate(); err != nil {
		return fmt.Errorf("FFmpeg config validation failed: %w", err)
	}

	// Convert to plugin configuration format
	pluginConfig := c.ffmpegToPluginConfig(newConfig)

	// Use base service to update
	ctx := context.Background()
	return c.BaseConfigurationService.UpdateConfiguration(ctx, pluginConfig)
}

// Initialize loads and validates the FFmpeg configuration
func (c *FFmpegConfigurationService) Initialize() error {
	// Initialize base service
	if err := c.BaseConfigurationService.Initialize(); err != nil {
		return err
	}

	// Load FFmpeg-specific configuration
	pluginConfig, err := c.BaseConfigurationService.GetConfiguration(context.Background())
	if err != nil {
		return err
	}

	// Convert plugin config to FFmpeg config
	c.config = c.pluginToFFmpegConfig(pluginConfig)

	// Validate FFmpeg configuration
	if err := c.config.Validate(); err != nil {
		return fmt.Errorf("FFmpeg plugin configuration is invalid: %w", err)
	}

	return nil
}

// onConfigurationChanged is called when configuration changes
func (c *FFmpegConfigurationService) onConfigurationChanged(oldConfig, newConfig *plugins.PluginConfiguration) error {
	// Convert plugin config to FFmpeg config
	c.config = c.pluginToFFmpegConfig(newConfig)

	// Validate the new configuration
	if err := c.config.Validate(); err != nil {
		return fmt.Errorf("invalid FFmpeg configuration: %w", err)
	}

	return nil
}

// ffmpegToPluginConfig converts FFmpeg config to plugin configuration format
func (c *FFmpegConfigurationService) ffmpegToPluginConfig(ffmpegConfig *Config) *plugins.PluginConfiguration {
	// Serialize FFmpeg config to JSON
	configBytes, _ := json.Marshal(ffmpegConfig)
	var configMap map[string]interface{}
	json.Unmarshal(configBytes, &configMap)

	return &plugins.PluginConfiguration{
		Version:  "1.0.0",
		Enabled:  ffmpegConfig.Enabled,
		Settings: configMap,
		Features: map[string]bool{
			"enabled":         ffmpegConfig.Enabled,
			"debug_mode":      ffmpegConfig.Debug.Enabled,
			"ffmpeg_logging":  ffmpegConfig.Debug.LogFFmpegOutput,
			"two_pass":        ffmpegConfig.Performance.TwoPass,
			"hw_acceleration": ffmpegConfig.Hardware.Acceleration,
		},
		Thresholds: &plugins.HealthThresholds{
			MaxMemoryUsage:      512 * 1024 * 1024, // 512MB
			MaxCPUUsage:         80.0,
			MaxErrorRate:        10.0,
			MaxResponseTime:     time.Duration(ffmpegConfig.Sessions.TimeoutMinutes) * time.Minute,
			HealthCheckInterval: 30 * time.Second,
		},
		LastModified: time.Now(),
		ModifiedBy:   "ffmpeg_plugin",
	}
}

// pluginToFFmpegConfig converts plugin configuration to FFmpeg config format
func (c *FFmpegConfigurationService) pluginToFFmpegConfig(pluginConfig *plugins.PluginConfiguration) *Config {
	// Try to extract FFmpeg config from settings
	if configBytes, err := json.Marshal(pluginConfig.Settings); err == nil {
		var ffmpegConfig Config
		if err := json.Unmarshal(configBytes, &ffmpegConfig); err == nil {
			return &ffmpegConfig
		}
	}

	// Fallback to default config if conversion fails
	return DefaultConfig()
}

// createFFmpegSchema creates the JSON schema for FFmpeg configuration
func (c *FFmpegConfigurationService) createFFmpegSchema() *plugins.ConfigurationSchema {
	schema := map[string]interface{}{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"title":   "FFmpeg Transcoder Configuration",
		"type":    "object",
		"properties": map[string]interface{}{
			"enabled": map[string]interface{}{
				"type":        "boolean",
				"title":       "Enable Plugin",
				"description": "Enable or disable the FFmpeg transcoder plugin",
				"default":     true,
			},
			"ffmpeg": map[string]interface{}{
				"type":  "object",
				"title": "FFmpeg Settings",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"title":       "FFmpeg Path",
						"description": "Path to the FFmpeg executable",
						"default":     "ffmpeg",
					},
					"threads": map[string]interface{}{
						"type":        "integer",
						"title":       "Thread Count",
						"description": "Number of threads (0 = auto)",
						"minimum":     0,
						"maximum":     32,
						"default":     0,
					},
					"priority": map[string]interface{}{
						"type":        "integer",
						"title":       "Process Priority",
						"description": "Process priority (-20 to 20)",
						"minimum":     -20,
						"maximum":     20,
						"default":     0,
					},
				},
			},
			"transcoding": map[string]interface{}{
				"type":  "object",
				"title": "Transcoding Settings",
				"properties": map[string]interface{}{
					"video_codec": map[string]interface{}{
						"type":        "string",
						"title":       "Video Codec",
						"description": "Default video codec",
						"enum":        []string{"h264", "h265", "vp8", "vp9"},
						"default":     "h264",
					},
					"video_preset": map[string]interface{}{
						"type":        "string",
						"title":       "Video Preset",
						"description": "Encoding preset",
						"enum":        []string{"ultrafast", "superfast", "veryfast", "faster", "fast", "medium", "slow", "slower", "veryslow"},
						"default":     "fast",
					},
					"video_crf": map[string]interface{}{
						"type":        "integer",
						"title":       "Video CRF",
						"description": "Constant Rate Factor (0-51, lower = better)",
						"minimum":     0,
						"maximum":     51,
						"default":     23,
					},
					"audio_codec": map[string]interface{}{
						"type":        "string",
						"title":       "Audio Codec",
						"description": "Default audio codec",
						"enum":        []string{"aac", "mp3", "opus", "vorbis"},
						"default":     "aac",
					},
					"audio_bitrate": map[string]interface{}{
						"type":        "integer",
						"title":       "Audio Bitrate (kbps)",
						"description": "Default audio bitrate",
						"minimum":     64,
						"maximum":     320,
						"default":     128,
					},
					"audio_channels": map[string]interface{}{
						"type":        "integer",
						"title":       "Audio Channels",
						"description": "Default audio channels",
						"enum":        []string{"2", "6"},
						"default":     2,
					},
					"default_container": map[string]interface{}{
						"type":        "string",
						"title":       "Default Container",
						"description": "Default output container",
						"enum":        []string{"mp4", "mkv", "avi", "mov", "webm"},
						"default":     "mp4",
					},
				},
			},
			"sessions": map[string]interface{}{
				"type":  "object",
				"title": "Session Management",
				"properties": map[string]interface{}{
					"max_concurrent": map[string]interface{}{
						"type":        "integer",
						"title":       "Max Concurrent Sessions",
						"description": "Maximum number of concurrent transcoding sessions",
						"minimum":     1,
						"maximum":     100,
						"default":     10,
					},
					"timeout_minutes": map[string]interface{}{
						"type":        "integer",
						"title":       "Session Timeout (minutes)",
						"description": "Session timeout in minutes",
						"minimum":     1,
						"maximum":     120,
						"default":     120,
					},
					"idle_minutes": map[string]interface{}{
						"type":        "integer",
						"title":       "Idle Timeout (minutes)",
						"description": "Idle timeout in minutes",
						"minimum":     1,
						"maximum":     120,
						"default":     10,
					},
				},
			},
			"performance": map[string]interface{}{
				"type":  "object",
				"title": "Performance Settings",
				"properties": map[string]interface{}{
					"buffer_size": map[string]interface{}{
						"type":        "integer",
						"title":       "Buffer Size",
						"description": "Streaming buffer size",
						"minimum":     1024,
						"maximum":     1048576,
						"default":     32768,
					},
					"progress_interval": map[string]interface{}{
						"type":        "integer",
						"title":       "Progress Interval (seconds)",
						"description": "Progress update interval",
						"minimum":     5,
						"maximum":     300,
						"default":     5,
					},
					"two_pass": map[string]interface{}{
						"type":        "boolean",
						"title":       "Two-Pass Encoding",
						"description": "Enable two-pass encoding",
						"default":     false,
					},
				},
			},
			"cleanup": map[string]interface{}{
				"type":  "object",
				"title": "Cleanup Settings",
				"properties": map[string]interface{}{
					"retention_hours": map[string]interface{}{
						"type":        "integer",
						"title":       "Retention Hours",
						"description": "Keep files for N hours",
						"minimum":     1,
						"maximum":     24,
						"default":     2,
					},
					"extended_hours": map[string]interface{}{
						"type":        "integer",
						"title":       "Extended Hours",
						"description": "Extended retention for small files",
						"minimum":     1,
						"maximum":     24,
						"default":     8,
					},
					"max_size_gb": map[string]interface{}{
						"type":        "integer",
						"title":       "Max Size (GB)",
						"description": "Maximum total size in GB",
						"minimum":     1,
						"maximum":     100,
						"default":     10,
					},
					"interval_minutes": map[string]interface{}{
						"type":        "integer",
						"title":       "Cleanup Interval (minutes)",
						"description": "Cleanup interval in minutes",
						"minimum":     1,
						"maximum":     60,
						"default":     30,
					},
				},
			},
			"debug": map[string]interface{}{
				"type":  "object",
				"title": "Debug Settings",
				"properties": map[string]interface{}{
					"enabled": map[string]interface{}{
						"type":        "boolean",
						"title":       "Debug Mode",
						"description": "Enable debug logging",
						"default":     false,
					},
					"log_ffmpeg": map[string]interface{}{
						"type":        "boolean",
						"title":       "Log FFmpeg Output",
						"description": "Log FFmpeg command output",
						"default":     false,
					},
				},
			},
		},
		"required": []string{"enabled", "ffmpeg", "transcoding", "sessions"},
	}

	examples := map[string]interface{}{
		"basic": map[string]interface{}{
			"enabled": true,
			"ffmpeg": map[string]interface{}{
				"path":     "ffmpeg",
				"threads":  0,
				"priority": 0,
			},
			"transcoding": map[string]interface{}{
				"video_codec":       "h264",
				"video_preset":      "fast",
				"video_crf":         23,
				"audio_codec":       "aac",
				"audio_bitrate":     128,
				"audio_channels":    2,
				"default_container": "mp4",
			},
			"sessions": map[string]interface{}{
				"max_concurrent":  10,
				"timeout_minutes": 120,
				"idle_minutes":    10,
			},
		},
		"high_performance": map[string]interface{}{
			"enabled": true,
			"ffmpeg": map[string]interface{}{
				"path":     "/usr/bin/ffmpeg",
				"threads":  8,
				"priority": -5,
			},
			"transcoding": map[string]interface{}{
				"video_codec":       "h265",
				"video_preset":      "ultrafast",
				"video_crf":         20,
				"audio_codec":       "aac",
				"audio_bitrate":     192,
				"audio_channels":    6,
				"default_container": "mkv",
			},
			"sessions": map[string]interface{}{
				"max_concurrent":  10,
				"timeout_minutes": 120,
				"idle_minutes":    10,
			},
		},
	}

	defaults := map[string]interface{}{
		"enabled": true,
		"ffmpeg": map[string]interface{}{
			"path":     "ffmpeg",
			"threads":  0,
			"priority": 0,
		},
		"transcoding": map[string]interface{}{
			"video_codec":       "h264",
			"video_preset":      "fast",
			"video_crf":         23,
			"audio_codec":       "aac",
			"audio_bitrate":     128,
			"audio_channels":    2,
			"default_container": "mp4",
		},
		"sessions": map[string]interface{}{
			"max_concurrent":  10,
			"timeout_minutes": 120,
			"idle_minutes":    10,
		},
		"performance": map[string]interface{}{
			"buffer_size":       32768,
			"progress_interval": 5,
			"two_pass":          false,
		},
		"cleanup": map[string]interface{}{
			"retention_hours":  2,
			"extended_hours":   8,
			"max_size_gb":      10,
			"interval_minutes": 30,
		},
		"debug": map[string]interface{}{
			"enabled":    false,
			"log_ffmpeg": false,
			"log_level":  "info",
		},
	}

	return &plugins.ConfigurationSchema{
		Schema:   schema,
		Examples: examples,
		Defaults: defaults,
	}
}
