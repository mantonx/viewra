package executor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	infraffmpeg "github.com/mantonx/viewra/internal/infrastructure/ffmpeg"
	"github.com/mantonx/viewra/internal/infrastructure/ffmpeg/process"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/config"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/profile"
)

// Type aliases for test readability
type TranscodeConfig = config.TranscodeConfig
type AdaptiveProfile = profile.AdaptiveProfile
type HardwareAccel = config.HardwareAccel

// Constants from config package
const (
	AccelNone         = config.AccelNone
	AccelNVENC        = config.AccelNVENC
	AccelVAAPI        = config.AccelVAAPI
	AccelQSV          = config.AccelQSV
	AccelVideoToolbox = config.AccelVideoToolbox
)

// newTestProcessManager creates a process manager for tests
func newTestProcessManager(cfg *config.TranscodeConfig) *process.Manager {
	return process.NewManager(convertToProcessConfig(cfg))
}

// argsContain checks if the args slice contains a flag with the expected value
func argsContain(args []string, flag, expectedValue string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag && args[i+1] == expectedValue {
			return true
		}
	}
	return false
}

// argsContainFlag checks if the args slice contains a specific flag (without checking value)
func argsContainFlag(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag {
			return true
		}
	}
	return false
}

// TestNewFFmpegExecutor tests the creation of a new FFmpeg executor with default config.
func TestNewFFmpegExecutor(t *testing.T) {
	// This test assumes FFmpeg is available in the environment or VIEWRA_FFMPEG_PATH is set
	executor, err := NewFFmpegExecutor()

	// The error depends on whether FFmpeg is installed
	// We can't guarantee FFmpeg is available in test environment
	if err != nil {
		// If it fails, it should be because FFmpeg paths aren't configured
		if !strings.Contains(err.Error(), "ffmpeg") {
			t.Errorf("Expected ffmpeg-related error, got: %v", err)
		}
		return
	}

	if executor == nil {
		t.Fatal("NewFFmpegExecutor() returned nil executor without error")
	}

	if executor.Cfg == nil {
		t.Error("executor.Cfg should not be nil")
	}

	if executor.processManager == nil {
		t.Error("executor.processManager should not be nil")
	}
}

// TestNewFFmpegExecutorWithConfig tests the creation with a custom config.
func TestNewFFmpegExecutorWithConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *TranscodeConfig
		shouldError bool
		errorMsg    string
	}{
		{
			name: "Valid config",
			config: &TranscodeConfig{
				FFmpegPaths: &infraffmpeg.Paths{
					FFmpeg:  "/usr/bin/ffmpeg",
					FFprobe: "/usr/bin/ffprobe",
				},
				HardwareAccel:    AccelNone,
				ProcessGroupKill: true,
			},
			shouldError: false,
		},
		{
			name:        "Nil config uses default",
			config:      nil,
			shouldError: false,
		},
		{
			name: "Config with nil FFmpegPaths",
			config: &TranscodeConfig{
				FFmpegPaths:      nil,
				HardwareAccel:    AccelNone,
				ProcessGroupKill: true,
			},
			shouldError: true,
			errorMsg:    "ffmpeg paths not configured",
		},
		{
			name: "Config with empty FFmpeg path",
			config: &TranscodeConfig{
				FFmpegPaths: &infraffmpeg.Paths{
					FFmpeg:  "", // Empty path
					FFprobe: "/usr/bin/ffprobe",
				},
				HardwareAccel:    AccelNone,
				ProcessGroupKill: true,
			},
			shouldError: true,
			errorMsg:    "ffmpeg paths not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor, err := NewFFmpegExecutorWithConfig(tt.config)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errorMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got: %v", tt.errorMsg, err)
				}
				if executor != nil {
					t.Error("Expected nil executor when error occurs")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}
				if executor == nil {
					t.Fatal("Expected non-nil executor")
				}
				if executor.Cfg == nil {
					t.Error("executor.Cfg should not be nil")
				}
				if executor.processManager == nil {
					t.Error("executor.processManager should not be nil")
				}
			}
		})
	}
}

