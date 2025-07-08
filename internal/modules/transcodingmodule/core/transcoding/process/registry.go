// Package process provides process management for transcoding operations.
// This provides a thread-safe registry with proper lifecycle handling.
package process

import (
	"context"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/hashicorp/go-hclog"
)

// ProcessInfo holds information about a managed process
type ProcessInfo struct {
	PID       int
	SessionID string
	Provider  string
	StartTime time.Time
	Cmd       interface{} // *exec.Cmd
}

// ProcessRegistry provides thread-safe process management with proper lifecycle handling
// and prevents race conditions.
type ProcessRegistry struct {
	// Process tracking
	processes map[int]*ProcessInfo    // PID -> ProcessInfo
	sessions  map[string][]int        // SessionID -> []PID (sessions can have multiple processes)
	providers map[string]map[int]bool // Provider -> PID set

	// Synchronization
	mu sync.RWMutex

	// Process lifecycle management
	cleanupInterval time.Duration
	maxProcessAge   time.Duration

	// Shutdown coordination
	shutdownCh chan struct{}
	shutdownWg sync.WaitGroup

	logger hclog.Logger
}

// ExtendedProcessInfo extends ProcessInfo with additional metadata
type ExtendedProcessInfo struct {
	*ProcessInfo

	// Process lifecycle
	RegisterTime time.Time
	LastActivity time.Time

	// Process group information
	PGID int

	// Resource monitoring
	MemoryUsage int64 // KB
	CPUPercent  float64

	// State
	IsOrphaned bool
	KillSignal int // Last signal sent
}

// ProcessLifecycleState represents the state of a process
type ProcessLifecycleState string

const (
	ProcessStateActive      ProcessLifecycleState = "active"
	ProcessStateStale       ProcessLifecycleState = "stale"
	ProcessStateTerminating ProcessLifecycleState = "terminating"
	ProcessStateOrphaned    ProcessLifecycleState = "orphaned"
)

// RegistryConfig contains configuration for the unified process registry
type RegistryConfig struct {
	CleanupInterval          time.Duration
	MaxProcessAge            time.Duration
	OrphanCheckInterval      time.Duration
	GracefulShutdownTime     time.Duration
	ForcefulShutdownTime     time.Duration
	EnableResourceMonitoring bool
}

// DefaultRegistryConfig returns default configuration
func DefaultRegistryConfig() RegistryConfig {
	return RegistryConfig{
		CleanupInterval:          5 * time.Minute,
		MaxProcessAge:            30 * time.Minute,
		OrphanCheckInterval:      1 * time.Minute,
		GracefulShutdownTime:     5 * time.Second,
		ForcefulShutdownTime:     2 * time.Second,
		EnableResourceMonitoring: false, // Disabled by default for performance
	}
}

// Global registry instance with proper initialization
var (
	globalRegistry   *ProcessRegistry
	registryInitOnce sync.Once
)

// GetRegistry returns the global process registry
func GetRegistry(logger hclog.Logger, config RegistryConfig) *ProcessRegistry {
	registryInitOnce.Do(func() {
		globalRegistry = NewProcessRegistry(logger, config)
	})
	return globalRegistry
}

// NewProcessRegistry creates a new process registry
func NewProcessRegistry(logger hclog.Logger, config RegistryConfig) *ProcessRegistry {
	registry := &ProcessRegistry{
		processes:       make(map[int]*ProcessInfo),
		sessions:        make(map[string][]int),
		providers:       make(map[string]map[int]bool),
		cleanupInterval: config.CleanupInterval,
		maxProcessAge:   config.MaxProcessAge,
		shutdownCh:      make(chan struct{}),
		logger:          logger.Named("unified-process-registry"),
	}

	// Start background cleanup if enabled
	if config.CleanupInterval > 0 {
		registry.shutdownWg.Add(1)
		go registry.runCleanupLoop()
	}

	// Start orphan detection if enabled
	if config.OrphanCheckInterval > 0 {
		registry.shutdownWg.Add(1)
		go registry.runOrphanDetection(config.OrphanCheckInterval)
	}

	return registry
}

// Register adds a process to the registry with enhanced tracking
func (pr *ProcessRegistry) Register(pid int, sessionID, provider string, cmd interface{}) error {
	if pid <= 0 {
		return fmt.Errorf("invalid PID: %d", pid)
	}

	if sessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	pr.mu.Lock()
	defer pr.mu.Unlock()

	// Check if process is already registered
	if existing, exists := pr.processes[pid]; exists {
		pr.logger.Warn("process already registered",
			"pid", pid,
			"existing_session", existing.SessionID,
			"new_session", sessionID)
		return fmt.Errorf("process %d already registered for session %s", pid, existing.SessionID)
	}

	// Get process group ID
	pgid, _ := syscall.Getpgid(pid)

	info := &ProcessInfo{
		PID:       pid,
		SessionID: sessionID,
		Provider:  provider,
		StartTime: time.Now(),
		Cmd:       cmd,
	}

	pr.processes[pid] = info

	// Add to session mapping
	pr.sessions[sessionID] = append(pr.sessions[sessionID], pid)

	// Add to provider mapping
	if pr.providers[provider] == nil {
		pr.providers[provider] = make(map[int]bool)
	}
	pr.providers[provider][pid] = true

	pr.logger.Info("registered process",
		"pid", pid,
		"session_id", sessionID,
		"provider", provider,
		"pgid", pgid)

	return nil
}

