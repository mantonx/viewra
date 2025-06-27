// Package pipeline provides streaming pipeline functionality for real-time transcoding.
// This file implements segment-based encoding for instant playback.
package pipeline

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/events"
)

// StreamEncoder handles segment-based encoding for streaming
type StreamEncoder struct {
	ffmpegPath      string
	segmentDuration int
	outputDir       string
	logger          hclog.Logger

	// Segment tracking
	segmentCount int
	segmentMutex sync.RWMutex

	// Process management
	cmd        *exec.Cmd
	cancelFunc context.CancelFunc
	monitor    *FFmpegMonitor

	// Fast-start optimization
	optimizer        *FastStartOptimizer
	fastStartEnabled bool
	seekOptimized    bool

	// Adaptive segment sizing
	adaptiveSizer    *AdaptiveSegmentSizer
	adaptiveSegments bool

	// Event callbacks
	onSegmentReady func(segmentPath string, segmentIndex int)
	onError        func(error)
	onProgress     func(FFmpegProgress) // External progress callback
	onComplete     func(func(error))    // Completion callback setter

	// Event system integration
	eventBus    *events.EventBus
	sessionID   string
	contentHash string

	// Monitoring
	lastProgress  FFmpegProgress
	progressMutex sync.RWMutex
}

// NewStreamEncoder creates a new streaming encoder
func NewStreamEncoder(outputDir string, segmentDuration int) *StreamEncoder {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "stream-encoder",
		Level: hclog.Info,
	})

	optimizer := NewFastStartOptimizer(logger)

	return &StreamEncoder{
		ffmpegPath:       "ffmpeg", // TODO: Make configurable
		segmentDuration:  segmentDuration,
		outputDir:        outputDir,
		logger:           logger,
		monitor:          NewFFmpegMonitor(logger),
		optimizer:        optimizer,
		fastStartEnabled: true, // Enable by default
		seekOptimized:    true, // Enable seek optimization
		adaptiveSizer:    NewAdaptiveSegmentSizer(logger, optimizer),
		adaptiveSegments: false, // Disabled by default for compatibility
	}
}

// SetCallbacks sets event callbacks for segment production
func (e *StreamEncoder) SetCallbacks(onSegment func(string, int), onError func(error)) {
	e.onSegmentReady = onSegment
	e.onError = onError
}

// SetProgressCallback sets an external progress monitoring callback
func (e *StreamEncoder) SetProgressCallback(onProgress func(FFmpegProgress)) {
	e.onProgress = onProgress
}

// SetEventBus integrates with the segment event system
func (e *StreamEncoder) SetEventBus(eventBus *events.EventBus, sessionID, contentHash string) {
	e.eventBus = eventBus
	e.sessionID = sessionID
	e.contentHash = contentHash
}

// StartEncoding begins segment-based encoding
func (e *StreamEncoder) StartEncoding(ctx context.Context, input string, profiles []EncodingProfile) error {
	// Generate adaptive segment plan if enabled
	if e.adaptiveSegments {
		plan, err := e.adaptiveSizer.GenerateAdaptiveSegmentPlan(ctx, input)
		if err != nil {
			e.logger.Warn("Failed to generate adaptive segment plan, using uniform segments", "error", err)
		} else {
			e.logger.Info("Generated adaptive segment plan",
				"segments", plan.SegmentCount,
				"avg_complexity", plan.AverageComplexity,
				"optimization_score", plan.OptimizationScore)

			// Log plan summary
			summary := e.adaptiveSizer.GetSegmentPlanSummary(plan)
			e.logger.Debug("Adaptive segment plan details", "summary", summary)
		}
	}
	// Create context with cancellation
	encodeCtx, cancel := context.WithCancel(ctx)
	e.cancelFunc = cancel

	// Ensure output directories exist
	if err := e.createOutputDirs(profiles); err != nil {
		return fmt.Errorf("failed to create output directories: %w", err)
	}

	// Build FFmpeg command for streaming segments
	args := e.buildStreamingArgs(input, profiles)

	logger.Info("Starting streaming encode",
		"input", input,
		"profiles", len(profiles),
		"segment_duration", e.segmentDuration)

	// Start FFmpeg process
	e.cmd = exec.CommandContext(encodeCtx, e.ffmpegPath, args...)
	
	// Log the full command for debugging
	e.logger.Info("FFmpeg command", "cmd", e.ffmpegPath, "args", strings.Join(args, " "))

	// Setup FFmpeg monitoring
	e.monitor.SetCallbacks(
		func(progress FFmpegProgress) {
			e.handleProgress(progress)
		},
		func(err error) {
			e.handleFFmpegError(err)
		},
	)

	// Start monitoring before starting the process
	if err := e.monitor.StartMonitoring(encodeCtx, e.cmd); err != nil {
		return fmt.Errorf("failed to start FFmpeg monitoring: %w", err)
	}

	// Start process
	if err := e.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	e.logger.Info("FFmpeg process started", "pid", e.cmd.Process.Pid)

	// Monitor segment production in background
	go e.monitorSegments(encodeCtx)

	// Track completion
	var onComplete func(error)
	
	// Allow setting completion callback
	e.onComplete = func(cb func(error)) {
		onComplete = cb
	}

	// Wait for process completion in background
	go func() {
		defer e.monitor.StopMonitoring()

		err := e.cmd.Wait()
		
		// Check if context was cancelled (user stopped) vs normal completion
		if err != nil && encodeCtx.Err() == nil {
			// Only treat as error if context wasn't cancelled
			if e.onError != nil {
				e.logger.Error("FFmpeg process failed", "error", err)
				e.onError(fmt.Errorf("FFmpeg process failed: %w", err))
			}
		} else {
			e.logger.Info("FFmpeg process completed", "cancelled", encodeCtx.Err() != nil)
		}
		
		// Call completion callback with nil if process completed normally or was cancelled
		if onComplete != nil {
			if encodeCtx.Err() != nil {
				// Context was cancelled, treat as normal completion
				onComplete(nil)
			} else {
				// Normal completion or actual error
				onComplete(err)
			}
		}
	}()

	return nil
}

