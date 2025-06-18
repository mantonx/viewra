package config

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/pkg/plugins"
)

// FFmpegConfig represents the FFmpeg transcoder configuration
type FFmpegConfig struct {
	// Core settings
	Enabled bool `json:"enabled"`

	// FFmpeg configuration
	FFmpegPath string `json:"ffmpeg_path" cue:"ffmpeg.path"`
	Preset     string `json:"preset" cue:"ffmpeg.preset"`
	Threads    int    `json:"threads" cue:"ffmpeg.threads"`
	Priority   int    `json:"priority" cue:"ffmpeg.priority"`

	// Quality settings
	CRFH264              float64 `json:"crf_h264" cue:"quality.crf_h264"`
	CRFHEVC              float64 `json:"crf_hevc" cue:"quality.crf_hevc"`
	MaxBitrateMultiplier float64 `json:"max_bitrate_multiplier"`
	BufferSizeMultiplier float64 `json:"buffer_size_multiplier"`

	// Audio settings
	AudioCodec      string `json:"audio_codec" cue:"audio.codec"`
	AudioBitrate    int    `json:"audio_bitrate" cue:"audio.bitrate"`
	AudioSampleRate int    `json:"audio_sample_rate" cue:"audio.sample_rate"`
	AudioChannels   int    `json:"audio_channels" cue:"audio.channels"`

	// Subtitle settings
	BurnInCodec string `json:"burn_in_codec"`
	SoftCodec   string `json:"soft_codec"`

	// Performance settings
	MaxConcurrentJobs int  `json:"max_concurrent_jobs" cue:"performance.max_concurrent_jobs"`
	TimeoutSeconds    int  `json:"timeout_seconds" cue:"performance.timeout_seconds"`
	CleanupOnExit     bool `json:"cleanup_on_exit" cue:"performance.cleanup_on_exit"`

	// File cleanup settings
	FileRetentionHours     int `json:"file_retention_hours" cue:"cleanup.file_retention_hours"`
	ExtendedRetentionHours int `json:"extended_retention_hours" cue:"cleanup.extended_retention_hours"`
	MaxSizeLimitGB         int `json:"max_size_limit_gb" cue:"cleanup.max_size_limit_gb"`
	LargeFileSizeMB        int `json:"large_file_size_mb" cue:"cleanup.large_file_size_mb"`

	// Logging
	LogLevel     string `json:"log_level" cue:"logging.level"`
	FFmpegOutput bool   `json:"ffmpeg_output" cue:"logging.ffmpeg_output"`
}

// DefaultFFmpegConfig returns the default configuration
func DefaultFFmpegConfig() *FFmpegConfig {
	return &FFmpegConfig{
		Enabled:                true,
		FFmpegPath:             "ffmpeg",
		Preset:                 "fast",
		Threads:                0, // Auto
		Priority:               50,
		CRFH264:                23,
		CRFHEVC:                28,
		MaxBitrateMultiplier:   1.5,
		BufferSizeMultiplier:   2.0,
		AudioCodec:             "aac",
		AudioBitrate:           128,
		AudioSampleRate:        44100,
		AudioChannels:          2,
		BurnInCodec:            "subtitles",
		SoftCodec:              "mov_text",
		MaxConcurrentJobs:      25,
		TimeoutSeconds:         3600, // 1 hour
		CleanupOnExit:          true,
		FileRetentionHours:     2,   // Keep files for 2 hours (active window)
		ExtendedRetentionHours: 8,   // Keep smaller files for 8 hours
		MaxSizeLimitGB:         10,  // Emergency cleanup above 10GB
		LargeFileSizeMB:        500, // Files larger than 500MB are considered large
		LogLevel:               "info",
		FFmpegOutput:           false,
	}
}

// FFmpegConfigurationService manages plugin configuration
type FFmpegConfigurationService struct {
	context *plugins.PluginContext
	logger  plugins.Logger
	config  *FFmpegConfig
}

// NewFFmpegConfigurationService creates a new configuration service
func NewFFmpegConfigurationService(ctx *plugins.PluginContext, logger plugins.Logger) (*FFmpegConfigurationService, error) {
	service := &FFmpegConfigurationService{
		context: ctx,
		logger:  logger,
		config:  DefaultFFmpegConfig(),
	}

	// Load configuration from CUE file
	if err := service.loadConfiguration(); err != nil {
		logger.Warn("failed to load configuration, using defaults", "error", err)
	}

	return service, nil
}

// GetConfiguration returns the current plugin configuration
func (s *FFmpegConfigurationService) GetConfiguration(ctx context.Context) (*plugins.PluginConfiguration, error) {
	return &plugins.PluginConfiguration{
		Version:  "1.0",
		Enabled:  s.config.Enabled,
		Settings: s.configToMap(),
	}, nil
}

// UpdateConfiguration updates plugin configuration at runtime
func (s *FFmpegConfigurationService) UpdateConfiguration(ctx context.Context, config *plugins.PluginConfiguration) error {
	if err := s.validateConfiguration(config); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Update internal configuration
	if err := s.mapToConfig(config.Settings); err != nil {
		return fmt.Errorf("failed to update configuration: %w", err)
	}

	s.logger.Info("configuration updated successfully")
	return nil
}

// ReloadConfiguration reloads configuration from source
func (s *FFmpegConfigurationService) ReloadConfiguration(ctx context.Context) error {
	return s.loadConfiguration()
}

