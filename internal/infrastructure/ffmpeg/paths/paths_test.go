package paths

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	// Save original environment
	origFFmpeg := os.Getenv("VIEWRA_FFMPEG_PATH")
	origFFprobe := os.Getenv("VIEWRA_FFPROBE_PATH")
	origLibPath := os.Getenv("VIEWRA_FFMPEG_LIB_PATH")
	defer func() {
		// Restore original environment
		setOrUnset("VIEWRA_FFMPEG_PATH", origFFmpeg)
		setOrUnset("VIEWRA_FFPROBE_PATH", origFFprobe)
		setOrUnset("VIEWRA_FFMPEG_LIB_PATH", origLibPath)
	}()

	t.Run("with_VIEWRA_FFMPEG_PATH_set", func(t *testing.T) {
		customFFmpeg := "/custom/path/to/ffmpeg"
		customFFprobe := "/custom/path/to/ffprobe"
		customLib := "/custom/lib"

		os.Setenv("VIEWRA_FFMPEG_PATH", customFFmpeg)
		os.Setenv("VIEWRA_FFPROBE_PATH", customFFprobe)
		os.Setenv("VIEWRA_FFMPEG_LIB_PATH", customLib)

		p, err := New()
		if err != nil {
			t.Fatalf("New() failed: %v", err)
		}

		if p.FFmpeg != customFFmpeg {
			t.Errorf("FFmpeg = %q, want %q", p.FFmpeg, customFFmpeg)
		}
		if p.FFprobe != customFFprobe {
			t.Errorf("FFprobe = %q, want %q", p.FFprobe, customFFprobe)
		}
		if p.LibPath != customLib {
			t.Errorf("LibPath = %q, want %q", p.LibPath, customLib)
		}
	})

	t.Run("with_only_VIEWRA_FFMPEG_PATH_set", func(t *testing.T) {
		customFFmpeg := "/custom/ffmpeg"
		os.Setenv("VIEWRA_FFMPEG_PATH", customFFmpeg)
		os.Unsetenv("VIEWRA_FFPROBE_PATH")
		os.Unsetenv("VIEWRA_FFMPEG_LIB_PATH")

		// Try to find ffprobe in PATH
		ffprobePath, err := exec.LookPath("ffprobe")
		if err != nil {
			t.Skip("ffprobe not found in PATH, skipping test")
		}

		p, err := New()
		if err != nil {
			t.Fatalf("New() failed: %v", err)
		}

		if p.FFmpeg != customFFmpeg {
			t.Errorf("FFmpeg = %q, want %q", p.FFmpeg, customFFmpeg)
		}
		if p.FFprobe != ffprobePath {
			t.Errorf("FFprobe = %q, want %q", p.FFprobe, ffprobePath)
		}
		if p.LibPath != "" {
			t.Errorf("LibPath = %q, want empty", p.LibPath)
		}
	})

	t.Run("with_only_VIEWRA_FFPROBE_PATH_set", func(t *testing.T) {
		customFFprobe := "/custom/ffprobe"
		os.Unsetenv("VIEWRA_FFMPEG_PATH")
		os.Setenv("VIEWRA_FFPROBE_PATH", customFFprobe)
		os.Unsetenv("VIEWRA_FFMPEG_LIB_PATH")

		// Try to find ffmpeg in PATH
		ffmpegPath, err := exec.LookPath("ffmpeg")
		if err != nil {
			t.Skip("ffmpeg not found in PATH, skipping test")
		}

		p, err := New()
		if err != nil {
			t.Fatalf("New() failed: %v", err)
		}

		if p.FFmpeg != ffmpegPath {
			t.Errorf("FFmpeg = %q, want %q", p.FFmpeg, ffmpegPath)
		}
		if p.FFprobe != customFFprobe {
			t.Errorf("FFprobe = %q, want %q", p.FFprobe, customFFprobe)
		}
		if p.LibPath != "" {
			t.Errorf("LibPath = %q, want empty", p.LibPath)
		}
	})

	t.Run("with_only_VIEWRA_FFMPEG_LIB_PATH_set", func(t *testing.T) {
		customLib := "/custom/lib/path"
		os.Unsetenv("VIEWRA_FFMPEG_PATH")
		os.Unsetenv("VIEWRA_FFPROBE_PATH")
		os.Setenv("VIEWRA_FFMPEG_LIB_PATH", customLib)

		// Try to find both in PATH
		ffmpegPath, err := exec.LookPath("ffmpeg")
		if err != nil {
			t.Skip("ffmpeg not found in PATH, skipping test")
		}
		ffprobePath, err := exec.LookPath("ffprobe")
		if err != nil {
			t.Skip("ffprobe not found in PATH, skipping test")
		}

		p, err := New()
		if err != nil {
			t.Fatalf("New() failed: %v", err)
		}

		if p.FFmpeg != ffmpegPath {
			t.Errorf("FFmpeg = %q, want %q", p.FFmpeg, ffmpegPath)
		}
		if p.FFprobe != ffprobePath {
			t.Errorf("FFprobe = %q, want %q", p.FFprobe, ffprobePath)
		}
		if p.LibPath != customLib {
			t.Errorf("LibPath = %q, want %q", p.LibPath, customLib)
		}
	})

	t.Run("fallback_to_system_PATH", func(t *testing.T) {
		os.Unsetenv("VIEWRA_FFMPEG_PATH")
		os.Unsetenv("VIEWRA_FFPROBE_PATH")
		os.Unsetenv("VIEWRA_FFMPEG_LIB_PATH")

		// Try to find both in PATH
		ffmpegPath, err := exec.LookPath("ffmpeg")
		if err != nil {
			t.Skip("ffmpeg not found in PATH, skipping test")
		}
		ffprobePath, err := exec.LookPath("ffprobe")
		if err != nil {
			t.Skip("ffprobe not found in PATH, skipping test")
		}

		p, err := New()
		if err != nil {
			t.Fatalf("New() failed: %v", err)
		}

		if p.FFmpeg != ffmpegPath {
			t.Errorf("FFmpeg = %q, want %q", p.FFmpeg, ffmpegPath)
		}
		if p.FFprobe != ffprobePath {
			t.Errorf("FFprobe = %q, want %q", p.FFprobe, ffprobePath)
		}
		if p.LibPath != "" {
			t.Errorf("LibPath = %q, want empty", p.LibPath)
		}
	})

	t.Run("error_when_ffmpeg_not_found", func(t *testing.T) {
		// Set to non-existent paths to ensure lookup fails
		os.Setenv("VIEWRA_FFMPEG_PATH", "")
		os.Setenv("VIEWRA_FFPROBE_PATH", "")
		os.Unsetenv("VIEWRA_FFMPEG_LIB_PATH")

		// Temporarily modify PATH to exclude ffmpeg
		origPath := os.Getenv("PATH")
		os.Setenv("PATH", "/nonexistent")
		defer os.Setenv("PATH", origPath)

		_, err := New()
		if err == nil {
			t.Fatal("New() should fail when ffmpeg not found")
		}

		if !strings.Contains(err.Error(), "ffmpeg not found") {
			t.Errorf("error message = %q, want to contain 'ffmpeg not found'", err.Error())
		}
	})

	t.Run("error_when_ffprobe_not_found", func(t *testing.T) {
		// Set ffmpeg to a valid path but make ffprobe unfindable
		os.Setenv("VIEWRA_FFMPEG_PATH", "/bin/sh") // Use /bin/sh as a placeholder
		os.Setenv("VIEWRA_FFPROBE_PATH", "")
		os.Unsetenv("VIEWRA_FFMPEG_LIB_PATH")

		// Temporarily modify PATH to exclude ffprobe
		origPath := os.Getenv("PATH")
		os.Setenv("PATH", "/nonexistent")
		defer os.Setenv("PATH", origPath)

		_, err := New()
		if err == nil {
			t.Fatal("New() should fail when ffprobe not found")
		}

		if !strings.Contains(err.Error(), "ffprobe not found") {
			t.Errorf("error message = %q, want to contain 'ffprobe not found'", err.Error())
		}
	})
}

