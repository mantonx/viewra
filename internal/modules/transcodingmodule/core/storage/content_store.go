// Package storage provides content-addressable storage for transcoded media files.
// It manages deduplication, metadata tracking, and efficient file retrieval.
package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
)

// ContentStore manages content-addressable storage for transcoded media files.
// It provides deduplication, metadata tracking, and efficient file retrieval.
type ContentStore struct {
	baseDir  string
	logger   hclog.Logger
	metadata *MetadataStore
	mu       sync.RWMutex
}

// ContentMetadata represents metadata for stored content
type ContentMetadata struct {
	Hash          string            `json:"hash"`
	MediaID       string            `json:"media_id"`
	CreatedAt     time.Time         `json:"created_at"`
	LastAccessed  time.Time         `json:"last_accessed"`
	Size          int64             `json:"size"`
	Format        string            `json:"format"` // "mp4", "mkv", etc.
	AccessCount   int64             `json:"access_count"`
	RetentionDays int               `json:"retention_days"`
	Tags          map[string]string `json:"tags"`
	
	// Transcoding parameters that generated this content
	VideoCodec   string `json:"video_codec,omitempty"`
	AudioCodec   string `json:"audio_codec,omitempty"`
	VideoBitrate int    `json:"video_bitrate,omitempty"`
	AudioBitrate int    `json:"audio_bitrate,omitempty"`
	Resolution   string `json:"resolution,omitempty"`
}

// NewContentStore creates a new content-addressable storage manager
func NewContentStore(baseDir string, logger hclog.Logger) (*ContentStore, error) {
	// Ensure base directory exists
	contentDir := filepath.Join(baseDir, "content")
	if err := os.MkdirAll(contentDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create content directory: %w", err)
	}

	// Create metadata store
	metadataStore, err := NewMetadataStore(filepath.Join(baseDir, "metadata"), logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create metadata store: %w", err)
	}

	return &ContentStore{
		baseDir:  baseDir,
		logger:   logger,
		metadata: metadataStore,
	}, nil
}

// Store saves content with the given hash and metadata.
// It moves files from the source directory to content-addressable storage.
func (cs *ContentStore) Store(contentHash string, sourceDir string, metadata ContentMetadata) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Create content directory with sharding (first 2 chars of hash)
	contentPath := cs.getContentPath(contentHash)
	contentDir := filepath.Dir(contentPath)

	if err := os.MkdirAll(contentDir, 0755); err != nil {
		return fmt.Errorf("failed to create content directory: %w", err)
	}

	// Check if content already exists
	if _, err := os.Stat(contentPath); err == nil {
		cs.logger.Info("content already exists", "hash", contentHash)
		return nil
	}

	// Move files from source directory
	if err := os.Rename(sourceDir, contentPath); err != nil {
		// If rename fails (cross-device), fall back to copy
		if err := cs.copyDirectory(sourceDir, contentPath); err != nil {
			return fmt.Errorf("failed to move content: %w", err)
		}
		// Remove source after successful copy
		os.RemoveAll(sourceDir)
	}

	// Calculate actual size
	size, err := cs.calculateSize(contentPath)
	if err != nil {
		cs.logger.Warn("failed to calculate content size", "error", err)
		size = 0
	}

	// Update metadata
	metadata.Hash = contentHash
	metadata.CreatedAt = time.Now()
	metadata.LastAccessed = time.Now()
	metadata.Size = size
	metadata.AccessCount = 0

	// Save metadata
	if err := cs.metadata.Save(contentHash, metadata); err != nil {
		// Rollback: remove content
		os.RemoveAll(contentPath)
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	cs.logger.Info("stored content",
		"hash", contentHash,
		"size", size,
		"format", metadata.Format,
		"mediaID", metadata.MediaID,
	)

	return nil
}

// Get retrieves content metadata and path by hash
func (cs *ContentStore) Get(contentHash string) (interface{}, string, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	// Load metadata
	metadata, err := cs.metadata.Load(contentHash)
	if err != nil {
		return nil, "", fmt.Errorf("content not found: %w", err)
	}

	// Check if content exists
	contentPath := cs.getContentPath(contentHash)
	if _, err := os.Stat(contentPath); err != nil {
		return nil, "", fmt.Errorf("content files missing: %w", err)
	}

	// Update access time and count asynchronously
	go func() {
		metadata.LastAccessed = time.Now()
		metadata.AccessCount++
		cs.metadata.Save(contentHash, *metadata)
	}()

	return metadata, contentPath, nil
}

// Exists checks if content exists
func (cs *ContentStore) Exists(contentHash string) bool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	contentPath := cs.getContentPath(contentHash)
	_, err := os.Stat(contentPath)
	return err == nil
}

// Delete removes content and its metadata
func (cs *ContentStore) Delete(contentHash string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Remove content directory
	contentPath := cs.getContentPath(contentHash)
	if err := os.RemoveAll(contentPath); err != nil {
		return fmt.Errorf("failed to remove content: %w", err)
	}

	// Remove metadata
	if err := cs.metadata.Delete(contentHash); err != nil {
		cs.logger.Warn("failed to delete metadata", "hash", contentHash, "error", err)
	}

	cs.logger.Info("deleted content", "hash", contentHash)
	return nil
}

