// Package pipeline provides a streaming-first transcoding pipeline.
// This replaces the legacy two-stage approach with real-time segment-based processing.
package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/modules/modulemanager"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/session"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/storage"
	plugins "github.com/mantonx/viewra/sdk"
)

// StreamingPipeline implements real-time transcoding with segment-based processing
type StreamingPipeline struct {
	logger       hclog.Logger
	encoder      *StreamEncoder
	packager     *StreamPackager
	sessionStore *session.SessionStore
	contentStore *storage.ContentStore
	prefetcher   *SegmentPrefetcher

	// Configuration
	config *StreamingConfig

	// Active sessions
	sessions     map[string]*StreamingSession
	sessionMutex sync.RWMutex

	// Health monitoring
	healthMonitor *StreamingHealthMonitor

	// Callbacks
	onSegmentReady    func(sessionID string, segment SegmentInfo)
	onManifestUpdate  func(sessionID string, manifestPath string)
	onContentComplete func(sessionID string, contentHash string)
}

// StreamingConfig contains configuration for the streaming pipeline
type StreamingConfig struct {
	BaseDir                string
	SegmentDuration        int // Seconds per segment (2-4 recommended)
	BufferAhead            int // Seconds to buffer ahead of viewer
	ManifestUpdateInterval time.Duration
	EnableABR              bool
	ABRProfiles            []EncodingProfile
	UseShakaPackager       bool // Enable Shaka Packager for low-latency VOD
}

// StreamingSession represents an active streaming transcoding session
type StreamingSession struct {
	ID           string
	Request      plugins.TranscodeRequest
	Handle       *plugins.TranscodeHandle
	OutputDir    string
	ManifestPath string
	ContentHash  string

	// Streaming state
	SegmentsReady  int
	SegmentsTotal  int
	IsLive         bool
	BufferPosition time.Duration
	ViewerPosition time.Duration

	// Progress tracking
	StartTime        time.Time
	FirstSegmentTime time.Time
	Status           plugins.TranscodeStatus
	Progress         float64

	// Components
	encoder      *StreamEncoder
	shakaEncoder *ShakaStreamEncoder // Alternative Shaka-based encoder
	packager     *StreamPackager
	prefetcher   *SegmentPrefetcher
	// Remove pipeline field - using encoder directly

	// Control
	ctx    context.Context
	cancel context.CancelFunc
}

// NewStreamingPipeline creates a new streaming pipeline
func NewStreamingPipeline(logger hclog.Logger, sessionStore *session.SessionStore, contentStore *storage.ContentStore, config *StreamingConfig) *StreamingPipeline {
	// Create segment prefetcher for intelligent buffering
	prefetcher := NewSegmentPrefetcher(config.BaseDir, logger)

	// Create health monitor
	healthMonitor := NewStreamingHealthMonitor(logger)

	return &StreamingPipeline{
		logger:        logger,
		sessionStore:  sessionStore,
		contentStore:  contentStore,
		prefetcher:    prefetcher,
		healthMonitor: healthMonitor,
		config:        config,
		sessions:      make(map[string]*StreamingSession),
	}
}

// SetCallbacks configures event callbacks
func (p *StreamingPipeline) SetCallbacks(
	onSegment func(string, SegmentInfo),
	onManifest func(string, string),
	onComplete func(string, string),
) {
	p.onSegmentReady = onSegment
	p.onManifestUpdate = onManifest
	p.onContentComplete = onComplete
}