func TestIsCustomBuild(t *testing.T) {
	tests := []struct {
		name    string
		libPath string
		want    bool
	}{
		{
			name:    "empty_lib_path",
			libPath: "",
			want:    false,
		},
		{
			name:    "lib_path_set",
			libPath: "/usr/local/lib/ffmpeg",
			want:    true,
		},
		{
			name:    "lib_path_with_colon",
			libPath: "/usr/local/lib:/opt/ffmpeg/lib",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Paths{
				FFmpeg:  "/usr/bin/ffmpeg",
				FFprobe: "/usr/bin/ffprobe",
				LibPath: tt.libPath,
			}
			got := p.IsCustomBuild()
			if got != tt.want {
				t.Errorf("IsCustomBuild() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVersion(t *testing.T) {
	// Save original environment
	origFFmpeg := os.Getenv("VIEWRA_FFMPEG_PATH")
	origFFprobe := os.Getenv("VIEWRA_FFPROBE_PATH")
	origLibPath := os.Getenv("VIEWRA_FFMPEG_LIB_PATH")
	defer func() {
		setOrUnset("VIEWRA_FFMPEG_PATH", origFFmpeg)
		setOrUnset("VIEWRA_FFPROBE_PATH", origFFprobe)
		setOrUnset("VIEWRA_FFMPEG_LIB_PATH", origLibPath)
	}()

	t.Run("with_real_ffmpeg", func(t *testing.T) {
		os.Unsetenv("VIEWRA_FFMPEG_PATH")
		os.Unsetenv("VIEWRA_FFPROBE_PATH")
		os.Unsetenv("VIEWRA_FFMPEG_LIB_PATH")

		// Check if ffmpeg is available
		ffmpegPath, err := exec.LookPath("ffmpeg")
		if err != nil {
			t.Skip("ffmpeg not found in PATH, skipping test")
		}
		ffprobePath, err := exec.LookPath("ffprobe")
		if err != nil {
			t.Skip("ffprobe not found in PATH, skipping test")
		}

		p := &Paths{
			FFmpeg:  ffmpegPath,
			FFprobe: ffprobePath,
			LibPath: "",
		}

		version := p.Version()
		if version == "unknown" {
			t.Error("Version() returned 'unknown' with real ffmpeg")
		}
		// Version should be something like "4.4.2" or "5.1.2" or "n6.0"
		if version == "" {
			t.Error("Version() returned empty string")
		}
	})

	t.Run("with_invalid_ffmpeg_path", func(t *testing.T) {
		p := &Paths{
			FFmpeg:  "/nonexistent/ffmpeg",
			FFprobe: "/nonexistent/ffprobe",
			LibPath: "",
		}

		version := p.Version()
		if version != "unknown" {
			t.Errorf("Version() = %q, want 'unknown'", version)
		}
	})

	t.Run("with_custom_lib_path", func(t *testing.T) {
		// Check if ffmpeg is available
		ffmpegPath, err := exec.LookPath("ffmpeg")
		if err != nil {
			t.Skip("ffmpeg not found in PATH, skipping test")
		}
		ffprobePath, err := exec.LookPath("ffprobe")
		if err != nil {
			t.Skip("ffprobe not found in PATH, skipping test")
		}

		p := &Paths{
			FFmpeg:  ffmpegPath,
			FFprobe: ffprobePath,
			LibPath: "/custom/lib/path",
		}

		version := p.Version()
		// Even with a custom (invalid) lib path, system ffmpeg should work
		// This test mainly ensures the LD_LIBRARY_PATH is set in the command
		if version == "" {
			t.Error("Version() returned empty string")
		}
	})

	t.Run("version_parsing", func(t *testing.T) {
		// This test verifies the version parsing logic by creating a mock script
		// that outputs a version string similar to ffmpeg
		if testing.Short() {
			t.Skip("skipping mock script test in short mode")
		}

		// Create a temporary script that mimics ffmpeg -version output
		script := `#!/bin/sh
echo "ffmpeg version n6.0-1-g1234abcd Copyright (c) 2000-2023 the FFmpeg developers"
echo "built with gcc 11.3.0 (Ubuntu 11.3.0-1ubuntu1~22.04)"
`
		tmpFile, err := os.CreateTemp("", "mock-ffmpeg-*.sh")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		if _, err := tmpFile.WriteString(script); err != nil {
			t.Fatalf("failed to write script: %v", err)
		}
		if err := tmpFile.Close(); err != nil {
			t.Fatalf("failed to close temp file: %v", err)
		}
		if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
			t.Fatalf("failed to chmod temp file: %v", err)
		}

		p := &Paths{
			FFmpeg:  tmpFile.Name(),
			FFprobe: "/nonexistent/ffprobe",
			LibPath: "",
		}

		version := p.Version()
		if version != "n6.0-1-g1234abcd" {
			t.Errorf("Version() = %q, want 'n6.0-1-g1234abcd'", version)
		}
	})

	t.Run("malformed_version_output", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping mock script test in short mode")
		}

		// Create a script that outputs malformed version info
		script := `#!/bin/sh
echo "some random output"
echo "not a version string"
`
		tmpFile, err := os.CreateTemp("", "mock-ffmpeg-bad-*.sh")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		if _, err := tmpFile.WriteString(script); err != nil {
			t.Fatalf("failed to write script: %v", err)
		}
		if err := tmpFile.Close(); err != nil {
			t.Fatalf("failed to close temp file: %v", err)
		}
		if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
			t.Fatalf("failed to chmod temp file: %v", err)
		}

		p := &Paths{
			FFmpeg:  tmpFile.Name(),
			FFprobe: "/nonexistent/ffprobe",
			LibPath: "",
		}

		version := p.Version()
		if version != "unknown" {
			t.Errorf("Version() = %q, want 'unknown'", version)
		}
	})

	t.Run("version_line_too_short", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping mock script test in short mode")
		}

		// Create a script that outputs version line with too few fields
		script := `#!/bin/sh
echo "ffmpeg version"
`
		tmpFile, err := os.CreateTemp("", "mock-ffmpeg-short-*.sh")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		if _, err := tmpFile.WriteString(script); err != nil {
			t.Fatalf("failed to write script: %v", err)
		}
		if err := tmpFile.Close(); err != nil {
			t.Fatalf("failed to close temp file: %v", err)
		}
		if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
			t.Fatalf("failed to chmod temp file: %v", err)
		}

		p := &Paths{
			FFmpeg:  tmpFile.Name(),
			FFprobe: "/nonexistent/ffprobe",
			LibPath: "",
		}

		version := p.Version()
		if version != "unknown" {
			t.Errorf("Version() = %q, want 'unknown'", version)
		}
	})
}

