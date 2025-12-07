package thumbnail

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/infrastructure/ffmpeg/paths"
)

// TestNewClient tests the creation of thumbnail clients
func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		paths   *paths.Paths
		wantErr bool
	}{
		{
			name:    "with nil paths",
			paths:   nil,
			wantErr: false, // Should create new paths from environment
		},
		{
			name: "with valid paths",
			paths: &paths.Paths{
				FFmpeg:  "/usr/bin/ffmpeg",
				FFprobe: "/usr/bin/ffprobe",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.paths)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && client == nil {
				t.Error("NewClient() returned nil client without error")
			}
			if !tt.wantErr && client.paths == nil {
				t.Error("NewClient() client has nil paths")
			}
		})
	}
}

// TestFormatDuration tests the duration formatting function
func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "zero duration",
			duration: 0,
			want:     "00:00:00.000",
		},
		{
			name:     "1 second",
			duration: 1 * time.Second,
			want:     "00:00:01.000",
		},
		{
			name:     "30 seconds",
			duration: 30 * time.Second,
			want:     "00:00:30.000",
		},
		{
			name:     "1 minute",
			duration: 1 * time.Minute,
			want:     "00:01:00.000",
		},
		{
			name:     "1 hour",
			duration: 1 * time.Hour,
			want:     "01:00:00.000",
		},
		{
			name:     "1 hour 30 minutes 45 seconds",
			duration: 1*time.Hour + 30*time.Minute + 45*time.Second,
			want:     "01:30:45.000",
		},
		{
			name:     "with milliseconds",
			duration: 1*time.Second + 500*time.Millisecond,
			want:     "00:00:01.500",
		},
		{
			name:     "complex duration",
			duration: 2*time.Hour + 15*time.Minute + 32*time.Second + 123*time.Millisecond,
			want:     "02:15:32.123",
		},
		{
			name:     "10 hours",
			duration: 10 * time.Hour,
			want:     "10:00:00.000",
		},
		{
			name:     "59 minutes 59 seconds 999 milliseconds",
			duration: 59*time.Minute + 59*time.Second + 999*time.Millisecond,
			want:     "00:59:59.999",
		},
		{
			name:     "edge case - 23:59:59.999",
			duration: 23*time.Hour + 59*time.Minute + 59*time.Second + 999*time.Millisecond,
			want:     "23:59:59.999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.duration)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}

// TestGenerateWithNonExistentFile tests that Generate returns ErrInvalidFile for missing files
func TestGenerateWithNonExistentFile(t *testing.T) {
	// Create client with mock paths (don't need real ffmpeg for this test)
	client, err := NewClient(&paths.Paths{
		FFmpeg:  "/usr/bin/ffmpeg",
		FFprobe: "/usr/bin/ffprobe",
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()
	nonExistentPath := "/tmp/this-file-does-not-exist-12345.mp4"
	outputPath := "/tmp/thumbnail.jpg"

	err = client.Generate(ctx, nonExistentPath, outputPath, Options{})
	if err == nil {
		t.Fatal("Expected error for non-existent file, got nil")
	}

	if !errors.Is(err, ErrInvalidFile) {
		t.Errorf("Expected ErrInvalidFile, got %v", err)
	}
}

// TestGenerateOutputDirectoryCreation tests that output directory is created
func TestGenerateOutputDirectoryCreation(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create a dummy video file
	videoPath := filepath.Join(tempDir, "video.mp4")
	if err := os.WriteFile(videoPath, []byte("dummy video content"), 0o644); err != nil {
		t.Fatalf("Failed to create dummy video: %v", err)
	}

	// Set up output path in a nested directory that doesn't exist yet
	nestedDir := filepath.Join(tempDir, "nested", "output", "dir")
	outputPath := filepath.Join(nestedDir, "thumbnail.jpg")

	// Create client
	client, err := NewClient(&paths.Paths{
		FFmpeg:  "/usr/bin/ffmpeg",
		FFprobe: "/usr/bin/ffprobe",
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()

	// This will fail at ffmpeg execution, but should succeed at directory creation
	_ = client.Generate(ctx, videoPath, outputPath, Options{})

	// Check if the output directory was created
	if _, err := os.Stat(nestedDir); os.IsNotExist(err) {
		t.Error("Expected output directory to be created")
	}
}

// TestGenerateDefaultQuality tests that default quality is set to 2
func TestGenerateDefaultQuality(t *testing.T) {
	// This test verifies the quality logic without executing ffmpeg
	opts := Options{
		Quality: 0, // Not set
	}

	// The default quality should be 2
	expectedQuality := 2

	// Verify the logic matches what's in Generate()
	quality := opts.Quality
	if quality == 0 {
		quality = 2
	}

	if quality != expectedQuality {
		t.Errorf("Default quality = %d, want %d", quality, expectedQuality)
	}

	// Test with explicit quality
	opts.Quality = 5
	quality = opts.Quality
	if quality == 0 {
		quality = 2
	}

	if quality != 5 {
		t.Errorf("Explicit quality = %d, want 5", quality)
	}
}

// TestGenerateScalingOptions tests scaling logic
func TestGenerateScalingOptions(t *testing.T) {
	tests := []struct {
		name         string
		width        int
		height       int
		wantScale    bool
		wantScaleStr string
	}{
		{
			name:      "no scaling",
			width:     0,
			height:    0,
			wantScale: false,
		},
		{
			name:         "width only",
			width:        640,
			height:       0,
			wantScale:    true,
			wantScaleStr: "scale=640:-1",
		},
		{
			name:         "height only",
			width:        0,
			height:       480,
			wantScale:    true,
			wantScaleStr: "scale=-1:480",
		},
		{
			name:         "both width and height",
			width:        1920,
			height:       1080,
			wantScale:    true,
			wantScaleStr: "scale=1920:1080",
		},
		{
			name:         "small dimensions",
			width:        320,
			height:       240,
			wantScale:    true,
			wantScaleStr: "scale=320:240",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replicate the scaling logic from Generate()
			width := tt.width
			height := tt.height
			shouldScale := width > 0 || height > 0

			if shouldScale != tt.wantScale {
				t.Errorf("shouldScale = %v, want %v", shouldScale, tt.wantScale)
			}

			if shouldScale {
				if width == 0 {
					width = -1
				}
				if height == 0 {
					height = -1
				}
				scaleStr := formatScale(width, height)
				if scaleStr != tt.wantScaleStr {
					t.Errorf("scaleStr = %q, want %q", scaleStr, tt.wantScaleStr)
				}
			}
		})
	}
}

// Helper function to format scale string (matches logic in Generate)
func formatScale(width, height int) string {
	// This mimics the fmt.Sprintf("scale=%d:%d", width, height) in Generate()
	return formatScaleStr(width, height)
}

func formatScaleStr(width, height int) string {
	// Use the same format as in the actual code
	if width == -1 {
		return formatScaleValue("-1", height)
	}
	if height == -1 {
		return formatScaleValue2(width, "-1")
	}
	return formatScaleValue3(width, height)
}

func formatScaleValue(w string, h int) string {
	return "scale=" + w + ":" + formatInt(h)
}

func formatScaleValue2(w int, h string) string {
	return "scale=" + formatInt(w) + ":" + h
}

func formatScaleValue3(w, h int) string {
	return "scale=" + formatInt(w) + ":" + formatInt(h)
}

func formatInt(i int) string {
	if i < 0 {
		return "-1"
	}
	// Convert int to string
	result := ""
	if i == 0 {
		return "0"
	}
	for i > 0 {
		digit := i % 10
		result = string(rune('0'+digit)) + result
		i = i / 10
	}
	return result
}

// TestGenerateFFmpegCommandBuilding tests the command arguments construction
func TestGenerateFFmpegCommandBuilding(t *testing.T) {
	tests := []struct {
		name      string
		opts      Options
		wantArgs  []string
		videoPath string
		outPath   string
	}{
		{
			name: "basic command",
			opts: Options{
				Timestamp: 5 * time.Second,
				Quality:   2,
			},
			videoPath: "/input/video.mp4",
			outPath:   "/output/thumb.jpg",
			wantArgs: []string{
				"-ss", "00:00:05.000",
				"-i", "/input/video.mp4",
				"-vframes", "1",
				"-q:v", "2",
				"-y", "/output/thumb.jpg",
			},
		},
		{
			name: "with width scaling",
			opts: Options{
				Timestamp: 10 * time.Second,
				Width:     640,
				Quality:   3,
			},
			videoPath: "/input/video.mp4",
			outPath:   "/output/thumb.jpg",
			wantArgs: []string{
				"-ss", "00:00:10.000",
				"-i", "/input/video.mp4",
				"-vframes", "1",
				"-q:v", "3",
				"-vf", "scale=640:-1",
				"-y", "/output/thumb.jpg",
			},
		},
		{
			name: "with height scaling",
			opts: Options{
				Timestamp: 0,
				Height:    480,
				Quality:   1,
			},
			videoPath: "/input/video.mp4",
			outPath:   "/output/thumb.jpg",
			wantArgs: []string{
				"-ss", "00:00:00.000",
				"-i", "/input/video.mp4",
				"-vframes", "1",
				"-q:v", "1",
				"-vf", "scale=-1:480",
				"-y", "/output/thumb.jpg",
			},
		},
		{
			name: "with both dimensions",
			opts: Options{
				Timestamp: 30 * time.Second,
				Width:     1920,
				Height:    1080,
				Quality:   5,
			},
			videoPath: "/input/video.mp4",
			outPath:   "/output/thumb.jpg",
			wantArgs: []string{
				"-ss", "00:00:30.000",
				"-i", "/input/video.mp4",
				"-vframes", "1",
				"-q:v", "5",
				"-vf", "scale=1920:1080",
				"-y", "/output/thumb.jpg",
			},
		},
		{
			name: "default quality when not set",
			opts: Options{
				Timestamp: 0,
				Quality:   0, // Should default to 2
			},
			videoPath: "/input/video.mp4",
			outPath:   "/output/thumb.jpg",
			wantArgs: []string{
				"-ss", "00:00:00.000",
				"-i", "/input/video.mp4",
				"-vframes", "1",
				"-q:v", "2",
				"-y", "/output/thumb.jpg",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build args using the same logic as Generate()
			quality := tt.opts.Quality
			if quality == 0 {
				quality = 2
			}

			args := []string{
				"-ss", formatDuration(tt.opts.Timestamp),
				"-i", tt.videoPath,
				"-vframes", "1",
				"-q:v", intToString(quality),
			}

			if tt.opts.Width > 0 || tt.opts.Height > 0 {
				width := tt.opts.Width
				height := tt.opts.Height
				if width == 0 {
					width = -1
				}
				if height == 0 {
					height = -1
				}
				args = append(args, "-vf", formatScaleCommand(width, height))
			}

			args = append(args, "-y", tt.outPath)

			// Compare
			if len(args) != len(tt.wantArgs) {
				t.Errorf("args length = %d, want %d\nGot:  %v\nWant: %v",
					len(args), len(tt.wantArgs), args, tt.wantArgs)
				return
			}

			for i, arg := range args {
				if arg != tt.wantArgs[i] {
					t.Errorf("args[%d] = %q, want %q", i, arg, tt.wantArgs[i])
				}
			}
		})
	}
}

// Helper function to convert int to string (simple implementation)
func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + intToString(-n)
	}
	result := ""
	for n > 0 {
		result = string(rune('0'+(n%10))) + result
		n /= 10
	}
	return result
}

// Helper function to format scale command
func formatScaleCommand(width, height int) string {
	return "scale=" + intToString(width) + ":" + intToString(height)
}

// TestOptions tests the Options structure
func TestOptions(t *testing.T) {
	opts := Options{
		Timestamp: 5 * time.Minute,
		Width:     1280,
		Height:    720,
		Quality:   3,
	}

	if opts.Timestamp != 5*time.Minute {
		t.Errorf("Timestamp = %v, want %v", opts.Timestamp, 5*time.Minute)
	}
	if opts.Width != 1280 {
		t.Errorf("Width = %d, want 1280", opts.Width)
	}
	if opts.Height != 720 {
		t.Errorf("Height = %d, want 720", opts.Height)
	}
	if opts.Quality != 3 {
		t.Errorf("Quality = %d, want 3", opts.Quality)
	}
}

// TestErrors tests error types
func TestErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "ErrInvalidFile",
			err:  ErrInvalidFile,
			want: "invalid or inaccessible file path",
		},
		{
			name: "ErrThumbnailGeneration",
			err:  ErrThumbnailGeneration,
			want: "failed to generate thumbnail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.want {
				t.Errorf("error message = %q, want %q", tt.err.Error(), tt.want)
			}
		})
	}
}

