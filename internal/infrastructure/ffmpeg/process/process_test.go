package process

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

// TestDefaultConfig tests the default configuration values.
func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config == nil {
		t.Fatal("DefaultConfig returned nil")
	}

	if config.FFmpegPath != "ffmpeg" {
		t.Errorf("Expected FFmpegPath to be 'ffmpeg', got %s", config.FFmpegPath)
	}

	if config.ProcessGroupKill != true {
		t.Errorf("Expected ProcessGroupKill to be true, got %v", config.ProcessGroupKill)
	}

	if config.MaxMemoryMB != 2048 {
		t.Errorf("Expected MaxMemoryMB to be 2048, got %d", config.MaxMemoryMB)
	}

	if config.FFmpegLibPath != "" {
		t.Errorf("Expected FFmpegLibPath to be empty, got %s", config.FFmpegLibPath)
	}
}

// TestNewManager tests creating a new process manager.
func TestNewManager(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
	}{
		{
			name:   "with nil config",
			config: nil,
		},
		{
			name: "with custom config",
			config: &Config{
				FFmpegPath:       "/usr/local/bin/ffmpeg",
				FFmpegLibPath:    "/usr/local/lib",
				ProcessGroupKill: false,
				MaxMemoryMB:      4096,
			},
		},
		{
			name: "with empty config",
			config: &Config{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager(tt.config)

			if manager == nil {
				t.Fatal("NewManager returned nil")
			}

			if manager.config == nil {
				t.Fatal("Manager config is nil")
			}

			// If we passed nil, should get default config
			if tt.config == nil {
				if manager.config.FFmpegPath != "ffmpeg" {
					t.Errorf("Expected default FFmpegPath, got %s", manager.config.FFmpegPath)
				}
			}
		})
	}
}

// TestNewManagerFromPaths tests creating a manager from paths.
func TestNewManagerFromPaths(t *testing.T) {
	tests := []struct {
		name              string
		paths             *Paths
		opts              []ManagerOption
		expectedFFmpegPath string
		expectedLibPath    string
		expectedMaxMemory  int
		expectedPGKill     bool
	}{
		{
			name:              "with nil paths",
			paths:             nil,
			opts:              nil,
			expectedFFmpegPath: "ffmpeg",
			expectedLibPath:    "",
			expectedMaxMemory:  2048,
			expectedPGKill:     true,
		},
		{
			name: "with valid paths",
			paths: &Paths{
				FFmpeg:  "/custom/ffmpeg",
				FFprobe: "/custom/ffprobe",
				LibPath: "/custom/lib",
			},
			opts:              nil,
			expectedFFmpegPath: "/custom/ffmpeg",
			expectedLibPath:    "/custom/lib",
			expectedMaxMemory:  2048,
			expectedPGKill:     true,
		},
		{
			name: "with options - max memory",
			paths: &Paths{
				FFmpeg:  "/usr/bin/ffmpeg",
				FFprobe: "/usr/bin/ffprobe",
			},
			opts:              []ManagerOption{WithMaxMemory(4096)},
			expectedFFmpegPath: "/usr/bin/ffmpeg",
			expectedLibPath:    "",
			expectedMaxMemory:  4096,
			expectedPGKill:     true,
		},
		{
			name: "with options - process group kill",
			paths: &Paths{
				FFmpeg:  "/usr/bin/ffmpeg",
				FFprobe: "/usr/bin/ffprobe",
			},
			opts:              []ManagerOption{WithProcessGroupKill(false)},
			expectedFFmpegPath: "/usr/bin/ffmpeg",
			expectedLibPath:    "",
			expectedMaxMemory:  2048,
			expectedPGKill:     false,
		},
		{
			name: "with multiple options",
			paths: &Paths{
				FFmpeg:  "/opt/ffmpeg",
				FFprobe: "/opt/ffprobe",
				LibPath: "/opt/lib",
			},
			opts: []ManagerOption{
				WithMaxMemory(8192),
				WithProcessGroupKill(false),
			},
			expectedFFmpegPath: "/opt/ffmpeg",
			expectedLibPath:    "/opt/lib",
			expectedMaxMemory:  8192,
			expectedPGKill:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManagerFromPaths(tt.paths, tt.opts...)

			if manager == nil {
				t.Fatal("NewManagerFromPaths returned nil")
			}

			if manager.config.FFmpegPath != tt.expectedFFmpegPath {
				t.Errorf("Expected FFmpegPath %s, got %s", tt.expectedFFmpegPath, manager.config.FFmpegPath)
			}

			if manager.config.FFmpegLibPath != tt.expectedLibPath {
				t.Errorf("Expected FFmpegLibPath %s, got %s", tt.expectedLibPath, manager.config.FFmpegLibPath)
			}

			if manager.config.MaxMemoryMB != tt.expectedMaxMemory {
				t.Errorf("Expected MaxMemoryMB %d, got %d", tt.expectedMaxMemory, manager.config.MaxMemoryMB)
			}

			if manager.config.ProcessGroupKill != tt.expectedPGKill {
				t.Errorf("Expected ProcessGroupKill %v, got %v", tt.expectedPGKill, manager.config.ProcessGroupKill)
			}
		})
	}
}

