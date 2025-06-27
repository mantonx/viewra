// Package pipeline provides a streaming-first transcoding provider implementation.
// This provider implements the TranscodingProvider interface and uses the StreamingPipeline
// for real-time segment-based transcoding.
package pipeline

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/session"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/storage"
	plugins "github.com/mantonx/viewra/sdk"
)

// Provider implements the TranscodingProvider interface with streaming-first architecture
type Provider struct {
	baseDir      string
	logger       hclog.Logger
	pipeline     *StreamingPipeline
	sessionStore *session.SessionStore
	contentStore *storage.ContentStore

	// Callbacks
	contentHashCallback func(sessionID, contentHash string) error

	// Active handles mapped by session ID
	handles      map[string]*plugins.TranscodeHandle
	handlesMutex sync.RWMutex
}

// NewProvider creates a new streaming pipeline provider
func NewProvider(baseDir string) *Provider {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "streaming-provider",
		Level: hclog.Info,
	})

	return &Provider{
		baseDir: baseDir,
		logger:  logger,
		handles: make(map[string]*plugins.TranscodeHandle),
	}
}

// NewProviderWithCallback creates a provider with a content hash callback
func NewProviderWithCallback(baseDir string, callback func(string, string, string)) *Provider {
	p := NewProvider(baseDir)
	// Wrap the callback to match our internal signature
	p.contentHashCallback = func(sessionID, contentHash string) error {
		// Generate content URL
		contentURL := fmt.Sprintf("/api/v1/content/%s/", contentHash)
		callback(sessionID, contentHash, contentURL)
		return nil
	}
	return p
}

// Initialize sets up the provider with required services
func (p *Provider) Initialize(sessionStore *session.SessionStore, contentStore *storage.ContentStore) {
	p.sessionStore = sessionStore
	p.contentStore = contentStore

	// Create streaming pipeline with configuration
	config := &StreamingConfig{
		BaseDir:                p.baseDir,
		SegmentDuration:        4,  // 4 second segments for balance between latency and efficiency
		BufferAhead:            12, // 12 seconds of buffer
		ManifestUpdateInterval: 2 * time.Second,
		EnableABR:              true,
		ABRProfiles: []EncodingProfile{
			{Name: "360p", Width: 640, Height: 360, VideoBitrate: 800, Quality: 28},
			{Name: "720p", Width: 1280, Height: 720, VideoBitrate: 2500, Quality: 25},
			{Name: "1080p", Width: 1920, Height: 1080, VideoBitrate: 5000, Quality: 23},
		},
	}

	p.pipeline = NewStreamingPipeline(p.logger, sessionStore, contentStore, config)

	// Set callbacks for pipeline events
	p.pipeline.SetCallbacks(
		p.onSegmentReady,
		p.onManifestUpdate,
		p.onContentComplete,
	)
}

// GetInfo returns provider information
func (p *Provider) GetInfo() plugins.ProviderInfo {
	return plugins.ProviderInfo{
		ID:          "streaming_pipeline",
		Name:        "Streaming Pipeline Provider",
		Description: "Real-time segment-based transcoding with instant playback",
		Version:     "2.0.0",
		Author:      "Viewra Team",
		Priority:    100, // High priority for streaming formats
	}
}

// GetSupportedFormats returns supported container formats
func (p *Provider) GetSupportedFormats() []plugins.ContainerFormat {
	return []plugins.ContainerFormat{
		{
			Format:      "dash",
			MimeType:    "application/dash+xml",
			Extensions:  []string{".mpd"},
			Description: "MPEG-DASH streaming with real-time segments",
			Adaptive:    true,
		},
		{
			Format:      "hls",
			MimeType:    "application/vnd.apple.mpegurl",
			Extensions:  []string{".m3u8"},
			Description: "HLS streaming with real-time segments",
			Adaptive:    true,
		},
	}
}

// StartTranscode starts a new transcoding operation
func (p *Provider) StartTranscode(ctx context.Context, req plugins.TranscodeRequest) (*plugins.TranscodeHandle, error) {
	// Validate request
	if req.Container != "dash" && req.Container != "hls" {
		return nil, fmt.Errorf("unsupported container for streaming: %s", req.Container)
	}

	// Start streaming pipeline
	handle, err := p.pipeline.StartStreaming(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to start streaming: %w", err)
	}

	// Store handle
	p.handlesMutex.Lock()
	p.handles[handle.SessionID] = handle
	p.handlesMutex.Unlock()

	return handle, nil
}

// GetProgress returns the progress of a transcoding operation
func (p *Provider) GetProgress(handle *plugins.TranscodeHandle) (*plugins.TranscodingProgress, error) {
	return p.pipeline.GetProgress(handle.SessionID)
}