// TestBuildFFmpegArgs tests the FFmpeg argument building for transcoding.
func TestBuildFFmpegArgs(t *testing.T) {
	tests := []struct {
		name           string
		opts           TranscodeOptions
		hwAccel        HardwareAccel
		expectedArgs   []string // Args that must be present
		unexpectedArgs []string // Args that must NOT be present
	}{
		{
			name: "Basic transcoding with software encoding",
			opts: TranscodeOptions{
				InputPath: "/path/to/input.mp4",
				OutputDir: "/tmp/output",
				Profile: &AdaptiveProfile{
					ID:              "720p-4m",
					Width:           1280,
					Height:          720,
					VideoBitrate:    4_000_000,
					VideoMaxRate:    4_400_000,
					VideoBufSize:    8_000_000,
					AudioBitrate:    128_000,
					AudioChannels:   2,
					AudioSampleRate: 48000,
					GOPSize:         60,
					SegmentDuration: 4,
				},
			},
			hwAccel: AccelNone,
			expectedArgs: []string{
				"-loglevel", "error",
				"-i", "/path/to/input.mp4",
				"-c:v", "libx264",
				"-preset", "medium",
				"-b:v", "4000k",
				"-c:a", "aac",
				"-b:a", "128k",
				"-ac", "2",
				"-f", "hls",
			},
		},
		{
			name: "Transcoding with seek position",
			opts: TranscodeOptions{
				InputPath:        "/path/to/input.mkv",
				OutputDir:        "/tmp/output",
				UseStartPosition: true,
				StartPosition:    120, // Seek to 2 minutes
				Profile: &AdaptiveProfile{
					Width:           1920,
					Height:          1080,
					VideoBitrate:    8_000_000,
					VideoMaxRate:    8_800_000,
					VideoBufSize:    16_000_000,
					AudioBitrate:    192_000,
					AudioChannels:   2,
					AudioSampleRate: 48000,
					GOPSize:         60,
					SegmentDuration: 4,
				},
			},
			hwAccel: AccelNone,
			expectedArgs: []string{
				"-ss", "120",
				"-i", "/path/to/input.mkv",
			},
		},
		{
			name: "Transcoding with specific audio track",
			opts: TranscodeOptions{
				InputPath:             "/path/to/input.mkv",
				OutputDir:             "/tmp/output",
				UseSpecificAudioTrack: true,
				AudioTrackIndex:       2,
				Profile: &AdaptiveProfile{
					Width:           1920,
					Height:          1080,
					VideoBitrate:    8_000_000,
					VideoMaxRate:    8_800_000,
					VideoBufSize:    16_000_000,
					AudioBitrate:    192_000,
					AudioChannels:   2,
					AudioSampleRate: 48000,
					GOPSize:         60,
					SegmentDuration: 4,
				},
			},
			hwAccel: AccelNone,
			expectedArgs: []string{
				"-map", "0:v:0",
				"-map", "0:a:2",
			},
		},
		{
			name: "Transcoding with H.265 codec",
			opts: TranscodeOptions{
				InputPath: "/path/to/input.mp4",
				OutputDir: "/tmp/output",
				Profile: &AdaptiveProfile{
					Width:           3840,
					Height:          2160,
					PreferredCodec:  "h265",
					VideoBitrate:    40_000_000,
					VideoMaxRate:    44_000_000,
					VideoBufSize:    80_000_000,
					AudioBitrate:    192_000,
					AudioChannels:   2,
					AudioSampleRate: 48000,
					GOPSize:         60,
					SegmentDuration: 4,
				},
			},
			hwAccel: AccelNone,
			expectedArgs: []string{
				"-c:v", "libx265",
				"-preset", "medium",
			},
		},
		{
			name: "Transcoding with NVENC hardware acceleration",
			opts: TranscodeOptions{
				InputPath: "/path/to/input.mp4",
				OutputDir: "/tmp/output",
				Profile: &AdaptiveProfile{
					Width:           1920,
					Height:          1080,
					VideoBitrate:    8_000_000,
					VideoMaxRate:    8_800_000,
					VideoBufSize:    16_000_000,
					AudioBitrate:    192_000,
					AudioChannels:   2,
					AudioSampleRate: 48000,
					GOPSize:         60,
					SegmentDuration: 4,
				},
			},
			hwAccel: AccelNVENC,
			expectedArgs: []string{
				"-hwaccel", "cuda",
				"-c:v", "h264_nvenc",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &TranscodeConfig{
				FFmpegPaths: &infraffmpeg.Paths{
					FFmpeg:  "/usr/bin/ffmpeg",
					FFprobe: "/usr/bin/ffprobe",
				},
				HardwareAccel:    tt.hwAccel,
				ProcessGroupKill: true,
				MaxMemoryMB:      2048,
			}

			executor := &FFmpegExecutor{
				Cfg:            config,
				processManager: newTestProcessManager(config),
			}

			args := executor.buildFFmpegArgs(tt.opts)

			// Check that all expected args are present
			for i := 0; i < len(tt.expectedArgs); i += 2 {
				if i+1 < len(tt.expectedArgs) {
					flag := tt.expectedArgs[i]
					value := tt.expectedArgs[i+1]
					if !argsContain(args, flag, value) {
						t.Errorf("Expected args to contain %s %s, args: %v", flag, value, args)
					}
				}
			}

			// Check that unexpected args are not present
			for _, arg := range tt.unexpectedArgs {
				if argsContainFlag(args, arg) {
					t.Errorf("Expected args to NOT contain %s, args: %v", arg, args)
				}
			}

			// Verify output file path is correct
			playlistPath := filepath.Join(tt.opts.OutputDir, "playlist.m3u8")
			if !argsContainFlag(args, playlistPath) {
				t.Errorf("Expected args to contain output path %s", playlistPath)
			}
		})
	}
}

