// Package pipeline provides FFmpeg process monitoring and error parsing
package pipeline

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
)

// FFmpegMonitor monitors FFmpeg process health and parses output
type FFmpegMonitor struct {
	logger hclog.Logger
	cmd    *exec.Cmd

	// Progress tracking
	progressCallback func(progress FFmpegProgress)
	errorCallback    func(error)

	// Process state
	isRunning    bool
	startTime    time.Time
	lastProgress time.Time
	mu           sync.RWMutex

	// Output parsing
	progressRegex *regexp.Regexp
	errorPatterns []*regexp.Regexp
}

// FFmpegProgress represents current encoding progress
type FFmpegProgress struct {
	Frame       int           `json:"frame"`
	FPS         float64       `json:"fps"`
	Quality     float64       `json:"quality"`
	Size        int64         `json:"size"`
	Time        time.Duration `json:"time"`
	Bitrate     string        `json:"bitrate"`
	Speed       float64       `json:"speed"`
	Progress    string        `json:"progress"` // "continue" or "end"
	PercentDone float64       `json:"percent_done"`
}

// FFmpegError represents a parsed FFmpeg error
type FFmpegError struct {
	Type        string    `json:"type"`
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
	Level       string    `json:"level"` // "error", "warning", "info"
	Component   string    `json:"component"`
	Recoverable bool      `json:"recoverable"`
}

// NewFFmpegMonitor creates a new FFmpeg process monitor
func NewFFmpegMonitor(logger hclog.Logger) *FFmpegMonitor {
	monitor := &FFmpegMonitor{
		logger:        logger,
		progressRegex: regexp.MustCompile(`^(\w+)=(.+)$`),
	}

	// Compile error patterns
	monitor.errorPatterns = []*regexp.Regexp{
		regexp.MustCompile(`\[error\]\s*(.+)`),
		regexp.MustCompile(`\[fatal\]\s*(.+)`),
		regexp.MustCompile(`Error\s+(.+)`),
		regexp.MustCompile(`Failed\s+(.+)`),
		regexp.MustCompile(`Cannot\s+(.+)`),
		regexp.MustCompile(`No such file or directory:\s*(.+)`),
		regexp.MustCompile(`Permission denied:\s*(.+)`),
		regexp.MustCompile(`Conversion failed!`),
		regexp.MustCompile(`Invalid\s+(.+)`),
		regexp.MustCompile(`Unsupported\s+(.+)`),
	}

	return monitor
}

// SetCallbacks sets progress and error callbacks
func (m *FFmpegMonitor) SetCallbacks(progressCallback func(FFmpegProgress), errorCallback func(error)) {
	m.progressCallback = progressCallback
	m.errorCallback = errorCallback
}

// StartMonitoring begins monitoring an FFmpeg command
func (m *FFmpegMonitor) StartMonitoring(ctx context.Context, cmd *exec.Cmd) error {
	m.mu.Lock()
	m.cmd = cmd
	m.isRunning = true
	m.startTime = time.Now()
	m.lastProgress = time.Now()
	m.mu.Unlock()

	// Create pipes for output capture
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start monitoring goroutines
	go m.monitorProgress(ctx, stdoutPipe)
	go m.monitorErrors(ctx, stderrPipe)
	go m.monitorHealth(ctx)

	m.logger.Info("FFmpeg monitoring started")
	return nil
}

// StopMonitoring stops monitoring
func (m *FFmpegMonitor) StopMonitoring() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.isRunning = false
	m.logger.Info("FFmpeg monitoring stopped")
}

// monitorProgress parses FFmpeg progress output
func (m *FFmpegMonitor) monitorProgress(ctx context.Context, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024) // Increase buffer size

	var currentProgress FFmpegProgress

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Parse progress line
		if m.parseProgressLine(line, &currentProgress) {
			// Update last progress time
			m.mu.Lock()
			m.lastProgress = time.Now()
			m.mu.Unlock()

			// Call progress callback
			if m.progressCallback != nil {
				m.progressCallback(currentProgress)
			}

			m.logger.Debug("FFmpeg progress",
				"frame", currentProgress.Frame,
				"fps", currentProgress.FPS,
				"time", currentProgress.Time,
				"speed", currentProgress.Speed,
			)
		}
	}

	if err := scanner.Err(); err != nil {
		m.logger.Error("Error reading FFmpeg progress", "error", err)
		if m.errorCallback != nil {
			m.errorCallback(fmt.Errorf("progress monitoring failed: %w", err))
		}
	}
}

