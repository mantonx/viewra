// Package session provides transcode session lifecycle management.
//
// This package handles:
//   - TranscodeSession: Long-running FFmpeg process management
//   - SessionManager: Session lifecycle, reuse, and cleanup
//   - ProgressWatchdog: Detecting stalled FFmpeg processes
//   - Segment watching: Detecting newly generated HLS segments
package session

import (
	"sync/atomic"
	"time"
)

// ProgressWatchdog monitors FFmpeg progress and kills stalled processes.
// Tracks the last time progress was observed and kills FFmpeg if no progress
// is detected within the timeout period.
type ProgressWatchdog struct {
	lastProgressTime atomic.Int64 // Unix timestamp (nanoseconds) of last progress update
	timeout          time.Duration
	session          *TranscodeSession
	stopChan         chan struct{}
	stoppedChan      chan struct{}
}

// NewProgressWatchdog creates a new watchdog for the given session.
// timeout specifies how long to wait without progress before killing FFmpeg.
// A typical value is 30 seconds.
func NewProgressWatchdog(session *TranscodeSession, timeout time.Duration) *ProgressWatchdog {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	wd := &ProgressWatchdog{
		timeout:     timeout,
		session:     session,
		stopChan:    make(chan struct{}),
		stoppedChan: make(chan struct{}),
	}

	// Initialize last progress time to now
	wd.lastProgressTime.Store(time.Now().UnixNano())

	return wd
}

// UpdateProgress should be called whenever FFmpeg makes progress.
// This resets the watchdog timer.
func (wd *ProgressWatchdog) UpdateProgress() {
	wd.lastProgressTime.Store(time.Now().UnixNano())
}

// Start begins monitoring FFmpeg progress in a background goroutine.
// It checks every timeout/3 (to ensure we catch stalls before timeout expires).
func (wd *ProgressWatchdog) Start() {
	checkInterval := wd.timeout / 3
	if checkInterval < 1*time.Second {
		checkInterval = 1 * time.Second
	}

	go func() {
		defer close(wd.stoppedChan)

		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-wd.stopChan:
				return
			case <-ticker.C:
				wd.checkProgress()
			}
		}
	}()
}

// checkProgress verifies that FFmpeg is making progress.
// If no progress is detected within the timeout period, it logs a warning
// and kills the FFmpeg process.
func (wd *ProgressWatchdog) checkProgress() {
	lastProgress := time.Unix(0, wd.lastProgressTime.Load())
	elapsed := time.Since(lastProgress)

	if elapsed > wd.timeout {
		// No progress detected - FFmpeg may be stalled
		wd.session.logger.Warn("FFmpeg progress watchdog: no progress detected, killing process",
			"session_id", wd.session.ID,
			"media_id", wd.session.MediaID,
			"elapsed_since_progress", elapsed,
			"timeout", wd.timeout)

		// Kill the FFmpeg process
		if wd.session.FFmpegCmd != nil && wd.session.FFmpegCmd.Process != nil {
			if err := wd.session.FFmpegCmd.Process.Kill(); err != nil {
				wd.session.logger.Error("Failed to kill stalled FFmpeg process",
					"session_id", wd.session.ID,
					"error", err)
			} else {
				wd.session.logger.Info("Killed stalled FFmpeg process",
					"session_id", wd.session.ID,
					"stall_duration", elapsed)
			}
		}

		// Stop monitoring since we've killed the process
		wd.Stop()
	}
}

// Stop stops the watchdog monitoring.
func (wd *ProgressWatchdog) Stop() {
	select {
	case <-wd.stopChan:
		// Already stopped
		return
	default:
		close(wd.stopChan)
	}
}

// Wait blocks until the watchdog has fully stopped.
func (wd *ProgressWatchdog) Wait() {
	<-wd.stoppedChan
}
