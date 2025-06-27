// Package transcoding provides a lightweight SDK for building transcoding plugins.
// This package contains only the essential interfaces and types needed by plugins.
//
// Plugins should implement the TranscodingProvider interface and embed or extend
// the Transcoder struct. The actual transcoding implementation is handled by
// the Viewra transcoding module - this SDK just provides the plugin interface.
//
// Example usage:
//   type MyTranscodingPlugin struct {
//       *transcoding.Transcoder
//       // plugin-specific fields
//   }
//
//   func (p *MyTranscodingPlugin) StartTranscode(ctx context.Context, req TranscodeRequest) (*TranscodeHandle, error) {
//       // plugin implementation
//   }
package transcoding

import (
	"context"
	"fmt"
	"io"
)

// Transcoder provides a base implementation that plugins can embed.
// It handles the basic provider interface and delegates to the actual
// implementation when available.
type Transcoder struct {
	// Plugin information
	name        string
	description string
	version     string
	author      string
	priority    int
	
	// The actual implementation will be injected from the module
	impl TranscodingProvider
}

// NewTranscoder creates a new transcoder for plugin use
func NewTranscoder(name, description, version, author string, priority int) *Transcoder {
	return &Transcoder{
		name:        name,
		description: description,
		version:     version,
		author:      author,
		priority:    priority,
	}
}

// SetLogger is kept for compatibility but does nothing as logging is handled by the module
func (t *Transcoder) SetLogger(logger Logger) {
	// No-op - logging is handled by the module implementation
}

// SetImplementation sets the actual provider implementation from the module
func (t *Transcoder) SetImplementation(impl TranscodingProvider) {
	t.impl = impl
}

// GetInfo returns provider information
func (t *Transcoder) GetInfo() ProviderInfo {
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
func (t *Transcoder) GetSupportedFormats() []ContainerFormat {
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
func (t *Transcoder) StartTranscode(ctx context.Context, req TranscodeRequest) (*TranscodeHandle, error) {
	if t.impl == nil {
		return nil, fmt.Errorf("no implementation set - module not initialized")
	}
	return t.impl.StartTranscode(ctx, req)
}

// GetProgress delegates to implementation
func (t *Transcoder) GetProgress(handle *TranscodeHandle) (*TranscodingProgress, error) {
	if t.impl == nil {
		return nil, fmt.Errorf("no implementation set - module not initialized")
	}
	return t.impl.GetProgress(handle)
}

// StopTranscode delegates to implementation
func (t *Transcoder) StopTranscode(handle *TranscodeHandle) error {
	if t.impl == nil {
		return fmt.Errorf("no implementation set - module not initialized")
	}
	return t.impl.StopTranscode(handle)
}

// StartStream delegates to implementation
func (t *Transcoder) StartStream(ctx context.Context, req TranscodeRequest) (*StreamHandle, error) {
	if t.impl == nil {
		return nil, fmt.Errorf("no implementation set - module not initialized")
	}
	return t.impl.StartStream(ctx, req)
}

// GetStream delegates to implementation
func (t *Transcoder) GetStream(handle *StreamHandle) (io.ReadCloser, error) {
	if t.impl == nil {
		return nil, fmt.Errorf("no implementation set - module not initialized")
	}
	return t.impl.GetStream(handle)
}

// StopStream delegates to implementation
func (t *Transcoder) StopStream(handle *StreamHandle) error {
	if t.impl == nil {
		return fmt.Errorf("no implementation set - module not initialized")
	}
	return t.impl.StopStream(handle)
}

// GetDashboardSections delegates to implementation
func (t *Transcoder) GetDashboardSections() []DashboardSection {
	if t.impl != nil {
		return t.impl.GetDashboardSections()
	}
	return []DashboardSection{}
}

// GetDashboardData delegates to implementation
func (t *Transcoder) GetDashboardData(sectionID string) (interface{}, error) {
	if t.impl == nil {
		return nil, fmt.Errorf("no implementation set - module not initialized")
	}
	return t.impl.GetDashboardData(sectionID)
}

// ExecuteDashboardAction delegates to implementation
func (t *Transcoder) ExecuteDashboardAction(actionID string, params map[string]interface{}) error {
	if t.impl == nil {
		return fmt.Errorf("no implementation set - module not initialized")
	}
	return t.impl.ExecuteDashboardAction(actionID, params)
}