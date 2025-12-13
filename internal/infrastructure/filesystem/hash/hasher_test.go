package hash

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

func TestHasher_Hash_EmptyFile(t *testing.T) {
	hasher := NewHasher()
	tmpDir := t.TempDir()

	// Create an empty file
	emptyFile := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(emptyFile, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	// Empty files should hash successfully
	hash, err := hasher.Hash(emptyFile)
	if err != nil {
		t.Fatalf("Failed to hash empty file: %v", err)
	}

	// Hash should not be empty (empty file still produces a hash)
	if hash == "" {
		t.Error("Hash of empty file should not be empty string")
	}

	// Verify consistency
	hash2, err := hasher.Hash(emptyFile)
	if err != nil {
		t.Fatalf("Failed to hash empty file second time: %v", err)
	}
	if hash != hash2 {
		t.Error("Hash of empty file should be consistent")
	}
}

func TestHasher_Hash_ExactlyTwoChunks(t *testing.T) {
	hasher := NewHasher()
	tmpDir := t.TempDir()

	// Create a file exactly 2 * chunkSize (128KB)
	// This tests the boundary condition where file size == chunkSize*2
	exactFile := filepath.Join(tmpDir, "exact.bin")
	content := make([]byte, ChunkSize*2)
	for i := range content {
		content[i] = byte(i % 256)
	}
	if err := os.WriteFile(exactFile, content, 0644); err != nil {
		t.Fatalf("Failed to create exact file: %v", err)
	}

	hash, err := hasher.Hash(exactFile)
	if err != nil {
		t.Fatalf("Failed to hash exact-size file: %v", err)
	}

	if hash == "" {
		t.Error("Hash should not be empty")
	}

	// Verify consistency
	hash2, err := hasher.Hash(exactFile)
	if err != nil {
		t.Fatalf("Failed to hash file second time: %v", err)
	}
	if hash != hash2 {
		t.Error("Hash should be consistent")
	}
}

func TestHasher_Hash_JustUnderTwoChunks(t *testing.T) {
	hasher := NewHasher()
	tmpDir := t.TempDir()

	// Create a file just under 2 * chunkSize (127KB)
	// This should trigger hashEntireFile path
	smallFile := filepath.Join(tmpDir, "small.bin")
	content := make([]byte, ChunkSize*2-1)
	for i := range content {
		content[i] = byte(i % 256)
	}
	if err := os.WriteFile(smallFile, content, 0644); err != nil {
		t.Fatalf("Failed to create small file: %v", err)
	}

	hash, err := hasher.Hash(smallFile)
	if err != nil {
		t.Fatalf("Failed to hash small file: %v", err)
	}

	if hash == "" {
		t.Error("Hash should not be empty")
	}
}

func TestHasher_Hash_JustOverTwoChunks(t *testing.T) {
	hasher := NewHasher()
	tmpDir := t.TempDir()

	// Create a file just over 2 * chunkSize (129KB)
	// This tests the partial hash path with minimal overlap
	largeFile := filepath.Join(tmpDir, "large.bin")
	content := make([]byte, ChunkSize*2+1)
	for i := range content {
		content[i] = byte(i % 256)
	}
	if err := os.WriteFile(largeFile, content, 0644); err != nil {
		t.Fatalf("Failed to create large file: %v", err)
	}

	hash, err := hasher.Hash(largeFile)
	if err != nil {
		t.Fatalf("Failed to hash large file: %v", err)
	}

	if hash == "" {
		t.Error("Hash should not be empty")
	}
}

func TestHasher_Hash_SameSizeDifferentContent(t *testing.T) {
	hasher := NewHasher()
	tmpDir := t.TempDir()

	// Create two files with same size but different content
	// This verifies the hash distinguishes files properly
	size := ChunkSize * 3 // Large enough to use partial hashing

	file1 := filepath.Join(tmpDir, "file1.bin")
	content1 := make([]byte, size)
	for i := range content1 {
		content1[i] = byte(i % 256)
	}
	if err := os.WriteFile(file1, content1, 0644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}

	file2 := filepath.Join(tmpDir, "file2.bin")
	content2 := make([]byte, size)
	for i := range content2 {
		content2[i] = byte((i + 1) % 256) // Different pattern
	}
	if err := os.WriteFile(file2, content2, 0644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	hash1, err := hasher.Hash(file1)
	if err != nil {
		t.Fatalf("Failed to hash file1: %v", err)
	}

	hash2, err := hasher.Hash(file2)
	if err != nil {
		t.Fatalf("Failed to hash file2: %v", err)
	}

	if hash1 == hash2 {
		t.Error("Files with different content should have different hashes")
	}
}

func TestHasher_HashFormat(t *testing.T) {
	hasher := NewHasher()
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	hash, err := hasher.Hash(testFile)
	if err != nil {
		t.Fatalf("Failed to hash file: %v", err)
	}

	// Hash should be 32 hex characters (128-bit XXH3 = 16 bytes = 32 hex chars)
	if len(hash) != 32 {
		t.Errorf("Expected hash length of 32, got %d", len(hash))
	}

	// Hash should only contain valid hex characters
	for _, c := range hash {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("Hash contains invalid character: %c", c)
		}
	}
}