// StopEncoding stops the encoding process
func (e *StreamEncoder) StopEncoding() error {
	if e.cancelFunc != nil {
		e.cancelFunc()
	}

	// Give FFmpeg time to finish current segment
	time.Sleep(500 * time.Millisecond)

	if e.cmd != nil && e.cmd.Process != nil {
		// Just kill the process - SIGINT doesn't work in Alpine container
		return e.cmd.Process.Kill()
	}

	return nil
}

// buildStreamingArgs constructs FFmpeg arguments for segment-based encoding
func (e *StreamEncoder) buildStreamingArgs(input string, profiles []EncodingProfile) []string {
	args := []string{
		"-i", input,
		"-loglevel", "info",
		"-progress", "pipe:1", // Progress to stdout for monitoring
	}

	// For DASH muxer, we need a simpler approach
	// Single quality first to get it working
	profile := profiles[0] // Use first profile for now
	
	// Video encoding
	args = append(args,
		"-map", "0:v:0",
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p", // Force 8-bit pixel format for compatibility
		"-profile:v", "main",
		"-preset", "fast",
		"-g", fmt.Sprintf("%d", e.segmentDuration*30), // GOP size
		"-keyint_min", fmt.Sprintf("%d", e.segmentDuration*15),
		"-crf", fmt.Sprintf("%d", profile.Quality),
		"-maxrate", fmt.Sprintf("%dk", profile.VideoBitrate),
		"-bufsize", fmt.Sprintf("%dk", profile.VideoBitrate*2),
	)
	
	// Scale if needed
	if profile.Width > 0 && profile.Height > 0 {
		args = append(args,
			"-vf", fmt.Sprintf("scale=%d:%d", profile.Width, profile.Height),
		)
	}

	// Audio encoding
	args = append(args,
		"-map", "0:a:0",
		"-c:a", "aac",
		"-b:a", "128k",
		"-ac", "2",
	)

	// Use DASH muxer for proper DASH VOD output
	args = append(args,
		"-f", "dash",
		"-seg_duration", fmt.Sprintf("%d", e.segmentDuration),
		"-use_template", "1",
		"-use_timeline", "1",
		"-dash_segment_type", "mp4",
		"-init_seg_name", "init-stream$RepresentationID$.mp4",
		"-media_seg_name", "segment-stream$RepresentationID$-$Number$.m4s",
		"-remove_at_exit", "0",      // Don't remove segments
		"-adaptation_sets", "id=0,streams=v id=1,streams=a", // Explicit adaptation sets
	)

	// Output manifest file
	args = append(args, filepath.Join(e.outputDir, "manifest.mpd"))

	e.logger.Debug("Built streaming args",
		"profile", profile.Name,
		"segment_duration", e.segmentDuration,
	)

	return args
}

// monitorSegments watches for new segment files
func (e *StreamEncoder) monitorSegments(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	lastSegmentIndex := -1

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check for new segments
			newSegments := e.checkForNewSegments(lastSegmentIndex)
			for _, segmentIndex := range newSegments {
				segmentPath := filepath.Join(e.outputDir, fmt.Sprintf("segment-stream0-%d.m4s", segmentIndex))

				// Ensure segment is fully written (check size stability)
				if e.isSegmentReady(segmentPath) {
					logger.Debug("New segment ready", "index", segmentIndex, "path", segmentPath)

					// Publish segment ready event
					if e.eventBus != nil {
						e.eventBus.PublishSegmentReady(e.sessionID, e.contentHash, segmentIndex, segmentPath)
					}

					// Legacy callback support
					if e.onSegmentReady != nil {
						e.onSegmentReady(segmentPath, segmentIndex)
					}

					lastSegmentIndex = segmentIndex

					e.segmentMutex.Lock()
					e.segmentCount = segmentIndex + 1
					e.segmentMutex.Unlock()
				}
			}
		}
	}
}