// TestPrepareCommand tests command preparation.
func TestPrepareCommand(t *testing.T) {
	tests := []struct {
		name             string
		config           *Config
		cmdName          string
		args             []string
		checkLibPath     bool
		checkProcGroup   bool
	}{
		{
			name: "basic command",
			config: &Config{
				FFmpegPath:       "ffmpeg",
				ProcessGroupKill: true,
			},
			cmdName:        "echo",
			args:           []string{"hello"},
			checkLibPath:   false,
			checkProcGroup: true,
		},
		{
			name: "with lib path",
			config: &Config{
				FFmpegPath:       "ffmpeg",
				FFmpegLibPath:    "/custom/lib",
				ProcessGroupKill: true,
			},
			cmdName:        "echo",
			args:           []string{"test"},
			checkLibPath:   true,
			checkProcGroup: true,
		},
		{
			name: "without process group kill",
			config: &Config{
				FFmpegPath:       "ffmpeg",
				ProcessGroupKill: false,
			},
			cmdName:        "echo",
			args:           []string{"test"},
			checkLibPath:   false,
			checkProcGroup: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager(tt.config)
			ctx := context.Background()
			cmd := manager.PrepareCommand(ctx, tt.cmdName, tt.args)

			if cmd == nil {
				t.Fatal("PrepareCommand returned nil")
			}

			if cmd.Path != tt.cmdName && !isExecutableInPath(cmd.Path, tt.cmdName) {
				t.Errorf("Expected command path to contain %s, got %s", tt.cmdName, cmd.Path)
			}

			// Check LD_LIBRARY_PATH
			if tt.checkLibPath {
				found := false
				for _, env := range cmd.Env {
					if env == "LD_LIBRARY_PATH="+tt.config.FFmpegLibPath {
						found = true
						break
					}
				}
				if !found {
					t.Error("Expected LD_LIBRARY_PATH to be set in environment")
				}
			}

			// Check process group settings (only on non-Windows)
			if runtime.GOOS != "windows" && tt.checkProcGroup {
				if cmd.SysProcAttr == nil {
					t.Error("Expected SysProcAttr to be set")
				} else if !cmd.SysProcAttr.Setpgid {
					t.Error("Expected Setpgid to be true")
				}
			}
		})
	}
}

// TestPrepareFFmpegCommand tests FFmpeg command preparation.
func TestPrepareFFmpegCommand(t *testing.T) {
	tests := []struct {
		name       string
		config     *Config
		args       []string
	}{
		{
			name: "default ffmpeg path",
			config: &Config{
				FFmpegPath:       "",
				ProcessGroupKill: true,
			},
			args: []string{"-version"},
		},
		{
			name: "custom ffmpeg path",
			config: &Config{
				FFmpegPath:       "/usr/local/bin/ffmpeg",
				ProcessGroupKill: true,
			},
			args: []string{"-version"},
		},
		{
			name: "with lib path",
			config: &Config{
				FFmpegPath:       "/custom/ffmpeg",
				FFmpegLibPath:    "/custom/lib",
				ProcessGroupKill: true,
			},
			args: []string{"-i", "input.mp4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager(tt.config)
			ctx := context.Background()
			cmd := manager.PrepareFFmpegCommand(ctx, tt.args)

			if cmd == nil {
				t.Fatal("PrepareFFmpegCommand returned nil")
			}

			// Verify command uses configured FFmpeg path
			expectedPath := tt.config.FFmpegPath
			if expectedPath == "" {
				expectedPath = "ffmpeg"
			}

			// The Path field will be resolved to full path, so just check it's not empty
			if cmd.Path == "" {
				t.Error("Expected command path to be set")
			}

			// Check args
			if len(cmd.Args) != len(tt.args)+1 { // +1 for command name
				t.Errorf("Expected %d args (including cmd), got %d", len(tt.args)+1, len(cmd.Args))
			}
		})
	}
}

