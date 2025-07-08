// Package types provides types and interfaces for the transcoding module.
package types

import (
	"context"

	plugins "github.com/mantonx/viewra/sdk"
)

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