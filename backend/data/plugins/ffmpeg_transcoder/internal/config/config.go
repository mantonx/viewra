package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mantonx/viewra/pkg/plugins"
)

// Config represents the complete FFmpeg transcoder plugin configuration
// This mirrors the CUE schema defined in plugin.cue
type Config struct {
	Enabled     bool              `json:"enabled"`
	FFmpeg      FFmpegConfig      `json:"ffmpeg"`
	Transcoding TranscodingConfig `json:"transcoding"`
	Sessions    SessionsConfig    `json:"sessions"`
	Performance PerformanceConfig `json:"performance"`
	Hardware    HardwareConfig    `json:"hardware"`
	Debug       DebugConfig       `json:"debug"`
}

// FFmpegConfig contains FFmpeg binary and execution settings
type FFmpegConfig struct {
	Path     string `json:"path"`     // Path to FFmpeg binary
	Threads  int    `json:"threads"`  // Number of threads (0 = auto)
	Priority int    `json:"priority"` // Process priority
}

// TranscodingConfig contains default transcoding settings
type TranscodingConfig struct {
	OutputDir    string `json:"output_dir"`    // Output directory for transcoded files
	Quality      int    `json:"quality"`       // Default CRF quality (0-51)
	Preset       string `json:"preset"`        // Default encoding preset
	AudioCodec   string `json:"audio_codec"`   // Default audio codec
	AudioBitrate int    `json:"audio_bitrate"` // Default audio bitrate in kbps
	Container    string `json:"container"`     // Default output container
}

// SessionsConfig contains session management settings
type SessionsConfig struct {
	MaxConcurrent int `json:"max_concurrent"` // Maximum concurrent sessions
	CleanupHours  int `json:"cleanup_hours"`  // Hours after which to clean up old sessions
	TimeoutHours  int `json:"timeout_hours"`  // Hours after which to timeout stuck sessions
}

// PerformanceConfig contains performance monitoring settings
type PerformanceConfig struct {
	EnableMetrics    bool `json:"enable_metrics"`    // Enable performance metrics collection
	MetricsInterval  int  `json:"metrics_interval"`  // Metrics collection interval in seconds
	ResourceMonitor  bool `json:"resource_monitor"`  // Monitor CPU/memory usage
	ProgressInterval int  `json:"progress_interval"` // Progress update interval in seconds
}

// HardwareConfig contains hardware acceleration settings
type HardwareConfig struct {
	Acceleration bool   `json:"acceleration"` // Always enabled (auto mode)
	Type         string `json:"type"`         // Always "auto" - let FFmpeg choose
	Fallback     bool   `json:"fallback"`     // Not used in auto mode
}

// DebugConfig contains debugging and logging settings
type DebugConfig struct {
	Enabled      bool   `json:"enabled"`       // Enable debug logging
	LogFFmpeg    bool   `json:"log_ffmpeg"`    // Log FFmpeg command output
	SaveCommands bool   `json:"save_commands"` // Save FFmpeg commands to file
	LogLevel     string `json:"log_level"`     // Log level (debug, info, warn, error)
}

// DefaultConfig returns the default configuration for the FFmpeg transcoder
func DefaultConfig() *Config {
	return &Config{
		Enabled: true,
		FFmpeg: FFmpegConfig{
			Path:     detectFFmpegPath(),
			Threads:  0, // Auto-detect
			Priority: 0, // Normal priority
		},
		Transcoding: TranscodingConfig{
			OutputDir:    "/app/viewra-data/transcoding",
			Quality:      23, // Good quality/size balance
			Preset:       "medium",
			AudioCodec:   "aac",
			AudioBitrate: 128,
			Container:    "mp4",
		},
		Sessions: SessionsConfig{
			MaxConcurrent: 4,
			CleanupHours:  24,
			TimeoutHours:  2,
		},
		Performance: PerformanceConfig{
			EnableMetrics:    true,
			MetricsInterval:  30,
			ResourceMonitor:  true,
			ProgressInterval: 5,
		},
		Hardware: HardwareConfig{
			Acceleration: true,
			Type:         "auto",
			Fallback:     true,
		},
		Debug: DebugConfig{
			Enabled:      false,
			LogFFmpeg:    false,
			SaveCommands: false,
			LogLevel:     "info",
		},
	}
}

// Validate validates the configuration and returns any errors
func (c *Config) Validate() error {
	// Validate FFmpeg path
	if c.FFmpeg.Path == "" {
		return fmt.Errorf("FFmpeg path cannot be empty")
	}

	// Check if FFmpeg binary exists
	if _, err := os.Stat(c.FFmpeg.Path); os.IsNotExist(err) {
		return fmt.Errorf("FFmpeg binary not found at path: %s", c.FFmpeg.Path)
	}

	// Validate transcoding settings
	if c.Transcoding.Quality < 0 || c.Transcoding.Quality > 51 {
		return fmt.Errorf("quality must be between 0 and 51, got: %d", c.Transcoding.Quality)
	}

	if c.Transcoding.AudioBitrate <= 0 {
		return fmt.Errorf("audio bitrate must be positive, got: %d", c.Transcoding.AudioBitrate)
	}

	// Validate session settings
	if c.Sessions.MaxConcurrent <= 0 {
		return fmt.Errorf("max concurrent sessions must be positive, got: %d", c.Sessions.MaxConcurrent)
	}

	// Validate output directory
	if c.Transcoding.OutputDir == "" {
		return fmt.Errorf("output directory cannot be empty")
	}

	return nil
}

// GetFFmpegPath returns the configured FFmpeg path
func (c *Config) GetFFmpegPath() string {
	return c.FFmpeg.Path
}

// GetOutputDir returns the configured output directory
func (c *Config) GetOutputDir() string {
	return c.Transcoding.OutputDir
}

