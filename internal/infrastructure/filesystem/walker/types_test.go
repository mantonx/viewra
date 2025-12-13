package walker

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestNewStats(t *testing.T) {
	stats := NewStats()

	if stats == nil {
		t.Fatal("NewStats returned nil")
	}

	if stats.FilesDiscovered != 0 {
		t.Errorf("FilesDiscovered should be 0, got %d", stats.FilesDiscovered)
	}
	if stats.DirsScanned != 0 {
		t.Errorf("DirsScanned should be 0, got %d", stats.DirsScanned)
	}
	if stats.DirsSkipped != 0 {
		t.Errorf("DirsSkipped should be 0, got %d", stats.DirsSkipped)
	}
	if stats.FilesSkipped != 0 {
		t.Errorf("FilesSkipped should be 0, got %d", stats.FilesSkipped)
	}
	if stats.PermissionErrors != 0 {
		t.Errorf("PermissionErrors should be 0, got %d", stats.PermissionErrors)
	}
	if stats.NetworkErrors != 0 {
		t.Errorf("NetworkErrors should be 0, got %d", stats.NetworkErrors)
	}
	if stats.OtherErrors != 0 {
		t.Errorf("OtherErrors should be 0, got %d", stats.OtherErrors)
	}
	if len(stats.SkippedPaths) != 0 {
		t.Errorf("SkippedPaths should be empty, got %d items", len(stats.SkippedPaths))
	}
	if stats.maxSkippedSamples != 100 {
		t.Errorf("maxSkippedSamples should be 100, got %d", stats.maxSkippedSamples)
	}
}

func TestToFileInfo(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.mkv")
	content := []byte("test content for video")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Read the directory to get a DirEntry
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read dir: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("No entries found")
	}

	// Test toFileInfo
	fileInfo, err := toFileInfo(testFile, entries[0])
	if err != nil {
		t.Fatalf("toFileInfo failed: %v", err)
	}

	if fileInfo.Path != testFile {
		t.Errorf("Expected path %s, got %s", testFile, fileInfo.Path)
	}
	if fileInfo.Size != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), fileInfo.Size)
	}
	if fileInfo.Extension != ".mkv" {
		t.Errorf("Expected extension .mkv, got %s", fileInfo.Extension)
	}
	if fileInfo.IsDir {
		t.Error("Expected IsDir to be false")
	}
	if fileInfo.ModTime.IsZero() {
		t.Error("ModTime should not be zero")
	}
}

func TestToFileInfo_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	// Read the parent directory to get a DirEntry for the subdir
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read dir: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("No entries found")
	}

	// Find the directory entry
	var dirEntry fs.DirEntry
	for _, e := range entries {
		if e.IsDir() {
			dirEntry = e
			break
		}
	}

	if dirEntry == nil {
		t.Fatal("Directory entry not found")
	}

	// Test toFileInfo for directory
	fileInfo, err := toFileInfo(subDir, dirEntry)
	if err != nil {
		t.Fatalf("toFileInfo failed: %v", err)
	}

	if !fileInfo.IsDir {
		t.Error("Expected IsDir to be true for directory")
	}
}

// mockErrorDirEntry simulates a DirEntry that returns an error on Info()
type mockErrorDirEntry struct {
	name string
}

func (m *mockErrorDirEntry) Name() string               { return m.name }
func (m *mockErrorDirEntry) IsDir() bool                { return false }
func (m *mockErrorDirEntry) Type() fs.FileMode          { return 0 }
func (m *mockErrorDirEntry) Info() (fs.FileInfo, error) { return nil, errors.New("info error") }

func TestToFileInfo_Error(t *testing.T) {
	entry := &mockErrorDirEntry{name: "test.txt"}

	_, err := toFileInfo("/path/test.txt", entry)
	if err == nil {
		t.Error("Expected error when Info() fails")
	}
}

