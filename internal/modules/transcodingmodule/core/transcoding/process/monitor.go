// Package process provides process monitoring capabilities
package process

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/mantonx/viewra/internal/logger"
)

// Monitor provides process monitoring capabilities
type Monitor struct {
	registry *ProcessRegistry
}

// NewMonitor creates a new process monitor
func NewMonitor(registry *ProcessRegistry) *Monitor {
	return &Monitor{
		registry: registry,
	}
}

// MonitorProcess starts monitoring a process
func (m *Monitor) MonitorProcess(cmd *exec.Cmd, sessionID, provider string) error {
	if cmd == nil || cmd.Process == nil {
		return fmt.Errorf("invalid command or process not started")
	}

	pid := cmd.Process.Pid
	m.registry.Register(pid, sessionID, provider, cmd)

	// Start goroutine to monitor process
	go func() {
		// Wait for process to complete
		err := cmd.Wait()

		// Process completed, unregister
		m.registry.Unregister(pid)

		if err != nil {
			logger.Warn("Process exited with error",
				"pid", pid,
				"sessionId", sessionID,
				"error", err)
		} else {
			logger.Debug("Process completed successfully",
				"pid", pid,
				"sessionId", sessionID)
		}
	}()

	return nil
}

// StopProcess attempts to gracefully stop a process
func (m *Monitor) StopProcess(pid int, timeout time.Duration) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("process not found: %w", err)
	}

	// Send SIGTERM for graceful shutdown
	if err := process.Signal(os.Interrupt); err != nil {
		return fmt.Errorf("failed to send interrupt signal: %w", err)
	}

	// Wait for timeout
	done := make(chan bool, 1)
	go func() {
		// Check if process still exists
		for {
			if err := process.Signal(os.Signal(nil)); err != nil {
				// Process no longer exists
				done <- true
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	select {
	case <-done:
		m.registry.Unregister(pid)
		return nil
	case <-time.After(timeout):
		// Force kill
		if err := process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
		m.registry.Unregister(pid)
		return nil
	}
}

// IsProcessRunning checks if a process is still running
func (m *Monitor) IsProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Try to send signal 0 (no-op) to check if process exists
	err = process.Signal(os.Signal(nil))
	return err == nil
}

// GetProcessInfo returns information about a monitored process
func (m *Monitor) GetProcessInfo(pid int) (*ProcessInfo, error) {
	info, exists := m.registry.GetProcess(pid)
	if !exists {
		return nil, fmt.Errorf("process not monitored: %d", pid)
	}
	return info, nil
}
