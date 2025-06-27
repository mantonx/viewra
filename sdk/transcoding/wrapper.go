// Package transcoding provides a thin wrapper around the transcoding module.
// The actual implementation has been moved to internal/modules/transcodingmodule
// to keep the SDK lightweight and focused on interfaces.
package transcoding

import (
	"context"
	"fmt"
	"io"
)

// TranscoderWrapper is a thin wrapper that delegates to the transcoding module.
// This replaces the heavy implementation that was previously in the SDK.
type TranscoderWrapper struct {
	name        string
	description string
	version     string
	author      string
	priority    int
	
	// The actual implementation will be injected from the transcoding module
	impl TranscodingProvider
}

// NewTranscoderWrapper creates a new thin wrapper
func NewTranscoderWrapper(name, description, version, author string, priority int) *TranscoderWrapper {
	return &TranscoderWrapper{
		name:        name,
		description: description,
		version:     version,
		author:      author,
		priority:    priority,
	}
}

// SetImplementation sets the actual implementation from the transcoding module
func (t *TranscoderWrapper) SetImplementation(impl TranscodingProvider) {
	t.impl = impl
}

// GetInfo returns provider information
func (t *TranscoderWrapper) GetInfo() ProviderInfo {
	if t.impl != nil {
		return t.impl.GetInfo()
	}
	return ProviderInfo{
		ID:          t.name,
		Name:        t.name,
		Description: t.description,
		Version:     t.version,
		Author:      t.author,
		Priority:    t.priority,
	}
}

// GetSupportedFormats delegates to implementation
func (t *TranscoderWrapper) GetSupportedFormats() []ContainerFormat {
	if t.impl != nil {
		return t.impl.GetSupportedFormats()
	}
	// Basic fallback formats
	return []ContainerFormat{
		{Format: "mp4", MimeType: "video/mp4", Extensions: []string{".mp4"}, Description: "MPEG-4 Container"},
		{Format: "webm", MimeType: "video/webm", Extensions: []string{".webm"}, Description: "WebM Container"},
	}
}

// StartTranscode delegates to implementation
func (t *TranscoderWrapper) StartTranscode(ctx context.Context, req TranscodeRequest) (*TranscodeHandle, error) {
	if t.impl == nil {
		return nil, fmt.Errorf("no implementation set - module not initialized")
	}
	return t.impl.StartTranscode(ctx, req)
}

// GetProgress delegates to implementation
func (t *TranscoderWrapper) GetProgress(handle *TranscodeHandle) (*TranscodingProgress, error) {
	if t.impl == nil {
		return nil, fmt.Errorf("no implementation set - module not initialized")
	}
	return t.impl.GetProgress(handle)
}

// StopTranscode delegates to implementation
func (t *TranscoderWrapper) StopTranscode(handle *TranscodeHandle) error {
	if t.impl == nil {
		return fmt.Errorf("no implementation set - module not initialized")
	}
	return t.impl.StopTranscode(handle)
}

// StartStream delegates to implementation
func (t *TranscoderWrapper) StartStream(ctx context.Context, req TranscodeRequest) (*StreamHandle, error) {
	if t.impl == nil {
		return nil, fmt.Errorf("no implementation set - module not initialized")
	}
	return t.impl.StartStream(ctx, req)
}

// GetStream delegates to implementation
func (t *TranscoderWrapper) GetStream(handle *StreamHandle) (io.ReadCloser, error) {
	if t.impl == nil {
		return nil, fmt.Errorf("no implementation set - module not initialized")
	}
	return t.impl.GetStream(handle)
}

// StopStream delegates to implementation
func (t *TranscoderWrapper) StopStream(handle *StreamHandle) error {
	if t.impl == nil {
		return fmt.Errorf("no implementation set - module not initialized")
	}
	return t.impl.StopStream(handle)
}

// GetDashboardSections delegates to implementation
func (t *TranscoderWrapper) GetDashboardSections() []DashboardSection {
	if t.impl != nil {
		return t.impl.GetDashboardSections()
	}
	return []DashboardSection{}
}

// GetDashboardData delegates to implementation
func (t *TranscoderWrapper) GetDashboardData(sectionID string) (interface{}, error) {
	if t.impl == nil {
		return nil, fmt.Errorf("no implementation set - module not initialized")
	}
	return t.impl.GetDashboardData(sectionID)
}

// ExecuteDashboardAction delegates to implementation
func (t *TranscoderWrapper) ExecuteDashboardAction(actionID string, params map[string]interface{}) error {
	if t.impl == nil {
		return fmt.Errorf("no implementation set - module not initialized")
	}
	return t.impl.ExecuteDashboardAction(actionID, params)
}