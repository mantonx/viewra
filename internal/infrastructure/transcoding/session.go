package transcoding

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
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

	// Segment configuration
	SegmentDurationSec int // Segment duration in seconds (used to calculate first segment number)

	// Timing metrics for debugging startup latency
	FFmpegStartedAt    time.Time // When FFmpeg process was started
	FirstFrameAt       time.Time // When first frame was processed
	FirstSegmentAt     time.Time // When first segment was written
	ManifestReadyAt    time.Time // When manifest became available
	firstFrameLogged   bool      // Whether we've logged first frame timing
	firstSegmentLogged bool      // Whether we've logged first segment timing

	// Segment tracking
	segmentMutex      sync.RWMutex
	generatedSegments map[int]bool // Track which segments have been generated
	segmentCond       *sync.Cond   // Condition variable for instant segment notification

	// File system watcher for instant segment detection
	// Uses inotify (Linux), FSEvents (macOS), or ReadDirectoryChangesW (Windows)
	fsWatcher *fsnotify.Watcher

	// Process management
	ctx      context.Context
	cancel   context.CancelFunc
	logger   *slog.Logger
	waitOnce sync.Once // Ensure Wait() is only called once

	// FFmpeg log capture for debugging
	logWriter *LogWriter
}

// NewTranscodeSession creates a new transcode session but does not start it.
// The logWriter parameter is optional - if nil, FFmpeg output will still be processed
// but not persisted for later access.
func NewTranscodeSession(
	mediaID int64,
	quality string,
	startPosition float64,
	outputDir string,
	logger *slog.Logger,
	logWriter *LogWriter,
) *TranscodeSession {
	ctx, cancel := context.WithCancel(context.Background())

	sessionID := fmt.Sprintf("%d_%s_%d", mediaID, quality, time.Now().Unix())
	sessionOutputDir := filepath.Join(outputDir, fmt.Sprintf("%d", mediaID), quality)

	session := &TranscodeSession{
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
		logWriter:         logWriter,
	}

	// Initialize condition variable for instant segment notification
	session.segmentCond = sync.NewCond(&session.segmentMutex)

	return session
}

