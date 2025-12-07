package hls

import (
	"os/exec"
	"strings"
	"testing"
)

func TestSetFFmpegPath(t *testing.T) {
	// Save original values
	origPath := configuredFFmpegPath
	origLibDir := configuredFFmpegLibDir
	defer func() {
		configuredFFmpegPath = origPath
		configuredFFmpegLibDir = origLibDir
	}()

	// Test setting path
	SetFFmpegPath("/custom/ffmpeg", "/custom/lib")

	if configuredFFmpegPath != "/custom/ffmpeg" {
		t.Errorf("configuredFFmpegPath = %s, want /custom/ffmpeg", configuredFFmpegPath)
	}
	if configuredFFmpegLibDir != "/custom/lib" {
		t.Errorf("configuredFFmpegLibDir = %s, want /custom/lib", configuredFFmpegLibDir)
	}
}

func TestGetFFmpegPath(t *testing.T) {
	// Save original value
	origPath := configuredFFmpegPath
	defer func() {
		configuredFFmpegPath = origPath
	}()

	tests := []struct {
		name         string
		configuredPath string
		want         string
	}{
		{"default", "", "ffmpeg"},
		{"custom path", "/usr/local/bin/ffmpeg", "/usr/local/bin/ffmpeg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configuredFFmpegPath = tt.configuredPath
			got := getFFmpegPath()
			if got != tt.want {
				t.Errorf("getFFmpegPath() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestGetFFmpegLibDir(t *testing.T) {
	// Save original value
	origLibDir := configuredFFmpegLibDir
	defer func() {
		configuredFFmpegLibDir = origLibDir
	}()

	tests := []struct {
		name      string
		libDir    string
		want      string
	}{
		{"empty", "", ""},
		{"custom lib dir", "/opt/ffmpeg/lib", "/opt/ffmpeg/lib"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configuredFFmpegLibDir = tt.libDir
			got := getFFmpegLibDir()
			if got != tt.want {
				t.Errorf("getFFmpegLibDir() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestPrepareFFmpegCommand(t *testing.T) {
	// Save original values
	origPath := configuredFFmpegPath
	origLibDir := configuredFFmpegLibDir
	defer func() {
		configuredFFmpegPath = origPath
		configuredFFmpegLibDir = origLibDir
	}()

	tests := []struct {
		name       string
		ffmpegPath string
		libDir     string
		args       []string
		wantPath   string
		wantEnv    bool
	}{
		{
			name:       "default ffmpeg",
			ffmpegPath: "",
			libDir:     "",
			args:       []string{"-version"},
			wantPath:   "ffmpeg",
			wantEnv:    false,
		},
		{
			name:       "custom ffmpeg with lib dir",
			ffmpegPath: "/custom/ffmpeg",
			libDir:     "/custom/lib",
			args:       []string{"-version"},
			wantPath:   "/custom/ffmpeg",
			wantEnv:    true,
		},
		{
			name:       "custom ffmpeg without lib dir",
			ffmpegPath: "/usr/local/bin/ffmpeg",
			libDir:     "",
			args:       []string{"-version"},
			wantPath:   "/usr/local/bin/ffmpeg",
			wantEnv:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetFFmpegPath(tt.ffmpegPath, tt.libDir)
			cmd := prepareFFmpegCommand(tt.args...)

			if cmd.Path != tt.wantPath && !strings.HasSuffix(cmd.Path, tt.wantPath) {
				t.Errorf("cmd.Path = %s, want %s", cmd.Path, tt.wantPath)
			}

			hasLDLibraryPath := false
			if cmd.Env != nil {
				for _, env := range cmd.Env {
					if strings.HasPrefix(env, "LD_LIBRARY_PATH=") {
						hasLDLibraryPath = true
						if tt.libDir != "" && !strings.Contains(env, tt.libDir) {
							t.Errorf("LD_LIBRARY_PATH doesn't contain %s: %s", tt.libDir, env)
						}
					}
				}
			}

			if tt.wantEnv && !hasLDLibraryPath {
				t.Error("Expected LD_LIBRARY_PATH in environment")
			}
		})
	}
}

func TestNewHardwareTestEncoder(t *testing.T) {
	encoder := NewHardwareTestEncoder()
	if encoder == nil {
		t.Fatal("NewHardwareTestEncoder returned nil")
	}
}

func TestGetHardwareInitArgs(t *testing.T) {
	encoder := NewHardwareTestEncoder()

	tests := []struct {
		name         string
		hwAccel      HardwareAccel
		wantContains string
	}{
		{"vaapi", AccelVAAPI, "-init_hw_device"},
		{"qsv", AccelQSV, "-init_hw_device"},
		{"nvenc", AccelNVENC, "-hwaccel"},
		{"videotoolbox", AccelVideoToolbox, ""},
		{"none", AccelNone, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := encoder.getHardwareInitArgs(tt.hwAccel)

			if tt.wantContains == "" {
				if len(args) != 0 {
					t.Errorf("Expected no args, got %v", args)
				}
			} else {
				found := false
				for _, arg := range args {
					if strings.Contains(arg, tt.wantContains) || arg == tt.wantContains {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Args should contain %s, got %v", tt.wantContains, args)
				}
			}
		})
	}
}

func TestGetHardwareInitArgsWithDevice(t *testing.T) {
	encoder := NewHardwareTestEncoder()

	tests := []struct {
		name         string
		hwAccel      HardwareAccel
		device       string
		wantContains string
	}{
		{"vaapi with custom device", AccelVAAPI, "/dev/dri/renderD129", "renderD129"},
		{"qsv with custom device", AccelQSV, "/dev/dri/renderD130", "renderD130"},
		{"nvenc ignores device", AccelNVENC, "/dev/dri/renderD128", "-hwaccel"},
		{"videotoolbox no device", AccelVideoToolbox, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := encoder.getHardwareInitArgsWithDevice(tt.hwAccel, tt.device)

			if tt.wantContains == "" && len(args) > 0 {
				// For VideoToolbox, expect empty args
				if tt.hwAccel != AccelVideoToolbox || len(args) != 0 {
					t.Errorf("Expected no args, got %v", args)
				}
			} else if tt.wantContains != "" {
				found := false
				for _, arg := range args {
					if strings.Contains(arg, tt.wantContains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Args should contain %s, got %v", tt.wantContains, args)
				}
			}
		})
	}
}

func TestHardwareInitArgsVAAAPIStructure(t *testing.T) {
	encoder := NewHardwareTestEncoder()
	args := encoder.getHardwareInitArgs(AccelVAAPI)

	// VAAPI should have specific structure
	expectedArgs := []string{"-init_hw_device", "vaapi=va:/dev/dri/renderD128", "-filter_hw_device", "va", "-vf", "format=nv12,hwupload"}
	if len(args) != len(expectedArgs) {
		t.Fatalf("VAAPI args length = %d, want %d", len(args), len(expectedArgs))
	}

	for i, want := range expectedArgs {
		if args[i] != want {
			t.Errorf("VAAPI args[%d] = %s, want %s", i, args[i], want)
		}
	}
}

func TestHardwareInitArgsQSVStructure(t *testing.T) {
	encoder := NewHardwareTestEncoder()
	args := encoder.getHardwareInitArgs(AccelQSV)

	// QSV should have specific structure
	expectedArgs := []string{"-init_hw_device", "qsv=qsv:/dev/dri/renderD128", "-filter_hw_device", "qsv", "-vf", "hwupload=extra_hw_frames=64,format=qsv"}
	if len(args) != len(expectedArgs) {
		t.Fatalf("QSV args length = %d, want %d", len(args), len(expectedArgs))
	}

	for i, want := range expectedArgs {
		if args[i] != want {
			t.Errorf("QSV args[%d] = %s, want %s", i, args[i], want)
		}
	}
}

func TestHardwareInitArgsNVENCStructure(t *testing.T) {
	encoder := NewHardwareTestEncoder()
	args := encoder.getHardwareInitArgs(AccelNVENC)

	// NVENC should have hwaccel args
	expectedArgs := []string{"-hwaccel", "cuda", "-hwaccel_output_format", "cuda"}
	if len(args) != len(expectedArgs) {
		t.Fatalf("NVENC args length = %d, want %d", len(args), len(expectedArgs))
	}

	for i, want := range expectedArgs {
		if args[i] != want {
			t.Errorf("NVENC args[%d] = %s, want %s", i, args[i], want)
		}
	}
}

func TestCheckFFmpegEncoder(t *testing.T) {
	// Save original values
	origPath := configuredFFmpegPath
	defer func() {
		configuredFFmpegPath = origPath
	}()

	// This test will only work if ffmpeg is in PATH
	// We'll check for a common encoder
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not in PATH, skipping CheckFFmpegEncoder test")
	}

	// libx264 is very commonly available
	result := CheckFFmpegEncoder("libx264")
	// We can't assert true because ffmpeg might not have libx264
	// But we can verify it doesn't panic and returns a bool
	_ = result
}

func TestCheckFFmpegFilter(t *testing.T) {
	// Save original values
	origPath := configuredFFmpegPath
	defer func() {
		configuredFFmpegPath = origPath
	}()

	// This test will only work if ffmpeg is in PATH
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not in PATH, skipping CheckFFmpegFilter test")
	}

	// scale is a very common filter
	result := CheckFFmpegFilter("scale")
	// We can't assert true because ffmpeg might not be available
	// But we can verify it doesn't panic and returns a bool
	_ = result
}

func TestCheckFFmpegEncoderMissing(t *testing.T) {
	// Test with a definitely non-existent encoder
	result := CheckFFmpegEncoder("nonexistent_encoder_xyz_12345")
	if result {
		t.Error("CheckFFmpegEncoder should return false for non-existent encoder")
	}
}

func TestCheckFFmpegFilterMissing(t *testing.T) {
	// Test with a definitely non-existent filter
	result := CheckFFmpegFilter("nonexistent_filter_xyz_12345")
	if result {
		t.Error("CheckFFmpegFilter should return false for non-existent filter")
	}
}

func TestPrepareFFmpegCommandArgs(t *testing.T) {
	// Test that args are properly passed through
	args := []string{"-version", "-hide_banner"}
	cmd := prepareFFmpegCommand(args...)

	if len(cmd.Args) < len(args)+1 { // +1 for the command itself
		t.Errorf("Command args length = %d, want at least %d", len(cmd.Args), len(args)+1)
	}

	// Check that our args are in the command
	for _, arg := range args {
		found := false
		for _, cmdArg := range cmd.Args {
			if cmdArg == arg {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Arg %s not found in command args: %v", arg, cmd.Args)
		}
	}
}

func TestSetFFmpegPathConcurrency(t *testing.T) {
	// Test that concurrent reads/writes don't panic
	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			SetFFmpegPath("/path/"+string(rune(i)), "/lib/"+string(rune(i)))
		}
		done <- true
	}()

	// Reader goroutines
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = getFFmpegPath()
				_ = getFFmpegLibDir()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 6; i++ {
		<-done
	}
}

func TestHardwareTestEncoderDifferentAccelerators(t *testing.T) {
	encoder := NewHardwareTestEncoder()

	accelerators := []HardwareAccel{
		AccelNone,
		AccelNVENC,
		AccelVAAPI,
		AccelQSV,
		AccelVideoToolbox,
	}

	for _, accel := range accelerators {
		t.Run(string(accel), func(t *testing.T) {
			// Should not panic
			args := encoder.getHardwareInitArgs(accel)
			_ = args
		})
	}
}