// checkForNewSegments finds segments newer than the last processed index
func (e *StreamEncoder) checkForNewSegments(lastIndex int) []int {
	var newSegments []int

	// DASH muxer creates segments with pattern: segment-stream$RepresentationID$-$Number$.m4s
	// Look for video segments (RepresentationID=0)
	for i := lastIndex + 1; i <= lastIndex+10; i++ {
		segmentPath := filepath.Join(e.outputDir, fmt.Sprintf("segment-stream0-%d.m4s", i))
		if _, err := os.Stat(segmentPath); err == nil {
			newSegments = append(newSegments, i)
		} else {
			// Stop checking once we hit a missing segment
			break
		}
	}

	return newSegments
}

// isSegmentReady checks if a segment file is fully written
func (e *StreamEncoder) isSegmentReady(path string) bool {
	// Check file size twice with small delay
	info1, err := os.Stat(path)
	if err != nil {
		return false
	}

	time.Sleep(100 * time.Millisecond)

	info2, err := os.Stat(path)
	if err != nil {
		return false
	}

	// File is ready if size hasn't changed and is non-zero
	return info1.Size() == info2.Size() && info1.Size() > 0
}

// createOutputDirs creates necessary output directories
func (e *StreamEncoder) createOutputDirs(profiles []EncodingProfile) error {
	// Create main output directory
	if err := os.MkdirAll(e.outputDir, 0755); err != nil {
		return err
	}

	// Create profile-specific directories if needed
	for _, profile := range profiles {
		profileDir := filepath.Join(e.outputDir, profile.Name)
		if err := os.MkdirAll(profileDir, 0755); err != nil {
			return err
		}
	}

	return nil
}

// GetSegmentCount returns the current number of encoded segments
func (e *StreamEncoder) GetSegmentCount() int {
	e.segmentMutex.RLock()
	defer e.segmentMutex.RUnlock()
	return e.segmentCount
}

// handleProgress processes FFmpeg progress updates
func (e *StreamEncoder) handleProgress(progress FFmpegProgress) {
	e.progressMutex.Lock()
	e.lastProgress = progress
	e.progressMutex.Unlock()

	// Log progress at reasonable intervals
	if progress.Frame%100 == 0 || progress.Progress == "end" {
		e.logger.Debug("Encoding progress",
			"frame", progress.Frame,
			"fps", progress.FPS,
			"time", progress.Time,
			"speed", progress.Speed,
			"quality", progress.Quality,
		)
	}

	// Publish progress event if event bus is available
	if e.eventBus != nil {
		e.eventBus.PublishProgress(e.sessionID, map[string]interface{}{
			"frame":    progress.Frame,
			"fps":      progress.FPS,
			"time":     progress.Time.String(),
			"speed":    progress.Speed,
			"progress": progress.Progress,
		})
	}

	// Call external progress callback if set
	if e.onProgress != nil {
		e.onProgress(progress)
	}
}

// handleFFmpegError processes FFmpeg errors
func (e *StreamEncoder) handleFFmpegError(err error) {
	e.logger.Error("FFmpeg error detected", "error", err)

	// Publish error event
	if e.eventBus != nil {
		e.eventBus.PublishError(e.sessionID, err.Error())
	}

	// Call error callback
	if e.onError != nil {
		e.onError(err)
	}
}

// GetProgress returns the latest encoding progress
func (e *StreamEncoder) GetProgress() FFmpegProgress {
	e.progressMutex.RLock()
	defer e.progressMutex.RUnlock()
	return e.lastProgress
}

// GetMonitorInfo returns FFmpeg monitoring information
func (e *StreamEncoder) GetMonitorInfo() map[string]interface{} {
	if e.monitor == nil {
		return map[string]interface{}{"status": "not_monitoring"}
	}

	info := e.monitor.GetProcessInfo()
	info["healthy"] = e.monitor.IsHealthy()

	e.progressMutex.RLock()
	info["last_progress"] = e.lastProgress
	e.progressMutex.RUnlock()

	return info
}

// IsHealthy returns true if the encoding process is healthy
func (e *StreamEncoder) IsHealthy() bool {
	if e.monitor == nil {
		return false
	}
	return e.monitor.IsHealthy()
}

