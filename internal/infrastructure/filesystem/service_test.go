package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultFileSystem_Stat(t *testing.T) {
	dfs := &DefaultFileSystem{}
	tmpDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test Stat on existing file
	info, err := dfs.Stat(testFile)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if info.Name() != "test.txt" {
		t.Errorf("Expected name 'test.txt', got '%s'", info.Name())
	}
	if info.Size() != 4 {
		t.Errorf("Expected size 4, got %d", info.Size())
	}

	// Test Stat on non-existent file
	_, err = dfs.Stat("/nonexistent/file.txt")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestDefaultFileSystem_ReadDir(t *testing.T) {
	dfs := &DefaultFileSystem{}
	tmpDir := t.TempDir()

	// Create test files
	files := []string{"file1.txt", "file2.txt", "file3.txt"}
	for _, f := range files {
		path := filepath.Join(tmpDir, f)
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	// Create a subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	// Test ReadDir
	entries, err := dfs.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	if len(entries) != 4 { // 3 files + 1 directory
		t.Errorf("Expected 4 entries, got %d", len(entries))
	}

	// Test ReadDir on non-existent directory
	_, err = dfs.ReadDir("/nonexistent/directory")
	if err == nil {
		t.Error("Expected error for non-existent directory")
	}
}
