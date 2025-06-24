package core

import (
	"fmt"
	"sync"
	"syscall"
	"time"

	"github.com/hashicorp/go-hclog"
)

// ProcessInfo stores information about a running FFmpeg process
type ProcessInfo struct {
	PID       int
	SessionID string
	StartTime time.Time
	Provider  string
}

// ProcessRegistry tracks all active FFmpeg processes globally
type ProcessRegistry struct {
	processes map[int]*ProcessInfo // Map of PID to ProcessInfo
	sessions  map[string]int       // Map of SessionID to PID
	mu        sync.RWMutex
	logger    hclog.Logger
}

// NewProcessRegistry creates a new process registry
func NewProcessRegistry(logger hclog.Logger) *ProcessRegistry {
	return &ProcessRegistry{
		processes: make(map[int]*ProcessInfo),
		sessions:  make(map[string]int),
		logger:    logger.Named("process-registry"),
	}
}

// Register registers a new process
func (pr *ProcessRegistry) Register(pid int, sessionID string, provider string) {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	info := &ProcessInfo{
		PID:       pid,
		SessionID: sessionID,
		StartTime: time.Now(),
		Provider:  provider,
	}

	pr.processes[pid] = info
	pr.sessions[sessionID] = pid

	pr.logger.Info("registered FFmpeg process",
		"pid", pid,
		"session_id", sessionID,
		"provider", provider)
}

// Unregister removes a process from the registry
func (pr *ProcessRegistry) Unregister(pid int) {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	if info, ok := pr.processes[pid]; ok {
		delete(pr.processes, pid)
		delete(pr.sessions, info.SessionID)

		pr.logger.Info("unregistered FFmpeg process",
			"pid", pid,
			"session_id", info.SessionID)
	}
}

// GetProcessBySession returns the PID for a session
func (pr *ProcessRegistry) GetProcessBySession(sessionID string) (int, bool) {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	pid, ok := pr.sessions[sessionID]
	return pid, ok
}

// GetAllProcesses returns all registered processes
func (pr *ProcessRegistry) GetAllProcesses() map[int]*ProcessInfo {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	// Create a copy to avoid race conditions
	copy := make(map[int]*ProcessInfo)
	for pid, info := range pr.processes {
		copy[pid] = info
	}
	return copy
}

// CleanupOrphaned checks for and kills orphaned processes
func (pr *ProcessRegistry) CleanupOrphaned() int {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	killedCount := 0
	toRemove := []int{}

	for pid, info := range pr.processes {
		// Check if process is still alive
		if !isProcessAlive(pid) {
			pr.logger.Debug("process no longer exists, removing from registry",
				"pid", pid,
				"session_id", info.SessionID)
			toRemove = append(toRemove, pid)
			continue
		}

		// Check if process has been running too long (30 minutes)
		// ABR transcoding with multiple quality levels can take a while
		// Only kill if it's been running for an unreasonably long time
		if time.Since(info.StartTime) > 30*time.Minute {
			pr.logger.Warn("killing long-running FFmpeg process",
				"pid", pid,
				"session_id", info.SessionID,
				"runtime", time.Since(info.StartTime))

			if err := KillProcessGroup(pid, pr.logger); err != nil {
				pr.logger.Error("failed to kill long-running process",
					"pid", pid,
					"error", err)
			} else {
				killedCount++
				toRemove = append(toRemove, pid)
			}
		}
	}

	// Remove dead processes from registry
	for _, pid := range toRemove {
		if info, ok := pr.processes[pid]; ok {
			delete(pr.processes, pid)
			delete(pr.sessions, info.SessionID)
		}
	}

	return killedCount
}

// KillProcessGroup kills a process and its entire process group with proper signal escalation
func KillProcessGroup(pid int, logger hclog.Logger) error {
	// First check if process exists
	if !isProcessAlive(pid) {
		return nil // Process already dead
	}

	// Try to get process group ID
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		// Process might have died, check again
		if !isProcessAlive(pid) {
			return nil
		}
		// Can't get pgid, just kill the process directly
		pgid = pid
	}

	// Step 1: Send SIGTERM to process group for graceful shutdown
	if pgid != pid {
		// Kill the entire process group
		if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
			logger.Debug("failed to send SIGTERM to process group",
				"pgid", pgid,
				"error", err)
		} else {
			logger.Debug("sent SIGTERM to process group", "pgid", pgid)
		}
	}

	// Also send SIGTERM to the main process
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		logger.Debug("failed to send SIGTERM to process",
			"pid", pid,
			"error", err)
	}

	// Step 2: Wait for graceful termination (5 seconds)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !isProcessAlive(pid) {
			logger.Debug("process terminated gracefully", "pid", pid)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Step 3: Force kill with SIGKILL
	logger.Warn("process did not terminate gracefully, sending SIGKILL",
		"pid", pid,
		"pgid", pgid)

	// Kill the process group
	if pgid != pid {
		if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
			logger.Debug("failed to send SIGKILL to process group",
				"pgid", pgid,
				"error", err)
		}
	}

	// Kill the main process
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		if err != syscall.ESRCH { // ESRCH means process doesn't exist
			return fmt.Errorf("failed to kill process %d: %w", pid, err)
		}
	}

	// Step 4: Verify process is dead
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !isProcessAlive(pid) {
			logger.Info("process killed successfully", "pid", pid)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("process %d could not be killed", pid)
}

// isProcessAlive checks if a process is still running
func isProcessAlive(pid int) bool {
	// Signal 0 doesn't actually send a signal, just checks if process exists
	err := syscall.Kill(pid, 0)
	return err == nil
}

// Global process registry instance
var globalProcessRegistry *ProcessRegistry
var registryOnce sync.Once

// GetProcessRegistry returns the global process registry
func GetProcessRegistry(logger hclog.Logger) *ProcessRegistry {
	registryOnce.Do(func() {
		globalProcessRegistry = NewProcessRegistry(logger)
	})
	return globalProcessRegistry
}