// Unregister removes a process from the registry
func (pr *ProcessRegistry) Unregister(pid int) error {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	info, exists := pr.processes[pid]
	if !exists {
		return fmt.Errorf("process %d not found in registry", pid)
	}

	// Remove from processes map
	delete(pr.processes, pid)

	// Remove from session mapping
	if pids, ok := pr.sessions[info.SessionID]; ok {
		newPids := make([]int, 0, len(pids)-1)
		for _, p := range pids {
			if p != pid {
				newPids = append(newPids, p)
			}
		}
		if len(newPids) == 0 {
			delete(pr.sessions, info.SessionID)
		} else {
			pr.sessions[info.SessionID] = newPids
		}
	}

	// Remove from provider mapping
	if providerPids, ok := pr.providers[info.Provider]; ok {
		delete(providerPids, pid)
		if len(providerPids) == 0 {
			delete(pr.providers, info.Provider)
		}
	}

	pr.logger.Info("unregistered process",
		"pid", pid,
		"session_id", info.SessionID,
		"provider", info.Provider)

	return nil
}

// GetProcess returns information about a specific process
func (pr *ProcessRegistry) GetProcess(pid int) (*ProcessInfo, bool) {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	info, exists := pr.processes[pid]
	return info, exists
}

// GetProcessesBySession returns all processes for a session
func (pr *ProcessRegistry) GetProcessesBySession(sessionID string) []*ProcessInfo {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	pids, ok := pr.sessions[sessionID]
	if !ok {
		return nil
	}

	processes := make([]*ProcessInfo, 0, len(pids))
	for _, pid := range pids {
		if info, exists := pr.processes[pid]; exists {
			processes = append(processes, info)
		}
	}

	return processes
}

// GetProcessesByProvider returns all processes for a provider
func (pr *ProcessRegistry) GetProcessesByProvider(provider string) []*ProcessInfo {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	providerPids, ok := pr.providers[provider]
	if !ok {
		return nil
	}

	processes := make([]*ProcessInfo, 0, len(providerPids))
	for pid := range providerPids {
		if info, exists := pr.processes[pid]; exists {
			processes = append(processes, info)
		}
	}

	return processes
}

// GetAllProcesses returns all registered processes
func (pr *ProcessRegistry) GetAllProcesses() map[int]*ProcessInfo {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	// Create a copy to avoid holding the lock
	copy := make(map[int]*ProcessInfo)
	for pid, info := range pr.processes {
		copy[pid] = info
	}
	return copy
}

// StopSession gracefully stops all processes for a session
func (pr *ProcessRegistry) StopSession(sessionID string) error {
	processes := pr.GetProcessesBySession(sessionID)
	if len(processes) == 0 {
		return fmt.Errorf("no processes found for session %s", sessionID)
	}

	var lastErr error
	for _, process := range processes {
		if err := pr.StopProcess(process.PID); err != nil {
			lastErr = err
			pr.logger.Error("failed to stop process in session",
				"pid", process.PID,
				"session_id", sessionID,
				"error", err)
		}
	}

	return lastErr
}

// StopProcess gracefully stops a specific process
func (pr *ProcessRegistry) StopProcess(pid int) error {
	// Use the existing KillProcessGroup function which handles graceful termination
	if err := KillProcessGroup(pid); err != nil {
		return fmt.Errorf("failed to stop process %d: %w", pid, err)
	}

	// Unregister the process after successful termination
	pr.Unregister(pid)
	return nil
}