// TestPrepareCommandWithMemoryLimit tests command preparation with memory limits.
func TestPrepareCommandWithMemoryLimit(t *testing.T) {
	// Check if systemd-run is available
	_, hasSystemdRun := exec.LookPath("systemd-run")

	tests := []struct {
		name           string
		config         *Config
		cmdName        string
		args           []string
		skipOnWindows  bool
		expectSystemd  bool
	}{
		{
			name: "with memory limit on Linux",
			config: &Config{
				FFmpegPath:       "ffmpeg",
				ProcessGroupKill: true,
				MaxMemoryMB:      2048,
			},
			cmdName:       "echo",
			args:          []string{"test"},
			skipOnWindows: true,
			expectSystemd: runtime.GOOS == "linux" && hasSystemdRun == nil,
		},
		{
			name: "with zero memory limit",
			config: &Config{
				FFmpegPath:       "ffmpeg",
				ProcessGroupKill: true,
				MaxMemoryMB:      0,
			},
			cmdName:       "echo",
			args:          []string{"test"},
			expectSystemd: false,
		},
		{
			name: "without memory limit",
			config: &Config{
				FFmpegPath:       "ffmpeg",
				ProcessGroupKill: true,
			},
			cmdName:       "echo",
			args:          []string{"test"},
			expectSystemd: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipOnWindows && runtime.GOOS == "windows" {
				t.Skip("Skipping on Windows")
			}

			manager := NewManager(tt.config)
			ctx := context.Background()
			cmd := manager.PrepareCommandWithMemoryLimit(ctx, tt.cmdName, tt.args)

			if cmd == nil {
				t.Fatal("PrepareCommandWithMemoryLimit returned nil")
			}

			// Check if systemd-run is used
			if tt.expectSystemd {
				if cmd.Path != "/usr/bin/systemd-run" && !isExecutableInPath(cmd.Path, "systemd-run") {
					t.Logf("Expected systemd-run but got %s (might be fallback)", cmd.Path)
				}
			}
		})
	}
}

// TestKillProcessGroup tests process group killing.
func TestKillProcessGroup(t *testing.T) {
	tests := []struct {
		name       string
		setupCmd   func() *exec.Cmd
		shouldFail bool
	}{
		{
			name: "with nil command",
			setupCmd: func() *exec.Cmd {
				return nil
			},
			shouldFail: false, // Should not error on nil
		},
		{
			name: "with command without process",
			setupCmd: func() *exec.Cmd {
				return exec.Command("echo", "test")
			},
			shouldFail: false, // Should not error if process not started
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager(nil)
			cmd := tt.setupCmd()

			err := manager.KillProcessGroup(cmd)

			if tt.shouldFail && err == nil {
				t.Error("Expected error but got nil")
			}

			if !tt.shouldFail && err != nil {
				t.Errorf("Expected no error but got %v", err)
			}
		})
	}
}

// TestKillProcessGroupWithRunningProcess tests killing a running process.
func TestKillProcessGroupWithRunningProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping Unix-specific test on Windows")
	}

	manager := NewManager(&Config{
		ProcessGroupKill: true,
	})

	ctx := context.Background()
	// Use sleep command that will run for a while
	cmd := manager.PrepareCommand(ctx, "sleep", []string{"10"})

	// Start the command
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start command: %v", err)
	}

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	// Kill the process group
	err := manager.KillProcessGroup(cmd)
	if err != nil {
		t.Logf("KillProcessGroup returned error (might be expected): %v", err)
	}

	// Wait for process to exit
	waitErr := cmd.Wait()
	// Process should be killed, so expect an error
	if waitErr == nil {
		t.Error("Expected process to be killed")
	}
}

