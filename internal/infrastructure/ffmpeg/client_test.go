package ffmpeg

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	t.Run("successful client creation", func(t *testing.T) {
		client, err := NewClient()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if client == nil {
			t.Fatal("expected non-nil client")
		}
		if client.ffmpegPath == "" {
			t.Error("expected ffmpegPath to be set")
		}
		if client.ffprobePath == "" {
			t.Error("expected ffprobePath to be set")
		}
	})
}

func TestExtractMetadata(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	t.Run("invalid file path", func(t *testing.T) {
		ctx := context.Background()
		_, err := client.ExtractMetadata(ctx, "/nonexistent/file.mp4")
		if err != ErrInvalidFile {
			t.Errorf("expected ErrInvalidFile, got %v", err)
		}
	})

	t.Run("extract metadata from test video", func(t *testing.T) {
		// Create a minimal test video using FFmpeg
		testVideoPath := createTestVideo(t, client)
		defer os.Remove(testVideoPath)

		ctx := context.Background()
		metadata, err := client.ExtractMetadata(ctx, testVideoPath)
		if err != nil {
			t.Fatalf("failed to extract metadata: %v", err)
		}

		// Verify basic metadata
		if metadata.Duration == 0 {
			t.Error("expected non-zero duration")
		}
		if metadata.Width == 0 {
			t.Error("expected non-zero width")
		}
		if metadata.Height == 0 {
			t.Error("expected non-zero height")
		}
		if metadata.VideoCodec == "" {
			t.Error("expected video codec to be set")
		}
		if metadata.FileSize == 0 {
			t.Error("expected non-zero file size")
		}

		t.Logf("Extracted metadata: Duration=%v, Resolution=%dx%d, Codec=%s, FileSize=%d",
			metadata.Duration, metadata.Width, metadata.Height, metadata.VideoCodec, metadata.FileSize)
	})

	t.Run("context cancellation", func(t *testing.T) {
		testVideoPath := createTestVideo(t, client)
		defer os.Remove(testVideoPath)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := client.ExtractMetadata(ctx, testVideoPath)
		if err == nil {
			t.Error("expected error due to context cancellation")
		}
	})
}

func TestGenerateThumbnail(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	t.Run("invalid input file", func(t *testing.T) {
		ctx := context.Background()
		outputPath := filepath.Join(t.TempDir(), "thumb.jpg")
		opts := ThumbnailOptions{Timestamp: time.Second}

		err := client.GenerateThumbnail(ctx, "/nonexistent/file.mp4", outputPath, opts)
		if err != ErrInvalidFile {
			t.Errorf("expected ErrInvalidFile, got %v", err)
		}
	})

	t.Run("generate thumbnail with defaults", func(t *testing.T) {
		testVideoPath := createTestVideo(t, client)
		defer os.Remove(testVideoPath)

		outputPath := filepath.Join(t.TempDir(), "thumb.jpg")
		opts := ThumbnailOptions{
			Timestamp: time.Millisecond * 500,
		}

		ctx := context.Background()
		err := client.GenerateThumbnail(ctx, testVideoPath, outputPath, opts)
		if err != nil {
			t.Fatalf("failed to generate thumbnail: %v", err)
		}

		// Verify thumbnail was created
		info, err := os.Stat(outputPath)
		if err != nil {
			t.Fatalf("thumbnail file not found: %v", err)
		}
		if info.Size() == 0 {
			t.Error("expected non-zero thumbnail file size")
		}

		t.Logf("Generated thumbnail: %s (size: %d bytes)", outputPath, info.Size())
	})

	t.Run("generate thumbnail with custom dimensions", func(t *testing.T) {
		testVideoPath := createTestVideo(t, client)
		defer os.Remove(testVideoPath)

		outputPath := filepath.Join(t.TempDir(), "thumb_custom.jpg")
		opts := ThumbnailOptions{
			Timestamp: time.Millisecond * 500,
			Width:     320,
			Height:    180,
			Quality:   5,
		}

		ctx := context.Background()
		err := client.GenerateThumbnail(ctx, testVideoPath, outputPath, opts)
		if err != nil {
			t.Fatalf("failed to generate thumbnail: %v", err)
		}

		// Verify thumbnail was created
		info, err := os.Stat(outputPath)
		if err != nil {
			t.Fatalf("thumbnail file not found: %v", err)
		}
		if info.Size() == 0 {
			t.Error("expected non-zero thumbnail file size")
		}

		t.Logf("Generated custom thumbnail: %s (size: %d bytes)", outputPath, info.Size())
	})

	t.Run("generate thumbnail with auto-scaled width", func(t *testing.T) {
		testVideoPath := createTestVideo(t, client)
		defer os.Remove(testVideoPath)

		outputPath := filepath.Join(t.TempDir(), "thumb_auto.jpg")
		opts := ThumbnailOptions{
			Timestamp: time.Millisecond * 500,
			Height:    180, // Width will be auto-calculated
		}

		ctx := context.Background()
		err := client.GenerateThumbnail(ctx, testVideoPath, outputPath, opts)
		if err != nil {
			t.Fatalf("failed to generate thumbnail: %v", err)
		}

		// Verify thumbnail was created
		if _, err := os.Stat(outputPath); err != nil {
			t.Fatalf("thumbnail file not found: %v", err)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		testVideoPath := createTestVideo(t, client)
		defer os.Remove(testVideoPath)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		outputPath := filepath.Join(t.TempDir(), "thumb_cancelled.jpg")
		opts := ThumbnailOptions{Timestamp: time.Second}

		err := client.GenerateThumbnail(ctx, testVideoPath, outputPath, opts)
		if err == nil {
			t.Error("expected error due to context cancellation")
		}
	})
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "zero duration",
			duration: 0,
			expected: "00:00:00.000",
		},
		{
			name:     "one second",
			duration: time.Second,
			expected: "00:00:01.000",
		},
		{
			name:     "one minute",
			duration: time.Minute,
			expected: "00:01:00.000",
		},
		{
			name:     "one hour",
			duration: time.Hour,
			expected: "01:00:00.000",
		},
		{
			name:     "complex duration",
			duration: time.Hour + 23*time.Minute + 45*time.Second + 678*time.Millisecond,
			expected: "01:23:45.678",
		},
		{
			name:     "milliseconds only",
			duration: 500 * time.Millisecond,
			expected: "00:00:00.500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// createTestVideo creates a minimal test video file for testing purposes.
// The video is 2 seconds long, 640x480, with a simple color pattern.
func createTestVideo(t *testing.T, client *Client) string {
	t.Helper()

	tempDir := t.TempDir()
	videoPath := filepath.Join(tempDir, "test_video.mp4")

	// Generate a 2-second test video with a color pattern
	// -f lavfi: use libavfilter virtual input device
	// testsrc: generate test pattern
	// -t 2: duration of 2 seconds
	// -pix_fmt yuv420p: pixel format for compatibility
	cmd := exec.Command(client.ffmpegPath,
		"-f", "lavfi",
		"-i", "testsrc=duration=2:size=640x480:rate=30",
		"-pix_fmt", "yuv420p",
		"-c:v", "libx264",
		"-y",
		videoPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to create test video: %v\nOutput: %s", err, string(output))
	}

	// Verify the video was created
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		t.Fatalf("test video was not created at %s", videoPath)
	}

	return videoPath
}
