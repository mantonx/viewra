// Package pipeline provides Shaka Packager integration for real-time VOD streaming
package pipeline

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/hashicorp/go-hclog"
)

// ShakaStreamEncoder handles real-time VOD encoding with Shaka Packager
type ShakaStreamEncoder struct {
	outputDir       string
	segmentDuration int
	
	// Processes
	ffmpegCmd *exec.Cmd
	shakaCmd  *exec.Cmd
	
	// Named pipe for FFmpeg -> Shaka communication
	pipePath string
	
	// Context and control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	
	// State
	isRunning bool
	mu        sync.Mutex
	
	// Callbacks
	onSegmentReady   func(segmentPath string, index int)
	onManifestUpdate func(manifestPath string)
	onError          func(error)
	onProgress       func(progress FFmpegProgress)
	onComplete       func(func(error))
	
	logger hclog.Logger
}

// NewShakaStreamEncoder creates a new Shaka-based encoder
func NewShakaStreamEncoder(outputDir string, segmentDuration int) *ShakaStreamEncoder {
	return &ShakaStreamEncoder{
		outputDir:       outputDir,
		segmentDuration: segmentDuration,
		logger: hclog.New(&hclog.LoggerOptions{
			Name:  "shaka-encoder",
			Level: hclog.Debug,
		}),
	}
}

// StartEncoding starts the FFmpeg -> Shaka pipeline for real-time VOD
func (s *ShakaStreamEncoder) StartEncoding(ctx context.Context, inputPath string, profiles []EncodingProfile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.isRunning {
		return fmt.Errorf("encoder already running")
	}
	
	s.ctx, s.cancel = context.WithCancel(ctx)
	
	// Create output directory
	if err := os.MkdirAll(s.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	
	// Create named pipe for FFmpeg -> Shaka communication
	s.pipePath = filepath.Join(s.outputDir, "ffmpeg_to_shaka.pipe")
	if err := syscall.Mkfifo(s.pipePath, 0600); err != nil {
		return fmt.Errorf("failed to create named pipe: %w", err)
	}
	
	// Start Shaka Packager first (it will wait for input)
	if err := s.startShakaPackager(); err != nil {
		os.Remove(s.pipePath)
		return fmt.Errorf("failed to start Shaka Packager: %w", err)
	}
	
	// Start FFmpeg encoder
	if err := s.startFFmpeg(inputPath, profiles); err != nil {
		s.stopShakaPackager()
		os.Remove(s.pipePath)
		return fmt.Errorf("failed to start FFmpeg: %w", err)
	}
	
	s.isRunning = true
	
	// Monitor processes
	s.wg.Add(2)
	go s.monitorFFmpeg()
	go s.monitorShaka()
	
	// Monitor for manifest and segments
	go s.monitorOutput()
	
	return nil
}

// startFFmpeg starts the FFmpeg encoder process
func (s *ShakaStreamEncoder) startFFmpeg(inputPath string, profiles []EncodingProfile) error {
	// For now, use single profile (will extend for ABR later)
	profile := profiles[0]
	
	args := []string{
		"-i", inputPath,
		"-c:v", "libx264",
		"-preset", "fast",
		"-crf", fmt.Sprintf("%d", profile.Quality),
		"-vf", fmt.Sprintf("scale=%d:%d", profile.Width, profile.Height),
		"-c:a", "aac",
		"-b:a", "128k",
		"-movflags", "+frag_keyframe+empty_moov+default_base_moof",
		"-f", "mp4",
		"-y", s.pipePath,
	}
	
	s.logger.Info("Starting FFmpeg", "args", strings.Join(args, " "))
	
	s.ffmpegCmd = exec.CommandContext(s.ctx, "ffmpeg", args...)
	
	// Capture stderr for debugging
	stderrPipe, err := s.ffmpegCmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	
	// Log FFmpeg output
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			s.logger.Debug("FFmpeg output", "line", line)
			
			// Parse progress if available
			if strings.Contains(line, "time=") && s.onProgress != nil {
				// Simple progress parsing
				s.onProgress(FFmpegProgress{
					Progress: "encoding",
					Time:     1, // Simplified for now
				})
			}
		}
	}()
	
	return s.ffmpegCmd.Start()
}