// TestBuildRemuxArgs tests the argument building for remuxing (copy streams).
func TestBuildRemuxArgs(t *testing.T) {
	opts := TranscodeOptions{
		InputPath: "/path/to/input.mkv",
		OutputDir: "/tmp/output",
		Profile: &AdaptiveProfile{
			Width:           1920,
			Height:          1080,
			VideoBitrate:    8_000_000,
			AudioBitrate:    192_000,
			AudioChannels:   2,
			AudioSampleRate: 48000,
			GOPSize:         60,
			SegmentDuration: 4,
		},
	}

	config := &TranscodeConfig{
		FFmpegPaths: &infraffmpeg.Paths{
			FFmpeg:  "/usr/bin/ffmpeg",
			FFprobe: "/usr/bin/ffprobe",
		},
		HardwareAccel:    AccelNone,
		ProcessGroupKill: true,
	}

	executor := &FFmpegExecutor{
		Cfg:            config,
		processManager: newTestProcessManager(config),
	}

	args := executor.buildRemuxArgs(opts)

	// Check key remux args
	expectedArgs := [][]string{
		{"-loglevel", "error"},
		{"-i", "/path/to/input.mkv"},
		{"-c:v", "copy"},
		{"-c:a", "copy"},
		{"-map", "0:v:0"},
		{"-map", "0:a:0"},
		{"-f", "hls"},
	}

	for _, pair := range expectedArgs {
		if !argsContain(args, pair[0], pair[1]) {
			t.Errorf("Expected args to contain %s %s, args: %v", pair[0], pair[1], args)
		}
	}

	// Verify no encoding parameters are present (since we're copying)
	unexpectedArgs := []string{"-b:v", "-preset", "-maxrate", "-bufsize"}
	for _, arg := range unexpectedArgs {
		if argsContainFlag(args, arg) {
			t.Errorf("Remux should not contain encoding parameter %s, args: %v", arg, args)
		}
	}
}

// TestBuildRemuxArgs_WithStartPosition tests remuxing with seek position.
func TestBuildRemuxArgs_WithStartPosition(t *testing.T) {
	opts := TranscodeOptions{
		InputPath: "/path/to/input.mkv",
		OutputDir: "/tmp/output",
		Profile: &AdaptiveProfile{
			Width:           1920,
			Height:          1080,
			VideoBitrate:    8_000_000,
			AudioBitrate:    192_000,
			AudioChannels:   2,
			AudioSampleRate: 48000,
			GOPSize:         60,
			SegmentDuration: 4,
		},
		StartPosition:    3600, // 1 hour
		UseStartPosition: true,
	}

	config := &TranscodeConfig{
		FFmpegPaths: &infraffmpeg.Paths{
			FFmpeg:  "/usr/bin/ffmpeg",
			FFprobe: "/usr/bin/ffprobe",
		},
		HardwareAccel:    AccelNone,
		ProcessGroupKill: true,
	}

	executor := &FFmpegExecutor{
		Cfg:            config,
		processManager: newTestProcessManager(config),
	}

	args := executor.buildRemuxArgs(opts)

	// Check for seek position - should have -ss with the start position
	if !argsContainFlag(args, "-ss") {
		t.Errorf("Expected args to contain -ss for start position, args: %v", args)
	}

	// Verify other key remux args are present
	if !argsContain(args, "-c:v", "copy") {
		t.Errorf("Expected args to contain -c:v copy, args: %v", args)
	}
}

