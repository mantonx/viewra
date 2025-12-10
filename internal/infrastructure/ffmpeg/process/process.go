// Package process provides FFmpeg process lifecycle management.
//
// This package handles:
//   - Process creation with proper process group settings
//   - Memory limits using systemd-run on Linux
//   - Clean process group termination
//   - Hardware acceleration detection
package process

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"

	"github.com/mantonx/viewra/internal/infrastructure/ffmpeg/hls"
	"github.com/mantonx/viewra/internal/infrastructure/ffmpeg/paths"
)

// Paths is an alias to the paths.Paths type for convenience.
type Paths = paths.Paths

// Config holds process management configuration.
type Config struct {
	FFmpegPath       string // Path to FFmpeg binary
	FFmpegLibPath    string // LD_LIBRARY_PATH for custom builds
	ProcessGroupKill bool   // Enable process group killing for clean shutdown
	MaxMemoryMB      int    // Maximum memory limit for subtitle burn-in
}

// DefaultConfig returns a default configuration.
func DefaultConfig() *Config {
	return &Config{
		FFmpegPath:       "ffmpeg",
		ProcessGroupKill: true,
		MaxMemoryMB:      2048,
	}
}

// Manager handles FFmpeg process lifecycle and cleanup.
type Manager struct {
	config *Config
}

// NewManager creates a new process manager with the given config.
func NewManager(config *Config) *Manager {
	if config == nil {
		config = DefaultConfig()
	}
	return &Manager{
		config: config,
	}
}

// NewManagerFromPaths creates a new process manager using FFmpeg paths configuration.
// This is the preferred constructor when using the parent ffmpeg.Paths type.
func NewManagerFromPaths(paths *Paths, opts ...ManagerOption) *Manager {
	config := DefaultConfig()
	if paths != nil {
		config.FFmpegPath = paths.FFmpeg
		config.FFmpegLibPath = paths.LibPath
	}
	for _, opt := range opts {
		opt(config)
	}
	return &Manager{config: config}
}

// ManagerOption is a functional option for configuring the Manager.
type ManagerOption func(*Config)

// WithMaxMemory sets the maximum memory limit in MB.
func WithMaxMemory(mb int) ManagerOption {
	return func(c *Config) {
		c.MaxMemoryMB = mb
	}
}

// WithProcessGroupKill enables or disables process group killing.
func WithProcessGroupKill(enabled bool) ManagerOption {
	return func(c *Config) {
		c.ProcessGroupKill = enabled
	}
}

// PrepareCommand prepares an exec.Cmd with proper process group settings and resource limits.
// On Unix systems, this creates a new process group to ensure all children can be killed together,
// and applies memory limits to prevent runaway processes from crashing the system.
// If FFmpegLibPath is set, it adds it to LD_LIBRARY_PATH for the patched FFmpeg.
func (pm *Manager) PrepareCommand(ctx context.Context, name string, args []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)

	// If using custom FFmpeg libraries, set LD_LIBRARY_PATH
	if pm.config.FFmpegLibPath != "" {
		cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+pm.config.FFmpegLibPath)
	}

	// On Unix systems, configure process group and resource limits
	if runtime.GOOS != "windows" {
		sysProcAttr := &syscall.SysProcAttr{}

		// Create new process group for clean shutdown
		if pm.config.ProcessGroupKill {
			sysProcAttr.Setpgid = true
		}

		cmd.SysProcAttr = sysProcAttr
	}

	return cmd
}

// PrepareFFmpegCommand prepares an FFmpeg command using the configured path.
func (pm *Manager) PrepareFFmpegCommand(ctx context.Context, args []string) *exec.Cmd {
	ffmpegPath := pm.config.FFmpegPath
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	return pm.PrepareCommand(ctx, ffmpegPath, args)
}