// Start begins the FFmpeg transcoding process.
// FFmpeg will run continuously, writing segments progressively to the output directory.
// clientSupportedCodecs is used to select the best codec from the profile's preferred/fallback list.
func (s *TranscodeSession) Start(inputPath string, profile *AdaptiveProfile, strategy StreamStrategy, hwAccel HardwareAccel, hwDevice string, videoInfo *VideoInfo, config *TranscodeConfig, clientSupportedCodecs []string) error {
	// Create output directory
	if err := os.MkdirAll(s.OutputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Store segment duration for WaitForManifest calculation
	s.SegmentDurationSec = profile.SegmentDuration

	// Build FFmpeg arguments
	args := s.buildFFmpegArgs(inputPath, profile, strategy, hwAccel, hwDevice, videoInfo, config, clientSupportedCodecs)

	// Log FFmpeg command for debugging
	ffmpegCommand := fmt.Sprintf("ffmpeg %s", strings.Join(args, " "))
	s.logger.Debug("Starting FFmpeg process",
		"session_id", s.ID,
		"command", ffmpegCommand)

	// Write command to persistent log file
	if s.logWriter != nil {
		s.logWriter.WriteString(fmt.Sprintf("Command: %s\n\n", ffmpegCommand))
	}

	// Create FFmpeg command with memory limits via systemd-run (if available)
	// This provides a hard cgroup limit as a safety net against memory spikes
	s.FFmpegCmd = createFFmpegCommand(s.ctx, args, config, s.logger)

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
	s.FFmpegStartedAt = time.Now()
	if err := s.FFmpegCmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	// Log startup timing to persistent log
	if s.logWriter != nil {
		s.logWriter.WriteString(fmt.Sprintf("\n# TIMING: FFmpeg started at %s (%.2fms after session creation)\n",
			s.FFmpegStartedAt.Format("15:04:05.000"),
			float64(s.FFmpegStartedAt.Sub(s.CreatedAt).Microseconds())/1000))
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
	// Also write to persistent log file for debugging
	go func() {
		buf := make([]byte, 4096)
		var stderrBuffer []byte
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				chunk := buf[:n]
				stderrBuffer = append(stderrBuffer, chunk...)

				// Write to persistent log file (if available)
				if s.logWriter != nil {
					s.logWriter.Write(chunk)
				}

				// Track timing metrics from FFmpeg progress output
				output := string(chunk)

				// Detect first frame: look for "frame=" in progress output
				if !s.firstFrameLogged && strings.Contains(output, "frame=") && !strings.Contains(output, "frame=    0") {
					s.FirstFrameAt = time.Now()
					s.firstFrameLogged = true
					timeSinceStart := s.FirstFrameAt.Sub(s.FFmpegStartedAt)
					s.logger.Info("FFmpeg first frame",
						"session_id", s.ID,
						"time_to_first_frame_ms", timeSinceStart.Milliseconds())
					if s.logWriter != nil {
						s.logWriter.WriteString(fmt.Sprintf("\n# TIMING: First frame at %s (%.0fms after FFmpeg start)\n",
							s.FirstFrameAt.Format("15:04:05.000"),
							float64(timeSinceStart.Milliseconds())))
					}
				}

				// Detect first segment: look for seg_000000.ts being opened
				if !s.firstSegmentLogged && strings.Contains(output, "seg_000000.ts") {
					s.FirstSegmentAt = time.Now()
					s.firstSegmentLogged = true
					timeSinceStart := s.FirstSegmentAt.Sub(s.FFmpegStartedAt)
					s.logger.Info("FFmpeg first segment",
						"session_id", s.ID,
						"time_to_first_segment_ms", timeSinceStart.Milliseconds())
					if s.logWriter != nil {
						s.logWriter.WriteString(fmt.Sprintf("\n# TIMING: First segment at %s (%.0fms after FFmpeg start)\n",
							s.FirstSegmentAt.Format("15:04:05.000"),
							float64(timeSinceStart.Milliseconds())))
					}
				}

				// Classify FFmpeg output: real errors vs informational warnings
				lowerOutput := strings.ToLower(output)

				// Known non-fatal warnings that shouldn't be logged as errors
				isNonFatalWarning := strings.Contains(output, "Invalid Block Addition") || // Dolby Vision metadata
					strings.Contains(output, "Discarding interleaved") ||
					strings.Contains(output, "discarding unsupported")

				if isNonFatalWarning {
					s.logger.Warn("FFmpeg warning", "session_id", s.ID, "output", output)
				} else if strings.Contains(lowerOutput, "error") ||
					strings.Contains(lowerOutput, "failed") ||
					strings.Contains(lowerOutput, "invalid") {
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
		s.waitOnce.Do(func() {
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
		})
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

// Stop kills the FFmpeg process gracefully and waits for it to exit.
// This method blocks until the FFmpeg process has fully terminated to ensure
// it's safe to clean up output files afterward.
func (s *TranscodeSession) Stop() error {
	s.logger.Info("Stopping transcode session", "session_id", s.ID)

	// Cancel context to stop watchers
	s.cancel()

	// Close fsnotify watcher if active
	if s.fsWatcher != nil {
		s.fsWatcher.Close()
	}

	// Wake up any waiting goroutines so they can exit
	s.segmentCond.Broadcast()

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

		// CRITICAL: Wait synchronously for process to fully exit before returning.
		// This ensures the output directory can be safely cleaned up afterward.
		// Use waitOnce to avoid "no child processes" error if monitoring goroutine already called Wait()
		s.waitOnce.Do(func() {
			if err := s.FFmpegCmd.Wait(); err != nil {
				s.logger.Debug("FFmpeg process wait completed", "error", err)
			}
		})
	}

	return nil
}

// watchSegments monitors the output directory for newly generated segments.
// Uses inotify (Linux) / fsnotify for instant detection, with polling fallback.
func (s *TranscodeSession) watchSegments() {
	// Try to set up inotify-based watching for instant segment detection
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		s.logger.Debug("Failed to create fsnotify watcher, falling back to polling",
			"session_id", s.ID,
			"error", err)
		s.watchSegmentsPolling()
		return
	}

	// Add watch on output directory
	if err := watcher.Add(s.OutputDir); err != nil {
		watcher.Close()
		s.logger.Debug("Failed to watch output directory, falling back to polling",
			"session_id", s.ID,
			"path", s.OutputDir,
			"error", err)
		s.watchSegmentsPolling()
		return
	}

	s.fsWatcher = watcher
	s.logger.Debug("Using fsnotify for instant segment detection",
		"session_id", s.ID,
		"path", s.OutputDir)

	defer watcher.Close()

	for {
		select {
		case <-s.ctx.Done():
			return

		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			// Only process Create and Write events for .ts files
			if event.Op&(fsnotify.Create|fsnotify.Write) != 0 {
				filename := filepath.Base(event.Name)
				if strings.HasPrefix(filename, "seg_") && strings.HasSuffix(filename, ".ts") {
					segNum := ParseSegmentNumber(filename)
					if segNum >= 0 {
						s.segmentMutex.Lock()
						if !s.generatedSegments[segNum] {
							s.generatedSegments[segNum] = true
							// Broadcast to all waiting goroutines
							s.segmentCond.Broadcast()
						}
						s.segmentMutex.Unlock()
					}
				}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			s.logger.Warn("fsnotify error", "session_id", s.ID, "error", err)
		}
	}
}

// watchSegmentsPolling is the fallback polling-based segment watcher.
// Used when inotify is not available (e.g., NFS mounts, or watcher creation fails).
func (s *TranscodeSession) watchSegmentsPolling() {
	ticker := time.NewTicker(100 * time.Millisecond) // Faster polling as fallback
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			// Scan output directory for MPEG-TS segment files
			pattern := filepath.Join(s.OutputDir, "seg_*.ts")
			files, err := filepath.Glob(pattern)
			if err != nil {
				continue
			}

			s.segmentMutex.Lock()
			newSegments := false
			for _, file := range files {
				segNum := ParseSegmentNumber(filepath.Base(file))
				if segNum >= 0 && !s.generatedSegments[segNum] {
					s.generatedSegments[segNum] = true
					newSegments = true
				}
			}
			if newSegments {
				s.segmentCond.Broadcast()
			}
			s.segmentMutex.Unlock()
		}
	}
}

// WaitForSegment blocks until the specified segment is available or timeout occurs.
// Uses condition variable for instant notification when inotify detects new segments.
// Returns the absolute path to the segment file.
func (s *TranscodeSession) WaitForSegment(segmentNum int, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)

	s.segmentMutex.Lock()
	defer s.segmentMutex.Unlock()

	for {
		// Check if segment exists
		if s.generatedSegments[segmentNum] {
			segmentPath := filepath.Join(s.OutputDir, SegmentFilename(segmentNum))
			return segmentPath, nil
		}

		// Check if process has died
		if s.FFmpegCmd != nil && s.FFmpegCmd.ProcessState != nil && s.FFmpegCmd.ProcessState.Exited() {
			return "", fmt.Errorf("ffmpeg process has exited")
		}

		// Check timeout
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}

		// Wait for notification with timeout
		// Use a goroutine to implement timeout since sync.Cond doesn't support it directly
		done := make(chan struct{})
		go func() {
			select {
			case <-time.After(remaining):
				// Timeout - wake up the waiting goroutine
				s.segmentCond.Broadcast()
			case <-done:
				// Segment arrived or context cancelled
			}
		}()

		s.segmentCond.Wait()
		close(done)
	}

	return "", fmt.Errorf("timeout waiting for segment %d", segmentNum)
}