func TestWalkerOptions(t *testing.T) {
	t.Run("WithParallelWalking", func(t *testing.T) {
		walker := New(WithParallelWalking(8))

		if !walker.enableParallel {
			t.Error("enableParallel should be true")
		}
		if walker.parallelWorkers != 8 {
			t.Errorf("parallelWorkers should be 8, got %d", walker.parallelWorkers)
		}
	})

	t.Run("WithProgressLogging", func(t *testing.T) {
		walker := New(WithProgressLogging(1000))

		if walker.progressInterval != 1000 {
			t.Errorf("progressInterval should be 1000, got %d", walker.progressInterval)
		}
	})

	t.Run("WithProgressCallback", func(t *testing.T) {
		called := false
		callback := func(filesDiscovered int64) {
			called = true
		}

		walker := New(WithProgressCallback(callback))

		if walker.progressCallback == nil {
			t.Error("progressCallback should not be nil")
		}

		// Call the callback to verify it was set correctly
		walker.progressCallback(10)
		if !called {
			t.Error("progressCallback was not called")
		}
	})

	t.Run("WithLogger", func(t *testing.T) {
		// Just verify it doesn't panic with a nil logger
		walker := New(WithLogger(nil))
		if walker == nil {
			t.Error("Walker should not be nil")
		}
	})

	t.Run("multiple options", func(t *testing.T) {
		callCount := 0
		callback := func(filesDiscovered int64) {
			callCount++
		}

		walker := New(
			WithParallelWalking(4),
			WithProgressLogging(500),
			WithProgressCallback(callback),
		)

		if !walker.enableParallel {
			t.Error("enableParallel should be true")
		}
		if walker.parallelWorkers != 4 {
			t.Errorf("parallelWorkers should be 4, got %d", walker.parallelWorkers)
		}
		if walker.progressInterval != 500 {
			t.Errorf("progressInterval should be 500, got %d", walker.progressInterval)
		}
		if walker.progressCallback == nil {
			t.Error("progressCallback should not be nil")
		}
	})
}

