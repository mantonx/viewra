package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHasher_Hash(t *testing.T) {
	hasher := NewHasher()

	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("This is a test file for hashing")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test hashing
	hash1, err := hasher.Hash(testFile)
	if err != nil {
		t.Fatalf("Failed to hash file: %v", err)
	}

	if hash1 == "" {
		t.Error("Hash should not be empty")
	}

	// Hash the same file again - should get the same result
	hash2, err := hasher.Hash(testFile)
	if err != nil {
		t.Fatalf("Failed to hash file second time: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("Hash mismatch: %s != %s", hash1, hash2)
	}
}

func TestHasher_Hash_LargeFile(t *testing.T) {
	hasher := NewHasher()

	// Create a large test file (200KB)
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large.bin")
	content := make([]byte, 200*1024) // 200KB
	for i := range content {
		content[i] = byte(i % 256)
	}
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test hashing
	hash, err := hasher.Hash(testFile)
	if err != nil {
		t.Fatalf("Failed to hash file: %v", err)
	}

	if hash == "" {
		t.Error("Hash should not be empty")
	}

	// Verify hash is consistent
	hash2, err := hasher.Hash(testFile)
	if err != nil {
		t.Fatalf("Failed to hash file second time: %v", err)
	}

	if hash != hash2 {
		t.Errorf("Hash mismatch for large file")
	}
}

func TestHasher_Hash_DifferentFiles(t *testing.T) {
	hasher := NewHasher()
	tmpDir := t.TempDir()

	// Create two different files
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	if err := os.WriteFile(file1, []byte("content A"), 0644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("content B"), 0644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	// Hash both files
	hash1, err := hasher.Hash(file1)
	if err != nil {
		t.Fatalf("Failed to hash file1: %v", err)
	}

	hash2, err := hasher.Hash(file2)
	if err != nil {
		t.Fatalf("Failed to hash file2: %v", err)
	}

	// Hashes should be different
	if hash1 == hash2 {
		t.Error("Different files should have different hashes")
	}
}

func TestHasher_Hash_NonExistentFile(t *testing.T) {
	hasher := NewHasher()

	// Try to hash a file that doesn't exist
	_, err := hasher.Hash("/nonexistent/file.txt")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}