// WaitForManifest blocks until the manifest AND first segment exist.
// This ensures HLS.js has actual playable content, not just an empty manifest.
// Returns an error if the manifest doesn't become playable within the timeout.
func (s *TranscodeSession) WaitForManifest(timeout time.Duration) error {
	callStartTime := time.Now()
	deadline := callStartTime.Add(timeout)

	// Calculate first segment number based on start position
	// Must match FFmpeg's -start_number calculation
	segmentDuration := s.SegmentDurationSec
	if segmentDuration <= 0 {
		segmentDuration = SegmentDuration // Default to 2 seconds if not set
	}
	firstSegmentNum := 0
	if s.StartPosition > 0 {
		firstSegmentNum = int(s.StartPosition) / segmentDuration
	}
	firstSegmentPath := filepath.Join(s.OutputDir, SegmentFilename(firstSegmentNum))

	s.logger.Debug("Waiting for manifest and first segment",
		"session_id", s.ID,
		"start_position", s.StartPosition,
		"first_segment_num", firstSegmentNum,
		"first_segment_path", firstSegmentPath)

	manifestExists := false
	firstSegmentExists := false

	for time.Now().Before(deadline) {
		// Check if manifest file exists AND was created by THIS session
		// (not a stale manifest from a previous session with different seek position)
		if !manifestExists {
			if info, err := os.Stat(s.ManifestPath); err == nil {
				// Only accept manifests created after FFmpeg started for this session
				if info.ModTime().After(s.FFmpegStartedAt) || info.ModTime().Equal(s.FFmpegStartedAt) {
					manifestExists = true
					s.logger.Debug("Manifest file created, waiting for first segment",
						"session_id", s.ID,
						"manifest_path", s.ManifestPath)
				}
			}
		}

		// Check if first segment exists (this is what HLS.js actually needs)
		// IMPORTANT: Verify the segment was created AFTER this session started.
		// This prevents a race condition where old segments from a previous session
		// (with the same segment numbers due to overlapping seek positions) are
		// mistakenly detected as valid, causing the wrong manifest to be served.
		if manifestExists && !firstSegmentExists {
			if info, err := os.Stat(firstSegmentPath); err == nil && info.Size() > 0 {
				// Only accept segments created after FFmpeg started for this session
				if info.ModTime().After(s.FFmpegStartedAt) || info.ModTime().Equal(s.FFmpegStartedAt) {
					firstSegmentExists = true
				}
			}
		}

		// Ready when both manifest and first segment exist
		if manifestExists && firstSegmentExists {
			now := time.Now()
			waitDuration := now.Sub(callStartTime)

			// Only log timing info on first manifest ready (when ManifestReadyAt is zero)
			// This avoids misleading logs when session is reused
			isFirstReady := s.ManifestReadyAt.IsZero()
			if isFirstReady {
				s.ManifestReadyAt = now
				timeSinceCreation := now.Sub(s.CreatedAt)
				timeSinceFFmpegStart := now.Sub(s.FFmpegStartedAt)

				s.logger.Info("Manifest ready with first segment",
					"session_id", s.ID,
					"first_segment", firstSegmentNum,
					"wait_ms", waitDuration.Milliseconds(),
					"time_since_session_ms", timeSinceCreation.Milliseconds(),
					"time_from_ffmpeg_start_ms", timeSinceFFmpegStart.Milliseconds())

				if s.logWriter != nil {
					s.logWriter.WriteString(fmt.Sprintf("\n# TIMING: Manifest ready at %s (%.0fms wait, %.0fms since session, %.0fms after FFmpeg start)\n",
						now.Format("15:04:05.000"),
						float64(waitDuration.Milliseconds()),
						float64(timeSinceCreation.Milliseconds()),
						float64(timeSinceFFmpegStart.Milliseconds())))
				}
			} else {
				// Session already had manifest ready - this is a reused session
				s.logger.Debug("Manifest already available (session reused)",
					"session_id", s.ID,
					"first_segment", firstSegmentNum,
					"wait_ms", waitDuration.Milliseconds())
			}
			return nil
		}

		// Check if process has died
		if s.FFmpegCmd != nil && s.FFmpegCmd.ProcessState != nil && s.FFmpegCmd.ProcessState.Exited() {
			return fmt.Errorf("ffmpeg process exited before creating manifest")
		}

		// Poll every 50ms for faster startup
		time.Sleep(50 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for manifest file to be created")
}

// WaitForInitSegment blocks until the fMP4 init segment exists or timeout occurs.
// Returns the path to the init segment file.
func (s *TranscodeSession) WaitForInitSegment(timeout time.Duration) (string, error) {
	initPath := filepath.Join(s.OutputDir, InitSegmentFilename)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		// Check if init segment exists
		if _, err := os.Stat(initPath); err == nil {
			return initPath, nil
		}

		// Check if process has died
		if s.FFmpegCmd != nil && s.FFmpegCmd.ProcessState != nil && s.FFmpegCmd.ProcessState.Exited() {
			return "", fmt.Errorf("ffmpeg process exited before creating init segment")
		}

		// Poll every 50ms
		time.Sleep(50 * time.Millisecond)
	}

	return "", fmt.Errorf("timeout waiting for init segment")
}

// UpdateLastAccessed updates the last accessed timestamp.
func (s *TranscodeSession) UpdateLastAccessed() {
	s.LastAccessed = time.Now()
}

// GetLogWriter returns the log writer for this session (may be nil).
func (s *TranscodeSession) GetLogWriter() *LogWriter {
	return s.logWriter
}

// SetLogWriter sets the log writer for this session.
func (s *TranscodeSession) SetLogWriter(w *LogWriter) {
	s.logWriter = w
}

// buildFFmpegArgs builds the FFmpeg command arguments for progressive HLS transcoding.
// Supports hardware acceleration for Transcode strategy.
// clientSupportedCodecs is used to select the best codec from the profile's preferred/fallback list.
func (s *TranscodeSession) buildFFmpegArgs(inputPath string, profile *AdaptiveProfile, strategy StreamStrategy, hwAccel HardwareAccel, hwDevice string, videoInfo *VideoInfo, config *TranscodeConfig, clientSupportedCodecs []string) []string {
	// Determine target codec based on client support and profile preferences
	// This ensures we encode to a codec the client can actually play
	targetCodec := selectBestCodec(profile, clientSupportedCodecs, hwAccel)

	s.logger.Debug("Selected target codec for transcoding",
		"session_id", s.ID,
		"target_codec", targetCodec,
		"profile_preferred", profile.PreferredCodec,
		"profile_fallbacks", profile.FallbackCodecs,
		"client_codecs", clientSupportedCodecs,
		"hw_accel", hwAccel)

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
		// Both use tonemap_opencl for fast HDR→SDR conversion
		// Only needed if not using libplacebo (which uses Vulkan instead)
		if (hwAccel == AccelNVENC || hwAccel == AccelQSV) && config.ToneMappingEnabled && videoInfo != nil && videoInfo.IsHDR {
			// Check if we need OpenCL (not using libplacebo)
			backend := config.ToneMappingBackend
			if backend == "" {
				backend = "auto"
			}
			// In auto mode, both NVENC and QSV use OpenCL tone mapping (not libplacebo)
			// libplacebo is only used for software encoding due to GPU transfer overhead
			if backend == "opencl" || backend == "auto" {
				builder.AddOpenCLDevice().AddOpenCLFilterDevice()
			}
		}
	}

	// Add memory safety options for ALL transcoding operations to prevent OOM crashes
	// Memory-intensive operations include: HDR tone mapping (especially libplacebo
	// with peak_detect), 4K scaling, and complex filter chains
	builder.AddMemorySafetyOptions(config.MaxMemoryMB)

	// Determine if we need -noaccurate_seek for A/V sync.
	// When copying video but transcoding audio, FFmpeg seeks video to the nearest keyframe
	// but starts audio from the exact seek position without this flag.
	needNoAccurateSeek := strategy == RemuxWithAudioDownmix || strategy == RemuxHEVC

	builder.AddSeekPosition()
	if needNoAccurateSeek {
		builder.AddNoAccurateSeek()
	}
	builder.AddInput().AddTimestampReset()

	// Add encoding based on strategy
	// IMPORTANT: Always add explicit stream mapping to prevent FFmpeg from auto-selecting
	// all streams (including subtitles), which breaks HLS output by creating separate .vtt files
	//
	// For remux strategies, we use the segment muxer instead of HLS muxer for better
	// timestamp handling when seeking. This requires patched FFmpeg (viewra-ffmpeg)
	// with start_pts tracking fix for correct A/V sync.
	useSegmentMuxer := false

	switch strategy {
	case Remux:
		// Copy both streams with H.264 bitstream filter for MPEG-TS compatibility
		// Use segment muxer for proper A/V sync when seeking
		// Note: Don't use -copyts here - let FFmpeg reset timestamps naturally
		// The segment muxer works better when output timestamps start from 0
		builder.AddStreamMapping().AddH264Copy().AddAudioCodec("copy")
		useSegmentMuxer = true

	case RemuxWithAudioDownmix:
		// Copy video with H.264 bitstream filter, transcode audio to AAC
		// Use -noaccurate_seek (added above) to align audio with video keyframe
		builder.AddStreamMapping().AddH264Copy().AddAudioDownmix()
		useSegmentMuxer = true

	case RemuxHEVC:
		// Copy HEVC video with bitstream filter, transcode audio to AAC
		// This is extremely fast (~50x realtime) since no video encoding happens
		// The hevc_mp4toannexb filter converts NAL units for MPEG-TS compatibility
		// Use -noaccurate_seek (added above) to align audio with video keyframe
		s.logger.Info("Using HEVC remux strategy",
			"session_id", s.ID,
			"media_id", s.MediaID)
		builder.AddStreamMapping().AddHEVCCopy().AddAudioDownmix()
		useSegmentMuxer = true

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

		builder.AddStreamMapping().AddVideoCodec(videoEncoder, videoPreset)

		// Use hardware or software encoding
		if hwAccel != AccelNone {
			builder.AddHardwareVideoEncoding(hwAccel)
		} else {
			builder.AddVideoEncoding()
		}

		builder.AddAudioEncoding()
		// Note: No need for force_key_frames - the GOP settings (-g/-keyint_min)
		// already ensure frame 0 is a keyframe since it's the start of the first GOP.
	}

	// Add output settings based on muxer type
	var args []string
	if useSegmentMuxer {
		// Use segment muxer for remux strategies (requires patched FFmpeg)
		// This provides proper A/V sync when seeking into the middle of files
		builder.AddSegmentMuxerOutput().AddOverwriteOutput().AddSegmentMuxerOutputFile()
		args = builder.Build()
	} else {
		// Use HLS muxer for transcode (generates new keyframes, so timing is controlled)
		builder.AddHLSOutput().AddOverwriteOutput()
		args = builder.Build()
		args = append(args, s.ManifestPath)
	}

	return args
}

