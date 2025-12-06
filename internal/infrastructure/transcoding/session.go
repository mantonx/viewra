// Package transcoding provides FFmpeg-based video transcoding capabilities.
//
// The TranscodeSession is split across multiple files for better organization:
//   - session.go: Core struct, constructor, Start/Stop lifecycle (this file)
//   - session_segment_watcher.go: Segment detection and waiting logic
//   - session_ffmpeg.go: FFmpeg argument building and codec selection
package transcoding

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// TranscodeSession represents a long-running FFmpeg process for progressive HLS transcoding.
// Implements the Jellyfin-style approach where a single FFmpeg process continuously
// generates segments, and seeking is handled by killing and restarting the process.
type TranscodeSession struct {
	ID              string
	MediaID         int64
	Quality         string
	StartPosition   float64 // Start position in seconds
	AudioTrackIndex int     // Audio track index to mux (0 = first/default, -1 = not specified)

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
// audioTrackIndex specifies which audio track to mux (-1 means use default/first track).
func NewTranscodeSession(
	mediaID int64,
	quality string,
	startPosition float64,
	audioTrackIndex int,
	outputDir string,
	logger *slog.Logger,
	logWriter *LogWriter,
) *TranscodeSession {
	ctx, cancel := context.WithCancel(context.Background())

	sessionID := fmt.Sprintf("%d_%s_%d", mediaID, quality, time.Now().Unix())
	sessionOutputDir := filepath.Join(outputDir, fmt.Sprintf("%d", mediaID), quality)

	// Include audio track in output dir if non-default to separate cached segments
	if audioTrackIndex > 0 {
		sessionOutputDir = filepath.Join(outputDir, fmt.Sprintf("%d", mediaID), fmt.Sprintf("%s_audio%d", quality, audioTrackIndex))
	}

	session := &TranscodeSession{
		ID:                sessionID,
		MediaID:           mediaID,
		Quality:           quality,
		StartPosition:     startPosition,
		AudioTrackIndex:   audioTrackIndex,
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
	go s.monitorStdout(stdout)

	// Log FFmpeg stderr in background with better error visibility
	// Also write to persistent log file for debugging
	go s.monitorStderr(stderr)

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

// monitorStdout monitors FFmpeg stdout for HLS muxer progress.
func (s *TranscodeSession) monitorStdout(stdout io.Reader) {
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
}

// monitorStderr monitors FFmpeg stderr for errors and timing metrics.
func (s *TranscodeSession) monitorStderr(stderr io.Reader) {
	buf := make([]byte, 4096)
	for {
		n, err := stderr.Read(buf)
		if n > 0 {
			chunk := buf[:n]

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