// StartStreaming begins a new streaming transcoding session
func (p *StreamingPipeline) StartStreaming(ctx context.Context, req plugins.TranscodeRequest) (*plugins.TranscodeHandle, error) {
	// Create session in database
	dbSession, err := p.sessionStore.CreateSession("streaming_pipeline", &req)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Create output directory
	outputDir := filepath.Join(p.config.BaseDir, "sessions", dbSession.ID)

	// Create session context
	sessionCtx, cancel := context.WithCancel(ctx)

	// Create handle
	handle := &plugins.TranscodeHandle{
		SessionID:  dbSession.ID,
		Provider:   "streaming_pipeline",
		StartTime:  time.Now(),
		Status:     plugins.TranscodeStatusStarting,
		Directory:  outputDir,
		Context:    sessionCtx,
		CancelFunc: cancel,
	}

	// Register session with health monitor
	if p.healthMonitor != nil {
		p.healthMonitor.RegisterSession(dbSession.ID, dbSession.ContentHash)
	}

	// Create session
	session := &StreamingSession{
		ID:          dbSession.ID,
		Request:     req,
		Handle:      handle,
		OutputDir:   outputDir,
		ContentHash: dbSession.ContentHash,
		IsLive:      true,
		StartTime:   time.Now(),
		Status:      plugins.TranscodeStatusStarting,
		ctx:         sessionCtx,
		cancel:      cancel,
		prefetcher:  p.prefetcher,
	}
	
	// Initialize encoders based on configuration
	if p.config.UseShakaPackager {
		// Create Shaka encoder for low-latency VOD
		session.shakaEncoder = NewShakaStreamEncoder(outputDir, p.config.SegmentDuration)
		session.shakaEncoder.SetCallbacks(
			func(segmentPath string, index int) {
				p.onSegmentReady(session.ID, SegmentInfo{
					Path:  segmentPath,
					Index: index,
					Type:  "video",
				})
			},
			func(err error) {
				p.handleError(session, err)
			},
		)
		session.shakaEncoder.SetManifestCallback(func(manifestPath string) {
			p.onManifestUpdate(session.ID, manifestPath)
		})
	} else {
		// Create standard stream encoder
		session.encoder = NewStreamEncoder(outputDir, p.config.SegmentDuration)
		session.encoder.SetCallbacks(
			func(segmentPath string, index int) {
				p.onSegmentReady(session.ID, SegmentInfo{
					Path:  segmentPath,
					Index: index,
					Type:  "video",
				})
			},
			func(err error) {
				p.handleError(session, err)
			},
		)
	}

	// Determine manifest path
	switch req.Container {
	case "dash":
		session.ManifestPath = filepath.Join(outputDir, "stream.mpd")
	case "hls":
		session.ManifestPath = filepath.Join(outputDir, "stream.m3u8")
	default:
		cancel()
		return nil, fmt.Errorf("unsupported container for streaming: %s", req.Container)
	}

	// Store session
	p.sessionMutex.Lock()
	p.sessions[dbSession.ID] = session
	p.sessionMutex.Unlock()

	// Start streaming pipeline
	go p.runStreamingPipeline(session)

	return handle, nil
}

