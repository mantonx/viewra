// Package process provides utilities for managing transcoding processes.
// It handles process logging, monitoring, and lifecycle management.
package process

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// SetupProcessLogs configures logging for a transcoding process.
// It creates separate log files for stdout and stderr in the session directory.
func SetupProcessLogs(cmd *exec.Cmd, sessionPath, sessionID string) error {
	// Create logs directory within session directory
	logsDir := filepath.Join(sessionPath, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Create stdout log file
	stdoutPath := filepath.Join(logsDir, fmt.Sprintf("%s-stdout.log", sessionID))
	stdoutFile, err := os.Create(stdoutPath)
	if err != nil {
		return fmt.Errorf("failed to create stdout log: %w", err)
	}

	// Create stderr log file
	stderrPath := filepath.Join(logsDir, fmt.Sprintf("%s-stderr.log", sessionID))
	stderrFile, err := os.Create(stderrPath)
	if err != nil {
		stdoutFile.Close()
		return fmt.Errorf("failed to create stderr log: %w", err)
	}

	// Assign files to command
	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile

	return nil
}

// LogCommandDetails writes command details to a debug log file.
// This is useful for troubleshooting transcoding issues.
func LogCommandDetails(sessionPath string, command string, args []string) error {
	debugPath := filepath.Join(sessionPath, "logs", "command.log")

	// Ensure logs directory exists
	logsDir := filepath.Dir(debugPath)
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Write command details
	content := fmt.Sprintf("Command: %s\n", command)
	content += fmt.Sprintf("Arguments:\n")
	for i, arg := range args {
		content += fmt.Sprintf("  [%d]: %s\n", i, arg)
	}

	return os.WriteFile(debugPath, []byte(content), 0644)
}

// CleanupProcessLogs closes any open log files associated with a command.
// This should be called after the process completes.
func CleanupProcessLogs(cmd *exec.Cmd) {
	if file, ok := cmd.Stdout.(*os.File); ok && file != nil {
		file.Close()
	}
	if file, ok := cmd.Stderr.(*os.File); ok && file != nil {
		file.Close()
	}
}
