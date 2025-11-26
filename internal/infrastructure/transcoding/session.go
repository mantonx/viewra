package transcoding

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// TranscodeSession represents a long-running FFmpeg process for progressive HLS transcoding.
// Implements the Jellyfin-style approach where a single FFmpeg process continuously
// generates segments, and seeking is handled by killing and restarting the process.
type TranscodeSession struct {
	ID            string
	MediaID       int64
	Quality       string
	StartPosition float64 // Start position in seconds

	FFmpegCmd    *exec.Cmd
	OutputDir    string
	ManifestPath string

	CreatedAt    time.Time
	LastAccessed time.Time

	// Segment tracking
	segmentMutex      sync.RWMutex
	generatedSegments map[int]bool // Track which segments have been generated

	// Process management
	ctx    context.Context
	cancel context.CancelFunc
	logger *slog.Logger
}

// NewTranscodeSession creates a new transcode session but does not start it.
func NewTranscodeSession(
	mediaID int64,
	quality string,
	startPosition float64,
	outputDir string,
	logger *slog.Logger,
) *TranscodeSession {
	ctx, cancel := context.WithCancel(context.Background())

	sessionID := fmt.Sprintf("%d_%s_%d", mediaID, quality, time.Now().Unix())
	sessionOutputDir := filepath.Join(outputDir, fmt.Sprintf("%d", mediaID), quality)

	return &TranscodeSession{
		ID:                sessionID,
		MediaID:           mediaID,
		Quality:           quality,
		StartPosition:     startPosition,
		OutputDir:         sessionOutputDir,
		ManifestPath:      filepath.Join(sessionOutputDir, "playlist.m3u8"),
		CreatedAt:         time.Now(),
		LastAccessed:      time.Now(),
		generatedSegments: make(map[int]bool),
		ctx:               ctx,
		cancel:            cancel,
		logger:            logger,
	}
}

// Start begins the FFmpeg transcoding process.
// FFmpeg will run continuously, writing segments progressively to the output directory.
func (s *TranscodeSession) Start(inputPath string, profile *AdaptiveProfile, strategy StreamStrategy, hwAccel HardwareAccel, hwDevice string, videoInfo *VideoInfo, config *TranscodeConfig) error {
	// Create output directory
	if err := os.MkdirAll(s.OutputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Build FFmpeg arguments
	args := s.buildFFmpegArgs(inputPath, profile, strategy, hwAccel, hwDevice, videoInfo, config)

	// FFmpeg command available via debug logging if needed
	s.logger.Debug("Starting FFmpeg process",
		"session_id", s.ID,
		"command", fmt.Sprintf("ffmpeg %s", strings.Join(args, " ")))

	// Create FFmpeg command
	s.FFmpegCmd = exec.CommandContext(s.ctx, "ffmpeg", args...)

	// Capture both stdout and stderr
	stderr, err := s.FFmpegCmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	stdout, err := s.FFmpegCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Start the process
	if err := s.FFmpegCmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	// Monitor stdout for HLS muxer progress (at DEBUG level)
	go func() {
		scanner := bufio.NewScanner(stdout)
		segmentCount := 0
		playlistUpdates := 0

		for scanner.Scan() {
			line := scanner.Text()

			// Count segment creations (.ts files)
			if strings.Contains(line, "Opening") && strings.Contains(line, ".ts'") {
				segmentCount++
				// Log every 10 segments to avoid spam
				if segmentCount%10 == 0 {
					s.logger.Debug("HLS encoding progress",
						"session_id", s.ID,
						"segments_created", segmentCount)
				}
			} else if strings.Contains(line, "playlist.m3u8") && strings.Contains(line, "Opening") {
				// Playlist updates
				playlistUpdates++
				s.logger.Debug("HLS playlist updated",
					"session_id", s.ID,
					"update_num", playlistUpdates,
					"total_segments", segmentCount)
			}
		}

		// Log final stats when stdout closes
		if segmentCount > 0 || playlistUpdates > 0 {
			s.logger.Debug("HLS encoding completed",
				"session_id", s.ID,
				"total_segments", segmentCount,
				"playlist_updates", playlistUpdates)
		}
	}()

	// Log FFmpeg stderr in background with better error visibility
	go func() {
		buf := make([]byte, 4096)
		var stderrBuffer []byte
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				chunk := buf[:n]
				stderrBuffer = append(stderrBuffer, chunk...)

				// Only log errors, suppress verbose FFmpeg output
				output := string(chunk)
				if strings.Contains(strings.ToLower(output), "error") ||
					strings.Contains(strings.ToLower(output), "failed") ||
					strings.Contains(strings.ToLower(output), "invalid") {
					s.logger.Error("FFmpeg error detected", "session_id", s.ID, "output", output)
				}
				// Verbose FFmpeg output suppressed - full stderr only logged if process fails
			}
			if err != nil {
				// stderr pipe closed - don't log buffer here, wait for process exit status
				return
			}
		}
	}()

	// Monitor process exit status in background
	go func() {
		err := s.FFmpegCmd.Wait()
		if err != nil {
			s.logger.Error("FFmpeg process exited with error",
				"session_id", s.ID,
				"media_id", s.MediaID,
				"error", err)
		} else {
			s.logger.Info("FFmpeg process completed successfully",
				"session_id", s.ID,
				"media_id", s.MediaID)
		}
	}()

	s.logger.Info("Transcode session started",
		"session_id", s.ID,
		"media_id", s.MediaID,
		"quality", s.Quality,
		"start_position", s.StartPosition)

	// Start watching for generated segments
	go s.watchSegments()

	return nil
}