// runStreamingPipeline executes the streaming transcoding process
func (p *StreamingPipeline) runStreamingPipeline(session *StreamingSession) {
	p.logger.Info("Starting streaming pipeline",
		"sessionID", session.ID,
		"container", session.Request.Container,
		"input", session.Request.InputPath)

	// Update status
	session.Status = plugins.TranscodeStatusRunning
	session.Handle.Status = plugins.TranscodeStatusRunning

	// Determine encoding profiles
	profiles := p.config.ABRProfiles
	if !p.config.EnableABR || len(profiles) == 0 {
		// Single quality profile
		profiles = []EncodingProfile{{
			Name:         "default",
			Width:        1920,
			Height:       1080,
			VideoBitrate: 4000,
			Quality:      23,
		}}
	}

	// Create encoder - use Shaka if enabled for low-latency VOD
	if p.config.UseShakaPackager {
		p.logger.Info("Using Shaka Packager for low-latency VOD streaming")
		session.shakaEncoder = NewShakaStreamEncoder(session.OutputDir, p.config.SegmentDuration)
		
		// Set Shaka encoder callbacks
		session.shakaEncoder.SetCallbacks(
			func(segmentPath string, index int) {
				// Handle segment ready
				p.handleSegmentReady(session, segmentPath, index)
			},
			func(err error) {
				p.handleEncodingError(session, err)
			},
		)
		
		// Set manifest callback
		session.shakaEncoder.SetManifestCallback(func(manifestPath string) {
			session.ManifestPath = manifestPath
			p.handleManifestUpdate(session, manifestPath)
		})
		
		// Set progress callback
		session.shakaEncoder.SetProgressCallback(func(progress FFmpegProgress) {
			// Update progress
			if progress.Progress == "end" {
				session.Progress = 1.0
			} else if progress.Time > 0 {
				session.Progress = 0.5
			}
		})
		
		// Set completion callback for proper finalization
		session.shakaEncoder.SetCompletionCallback(func(done func(error)) {
			// Signal completion
			done(nil)
		})
		
		// Set event bus integration if available
		// TODO: Add event bus integration when available
	} else {
		// Use standard FFmpeg DASH encoder
		session.encoder = NewStreamEncoder(session.OutputDir, p.config.SegmentDuration)
		
		// Set encoder callbacks
		session.encoder.SetCallbacks(
			func(segmentPath string, index int) {
				// Handle segment ready
				p.handleSegmentReady(session, segmentPath, index)
			},
			func(err error) {
				p.handleEncodingError(session, err)
			},
		)
		
		// Set progress callback to track manifest updates
		session.encoder.SetProgressCallback(func(progress FFmpegProgress) {
			// Check if manifest exists and update
			manifestPath := filepath.Join(session.OutputDir, "manifest.mpd")
			if _, err := os.Stat(manifestPath); err == nil {
				// Update manifest path if not set
				if session.ManifestPath == "" {
					session.ManifestPath = manifestPath
				}
				// Notify manifest update
				p.handleManifestUpdate(session, manifestPath)
			}
			
			// Update progress
			if progress.Progress == "end" {
				session.Progress = 1.0
			} else if progress.Time > 0 {
				// Calculate progress based on time (assuming we know duration)
				// For now, just track that we're making progress
				session.Progress = 0.5
			}
		})
		
		// Set event bus integration if available
		// TODO: Add event bus integration when available
	}
	

	
	// Start encoding with appropriate encoder
	if session.shakaEncoder != nil {
		// Start Shaka-based encoding
		if err := session.shakaEncoder.StartEncoding(session.ctx, session.Request.InputPath, profiles); err != nil {
			p.handleError(session, fmt.Errorf("failed to start Shaka encoding: %w", err))
			return
		}
		
		// Wait for context cancellation (Shaka monitors itself internally)
		<-session.ctx.Done()
		p.logger.Info("Shaka session context done", "sessionID", session.ID)
	} else {
		// Start standard FFmpeg encoding
		if err := session.encoder.StartEncoding(session.ctx, session.Request.InputPath, profiles); err != nil {
			p.handleError(session, fmt.Errorf("failed to start encoding: %w", err))
			return
		}
		
		// Track completion for standard encoder
		completeChan := make(chan error, 1)
		
		// Set completion callback
		if session.encoder.onComplete != nil {
			session.encoder.onComplete(func(err error) {
				completeChan <- err
			})
		}
		
		// Wait for completion or cancellation
		select {
		case <-session.ctx.Done():
			// Context cancelled (user stopped)
			p.logger.Info("Session cancelled", "sessionID", session.ID)
		case err := <-completeChan:
			// FFmpeg completed
			if err != nil {
				p.logger.Error("FFmpeg completed with error", "sessionID", session.ID, "error", err)
			} else {
				p.logger.Info("FFmpeg completed successfully", "sessionID", session.ID)
			}
		}
	}

	// Finalize (this will stop the pipeline)
	p.finalizeSession(session)
}

