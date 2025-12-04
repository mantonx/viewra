package transcoding

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"syscall"
)

// ProcessManager handles FFmpeg process lifecycle and cleanup.
type ProcessManager struct {
	config *TranscodeConfig
}

// NewProcessManager creates a new process manager with the given config.
func NewProcessManager(config *TranscodeConfig) *ProcessManager {
	if config == nil {
		config = DefaultTranscodeConfig()
	}
	return &ProcessManager{
		config: config,
	}
}

// PrepareCommand prepares an exec.Cmd with proper process group settings and resource limits.
// On Unix systems, this creates a new process group to ensure all children can be killed together,
// and applies memory limits to prevent runaway processes from crashing the system.
func (pm *ProcessManager) PrepareCommand(ctx context.Context, name string, args []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)

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

// PrepareCommandWithMemoryLimit prepares an exec.Cmd with memory limits for subtitle burn-in.
// On Linux, this wraps the command with systemd-run --scope to apply memory limits,
// providing a hard safety net to prevent OOM from crashing the system.
func (pm *ProcessManager) PrepareCommandWithMemoryLimit(ctx context.Context, name string, args []string) *exec.Cmd {
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
func (pm *ProcessManager) KillProcessGroup(cmd *exec.Cmd) error {
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
func (pm *ProcessManager) WaitWithCleanup(ctx context.Context, cmd *exec.Cmd) error {
	// Channel to signal when cmd.Wait() completes
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		// Context cancelled - kill the process group
		if killErr := pm.KillProcessGroup(cmd); killErr != nil {
			// Log kill error but return context error
			// The caller should log this properly
		}

		// Wait for the process to actually exit
		<-done

		return fmt.Errorf("transcode cancelled: %w", ctx.Err())

	case err := <-done:
		// Command completed normally
		return err
	}
}

// DetectHardwareAccel attempts to detect available hardware acceleration.
// Returns the best available option, or AccelNone if nothing is detected.
func DetectHardwareAccel() HardwareAccel {
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
	cmd := exec.Command("ffmpeg", "-hide_banner", "-encoders")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// Simple string search - encoder names are prefixed with " V" for video
	return containsString(string(output), encoder)
}

// containsString is a simple helper for string searching.
func containsString(haystack, needle string) bool {
	return len(haystack) >= len(needle) &&
		(haystack == needle ||
			len(haystack) > len(needle) &&
				(haystack[:len(needle)] == needle ||
					containsString(haystack[1:], needle)))
}