// Stop kills the FFmpeg process gracefully.
func (s *TranscodeSession) Stop() error {
	s.logger.Info("Stopping transcode session", "session_id", s.ID)

	// Cancel context to stop watchers
	s.cancel()

	// Try graceful shutdown first
	if s.FFmpegCmd != nil && s.FFmpegCmd.Process != nil {
		// Send SIGTERM for graceful shutdown
		if err := s.FFmpegCmd.Process.Signal(os.Interrupt); err != nil {
			s.logger.Warn("Failed to send interrupt signal", "error", err)
		}

		// Wait briefly for graceful shutdown
		time.Sleep(100 * time.Millisecond)

		// Force kill if still running
		if err := s.FFmpegCmd.Process.Kill(); err != nil {
			// Process may have already exited
			s.logger.Debug("Process kill failed (may have already exited)", "error", err)
		}

		// CRITICAL: Wait for process to prevent zombie processes
		// This reaps the process and cleans up the OS process table entry
		go func() {
			if err := s.FFmpegCmd.Wait(); err != nil {
				s.logger.Debug("FFmpeg process wait completed", "error", err)
			}
		}()
	}

	return nil
}

// watchSegments monitors the output directory for newly generated segments.
// Runs in background and updates the generatedSegments map.
func (s *TranscodeSession) watchSegments() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			// Scan output directory for .ts files
			pattern := filepath.Join(s.OutputDir, "seg_*.ts")
			files, err := filepath.Glob(pattern)
			if err != nil {
				continue
			}

			s.segmentMutex.Lock()
			for _, file := range files {
				segNum := ParseSegmentNumber(filepath.Base(file))
				if segNum >= 0 {
					s.generatedSegments[segNum] = true
				}
			}
			s.segmentMutex.Unlock()
		}
	}
}

// WaitForSegment blocks until the specified segment is available or timeout occurs.
// Returns the absolute path to the segment file.
func (s *TranscodeSession) WaitForSegment(segmentNum int, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		s.segmentMutex.RLock()
		exists := s.generatedSegments[segmentNum]
		s.segmentMutex.RUnlock()

		if exists {
			segmentPath := filepath.Join(s.OutputDir, fmt.Sprintf("seg_%06d.ts", segmentNum))
			return segmentPath, nil
		}

		// Check if process has died
		if s.FFmpegCmd != nil && s.FFmpegCmd.ProcessState != nil && s.FFmpegCmd.ProcessState.Exited() {
			return "", fmt.Errorf("ffmpeg process has exited")
		}

		// Poll every 50ms for faster segment delivery (reduced from 100ms)
		time.Sleep(50 * time.Millisecond)
	}

	return "", fmt.Errorf("timeout waiting for segment %d", segmentNum)
}