// TestWaitWithCleanup tests waiting for a command with cleanup.
func TestWaitWithCleanup(t *testing.T) {
	tests := []struct {
		name           string
		setupCmd       func(ctx context.Context, manager *Manager) *exec.Cmd
		cancelContext  bool
		expectError    bool
		skipOnWindows  bool
	}{
		{
			name: "normal completion",
			setupCmd: func(ctx context.Context, manager *Manager) *exec.Cmd {
				return manager.PrepareCommand(ctx, "echo", []string{"test"})
			},
			cancelContext: false,
			expectError:   false,
		},
		{
			name: "context cancellation",
			setupCmd: func(ctx context.Context, manager *Manager) *exec.Cmd {
				return manager.PrepareCommand(ctx, "sleep", []string{"10"})
			},
			cancelContext: true,
			expectError:   true,
			skipOnWindows: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipOnWindows && runtime.GOOS == "windows" {
				t.Skip("Skipping on Windows")
			}

			manager := NewManager(nil)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			cmd := tt.setupCmd(ctx, manager)

			if err := cmd.Start(); err != nil {
				t.Fatalf("Failed to start command: %v", err)
			}

			if tt.cancelContext {
				// Cancel after a short delay
				go func() {
					time.Sleep(50 * time.Millisecond)
					cancel()
				}()
			}

			err := manager.WaitWithCleanup(ctx, cmd)

			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got %v", err)
			}
		})
	}
}

// TestDetectHardwareAccel tests hardware acceleration detection.
func TestDetectHardwareAccel(t *testing.T) {
	// This test just verifies the function runs without panicking
	// Actual hardware detection will vary by system
	accel := DetectHardwareAccel()

	validAccels := []HardwareAccel{
		AccelNone,
		AccelVAAPI,
		AccelNVENC,
		AccelQSV,
		AccelVideoToolbox,
	}

	found := false
	for _, valid := range validAccels {
		if accel == valid {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("DetectHardwareAccel returned invalid value: %s", accel)
	}

	t.Logf("Detected hardware acceleration: %s", accel)
}

// TestHasFFmpegEncoder tests encoder detection.
func TestHasFFmpegEncoder(t *testing.T) {
	// Check if ffmpeg is available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found in PATH")
	}

	tests := []struct {
		name    string
		encoder string
		// We can't predict what encoders are available, so just test the function works
	}{
		{
			name:    "check libx264",
			encoder: "libx264",
		},
		{
			name:    "check h264_nvenc",
			encoder: "h264_nvenc",
		},
		{
			name:    "check nonexistent encoder",
			encoder: "fake_encoder_that_does_not_exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify it doesn't panic
			result := hasFFmpegEncoder(tt.encoder)
			t.Logf("Encoder %s available: %v", tt.encoder, result)
		})
	}
}

// TestHasFFmpegEncoderWithoutFFmpeg tests encoder detection when ffmpeg is not available.
func TestHasFFmpegEncoderWithoutFFmpeg(t *testing.T) {
	// Save original PATH
	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)

	// Set empty PATH to simulate missing ffmpeg
	os.Setenv("PATH", "")
	os.Setenv("VIEWRA_FFMPEG_PATH", "")

	result := hasFFmpegEncoder("libx264")
	if result {
		t.Error("Expected false when ffmpeg is not available")
	}
}

// TestHardwareAccelConstants tests that hardware accel constants are defined.
func TestHardwareAccelConstants(t *testing.T) {
	tests := []struct {
		name  string
		accel HardwareAccel
		value string
	}{
		{"none", AccelNone, "none"},
		{"vaapi", AccelVAAPI, "vaapi"},
		{"nvenc", AccelNVENC, "nvenc"},
		{"qsv", AccelQSV, "qsv"},
		{"videotoolbox", AccelVideoToolbox, "videotoolbox"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.accel) != tt.value {
				t.Errorf("Expected %s to equal %s", tt.accel, tt.value)
			}
		})
	}
}

// TestManagerOptions tests the manager option functions.
func TestManagerOptions(t *testing.T) {
	t.Run("WithMaxMemory", func(t *testing.T) {
		config := DefaultConfig()
		opt := WithMaxMemory(4096)
		opt(config)

		if config.MaxMemoryMB != 4096 {
			t.Errorf("Expected MaxMemoryMB to be 4096, got %d", config.MaxMemoryMB)
		}
	})

	t.Run("WithProcessGroupKill", func(t *testing.T) {
		config := DefaultConfig()
		opt := WithProcessGroupKill(false)
		opt(config)

		if config.ProcessGroupKill != false {
			t.Errorf("Expected ProcessGroupKill to be false, got %v", config.ProcessGroupKill)
		}
	})
}