// ValidateConfiguration validates a configuration before applying
func (s *FFmpegConfigurationService) ValidateConfiguration(config *plugins.PluginConfiguration) (*plugins.ValidationResult, error) {
	result := &plugins.ValidationResult{
		Valid:    true,
		Errors:   []string{},
		Warnings: []string{},
	}

	if err := s.validateConfiguration(config); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, err.Error())
	}

	return result, nil
}

// GetConfigurationSchema returns the JSON schema for this plugin's configuration
func (s *FFmpegConfigurationService) GetConfigurationSchema() (*plugins.ConfigurationSchema, error) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"enabled": map[string]interface{}{
				"type":        "boolean",
				"description": "Enable or disable the FFmpeg transcoder",
				"default":     true,
			},
			"ffmpeg_path": map[string]interface{}{
				"type":        "string",
				"description": "Path to FFmpeg executable",
				"default":     "ffmpeg",
			},
			"preset": map[string]interface{}{
				"type":        "string",
				"description": "FFmpeg encoding preset",
				"enum":        []string{"ultrafast", "superfast", "veryfast", "faster", "fast", "medium", "slow", "slower", "veryslow"},
				"default":     "fast",
			},
			"max_concurrent_jobs": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of concurrent transcoding jobs",
				"minimum":     1,
				"maximum":     15,
				"default":     5,
			},
		},
		"required": []string{"enabled"},
	}

	return &plugins.ConfigurationSchema{
		Schema:   schema,
		Examples: map[string]interface{}{},
		Defaults: s.configToMap(),
	}, nil
}

// GetFFmpegConfig returns the current FFmpeg configuration
func (s *FFmpegConfigurationService) GetFFmpegConfig() *FFmpegConfig {
	return s.config
}

// Helper methods

func (s *FFmpegConfigurationService) loadConfiguration() error {
	// This would load from the CUE file using the plugin SDK
	// For now, we'll use defaults
	s.logger.Info("loading configuration from CUE file")
	return nil
}

func (s *FFmpegConfigurationService) validateConfiguration(config *plugins.PluginConfiguration) error {
	if config == nil {
		return fmt.Errorf("configuration is nil")
	}

	// Validate specific settings
	if settings := config.Settings; settings != nil {
		if enabled, ok := settings["enabled"].(bool); ok && !enabled {
			s.logger.Info("plugin is disabled in configuration")
		}

		if maxJobs, ok := settings["max_concurrent_jobs"].(float64); ok {
			if maxJobs < 1 || maxJobs > 15 {
				return fmt.Errorf("max_concurrent_jobs must be between 1 and 15")
			}
		}

		if preset, ok := settings["preset"].(string); ok {
			validPresets := []string{"ultrafast", "superfast", "veryfast", "faster", "fast", "medium", "slow", "slower", "veryslow"}
			valid := false
			for _, validPreset := range validPresets {
				if preset == validPreset {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("invalid preset: %s", preset)
			}
		}
	}

	return nil
}

func (s *FFmpegConfigurationService) configToMap() map[string]interface{} {
	return map[string]interface{}{
		"enabled":                s.config.Enabled,
		"ffmpeg_path":            s.config.FFmpegPath,
		"preset":                 s.config.Preset,
		"threads":                s.config.Threads,
		"priority":               s.config.Priority,
		"crf_h264":               s.config.CRFH264,
		"crf_hevc":               s.config.CRFHEVC,
		"max_bitrate_multiplier": s.config.MaxBitrateMultiplier,
		"buffer_size_multiplier": s.config.BufferSizeMultiplier,
		"audio_codec":            s.config.AudioCodec,
		"audio_bitrate":          s.config.AudioBitrate,
		"audio_sample_rate":      s.config.AudioSampleRate,
		"audio_channels":         s.config.AudioChannels,
		"burn_in_codec":          s.config.BurnInCodec,
		"soft_codec":             s.config.SoftCodec,
		"max_concurrent_jobs":    s.config.MaxConcurrentJobs,
		"timeout_seconds":        s.config.TimeoutSeconds,
		"cleanup_on_exit":        s.config.CleanupOnExit,
		"log_level":              s.config.LogLevel,
		"ffmpeg_output":          s.config.FFmpegOutput,
	}
}

func (s *FFmpegConfigurationService) mapToConfig(settings map[string]interface{}) error {
	if enabled, ok := settings["enabled"].(bool); ok {
		s.config.Enabled = enabled
	}
	if ffmpegPath, ok := settings["ffmpeg_path"].(string); ok {
		s.config.FFmpegPath = ffmpegPath
	}
	if preset, ok := settings["preset"].(string); ok {
		s.config.Preset = preset
	}
	if threads, ok := settings["threads"].(float64); ok {
		s.config.Threads = int(threads)
	}
	if priority, ok := settings["priority"].(float64); ok {
		s.config.Priority = int(priority)
	}
	if maxJobs, ok := settings["max_concurrent_jobs"].(float64); ok {
		s.config.MaxConcurrentJobs = int(maxJobs)
	}
	if timeout, ok := settings["timeout_seconds"].(float64); ok {
		s.config.TimeoutSeconds = int(timeout)
	}
	if cleanup, ok := settings["cleanup_on_exit"].(bool); ok {
		s.config.CleanupOnExit = cleanup
	}
	if logLevel, ok := settings["log_level"].(string); ok {
		s.config.LogLevel = logLevel
	}
	if ffmpegOutput, ok := settings["ffmpeg_output"].(bool); ok {
		s.config.FFmpegOutput = ffmpegOutput
	}

	return nil
}