// handleSegmentReady processes newly encoded segments
func (p *StreamingPipeline) handleSegmentReady(session *StreamingSession, segmentPath string, segmentIndex int) {
	// Get segment size for metrics
	var segmentSize int64
	if info, err := os.Stat(segmentPath); err == nil {
		segmentSize = info.Size()
	}

	// Record segment production with health monitor
	if p.healthMonitor != nil {
		// Calculate encode time (approximate)
		encodeTime := time.Duration(p.config.SegmentDuration) * time.Second
		p.healthMonitor.RecordSegmentProduced(session.ID, segmentIndex, encodeTime, segmentSize)
	}
	p.logger.Debug("Segment ready",
		"sessionID", session.ID,
		"index", segmentIndex,
		"path", segmentPath)

	// Track first segment time for startup metrics
	if session.FirstSegmentTime.IsZero() {
		session.FirstSegmentTime = time.Now()
		startupTime := session.FirstSegmentTime.Sub(session.StartTime)
		p.logger.Info("First segment ready",
			"sessionID", session.ID,
			"startupTime", startupTime)

		// Trigger startup prefetching for snappy playback
		if session.prefetcher != nil {
			go func() {
				if err := session.prefetcher.PrefetchForStartup(session.ContentHash, session.ID); err != nil {
					p.logger.Warn("Failed to prefetch startup segments",
						"sessionID", session.ID,
						"error", err)
				}
			}()
		}
	}

	// Segment is already packaged by FFmpeg DASH muxer

	// Update progress
	session.SegmentsReady = segmentIndex + 1

	// Notify callback
	if p.onSegmentReady != nil {
		segment := SegmentInfo{
			Path:      segmentPath,
			Index:     segmentIndex,
			Profile:   "default",
			Type:      "video",
			Timestamp: time.Now(),
		}
		p.onSegmentReady(session.ID, segment)
	}

	// Publish segment ready event to the global event bus
	eventHandler := modulemanager.GetModuleEventHandler()
	if eventHandler != nil {
		eventHandler.PublishModuleEvent("system.transcoding", events.NewSegmentReadyEvent(
			session.ID,
			segmentIndex,
			segmentPath,
			float64(p.config.SegmentDuration),
		))

		p.logger.Debug("Published segment ready event",
			"sessionID", session.ID,
			"segment", segmentIndex)
	}
}

// handleManifestUpdate processes manifest updates
func (p *StreamingPipeline) handleManifestUpdate(session *StreamingSession, manifestPath string) {
	p.logger.Debug("Manifest updated",
		"sessionID", session.ID,
		"path", manifestPath)

	// Update session store with progress
	progress := &plugins.TranscodingProgress{
		PercentComplete: float64(session.SegmentsReady) / float64(session.SegmentsTotal) * 100,
		TimeElapsed:     time.Since(session.StartTime),
	}

	if err := p.sessionStore.UpdateProgress(session.ID, progress); err != nil {
		p.logger.Warn("Failed to update session progress",
			"sessionID", session.ID,
			"error", err)
	}

	// Notify callback
	if p.onManifestUpdate != nil {
		p.onManifestUpdate(session.ID, manifestPath)
	}

	// Publish manifest updated event to the global event bus
	eventHandler := modulemanager.GetModuleEventHandler()
	if eventHandler != nil {
		event := events.NewEventBuilder(events.EventManifestUpdated, "module:transcoding").
			WithTitle("Manifest Updated").
			WithMessage(fmt.Sprintf("Manifest updated for session %s", session.ID)).
			WithDataValue("session_id", session.ID).
			WithDataValue("manifest_path", manifestPath).
			WithDataValue("content_hash", session.ContentHash).
			Build()

		eventHandler.PublishModuleEvent("system.transcoding", event)

		p.logger.Debug("Published manifest updated event",
			"sessionID", session.ID,
			"manifest", manifestPath)
	}
}

