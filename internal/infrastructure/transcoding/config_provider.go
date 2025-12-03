package transcoding

import (
	"context"
)

// ConfigProvider provides transcoding configuration, potentially dynamically from settings.
// This interface allows the SessionManager to fetch fresh configuration before each transcode,
// enabling runtime setting changes without server restart.
type ConfigProvider interface {
	// GetConfig returns the current transcoding configuration.
	// Implementations may read from settings service, environment, or return static config.
	GetConfig(ctx context.Context) *TranscodeConfig
}

// StaticConfigProvider provides a fixed configuration.
// Used when settings service is not available or for testing.
type StaticConfigProvider struct {
	config *TranscodeConfig
}

// NewStaticConfigProvider creates a provider that always returns the same config.
func NewStaticConfigProvider(config *TranscodeConfig) *StaticConfigProvider {
	return &StaticConfigProvider{config: config}
}

// GetConfig returns the static configuration.
func (p *StaticConfigProvider) GetConfig(_ context.Context) *TranscodeConfig {
	return p.config
}
