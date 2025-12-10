package transcode

import (
	"context"

	"github.com/mantonx/viewra/internal/application/settings"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/config"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/session"
)

// SettingsConfigProvider provides transcoding configuration from the settings service.
// This enables runtime configuration changes without server restart for settings like
// tone mapping enabled/algorithm, while respecting environment variable overrides.
//
// Implements session.ConfigProvider interface.
type SettingsConfigProvider struct {
	settingsService *settings.Service
	baseConfig      *config.TranscodeConfig
}

// NewSettingsConfigProvider creates a provider that reads from settings service.
// baseConfig provides default values and non-settings-controlled fields (like hardware accel).
func NewSettingsConfigProvider(
	settingsService *settings.Service,
	baseConfig *config.TranscodeConfig,
) *SettingsConfigProvider {
	return &SettingsConfigProvider{
		settingsService: settingsService,
		baseConfig:      baseConfig,
	}
}

// GetConfig returns current session configuration, merging settings values with base config.
// Implements session.ConfigProvider interface.
//
// Settings that can be changed at runtime:
// - transcoding.tone_mapping_enabled
// - transcoding.tone_mapping_algorithm
//
// Settings that require restart (use base config):
// - Hardware acceleration (determined at startup)
// - Workers count (queue worker pool size)
func (p *SettingsConfigProvider) GetConfig(ctx context.Context) *session.Config {
	// Start with base config values
	cfg := &session.Config{
		FFmpegPaths:                p.baseConfig.FFmpegPaths,
		MaxMemoryMB:                p.baseConfig.MaxMemoryMB,
		ToneMappingEnabled:         p.baseConfig.ToneMappingEnabled,
		ToneMappingAlgorithm:       p.baseConfig.ToneMappingAlgorithm,
		ToneMappingBackend:         p.baseConfig.ToneMappingBackend,
		LibPlaceboPeakDetect:       p.baseConfig.LibPlaceboPeakDetect,
		LibPlaceboContrastRecovery: p.baseConfig.LibPlaceboContrastRecovery,
	}

	// If no settings service, return base config
	if p.settingsService == nil {
		return cfg
	}

	// Override with settings values (respects env var > database > default priority)
	// These settings take effect immediately without restart

	// Tone mapping enabled
	if effectiveValue, err := p.settingsService.GetEffectiveSystemValue(ctx, "transcoding.tone_mapping_enabled"); err == nil {
		if enabled, ok := effectiveValue.Value.(bool); ok {
			cfg.ToneMappingEnabled = enabled
		}
	}

	// Tone mapping algorithm
	if effectiveValue, err := p.settingsService.GetEffectiveSystemValue(ctx, "transcoding.tone_mapping_algorithm"); err == nil {
		if algorithm, ok := effectiveValue.Value.(string); ok && algorithm != "" {
			cfg.ToneMappingAlgorithm = algorithm
		}
	}

	return cfg
}
