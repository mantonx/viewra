package images

import (
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/zeebo/xxh3"
)

// CacheService handles copying images to cache directory and managing cached files
type CacheService struct {
	cacheDir string
}

// NewCacheService creates a new cache service
func NewCacheService(cacheDir string) *CacheService {
	return &CacheService{
		cacheDir: cacheDir,
	}
}

// EnsureCacheDir creates the cache directory if it doesn't exist
func (s *CacheService) EnsureCacheDir() error {
	return os.MkdirAll(s.cacheDir, 0755)
}

// CopyToCache copies an image file to the cache directory using its hash as the filename
// Returns the cache path relative to the cache directory
// Format: {first2chars}/{next2chars}/{hash}_original.{ext}
// Example: 04/c3/04c38ff5542c1d931ab7d0830d314a2a03acf1c11b01d52e9deec7a31f0bdcf5_original.jpg
func (s *CacheService) CopyToCache(sourcePath, fileHash string) (string, error) {
	// Ensure cache directory exists
	if err := s.EnsureCacheDir(); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Get file extension
	ext := filepath.Ext(sourcePath)
	if ext == "" {
		ext = ".jpg" // Default to jpg if no extension
	}

	// Build sharded path: first2/next2/filename
	// This prevents filesystem performance issues with large directories
	if len(fileHash) < 4 {
		return "", fmt.Errorf("file hash too short for sharding: %s", fileHash)
	}

	level1 := fileHash[0:2]
	level2 := fileHash[2:4]
	filename := fmt.Sprintf("%s_original%s", fileHash, ext)

	// Relative path for database storage
	relativePath := filepath.Join(level1, level2, filename)

	// Full path for filesystem operations
	fullPath := filepath.Join(s.cacheDir, relativePath)

	// Check if file already exists in cache (deduplication)
	if _, err := os.Stat(fullPath); err == nil {
		// File already cached, return relative path
		return relativePath, nil
	}

	// Create sharded subdirectories
	shardDir := filepath.Join(s.cacheDir, level1, level2)
	if err := os.MkdirAll(shardDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create shard directory: %w", err)
	}

	// Open source file
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return "", fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	// Create cache file
	cacheFile, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to create cache file: %w", err)
	}
	defer cacheFile.Close()

	// Copy file
	if _, err := io.Copy(cacheFile, sourceFile); err != nil {
		// Clean up partial file on error
		os.Remove(fullPath)
		return "", fmt.Errorf("failed to copy file to cache: %w", err)
	}

	// Return relative path for database storage
	return relativePath, nil
}

// GetCachedPath returns the full path to a cached file given its relative cache path
func (s *CacheService) GetCachedPath(relativePath string) string {
	return filepath.Join(s.cacheDir, relativePath)
}

// GetPresetPath returns the full path to a preset image file.
// Format: {cacheDir}/{first2}/{next2}/{hash}_{imageType}_{preset}.webp
// Returns empty string and error if hash is too short.
func (s *CacheService) GetPresetPath(hash, imageType, preset string) (string, error) {
	if len(hash) < 4 {
		return "", fmt.Errorf("hash too short: %s", hash)
	}

	level1 := hash[0:2]
	level2 := hash[2:4]
	filename := fmt.Sprintf("%s_%s_%s.webp", hash, imageType, preset)
	relativePath := filepath.Join(level1, level2, filename)

	return filepath.Join(s.cacheDir, relativePath), nil
}

// FileExists checks if a cached file exists
func (s *CacheService) FileExists(relativePath string) bool {
	fullPath := s.GetCachedPath(relativePath)
	_, err := os.Stat(fullPath)
	return err == nil
}

// ComputeFileHash computes XXH3-128 hash of a file (50x faster than SHA256, zero collision risk)
// This is a utility function in case we need to recompute hashes
func (s *CacheService) ComputeFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := xxh3.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to compute hash: %w", err)
	}

	// Use Sum128() for 128-bit hash instead of Sum64()
	hash128 := hash.Sum128()
	hashBytes := hash128.Bytes()
	return hex.EncodeToString(hashBytes[:]), nil
}

// DeleteCachedFile removes a file from the cache
func (s *CacheService) DeleteCachedFile(relativePath string) error {
	fullPath := s.GetCachedPath(relativePath)
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete cached file: %w", err)
	}
	return nil
}

// ListCachedFiles returns all files in the cache directory
func (s *CacheService) ListCachedFiles() ([]string, error) {
	entries, err := os.ReadDir(s.cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read cache directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}

	return files, nil
}

// GetCacheSize returns the total size of all cached files in bytes
func (s *CacheService) GetCacheSize() (int64, error) {
	var totalSize int64

	err := filepath.Walk(s.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("failed to calculate cache size: %w", err)
	}

	return totalSize, nil
}