// IsDebugEnabled returns whether debug mode is enabled
func (c *Config) IsDebugEnabled() bool {
	return c.Debug.Enabled
}

// GetMaxConcurrentSessions returns the maximum number of concurrent sessions
func (c *Config) GetMaxConcurrentSessions() int {
	return c.Sessions.MaxConcurrent
}

// GetHardwareAcceleration returns whether hardware acceleration is enabled
func (c *Config) GetHardwareAcceleration() bool {
	return c.Hardware.Acceleration
}

// GetHardwareType returns the hardware acceleration type
func (c *Config) GetHardwareType() string {
	return c.Hardware.Type
}

// GetHardwareFallback returns whether to fallback to software if hardware fails
func (c *Config) GetHardwareFallback() bool {
	return c.Hardware.Fallback
}

// detectFFmpegPath attempts to find FFmpeg in common locations
func detectFFmpegPath() string {
	commonPaths := []string{
		"ffmpeg",                   // In PATH
		"/usr/bin/ffmpeg",          // Common Linux location
		"/usr/local/bin/ffmpeg",    // Homebrew location
		"/opt/homebrew/bin/ffmpeg", // Apple Silicon Homebrew
	}

	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Default to assuming it's in PATH
	return "ffmpeg"
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
			"enabled":          ffmpegConfig.Enabled,
			"performance_mode": ffmpegConfig.Performance.EnableMetrics,
			"resource_monitor": ffmpegConfig.Performance.ResourceMonitor,
			"debug_mode":       ffmpegConfig.Debug.Enabled,
			"ffmpeg_logging":   ffmpegConfig.Debug.LogFFmpeg,
		},
		Thresholds: &plugins.HealthThresholds{
			MaxMemoryUsage:      512 * 1024 * 1024, // 512MB
			MaxCPUUsage:         80.0,
			MaxErrorRate:        10.0,
			MaxResponseTime:     time.Duration(ffmpegConfig.Sessions.TimeoutHours) * time.Hour,
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
					"output_dir": map[string]interface{}{
						"type":        "string",
						"title":       "Output Directory",
						"description": "Directory for transcoded files",
						"default":     "/app/viewra-data/transcoding",
					},
					"quality": map[string]interface{}{
						"type":        "integer",
						"title":       "Video Quality (CRF)",
						"description": "Constant Rate Factor (0-51, lower = better)",
						"minimum":     0,
						"maximum":     51,
						"default":     23,
					},
					"preset": map[string]interface{}{
						"type":        "string",
						"title":       "Encoding Preset",
						"description": "Speed/quality tradeoff",
						"enum":        []string{"ultrafast", "superfast", "veryfast", "faster", "fast", "medium", "slow", "slower", "veryslow"},
						"default":     "medium",
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
						"default":     4,
					},
					"cleanup_hours": map[string]interface{}{
						"type":        "integer",
						"title":       "Cleanup After (hours)",
						"description": "Hours after which to clean up old sessions",
						"minimum":     1,
						"maximum":     168,
						"default":     24,
					},
					"timeout_hours": map[string]interface{}{
						"type":        "integer",
						"title":       "Session Timeout (hours)",
						"description": "Hours after which to timeout stuck sessions",
						"minimum":     1,
						"maximum":     24,
						"default":     2,
					},
				},
			},
			"performance": map[string]interface{}{
				"type":  "object",
				"title": "Performance Settings",
				"properties": map[string]interface{}{
					"enable_metrics": map[string]interface{}{
						"type":        "boolean",
						"title":       "Enable Metrics",
						"description": "Enable performance metrics collection",
						"default":     true,
					},
					"metrics_interval": map[string]interface{}{
						"type":        "integer",
						"title":       "Metrics Interval (seconds)",
						"description": "Metrics collection interval",
						"minimum":     5,
						"maximum":     300,
						"default":     30,
					},
					"resource_monitor": map[string]interface{}{
						"type":        "boolean",
						"title":       "Resource Monitoring",
						"description": "Monitor CPU/memory usage",
						"default":     true,
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
					"save_commands": map[string]interface{}{
						"type":        "boolean",
						"title":       "Save Commands",
						"description": "Save FFmpeg commands to file",
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
				"output_dir":    "/app/viewra-data/transcoding",
				"quality":       23,
				"preset":        "medium",
				"audio_codec":   "aac",
				"audio_bitrate": 128,
			},
			"sessions": map[string]interface{}{
				"max_concurrent": 4,
				"cleanup_hours":  24,
				"timeout_hours":  2,
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
				"output_dir":    "/fast/storage/transcoding",
				"quality":       20,
				"preset":        "fast",
				"audio_codec":   "aac",
				"audio_bitrate": 192,
			},
			"sessions": map[string]interface{}{
				"max_concurrent": 10,
				"cleanup_hours":  12,
				"timeout_hours":  1,
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
			"output_dir":    "/app/viewra-data/transcoding",
			"quality":       23,
			"preset":        "medium",
			"audio_codec":   "aac",
			"audio_bitrate": 128,
			"container":     "mp4",
		},
		"sessions": map[string]interface{}{
			"max_concurrent": 4,
			"cleanup_hours":  24,
			"timeout_hours":  2,
		},
		"performance": map[string]interface{}{
			"enable_metrics":    true,
			"metrics_interval":  30,
			"resource_monitor":  true,
			"progress_interval": 5,
		},
		"debug": map[string]interface{}{
			"enabled":       false,
			"log_ffmpeg":    false,
			"save_commands": false,
			"log_level":     "info",
		},
	}

	return &plugins.ConfigurationSchema{
		Schema:   schema,
		Examples: examples,
		Defaults: defaults,
	}
}
