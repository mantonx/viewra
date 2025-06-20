package config

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mantonx/viewra/pkg/plugins"
)

// Config represents the complete transcoder configuration
// Split into generic settings and FFmpeg-specific settings
type Config struct {
	// Generic settings that all transcoders need
	Core     CoreConfig     `json:"core"`
	Hardware HardwareConfig `json:"hardware"`
	Sessions SessionConfig  `json:"sessions"`
	Cleanup  CleanupConfig  `json:"cleanup"`
	Debug    DebugConfig    `json:"debug"`

	// FFmpeg-specific settings
	FFmpeg FFmpegConfig `json:"ffmpeg"`
}

// CoreConfig contains generic core settings
type CoreConfig struct {
	Enabled         bool   `json:"enabled"`
	Priority        int    `json:"priority"`
	OutputDirectory string `json:"output_directory"`
}

// HardwareConfig contains generic hardware acceleration settings
type HardwareConfig struct {
	Enabled         bool                 `json:"enabled"`
	PreferredType   plugins.HardwareType `json:"preferred_type"`   // "auto", "cuda", "vaapi", etc.
	DeviceSelection string               `json:"device_selection"` // "auto", "first", "load-balanced"
	Fallback        bool                 `json:"fallback"`         // Fall back to software if HW fails
}

// SessionConfig contains generic session management settings
type SessionConfig struct {
	MaxConcurrent  int `json:"max_concurrent"`
	TimeoutMinutes int `json:"timeout_minutes"`
	IdleMinutes    int `json:"idle_minutes"`
}

// CleanupConfig contains generic cleanup settings
type CleanupConfig struct {
	Enabled         bool `json:"enabled"`
	RetentionHours  int  `json:"retention_hours"`
	ExtendedHours   int  `json:"extended_hours"` // Extended retention for small files
	MaxSizeGB       int  `json:"max_size_gb"`
	IntervalMinutes int  `json:"interval_minutes"`
}

// DebugConfig contains generic debug settings
type DebugConfig struct {
	Enabled  bool   `json:"enabled"`
	LogLevel string `json:"log_level"` // "debug", "info", "warn", "error"
}

// FFmpegConfig contains FFmpeg-specific settings
type FFmpegConfig struct {
	BinaryPath string `json:"binary_path"`
	ProbePath  string `json:"probe_path"`
	Threads    int    `json:"threads"` // 0 = auto

	// FFmpeg-specific quality defaults (will be mapped from generic quality)
	DefaultCRF    map[string]int `json:"default_crf"`    // Per-codec CRF defaults
	DefaultPreset string         `json:"default_preset"` // FFmpeg preset default

	// Advanced FFmpeg options
	ExtraArgs       []string `json:"extra_args"`        // Additional FFmpeg arguments
	TwoPass         bool     `json:"two_pass"`          // Enable two-pass encoding
	AudioBitrate    int      `json:"audio_bitrate"`     // Default audio bitrate in kbps
	LogFFmpegOutput bool     `json:"log_ffmpeg_output"` // Log FFmpeg command output
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Core: CoreConfig{
			Enabled:         true,
			Priority:        50,
			OutputDirectory: "/viewra-data/transcoding",
		},
		Hardware: HardwareConfig{
			Enabled:         true,
			PreferredType:   plugins.HardwareTypeNone,
			DeviceSelection: "auto",
			Fallback:        true,
		},
		Sessions: SessionConfig{
			MaxConcurrent:  10,
			TimeoutMinutes: 120,
			IdleMinutes:    10,
		},
		Cleanup: CleanupConfig{
			Enabled:         true,
			RetentionHours:  2,
			ExtendedHours:   8,
			MaxSizeGB:       10,
			IntervalMinutes: 30,
		},
		Debug: DebugConfig{
			Enabled:  false,
			LogLevel: "info",
		},
		FFmpeg: FFmpegConfig{
			BinaryPath: "ffmpeg",
			ProbePath:  "ffprobe",
			Threads:    0, // Auto-detect
			DefaultCRF: map[string]int{
				"h264": 23,
				"h265": 28,
				"vp9":  31,
				"av1":  30,
			},
			DefaultPreset:   "fast",
			AudioBitrate:    128,
			TwoPass:         false,
			LogFFmpegOutput: false,
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
	return c.FFmpeg.BinaryPath
}

// GetOutputDirectory returns the output directory
func (c *Config) GetOutputDirectory() string {
	return c.Core.OutputDirectory
}

// IsEnabled returns whether the plugin is enabled
func (c *Config) IsEnabled() bool {
	return c.Core.Enabled
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate core settings
	if c.Core.OutputDirectory == "" {
		return fmt.Errorf("output directory cannot be empty")
	}

	// Validate FFmpeg settings
	if c.FFmpeg.BinaryPath == "" {
		return fmt.Errorf("FFmpeg path cannot be empty")
	}

	if c.FFmpeg.Threads < 0 {
		return fmt.Errorf("thread count must be non-negative")
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
		Enabled:  ffmpegConfig.Core.Enabled,
		Settings: configMap,
		Features: map[string]bool{
			"enabled":         ffmpegConfig.Core.Enabled,
			"debug_mode":      ffmpegConfig.Debug.Enabled,
			"ffmpeg_logging":  ffmpegConfig.FFmpeg.LogFFmpegOutput,
			"two_pass":        ffmpegConfig.FFmpeg.TwoPass,
			"hw_acceleration": ffmpegConfig.Hardware.Enabled,
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
					"binary_path": map[string]interface{}{
						"type":        "string",
						"title":       "FFmpeg Path",
						"description": "Path to the FFmpeg executable",
						"default":     "ffmpeg",
					},
					"probe_path": map[string]interface{}{
						"type":        "string",
						"title":       "Probe Path",
						"description": "Path to the ffprobe executable",
						"default":     "ffprobe",
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
				"binary_path": "ffmpeg",
				"threads":     0,
				"priority":    0,
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
				"binary_path": "ffmpeg",
				"threads":     8,
				"priority":    -5,
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
			"binary_path": "ffmpeg",
			"threads":     0,
			"priority":    0,
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