// TestBuildRemuxWithAudioDownmixArgs_WithStartPosition tests remuxing with seek.
func TestBuildRemuxWithAudioDownmixArgs_WithStartPosition(t *testing.T) {
	opts := TranscodeOptions{
		InputPath: "/path/to/input.mkv",
		OutputDir: "/tmp/output",
		Profile: &AdaptiveProfile{
			Width:           1920,
			Height:          1080,
			VideoBitrate:    8_000_000,
			AudioBitrate:    192_000,
			AudioChannels:   2,
			AudioSampleRate: 48000,
			GOPSize:         60,
			SegmentDuration: 4,
		},
		StartPosition:    1800, // 30 minutes
		UseStartPosition: true,
	}

	config := &TranscodeConfig{
		FFmpegPaths: &infraffmpeg.Paths{
			FFmpeg:  "/usr/bin/ffmpeg",
			FFprobe: "/usr/bin/ffprobe",
		},
		HardwareAccel:    AccelNone,
		ProcessGroupKill: true,
	}

	executor := &FFmpegExecutor{
		Cfg:            config,
		processManager: newTestProcessManager(config),
	}

	args := executor.buildRemuxWithAudioDownmixArgs(opts)

	// Check for seek position
	if !argsContainFlag(args, "-ss") {
		t.Errorf("Expected args to contain -ss for start position, args: %v", args)
	}

	// Verify audio downmix is still present with seek
	if !argsContain(args, "-ac", "2") {
		t.Errorf("Expected args to contain -ac 2 for stereo downmix, args: %v", args)
	}

	if !argsContain(args, "-c:v", "copy") {
		t.Errorf("Expected args to contain -c:v copy, args: %v", args)
	}
}

// TestBuildRemuxWithAudioDownmixArgs tests remuxing with audio downmix.
func TestBuildRemuxWithAudioDownmixArgs(t *testing.T) {
	opts := TranscodeOptions{
		InputPath: "/path/to/input.mkv",
		OutputDir: "/tmp/output",
		Profile: &AdaptiveProfile{
			Width:           1920,
			Height:          1080,
			VideoBitrate:    8_000_000,
			AudioBitrate:    192_000,
			AudioChannels:   2,
			AudioSampleRate: 48000,
			GOPSize:         60,
			SegmentDuration: 4,
		},
	}

	config := &TranscodeConfig{
		FFmpegPaths: &infraffmpeg.Paths{
			FFmpeg:  "/usr/bin/ffmpeg",
			FFprobe: "/usr/bin/ffprobe",
		},
		HardwareAccel:    AccelNone,
		ProcessGroupKill: true,
	}

	executor := &FFmpegExecutor{
		Cfg:            config,
		processManager: newTestProcessManager(config),
	}

	args := executor.buildRemuxWithAudioDownmixArgs(opts)

	// Check key args
	expectedArgs := [][]string{
		{"-loglevel", "error"},
		{"-i", "/path/to/input.mkv"},
		{"-c:v", "copy"},        // Video should be copied
		{"-c:a", "aac"},         // Audio should be re-encoded
		{"-ac", "2"},            // Force stereo
		{"-map", "0:v:0"},
		{"-map", "0:a:0"},
		{"-f", "hls"},
	}

	for _, pair := range expectedArgs {
		if !argsContain(args, pair[0], pair[1]) {
			t.Errorf("Expected args to contain %s %s, args: %v", pair[0], pair[1], args)
		}
	}

	// Video encoding parameters should not be present
	unexpectedVideoArgs := []string{"-b:v", "-preset", "-maxrate"}
	for _, arg := range unexpectedVideoArgs {
		if argsContainFlag(args, arg) {
			t.Errorf("Remux with audio downmix should not contain video encoding parameter %s, args: %v", arg, args)
		}
	}
}

