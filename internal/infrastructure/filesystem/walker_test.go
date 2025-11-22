package filesystem

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/domain/scanner"
)

func TestWalker_Walk(t *testing.T) {
	t.Run("walks directory successfully", func(t *testing.T) {
		// Create temp directory structure
		tmpDir := t.TempDir()
		createTestFiles(t, tmpDir, []string{
			"movie1.mkv",
			"movie2.mp4",
			"subdir/movie3.avi",
			"subdir/nested/movie4.mkv",
		})

		walker := NewWalker()
		var files []string

		err := walker.Walk(context.Background(), tmpDir, func(info scanner.FileInfo) error {
			if !info.IsDir {
				files = append(files, filepath.Base(info.Path))
			}
			return nil
		})

		if err != nil {
			t.Fatalf("Walk() error = %v", err)
		}

		expected := []string{"movie1.mkv", "movie2.mp4", "movie3.avi", "movie4.mkv"}
		if len(files) != len(expected) {
			t.Errorf("Walk() found %d files, want %d", len(files), len(expected))
		}

		// Check all expected files were found
		fileMap := make(map[string]bool)
		for _, f := range files {
			fileMap[f] = true
		}
		for _, exp := range expected {
			if !fileMap[exp] {
				t.Errorf("Walk() missing file: %s", exp)
			}
		}
	})

	t.Run("calls walkFn for directories and files", func(t *testing.T) {
		tmpDir := t.TempDir()
		createTestFiles(t, tmpDir, []string{
			"file.mkv",
			"subdir/file2.mp4",
		})

		walker := NewWalker()
		var dirs, files int

		err := walker.Walk(context.Background(), tmpDir, func(info scanner.FileInfo) error {
			if info.IsDir {
				dirs++
			} else {
				files++
			}
			return nil
		})

		if err != nil {
			t.Fatalf("Walk() error = %v", err)
		}

		// Should have root + subdir = 2 dirs
		if dirs != 2 {
			t.Errorf("Walk() found %d directories, want 2", dirs)
		}

		// Should have 2 files
		if files != 2 {
			t.Errorf("Walk() found %d files, want 2", files)
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		tmpDir := t.TempDir()
		createTestFiles(t, tmpDir, []string{
			"file1.mkv",
			"file2.mkv",
			"file3.mkv",
			"file4.mkv",
			"file5.mkv",
		})

		walker := NewWalker()
		ctx, cancel := context.WithCancel(context.Background())

		var count int
		err := walker.Walk(ctx, tmpDir, func(info scanner.FileInfo) error {
			count++
			if count >= 3 {
				cancel() // Cancel after processing 3 items
			}
			return nil
		})

		// Should get context.Canceled error
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Walk() error = %v, want context.Canceled", err)
		}
	})

	t.Run("handles empty directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		walker := NewWalker()
		var count int

		err := walker.Walk(context.Background(), tmpDir, func(info scanner.FileInfo) error {
			count++
			return nil
		})

		if err != nil {
			t.Fatalf("Walk() error = %v", err)
		}

		// Should only see the root directory
		if count != 1 {
			t.Errorf("Walk() called walkFn %d times, want 1 (root dir only)", count)
		}
	})

	t.Run("returns ErrInvalidPath for empty root", func(t *testing.T) {
		walker := NewWalker()

		err := walker.Walk(context.Background(), "", func(info scanner.FileInfo) error {
			return nil
		})

		if !errors.Is(err, scanner.ErrInvalidPath) {
			t.Errorf("Walk() error = %v, want ErrInvalidPath", err)
		}
	})

	t.Run("handles walkFn errors", func(t *testing.T) {
		tmpDir := t.TempDir()
		createTestFiles(t, tmpDir, []string{
			"file1.mkv",
			"file2.mkv",
		})

		walker := NewWalker()
		expectedErr := errors.New("test error")

		err := walker.Walk(context.Background(), tmpDir, func(info scanner.FileInfo) error {
			if !info.IsDir {
				return expectedErr
			}
			return nil
		})

		if !errors.Is(err, expectedErr) {
			t.Errorf("Walk() error = %v, want %v", err, expectedErr)
		}
	})

	t.Run("continues on stat errors", func(t *testing.T) {
		// Create a custom walker with a mock walkDirFunc that simulates errors
		walker := &Walker{
			walkDirFunc: func(root string, fn fs.WalkDirFunc) error {
				// Simulate successful directory entry
				mockEntry := &mockDirEntry{
					name:  "file.mkv",
					isDir: false,
				}
				if err := fn("/test/file.mkv", mockEntry, nil); err != nil {
					return err
				}

				// Simulate an error for another entry
				errorEntry := &mockDirEntry{
					name:    "error.mkv",
					isDir:   false,
					statErr: errors.New("permission denied"),
				}
				// Error should be ignored and walk continues
				return fn("/test/error.mkv", errorEntry, nil)
			},
		}

		var processed []string
		err := walker.Walk(context.Background(), "/test", func(info scanner.FileInfo) error {
			processed = append(processed, filepath.Base(info.Path))
			return nil
		})

		if err != nil {
			t.Fatalf("Walk() error = %v, expected no error", err)
		}

		// Should only process the successful entry
		if len(processed) != 1 || processed[0] != "file.mkv" {
			t.Errorf("Walk() processed %v, want [file.mkv]", processed)
		}
	})

	t.Run("converts FileInfo correctly", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.mkv")

		// Create a file with known content
		content := []byte("test content")
		if err := os.WriteFile(testFile, content, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		walker := NewWalker()
		var capturedInfo scanner.FileInfo

		err := walker.Walk(context.Background(), tmpDir, func(info scanner.FileInfo) error {
			if !info.IsDir && filepath.Base(info.Path) == "test.mkv" {
				capturedInfo = info
			}
			return nil
		})

		if err != nil {
			t.Fatalf("Walk() error = %v", err)
		}

		// Verify FileInfo fields
		if capturedInfo.Path != testFile {
			t.Errorf("FileInfo.Path = %q, want %q", capturedInfo.Path, testFile)
		}
		if capturedInfo.Size != int64(len(content)) {
			t.Errorf("FileInfo.Size = %d, want %d", capturedInfo.Size, len(content))
		}
		if capturedInfo.Extension != ".mkv" {
			t.Errorf("FileInfo.Extension = %q, want .mkv", capturedInfo.Extension)
		}
		if capturedInfo.IsDir {
			t.Error("FileInfo.IsDir = true, want false")
		}
		if capturedInfo.ModTime.IsZero() {
			t.Error("FileInfo.ModTime is zero")
		}
	})

	t.Run("handles deeply nested directories", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a deeply nested structure
		deepPath := filepath.Join(tmpDir, "a", "b", "c", "d", "e", "f")
		if err := os.MkdirAll(deepPath, 0755); err != nil {
			t.Fatalf("Failed to create deep directory: %v", err)
		}

		testFile := filepath.Join(deepPath, "deep.mkv")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create deep file: %v", err)
		}

		walker := NewWalker()
		var foundDeep bool

		err := walker.Walk(context.Background(), tmpDir, func(info scanner.FileInfo) error {
			if !info.IsDir && filepath.Base(info.Path) == "deep.mkv" {
				foundDeep = true
			}
			return nil
		})

		if err != nil {
			t.Fatalf("Walk() error = %v", err)
		}

		if !foundDeep {
			t.Error("Walk() did not find deeply nested file")
		}
	})
}