// StartAudioOnly begins an audio-only FFmpeg transcoding process.
// This is used for HLS multi-audio support where each audio track gets its own playlist.
func (s *TranscodeSession) StartAudioOnly(inputPath string, audioTrackIndex int, config *TranscodeConfig, videoInfo *VideoInfo) error {
	// Create output directory
	if err := os.MkdirAll(s.OutputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Audio uses 4-second segments (set before building args for WaitForManifest)
	s.SegmentDurationSec = 4

	// Build FFmpeg arguments for audio-only transcoding
	args := s.buildAudioOnlyFFmpegArgs(inputPath, audioTrackIndex, config, videoInfo)

	// Log FFmpeg command for debugging
	ffmpegCommand := fmt.Sprintf("ffmpeg %s", strings.Join(args, " "))
	s.logger.Debug("Starting audio-only FFmpeg process",
		"session_id", s.ID,
		"audio_track", audioTrackIndex,
		"command", ffmpegCommand)

	// Write command to persistent log file
	if s.logWriter != nil {
		s.logWriter.WriteString(fmt.Sprintf("Audio-only transcode\nTrack: %d\nCommand: %s\n\n", audioTrackIndex, ffmpegCommand))
	}

	// Create FFmpeg command with memory limits via systemd-run (if available)
	s.FFmpegCmd = createFFmpegCommand(s.ctx, args, config, s.logger)

	// Capture stderr for error logging
	stderr, err := s.FFmpegCmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the process
	s.FFmpegStartedAt = time.Now()
	if err := s.FFmpegCmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	// Log startup timing to persistent log
	if s.logWriter != nil {
		s.logWriter.WriteString(fmt.Sprintf("\n# TIMING: FFmpeg started at %s (%.2fms after session creation)\n",
			s.FFmpegStartedAt.Format("15:04:05.000"),
			float64(s.FFmpegStartedAt.Sub(s.CreatedAt).Microseconds())/1000))
	}

	// Log FFmpeg stderr in background
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				chunk := buf[:n]

				// Write to persistent log file (if available)
				if s.logWriter != nil {
					s.logWriter.Write(chunk)
				}

				output := string(chunk)

				// Detect first segment for audio (look for any .ts file being opened)
				if !s.firstSegmentLogged && strings.Contains(output, ".ts'") && strings.Contains(output, "Opening") {
					s.FirstSegmentAt = time.Now()
					s.firstSegmentLogged = true
					timeSinceStart := s.FirstSegmentAt.Sub(s.FFmpegStartedAt)
					s.logger.Info("FFmpeg audio first segment",
						"session_id", s.ID,
						"time_to_first_segment_ms", timeSinceStart.Milliseconds())
					if s.logWriter != nil {
						s.logWriter.WriteString(fmt.Sprintf("\n# TIMING: First audio segment at %s (%.0fms after FFmpeg start)\n",
							s.FirstSegmentAt.Format("15:04:05.000"),
							float64(timeSinceStart.Milliseconds())))
					}
				}

				lowerOutput := strings.ToLower(output)
				if strings.Contains(lowerOutput, "error") || strings.Contains(lowerOutput, "failed") {
					s.logger.Error("FFmpeg audio error", "session_id", s.ID, "output", output)
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// Monitor process exit in background
	go func() {
		s.waitOnce.Do(func() {
			err := s.FFmpegCmd.Wait()
			if err != nil {
				s.logger.Error("FFmpeg audio process exited with error",
					"session_id", s.ID,
					"error", err)
			} else {
				s.logger.Info("FFmpeg audio process completed",
					"session_id", s.ID)
			}
		})
	}()

	s.logger.Info("Audio transcode session started",
		"session_id", s.ID,
		"media_id", s.MediaID,
		"audio_track", audioTrackIndex,
		"start_position", s.StartPosition)

	// Start watching for generated segments (required for WaitForSegment to work)
	go s.watchSegments()

	return nil
}

// buildAudioOnlyFFmpegArgs builds FFmpeg arguments for audio-only HLS transcoding.
func (s *TranscodeSession) buildAudioOnlyFFmpegArgs(inputPath string, audioTrackIndex int, config *TranscodeConfig, videoInfo *VideoInfo) []string {
	args := []string{}

	// Seek position (before input for fast seeking)
	if s.StartPosition > 0 {
		args = append(args, "-ss", fmt.Sprintf("%d", int(s.StartPosition)))
	}

	// Input file
	args = append(args, "-i", inputPath)

	// Map only the specified audio track (0:a:N selects the Nth audio stream)
	args = append(args, "-map", fmt.Sprintf("0:a:%d", audioTrackIndex))

	// No video
	args = append(args, "-vn")

	// Audio encoding: AAC at 128kbps stereo for broad compatibility
	args = append(args,
		"-c:a", "aac",
		"-b:a", "128k",
		"-ac", "2", // Stereo downmix for compatibility
	)

	// Calculate start segment number based on seek position
	// Must match video's segment numbering for proper A/V sync
	// Use 4-second audio segments (hls_time=4), matching video segment math
	startSegmentNum := 0
	if s.StartPosition > 0 {
		// Audio uses 4-second segments, so divide by 4
		startSegmentNum = int(s.StartPosition) / 4
	}

	// HLS output settings (use 6-digit segment numbers to match SegmentFilenameFormat)
	// Use short initial segment (1s) for fast startup, then 4s segments
	args = append(args,
		"-f", "hls",
		"-hls_init_time", "1",
		"-hls_time", "4",
		"-hls_list_size", "0", // Keep all segments in playlist
		"-hls_flags", "independent_segments",
		"-hls_segment_type", "mpegts",
		"-hls_segment_filename", filepath.Join(s.OutputDir, SegmentFilenameFormat),
		"-start_number", fmt.Sprintf("%d", startSegmentNum),
	)

	// Overwrite output
	args = append(args, "-y")

	// Output manifest path
	args = append(args, s.ManifestPath)

	return args
}

// AudioQualityKey returns the quality key used for audio-only sessions.
// This matches the key format used by GetOrCreateAudioSession.
func AudioQualityKey(audioTrackIndex int) string {
	return fmt.Sprintf("audio_%d", audioTrackIndex)
}


// createFFmpegCommand creates an FFmpeg command with memory limits via systemd-run (if available).
// On Linux with systemd, this wraps FFmpeg with cgroup memory limits as a hard safety net
// against memory spikes from subtitle burn-in, HDR tone mapping, or complex filter chains.
// Falls back to regular exec.CommandContext on other platforms or when systemd-run isn't available.
func createFFmpegCommand(ctx context.Context, args []string, config *TranscodeConfig, logger *slog.Logger) *exec.Cmd {
	ffmpegPath := config.FFmpegPath
	libPath := config.FFmpegLibPath
	maxMemoryMB := config.MaxMemoryMB

	// On Linux with systemd, use systemd-run to apply memory limits
	if runtime.GOOS == "linux" && maxMemoryMB > 0 {
		if _, err := exec.LookPath("systemd-run"); err == nil {
			// Apply 2x multiplier for virtual memory headroom (shared libs, mmap, etc.)
			// The -max_alloc FFmpeg flag handles the tight limit; this is a safety net
			limitMB := maxMemoryMB * 2

			// Build systemd-run command with memory limit
			// --scope: Run as a transient scope unit (not a service)
			// --user: Run in user session (no root required)
			// -p MemoryMax: Hard memory limit
			// -p MemorySwapMax=0: Prevent swapping (fail fast rather than thrash)
			// -E: Pass environment variable to the child process
			systemdArgs := []string{
				"--scope",
				"--user",
				"-p", fmt.Sprintf("MemoryMax=%dM", limitMB),
				"-p", "MemorySwapMax=0",
			}

			// Pass LD_LIBRARY_PATH to FFmpeg via systemd-run's -E flag
			// (setting cmd.Env doesn't work because systemd-run spawns a new process)
			if libPath != "" {
				systemdArgs = append(systemdArgs, "-E", "LD_LIBRARY_PATH="+libPath)
			}

			systemdArgs = append(systemdArgs, "--", ffmpegPath)
			systemdArgs = append(systemdArgs, args...)

			logger.Debug("Using systemd-run for memory-limited FFmpeg",
				"memory_limit_mb", limitMB,
				"ffmpeg_max_alloc_mb", maxMemoryMB,
				"ffmpeg_path", ffmpegPath,
				"lib_path", libPath)

			cmd := exec.CommandContext(ctx, "systemd-run", systemdArgs...)
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Setpgid: true, // Create new process group for clean shutdown
			}
			return cmd
		}
	}

	// Fallback: no system-level memory limit, rely on FFmpeg's -max_alloc
	cmd := exec.CommandContext(ctx, ffmpegPath, args...)
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}
	}
	// Set LD_LIBRARY_PATH for custom FFmpeg builds
	if libPath != "" {
		cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+libPath)
	}
	return cmd
}

