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

// PrepareCommand prepares an exec.Cmd with proper process group settings.
// On Unix systems, this creates a new process group to ensure all children can be killed together.
func (pm *ProcessManager) PrepareCommand(ctx context.Context, name string, args []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)

	// On Unix systems, create a new process group
	// This allows us to kill the entire process tree
	if runtime.GOOS != "windows" && pm.config.ProcessGroupKill {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true, // Create new process group
		}
	}

	return cmd
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