// TestCategorizeError tests the categorizeError function for all error types
// Note: categorizeError uses filepath.Base on error strings, which means:
// - For errors without path separators, the entire error string is used
// - For errors with path separators, only the last component is compared
// This tests actual behavior to ensure correct classification
func TestCategorizeError(t *testing.T) {
	t.Run("nil error does nothing", func(t *testing.T) {
		stats := NewStats()
		categorizeError(nil, stats)

		if stats.PermissionErrors != 0 || stats.NetworkErrors != 0 || stats.OtherErrors != 0 {
			t.Error("nil error should not modify stats")
		}
	})

	t.Run("permission denied via os.IsPermission", func(t *testing.T) {
		stats := NewStats()
		categorizeError(os.ErrPermission, stats)

		if stats.PermissionErrors != 1 {
			t.Errorf("Expected PermissionErrors=1, got %d", stats.PermissionErrors)
		}
		if stats.NetworkErrors != 0 || stats.OtherErrors != 0 {
			t.Error("Other error types should be 0")
		}
	})

	t.Run("permission denied exact string", func(t *testing.T) {
		stats := NewStats()
		// filepath.Base("permission denied") == "permission denied" (no path separator)
		err := errors.New("permission denied")
		categorizeError(err, stats)

		if stats.PermissionErrors != 1 {
			t.Errorf("Expected PermissionErrors=1, got %d", stats.PermissionErrors)
		}
	})

	t.Run("access denied exact string", func(t *testing.T) {
		stats := NewStats()
		// filepath.Base("access denied") == "access denied" (no path separator)
		err := errors.New("access denied")
		categorizeError(err, stats)

		if stats.PermissionErrors != 1 {
			t.Errorf("Expected PermissionErrors=1, got %d", stats.PermissionErrors)
		}
	})

	t.Run("no such host exact string", func(t *testing.T) {
		stats := NewStats()
		// filepath.Base("no such host") == "no such host" (no path separator)
		err := errors.New("no such host")
		categorizeError(err, stats)

		if stats.NetworkErrors != 1 {
			t.Errorf("Expected NetworkErrors=1, got %d", stats.NetworkErrors)
		}
	})

	t.Run("network unreachable exact string", func(t *testing.T) {
		stats := NewStats()
		err := errors.New("network is unreachable")
		categorizeError(err, stats)

		if stats.NetworkErrors != 1 {
			t.Errorf("Expected NetworkErrors=1, got %d", stats.NetworkErrors)
		}
	})

	t.Run("connection refused exact string", func(t *testing.T) {
		stats := NewStats()
		err := errors.New("connection refused")
		categorizeError(err, stats)

		if stats.NetworkErrors != 1 {
			t.Errorf("Expected NetworkErrors=1, got %d", stats.NetworkErrors)
		}
	})

	t.Run("connection reset exact string", func(t *testing.T) {
		stats := NewStats()
		err := errors.New("connection reset")
		categorizeError(err, stats)

		if stats.NetworkErrors != 1 {
			t.Errorf("Expected NetworkErrors=1, got %d", stats.NetworkErrors)
		}
	})

	t.Run("io timeout exact string", func(t *testing.T) {
		stats := NewStats()
		// i/o timeout is compared via direct string match due to slash
		err := errors.New("i/o timeout")
		categorizeError(err, stats)

		if stats.NetworkErrors != 1 {
			t.Errorf("Expected NetworkErrors=1, got %d", stats.NetworkErrors)
		}
	})

	t.Run("stale file handle exact string", func(t *testing.T) {
		stats := NewStats()
		err := errors.New("stale file handle")
		categorizeError(err, stats)

		if stats.NetworkErrors != 1 {
			t.Errorf("Expected NetworkErrors=1, got %d", stats.NetworkErrors)
		}
	})

	t.Run("prefixed error falls through to other", func(t *testing.T) {
		stats := NewStats()
		// Error strings with prefixes won't match the exact strings
		// since filepath.Base doesn't split on colons
		err := errors.New("dial tcp: no such host")
		categorizeError(err, stats)

		// This should fall through to OtherErrors because
		// filepath.Base("dial tcp: no such host") == "dial tcp: no such host"
		// which doesn't equal "no such host"
		if stats.OtherErrors != 1 {
			t.Errorf("Expected OtherErrors=1, got %d", stats.OtherErrors)
		}
	})

	t.Run("other error", func(t *testing.T) {
		stats := NewStats()
		err := errors.New("some random error")
		categorizeError(err, stats)

		if stats.OtherErrors != 1 {
			t.Errorf("Expected OtherErrors=1, got %d", stats.OtherErrors)
		}
		if stats.PermissionErrors != 0 || stats.NetworkErrors != 0 {
			t.Error("Permission and network errors should be 0")
		}
	})

	t.Run("multiple errors accumulate", func(t *testing.T) {
		stats := NewStats()
		categorizeError(os.ErrPermission, stats)           // PermissionErrors++
		categorizeError(errors.New("no such host"), stats) // NetworkErrors++
		categorizeError(errors.New("unknown error"), stats) // OtherErrors++
		categorizeError(errors.New("i/o timeout"), stats)   // NetworkErrors++

		if stats.PermissionErrors != 1 {
			t.Errorf("Expected PermissionErrors=1, got %d", stats.PermissionErrors)
		}
		if stats.NetworkErrors != 2 {
			t.Errorf("Expected NetworkErrors=2, got %d", stats.NetworkErrors)
		}
		if stats.OtherErrors != 1 {
			t.Errorf("Expected OtherErrors=1, got %d", stats.OtherErrors)
		}
	})
}

// Test WalkStats.TotalErrors method
func TestWalkStats_TotalErrors_Additional(t *testing.T) {
	t.Run("returns sum when all error types present", func(t *testing.T) {
		stats := NewStats()
		stats.PermissionErrors = 1
		stats.NetworkErrors = 2
		stats.OtherErrors = 3

		if stats.TotalErrors() != 6 {
			t.Errorf("TotalErrors() = %d, want 6", stats.TotalErrors())
		}
	})

	t.Run("returns zero when only permission errors", func(t *testing.T) {
		stats := NewStats()
		stats.PermissionErrors = 5

		if stats.TotalErrors() != 5 {
			t.Errorf("TotalErrors() = %d, want 5", stats.TotalErrors())
		}
	})

	t.Run("returns zero for new stats", func(t *testing.T) {
		stats := NewStats()

		if stats.TotalErrors() != 0 {
			t.Errorf("TotalErrors() = %d, want 0", stats.TotalErrors())
		}
	})
}