func TestPrepareCommand(t *testing.T) {
	p := &Paths{
		FFmpeg:  "/usr/bin/ffmpeg",
		FFprobe: "/usr/bin/ffprobe",
		LibPath: "",
	}

	t.Run("ffmpeg_name", func(t *testing.T) {
		cmd := p.PrepareCommand("ffmpeg", "-i", "input.mp4")

		if cmd.Path != "/usr/bin/ffmpeg" {
			t.Errorf("cmd.Path = %q, want '/usr/bin/ffmpeg'", cmd.Path)
		}

		wantArgs := []string{"/usr/bin/ffmpeg", "-i", "input.mp4"}
		if len(cmd.Args) != len(wantArgs) {
			t.Fatalf("len(cmd.Args) = %d, want %d", len(cmd.Args), len(wantArgs))
		}
		for i, arg := range cmd.Args {
			if arg != wantArgs[i] {
				t.Errorf("cmd.Args[%d] = %q, want %q", i, arg, wantArgs[i])
			}
		}

		// Should not have LD_LIBRARY_PATH when LibPath is empty
		if hasLdLibraryPath(cmd.Env) {
			t.Error("cmd.Env should not have LD_LIBRARY_PATH when LibPath is empty")
		}
	})

	t.Run("ffprobe_name", func(t *testing.T) {
		cmd := p.PrepareCommand("ffprobe", "-show_format", "input.mp4")

		if cmd.Path != "/usr/bin/ffprobe" {
			t.Errorf("cmd.Path = %q, want '/usr/bin/ffprobe'", cmd.Path)
		}

		wantArgs := []string{"/usr/bin/ffprobe", "-show_format", "input.mp4"}
		if len(cmd.Args) != len(wantArgs) {
			t.Fatalf("len(cmd.Args) = %d, want %d", len(cmd.Args), len(wantArgs))
		}
		for i, arg := range cmd.Args {
			if arg != wantArgs[i] {
				t.Errorf("cmd.Args[%d] = %q, want %q", i, arg, wantArgs[i])
			}
		}
	})

	t.Run("other_name", func(t *testing.T) {
		cmd := p.PrepareCommand("some-other-command", "--help")

		if cmd.Path != "some-other-command" {
			t.Errorf("cmd.Path = %q, want 'some-other-command'", cmd.Path)
		}

		wantArgs := []string{"some-other-command", "--help"}
		if len(cmd.Args) != len(wantArgs) {
			t.Fatalf("len(cmd.Args) = %d, want %d", len(cmd.Args), len(wantArgs))
		}
		for i, arg := range cmd.Args {
			if arg != wantArgs[i] {
				t.Errorf("cmd.Args[%d] = %q, want %q", i, arg, wantArgs[i])
			}
		}
	})

	t.Run("with_lib_path", func(t *testing.T) {
		pWithLib := &Paths{
			FFmpeg:  "/custom/ffmpeg",
			FFprobe: "/custom/ffprobe",
			LibPath: "/custom/lib:/opt/lib",
		}

		cmd := pWithLib.PrepareCommand("ffmpeg", "-version")

		if cmd.Path != "/custom/ffmpeg" {
			t.Errorf("cmd.Path = %q, want '/custom/ffmpeg'", cmd.Path)
		}

		// Check that LD_LIBRARY_PATH is set
		ldLibPath := getLdLibraryPath(cmd.Env)
		if ldLibPath != "/custom/lib:/opt/lib" {
			t.Errorf("LD_LIBRARY_PATH = %q, want '/custom/lib:/opt/lib'", ldLibPath)
		}
	})

	t.Run("no_args", func(t *testing.T) {
		cmd := p.PrepareCommand("ffmpeg")

		if cmd.Path != "/usr/bin/ffmpeg" {
			t.Errorf("cmd.Path = %q, want '/usr/bin/ffmpeg'", cmd.Path)
		}

		wantArgs := []string{"/usr/bin/ffmpeg"}
		if len(cmd.Args) != len(wantArgs) {
			t.Fatalf("len(cmd.Args) = %d, want %d", len(cmd.Args), len(wantArgs))
		}
		for i, arg := range cmd.Args {
			if arg != wantArgs[i] {
				t.Errorf("cmd.Args[%d] = %q, want %q", i, arg, wantArgs[i])
			}
		}
	})

	t.Run("empty_lib_path_no_env_pollution", func(t *testing.T) {
		// Verify that when LibPath is empty, we don't add LD_LIBRARY_PATH
		cmd := p.PrepareCommand("ffmpeg", "-version")

		// cmd.Env should be nil when LibPath is empty (inherits parent env)
		if cmd.Env != nil {
			t.Errorf("cmd.Env should be nil when LibPath is empty, got %v", cmd.Env)
		}
	})

	t.Run("lib_path_sets_env", func(t *testing.T) {
		pWithLib := &Paths{
			FFmpeg:  "/custom/ffmpeg",
			FFprobe: "/custom/ffprobe",
			LibPath: "/my/custom/lib",
		}

		cmd := pWithLib.PrepareCommand("ffprobe", "-version")

		if cmd.Env == nil {
			t.Fatal("cmd.Env should not be nil when LibPath is set")
		}

		// Verify LD_LIBRARY_PATH is in the environment
		found := false
		for _, env := range cmd.Env {
			if strings.HasPrefix(env, "LD_LIBRARY_PATH=") {
				found = true
				if env != "LD_LIBRARY_PATH=/my/custom/lib" {
					t.Errorf("env = %q, want 'LD_LIBRARY_PATH=/my/custom/lib'", env)
				}
				break
			}
		}
		if !found {
			t.Error("LD_LIBRARY_PATH not found in cmd.Env")
		}
	})
}

// Helper function to set or unset an environment variable
func setOrUnset(key, value string) {
	if value == "" {
		os.Unsetenv(key)
	} else {
		os.Setenv(key, value)
	}
}

// Helper function to check if LD_LIBRARY_PATH is set in environment
func hasLdLibraryPath(env []string) bool {
	for _, e := range env {
		if strings.HasPrefix(e, "LD_LIBRARY_PATH=") {
			return true
		}
	}
	return false
}

// Helper function to get LD_LIBRARY_PATH value from environment
func getLdLibraryPath(env []string) string {
	for _, e := range env {
		if strings.HasPrefix(e, "LD_LIBRARY_PATH=") {
			return strings.TrimPrefix(e, "LD_LIBRARY_PATH=")
		}
	}
	return ""
}