// PrepareCommandWithMemoryLimit prepares an exec.Cmd with memory limits for subtitle burn-in.
// On Linux, this wraps the command with systemd-run --scope to apply memory limits,
// providing a hard safety net to prevent OOM from crashing the system.
func (pm *Manager) PrepareCommandWithMemoryLimit(ctx context.Context, name string, args []string) *exec.Cmd {
	// On Linux with systemd, use systemd-run to apply memory limits
	// This is more reliable than trying to set rlimits directly
	if runtime.GOOS == "linux" && pm.config.MaxMemoryMB > 0 {
		if _, err := exec.LookPath("systemd-run"); err == nil {
			// Apply 2x multiplier for virtual memory headroom (shared libs, mmap, etc.)
			// The -max_alloc FFmpeg flag handles the tight limit; this is a safety net
			limitMB := pm.config.MaxMemoryMB * 2

			// Build systemd-run command with memory limit
			// --scope: Run as a transient scope unit (not a service)
			// --user: Run in user session (no root required)
			// -p MemoryMax: Hard memory limit
			// -p MemorySwapMax=0: Prevent swapping (fail fast rather than thrash)
			systemdArgs := []string{
				"--scope",
				"--user",
				"-p", fmt.Sprintf("MemoryMax=%dM", limitMB),
				"-p", "MemorySwapMax=0",
				"--", // End of systemd-run options
				name,
			}
			systemdArgs = append(systemdArgs, args...)

			cmd := exec.CommandContext(ctx, "systemd-run", systemdArgs...)

			// Still set process group for clean shutdown
			if pm.config.ProcessGroupKill {
				cmd.SysProcAttr = &syscall.SysProcAttr{
					Setpgid: true,
				}
			}

			return cmd
		}
	}

	// Fallback: no system-level memory limit, rely on FFmpeg's -max_alloc
	return pm.PrepareCommand(ctx, name, args)
}

// KillProcessGroup kills the process and all its children.
// On Unix, this kills the entire process group. On Windows, it kills just the process.
func (pm *Manager) KillProcessGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	if runtime.GOOS == "windows" {
		// On Windows, just kill the process
		// Note: This may leave orphaned child processes
		// For production Windows deployment, consider using Job Objects
		return cmd.Process.Kill()
	}

	// On Unix, kill the entire process group
	// The negative PID signals the process group
	pgid := cmd.Process.Pid
	if cmd.SysProcAttr != nil && cmd.SysProcAttr.Setpgid {
		// Process group ID is the process ID when Setpgid is true
		if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
			// If SIGTERM fails, try SIGKILL
			return syscall.Kill(-pgid, syscall.SIGKILL)
		}
		return nil
	}

	// Fallback: just kill the process
	return cmd.Process.Kill()
}

// WaitWithCleanup waits for the command to finish and ensures cleanup on context cancellation.
// Returns the error from cmd.Wait(), or context error if cancelled.
func (pm *Manager) WaitWithCleanup(ctx context.Context, cmd *exec.Cmd) error {
	// Channel to signal when cmd.Wait() completes
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		// Context cancelled - kill the process group
		_ = pm.KillProcessGroup(cmd) // Ignore kill error, return context error

		// Wait for the process to actually exit
		<-done

		return fmt.Errorf("transcode cancelled: %w", ctx.Err())

	case err := <-done:
		// Command completed normally
		return err
	}
}

// HardwareAccel is an alias for hls.HardwareAccel.
// This provides backwards compatibility for code using process.HardwareAccel.
type HardwareAccel = hls.HardwareAccel

// Hardware acceleration constants - re-exported from hls package for convenience.
const (
	AccelNone         = hls.AccelNone
	AccelVAAPI        = hls.AccelVAAPI
	AccelNVENC        = hls.AccelNVENC
	AccelQSV          = hls.AccelQSV
	AccelVideoToolbox = hls.AccelVideoToolbox
)

// DetectHardwareAccel attempts to detect available hardware acceleration.
// Returns the best available option, or AccelNone if nothing is detected.
func DetectHardwareAccel() hls.HardwareAccel {
	// Try to detect NVIDIA GPU
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
		// NVIDIA detected, check if FFmpeg has nvenc
		if hasFFmpegEncoder("h264_nvenc") {
			return AccelNVENC
		}
	}

	// Try to detect Intel QSV (check for /dev/dri/renderD128)
	if runtime.GOOS == "linux" {
		if hasFFmpegEncoder("h264_qsv") {
			return AccelQSV
		}

		// Try VAAPI (most Linux systems with GPU)
		if hasFFmpegEncoder("h264_vaapi") {
			return AccelVAAPI
		}
	}

	// Check for macOS VideoToolbox
	if runtime.GOOS == "darwin" {
		if hasFFmpegEncoder("h264_videotoolbox") {
			return AccelVideoToolbox
		}
	}

	// No hardware acceleration available
	return AccelNone
}

// hasFFmpegEncoder checks if FFmpeg has a specific encoder available.
func hasFFmpegEncoder(encoder string) bool {
	p, err := paths.New()
	if err != nil {
		return false
	}

	cmd := p.PrepareCommand("ffmpeg", "-hide_banner", "-encoders")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// Simple string search - encoder names are prefixed with " V" for video
	return strings.Contains(string(output), encoder)
}