// TestGetVideoDuration tests duration extraction from video files.
func TestGetVideoDuration(t *testing.T) {
	t.Run("non-existent ffprobe binary", func(t *testing.T) {
		config := &TranscodeConfig{
			FFmpegPaths: &infraffmpeg.Paths{
				FFmpeg:  "/nonexistent/ffmpeg",
				FFprobe: "/nonexistent/ffprobe",
			},
			HardwareAccel:    AccelNone,
			ProcessGroupKill: true,
		}

		executor := &FFmpegExecutor{
			Cfg:            config,
			processManager: newTestProcessManager(config),
		}

		// Test with non-existent ffprobe binary
		duration, err := executor.getVideoDuration("/some/video.mp4")

		// Should return error since ffprobe doesn't exist
		if err == nil {
			t.Error("Expected error when ffprobe doesn't exist")
		}

		// Duration should be 0 on error
		if duration != 0 {
			t.Errorf("Expected duration to be 0 on error, got: %v", duration)
		}
	})

	t.Run("non-existent video file", func(t *testing.T) {
		// Try to get real ffprobe path
		paths, err := infraffmpeg.NewPaths()
		if err != nil {
			t.Skip("FFmpeg not available in test environment")
		}

		config := &TranscodeConfig{
			FFmpegPaths:      paths,
			HardwareAccel:    AccelNone,
			ProcessGroupKill: true,
		}

		executor := &FFmpegExecutor{
			Cfg:            config,
			processManager: newTestProcessManager(config),
		}

		// Test with non-existent file
		duration, err := executor.getVideoDuration("/nonexistent/video.mp4")

		// Should return error since file doesn't exist
		if err == nil {
			t.Error("Expected error when video file doesn't exist")
		}

		// Duration should be 0 on error
		if duration != 0 {
			t.Errorf("Expected duration to be 0 on error, got: %v", duration)
		}
	})

	t.Run("invalid duration format", func(t *testing.T) {
		// Try to get real ffprobe path
		paths, err := infraffmpeg.NewPaths()
		if err != nil {
			t.Skip("FFmpeg not available in test environment")
		}

		// Create a test file that's not a video (won't produce valid duration)
		tempDir := t.TempDir()
		testFile := filepath.Join(tempDir, "not_a_video.txt")
		if err := os.WriteFile(testFile, []byte("not a video"), 0o644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		config := &TranscodeConfig{
			FFmpegPaths:      paths,
			HardwareAccel:    AccelNone,
			ProcessGroupKill: true,
		}

		executor := &FFmpegExecutor{
			Cfg:            config,
			processManager: newTestProcessManager(config),
		}

		// Test with invalid file (not a video)
		duration, err := executor.getVideoDuration(testFile)

		// Should return error (either ffprobe fails or parse fails)
		if err == nil {
			t.Error("Expected error when file is not a video")
		}

		// Duration should be 0 on error
		if duration != 0 {
			t.Errorf("Expected duration to be 0 on error, got: %v", duration)
		}
	})
}

// TestCleanupOutputDir tests the cleanup of output directories.
func TestCleanupOutputDir(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create a test output directory with some files
	outputDir := filepath.Join(tempDir, "test_output")
	err := os.MkdirAll(outputDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create some test files
	testFiles := []string{"playlist.m3u8", "segment0.ts", "segment1.ts"}
	for _, file := range testFiles {
		filePath := filepath.Join(outputDir, file)
		if err := os.WriteFile(filePath, []byte("test"), 0o644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", file, err)
		}
	}

	// Verify files exist
	for _, file := range testFiles {
		filePath := filepath.Join(outputDir, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Fatalf("Test file %s should exist before cleanup", file)
		}
	}

	// Clean up the directory
	err = CleanupOutputDir(outputDir)
	if err != nil {
		t.Errorf("CleanupOutputDir failed: %v", err)
	}

	// Verify directory no longer exists
	if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
		t.Error("Output directory should not exist after cleanup")
	}
}

// TestCleanupOutputDir_NonExistent tests cleanup of non-existent directory.
func TestCleanupOutputDir_NonExistent(t *testing.T) {
	// Try to clean up a directory that doesn't exist
	err := CleanupOutputDir("/nonexistent/directory/path")

	// Should not return an error
	if err != nil {
		t.Errorf("CleanupOutputDir should not error on non-existent directory, got: %v", err)
	}
}

// TestCleanupOutputDir_NestedFiles tests cleanup with nested subdirectories.
func TestCleanupOutputDir_NestedFiles(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create nested structure
	outputDir := filepath.Join(tempDir, "test_output")
	subDir := filepath.Join(outputDir, "subdir")
	err := os.MkdirAll(subDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create test directories: %v", err)
	}

	// Create files at different levels
	files := map[string]string{
		filepath.Join(outputDir, "playlist.m3u8"): "root file",
		filepath.Join(subDir, "segment.ts"):       "nested file",
	}

	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", path, err)
		}
	}

	// Clean up
	err = CleanupOutputDir(outputDir)
	if err != nil {
		t.Errorf("CleanupOutputDir failed: %v", err)
	}

	// Verify entire directory tree is gone
	if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
		t.Error("Output directory and all contents should be removed")
	}
}