// startShakaPackager starts the Shaka Packager process
func (s *ShakaStreamEncoder) startShakaPackager() error {
	// Build Shaka arguments for low-latency VOD
	// Video stream (stream=video)
	// Note: $RepresentationID$ is not supported by Shaka Packager yet, so we use static names
	args := []string{
		fmt.Sprintf("in=%s,stream=video,segment_template=%s/video/segment-video-$Number$.m4s,init_segment=%s/video/init-video.mp4",
			s.pipePath, s.outputDir, s.outputDir),
		// Audio stream (stream=audio)
		fmt.Sprintf("in=%s,stream=audio,segment_template=%s/audio/segment-audio-$Number$.m4s,init_segment=%s/audio/init-audio.mp4",
			s.pipePath, s.outputDir, s.outputDir),
		"--segment_duration", fmt.Sprintf("%d", s.segmentDuration),
		"--mpd_output", filepath.Join(s.outputDir, "manifest.mpd"),
		"--generate_static_live_mpd", // This enables live-style manifest for VOD
		"--low_latency_dash_mode", // Enable low latency features
		"--utc_timings", "urn:mpeg:dash:utc:direct:2014", // For low latency
		"--suggested_presentation_delay", "8", // 2 segments buffer
		"--min_buffer_time", "8", // 2 segments minimum
		"--minimum_update_period", "2", // Update manifest every 2 seconds
		"--time_shift_buffer_depth", "300", // 5 minutes of DVR
		"--preserved_segments_outside_live_window", "10",
		"--default_language", "en",
		"--dump_stream_info", // For debugging
		"--allow_approximate_segment_timeline", // Allow minor timing adjustments
	}
	
	// Create necessary directories
	for _, dir := range []string{"video", "audio", "manifests"} {
		if err := os.MkdirAll(filepath.Join(s.outputDir, dir), 0755); err != nil {
			return fmt.Errorf("failed to create %s directory: %w", dir, err)
		}
	}
	
	s.logger.Info("Starting Shaka Packager", "args", strings.Join(args, " "))
	
	s.shakaCmd = exec.CommandContext(s.ctx, "packager", args...)
	
	// Capture stderr for debugging
	stderrPipe, err := s.shakaCmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	
	// Log Shaka output
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "ERROR") {
				s.logger.Error("Shaka error", "line", line)
			} else if strings.Contains(line, "WARNING") {
				s.logger.Warn("Shaka warning", "line", line)
			} else {
				s.logger.Debug("Shaka output", "line", line)
			}
		}
	}()
	
	return s.shakaCmd.Start()
}

// monitorFFmpeg monitors the FFmpeg process
func (s *ShakaStreamEncoder) monitorFFmpeg() {
	defer s.wg.Done()
	
	err := s.ffmpegCmd.Wait()
	if err != nil && s.ctx.Err() == nil {
		s.logger.Error("FFmpeg failed", "error", err)
		if s.onError != nil {
			s.onError(fmt.Errorf("FFmpeg failed: %w", err))
		}
	} else if err == nil {
		s.logger.Info("FFmpeg completed successfully")
	}
}

// monitorShaka monitors the Shaka process
func (s *ShakaStreamEncoder) monitorShaka() {
	defer s.wg.Done()
	
	err := s.shakaCmd.Wait()
	if err != nil && s.ctx.Err() == nil {
		s.logger.Error("Shaka failed", "error", err)
		if s.onError != nil {
			s.onError(fmt.Errorf("Shaka failed: %w", err))
		}
	} else if err == nil {
		s.logger.Info("Shaka completed successfully")
		if s.onComplete != nil {
			s.onComplete(func(err error) {
				s.logger.Info("Shaka completion callback", "error", err)
			})
		}
	}
}