// WaitForManifest blocks until the manifest file exists or timeout occurs.
// Returns an error if the manifest doesn't appear within the timeout.
func (s *TranscodeSession) WaitForManifest(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		// Check if manifest file exists
		if _, err := os.Stat(s.ManifestPath); err == nil {
			return nil
		}

		// Check if process has died
		if s.FFmpegCmd != nil && s.FFmpegCmd.ProcessState != nil && s.FFmpegCmd.ProcessState.Exited() {
			return fmt.Errorf("ffmpeg process exited before creating manifest")
		}

		// Poll every 50ms for faster startup (reduced from 100ms)
		time.Sleep(50 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for manifest file to be created")
}

// UpdateLastAccessed updates the last accessed timestamp.
func (s *TranscodeSession) UpdateLastAccessed() {
	s.LastAccessed = time.Now()
}

// buildFFmpegArgs builds the FFmpeg command arguments for progressive HLS transcoding.
// Supports hardware acceleration for Transcode strategy.
func (s *TranscodeSession) buildFFmpegArgs(inputPath string, profile *AdaptiveProfile, strategy StreamStrategy, hwAccel HardwareAccel, hwDevice string, videoInfo *VideoInfo, config *TranscodeConfig) []string {
	// Determine target codec from profile (e.g., h265 for 4K profiles)
	targetCodec := CodecH264 // Default fallback
	if profile != nil && profile.PreferredCodec != "" {
		targetCodec = VideoCodec(profile.PreferredCodec)
	}

	// Create TranscodeOptions from session data
	opts := TranscodeOptions{
		InputPath:                  inputPath,
		OutputDir:                  s.OutputDir,
		Profile:                    profile,
		UseStartPosition:           s.StartPosition > 0,
		StartPosition:              int(s.StartPosition),
		VideoInfo:                  videoInfo,
		ToneMappingEnabled:         config.ToneMappingEnabled,
		ToneMappingAlgorithm:       config.ToneMappingAlgorithm,
		ToneMappingBackend:         config.ToneMappingBackend,
		LibPlaceboPeakDetect:       config.LibPlaceboPeakDetect,
		LibPlaceboContrastRecovery: config.LibPlaceboContrastRecovery,
		VideoCodec:                 targetCodec,
	}

	// Build arguments based on strategy
	builder := NewFFmpegArgsBuilder(opts)

	// Add hardware acceleration args (if not None)
	if hwAccel != AccelNone && strategy == Transcode {
		builder.AddHardwareAccel(GetHardwareAccelArgsWithDevice(hwAccel, hwDevice))

		// For NVENC and QSV with HDR content, initialize OpenCL device for GPU tone mapping
		// Both use tonemap_opencl for algorithm selection (matches Jellyfin/Emby approach)
		// Only needed if not using libplacebo (which uses Vulkan instead)
		if (hwAccel == AccelNVENC || hwAccel == AccelQSV) && config.ToneMappingEnabled && videoInfo != nil && videoInfo.IsHDR {
			// Check if we need OpenCL (not using libplacebo)
			backend := config.ToneMappingBackend
			if backend == "" {
				backend = "auto"
			}
			if backend == "opencl" || (backend == "auto" && hwAccel == AccelQSV) {
				builder.AddOpenCLDevice().AddOpenCLFilterDevice()
			}
		}
	}

	builder.AddSeekPosition().AddInput()

	// Note: Progressive transcoding doesn't use stream mapping - it transcodes all streams
	// Add encoding based on strategy
	switch strategy {
	case Remux:
		// Copy both streams (no re-encoding)
		builder.AddVideoCodec("copy", "").AddAudioCodec("copy")

	case RemuxWithAudioDownmix:
		// Copy video, downmix audio
		builder.AddVideoCodec("copy", "").AddAudioDownmix()

	case Transcode:
		// Get encoder and preset based on hardware acceleration and target codec
		// targetCodec was computed at function start and set in opts.VideoCodec
		videoEncoder, videoPreset := GetVideoCodecAndPresetForCodec(hwAccel, targetCodec)

		// For real-time progressive transcoding, override software preset to veryfast
		if hwAccel == AccelNone {
			videoPreset = "veryfast"
		}

		s.logger.Debug("Selected video encoder",
			"session_id", s.ID,
			"target_codec", targetCodec,
			"encoder", videoEncoder,
			"preset", videoPreset,
			"hw_accel", hwAccel)

		builder.AddVideoCodec(videoEncoder, videoPreset)

		// Use hardware or software encoding
		if hwAccel != AccelNone {
			builder.AddHardwareVideoEncoding(hwAccel)
		} else {
			// Software encoding - use veryfast preset for real-time
			builder.AddVideoEncoding()
		}

		builder.AddAudioEncoding()
	}

	// Add HLS output settings and manifest path
	builder.AddHLSOutput().AddOverwriteOutput()

	// Progressive sessions write directly to manifest path (not using AddOutputFile)
	args := builder.Build()
	args = append(args, s.ManifestPath)

	return args
}
