// Package process provides centralized process tracking and management.
// The registry maintains a global view of all FFmpeg processes spawned by the
// transcoding SDK, enabling proper cleanup even in cases of crashes or restarts.
// This is critical for preventing resource leaks and orphaned processes that
// could consume system resources indefinitely.
//
// The registry addresses several operational challenges:
// - Container restarts that may leave FFmpeg processes running
// - Multiple transcoding providers spawning processes independently
// - Graceful shutdown requiring all processes to be terminated
// - Debugging and monitoring of active transcoding operations
//
// Key features:
// - Global process tracking across all transcoding providers
// - Thread-safe operations for concurrent access
// - Graceful termination with signal escalation (SIGTERM -> SIGKILL)
// - Process group management to handle child processes
// - Session-to-process mapping for easy lookup
//
// The registry uses a singleton pattern to ensure all providers share
// the same tracking information, preventing duplicate registrations and
// ensuring comprehensive cleanup during shutdown.
package process

import (
	"fmt"
	"sync"
	"syscall"
	"time"

	"github.com/mantonx/viewra/sdk/transcoding/types"
)

// Registry type alias for backward compatibility
type Registry = ProcessRegistry

// ProcessRegistry provides process tracking for the SDK transcoder
type ProcessRegistry struct {
	processes map[int]*ProcessEntry
	sessions  map[string]int
	mu        sync.RWMutex
	logger    types.Logger
}

// ProcessEntry tracks a running process
type ProcessEntry struct {
	PID       int
	SessionID string
	StartTime time.Time
	Provider  string
}

// Global SDK process registry
var sdkProcessRegistry *ProcessRegistry
var sdkRegistryOnce sync.Once

// GetTranscoderProcessRegistry returns the SDK process registry
func GetTranscoderProcessRegistry(logger types.Logger) *ProcessRegistry {
	sdkRegistryOnce.Do(func() {
		sdkProcessRegistry = &ProcessRegistry{
			processes: make(map[int]*ProcessEntry),
			sessions:  make(map[string]int),
			logger:    logger,
		}
	})
	return sdkProcessRegistry
}

// Register registers a new process
func (pr *ProcessRegistry) Register(pid int, sessionID string, provider string) {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	info := &ProcessEntry{
		PID:       pid,
		SessionID: sessionID,
		StartTime: time.Now(),
		Provider:  provider,
	}

	pr.processes[pid] = info
	pr.sessions[sessionID] = pid

	if pr.logger != nil {
		pr.logger.Info("registered FFmpeg process",
			"pid", pid,
			"session_id", sessionID,
			"provider", provider)
	}
}

// Unregister removes a process from the registry
func (pr *ProcessRegistry) Unregister(pid int) {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	if info, ok := pr.processes[pid]; ok {
		delete(pr.processes, pid)
		delete(pr.sessions, info.SessionID)

		if pr.logger != nil {
			pr.logger.Info("unregistered FFmpeg process",
				"pid", pid,
				"session_id", info.SessionID)
		}
	}
}

// KillProcess kills a process with proper signal escalation
func (pr *ProcessRegistry) KillProcess(pid int) error {
	// First check if process exists
	if err := syscall.Kill(pid, 0); err != nil {
		// Process doesn't exist
		return nil
	}

	// Try to get process group ID
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		// Can't get pgid, just use pid
		pgid = pid
	}

	// Step 1: Send SIGTERM to process group
	if pgid != pid {
		syscall.Kill(-pgid, syscall.SIGTERM)
	}
	syscall.Kill(pid, syscall.SIGTERM)

	// Step 2: Wait for graceful termination (5 seconds)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); err != nil {
			// Process is gone
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Step 3: Force kill with SIGKILL
	if pr.logger != nil {
		pr.logger.Warn("process did not terminate gracefully, sending SIGKILL", "pid", pid)
	}

	if pgid != pid {
		syscall.Kill(-pgid, syscall.SIGKILL)
	}
	syscall.Kill(pid, syscall.SIGKILL)

	// Step 4: Verify process is dead
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); err != nil {
			// Process is gone
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("process %d could not be killed", pid)
}

// GetEntry returns information about a specific process
func (pr *ProcessRegistry) GetEntry(pid int) *ProcessEntry {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return pr.processes[pid]
}

// GetAllEntries returns all registered processes
func (pr *ProcessRegistry) GetAllEntries() []*ProcessEntry {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	
	entries := make([]*ProcessEntry, 0, len(pr.processes))
	for _, entry := range pr.processes {
		entries = append(entries, entry)
	}
	return entries
}