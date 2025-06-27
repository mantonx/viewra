package core

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/hashicorp/go-hclog"
)

// FFmpegProcess wraps an FFmpeg process with proper lifecycle management
type FFmpegProcess struct {
	cmd       *exec.Cmd
	sessionID string
	logger    hclog.Logger
	startTime time.Time
	mu        sync.Mutex
	done      chan struct{}
	ctx       context.Context
	cancel    context.CancelFunc
	provider  string
	registry  *ProcessRegistry
}

// NewFFmpegProcess creates a new managed FFmpeg process
func NewFFmpegProcess(ctx context.Context, sessionID string, args []string, provider string, logger hclog.Logger) *FFmpegProcess {
	processCtx, cancel := context.WithCancel(ctx)

	cmd := exec.CommandContext(processCtx, "ffmpeg", args...)

	// CRITICAL: Set process group to ensure all children are killed
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:   true,            // Create new process group
		Pdeathsig: syscall.SIGKILL, // Kill if parent dies
	}

	// Set up pipes for output
	cmd.Stdout = nil // We'll handle this separately if needed
	cmd.Stderr = nil

	return &FFmpegProcess{
		cmd:       cmd,
		sessionID: sessionID,
		logger:    logger,
		done:      make(chan struct{}),
		ctx:       processCtx,
		cancel:    cancel,
		provider:  provider,
		registry:  GetProcessRegistry(logger),
	}
}

// Start starts the FFmpeg process with proper monitoring
func (fp *FFmpegProcess) Start() error {
	fp.mu.Lock()
	defer fp.mu.Unlock()

	fp.startTime = time.Now()

	// Start the process
	if err := fp.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	pid := fp.cmd.Process.Pid
	fp.logger.Info("started FFmpeg process",
		"session_id", fp.sessionID,
		"pid", pid)

	// Register process in global registry
	fp.registry.Register(pid, fp.sessionID, fp.provider)

	// Monitor process in goroutine
	go fp.monitor()

	return nil
}

// monitor watches the process and ensures cleanup
func (fp *FFmpegProcess) monitor() {
	defer close(fp.done)

	// Get PID before waiting
	pid := fp.cmd.Process.Pid

	// Wait for process to exit
	err := fp.cmd.Wait()

	fp.mu.Lock()
	defer fp.mu.Unlock()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			fp.logger.Warn("FFmpeg process exited with error",
				"session_id", fp.sessionID,
				"exit_code", exitErr.ExitCode(),
				"duration", time.Since(fp.startTime))
		} else {
			fp.logger.Error("FFmpeg process error",
				"session_id", fp.sessionID,
				"error", err,
				"duration", time.Since(fp.startTime))
		}
	} else {
		fp.logger.Info("FFmpeg process completed successfully",
			"session_id", fp.sessionID,
			"duration", time.Since(fp.startTime))
	}

	// Ensure cleanup
	fp.cleanup()

	// Unregister from global registry
	fp.registry.Unregister(pid)
}

// Stop stops the FFmpeg process gracefully
func (fp *FFmpegProcess) Stop() error {
	fp.mu.Lock()
	defer fp.mu.Unlock()

	if fp.cmd.Process == nil {
		return nil
	}

	pid := fp.cmd.Process.Pid
	fp.logger.Info("stopping FFmpeg process",
		"session_id", fp.sessionID,
		"pid", pid)

	// Cancel context first
	fp.cancel()

	// Use the centralized kill function with proper timeout
	err := KillProcessGroup(pid, fp.logger)

	// Unregister from global registry
	fp.registry.Unregister(pid)

	// Wait for monitor goroutine to finish
	select {
	case <-fp.done:
		// Process monitor has finished
	case <-time.After(10 * time.Second):
		fp.logger.Warn("monitor goroutine did not finish in time",
			"session_id", fp.sessionID)
	}

	return err
}

// cleanup ensures the process and all children are terminated
func (fp *FFmpegProcess) cleanup() {
	if fp.cmd.Process == nil {
		return
	}

	pid := fp.cmd.Process.Pid

	// Use the centralized kill function
	if err := KillProcessGroup(pid, fp.logger); err != nil {
		fp.logger.Error("failed to kill process group",
			"pid", pid,
			"session_id", fp.sessionID,
			"error", err)
	}
}

// IsRunning checks if the process is still running
func (fp *FFmpegProcess) IsRunning() bool {
	fp.mu.Lock()
	defer fp.mu.Unlock()

	if fp.cmd.Process == nil {
		return false
	}

	// Check if process exists
	err := fp.cmd.Process.Signal(syscall.Signal(0))
	return err == nil
}

// GetPID returns the process ID
func (fp *FFmpegProcess) GetPID() int {
	fp.mu.Lock()
	defer fp.mu.Unlock()

	if fp.cmd.Process == nil {
		return -1
	}

	return fp.cmd.Process.Pid
}

// FFmpegProcessManager manages multiple FFmpeg processes
type FFmpegProcessManager struct {
	processes map[string]*FFmpegProcess
	mu        sync.RWMutex
	logger    hclog.Logger
}

// NewFFmpegProcessManager creates a new process manager
func NewFFmpegProcessManager(logger hclog.Logger) *FFmpegProcessManager {
	return &FFmpegProcessManager{
		processes: make(map[string]*FFmpegProcess),
		logger:    logger,
	}
}

// StartProcess starts a new FFmpeg process
func (m *FFmpegProcessManager) StartProcess(ctx context.Context, sessionID string, args []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if process already exists
	if existing, ok := m.processes[sessionID]; ok {
		if existing.IsRunning() {
			return fmt.Errorf("process already running for session %s", sessionID)
		}
		// Clean up dead process
		delete(m.processes, sessionID)
	}

	// Create new process
	process := NewFFmpegProcess(ctx, sessionID, args, "ffmpeg_wrapper", m.logger)

	// Start it
	if err := process.Start(); err != nil {
		return err
	}

	// Track it
	m.processes[sessionID] = process

	return nil
}

// StopProcess stops a process by session ID
func (m *FFmpegProcessManager) StopProcess(sessionID string) error {
	m.mu.Lock()
	process, ok := m.processes[sessionID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("no process found for session %s", sessionID)
	}
	delete(m.processes, sessionID)
	m.mu.Unlock()

	return process.Stop()
}

// StopAllProcesses stops all managed processes
func (m *FFmpegProcessManager) StopAllProcesses() {
	m.mu.Lock()
	processes := make([]*FFmpegProcess, 0, len(m.processes))
	for _, p := range m.processes {
		processes = append(processes, p)
	}
	m.processes = make(map[string]*FFmpegProcess)
	m.mu.Unlock()

	var wg sync.WaitGroup
	for _, p := range processes {
		wg.Add(1)
		go func(process *FFmpegProcess) {
			defer wg.Done()
			process.Stop()
		}(p)
	}

	wg.Wait()
}

// CleanupZombies removes any zombie processes
func (m *FFmpegProcessManager) CleanupZombies() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for sessionID, process := range m.processes {
		if !process.IsRunning() {
			m.logger.Info("removing dead process entry", "session_id", sessionID)
			delete(m.processes, sessionID)
		}
	}
}

// GetRunningCount returns the number of running processes
func (m *FFmpegProcessManager) GetRunningCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, process := range m.processes {
		if process.IsRunning() {
			count++
		}
	}

	return count
}