// TestBuildFFmpegArgs_MemorySafety verifies memory safety options are included.
func TestBuildFFmpegArgs_MemorySafety(t *testing.T) {
	opts := TranscodeOptions{
		InputPath: "/path/to/input.mp4",
		OutputDir: "/tmp/output",
		Profile: &AdaptiveProfile{
			Width:           1920,
			Height:          1080,
			VideoBitrate:    8_000_000,
			VideoMaxRate:    8_800_000,
			VideoBufSize:    16_000_000,
			AudioBitrate:    192_000,
			AudioChannels:   2,
			AudioSampleRate: 48000,
			GOPSize:         60,
			SegmentDuration: 4,
		},
	}

	config := &TranscodeConfig{
		FFmpegPaths: &infraffmpeg.Paths{
			FFmpeg:  "/usr/bin/ffmpeg",
			FFprobe: "/usr/bin/ffprobe",
		},
		HardwareAccel:    AccelNone,
		ProcessGroupKill: true,
		MaxMemoryMB:      3072, // 3GB
	}

	executor := &FFmpegExecutor{
		Cfg:            config,
		processManager: newTestProcessManager(config),
	}

	args := executor.buildFFmpegArgs(opts)

	// Memory safety options should be present
	expectedMaxAlloc := "3221225472" // 3GB in bytes
	if !argsContain(args, "-max_alloc", expectedMaxAlloc) {
		t.Errorf("Expected memory safety option -max_alloc %s in args: %v", expectedMaxAlloc, args)
	}

	// Note: -thread_queue_size is not used by the hls builder - memory safety
	// is primarily handled by -max_alloc which limits single allocation size
}

// TestBuildFFmpegArgs_HardwareAccelOptions verifies hardware accel args.
func TestBuildFFmpegArgs_HardwareAccelOptions(t *testing.T) {
	tests := []struct {
		name         string
		hwAccel      HardwareAccel
		expectedArgs []string
	}{
		{
			name:    "NVENC",
			hwAccel: AccelNVENC,
			expectedArgs: []string{
				"-hwaccel", "cuda",
				"-c:v", "h264_nvenc",
			},
		},
		{
			name:    "VAAPI",
			hwAccel: AccelVAAPI,
			expectedArgs: []string{
				"-hwaccel", "vaapi",
				"-c:v", "h264_vaapi",
			},
		},
		{
			name:    "QSV",
			hwAccel: AccelQSV,
			expectedArgs: []string{
				"-hwaccel", "qsv",
				"-c:v", "h264_qsv",
			},
		},
		{
			name:    "Software",
			hwAccel: AccelNone,
			expectedArgs: []string{
				"-c:v", "libx264",
				"-preset", "medium",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := TranscodeOptions{
				InputPath: "/path/to/input.mp4",
				OutputDir: "/tmp/output",
				Profile: &AdaptiveProfile{
					Width:           1920,
					Height:          1080,
					VideoBitrate:    8_000_000,
					VideoMaxRate:    8_800_000,
					VideoBufSize:    16_000_000,
					AudioBitrate:    192_000,
					AudioChannels:   2,
					AudioSampleRate: 48000,
					GOPSize:         60,
					SegmentDuration: 4,
				},
			}

			config := &TranscodeConfig{
				FFmpegPaths: &infraffmpeg.Paths{
					FFmpeg:  "/usr/bin/ffmpeg",
					FFprobe: "/usr/bin/ffprobe",
				},
				HardwareAccel:    tt.hwAccel,
				HardwareDevice:   "/dev/dri/renderD128",
				ProcessGroupKill: true,
				MaxMemoryMB:      2048,
			}

			executor := &FFmpegExecutor{
				Cfg:            config,
				processManager: newTestProcessManager(config),
			}

			args := executor.buildFFmpegArgs(opts)

			// Check hardware accel specific args
			for i := 0; i < len(tt.expectedArgs); i += 2 {
				if i+1 < len(tt.expectedArgs) {
					flag := tt.expectedArgs[i]
					value := tt.expectedArgs[i+1]
					if !argsContain(args, flag, value) {
						t.Errorf("Expected args to contain %s %s for %s, args: %v",
							flag, value, tt.name, args)
					}
				}
			}
		})
	}
}

// TestBuildFFmpegArgs_ProgressReporting verifies progress reporting args.
func TestBuildFFmpegArgs_ProgressReporting(t *testing.T) {
	opts := TranscodeOptions{
		InputPath: "/path/to/input.mp4",
		OutputDir: "/tmp/output",
		Profile: &AdaptiveProfile{
			Width:           1920,
			Height:          1080,
			VideoBitrate:    8_000_000,
			VideoMaxRate:    8_800_000,
			VideoBufSize:    16_000_000,
			AudioBitrate:    192_000,
			AudioChannels:   2,
			AudioSampleRate: 48000,
			GOPSize:         60,
			SegmentDuration: 4,
		},
	}

	config := &TranscodeConfig{
		FFmpegPaths: &infraffmpeg.Paths{
			FFmpeg:  "/usr/bin/ffmpeg",
			FFprobe: "/usr/bin/ffprobe",
		},
		HardwareAccel:    AccelNone,
		ProcessGroupKill: true,
		MaxMemoryMB:      2048,
	}

	executor := &FFmpegExecutor{
		Cfg:            config,
		processManager: newTestProcessManager(config),
	}

	args := executor.buildFFmpegArgs(opts)

	// Progress reporting should be enabled
	if !argsContain(args, "-progress", "pipe:2") {
		t.Errorf("Expected progress reporting in args: %v", args)
	}

	// Note: -stats is not used when -progress pipe:2 is present, as
	// -progress already provides structured progress output to stderr

	// Should always overwrite output
	if !argsContainFlag(args, "-y") {
		t.Errorf("Expected -y flag in args: %v", args)
	}
}