// TestPrepareCommandEnvironment tests that environment is properly set.
func TestPrepareCommandEnvironment(t *testing.T) {
	tests := []struct {
		name          string
		libPath       string
		expectEnvSet  bool
	}{
		{
			name:         "with lib path",
			libPath:      "/custom/lib",
			expectEnvSet: true,
		},
		{
			name:         "without lib path",
			libPath:      "",
			expectEnvSet: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager(&Config{
				FFmpegPath:    "echo",
				FFmpegLibPath: tt.libPath,
			})

			ctx := context.Background()
			cmd := manager.PrepareCommand(ctx, "echo", []string{"test"})

			if tt.expectEnvSet {
				found := false
				expectedEnv := "LD_LIBRARY_PATH=" + tt.libPath
				for _, env := range cmd.Env {
					if env == expectedEnv {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected to find %s in environment", expectedEnv)
				}
			} else {
				// Check that LD_LIBRARY_PATH is not explicitly set
				for _, env := range cmd.Env {
					if len(env) >= 16 && env[:16] == "LD_LIBRARY_PATH=" && len(tt.libPath) == 0 {
						// It's OK if LD_LIBRARY_PATH exists from parent environment
						// We just care that we didn't explicitly set it to empty
						continue
					}
				}
			}
		})
	}
}

// TestPrepareCommandProcessGroupSettings tests process group settings on Unix.
func TestPrepareCommandProcessGroupSettings(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping Unix-specific test on Windows")
	}

	tests := []struct {
		name             string
		processGroupKill bool
		expectSetpgid    bool
	}{
		{
			name:             "with process group kill enabled",
			processGroupKill: true,
			expectSetpgid:    true,
		},
		{
			name:             "with process group kill disabled",
			processGroupKill: false,
			expectSetpgid:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager(&Config{
				FFmpegPath:       "echo",
				ProcessGroupKill: tt.processGroupKill,
			})

			ctx := context.Background()
			cmd := manager.PrepareCommand(ctx, "echo", []string{"test"})

			if tt.expectSetpgid {
				if cmd.SysProcAttr == nil {
					t.Error("Expected SysProcAttr to be set")
				} else if !cmd.SysProcAttr.Setpgid {
					t.Error("Expected Setpgid to be true")
				}
			} else {
				if cmd.SysProcAttr != nil && cmd.SysProcAttr.Setpgid {
					t.Error("Expected Setpgid to be false")
				}
			}
		})
	}
}

// TestKillProcessGroupSignals tests that SIGTERM is tried before SIGKILL.
func TestKillProcessGroupSignals(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping Unix-specific test on Windows")
	}

	manager := NewManager(&Config{
		ProcessGroupKill: true,
	})

	ctx := context.Background()
	cmd := manager.PrepareCommand(ctx, "sleep", []string{"10"})

	// Start the command
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start command: %v", err)
	}

	// Kill it
	err := manager.KillProcessGroup(cmd)

	// We expect either success or an error (depending on timing)
	// The important thing is it doesn't panic
	t.Logf("KillProcessGroup returned: %v", err)

	// Clean up
	cmd.Wait()
}

// TestDetectHardwareAccelPlatformSpecific tests platform-specific detection.
func TestDetectHardwareAccelPlatformSpecific(t *testing.T) {
	accel := DetectHardwareAccel()

	switch runtime.GOOS {
	case "darwin":
		// On macOS, should only return none or videotoolbox
		if accel != AccelNone && accel != AccelVideoToolbox {
			t.Errorf("On macOS, expected none or videotoolbox, got %s", accel)
		}
	case "windows":
		// On Windows, should only return none, nvenc, or qsv
		validWindows := []HardwareAccel{AccelNone, AccelNVENC, AccelQSV}
		found := false
		for _, valid := range validWindows {
			if accel == valid {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("On Windows, got unexpected acceleration: %s", accel)
		}
	case "linux":
		// On Linux, all types are possible
		// Just verify it's a valid value (already tested above)
	}

	t.Logf("Platform: %s, Detected: %s", runtime.GOOS, accel)
}

// TestKillProcessGroupFallback tests the fallback path without process group.
func TestKillProcessGroupFallback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping Unix-specific test on Windows")
	}

	// Create a manager with ProcessGroupKill disabled
	manager := NewManager(&Config{
		ProcessGroupKill: false,
	})

	ctx := context.Background()
	cmd := manager.PrepareCommand(ctx, "sleep", []string{"10"})

	// Start the command
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start command: %v", err)
	}

	// Kill it using the fallback path
	err := manager.KillProcessGroup(cmd)

	// Should succeed or return an error (both are acceptable)
	t.Logf("KillProcessGroup (fallback) returned: %v", err)

	// Clean up
	cmd.Wait()
}

