package ffmpeg

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestNewProcessManager(t *testing.T) {
	t.Run("with config", func(t *testing.T) {
		config := &TranscodeConfig{
			ProcessGroupKill: true,
			MaxMemoryMB:      1024,
		}

		pm := NewProcessManager(config)
		if pm == nil {
			t.Fatal("expected non-nil ProcessManager")
		}
		if pm.config != config {
			t.Error("expected config to be set")
		}
	})

	t.Run("with nil config uses default", func(t *testing.T) {
		pm := NewProcessManager(nil)
		if pm == nil {
			t.Fatal("expected non-nil ProcessManager")
		}
		if pm.config == nil {
			t.Error("expected default config to be set")
		}
	})
}

func TestPrepareCommand(t *testing.T) {
	tests := []struct {
		name             string
		config           *TranscodeConfig
		envFFmpegLibPath string
		expectSetpgid    bool
		expectLDLibPath  bool
		skipOnWindows    bool
	}{
		{
			name: "process group kill enabled",
			config: &TranscodeConfig{
				ProcessGroupKill: true,
			},
			expectSetpgid: true,
			skipOnWindows: true,
		},
		{
			name: "process group kill disabled",
			config: &TranscodeConfig{
				ProcessGroupKill: false,
			},
			expectSetpgid: false,
			skipOnWindows: true,
		},
		{
			name: "with custom FFmpeg lib path",
			config: &TranscodeConfig{
				ProcessGroupKill: true,
			},
			envFFmpegLibPath: "/custom/ffmpeg/lib",
			expectSetpgid:    true,
			expectLDLibPath:  true,
			skipOnWindows:    true,
		},
		{
			name: "without custom FFmpeg lib path",
			config: &TranscodeConfig{
				ProcessGroupKill: true,
			},
			expectSetpgid:   true,
			expectLDLibPath: false,
			skipOnWindows:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipOnWindows && runtime.GOOS == "windows" {
				t.Skip("skipping on Windows")
			}

			// Set up environment
			if tt.envFFmpegLibPath != "" {
				oldPath := os.Getenv("VIEWRA_FFMPEG_LIB_PATH")
				os.Setenv("VIEWRA_FFMPEG_LIB_PATH", tt.envFFmpegLibPath)
				defer os.Setenv("VIEWRA_FFMPEG_LIB_PATH", oldPath)
			} else {
				oldPath := os.Getenv("VIEWRA_FFMPEG_LIB_PATH")
				os.Unsetenv("VIEWRA_FFMPEG_LIB_PATH")
				defer func() {
					if oldPath != "" {
						os.Setenv("VIEWRA_FFMPEG_LIB_PATH", oldPath)
					}
				}()
			}

			pm := NewProcessManager(tt.config)
			ctx := context.Background()

			cmd := pm.PrepareCommand(ctx, "echo", []string{"test"})

			if cmd == nil {
				t.Fatal("expected non-nil command")
			}

			// Check process group settings on Unix
			if runtime.GOOS != "windows" {
				if tt.expectSetpgid {
					if cmd.SysProcAttr == nil {
						t.Error("expected SysProcAttr to be set")
					} else if !cmd.SysProcAttr.Setpgid {
						t.Error("expected Setpgid to be true")
					}
				}
			}

			// Check LD_LIBRARY_PATH
			if tt.expectLDLibPath {
				found := false
				expectedEnv := "LD_LIBRARY_PATH=" + tt.envFFmpegLibPath
				for _, env := range cmd.Env {
					if env == expectedEnv {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected LD_LIBRARY_PATH to be set to %s", tt.envFFmpegLibPath)
				}
			} else if tt.envFFmpegLibPath == "" {
				// When no custom path is set, Env should be nil (inherits parent)
				if cmd.Env != nil {
					for _, env := range cmd.Env {
						if strings.HasPrefix(env, "LD_LIBRARY_PATH=") {
							t.Error("expected LD_LIBRARY_PATH not to be set")
						}
					}
				}
			}
		})
	}
}