// monitorErrors parses FFmpeg error output
func (m *FFmpegMonitor) monitorErrors(ctx context.Context, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Parse error line
		if ffmpegError := m.parseErrorLine(line); ffmpegError != nil {
			m.logger.Warn("FFmpeg error detected",
				"type", ffmpegError.Type,
				"level", ffmpegError.Level,
				"message", ffmpegError.Message,
				"recoverable", ffmpegError.Recoverable,
			)

			// Call error callback for non-recoverable errors
			if !ffmpegError.Recoverable && m.errorCallback != nil {
				m.errorCallback(fmt.Errorf("FFmpeg error: %s", ffmpegError.Message))
			}
		} else {
			// Log unrecognized stderr output at debug level
			m.logger.Debug("FFmpeg stderr", "line", line)
		}
	}

	if err := scanner.Err(); err != nil {
		m.logger.Error("Error reading FFmpeg stderr", "error", err)
		if m.errorCallback != nil {
			m.errorCallback(fmt.Errorf("error monitoring failed: %w", err))
		}
	}
}

// monitorHealth checks for process health issues
func (m *FFmpegMonitor) monitorHealth(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkHealth()
		}
	}
}

// checkHealth verifies FFmpeg process health
func (m *FFmpegMonitor) checkHealth() {
	m.mu.RLock()
	isRunning := m.isRunning
	lastProgress := m.lastProgress
	startTime := m.startTime
	cmd := m.cmd
	m.mu.RUnlock()

	if !isRunning {
		return
	}

	now := time.Now()

	// Check if process is still alive by checking ProcessState
	if cmd != nil && cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		m.logger.Info("FFmpeg process completed", "exit_code", cmd.ProcessState.ExitCode())
		// Process completed, stop health monitoring
		m.mu.Lock()
		m.isRunning = false
		m.mu.Unlock()
		return
	}

	// Check for stalled progress (no progress for 30 seconds)
	if now.Sub(lastProgress) > 30*time.Second && now.Sub(startTime) > 10*time.Second {
		m.logger.Warn("FFmpeg appears stalled",
			"last_progress", lastProgress,
			"stalled_duration", now.Sub(lastProgress),
		)
		if m.errorCallback != nil {
			m.errorCallback(fmt.Errorf("FFmpeg stalled for %v", now.Sub(lastProgress)))
		}
	}
}

// parseProgressLine parses a single progress line
func (m *FFmpegMonitor) parseProgressLine(line string, progress *FFmpegProgress) bool {
	matches := m.progressRegex.FindStringSubmatch(line)
	if len(matches) != 3 {
		return false
	}

	key := matches[1]
	value := matches[2]

	switch key {
	case "frame":
		if frame, err := strconv.Atoi(value); err == nil {
			progress.Frame = frame
		}
	case "fps":
		if fps, err := strconv.ParseFloat(value, 64); err == nil {
			progress.FPS = fps
		}
	case "q", "stream_0_0_q":
		if quality, err := strconv.ParseFloat(value, 64); err == nil {
			progress.Quality = quality
		}
	case "size":
		progress.Size = m.parseSize(value)
	case "time":
		if duration, err := m.parseDuration(value); err == nil {
			progress.Time = duration
		}
	case "bitrate":
		progress.Bitrate = value
	case "speed":
		if speed := m.parseSpeed(value); speed > 0 {
			progress.Speed = speed
		}
	case "progress":
		progress.Progress = value
		// Progress line indicates completion of a progress update
		return true
	}

	return false
}

// parseErrorLine parses error messages from FFmpeg stderr
func (m *FFmpegMonitor) parseErrorLine(line string) *FFmpegError {
	now := time.Now()

	// Check each error pattern
	for _, pattern := range m.errorPatterns {
		if matches := pattern.FindStringSubmatch(line); len(matches) > 0 {
			errorType := m.classifyError(line)
			level := m.getErrorLevel(line)
			recoverable := m.isRecoverableError(line)

			return &FFmpegError{
				Type:        errorType,
				Message:     strings.TrimSpace(line),
				Timestamp:   now,
				Level:       level,
				Component:   m.extractComponent(line),
				Recoverable: recoverable,
			}
		}
	}

	// Check for warnings
	if strings.Contains(strings.ToLower(line), "warning") ||
		strings.Contains(line, "[warning]") {
		return &FFmpegError{
			Type:        "warning",
			Message:     line,
			Timestamp:   now,
			Level:       "warning",
			Component:   m.extractComponent(line),
			Recoverable: true,
		}
	}

	return nil
}

