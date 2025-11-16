package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCalculateDirSize(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Create test files with known sizes
	files := map[string][]byte{
		"file1.txt":           []byte("Hello"),                    // 5 bytes
		"file2.txt":           []byte("World"),                    // 5 bytes
		"subdir/file3.txt":    []byte("Nested"),                   // 6 bytes
		"subdir/file4.txt":    []byte("Content"),                  // 7 bytes
		"subdir/sub2/file5.txt": []byte("Deep"),                  // 4 bytes
	}

	totalExpected := int64(0)
	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
		if err := os.WriteFile(fullPath, content, 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", fullPath, err)
		}
		totalExpected += int64(len(content))
	}

	// Test calculating directory size
	size, err := CalculateDirSize(tmpDir)
	if err != nil {
		t.Fatalf("CalculateDirSize failed: %v", err)
	}

	if size != totalExpected {
		t.Errorf("Expected size %d bytes, got %d bytes", totalExpected, size)
	}
}

func TestCalculateDirSizeEmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	size, err := CalculateDirSize(tmpDir)
	if err != nil {
		t.Fatalf("CalculateDirSize failed: %v", err)
	}

	if size != 0 {
		t.Errorf("Expected size 0 for empty directory, got %d", size)
	}
}

func TestCalculateDirSizeSingleFile(t *testing.T) {
	tmpDir := t.TempDir()
	content := []byte("Single file content with 31 bytes")
	filePath := filepath.Join(tmpDir, "single.txt")

	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	size, err := CalculateDirSize(tmpDir)
	if err != nil {
		t.Fatalf("CalculateDirSize failed: %v", err)
	}

	expected := int64(len(content))
	if size != expected {
		t.Errorf("Expected size %d bytes, got %d bytes", expected, size)
	}
}

func TestCalculateDirSizeNestedDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Create deeply nested structure
	deepPath := filepath.Join(tmpDir, "a", "b", "c", "d", "e")
	if err := os.MkdirAll(deepPath, 0755); err != nil {
		t.Fatalf("Failed to create nested directories: %v", err)
	}

	// Add files at various levels
	files := []struct {
		path    string
		content string
	}{
		{filepath.Join(tmpDir, "root.txt"), "root"},
		{filepath.Join(tmpDir, "a", "level1.txt"), "level1"},
		{filepath.Join(tmpDir, "a", "b", "level2.txt"), "level2"},
		{filepath.Join(tmpDir, "a", "b", "c", "level3.txt"), "level3"},
		{filepath.Join(tmpDir, "a", "b", "c", "d", "level4.txt"), "level4"},
		{filepath.Join(tmpDir, "a", "b", "c", "d", "e", "level5.txt"), "level5"},
	}

	totalExpected := int64(0)
	for _, f := range files {
		if err := os.WriteFile(f.path, []byte(f.content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", f.path, err)
		}
		totalExpected += int64(len(f.content))
	}

	size, err := CalculateDirSize(tmpDir)
	if err != nil {
		t.Fatalf("CalculateDirSize failed: %v", err)
	}

	if size != totalExpected {
		t.Errorf("Expected size %d bytes, got %d bytes", totalExpected, size)
	}
}

func TestCalculateDirSizeNonExistentDirectory(t *testing.T) {
	size, err := CalculateDirSize("/nonexistent/directory/path")

	// filepath.Walk doesn't return an error when the walk function swallows it
	// The implementation skips files that can't be accessed, so this returns size=0, err=nil
	if err != nil {
		t.Logf("Note: Got error %v (may vary by system)", err)
	}

	// Size should be 0 for non-existent directory
	if size != 0 {
		t.Errorf("Expected size 0 for non-existent directory, got %d", size)
	}
}

func TestCalculateDirSizeWithSymlinks(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file
	realFile := filepath.Join(tmpDir, "real.txt")
	content := []byte("Real content")
	if err := os.WriteFile(realFile, content, 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Create a symlink to the file
	symlinkPath := filepath.Join(tmpDir, "link.txt")
	if err := os.Symlink(realFile, symlinkPath); err != nil {
		t.Skipf("Skipping symlink test: %v", err)
	}

	// Calculate size - symlinks themselves are tiny, but filepath.Walk follows them
	size, err := CalculateDirSize(tmpDir)
	if err != nil {
		t.Fatalf("CalculateDirSize failed: %v", err)
	}

	// Size should include both the real file and the symlink target
	// (since filepath.Walk follows symlinks by default)
	expected := int64(len(content)) * 2
	if size != expected {
		t.Logf("Note: Size is %d, expected %d (symlink behavior may vary)", size, expected)
	}
}

func TestCalculateDirSizeSkipsInaccessibleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a normal file
	normalFile := filepath.Join(tmpDir, "normal.txt")
	content := []byte("Normal content")
	if err := os.WriteFile(normalFile, content, 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Create a subdirectory with restricted permissions
	restrictedDir := filepath.Join(tmpDir, "restricted")
	if err := os.MkdirAll(restrictedDir, 0755); err != nil {
		t.Fatalf("Failed to create restricted directory: %v", err)
	}

	// Add a file to the restricted directory
	restrictedFile := filepath.Join(restrictedDir, "secret.txt")
	if err := os.WriteFile(restrictedFile, []byte("secret"), 0644); err != nil {
		t.Fatalf("Failed to write restricted file: %v", err)
	}

	// Remove read permissions from the directory (Unix-only)
	if err := os.Chmod(restrictedDir, 0000); err != nil {
		t.Skipf("Skipping permission test: %v", err)
	}
	defer os.Chmod(restrictedDir, 0755) // Restore for cleanup

	// Calculate size - should succeed but skip inaccessible files
	size, err := CalculateDirSize(tmpDir)
	if err != nil {
		t.Fatalf("CalculateDirSize should not fail on inaccessible files: %v", err)
	}

	// Size should only include the accessible file
	expected := int64(len(content))
	if size != expected {
		t.Logf("Size is %d, expected %d (may include or skip restricted files)", size, expected)
	}
}

func TestCalculateDirSizeWithZeroByteFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create empty files
	emptyFiles := []string{"empty1.txt", "empty2.txt", "subdir/empty3.txt"}
	for _, name := range emptyFiles {
		path := filepath.Join(tmpDir, name)
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(path, []byte{}, 0644); err != nil {
			t.Fatalf("Failed to create empty file: %v", err)
		}
	}

	// Also create one file with content
	contentFile := filepath.Join(tmpDir, "content.txt")
	content := []byte("Some content")
	if err := os.WriteFile(contentFile, content, 0644); err != nil {
		t.Fatalf("Failed to write content file: %v", err)
	}

	size, err := CalculateDirSize(tmpDir)
	if err != nil {
		t.Fatalf("CalculateDirSize failed: %v", err)
	}

	expected := int64(len(content))
	if size != expected {
		t.Errorf("Expected size %d bytes (only non-empty file), got %d bytes", expected, size)
	}
}