// CleanupOrphaned finds and kills orphaned or long-running processes
func (pr *ProcessRegistry) CleanupOrphaned() int {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	killedCount := 0
	toRemove := []int{}
	now := time.Now()

	for pid, info := range pr.processes {
		// Check if process is still alive
		if !isProcessAlive(pid) {
			pr.logger.Debug("process no longer exists, removing from registry",
				"pid", pid,
				"session_id", info.SessionID)
			toRemove = append(toRemove, pid)
			continue
		}

		// Check if process has been running too long
		if now.Sub(info.StartTime) > pr.maxProcessAge {
			pr.logger.Warn("killing long-running process",
				"pid", pid,
				"session_id", info.SessionID,
				"provider", info.Provider,
				"runtime", now.Sub(info.StartTime))

			// Release lock temporarily for the kill operation
			pr.mu.Unlock()
			if err := KillProcessGroup(pid); err != nil {
				pr.logger.Error("failed to kill long-running process",
					"pid", pid,
					"error", err)
			} else {
				killedCount++
				toRemove = append(toRemove, pid)
			}
			pr.mu.Lock()
		}
	}

	// Remove dead/killed processes from all mappings
	for _, pid := range toRemove {
		if info, ok := pr.processes[pid]; ok {
			delete(pr.processes, pid)

			// Remove from session mapping
			if pids, sessionOk := pr.sessions[info.SessionID]; sessionOk {
				newPids := make([]int, 0, len(pids)-1)
				for _, p := range pids {
					if p != pid {
						newPids = append(newPids, p)
					}
				}
				if len(newPids) == 0 {
					delete(pr.sessions, info.SessionID)
				} else {
					pr.sessions[info.SessionID] = newPids
				}
			}

			// Remove from provider mapping
			if providerPids, providerOk := pr.providers[info.Provider]; providerOk {
				delete(providerPids, pid)
				if len(providerPids) == 0 {
					delete(pr.providers, info.Provider)
				}
			}
		}
	}

	if killedCount > 0 {
		pr.logger.Info("cleanup completed", "killed_processes", killedCount, "removed_dead", len(toRemove)-killedCount)
	}

	return killedCount
}

// GetRegistryStats returns comprehensive statistics about the registry
func (pr *ProcessRegistry) GetRegistryStats() map[string]interface{} {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	stats := map[string]interface{}{
		"total_processes":  len(pr.processes),
		"active_sessions":  len(pr.sessions),
		"active_providers": len(pr.providers),
		"by_provider":      make(map[string]int),
		"oldest_process":   time.Time{},
		"newest_process":   time.Time{},
	}

	// Calculate provider statistics and process ages
	oldest := time.Now()
	newest := time.Time{}

	for _, info := range pr.processes {
		// Count by provider
		providerStats := stats["by_provider"].(map[string]int)
		providerStats[info.Provider]++

		// Track oldest and newest
		if info.StartTime.Before(oldest) {
			oldest = info.StartTime
		}
		if info.StartTime.After(newest) {
			newest = info.StartTime
		}
	}

	if len(pr.processes) > 0 {
		stats["oldest_process"] = oldest
		stats["newest_process"] = newest
	}

	return stats
}

// runCleanupLoop runs periodic cleanup in the background
func (pr *ProcessRegistry) runCleanupLoop() {
	defer pr.shutdownWg.Done()

	ticker := time.NewTicker(pr.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pr.CleanupOrphaned()
		case <-pr.shutdownCh:
			return
		}
	}
}

// runOrphanDetection runs periodic orphan detection
func (pr *ProcessRegistry) runOrphanDetection(interval time.Duration) {
	defer pr.shutdownWg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pr.detectOrphans()
		case <-pr.shutdownCh:
			return
		}
	}
}

// detectOrphans detects processes that may have become orphaned
func (pr *ProcessRegistry) detectOrphans() {
	pr.mu.RLock()
	processes := make([]*ProcessInfo, 0, len(pr.processes))
	for _, info := range pr.processes {
		processes = append(processes, info)
	}
	pr.mu.RUnlock()

	for _, info := range processes {
		if !isProcessAlive(info.PID) {
			pr.logger.Debug("detected dead process, will remove in next cleanup",
				"pid", info.PID,
				"session_id", info.SessionID)
		}
	}
}

// Shutdown gracefully shuts down the process registry
func (pr *ProcessRegistry) Shutdown(ctx context.Context) error {
	pr.logger.Info("shutting down process registry")

	// Signal background goroutines to stop
	close(pr.shutdownCh)

	// Get all processes before shutdown
	allProcesses := pr.GetAllProcesses()

	// Stop all processes
	for pid, info := range allProcesses {
		pr.logger.Debug("stopping process during shutdown",
			"pid", pid,
			"session_id", info.SessionID)

		if err := pr.StopProcess(pid); err != nil {
			pr.logger.Error("failed to stop process during shutdown",
				"pid", pid,
				"error", err)
		}
	}

	// Wait for background goroutines to finish
	done := make(chan struct{})
	go func() {
		pr.shutdownWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		pr.logger.Info("process registry shutdown completed")
	case <-ctx.Done():
		pr.logger.Warn("process registry shutdown timed out")
		return ctx.Err()
	}

	return nil
}

// Helper functions

// isProcessAlive checks if a process with the given PID is still running
func isProcessAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix, sending signal 0 checks if process exists without actually sending a signal
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// KillProcessGroup terminates a process and all its children
func KillProcessGroup(pid int) error {
	// Try to kill the process group
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		// If that fails, just kill the process itself
		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
			return fmt.Errorf("failed to kill process %d: %w", pid, err)
		}
	}

	// Give it a moment to terminate gracefully
	time.Sleep(100 * time.Millisecond)

	// Force kill if still alive
	if isProcessAlive(pid) {
		syscall.Kill(-pid, syscall.SIGKILL)
		syscall.Kill(pid, syscall.SIGKILL)
	}

	return nil
}
