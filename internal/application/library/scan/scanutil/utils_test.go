package scanutil

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsMediaFile(t *testing.T) {
	tests := []struct {
		ext  string
		want bool
	}{
		// Video extensions
		{".mkv", true},
		{".mp4", true},
		{".avi", true},
		{".mov", true},
		{".m2ts", true},
		{"mkv", true},  // Without dot
		{".MKV", true}, // Uppercase

		// Audio extensions
		{".mp3", true},
		{".flac", true},
		{".m4a", true},

		// Non-media extensions
		{".txt", false},
		{".nfo", false},
		{".jpg", false},
		{".srt", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			if got := IsMediaFile(tt.ext); got != tt.want {
				t.Errorf("IsMediaFile(%q) = %v, want %v", tt.ext, got, tt.want)
			}
		})
	}
}

func TestIsAudioFile(t *testing.T) {
	tests := []struct {
		ext  string
		want bool
	}{
		// Audio extensions
		{".mp3", true},
		{".flac", true},
		{".m4a", true},
		{".wav", true},
		{".ogg", true},
		{"mp3", true},  // Without dot
		{".MP3", true}, // Uppercase

		// Video extensions (not audio)
		{".mkv", false},
		{".mp4", false},
		{".avi", false},

		// Non-media
		{".txt", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			if got := IsAudioFile(tt.ext); got != tt.want {
				t.Errorf("IsAudioFile(%q) = %v, want %v", tt.ext, got, tt.want)
			}
		})
	}
}

func TestIsExtra(t *testing.T) {
	tests := []struct {
		filepath string
		want     bool
	}{
		// Trailers
		{"/movies/Movie (2024)/Movie-trailer.mp4", true},
		{"/movies/Movie (2024)/Movie_trailer.mkv", true},
		{"/movies/Movie (2024)/Movie.trailer.mp4", true},

		// Deleted scenes
		{"/movies/Movie (2024)/Movie-deleted.mp4", true},
		{"/movies/Movie (2024)/Movie_deleted.mkv", true},

		// Featurettes
		{"/movies/Movie (2024)/Movie-featurette.mp4", true},
		{"/movies/Movie (2024)/Movie_featurette.mkv", true},

		// Extras
		{"/movies/Movie (2024)/Movie-extra.mp4", true},
		{"/movies/Movie (2024)/Movie_extra.mkv", true},

		// Bonus
		{"/movies/Movie (2024)/Movie-bonus.mp4", true},
		{"/movies/Movie (2024)/Movie_bonus.mkv", true},

		// Regular files (not extras)
		{"/movies/Movie (2024)/Movie (2024).mkv", false},
		{"/movies/Movie (2024)/Movie.en.srt", false},
		{"/tv/Show S01E01.mkv", false},

		// Case insensitive
		{"/movies/Movie-TRAILER.mp4", true},
		{"/movies/Movie-Deleted.mp4", true},
	}

	for _, tt := range tests {
		t.Run(tt.filepath, func(t *testing.T) {
			if got := IsExtra(tt.filepath); got != tt.want {
				t.Errorf("IsExtra(%q) = %v, want %v", tt.filepath, got, tt.want)
			}
		})
	}
}

func TestCalculateProcessingTimeout(t *testing.T) {
	const GB = 1024 * 1024 * 1024

	tests := []struct {
		name     string
		fileSize int64
		config   TimeoutConfig
		want     time.Duration
	}{
		{
			name:     "small local file",
			fileSize: 100 * 1024 * 1024, // 100MB
			config: TimeoutConfig{
				BaseFileTimeout:      30 * time.Second,
				RemoteStorageTimeout: 60 * time.Second,
				MaxExtraTimeout:      120 * time.Second,
				IsRemoteStorage:      false,
			},
			want: 30 * time.Second,
		},
		{
			name:     "small remote file",
			fileSize: 100 * 1024 * 1024, // 100MB
			config: TimeoutConfig{
				BaseFileTimeout:      30 * time.Second,
				RemoteStorageTimeout: 60 * time.Second,
				MaxExtraTimeout:      120 * time.Second,
				IsRemoteStorage:      true,
			},
			want: 60 * time.Second,
		},
		{
			name:     "5GB local file",
			fileSize: 5 * GB,
			config: TimeoutConfig{
				BaseFileTimeout:      30 * time.Second,
				RemoteStorageTimeout: 60 * time.Second,
				MaxExtraTimeout:      120 * time.Second,
				IsRemoteStorage:      false,
			},
			want: 35 * time.Second, // 30 + 5
		},
		{
			name:     "large file capped at MaxExtraTimeout",
			fileSize: 200 * GB,
			config: TimeoutConfig{
				BaseFileTimeout:      30 * time.Second,
				RemoteStorageTimeout: 60 * time.Second,
				MaxExtraTimeout:      120 * time.Second,
				IsRemoteStorage:      false,
			},
			want: 150 * time.Second, // 30 + 120 (capped)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalculateProcessingTimeout(tt.fileSize, tt.config); got != tt.want {
				t.Errorf("CalculateProcessingTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStatWithTimeout(t *testing.T) {
	// Create a temp file for testing
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("existing file succeeds", func(t *testing.T) {
		ctx := context.Background()
		info, err := StatWithTimeout(ctx, tmpFile, time.Second)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if info == nil {
			t.Error("expected file info, got nil")
		}
		if info.Name() != "test.txt" {
			t.Errorf("expected name 'test.txt', got %q", info.Name())
		}
	})

	t.Run("non-existent file returns error", func(t *testing.T) {
		ctx := context.Background()
		_, err := StatWithTimeout(ctx, "/nonexistent/path", time.Second)
		if err == nil {
			t.Error("expected error for non-existent file")
		}
	})

	t.Run("cancelled context returns error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		_, err := StatWithTimeout(ctx, tmpFile, time.Second)
		if err == nil {
			t.Error("expected error for cancelled context")
		}
	})
}

func BenchmarkIsMediaFile(b *testing.B) {
	extensions := []string{".mkv", ".mp4", ".avi", ".mp3", ".txt", ".nfo"}
	for i := 0; i < b.N; i++ {
		IsMediaFile(extensions[i%len(extensions)])
	}
}

func BenchmarkIsExtra(b *testing.B) {
	paths := []string{
		"/movies/Movie (2024)/Movie (2024).mkv",
		"/movies/Movie (2024)/Movie-trailer.mp4",
		"/movies/Movie (2024)/Movie-deleted.mp4",
	}
	for i := 0; i < b.N; i++ {
		IsExtra(paths[i%len(paths)])
	}
}
