// Package pipeline provides a file-based transcoding provider implementation.
// The provider manages file transcoding operations using FFmpeg, offering
// a straightforward approach to media processing without the complexity
// of real-time streaming pipelines.
package pipeline

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/session"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/storage"
	plugins "github.com/mantonx/viewra/sdk"
)

// Provider implements the TranscodingProvider interface with file-based transcoding.
// It manages transcoding sessions, handles content-addressable storage, and
// provides a clean interface for the transcoding module to use.
//
// The provider focuses on reliability and simplicity:
//   - Complete file transcoding (no streaming complexity)
//   - Content deduplication via SHA256 hashes
//   - Session management with database persistence
//   - Progress tracking for long-running operations
type Provider struct {
	baseDir      string
	logger       hclog.Logger
	pipeline     *FilePipeline
	sessionStore *session.SessionStore
	contentStore *storage.ContentStore

	// Active handles mapped by session ID
	handles      map[string]*plugins.TranscodeHandle
	handlesMutex sync.RWMutex
}

// NewProvider creates a new file pipeline provider with all required dependencies.
//
// Parameters:
//   - baseDir: Base directory for temporary transcoding files and output
//   - sessionStore: Manages transcoding session persistence
//   - contentStore: Handles content-addressable storage
//   - logger: Logger instance for this provider
//
// The provider will create necessary subdirectories under baseDir:
//   - sessions/: Temporary files during transcoding
//   - content/: Content-addressable storage for completed files
func NewProvider(baseDir string, sessionStore *session.SessionStore, contentStore *storage.ContentStore, logger hclog.Logger) *Provider {
	// Create file pipeline immediately with all dependencies
	pipeline := NewFilePipeline(logger.Named("pipeline"), sessionStore, contentStore, baseDir)

	return &Provider{
		baseDir:      baseDir,
		logger:       logger.Named("file-provider"),
		pipeline:     pipeline,
		sessionStore: sessionStore,
		contentStore: contentStore,
		handles:      make(map[string]*plugins.TranscodeHandle),
	}
}





// GetInfo returns provider information for display and selection.
// The provider has high priority (100) as it's the primary built-in
// transcoding mechanism.
func (p *Provider) GetInfo() plugins.ProviderInfo {
	return plugins.ProviderInfo{
		ID:          "file_pipeline",
		Name:        "File Pipeline Provider",
		Description: "File-based transcoding for complete media files",
		Version:     "1.0.0",
		Author:      "Viewra Team",
		Priority:    100,
	}
}

// GetSupportedFormats returns supported output formats for this provider.
// The file pipeline supports both MP4 and MKV containers with various codecs.
func (p *Provider) GetSupportedFormats() []plugins.ContainerFormat {
	return []plugins.ContainerFormat{
		{
			Format:       "mp4",
			MimeType:     "video/mp4",
			Extensions:   []string{".mp4"},
			Description:  "MP4 container with H.264/H.265 video and AAC audio",
			Adaptive:     false,
			Intermediate: true,
		},
		{
			Format:       "mkv",
			MimeType:     "video/x-matroska",
			Extensions:   []string{".mkv"},
			Description:  "Matroska container with various codecs",
			Adaptive:     false,
			Intermediate: true,
		},
	}
}

// GetHardwareAccelerators returns available hardware acceleration options.
// Currently returns an empty slice as this provider uses software encoding.
func (p *Provider) GetHardwareAccelerators() []plugins.HardwareAccelerator {
	return []plugins.HardwareAccelerator{}
}

// GetQualityPresets returns available quality presets.
func (p *Provider) GetQualityPresets() []plugins.QualityPreset {
	return []plugins.QualityPreset{
		{
			ID:          "fast",
			Name:        "Fast",
			Description: "Fast encoding with good quality",
			Quality:     75,
			SpeedRating: 8,
			SizeRating:  6,
		},
		{
			ID:          "balanced",
			Name:        "Balanced",
			Description: "Balanced encoding speed and quality",
			Quality:     85,
			SpeedRating: 6,
			SizeRating:  7,
		},
		{
			ID:          "quality",
			Name:        "High Quality",
			Description: "Slow encoding with high quality",
			Quality:     95,
			SpeedRating: 3,
			SizeRating:  9,
		},
	}
}

// IsAvailable checks if the provider is available for use.
//
// TODO: Implement actual FFmpeg availability check by:
//   1. Checking if ffmpeg binary exists in PATH
//   2. Verifying minimum version requirements
//   3. Testing basic functionality
//
// For now, this always returns true assuming FFmpeg is installed.
func (p *Provider) IsAvailable() bool {
	// Check if FFmpeg is available
	return true // For now, assume it's available
}

// Transcode starts a new transcoding session using the file pipeline.
//
// The method:
//   1. Delegates to FilePipeline for actual transcoding
//   2. Stores the handle for later retrieval
//   3. Returns immediately while transcoding runs in background
//
// Parameters:
//   - ctx: Context for cancellation (passed to FFmpeg)
//   - req: Transcoding parameters including input path, codecs, etc.
//
// Returns:
//   - TranscodeHandle for progress tracking and control
//   - Error if request invalid
func (p *Provider) Transcode(ctx context.Context, req plugins.TranscodeRequest) (*plugins.TranscodeHandle, error) {
	// Use the file pipeline (always initialized in constructor)
	handle, err := p.pipeline.Transcode(ctx, req)
	if err != nil {
		return nil, err
	}

	// Store handle
	p.handlesMutex.Lock()
	p.handles[handle.SessionID] = handle
	p.handlesMutex.Unlock()

	return handle, nil
}