// classifyError determines the type of error
func (m *FFmpegMonitor) classifyError(message string) string {
	lower := strings.ToLower(message)

	if strings.Contains(lower, "no such file") || strings.Contains(lower, "not found") {
		return "file_not_found"
	}
	if strings.Contains(lower, "permission denied") || strings.Contains(lower, "access denied") {
		return "permission_error"
	}
	if strings.Contains(lower, "invalid") || strings.Contains(lower, "unsupported") {
		return "format_error"
	}
	if strings.Contains(lower, "memory") || strings.Contains(lower, "allocation") {
		return "memory_error"
	}
	if strings.Contains(lower, "codec") {
		return "codec_error"
	}
	if strings.Contains(lower, "connection") || strings.Contains(lower, "network") {
		return "network_error"
	}

	return "general_error"
}

// getErrorLevel determines error severity
func (m *FFmpegMonitor) getErrorLevel(message string) string {
	lower := strings.ToLower(message)

	if strings.Contains(lower, "fatal") || strings.Contains(lower, "[fatal]") {
		return "fatal"
	}
	if strings.Contains(lower, "error") || strings.Contains(lower, "[error]") {
		return "error"
	}
	if strings.Contains(lower, "warning") || strings.Contains(lower, "[warning]") {
		return "warning"
	}

	return "info"
}

// isRecoverableError determines if an error is recoverable
func (m *FFmpegMonitor) isRecoverableError(message string) bool {
	lower := strings.ToLower(message)

	// Non-recoverable errors
	nonRecoverable := []string{
		"no such file",
		"permission denied",
		"invalid data found",
		"conversion failed",
		"fatal",
		"unable to find",
		"could not open",
	}

	for _, pattern := range nonRecoverable {
		if strings.Contains(lower, pattern) {
			return false
		}
	}

	// Recoverable warnings and info
	return true
}

// extractComponent extracts the FFmpeg component from error message
func (m *FFmpegMonitor) extractComponent(message string) string {
	// Look for component indicators in brackets
	if idx := strings.Index(message, "["); idx != -1 {
		if end := strings.Index(message[idx:], "]"); end != -1 {
			component := message[idx+1 : idx+end]
			if component != "error" && component != "warning" && component != "info" {
				return component
			}
		}
	}

	// Look for common components
	components := []string{"libx264", "aac", "mp4", "segment", "format", "protocol"}
	lower := strings.ToLower(message)

	for _, comp := range components {
		if strings.Contains(lower, comp) {
			return comp
		}
	}

	return "unknown"
}

// parseSize parses FFmpeg size output (e.g., "1024kB", "1MB")
func (m *FFmpegMonitor) parseSize(sizeStr string) int64 {
	if sizeStr == "N/A" || sizeStr == "" {
		return 0
	}

	// Remove 'B' suffix and convert units
	sizeStr = strings.TrimSuffix(sizeStr, "B")

	var multiplier int64 = 1
	if strings.HasSuffix(sizeStr, "k") {
		multiplier = 1024
		sizeStr = strings.TrimSuffix(sizeStr, "k")
	} else if strings.HasSuffix(sizeStr, "M") {
		multiplier = 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "M")
	} else if strings.HasSuffix(sizeStr, "G") {
		multiplier = 1024 * 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "G")
	}

	if size, err := strconv.ParseInt(sizeStr, 10, 64); err == nil {
		return size * multiplier
	}

	return 0
}

// parseDuration parses FFmpeg time format (HH:MM:SS.mmm)
func (m *FFmpegMonitor) parseDuration(timeStr string) (time.Duration, error) {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid time format: %s", timeStr)
	}

	hours, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}

	minutes, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}

	seconds, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return 0, err
	}

	total := time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds*float64(time.Second))

	return total, nil
}

// parseSpeed parses FFmpeg speed indicator (e.g., "2.5x")
func (m *FFmpegMonitor) parseSpeed(speedStr string) float64 {
	if speedStr == "" || speedStr == "N/A" {
		return 0
	}

	speedStr = strings.TrimSuffix(speedStr, "x")
	if speed, err := strconv.ParseFloat(speedStr, 64); err == nil {
		return speed
	}

	return 0
}

// GetProcessInfo returns current process information
func (m *FFmpegMonitor) GetProcessInfo() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	info := map[string]interface{}{
		"running":       m.isRunning,
		"start_time":    m.startTime,
		"last_progress": m.lastProgress,
		"uptime":        time.Since(m.startTime),
	}

	if m.cmd != nil && m.cmd.Process != nil {
		info["pid"] = m.cmd.Process.Pid
	}

	return info
}

// IsHealthy returns true if the process appears healthy
func (m *FFmpegMonitor) IsHealthy() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.isRunning {
		return false
	}

	// Consider healthy if progress was made in last 30 seconds
	return time.Since(m.lastProgress) < 30*time.Second
}