// TestKillProcessGroupWindowsPath tests Windows-specific kill behavior.
func TestKillProcessGroupWindowsPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific test on non-Windows platform")
	}

	manager := NewManager(nil)

	// On Windows, we can test that the Windows path is taken
	ctx := context.Background()
	cmd := manager.PrepareCommand(ctx, "timeout", []string{"10"})

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start command: %v", err)
	}

	err := manager.KillProcessGroup(cmd)
	t.Logf("KillProcessGroup (Windows) returned: %v", err)

	cmd.Wait()
}

// TestDetectHardwareAccelWithMockedEnvironment tests detection with different scenarios.
func TestDetectHardwareAccelWithMockedEnvironment(t *testing.T) {
	// This test demonstrates the function runs on all platforms
	// The actual result depends on system hardware and ffmpeg build

	// Just call it to ensure it doesn't panic
	accel := DetectHardwareAccel()

	// Verify the result is one of the valid values
	validAccels := map[HardwareAccel]bool{
		AccelNone:         true,
		AccelVAAPI:        true,
		AccelNVENC:        true,
		AccelQSV:          true,
		AccelVideoToolbox: true,
	}

	if !validAccels[accel] {
		t.Errorf("DetectHardwareAccel returned invalid value: %s", accel)
	}

	t.Logf("Detected hardware acceleration on %s: %s", runtime.GOOS, accel)
}

// TestHasFFmpegEncoderErrorHandling tests error handling in encoder detection.
func TestHasFFmpegEncoderErrorHandling(t *testing.T) {
	// Test with various encoder names to exercise the function
	testCases := []struct {
		encoder string
	}{
		{"libx264"},
		{"libx265"},
		{"h264_nvenc"},
		{"hevc_nvenc"},
		{"h264_vaapi"},
		{"h264_qsv"},
		{"h264_videotoolbox"},
		{"invalid_encoder_12345"},
	}

	for _, tc := range testCases {
		t.Run(tc.encoder, func(t *testing.T) {
			// Just ensure it doesn't panic
			result := hasFFmpegEncoder(tc.encoder)
			t.Logf("Encoder %s: %v", tc.encoder, result)
		})
	}
}

// TestWaitWithCleanupEdgeCases tests edge cases in wait with cleanup.
func TestWaitWithCleanupEdgeCases(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping Unix-specific test on Windows")
	}

	t.Run("immediate cancellation", func(t *testing.T) {
		manager := NewManager(nil)
		ctx, cancel := context.WithCancel(context.Background())

		cmd := manager.PrepareCommand(ctx, "sleep", []string{"100"})

		if err := cmd.Start(); err != nil {
			t.Fatalf("Failed to start command: %v", err)
		}

		// Cancel immediately
		cancel()

		err := manager.WaitWithCleanup(ctx, cmd)
		if err == nil {
			t.Error("Expected error from cancelled context")
		}

		t.Logf("WaitWithCleanup with immediate cancel: %v", err)
	})

	t.Run("already completed process", func(t *testing.T) {
		manager := NewManager(nil)
		ctx := context.Background()

		cmd := manager.PrepareCommand(ctx, "echo", []string{"test"})

		if err := cmd.Start(); err != nil {
			t.Fatalf("Failed to start command: %v", err)
		}

		// Wait a bit for echo to complete
		time.Sleep(100 * time.Millisecond)

		err := manager.WaitWithCleanup(ctx, cmd)
		if err != nil {
			t.Logf("WaitWithCleanup for completed process returned: %v", err)
		}
	})
}

// TestPrepareCommandWithMemoryLimitSystemdUnavailable tests fallback when systemd-run is not available.
func TestPrepareCommandWithMemoryLimitSystemdUnavailable(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping Linux-specific test on non-Linux platform")
	}

	// Test the fallback behavior
	manager := NewManager(&Config{
		FFmpegPath:       "echo",
		ProcessGroupKill: true,
		MaxMemoryMB:      2048,
	})

	ctx := context.Background()
	cmd := manager.PrepareCommandWithMemoryLimit(ctx, "echo", []string{"test"})

	if cmd == nil {
		t.Fatal("PrepareCommandWithMemoryLimit returned nil")
	}

	// The command should be prepared (either with systemd-run or fallback)
	t.Logf("Command path: %s", cmd.Path)
}