// GetProgress returns the current progress of a transcoding session.
//
// For file-based transcoding, progress is estimated based on status:
//   - completed: 100%
//   - running: 50% (estimate until real progress parsing is implemented)
//   - other states: 0%
//
// Parameters:
//   - handle: The transcoding handle to query
//
// Returns:
//   - Progress information including percentage and timestamps
//   - Error if session not found
func (p *Provider) GetProgress(handle *plugins.TranscodeHandle) (*plugins.TranscodingProgress, error) {
	return p.pipeline.GetProgress(handle.SessionID)
}

// Stop stops an active transcoding session.
//
// This method:
//   1. Removes the session handle from memory
//   2. Delegates to FilePipeline to stop FFmpeg process
//   3. Updates session status in database
//
// Parameters:
//   - sessionID: The session to stop
//
// Returns:
//   - Error if session not found or stop failed
//
// Note: Temporary files are cleaned up by the cleanup service.
func (p *Provider) Stop(sessionID string) error {
	// Remove handle
	p.handlesMutex.Lock()
	delete(p.handles, sessionID)
	p.handlesMutex.Unlock()

	return p.pipeline.Stop(sessionID)
}

// Cleanup cleans up provider resources.
//
// For the file pipeline provider, cleanup is handled by:
//   - Individual session cleanup on completion
//   - Global cleanup service for orphaned files
//   - Content store expiration policies
//
// This method is a no-op but exists to satisfy the interface.
func (p *Provider) Cleanup() error {
	// Nothing to clean up for file pipeline
	return nil
}

// GetHandle returns a transcoding handle by session ID.
// This is used internally to retrieve handles for active sessions.
//
// Parameters:
//   - sessionID: The session handle to retrieve
//
// Returns:
//   - The transcoding handle if found
//   - Boolean indicating if handle exists
func (p *Provider) GetHandle(sessionID string) (*plugins.TranscodeHandle, bool) {
	p.handlesMutex.RLock()
	defer p.handlesMutex.RUnlock()
	
	handle, exists := p.handles[sessionID]
	return handle, exists
}

// Required interface methods for TranscodingProvider

// SupportsIntermediateOutput indicates if the provider outputs intermediate files
func (p *Provider) SupportsIntermediateOutput() bool {
	return true
}

// GetIntermediateOutputPath returns the path to intermediate output file
func (p *Provider) GetIntermediateOutputPath(handle *plugins.TranscodeHandle) (string, error) {
	sess, err := p.sessionStore.GetSession(handle.SessionID)
	if err != nil {
		return "", err
	}
	
	return sess.DirectoryPath, nil
}

// GetABRVariants returns ABR encoding variants (not used by file pipeline)
func (p *Provider) GetABRVariants(req plugins.TranscodeRequest) ([]plugins.ABRVariant, error) {
	return []plugins.ABRVariant{}, nil
}

// StartTranscode starts a new transcoding operation
func (p *Provider) StartTranscode(ctx context.Context, req plugins.TranscodeRequest) (*plugins.TranscodeHandle, error) {
	return p.Transcode(ctx, req)
}

// GetProgressBySessionID returns progress for a session ID (internal method)
func (p *Provider) GetProgressBySessionID(sessionID string) (*plugins.TranscodingProgress, error) {
	return p.pipeline.GetProgress(sessionID)
}

// StopTranscode stops a transcoding operation
func (p *Provider) StopTranscode(handle *plugins.TranscodeHandle) error {
	return p.Stop(handle.SessionID)
}

// StartStream starts a streaming session (not implemented for file pipeline)
func (p *Provider) StartStream(ctx context.Context, req plugins.TranscodeRequest) (*plugins.StreamHandle, error) {
	return nil, fmt.Errorf("streaming not supported by file pipeline")
}

// GetStream returns a stream reader (not implemented for file pipeline)
func (p *Provider) GetStream(handle *plugins.StreamHandle) (io.ReadCloser, error) {
	return nil, fmt.Errorf("streaming not supported by file pipeline")
}

// StopStream stops a streaming session (not implemented for file pipeline)
func (p *Provider) StopStream(handle *plugins.StreamHandle) error {
	return fmt.Errorf("streaming not supported by file pipeline")
}

// GetDashboardSections returns dashboard sections for this provider
func (p *Provider) GetDashboardSections() []plugins.DashboardSection {
	return []plugins.DashboardSection{
		{
			ID:          "file_pipeline_status",
			Title:       "File Pipeline Status",
			Type:        "status",
			Description: "Current status of file-based transcoding",
		},
	}
}

// GetDashboardData returns data for a dashboard section
func (p *Provider) GetDashboardData(sectionID string) (interface{}, error) {
	switch sectionID {
	case "file_pipeline_status":
		return map[string]interface{}{
			"active_sessions": len(p.handles),
			"provider_status": "active",
		}, nil
	default:
		return nil, fmt.Errorf("unknown section: %s", sectionID)
	}
}

// ExecuteDashboardAction executes a dashboard action
func (p *Provider) ExecuteDashboardAction(actionID string, params map[string]interface{}) error {
	return fmt.Errorf("no actions supported")
}