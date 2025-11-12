package filesystem

import (
	"os"
	"testing"
)

func TestNormalizeExtension(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"uppercase", "file.MKV", ".mkv"},
		{"lowercase", "file.mkv", ".mkv"},
		{"mixed case", "file.Mp4", ".mp4"},
		{"no extension", "file", ""},
		{"hidden file", ".bashrc", ".bashrc"},
		{"multiple dots", "file.backup.txt", ".txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeExtension(tt.path)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestHasExtension(t *testing.T) {
	videoExts := []string{".mkv", ".mp4", ".avi"}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"has mkv", "movie.mkv", true},
		{"has MKV uppercase", "movie.MKV", true},
		{"has mp4", "video.mp4", true},
		{"has avi", "clip.avi", true},
		{"no match", "document.pdf", false},
		{"no extension", "readme", false},
		{"empty path", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasExtension(tt.path, videoExts)
			if result != tt.expected {
				t.Errorf("Expected %v for path %q", tt.expected, tt.path)
			}
		})
	}
}

func TestHasExtensionEmptyList(t *testing.T) {
	result := hasExtension("file.mkv", []string{})
	if result {
		t.Error("Expected false for empty extension list")
	}
}

func TestContainsAny(t *testing.T) {
	patterns := []string{"poster", "banner", "thumb"}

	tests := []struct {
		name     string
		str      string
		expected bool
	}{
		{"contains poster", "movie-poster.jpg", true},
		{"contains banner", "show-banner.png", true},
		{"contains thumb", "thumbnail.jpg", true},
		{"no match", "episode.mkv", false},
		{"empty string", "", false},
		{"partial match", "postcard.jpg", false}, // Does not contain full "poster" pattern
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsAny(tt.str, patterns)
			if result != tt.expected {
				t.Errorf("Expected %v for string %q", tt.expected, tt.str)
			}
		})
	}
}

func TestContainsAnyEmptyList(t *testing.T) {
	result := containsAny("test string", []string{})
	if result {
		t.Error("Expected false for empty substring list")
	}
}

func TestOpenFile(t *testing.T) {
	// Create a temporary file
	tmpFile := t.TempDir() + "/test.txt"
	content := []byte("test content")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Test successful open
	file, cleanup, err := openFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	if file == nil {
		t.Fatal("Expected non-nil file")
	}
	if cleanup == nil {
		t.Fatal("Expected non-nil cleanup function")
	}

	// Verify we can read from the file
	buf := make([]byte, 100)
	n, _ := file.Read(buf)
	if string(buf[:n]) != string(content) {
		t.Errorf("Expected content %q, got %q", content, buf[:n])
	}

	// Test cleanup function
	cleanup()

	// File should be closed after cleanup
	_, err = file.Read(buf)
	if err == nil {
		t.Error("Expected error reading from closed file")
	}
}

func TestOpenFileNonExistent(t *testing.T) {
	file, cleanup, err := openFile("/nonexistent/file.txt")

	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if file != nil {
		t.Error("Expected nil file for non-existent file")
	}
	if cleanup != nil {
		t.Error("Expected nil cleanup for failed open")
	}
}

func TestOpenFileWithDefer(t *testing.T) {
	tmpFile := t.TempDir() + "/test.txt"
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Test that cleanup works correctly with defer
	func() {
		file, cleanup, err := openFile(tmpFile)
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()

		// File should be open and readable here
		buf := make([]byte, 10)
		_, err = file.Read(buf)
		if err != nil {
			t.Errorf("Expected to read from file: %v", err)
		}
		// cleanup will be called automatically when this function returns
	}()

	// After the function returns, we can verify the file can be opened again
	// (which wouldn't work if there was a file lock issue)
	file, cleanup, err := openFile(tmpFile)
	if err != nil {
		t.Errorf("Failed to reopen file after cleanup: %v", err)
	}
	if cleanup != nil {
		defer cleanup()
	}
	if file != nil {
		file.Close()
	}
}