// SetFastStartEnabled enables or disables fast-start optimization
func (e *StreamEncoder) SetFastStartEnabled(enabled bool) {
	e.fastStartEnabled = enabled
	e.logger.Debug("Fast-start optimization", "enabled", enabled)
}

// SetSeekOptimized enables or disables seek optimization
func (e *StreamEncoder) SetSeekOptimized(enabled bool) {
	e.seekOptimized = enabled
	e.logger.Debug("Seek optimization", "enabled", enabled)
}

// SetAdaptiveSegments enables or disables adaptive segment sizing
func (e *StreamEncoder) SetAdaptiveSegments(enabled bool) {
	e.adaptiveSegments = enabled
	e.logger.Debug("Adaptive segment sizing", "enabled", enabled)
}

// AnalyzeSegmentKeyframes analyzes keyframes in encoded segments
func (e *StreamEncoder) AnalyzeSegmentKeyframes(ctx context.Context, segmentPath string) ([]KeyframeInfo, error) {
	if e.optimizer == nil {
		return nil, fmt.Errorf("optimizer not available")
	}

	return e.optimizer.AnalyzeKeyframes(ctx, segmentPath)
}

// ValidateSegmentOptimization validates optimization of a specific segment
func (e *StreamEncoder) ValidateSegmentOptimization(ctx context.Context, segmentPath string) (*OptimizationReport, error) {
	if e.optimizer == nil {
		return nil, fmt.Errorf("optimizer not available")
	}

	return e.optimizer.ValidateOptimization(ctx, segmentPath)
}

// GetOptimizationStatus returns current optimization settings
func (e *StreamEncoder) GetOptimizationStatus() map[string]interface{} {
	return map[string]interface{}{
		"fast_start_enabled":       e.fastStartEnabled,
		"seek_optimized":           e.seekOptimized,
		"adaptive_segments":        e.adaptiveSegments,
		"optimizer_available":      e.optimizer != nil,
		"adaptive_sizer_available": e.adaptiveSizer != nil,
		"segment_duration":         e.segmentDuration,
	}
}

// GetSeekPoints generates seek points for optimized seeking
func (e *StreamEncoder) GetSeekPoints(keyframes []KeyframeInfo) []SeekPoint {
	if e.optimizer == nil {
		return nil
	}

	segmentDuration := time.Duration(e.segmentDuration) * time.Second
	return e.optimizer.GenerateSeekPoints(keyframes, segmentDuration)
}

// OptimizeSegmentBoundaries adjusts segment timing for keyframe alignment
func (e *StreamEncoder) OptimizeSegmentBoundaries(ctx context.Context, inputPath string) ([]time.Duration, error) {
	if e.optimizer == nil {
		return nil, fmt.Errorf("optimizer not available")
	}

	// Analyze input keyframes
	keyframes, err := e.optimizer.AnalyzeKeyframes(ctx, inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze keyframes: %w", err)
	}

	// Optimize boundaries
	targetDuration := time.Duration(e.segmentDuration) * time.Second
	boundaries := e.optimizer.OptimizeSegmentBoundaries(keyframes, targetDuration)

	e.logger.Debug("Optimized segment boundaries",
		"input_keyframes", len(keyframes),
		"optimized_boundaries", len(boundaries),
		"target_duration", targetDuration,
	)

	return boundaries, nil
}

// AnalyzeInputComplexity analyzes scene complexity for adaptive encoding
func (e *StreamEncoder) AnalyzeInputComplexity(ctx context.Context, inputPath string) ([]float64, error) {
	if e.optimizer == nil {
		return nil, fmt.Errorf("optimizer not available")
	}

	return e.optimizer.AnalyzeSceneComplexity(ctx, inputPath)
}

// GenerateAdaptiveSegmentPlan creates an adaptive segmentation strategy
func (e *StreamEncoder) GenerateAdaptiveSegmentPlan(ctx context.Context, inputPath string) (*AdaptiveSegmentPlan, error) {
	if e.adaptiveSizer == nil {
		return nil, fmt.Errorf("adaptive sizer not available")
	}

	return e.adaptiveSizer.GenerateAdaptiveSegmentPlan(ctx, inputPath)
}

// GetAdaptiveSegmentSummary returns a summary of the adaptive segmentation plan
func (e *StreamEncoder) GetAdaptiveSegmentSummary(plan *AdaptiveSegmentPlan) string {
	if e.adaptiveSizer == nil {
		return "Adaptive sizer not available"
	}

	return e.adaptiveSizer.GetSegmentPlanSummary(plan)
}

// EncodingProfile represents a quality level for streaming
type EncodingProfile struct {
	Name         string
	Width        int
	Height       int
	VideoBitrate int // in kbps
	Quality      int // CRF value
}