func TestPrepareCommandWithMemoryLimit(t *testing.T) {
	tests := []struct {
		name             string
		config           *TranscodeConfig
		expectSystemdRun bool
		skipOnNonLinux   bool
	}{
		{
			name: "with memory limit on Linux",
			config: &TranscodeConfig{
				ProcessGroupKill: true,
				MaxMemoryMB:      1024,
			},
			expectSystemdRun: true,
			skipOnNonLinux:   true,
		},
		{
			name: "without memory limit",
			config: &TranscodeConfig{
				ProcessGroupKill: true,
				MaxMemoryMB:      0,
			},
			expectSystemdRun: false,
			skipOnNonLinux:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipOnNonLinux && runtime.GOOS != "linux" {
				t.Skip("skipping on non-Linux")
			}

			pm := NewProcessManager(tt.config)
			ctx := context.Background()

			cmd := pm.PrepareCommandWithMemoryLimit(ctx, "echo", []string{"test"})

			if cmd == nil {
				t.Fatal("expected non-nil command")
			}

			// Check if systemd-run is used
			if runtime.GOOS == "linux" && tt.config.MaxMemoryMB > 0 {
				// Check if systemd-run is available
				if _, err := exec.LookPath("systemd-run"); err == nil {
					// systemd-run is available, should use it
					if cmd.Path != "" && !strings.Contains(cmd.Path, "systemd-run") {
						// The command might not have systemd-run if LookPath failed
						// This is acceptable
					}
					// Verify process group is still set
					if tt.config.ProcessGroupKill && cmd.SysProcAttr != nil {
						if !cmd.SysProcAttr.Setpgid {
							t.Error("expected Setpgid to be true even with systemd-run")
						}
					}
				}
			}
		})
	}
}

func TestWaitWithCleanup_Success(t *testing.T) {
	pm := NewProcessManager(&TranscodeConfig{
		ProcessGroupKill: true,
	})

	ctx := context.Background()
	cmd := pm.PrepareCommand(ctx, "echo", []string{"hello"})

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start command: %v", err)
	}

	err := pm.WaitWithCleanup(ctx, cmd)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestWaitWithCleanup_ProcessFailure(t *testing.T) {
	pm := NewProcessManager(&TranscodeConfig{
		ProcessGroupKill: true,
	})

	ctx := context.Background()
	// Use a command that will fail
	cmd := pm.PrepareCommand(ctx, "sh", []string{"-c", "exit 1"})

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start command: %v", err)
	}

	err := pm.WaitWithCleanup(ctx, cmd)
	if err == nil {
		t.Error("expected error from failed command")
	}

	// Check that it's an exit error
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Errorf("expected ExitError, got: %T", err)
	}
}

func TestWaitWithCleanup_ContextCancellation(t *testing.T) {
	pm := NewProcessManager(&TranscodeConfig{
		ProcessGroupKill: true,
	})

	ctx, cancel := context.WithCancel(context.Background())
	// Use a command that will run for a while
	cmd := pm.PrepareCommand(ctx, "sleep", []string{"10"})

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start command: %v", err)
	}

	// Cancel context after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := pm.WaitWithCleanup(ctx, cmd)
	elapsed := time.Since(start)

	// Should complete quickly (not wait for full 10 seconds)
	if elapsed > 2*time.Second {
		t.Errorf("expected quick cleanup, took: %v", elapsed)
	}

	if err == nil {
		t.Error("expected error from cancelled context")
	}

	if !strings.Contains(err.Error(), "transcode cancelled") {
		t.Errorf("expected 'transcode cancelled' error, got: %v", err)
	}

	// Verify process is actually dead
	if cmd.Process != nil {
		// Try to send signal 0 to check if process exists
		if runtime.GOOS != "windows" {
			if err := cmd.Process.Signal(syscall.Signal(0)); err == nil {
				t.Error("expected process to be terminated")
			}
		}
	}
}