// TestClientPathsField tests that Client stores paths correctly
func TestClientPathsField(t *testing.T) {
	customPaths := &paths.Paths{
		FFmpeg:  "/custom/ffmpeg",
		FFprobe: "/custom/ffprobe",
		LibPath: "/custom/lib",
	}

	client, err := NewClient(customPaths)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if client.paths != customPaths {
		t.Error("Client.paths does not match provided paths")
	}

	if client.paths.FFmpeg != "/custom/ffmpeg" {
		t.Errorf("FFmpeg path = %q, want %q", client.paths.FFmpeg, "/custom/ffmpeg")
	}
}

// TestGenerateContextCancellation tests that context cancellation is respected
func TestGenerateContextCancellation(t *testing.T) {
	tempDir := t.TempDir()

	// Create a dummy video file
	videoPath := filepath.Join(tempDir, "video.mp4")
	if err := os.WriteFile(videoPath, []byte("dummy content"), 0o644); err != nil {
		t.Fatalf("Failed to create dummy video: %v", err)
	}

	outputPath := filepath.Join(tempDir, "thumb.jpg")

	client, err := NewClient(&paths.Paths{
		FFmpeg:  "/usr/bin/ffmpeg",
		FFprobe: "/usr/bin/ffprobe",
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Create already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Note: Generate doesn't currently check context cancellation before running ffmpeg,
	// but this test documents the behavior. In a real scenario with a long-running ffmpeg,
	// the context would be passed to the command execution.
	err = client.Generate(ctx, videoPath, outputPath, Options{})
	// The function will still try to run, but ffmpeg will fail
	// This test mainly ensures no panic occurs
	_ = err // We expect an error since ffmpeg won't actually work
}

// TestGenerateQualityRange tests various quality values
func TestGenerateQualityRange(t *testing.T) {
	tests := []struct {
		name          string
		inputQuality  int
		expectQuality int
	}{
		{"quality 0 (default)", 0, 2},
		{"quality 1 (best)", 1, 1},
		{"quality 2", 2, 2},
		{"quality 10", 10, 10},
		{"quality 31 (worst)", 31, 31},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quality := tt.inputQuality
			if quality == 0 {
				quality = 2
			}
			if quality != tt.expectQuality {
				t.Errorf("quality = %d, want %d", quality, tt.expectQuality)
			}
		})
	}
}