// handleSegmentPackaged processes packaged segments
func (p *StreamingPipeline) handleSegmentPackaged(session *StreamingSession, segmentPath string) {
	p.logger.Debug("Segment packaged",
		"sessionID", session.ID,
		"path", segmentPath)
}

// handleError handles pipeline errors
func (p *StreamingPipeline) handleError(session *StreamingSession, err error) {
	p.logger.Error("Streaming pipeline error",
		"sessionID", session.ID,
		"error", err)

	// Record error with health monitor
	if p.healthMonitor != nil {
		p.healthMonitor.RecordError(session.ID, "ffmpeg", err)
	}

	// Update status
	session.Status = plugins.TranscodeStatusFailed
	session.Handle.Status = plugins.TranscodeStatusFailed
	session.Handle.Error = err.Error()

	// Mark session as failed in database
	if err := p.sessionStore.FailSession(session.ID, err); err != nil {
		p.logger.Warn("Failed to update session status",
			"sessionID", session.ID,
			"error", err)
	}

	// Publish transcode failed event
	eventHandler := modulemanager.GetModuleEventHandler()
	if eventHandler != nil {
		event := events.NewTranscodeEvent(
			events.EventTranscodeFailed,
			session.ID,
			session.Request.MediaID,
			"failed",
		)
		event.Data["error"] = err.Error()
		event.Data["content_hash"] = session.ContentHash

		eventHandler.PublishModuleEvent("system.transcoding", event)

		p.logger.Debug("Published transcode failed event",
			"sessionID", session.ID,
			"error", err)
	}

	// Cancel session
	if session.cancel != nil {
		session.cancel()
	}
}

// handleEncodingError handles encoder errors
func (p *StreamingPipeline) handleEncodingError(session *StreamingSession, err error) {
	// Record specific encoding error
	if p.healthMonitor != nil {
		p.healthMonitor.RecordError(session.ID, "ffmpeg", err)
	}

	p.handleError(session, fmt.Errorf("encoding error: %w", err))
}

// handlePackagingError handles packager errors
func (p *StreamingPipeline) handlePackagingError(session *StreamingSession, err error) {
	// Record specific packaging error
	if p.healthMonitor != nil {
		p.healthMonitor.RecordError(session.ID, "storage", err)
	}

	p.handleError(session, fmt.Errorf("packaging error: %w", err))
}