// selectBestCodec selects the best codec for transcoding based on:
// 1. Client's supported codecs (what the browser can decode)
// 2. Profile's preferred codec and fallbacks
// 3. Hardware acceleration support
// Returns H.264 as the ultimate fallback for universal compatibility.
func selectBestCodec(profile *AdaptiveProfile, clientSupportedCodecs []string, hwAccel HardwareAccel) VideoCodec {
	// If no client codecs provided, assume H.264 only (most conservative)
	if len(clientSupportedCodecs) == 0 {
		clientSupportedCodecs = []string{"h264"}
	}

	// Build a set for fast lookup
	supported := make(map[string]bool)
	for _, codec := range clientSupportedCodecs {
		supported[codec] = true
	}

	// If profile is nil, return H.264
	if profile == nil {
		return CodecH264
	}

	// Check if preferred codec is supported by both client and hardware
	if supported[profile.PreferredCodec] && IsCodecSupported(hwAccel, VideoCodec(profile.PreferredCodec)) {
		return VideoCodec(profile.PreferredCodec)
	}

	// Check fallback codecs in order of preference
	for _, fallback := range profile.FallbackCodecs {
		if supported[fallback] && IsCodecSupported(hwAccel, VideoCodec(fallback)) {
			return VideoCodec(fallback)
		}
	}

	// Ultimate fallback: H.264 (universally supported)
	return CodecH264
}