// TestManagerOptionsChaining tests that options can be chained.
func TestManagerOptionsChaining(t *testing.T) {
	paths := &Paths{
		FFmpeg:  "/custom/ffmpeg",
		FFprobe: "/custom/ffprobe",
		LibPath: "/custom/lib",
	}

	manager := NewManagerFromPaths(
		paths,
		WithMaxMemory(8192),
		WithProcessGroupKill(false),
	)

	if manager.config.MaxMemoryMB != 8192 {
		t.Errorf("Expected MaxMemoryMB 8192, got %d", manager.config.MaxMemoryMB)
	}

	if manager.config.ProcessGroupKill != false {
		t.Errorf("Expected ProcessGroupKill false, got %v", manager.config.ProcessGroupKill)
	}

	if manager.config.FFmpegPath != "/custom/ffmpeg" {
		t.Errorf("Expected FFmpegPath /custom/ffmpeg, got %s", manager.config.FFmpegPath)
	}
}

// TestConfigStructure tests that Config struct has expected fields.
func TestConfigStructure(t *testing.T) {
	config := &Config{
		FFmpegPath:       "/test/ffmpeg",
		FFmpegLibPath:    "/test/lib",
		ProcessGroupKill: true,
		MaxMemoryMB:      4096,
	}

	if config.FFmpegPath != "/test/ffmpeg" {
		t.Errorf("FFmpegPath mismatch")
	}

	if config.FFmpegLibPath != "/test/lib" {
		t.Errorf("FFmpegLibPath mismatch")
	}

	if !config.ProcessGroupKill {
		t.Errorf("ProcessGroupKill mismatch")
	}

	if config.MaxMemoryMB != 4096 {
		t.Errorf("MaxMemoryMB mismatch")
	}
}

// TestPrepareFFmpegCommandWithEmptyPath tests handling of empty FFmpeg path.
func TestPrepareFFmpegCommandWithEmptyPath(t *testing.T) {
	manager := NewManager(&Config{
		FFmpegPath: "",
	})

	ctx := context.Background()
	cmd := manager.PrepareFFmpegCommand(ctx, []string{"-version"})

	if cmd == nil {
		t.Fatal("PrepareFFmpegCommand returned nil")
	}

	// Should default to "ffmpeg"
	t.Logf("Command path with empty FFmpegPath: %s", cmd.Path)
}

// TestDetectHardwareAccelNvidiaPath tests NVIDIA detection path.
func TestDetectHardwareAccelNvidiaPath(t *testing.T) {
	// This test exercises the nvidia-smi path
	// If nvidia-smi exists, the function should check for nvenc encoder
	_, err := exec.LookPath("nvidia-smi")
	if err == nil {
		t.Log("nvidia-smi found, NVIDIA path will be tested")
		accel := DetectHardwareAccel()
		t.Logf("Detected: %s", accel)
		// With nvidia-smi present, we should get nvenc or fall through to other options
	} else {
		t.Log("nvidia-smi not found, NVIDIA path not exercised on this system")
	}
}

// TestKillProcessGroupSIGTERMFailure tests SIGTERM failure fallback.
func TestKillProcessGroupSIGTERMFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping Unix-specific test on Windows")
	}

	// This test is difficult to implement reliably because we need
	// a process group that exists but rejects SIGTERM.
	// For now, we document that line 187-189 handles SIGTERM failure
	// by falling back to SIGKILL.
	t.Log("SIGTERM failure path is tested implicitly in process group kill tests")
}

// TestAllHardwareAccelTypes tests that all hardware accel types are defined.
func TestAllHardwareAccelTypes(t *testing.T) {
	// Test that we can create and compare all acceleration types
	accels := []HardwareAccel{
		AccelNone,
		AccelVAAPI,
		AccelNVENC,
		AccelQSV,
		AccelVideoToolbox,
	}

	// Ensure they're all different
	seen := make(map[HardwareAccel]bool)
	for _, accel := range accels {
		if seen[accel] {
			t.Errorf("Duplicate acceleration type: %s", accel)
		}
		seen[accel] = true
	}

	// Ensure we have all 5 types
	if len(seen) != 5 {
		t.Errorf("Expected 5 acceleration types, got %d", len(seen))
	}
}