// finalizeSession completes and stores the session content
func (p *StreamingPipeline) finalizeSession(session *StreamingSession) {
	p.logger.Info("Finalizing streaming session",
		"sessionID", session.ID,
		"segmentsReady", session.SegmentsReady)

	// Stop encoder
	if session.shakaEncoder != nil {
		if err := session.shakaEncoder.StopEncoding(); err != nil {
			p.logger.Warn("Error stopping Shaka encoder", "error", err)
		}
	} else if session.encoder != nil {
		if err := session.encoder.StopEncoding(); err != nil {
			p.logger.Warn("Error stopping encoder", "error", err)
		}
	}

	// Update status if not already failed
	if session.Status != plugins.TranscodeStatusFailed {
		session.Status = plugins.TranscodeStatusCompleted
		session.Handle.Status = plugins.TranscodeStatusCompleted

		// Store in content store if available
		if p.contentStore != nil && session.ContentHash != "" {
			// IMPORTANT: Ensure MediaID is not empty to prevent hash collisions
			mediaID := session.Request.MediaID
			if mediaID == "" {
				// Log warning but don't fail - use session ID as fallback
				p.logger.Warn("Empty MediaID detected, using session ID as fallback",
					"sessionID", session.ID,
					"contentHash", session.ContentHash)
				mediaID = fmt.Sprintf("session-%s", session.ID)
			}
			
			metadata := storage.ContentMetadata{
				MediaID:       mediaID,
				Format:        session.Request.Container,
				ManifestURL:   fmt.Sprintf("/api/v1/content/%s/manifest.%s", session.ContentHash, session.Request.Container),
				RetentionDays: 30,
				Tags: map[string]string{
					"sessionID": session.ID,
					"streaming": "true",
				},
			}

			if err := p.contentStore.Store(session.ContentHash, session.OutputDir, metadata); err != nil {
				p.logger.Warn("Failed to store content",
					"sessionID", session.ID,
					"contentHash", session.ContentHash,
					"error", err)
			} else {
				// Notify completion
				if p.onContentComplete != nil {
					p.onContentComplete(session.ID, session.ContentHash)
				}

				// Publish transcode completed event
				eventHandler := modulemanager.GetModuleEventHandler()
				if eventHandler != nil {
					event := events.NewTranscodeEvent(
						events.EventTranscodeCompleted,
						session.ID,
						session.Request.MediaID,
						"completed",
					)
					event.Data["content_hash"] = session.ContentHash
					event.Data["manifest_url"] = fmt.Sprintf("/api/v1/content/%s/manifest.%s", session.ContentHash, session.Request.Container)
					event.Data["segments_total"] = session.SegmentsReady
					event.Data["duration"] = time.Since(session.StartTime).String()

					eventHandler.PublishModuleEvent("system.transcoding", event)

					p.logger.Info("Published transcode completed event",
						"sessionID", session.ID,
						"contentHash", session.ContentHash)
				}
			}
		}

		// Mark session as completed in database
		result := &plugins.TranscodeResult{
			Success:     true,
			OutputPath:  session.ManifestPath,
			ManifestURL: fmt.Sprintf("/api/v1/content/%s/manifest.%s", session.ContentHash, session.Request.Container),
		}

		if err := p.sessionStore.CompleteSession(session.ID, result); err != nil {
			p.logger.Warn("Failed to complete session",
				"sessionID", session.ID,
				"error", err)
		}
	}

	// Unregister from health monitor
	if p.healthMonitor != nil {
		p.healthMonitor.UnregisterSession(session.ID)
	}

	// Remove from active sessions
	p.sessionMutex.Lock()
	delete(p.sessions, session.ID)
	p.sessionMutex.Unlock()
}

// StopStreaming stops an active streaming session
func (p *StreamingPipeline) StopStreaming(sessionID string) error {
	p.sessionMutex.RLock()
	session, exists := p.sessions[sessionID]
	p.sessionMutex.RUnlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Cancel session
	if session.cancel != nil {
		session.cancel()
	}

	return nil
}

// GetProgress returns the progress of a streaming session
func (p *StreamingPipeline) GetProgress(sessionID string) (*plugins.TranscodingProgress, error) {
	p.sessionMutex.RLock()
	session, exists := p.sessions[sessionID]
	p.sessionMutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	progress := &plugins.TranscodingProgress{
		PercentComplete: session.Progress,
		TimeElapsed:     time.Since(session.StartTime),
		CurrentSpeed:    1.0, // TODO: Calculate actual speed
		AverageSpeed:    1.0,
	}

	// Estimate time remaining based on segments
	if session.SegmentsReady > 0 && session.SegmentsTotal > 0 {
		segmentsRemaining := session.SegmentsTotal - session.SegmentsReady
		timePerSegment := progress.TimeElapsed / time.Duration(session.SegmentsReady)
		progress.TimeRemaining = timePerSegment * time.Duration(segmentsRemaining)
		progress.EstimatedTime = progress.TimeElapsed + progress.TimeRemaining
	}

	return progress, nil
}