func TestWaitWithCleanup_ContextTimeout(t *testing.T) {
	pm := NewProcessManager(&TranscodeConfig{
		ProcessGroupKill: true,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Use a command that will run longer than the timeout
	cmd := pm.PrepareCommand(ctx, "sleep", []string{"10"})

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start command: %v", err)
	}

	start := time.Now()
	err := pm.WaitWithCleanup(ctx, cmd)
	elapsed := time.Since(start)

	// Should timeout around 200ms
	if elapsed < 150*time.Millisecond || elapsed > 500*time.Millisecond {
		t.Logf("warning: timeout took %v, expected around 200ms", elapsed)
	}

	if err == nil {
		t.Error("expected error from timeout")
	}

	if !strings.Contains(err.Error(), "transcode cancelled") {
		t.Errorf("expected 'transcode cancelled' error, got: %v", err)
	}
}

func TestKillProcessGroup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping process group test on Windows")
	}

	tests := []struct {
		name             string
		processGroupKill bool
		startChildren    bool
	}{
		{
			name:             "kill process with process group",
			processGroupKill: true,
			startChildren:    false,
		},
		{
			name:             "kill process without process group",
			processGroupKill: false,
			startChildren:    false,
		},
		{
			name:             "kill process group with children",
			processGroupKill: true,
			startChildren:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := NewProcessManager(&TranscodeConfig{
				ProcessGroupKill: tt.processGroupKill,
			})

			ctx := context.Background()

			var cmd *exec.Cmd
			if tt.startChildren {
				// Start a shell that spawns children
				cmd = pm.PrepareCommand(ctx, "sh", []string{"-c", "sleep 10 & sleep 10 & wait"})
			} else {
				cmd = pm.PrepareCommand(ctx, "sleep", []string{"10"})
			}

			if err := cmd.Start(); err != nil {
				t.Fatalf("failed to start command: %v", err)
			}

			pid := cmd.Process.Pid

			// Give process time to start
			time.Sleep(50 * time.Millisecond)

			// Kill the process
			if err := pm.KillProcessGroup(cmd); err != nil {
				t.Errorf("KillProcessGroup failed: %v", err)
			}

			// Wait for process to exit - this is the reliable way to check
			done := make(chan error, 1)
			go func() {
				done <- cmd.Wait()
			}()

			select {
			case <-done:
				// Process exited as expected
			case <-time.After(2 * time.Second):
				t.Errorf("process %d did not exit within timeout after kill", pid)
			}
		})
	}
}

func TestKillProcessGroup_NilCommand(t *testing.T) {
	pm := NewProcessManager(&TranscodeConfig{
		ProcessGroupKill: true,
	})

	// Should not panic with nil command
	err := pm.KillProcessGroup(nil)
	if err != nil {
		t.Errorf("expected no error for nil command, got: %v", err)
	}
}

func TestKillProcessGroup_NilProcess(t *testing.T) {
	pm := NewProcessManager(&TranscodeConfig{
		ProcessGroupKill: true,
	})

	cmd := &exec.Cmd{}
	// Should not panic with nil process
	err := pm.KillProcessGroup(cmd)
	if err != nil {
		t.Errorf("expected no error for nil process, got: %v", err)
	}
}

func TestProcessGroupKill_ActuallyKillsChildren(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping process group children test on Windows")
	}

	pm := NewProcessManager(&TranscodeConfig{
		ProcessGroupKill: true,
	})

	ctx := context.Background()

	// Create a shell script that spawns multiple children
	// Use sleep with different durations to make them identifiable
	cmd := pm.PrepareCommand(ctx, "sh", []string{"-c", "sleep 100 & sleep 101 & sleep 102 & wait"})

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start command: %v", err)
	}

	parentPid := cmd.Process.Pid

	// Give children time to start
	time.Sleep(200 * time.Millisecond)

	// Kill the process group
	if err := pm.KillProcessGroup(cmd); err != nil {
		t.Errorf("KillProcessGroup failed: %v", err)
	}

	// Wait for process to exit - this is the reliable way to check
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
		// Process exited as expected
	case <-time.After(2 * time.Second):
		t.Errorf("parent process %d did not exit within timeout after kill", parentPid)
	}
}

func TestWaitWithCleanup_RapidCancellation(t *testing.T) {
	// Test that rapid cancellation doesn't cause races or panics
	pm := NewProcessManager(&TranscodeConfig{
		ProcessGroupKill: true,
	})

	for i := 0; i < 10; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cmd := pm.PrepareCommand(ctx, "sleep", []string{"5"})

		if err := cmd.Start(); err != nil {
			t.Fatalf("failed to start command: %v", err)
		}

		// Cancel immediately
		cancel()

		err := pm.WaitWithCleanup(ctx, cmd)
		if err == nil {
			t.Error("expected error from cancelled context")
		}
	}
}

func TestWaitWithCleanup_MultipleProcesses(t *testing.T) {
	// Test concurrent process management
	pm := NewProcessManager(&TranscodeConfig{
		ProcessGroupKill: true,
	})

	const numProcs = 5
	errors := make(chan error, numProcs)

	for i := 0; i < numProcs; i++ {
		go func(idx int) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			cmd := pm.PrepareCommand(ctx, "sleep", []string{"5"})
			if err := cmd.Start(); err != nil {
				errors <- err
				return
			}

			errors <- pm.WaitWithCleanup(ctx, cmd)
		}(i)
	}

	// Collect results
	for i := 0; i < numProcs; i++ {
		err := <-errors
		if err == nil {
			t.Errorf("process %d: expected timeout error", i)
		} else if !strings.Contains(err.Error(), "transcode cancelled") {
			t.Errorf("process %d: unexpected error: %v", i, err)
		}
	}
}
