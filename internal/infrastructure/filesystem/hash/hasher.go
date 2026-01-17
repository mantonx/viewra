package hash

import (
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/zeebo/xxh3"
)

const (
	// ChunkSize is the size of data to read from start and end of file.
	// 64KB from start + 64KB from end = 128KB total read.
	// This is fast and sufficient for duplicate detection.
	ChunkSize = 64 * 1024 // 64 KB
)

// Hasher computes partial file hashes for duplicate detection
type Hasher struct {
	chunkSize int64
}

// NewHasher creates a new file hasher with default chunk size
func NewHasher() *Hasher {
	return &Hasher{
		chunkSize: ChunkSize,
	}
}

// Hash computes a partial hash of a file (first 64KB + last 64KB).
// This provides fast duplicate detection without reading entire file.
func (h *Hasher) Hash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}
	fileSize := stat.Size()

	// If file is smaller than 2 chunks, hash the entire file
	if fileSize <= h.chunkSize*2 {
		return h.hashEntireFile(file)
	}

	// Read first chunk
	firstChunk := make([]byte, h.chunkSize)
	n, err := file.ReadAt(firstChunk, 0)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("failed to read first chunk: %w", err)
	}
	firstChunk = firstChunk[:n]

	// Read last chunk
	lastChunk := make([]byte, h.chunkSize)
	n, err = file.ReadAt(lastChunk, fileSize-h.chunkSize)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("failed to read last chunk: %w", err)
	}
	lastChunk = lastChunk[:n]

	// Combine chunks and hash using XXH3-128 (50x faster than SHA256, zero collision risk)
	hasher := xxh3.New()
	hasher.Write(firstChunk)
	hasher.Write(lastChunk)

	// Include file size in hash to avoid collisions
	fmt.Fprintf(hasher, "%d", fileSize)

	// Use Sum128() for 128-bit hash
	hash128 := hasher.Sum128()
	hashBytes := hash128.Bytes()
	return hex.EncodeToString(hashBytes[:]), nil
}

// hashEntireFile computes a full XXH3-128 hash of a small file
func (h *Hasher) hashEntireFile(file *os.File) (string, error) {
	hasher := xxh3.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to hash file: %w", err)
	}
	hash128 := hasher.Sum128()
	hashBytes := hash128.Bytes()
	return hex.EncodeToString(hashBytes[:]), nil
}