// GetStreamingStatus returns detailed status of a streaming session
func (p *StreamingPipeline) GetStreamingStatus(sessionID string) (*StreamingStatus, error) {
	p.sessionMutex.RLock()
	session, exists := p.sessions[sessionID]
	p.sessionMutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	status := &StreamingStatus{
		SessionID:      session.ID,
		Status:         string(session.Status),
		SegmentsReady:  session.SegmentsReady,
		SegmentsTotal:  session.SegmentsTotal,
		ManifestURL:    fmt.Sprintf("/api/v1/sessions/%s/manifest.%s", session.ID, session.Request.Container),
		ContentHash:    session.ContentHash,
		IsLive:         session.IsLive,
		BufferHealth:   p.calculateBufferHealth(session),
		StartupTime:    session.FirstSegmentTime.Sub(session.StartTime),
		ViewerPosition: session.ViewerPosition,
		BufferPosition: session.BufferPosition,
	}

	// Add prefetcher metrics if available
	if session.prefetcher != nil {
		status.PrefetchMetrics = session.prefetcher.GetMetrics()
		status.BufferStatus = session.prefetcher.GetBufferStatus(session.ContentHash)
	}

	// Add health metrics if available
	if p.healthMonitor != nil {
		status.HealthMetrics = p.healthMonitor.GetMetrics()
		status.HealthStatus = p.healthMonitor.GetOverallHealth()
	}

	return status, nil
}

// UpdatePlaybackPosition updates the viewer's current playback position
func (p *StreamingPipeline) UpdatePlaybackPosition(sessionID string, segmentIndex int, isPlaying bool, playbackSpeed float64) error {
	p.sessionMutex.RLock()
	session, exists := p.sessions[sessionID]
	p.sessionMutex.RUnlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Update session state
	session.ViewerPosition = time.Duration(segmentIndex*p.config.SegmentDuration) * time.Second

	// Update prefetcher for intelligent buffering
	if session.prefetcher != nil {
		session.prefetcher.UpdatePlaybackPosition(session.ContentHash, segmentIndex, isPlaying, playbackSpeed)
	}

	p.logger.Debug("Updated playback position",
		"sessionID", sessionID,
		"segment", segmentIndex,
		"playing", isPlaying,
		"speed", playbackSpeed)

	return nil
}

// GetSegment retrieves a segment with intelligent prefetching
func (p *StreamingPipeline) GetSegment(sessionID string, segmentIndex int) ([]byte, error) {
	p.sessionMutex.RLock()
	session, exists := p.sessions[sessionID]
	p.sessionMutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Try prefetcher first for buffered segments
	if session.prefetcher != nil {
		if data, err := session.prefetcher.GetSegment(session.ContentHash, segmentIndex); err == nil {
			p.logger.Debug("Served segment from prefetch buffer",
				"sessionID", sessionID,
				"segment", segmentIndex,
				"size", len(data))
			return data, nil
		}
	}

	// Fallback to direct file access
	segmentPath := filepath.Join(session.OutputDir, fmt.Sprintf("segment_%03d.mp4", segmentIndex))
	data, err := os.ReadFile(segmentPath)
	if err != nil {
		return nil, fmt.Errorf("segment not available: %w", err)
	}

	p.logger.Debug("Served segment from direct file access",
		"sessionID", sessionID,
		"segment", segmentIndex,
		"size", len(data))

	return data, nil
}

// GetPrefetchMetrics returns prefetching performance metrics
func (p *StreamingPipeline) GetPrefetchMetrics() map[string]interface{} {
	if p.prefetcher == nil {
		return map[string]interface{}{"status": "disabled"}
	}

	return p.prefetcher.GetMetrics()
}

// CleanupStaleBuffers removes old segment buffers to free memory
func (p *StreamingPipeline) CleanupStaleBuffers(maxAge time.Duration) {
	if p.prefetcher != nil {
		p.prefetcher.CleanupStaleBuffers(maxAge)
	}
}

// StartHealthMonitoring starts the health monitoring system
func (p *StreamingPipeline) StartHealthMonitoring(ctx context.Context) error {
	if p.healthMonitor == nil {
		return fmt.Errorf("health monitor not available")
	}

	return p.healthMonitor.Start(ctx)
}

