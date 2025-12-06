// Package transcoding provides application-level transcoding configuration.
package transcoding

import (
	"context"

	"github.com/mantonx/viewra/internal/application/settings"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding"
)

// SettingsConfigProvider provides transcoding configuration from the settings service.
// This enables runtime configuration changes without server restart for settings like
// tone mapping enabled/algorithm, while respecting environment variable overrides.
type SettingsConfigProvider struct {
	settingsService *settings.Service
	baseConfig      *transcoding.TranscodeConfig
}

// NewSettingsConfigProvider creates a provider that reads from settings service.
// baseConfig provides default values and non-settings-controlled fields (like hardware accel).
func NewSettingsConfigProvider(
	settingsService *settings.Service,
	baseConfig *transcoding.TranscodeConfig,
) *SettingsConfigProvider {
	return &SettingsConfigProvider{
		settingsService: settingsService,
		baseConfig:      baseConfig,
	}
}

// GetConfig returns current transcoding configuration, merging settings values with base config.
// Settings that can be changed at runtime:
// - transcoding.tone_mapping_enabled
// - transcoding.tone_mapping_algorithm
// - transcoding.min_free_disk_gb
//
// Settings that require restart (use base config):
// - Hardware acceleration (determined at startup)
// - Workers count (queue worker pool size)
func (p *SettingsConfigProvider) GetConfig(ctx context.Context) *transcoding.TranscodeConfig {
	// Start with a copy of base config
	config := &transcoding.TranscodeConfig{
		FFmpegPath:                 p.baseConfig.FFmpegPath,
		FFprobePath:                p.baseConfig.FFprobePath,
		FFmpegLibPath:              p.baseConfig.FFmpegLibPath,
		HardwareAccel:              p.baseConfig.HardwareAccel,
		HardwareDevice:             p.baseConfig.HardwareDevice,
		OutputBaseDir:              p.baseConfig.OutputBaseDir,
		MinFreeDiskGB:              p.baseConfig.MinFreeDiskGB,
		MaxCPUPercent:              p.baseConfig.MaxCPUPercent,
		MaxMemoryMB:                p.baseConfig.MaxMemoryMB,
		ProcessGroupKill:           p.baseConfig.ProcessGroupKill,
		ToneMappingEnabled:         p.baseConfig.ToneMappingEnabled,
		ToneMappingAlgorithm:       p.baseConfig.ToneMappingAlgorithm,
		ToneMappingBackend:         p.baseConfig.ToneMappingBackend,
		LibPlaceboPeakDetect:       p.baseConfig.LibPlaceboPeakDetect,
		LibPlaceboContrastRecovery: p.baseConfig.LibPlaceboContrastRecovery,
		FFmpegLogEnabled:           p.baseConfig.FFmpegLogEnabled,
		FFmpegLogRetentionHours:    p.baseConfig.FFmpegLogRetentionHours,
	}

	// If no settings service, return base config
	if p.settingsService == nil {
		return config
	}

	// Override with settings values (respects env var > database > default priority)
	// These settings take effect immediately without restart

	// Tone mapping enabled
	if effectiveValue, err := p.settingsService.GetEffectiveSystemValue(ctx, "transcoding.tone_mapping_enabled"); err == nil {
		if enabled, ok := effectiveValue.Value.(bool); ok {
			config.ToneMappingEnabled = enabled
		}
	}

	// Tone mapping algorithm
	if effectiveValue, err := p.settingsService.GetEffectiveSystemValue(ctx, "transcoding.tone_mapping_algorithm"); err == nil {
		if algorithm, ok := effectiveValue.Value.(string); ok && algorithm != "" {
			config.ToneMappingAlgorithm = algorithm
		}
	}

	// Min free disk GB
	if effectiveValue, err := p.settingsService.GetEffectiveSystemValue(ctx, "transcoding.min_free_disk_gb"); err == nil {
		if minDisk, ok := effectiveValue.Value.(int); ok {
			config.MinFreeDiskGB = int64(minDisk)
		}
	}

	return config
}