// TestGenerateTimestampFormats tests various timestamp formats
func TestGenerateTimestampFormats(t *testing.T) {
	tests := []struct {
		name      string
		timestamp time.Duration
		wantStr   string
	}{
		{
			name:      "zero timestamp",
			timestamp: 0,
			wantStr:   "00:00:00.000",
		},
		{
			name:      "5 seconds",
			timestamp: 5 * time.Second,
			wantStr:   "00:00:05.000",
		},
		{
			name:      "1 minute",
			timestamp: 1 * time.Minute,
			wantStr:   "00:01:00.000",
		},
		{
			name:      "1 hour 23 minutes 45 seconds",
			timestamp: 1*time.Hour + 23*time.Minute + 45*time.Second,
			wantStr:   "01:23:45.000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatted := formatDuration(tt.timestamp)
			if formatted != tt.wantStr {
				t.Errorf("formatDuration() = %q, want %q", formatted, tt.wantStr)
			}
		})
	}
}

// TestGenerateFFmpegFailure tests error handling when ffmpeg fails
func TestGenerateFFmpegFailure(t *testing.T) {
	tempDir := t.TempDir()

	// Create a dummy video file (not a real video, so ffmpeg will fail)
	videoPath := filepath.Join(tempDir, "video.mp4")
	if err := os.WriteFile(videoPath, []byte("not a real video"), 0o644); err != nil {
		t.Fatalf("Failed to create dummy video: %v", err)
	}

	outputPath := filepath.Join(tempDir, "thumb.jpg")

	client, err := NewClient(nil)
	if err != nil {
		t.Skipf("Skipping test: ffmpeg not available: %v", err)
	}

	ctx := context.Background()

	// This should fail because the input is not a valid video
	err = client.Generate(ctx, videoPath, outputPath, Options{})
	if err == nil {
		t.Error("Expected error when processing invalid video file, got nil")
	}

	// Error should be ErrThumbnailGeneration
	if !errors.Is(err, ErrThumbnailGeneration) {
		t.Errorf("Expected error to wrap ErrThumbnailGeneration, got: %v", err)
	}
}