// GetHealthReport returns a comprehensive health report
func (p *StreamingPipeline) GetHealthReport() *HealthReport {
	if p.healthMonitor == nil {
		return &HealthReport{
			OverallStatus:  HealthStatusUnknown,
			Timestamp:      time.Now(),
			Metrics:        &StreamingMetrics{},
			SessionsHealth: make(map[string]*SessionHealth),
			ActiveAlerts:   []HealthAlert{},
			SystemInfo:     map[string]interface{}{"status": "monitor_disabled"},
		}
	}

	return p.healthMonitor.GetHealthReport()
}

// GetHealthMetrics returns current streaming metrics
func (p *StreamingPipeline) GetHealthMetrics() *StreamingMetrics {
	if p.healthMonitor == nil {
		return &StreamingMetrics{
			LastUpdated:      time.Now(),
			MetricsStartTime: time.Now(),
		}
	}

	return p.healthMonitor.GetMetrics()
}

// GetSessionHealthStatus returns health status for a specific session
func (p *StreamingPipeline) GetSessionHealthStatus(sessionID string) (*SessionHealth, error) {
	if p.healthMonitor == nil {
		return nil, fmt.Errorf("health monitor not available")
	}

	return p.healthMonitor.GetSessionHealth(sessionID)
}

// IsHealthy returns true if the streaming system is healthy
func (p *StreamingPipeline) IsHealthy() bool {
	if p.healthMonitor == nil {
		return true // Assume healthy if no monitoring
	}

	return p.healthMonitor.IsHealthy()
}

// Stop shuts down the streaming pipeline and all components
func (p *StreamingPipeline) Stop() error {
	p.logger.Info("Stopping streaming pipeline")

	// Stop health monitor
	if p.healthMonitor != nil {
		p.healthMonitor.Stop()
	}

	// Stop all active sessions
	p.sessionMutex.Lock()
	for sessionID, session := range p.sessions {
		if session.cancel != nil {
			session.cancel()
		}
		p.logger.Debug("Stopped session", "sessionID", sessionID)
	}
	p.sessions = make(map[string]*StreamingSession)
	p.sessionMutex.Unlock()

	// Stop prefetcher
	if p.prefetcher != nil {
		p.prefetcher.Stop()
	}

	p.logger.Info("Streaming pipeline stopped")
	return nil
}

// calculateBufferHealth determines the health of the streaming buffer
func (p *StreamingPipeline) calculateBufferHealth(session *StreamingSession) string {
	bufferSeconds := int(session.BufferPosition - session.ViewerPosition)

	switch {
	case bufferSeconds >= p.config.BufferAhead:
		return "excellent"
	case bufferSeconds >= p.config.SegmentDuration*2:
		return "good"
	case bufferSeconds >= p.config.SegmentDuration:
		return "fair"
	default:
		return "poor"
	}
}

// StreamingStatus represents the status of a streaming session
type StreamingStatus struct {
	SessionID       string                 `json:"session_id"`
	Status          string                 `json:"status"`
	SegmentsReady   int                    `json:"segments_ready"`
	SegmentsTotal   int                    `json:"segments_total"`
	ManifestURL     string                 `json:"manifest_url"`
	ContentHash     string                 `json:"content_hash"`
	IsLive          bool                   `json:"is_live"`
	BufferHealth    string                 `json:"buffer_health"`
	StartupTime     time.Duration          `json:"startup_time"`
	ViewerPosition  time.Duration          `json:"viewer_position"`
	BufferPosition  time.Duration          `json:"buffer_position"`
	PrefetchMetrics map[string]interface{} `json:"prefetch_metrics,omitempty"`
	BufferStatus    map[string]interface{} `json:"buffer_status,omitempty"`
	HealthMetrics   *StreamingMetrics      `json:"health_metrics,omitempty"`
	HealthStatus    HealthStatus           `json:"health_status,omitempty"`
}
