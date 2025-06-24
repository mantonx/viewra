// Package utils provides utility functions and helpers for the transcoding SDK.
// This package contains common functionality used across different modules,
// including logging helpers, file operations, and process utilities.
package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mantonx/viewra/sdk/transcoding/types"
)

// LogWriter is a writer that logs output with a prefix
type LogWriter struct {
	Logger types.Logger
	Prefix string
}

// Write implements io.Writer interface
func (w *LogWriter) Write(p []byte) (n int, err error) {
	if w.Logger != nil {
		// Split output into lines and log each one
		lines := strings.Split(string(p), "\n")
		for _, line := range lines {
			if line != "" {
				w.Logger.Debug(w.Prefix + line)
			}
		}
	}
	return len(p), nil
}

// SetupDebugLogging sets up debug logging for a process
func SetupDebugLogging(sessionID string, outputDir string, request interface{}) (*os.File, error) {
	if os.Getenv("FFMPEG_DEBUG") != "true" {
		return nil, nil
	}

	// Create debug directory
	debugDir := filepath.Join(outputDir, "debug")
	if err := os.MkdirAll(debugDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create debug directory: %w", err)
	}

	// Create debug log file
	debugPath := filepath.Join(debugDir, fmt.Sprintf("ffmpeg_%s.log", sessionID))
	debugLog, err := os.Create(debugPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create debug log: %w", err)
	}

	// Write debug header
	fmt.Fprintf(debugLog, "=== FFmpeg Debug Log ===\n")
	fmt.Fprintf(debugLog, "Session: %s\n", sessionID)
	fmt.Fprintf(debugLog, "Time: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(debugLog, "Request: %+v\n", request)
	fmt.Fprintf(debugLog, "\n=== Environment ===\n")

	// Write relevant environment variables
	for _, env := range os.Environ() {
		if strings.Contains(env, "FFMPEG") || strings.Contains(env, "PATH") || strings.Contains(env, "VIEWRA") {
			fmt.Fprintf(debugLog, "%s\n", env)
		}
	}

	fmt.Fprintf(debugLog, "\n=== FFmpeg Output ===\n")

	return debugLog, nil
}

// SetupProcessLogging sets up standard logging files for a process
func SetupProcessLogging(outputDir string, sessionID string) (*os.File, *os.File, error) {
	// Create logs directory
	logDir := filepath.Join(outputDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Create stdout log
	stdoutPath := filepath.Join(logDir, fmt.Sprintf("%s_stdout.log", sessionID))
	stdoutFile, err := os.Create(stdoutPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create stdout log: %w", err)
	}

	// Create stderr log
	stderrPath := filepath.Join(logDir, fmt.Sprintf("%s_stderr.log", sessionID))
	stderrFile, err := os.Create(stderrPath)
	if err != nil {
		stdoutFile.Close()
		return nil, nil, fmt.Errorf("failed to create stderr log: %w", err)
	}

	return stdoutFile, stderrFile, nil
}

// CloseLogFiles safely closes log files
func CloseLogFiles(files ...*os.File) {
	for _, f := range files {
		if f != nil {
			f.Close()
		}
	}
}