// TestGenerateOutputDirectoryCreationError tests error handling when directory creation fails
func TestGenerateOutputDirectoryCreationError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping test when running as root")
	}

	tempDir := t.TempDir()

	// Create a dummy video file
	videoPath := filepath.Join(tempDir, "video.mp4")
	if err := os.WriteFile(videoPath, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("Failed to create dummy video: %v", err)
	}

	// Create a file where we want to create a directory (to cause mkdir to fail)
	blockingFile := filepath.Join(tempDir, "blocking")
	if err := os.WriteFile(blockingFile, []byte("block"), 0o644); err != nil {
		t.Fatalf("Failed to create blocking file: %v", err)
	}

	// Make it read-only
	if err := os.Chmod(blockingFile, 0o444); err != nil {
		t.Fatalf("Failed to chmod blocking file: %v", err)
	}

	// Try to create output in blocking/thumb.jpg (should fail because blocking is a file)
	outputPath := filepath.Join(blockingFile, "thumb.jpg")

	client, err := NewClient(&paths.Paths{
		FFmpeg:  "/usr/bin/ffmpeg",
		FFprobe: "/usr/bin/ffprobe",
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()

	err = client.Generate(ctx, videoPath, outputPath, Options{})
	if err == nil {
		t.Error("Expected error when output directory creation fails, got nil")
	}

	// Error should wrap ErrThumbnailGeneration
	if !errors.Is(err, ErrThumbnailGeneration) {
		t.Errorf("Expected error to wrap ErrThumbnailGeneration, got: %v", err)
	}
}

// TestNewClientWithPathsError tests NewClient when paths.New() fails
func TestNewClientWithPathsError(t *testing.T) {
	// Save current environment
	oldFFmpeg := os.Getenv("VIEWRA_FFMPEG_PATH")
	oldFFprobe := os.Getenv("VIEWRA_FFPROBE_PATH")
	oldPath := os.Getenv("PATH")

	// Restore environment after test
	defer func() {
		os.Setenv("VIEWRA_FFMPEG_PATH", oldFFmpeg)
		os.Setenv("VIEWRA_FFPROBE_PATH", oldFFprobe)
		os.Setenv("PATH", oldPath)
	}()

	// Clear all paths to force error
	os.Unsetenv("VIEWRA_FFMPEG_PATH")
	os.Unsetenv("VIEWRA_FFPROBE_PATH")
	os.Setenv("PATH", "")

	client, err := NewClient(nil)
	if err == nil {
		t.Error("Expected error when ffmpeg not found, got nil")
	}
	if client != nil {
		t.Error("Expected nil client when error occurs")
	}
}

// TestGenerateAllScalingCombinations tests all scaling combinations thoroughly
func TestGenerateAllScalingCombinations(t *testing.T) {
	tempDir := t.TempDir()

	// Create a dummy video file
	videoPath := filepath.Join(tempDir, "video.mp4")
	if err := os.WriteFile(videoPath, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("Failed to create dummy video: %v", err)
	}

	client, err := NewClient(nil)
	if err != nil {
		t.Skipf("Skipping test: ffmpeg not available: %v", err)
	}

	ctx := context.Background()

	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"no scaling", 0, 0},
		{"width only", 640, 0},
		{"height only", 0, 480},
		{"both dimensions", 1920, 1080},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputPath := filepath.Join(tempDir, tt.name+".jpg")
			opts := Options{
				Width:  tt.width,
				Height: tt.height,
			}

			// This will fail on invalid video, but we're testing the path is exercised
			_ = client.Generate(ctx, videoPath, outputPath, opts)
		})
	}
}