// StopTranscode stops a transcoding operation
func (p *Provider) StopTranscode(handle *plugins.TranscodeHandle) error {
	err := p.pipeline.StopStreaming(handle.SessionID)

	// Remove handle
	p.handlesMutex.Lock()
	delete(p.handles, handle.SessionID)
	p.handlesMutex.Unlock()

	return err
}

// StartStream starts a streaming session
func (p *Provider) StartStream(ctx context.Context, req plugins.TranscodeRequest) (*plugins.StreamHandle, error) {
	// For this provider, streaming is the default mode
	// Convert TranscodeHandle to StreamHandle
	handle, err := p.StartTranscode(ctx, req)
	if err != nil {
		return nil, err
	}

	return &plugins.StreamHandle{
		SessionID:   handle.SessionID,
		Provider:    handle.Provider,
		StartTime:   handle.StartTime,
		Context:     handle.Context,
		CancelFunc:  handle.CancelFunc,
		PrivateData: handle.PrivateData,
		Status:      handle.Status,
		Error:       handle.Error,
	}, nil
}

// GetStream returns a stream reader
func (p *Provider) GetStream(handle *plugins.StreamHandle) (io.ReadCloser, error) {
	// Get streaming status to find manifest path
	status, err := p.pipeline.GetStreamingStatus(handle.SessionID)
	if err != nil {
		return nil, err
	}

	// For streaming, we return the manifest file
	manifestPath := filepath.Join(p.baseDir, "sessions", handle.SessionID,
		fmt.Sprintf("stream.%s", getManifestExtension(status.ManifestURL)))

	return &streamReader{
		manifestPath: manifestPath,
		sessionID:    handle.SessionID,
		pipeline:     p.pipeline,
	}, nil
}

// StopStream stops a streaming session
func (p *Provider) StopStream(handle *plugins.StreamHandle) error {
	return p.pipeline.StopStreaming(handle.SessionID)
}

// GetDashboardSections returns dashboard sections for this provider
func (p *Provider) GetDashboardSections() []plugins.DashboardSection {
	return []plugins.DashboardSection{
		{
			ID:          "streaming-overview",
			Title:       "Streaming Pipeline Overview",
			Type:        "stats",
			Description: "Real-time streaming statistics and performance",
		},
		{
			ID:          "active-streams",
			Title:       "Active Streams",
			Type:        "table",
			Description: "Currently active streaming sessions",
		},
		{
			ID:          "buffer-health",
			Title:       "Buffer Health",
			Type:        "chart",
			Description: "Streaming buffer health across sessions",
		},
	}
}

// GetDashboardData returns data for a dashboard section
func (p *Provider) GetDashboardData(sectionID string) (interface{}, error) {
	switch sectionID {
	case "streaming-overview":
		return p.getStreamingOverview()
	case "active-streams":
		return p.getActiveStreams()
	case "buffer-health":
		return p.getBufferHealth()
	default:
		return nil, fmt.Errorf("unknown section: %s", sectionID)
	}
}

// ExecuteDashboardAction executes a dashboard action
func (p *Provider) ExecuteDashboardAction(actionID string, params map[string]interface{}) error {
	switch actionID {
	case "stop-stream":
		sessionID, ok := params["sessionID"].(string)
		if !ok {
			return fmt.Errorf("sessionID required")
		}
		return p.pipeline.StopStreaming(sessionID)
	default:
		return fmt.Errorf("unknown action: %s", actionID)
	}
}

// GetABRVariants returns the ABR encoding variants for a request
func (p *Provider) GetABRVariants(req plugins.TranscodeRequest) ([]plugins.ABRVariant, error) {
	// Return the configured ABR profiles as variants
	if p.pipeline == nil {
		return nil, fmt.Errorf("pipeline not initialized")
	}

	return []plugins.ABRVariant{
		{
			Name:         "360p",
			Resolution:   &plugins.Resolution{Width: 640, Height: 360},
			VideoBitrate: 800,
			AudioBitrate: 96,
			FrameRate:    30,
			Preset:       "fast",
			Profile:      "baseline",
			Level:        "3.0",
		},
		{
			Name:         "720p",
			Resolution:   &plugins.Resolution{Width: 1280, Height: 720},
			VideoBitrate: 2500,
			AudioBitrate: 128,
			FrameRate:    30,
			Preset:       "fast",
			Profile:      "main",
			Level:        "3.1",
		},
		{
			Name:         "1080p",
			Resolution:   &plugins.Resolution{Width: 1920, Height: 1080},
			VideoBitrate: 5000,
			AudioBitrate: 192,
			FrameRate:    30,
			Preset:       "fast",
			Profile:      "high",
			Level:        "4.0",
		},
	}, nil
}