// monitorOutput monitors for new segments and manifest updates
func (s *ShakaStreamEncoder) monitorOutput() {
	manifestPath := filepath.Join(s.outputDir, "manifest.mpd")
	videoDir := filepath.Join(s.outputDir, "video")
	
	lastSegmentCount := 0
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	
	// Wait for manifest to appear
	manifestReady := false
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			// Check for manifest
			if !manifestReady {
				if _, err := os.Stat(manifestPath); err == nil {
					manifestReady = true
					s.logger.Info("Manifest ready", "path", manifestPath)
					if s.onManifestUpdate != nil {
						s.onManifestUpdate(manifestPath)
					}
				}
			}
			
			// Check for new segments in video directory
			entries, err := os.ReadDir(videoDir)
			if err != nil {
				continue
			}
			
			segmentCount := 0
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), "segment-") && strings.HasSuffix(entry.Name(), ".m4s") {
					segmentCount++
				}
			}
			
			if segmentCount > lastSegmentCount {
				// New segments available - notify about video segments
				for i := lastSegmentCount; i < segmentCount; i++ {
					// Look for the actual segment files (they will have representation ID in the name)
					for _, entry := range entries {
						if strings.Contains(entry.Name(), fmt.Sprintf("-%d.m4s", i+1)) {
							segmentPath := filepath.Join(videoDir, entry.Name())
							if s.onSegmentReady != nil {
								s.onSegmentReady(segmentPath, i)
							}
							break
						}
					}
				}
				
				lastSegmentCount = segmentCount
				
				// Update manifest notification
				if manifestReady && s.onManifestUpdate != nil {
					s.onManifestUpdate(manifestPath)
				}
			}
		}
	}
}

// StopEncoding stops the encoding pipeline
func (s *ShakaStreamEncoder) StopEncoding() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.isRunning {
		return nil
	}
	
	s.logger.Info("Stopping encoder")
	
	// Cancel context
	if s.cancel != nil {
		s.cancel()
	}
	
	// Stop processes gracefully
	s.stopFFmpeg()
	s.stopShakaPackager()
	
	// Wait for goroutines
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// Completed normally
	case <-time.After(10 * time.Second):
		// Force kill if needed
		if s.ffmpegCmd != nil && s.ffmpegCmd.Process != nil {
			s.ffmpegCmd.Process.Kill()
		}
		if s.shakaCmd != nil && s.shakaCmd.Process != nil {
			s.shakaCmd.Process.Kill()
		}
	}
	
	// Clean up pipe
	if s.pipePath != "" {
		os.Remove(s.pipePath)
	}
	
	s.isRunning = false
	return nil
}

// stopFFmpeg stops the FFmpeg process gracefully
func (s *ShakaStreamEncoder) stopFFmpeg() {
	if s.ffmpegCmd != nil && s.ffmpegCmd.Process != nil {
		// Send quit signal to FFmpeg
		s.ffmpegCmd.Process.Signal(syscall.SIGTERM)
		
		// Wait briefly for graceful shutdown
		time.Sleep(2 * time.Second)
		
		// Force kill if still running
		if s.ffmpegCmd.ProcessState == nil {
			s.ffmpegCmd.Process.Kill()
		}
	}
}

// stopShakaPackager stops the Shaka process gracefully
func (s *ShakaStreamEncoder) stopShakaPackager() {
	if s.shakaCmd != nil && s.shakaCmd.Process != nil {
		// Shaka should stop when input closes
		s.shakaCmd.Process.Signal(syscall.SIGTERM)
		
		// Wait briefly
		time.Sleep(1 * time.Second)
		
		// Force kill if still running
		if s.shakaCmd.ProcessState == nil {
			s.shakaCmd.Process.Kill()
		}
	}
}

// SetCallbacks sets the callback functions
func (s *ShakaStreamEncoder) SetCallbacks(onSegment func(string, int), onError func(error)) {
	s.onSegmentReady = onSegment
	s.onError = onError
}

// SetProgressCallback sets the progress callback
func (s *ShakaStreamEncoder) SetProgressCallback(onProgress func(FFmpegProgress)) {
	s.onProgress = onProgress
}

// SetManifestCallback sets the manifest update callback
func (s *ShakaStreamEncoder) SetManifestCallback(onManifest func(string)) {
	s.onManifestUpdate = onManifest
}

// SetCompletionCallback sets the completion callback
func (s *ShakaStreamEncoder) SetCompletionCallback(onComplete func(func(error))) {
	s.onComplete = onComplete
}

// GetManifestPath returns the path to the DASH manifest
func (s *ShakaStreamEncoder) GetManifestPath() string {
	return filepath.Join(s.outputDir, "manifest.mpd")
}