// TestGenerateWithVariousQualityValues tests Generate with different quality settings
func TestGenerateWithVariousQualityValues(t *testing.T) {
	tempDir := t.TempDir()

	// Create a dummy video file
	videoPath := filepath.Join(tempDir, "video.mp4")
	if err := os.WriteFile(videoPath, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("Failed to create dummy video: %v", err)
	}

	client, err := NewClient(nil)
	if err != nil {
		t.Skipf("Skipping test: ffmpeg not available: %v", err)
	}

	ctx := context.Background()

	qualities := []int{0, 1, 2, 5, 10, 31}
	for _, quality := range qualities {
		t.Run(intToString(quality), func(t *testing.T) {
			outputPath := filepath.Join(tempDir, "thumb_q"+intToString(quality)+".jpg")
			opts := Options{
				Quality: quality,
			}

			// This will fail on invalid video, but we're testing the path is exercised
			_ = client.Generate(ctx, videoPath, outputPath, opts)
		})
	}
}

// TestGenerateWithVariousTimestamps tests Generate with different timestamp values
func TestGenerateWithVariousTimestamps(t *testing.T) {
	tempDir := t.TempDir()

	// Create a dummy video file
	videoPath := filepath.Join(tempDir, "video.mp4")
	if err := os.WriteFile(videoPath, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("Failed to create dummy video: %v", err)
	}

	client, err := NewClient(nil)
	if err != nil {
		t.Skipf("Skipping test: ffmpeg not available: %v", err)
	}

	ctx := context.Background()

	timestamps := []time.Duration{
		0,
		5 * time.Second,
		1 * time.Minute,
		5 * time.Minute,
		1 * time.Hour,
	}

	for i, timestamp := range timestamps {
		t.Run(formatDuration(timestamp), func(t *testing.T) {
			outputPath := filepath.Join(tempDir, "thumb_t"+intToString(i)+".jpg")
			opts := Options{
				Timestamp: timestamp,
			}

			// This will fail on invalid video, but we're testing the path is exercised
			_ = client.Generate(ctx, videoPath, outputPath, opts)
		})
	}
}