// TestExecuteFFmpeg_InvalidFFmpegPath tests error handling for invalid FFmpeg path.
func TestExecuteFFmpeg_InvalidFFmpegPath(t *testing.T) {
	config := &TranscodeConfig{
		FFmpegPaths: &infraffmpeg.Paths{
			FFmpeg:  "/nonexistent/ffmpeg",
			FFprobe: "/nonexistent/ffprobe",
		},
		HardwareAccel:    AccelNone,
		ProcessGroupKill: true,
		MaxMemoryMB:      2048,
	}

	executor := &FFmpegExecutor{
		Cfg:            config,
		processManager: newTestProcessManager(config),
	}

	opts := TranscodeOptions{
		InputPath: "/path/to/input.mp4",
		OutputDir: t.TempDir(),
		Profile: &AdaptiveProfile{
			Width:           1920,
			Height:          1080,
			VideoBitrate:    8_000_000,
			VideoMaxRate:    8_800_000,
			VideoBufSize:    16_000_000,
			AudioBitrate:    192_000,
			AudioChannels:   2,
			AudioSampleRate: 48000,
			GOPSize:         60,
			SegmentDuration: 4,
		},
	}

	args := executor.buildFFmpegArgs(opts)
	err := executor.executeFFmpeg(context.Background(), opts, args, "test")

	if err == nil {
		t.Error("Expected error when FFmpeg binary doesn't exist")
	}
}

// TestTranscodeToHLS_InvalidFFmpeg tests TranscodeToHLS with invalid FFmpeg.
func TestTranscodeToHLS_InvalidFFmpeg(t *testing.T) {
	config := &TranscodeConfig{
		FFmpegPaths: &infraffmpeg.Paths{
			FFmpeg:  "/nonexistent/ffmpeg",
			FFprobe: "/nonexistent/ffprobe",
		},
		HardwareAccel:    AccelNone,
		ProcessGroupKill: true,
		MaxMemoryMB:      2048,
	}

	executor := &FFmpegExecutor{
		Cfg:            config,
		processManager: newTestProcessManager(config),
	}

	opts := TranscodeOptions{
		InputPath: "/path/to/input.mp4",
		OutputDir: t.TempDir(),
		Profile: &AdaptiveProfile{
			Width:           1920,
			Height:          1080,
			VideoBitrate:    8_000_000,
			VideoMaxRate:    8_800_000,
			VideoBufSize:    16_000_000,
			AudioBitrate:    192_000,
			AudioChannels:   2,
			AudioSampleRate: 48000,
			GOPSize:         60,
			SegmentDuration: 4,
		},
	}

	err := executor.TranscodeToHLS(context.Background(), opts)

	if err == nil {
		t.Error("Expected error when FFmpeg binary doesn't exist")
	}
}

// TestRemuxToHLS_InvalidFFmpeg tests RemuxToHLS with invalid FFmpeg.
func TestRemuxToHLS_InvalidFFmpeg(t *testing.T) {
	config := &TranscodeConfig{
		FFmpegPaths: &infraffmpeg.Paths{
			FFmpeg:  "/nonexistent/ffmpeg",
			FFprobe: "/nonexistent/ffprobe",
		},
		HardwareAccel:    AccelNone,
		ProcessGroupKill: true,
		MaxMemoryMB:      2048,
	}

	executor := &FFmpegExecutor{
		Cfg:            config,
		processManager: newTestProcessManager(config),
	}

	opts := TranscodeOptions{
		InputPath: "/path/to/input.mp4",
		OutputDir: t.TempDir(),
		Profile: &AdaptiveProfile{
			Width:           1920,
			Height:          1080,
			VideoBitrate:    8_000_000,
			AudioBitrate:    192_000,
			AudioChannels:   2,
			AudioSampleRate: 48000,
			GOPSize:         60,
			SegmentDuration: 4,
		},
	}

	err := executor.RemuxToHLS(context.Background(), opts)

	if err == nil {
		t.Error("Expected error when FFmpeg binary doesn't exist")
	}
}