// ListExpired returns content that has exceeded its retention period
func (cs *ContentStore) ListExpired() ([]ContentMetadata, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	allMetadata, err := cs.metadata.ListAll()
	if err != nil {
		return nil, err
	}

	var expired []ContentMetadata
	now := time.Now()

	for _, meta := range allMetadata {
		retentionDuration := time.Duration(meta.RetentionDays) * 24 * time.Hour
		if retentionDuration > 0 && now.Sub(meta.LastAccessed) > retentionDuration {
			expired = append(expired, meta)
		}
	}

	return expired, nil
}

// ListByMediaID returns all content versions for a media ID
func (cs *ContentStore) ListByMediaID(mediaID string) ([]ContentMetadata, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.metadata.ListByMediaID(mediaID)
}

// GetStats returns storage statistics
func (cs *ContentStore) GetStats() (*ContentStoreStats, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	allMetadata, err := cs.metadata.ListAll()
	if err != nil {
		return nil, err
	}

	stats := &ContentStoreStats{
		TotalCount:   len(allMetadata),
		TotalSize:    0,
		ByFormat:     make(map[string]int),
		ByMediaID:    make(map[string]int),
		OldestAccess: time.Now(),
		NewestAccess: time.Time{},
	}

	for _, meta := range allMetadata {
		stats.TotalSize += meta.Size
		stats.ByFormat[meta.Format]++
		stats.ByMediaID[meta.MediaID]++

		if meta.LastAccessed.Before(stats.OldestAccess) {
			stats.OldestAccess = meta.LastAccessed
		}
		if meta.LastAccessed.After(stats.NewestAccess) {
			stats.NewestAccess = meta.LastAccessed
		}
	}

	return stats, nil
}

// getContentPath returns the filesystem path for content
func (cs *ContentStore) getContentPath(contentHash string) string {
	// Use first 2 characters for sharding to avoid too many files in one directory
	return filepath.Join(cs.baseDir, "content", contentHash[:2], contentHash)
}

// copyDirectory copies a directory recursively
func (cs *ContentStore) copyDirectory(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		// Copy file
		return cs.copyFile(path, dstPath)
	})
}

// copyFile copies a single file
func (cs *ContentStore) copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = dstFile.ReadFrom(srcFile)
	return err
}

// calculateSize calculates the total size of a directory
func (cs *ContentStore) calculateSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// ContentStoreStats represents statistics about the content store
type ContentStoreStats struct {
	TotalCount   int            `json:"total_count"`
	TotalSize    int64          `json:"total_size"`
	ByFormat     map[string]int `json:"by_format"`
	ByMediaID    map[string]int `json:"by_media_id"`
	OldestAccess time.Time      `json:"oldest_access"`
	NewestAccess time.Time      `json:"newest_access"`
}

// MetadataStore manages metadata persistence
type MetadataStore struct {
	baseDir string
	logger  hclog.Logger
	mu      sync.RWMutex
}

// NewMetadataStore creates a new metadata store
func NewMetadataStore(baseDir string, logger hclog.Logger) (*MetadataStore, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create metadata directory: %w", err)
	}

	return &MetadataStore{
		baseDir: baseDir,
		logger:  logger,
	}, nil
}

// Save persists metadata to disk
func (ms *MetadataStore) Save(contentHash string, metadata ContentMetadata) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	metaPath := ms.getMetadataPath(contentHash)
	metaDir := filepath.Dir(metaPath)

	if err := os.MkdirAll(metaDir, 0755); err != nil {
		return fmt.Errorf("failed to create metadata directory: %w", err)
	}

	return os.WriteFile(metaPath, data, 0644)
}

// Load retrieves metadata from disk
func (ms *MetadataStore) Load(contentHash string) (*ContentMetadata, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	metaPath := ms.getMetadataPath(contentHash)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}

	var metadata ContentMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &metadata, nil
}

// Delete removes metadata from disk
func (ms *MetadataStore) Delete(contentHash string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	metaPath := ms.getMetadataPath(contentHash)
	return os.Remove(metaPath)
}

// ListAll returns all metadata entries
func (ms *MetadataStore) ListAll() ([]ContentMetadata, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	var allMetadata []ContentMetadata

	err := filepath.Walk(ms.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			ms.logger.Warn("failed to read metadata file", "path", path, "error", err)
			return nil
		}

		var metadata ContentMetadata
		if err := json.Unmarshal(data, &metadata); err != nil {
			ms.logger.Warn("failed to unmarshal metadata", "path", path, "error", err)
			return nil
		}

		allMetadata = append(allMetadata, metadata)
		return nil
	})

	return allMetadata, err
}

// ListByMediaID returns metadata for a specific media ID
func (ms *MetadataStore) ListByMediaID(mediaID string) ([]ContentMetadata, error) {
	allMetadata, err := ms.ListAll()
	if err != nil {
		return nil, err
	}

	var filtered []ContentMetadata
	for _, meta := range allMetadata {
		if meta.MediaID == mediaID {
			filtered = append(filtered, meta)
		}
	}

	return filtered, nil
}

// getMetadataPath returns the filesystem path for metadata
func (ms *MetadataStore) getMetadataPath(contentHash string) string {
	return filepath.Join(ms.baseDir, contentHash[:2], contentHash+".json")
}

