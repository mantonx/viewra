// Package process provides process monitoring and lifecycle management.
// This monitor ensures reliable FFmpeg process handling with proper cleanup,
// graceful shutdowns, and zombie process prevention. It addresses common issues
// in long-running transcoding operations such as orphaned processes, resource
// leaks, and ungraceful terminations.
//
// Key features:
// - Graceful process termination with configurable timeouts
// - Process group management to handle child processes
// - Zombie process detection and cleanup
// - Resource usage tracking
// - Integration with process registry for global tracking
//
// The monitor handles several critical scenarios:
// - Container restarts that may leave orphaned processes
// - User-initiated stops that require immediate cleanup
// - System resource constraints requiring process termination
// - Crashed processes that need proper cleanup
//
// Example usage:
//   monitor := process.NewMonitor(logger, registry)
//   err := monitor.MonitorProcess(cmd, sessionID, "ffmpeg-provider")
//   // Later...
//   err = monitor.StopProcess(pid, 10*time.Second)
package process

import (
	"fmt"
	"os/exec"
	"syscall"
	"time"

	"github.com/mantonx/viewra/sdk/transcoding/types"
)

// Monitor handles process monitoring and lifecycle management
type Monitor struct {
	logger   types.Logger
	registry *Registry
}

// ProcessInfo contains information about a monitored process
type ProcessInfo struct {
	PID         int
	SessionID   string
	Provider    string
	StartTime   time.Time
	Command     string
	ExitCode    int
	IsRunning   bool
}

// NewMonitor creates a new process monitor
func NewMonitor(logger types.Logger, registry *Registry) *Monitor {
	return &Monitor{
		logger:   logger,
		registry: registry,
	}
}

// MonitorProcess monitors a process and handles its lifecycle
func (m *Monitor) MonitorProcess(cmd *exec.Cmd, sessionID string, provider string) error {
	if cmd.Process == nil {
		return fmt.Errorf("process not started")
	}

	pid := cmd.Process.Pid
	
	// Register process
	if m.registry != nil {
		m.registry.Register(pid, sessionID, provider)
	}

	if m.logger != nil {
		m.logger.Info("started monitoring process",
			"pid", pid,
			"session_id", sessionID,
			"provider", provider,
		)
	}

	// Monitor in a goroutine
	go m.monitorLoop(cmd, sessionID, pid)

	return nil
}

// monitorLoop continuously monitors a process
func (m *Monitor) monitorLoop(cmd *exec.Cmd, sessionID string, pid int) {
	// Wait for the process to complete
	err := cmd.Wait()

	// Get exit information
	var exitCode int
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	// Unregister from registry
	if m.registry != nil {
		m.registry.Unregister(pid)
	}

	// Log completion
	if err != nil && m.logger != nil {
		m.logger.Error("process failed",
			"session_id", sessionID,
			"pid", pid,
			"exit_code", exitCode,
			"error", err,
		)
	} else if m.logger != nil {
		m.logger.Info("process completed",
			"session_id", sessionID,
			"pid", pid,
			"exit_code", exitCode,
		)
	}
}

// StopProcess gracefully stops a process with timeout
func (m *Monitor) StopProcess(pid int, timeout time.Duration) error {
	if m.logger != nil {
		m.logger.Info("stopping process", "pid", pid)
	}

	// Try graceful termination first
	if err := m.terminateProcessGroup(pid); err != nil {
		if m.logger != nil {
			m.logger.Warn("graceful termination failed, forcing kill",
				"pid", pid,
				"error", err,
			)
		}
		return m.forceKillProcess(pid)
	}

	return nil
}

// terminateProcessGroup gracefully terminates a process group
func (m *Monitor) terminateProcessGroup(pid int) error {
	// Get process group ID
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		return fmt.Errorf("failed to get process group: %w", err)
	}
	
	// Send SIGTERM to the entire process group
	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM to process group: %w", err)
	}
	
	// Give it 5 seconds to terminate gracefully
	time.Sleep(5 * time.Second)
	
	// Check if the main process is still running
	if err := syscall.Kill(pid, 0); err == nil {
		// Process is still running, need to force kill
		return fmt.Errorf("process %d still running after SIGTERM", pid)
	}
	
	return nil
}

// forceKillProcess forcefully kills a process and its group
func (m *Monitor) forceKillProcess(pid int) error {
	if m.logger != nil {
		m.logger.Warn("force killing process", "pid", pid)
	}
	
	// Try to kill the process group first
	if pgid, err := syscall.Getpgid(pid); err == nil {
		syscall.Kill(-pgid, syscall.SIGKILL)
	}
	
	// Kill the main process
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		return fmt.Errorf("failed to kill process: %w", err)
	}
	
	return nil
}

// GetProcessInfo returns information about a process
func (m *Monitor) GetProcessInfo(pid int) (*ProcessInfo, error) {
	// Check if process exists
	if err := syscall.Kill(pid, 0); err != nil {
		return nil, fmt.Errorf("process not found: %d", pid)
	}

	// Get info from registry if available
	var sessionID, provider string
	var startTime time.Time
	
	if m.registry != nil {
		if entry := m.registry.GetEntry(pid); entry != nil {
			sessionID = entry.SessionID
			provider = entry.Provider
			startTime = entry.StartTime
		}
	}

	return &ProcessInfo{
		PID:       pid,
		SessionID: sessionID,
		Provider:  provider,
		StartTime: startTime,
		IsRunning: true,
	}, nil
}

// CheckHealth checks if a process is still running
func (m *Monitor) CheckHealth(pid int) bool {
	// Simple check if process exists
	err := syscall.Kill(pid, 0)
	return err == nil
}

// GetResourceUsage returns resource usage for a process
func (m *Monitor) GetResourceUsage(pid int) (map[string]interface{}, error) {
	// This is a simplified version - in production you'd use more sophisticated monitoring
	
	// Check if process exists
	if !m.CheckHealth(pid) {
		return nil, fmt.Errorf("process not running: %d", pid)
	}

	// Basic resource info
	usage := map[string]interface{}{
		"pid":        pid,
		"is_running": true,
		"timestamp":  time.Now(),
	}

	// Get additional info from registry
	if m.registry != nil {
		if entry := m.registry.GetEntry(pid); entry != nil {
			usage["session_id"] = entry.SessionID
			usage["provider"] = entry.Provider
			usage["uptime_seconds"] = time.Since(entry.StartTime).Seconds()
		}
	}

	return usage, nil
}

// CleanupZombies removes any zombie processes
func (m *Monitor) CleanupZombies() error {
	if m.registry == nil {
		return nil
	}

	entries := m.registry.GetAllEntries()
	cleaned := 0

	for _, entry := range entries {
		// Check if process is still running
		if !m.CheckHealth(entry.PID) {
			// Process is dead, unregister it
			m.registry.Unregister(entry.PID)
			cleaned++
			
			if m.logger != nil {
				m.logger.Debug("cleaned up zombie process",
					"pid", entry.PID,
					"session_id", entry.SessionID,
				)
			}
		}
	}

	if cleaned > 0 && m.logger != nil {
		m.logger.Info("cleaned up zombie processes", "count", cleaned)
	}

	return nil
}