// TestRemuxWithAudioDownmixHLS_InvalidFFmpeg tests RemuxWithAudioDownmixHLS with invalid FFmpeg.
func TestRemuxWithAudioDownmixHLS_InvalidFFmpeg(t *testing.T) {
	config := &TranscodeConfig{
		FFmpegPaths: &infraffmpeg.Paths{
			FFmpeg:  "/nonexistent/ffmpeg",
			FFprobe: "/nonexistent/ffprobe",
		},
		HardwareAccel:    AccelNone,
		ProcessGroupKill: true,
		MaxMemoryMB:      2048,
	}

	executor := &FFmpegExecutor{
		Cfg:            config,
		processManager: newTestProcessManager(config),
	}

	opts := TranscodeOptions{
		InputPath: "/path/to/input.mp4",
		OutputDir: t.TempDir(),
		Profile: &AdaptiveProfile{
			Width:           1920,
			Height:          1080,
			VideoBitrate:    8_000_000,
			AudioBitrate:    192_000,
			AudioChannels:   2,
			AudioSampleRate: 48000,
			GOPSize:         60,
			SegmentDuration: 4,
		},
	}

	err := executor.RemuxWithAudioDownmixHLS(context.Background(), opts)

	if err == nil {
		t.Error("Expected error when FFmpeg binary doesn't exist")
	}
}

// TestExecuteFFmpeg_MissingOutputFile tests that error is returned when output file is not created.
func TestExecuteFFmpeg_MissingOutputFile(t *testing.T) {
	// Skip if FFmpeg is not available
	paths, err := infraffmpeg.NewPaths()
	if err != nil {
		t.Skip("FFmpeg not available in test environment")
	}

	config := &TranscodeConfig{
		FFmpegPaths:      paths,
		HardwareAccel:    AccelNone,
		ProcessGroupKill: true,
		MaxMemoryMB:      2048,
	}

	executor := &FFmpegExecutor{
		Cfg:            config,
		processManager: newTestProcessManager(config),
	}

	tempDir := t.TempDir()

	opts := TranscodeOptions{
		InputPath: "/nonexistent/input.mp4",
		OutputDir: tempDir,
		Profile: &AdaptiveProfile{
			Width:           1920,
			Height:          1080,
			VideoBitrate:    8_000_000,
			VideoMaxRate:    8_800_000,
			VideoBufSize:    16_000_000,
			AudioBitrate:    192_000,
			AudioChannels:   2,
			AudioSampleRate: 48000,
			GOPSize:         60,
			SegmentDuration: 4,
		},
	}

	args := executor.buildFFmpegArgs(opts)
	err = executor.executeFFmpeg(context.Background(), opts, args, "test")

	// Should fail because input file doesn't exist or output wasn't created
	if err == nil {
		t.Error("Expected error when processing invalid input")
	}
}

// TestExecuteFFmpeg_WithLibPath tests FFmpeg execution with custom library path.
func TestExecuteFFmpeg_WithLibPath(t *testing.T) {
	config := &TranscodeConfig{
		FFmpegPaths: &infraffmpeg.Paths{
			FFmpeg:  "/nonexistent/ffmpeg",
			FFprobe: "/nonexistent/ffprobe",
			LibPath: "/custom/lib/path",
		},
		HardwareAccel:    AccelNone,
		ProcessGroupKill: true,
		MaxMemoryMB:      2048,
	}

	executor := &FFmpegExecutor{
		Cfg:            config,
		processManager: newTestProcessManager(config),
	}

	tempDir := t.TempDir()

	opts := TranscodeOptions{
		InputPath: "/some/input.mp4",
		OutputDir: tempDir,
		Profile: &AdaptiveProfile{
			Width:           1920,
			Height:          1080,
			VideoBitrate:    8_000_000,
			VideoMaxRate:    8_800_000,
			VideoBufSize:    16_000_000,
			AudioBitrate:    192_000,
			AudioChannels:   2,
			AudioSampleRate: 48000,
			GOPSize:         60,
			SegmentDuration: 4,
		},
	}

	args := executor.buildFFmpegArgs(opts)
	err := executor.executeFFmpeg(context.Background(), opts, args, "test")

	// Should fail but code path with LibPath is exercised
	if err == nil {
		t.Error("Expected error when FFmpeg doesn't exist")
	}
}

// TestGetVideoDuration_Success tests successful duration extraction.
func TestGetVideoDuration_Success(t *testing.T) {
	// This test requires a real video file and ffprobe, so we skip in most environments
	// The function is already tested in error paths
	t.Skip("Requires real video file for integration testing")
}