func TestWalker_Walk_Integration(t *testing.T) {
	t.Run("realistic media library structure", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a realistic structure
		files := []string{
			// Movies
			"Movies/Inception (2010)/Inception (2010).mkv",
			"Movies/Inception (2010)/poster.jpg",
			"Movies/Inception (2010)/Inception (2010).nfo",

			// TV Shows
			"TV Shows/Breaking Bad/Season 01/Breaking Bad - S01E01.mkv",
			"TV Shows/Breaking Bad/Season 01/Breaking Bad - S01E02.mkv",
			"TV Shows/Breaking Bad/Season 01/Breaking Bad - S01E01.srt",
			"TV Shows/Breaking Bad/poster.jpg",

			// Music
			"Music/Artist/Album/01 - Track.mp3",
			"Music/Artist/Album/02 - Track.mp3",
			"Music/Artist/Album/cover.jpg",

			// System files
			".DS_Store",
			"Movies/.DS_Store",
			"Thumbs.db",
		}

		createTestFiles(t, tmpDir, files)

		walker := NewWalker()
		filter := NewFilter()

		var mediaFiles []string

		err := walker.Walk(context.Background(), tmpDir, func(info scanner.FileInfo) error {
			if info.IsDir {
				return nil
			}

			// Convert to os.FileInfo for filter
			osInfo, err := os.Stat(info.Path)
			if err != nil {
				return nil
			}

			if filter.ShouldProcess(info.Path, osInfo) {
				mediaFiles = append(mediaFiles, filepath.Base(info.Path))
			}

			return nil
		})

		if err != nil {
			t.Fatalf("Walk() error = %v", err)
		}

		// Should find 5 media files:
		// - Inception.mkv
		// - S01E01.mkv, S01E02.mkv
		// - 01 - Track.mp3, 02 - Track.mp3
		expected := 5
		if len(mediaFiles) != expected {
			t.Errorf("Found %d media files, want %d. Files: %v", len(mediaFiles), expected, mediaFiles)
		}
	})
}

// Helper functions

func createTestFiles(t *testing.T, root string, files []string) {
	t.Helper()
	for _, file := range files {
		fullPath := filepath.Join(root, file)
		dir := filepath.Dir(fullPath)

		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}

		if err := os.WriteFile(fullPath, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", fullPath, err)
		}
	}
}

// mockDirEntry implements fs.DirEntry for testing
type mockDirEntry struct {
	name    string
	isDir   bool
	statErr error
}

func (m *mockDirEntry) Name() string {
	return m.name
}

func (m *mockDirEntry) IsDir() bool {
	return m.isDir
}

func (m *mockDirEntry) Type() fs.FileMode {
	if m.isDir {
		return fs.ModeDir
	}
	return 0
}

func (m *mockDirEntry) Info() (fs.FileInfo, error) {
	if m.statErr != nil {
		return nil, m.statErr
	}
	return mockFileInfo{
		name:    m.name,
		isDir:   m.isDir,
		modTime: time.Now(),
	}, nil
}