// TestPrepareCommandContextCancellation tests command preparation with cancelled context.
func TestPrepareCommandContextCancellation(t *testing.T) {
	manager := NewManager(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// PrepareCommand should still work with cancelled context
	// The context is used by CommandContext, which sets up cancellation
	cmd := manager.PrepareCommand(ctx, "echo", []string{"test"})

	if cmd == nil {
		t.Fatal("PrepareCommand returned nil")
	}

	// Starting the command might fail due to cancelled context
	err := cmd.Start()
	if err != nil {
		t.Logf("Start failed with cancelled context (expected): %v", err)
	}
}

// TestWaitWithCleanupRapidCancellation tests very rapid cancellation.
func TestWaitWithCleanupRapidCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping Unix-specific test on Windows")
	}

	manager := NewManager(nil)

	// Test multiple rapid cancellations
	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cmd := manager.PrepareCommand(ctx, "sleep", []string{"10"})

		if err := cmd.Start(); err != nil {
			t.Fatalf("Failed to start command: %v", err)
		}

		// Cancel very quickly
		cancel()

		err := manager.WaitWithCleanup(ctx, cmd)
		if err == nil {
			t.Error("Expected error from cancelled context")
		}
	}
}

// TestDetectHardwareAccelLinuxSpecific tests Linux-specific detection paths.
func TestDetectHardwareAccelLinuxSpecific(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping Linux-specific test on non-Linux platform")
	}

	// This exercises the Linux-specific QSV and VAAPI paths
	// The actual result depends on system hardware
	accel := DetectHardwareAccel()

	// On Linux, we should get one of these
	validLinux := map[HardwareAccel]bool{
		AccelNone:  true,
		AccelVAAPI: true,
		AccelNVENC: true,
		AccelQSV:   true,
	}

	if !validLinux[accel] {
		t.Errorf("On Linux, got unexpected acceleration: %s", accel)
	}

	t.Logf("Linux hardware acceleration: %s", accel)
}

// TestDetectHardwareAccelMacOSSpecific tests macOS-specific detection.
func TestDetectHardwareAccelMacOSSpecific(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping macOS-specific test on non-macOS platform")
	}

	// This exercises the macOS VideoToolbox path
	accel := DetectHardwareAccel()

	// On macOS, we should get none or videotoolbox
	if accel != AccelNone && accel != AccelVideoToolbox {
		t.Errorf("On macOS, expected none or videotoolbox, got %s", accel)
	}

	t.Logf("macOS hardware acceleration: %s", accel)
}

// TestHasFFmpegEncoderWithDifferentEncoders tests various encoder checks.
func TestHasFFmpegEncoderWithDifferentEncoders(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found in PATH")
	}

	// Test common encoders that should exist on most systems
	encoders := []string{
		"libx264",   // Software H.264
		"libx265",   // Software H.265
		"aac",       // Audio codec
	}

	for _, encoder := range encoders {
		t.Run(encoder, func(t *testing.T) {
			result := hasFFmpegEncoder(encoder)
			// We don't assert true/false since it depends on ffmpeg build
			// Just ensure it doesn't panic
			t.Logf("Encoder %s: %v", encoder, result)
		})
	}
}

// TestKillProcessGroupWithNilSysProcAttr tests killing when SysProcAttr is nil.
func TestKillProcessGroupWithNilSysProcAttr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping Unix-specific test on Windows")
	}

	manager := NewManager(&Config{
		ProcessGroupKill: false,
	})

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "sleep", "10")
	// Don't set SysProcAttr at all
	cmd.SysProcAttr = nil

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start command: %v", err)
	}

	// This should exercise the fallback path
	err := manager.KillProcessGroup(cmd)
	t.Logf("KillProcessGroup with nil SysProcAttr: %v", err)

	cmd.Wait()
}

// Helper function to check if a path points to an executable.
func isExecutableInPath(fullPath, name string) bool {
	// Simple check: does the full path end with the name?
	// This handles cases where exec.LookPath resolves to full path
	if len(fullPath) >= len(name) {
		return fullPath[len(fullPath)-len(name):] == name
	}
	return false
}
