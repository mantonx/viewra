package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Segment filename constants
const (
	// SegmentDuration is the duration of each HLS segment in seconds.
	SegmentDuration = 2

	// SegmentFilenameFormat is the format string for segment filenames
	SegmentFilenameFormat = "seg_%06d.ts"

	// InitSegmentFilename is kept for backwards compatibility (not used with MPEG-TS)
	InitSegmentFilename = "init.mp4"
)

// SegmentFilename generates a filename for a segment number.
func SegmentFilename(segmentNumber int) string {
	return fmt.Sprintf(SegmentFilenameFormat, segmentNumber)
}

// ParseSegmentNumber extracts the segment number from a filename.
// Returns -1 if the filename doesn't match the expected pattern.
func ParseSegmentNumber(filename string) int {
	// Match seg_NNNNNN.ts pattern
	if !strings.HasPrefix(filename, "seg_") || !strings.HasSuffix(filename, ".ts") {
		return -1
	}

	numStr := strings.TrimPrefix(filename, "seg_")
	numStr = strings.TrimSuffix(numStr, ".ts")

	var num int
	if _, err := fmt.Sscanf(numStr, "%d", &num); err != nil {
		return -1
	}

	return num
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

	// Create a single timer for the entire wait operation
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	// Channel to signal when we're done waiting (either success or final timeout)
	done := make(chan struct{})
	defer close(done)

	// Start a single goroutine that broadcasts on timer expiration
	// This goroutine exits when done is closed
	go func() {
		select {
		case <-timer.C:
			// Timeout expired - wake up the waiting goroutine
			s.segmentCond.Broadcast()
		case <-done:
			// Wait completed successfully or final timeout reached
		}
	}()

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
		if time.Now().After(deadline) {
			break
		}

		// Wait for notification (will be woken by Broadcast from segment watcher or timer)
		s.segmentCond.Wait()
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

		// Poll every 10ms for minimal startup latency
		// This tight loop is acceptable since WaitForManifest is called once per session
		time.Sleep(10 * time.Millisecond)
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