// GetContentStore returns the content store used by this provider
func (p *Provider) GetContentStore() *storage.ContentStore {
	return p.contentStore
}

// GetHardwareAccelerators returns available hardware accelerators
func (p *Provider) GetHardwareAccelerators() []plugins.HardwareAccelerator {
	// Streaming pipeline doesn't use hardware acceleration directly
	// It relies on the underlying FFmpeg configuration
	return []plugins.HardwareAccelerator{} // No hardware accelerators
}

// GetQualityPresets returns available quality presets
func (p *Provider) GetQualityPresets() []plugins.QualityPreset {
	return []plugins.QualityPreset{
		{ID: "low", Name: "Low", Description: "Low quality, fast encoding", Quality: 50, SpeedRating: 9},
		{ID: "medium", Name: "Medium", Description: "Medium quality, balanced", Quality: 75, SpeedRating: 6},
		{ID: "high", Name: "High", Description: "High quality, slower encoding", Quality: 90, SpeedRating: 3},
	}
}

// SupportsIntermediateOutput returns if the provider outputs intermediate files
func (p *Provider) SupportsIntermediateOutput() bool {
	// Streaming pipeline doesn't produce intermediate files
	return false
}

// GetIntermediateOutputPath returns the path to intermediate output
func (p *Provider) GetIntermediateOutputPath(handle *plugins.TranscodeHandle) (string, error) {
	// Streaming pipeline doesn't produce intermediate files
	return "", fmt.Errorf("streaming pipeline does not produce intermediate files")
}

// Callback handlers

func (p *Provider) onSegmentReady(sessionID string, segment SegmentInfo) {
	p.logger.Debug("Segment ready callback",
		"sessionID", sessionID,
		"segment", segment.Index)
}

func (p *Provider) onManifestUpdate(sessionID string, manifestPath string) {
	p.logger.Debug("Manifest update callback",
		"sessionID", sessionID,
		"path", manifestPath)
}

func (p *Provider) onContentComplete(sessionID string, contentHash string) {
	p.logger.Info("Content complete callback",
		"sessionID", sessionID,
		"contentHash", contentHash)

	// Call content hash callback if set
	if p.contentHashCallback != nil {
		if err := p.contentHashCallback(sessionID, contentHash); err != nil {
			p.logger.Error("Content hash callback failed",
				"sessionID", sessionID,
				"error", err)
		}
	}
}

// Dashboard data helpers

func (p *Provider) getStreamingOverview() (interface{}, error) {
	p.handlesMutex.RLock()
	activeCount := len(p.handles)
	p.handlesMutex.RUnlock()

	return map[string]interface{}{
		"active_sessions":  activeCount,
		"segment_duration": 4,
		"buffer_ahead":     12,
		"abr_enabled":      true,
		"profiles":         []string{"360p", "720p", "1080p"},
	}, nil
}

func (p *Provider) getActiveStreams() (interface{}, error) {
	streams := []map[string]interface{}{}

	p.handlesMutex.RLock()
	for sessionID := range p.handles {
		if status, err := p.pipeline.GetStreamingStatus(sessionID); err == nil {
			streams = append(streams, map[string]interface{}{
				"session_id":     sessionID,
				"status":         status.Status,
				"segments_ready": status.SegmentsReady,
				"buffer_health":  status.BufferHealth,
				"startup_time":   status.StartupTime.Seconds(),
			})
		}
	}
	p.handlesMutex.RUnlock()

	return streams, nil
}

func (p *Provider) getBufferHealth() (interface{}, error) {
	healthData := map[string]int{
		"excellent": 0,
		"good":      0,
		"fair":      0,
		"poor":      0,
	}

	p.handlesMutex.RLock()
	for sessionID := range p.handles {
		if status, err := p.pipeline.GetStreamingStatus(sessionID); err == nil {
			healthData[status.BufferHealth]++
		}
	}
	p.handlesMutex.RUnlock()

	return healthData, nil
}

// streamReader implements io.ReadCloser for streaming manifests
type streamReader struct {
	manifestPath string
	sessionID    string
	pipeline     *StreamingPipeline
	file         io.ReadCloser
}

func (r *streamReader) Read(p []byte) (n int, err error) {
	// Lazy open
	if r.file == nil {
		// Wait briefly for manifest to be available
		time.Sleep(100 * time.Millisecond)
		// TODO: Implement proper file watching
		r.file = nil // Placeholder
	}

	if r.file != nil {
		return r.file.Read(p)
	}

	return 0, io.EOF
}

func (r *streamReader) Close() error {
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}

// Helper functions

func getManifestExtension(url string) string {
	if filepath.Ext(url) == ".mpd" {
		return "mpd"
	}
	return "m3u8"
}
