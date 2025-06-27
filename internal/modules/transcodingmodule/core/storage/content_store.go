// Package storage provides content-addressable storage for streaming media.
// This system enables content deduplication, efficient caching, and CDN-friendly
// URL structures optimized for segment-based streaming content.
package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/types"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/utils/hash"
)

// ContentStore manages content-addressable storage for transcoded media
type ContentStore struct {
	baseDir  string
	logger   hclog.Logger
	metadata *MetadataStore
	mu       sync.RWMutex
}

// ContentMetadata represents metadata for stored streaming content
type ContentMetadata struct {
	Hash          string                  `json:"hash"`
	MediaID       string                  `json:"media_id"`
	CreatedAt     time.Time               `json:"created_at"`
	LastAccessed  time.Time               `json:"last_accessed"`
	Size          int64                   `json:"size"`
	Format        string                  `json:"format"` // "dash", "hls"
	Profiles      []types.EncodingProfile `json:"profiles"`
	ManifestURL   string                  `json:"manifest_url"`
	AccessCount   int64                   `json:"access_count"`
	RetentionDays int                     `json:"retention_days"`
	Tags          map[string]string       `json:"tags"`

	// Streaming-specific metadata
	SegmentCount    int           `json:"segment_count"`
	SegmentDuration int           `json:"segment_duration"`
	TotalDuration   time.Duration `json:"total_duration"`
	QualityLevels   []string      `json:"quality_levels"`
	LastSegmentTime time.Time     `json:"last_segment_time"`
	StreamingStatus string        `json:"streaming_status"` // "active", "completed", "failed"
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

// Store saves content with the given hash and metadata
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
		cs.logger.Info("Content already exists", "hash", contentHash)
		return nil
	}

	// Move streaming content from source directory (segments, manifests, init files)
	if err := cs.moveStreamingContent(sourceDir, contentPath); err != nil {
		return fmt.Errorf("failed to move streaming content: %w", err)
	}

	// Calculate actual size
	size, err := cs.calculateSize(contentPath)
	if err != nil {
		cs.logger.Warn("Failed to calculate content size", "error", err)
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

	cs.logger.Info("Stored content",
		"hash", contentHash,
		"size", size,
		"format", metadata.Format,
		"mediaID", metadata.MediaID,
	)

	return nil
}

// Get retrieves content by hash
func (cs *ContentStore) Get(contentHash string) (interface{}, string, error) {
	return cs.GetMetadata(contentHash)
}

// GetMetadata retrieves content metadata by hash
func (cs *ContentStore) GetMetadata(contentHash string) (*ContentMetadata, string, error) {
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

	// Update access time and count
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
		cs.logger.Warn("Failed to delete metadata", "hash", contentHash, "error", err)
	}

	cs.logger.Info("Deleted content", "hash", contentHash)
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

// GenerateContentHash creates a deterministic hash for content based on parameters
// This uses the standardized hash utility to ensure consistency
func (cs *ContentStore) GenerateContentHash(mediaID string, profiles []types.EncodingProfile, formats []types.StreamingFormat) string {
	// Use the first profile for hash generation (most common case)
	// For multiple profiles, we could extend the hash utility to support arrays
	var resolution *hash.Resolution
	quality := 65       // Default quality based on bitrate (medium quality)
	container := "dash" // Default container

	if len(profiles) > 0 {
		profile := profiles[0]
		// Derive quality from video bitrate
		// High quality: > 5000 kbps = 80
		// Medium quality: 2000-5000 kbps = 65
		// Low quality: < 2000 kbps = 50
		if profile.VideoBitrate > 5000 {
			quality = 80
		} else if profile.VideoBitrate > 2000 {
			quality = 65
		} else {
			quality = 50
		}

		if profile.Width > 0 && profile.Height > 0 {
			resolution = &hash.Resolution{
				Width:  profile.Width,
				Height: profile.Height,
			}
		}
	}

	if len(formats) > 0 {
		container = string(formats[0])
	}

	// Use the standardized hash utility
	return hash.GenerateContentHash(mediaID, container, quality, resolution)
}

// getContentPath returns the filesystem path for content
func (cs *ContentStore) getContentPath(contentHash string) string {
	// Use first 2 characters for sharding to avoid too many files in one directory
	return filepath.Join(cs.baseDir, "content", contentHash[:2], contentHash)
}

// moveStreamingContent moves streaming content with optimized structure
func (cs *ContentStore) moveStreamingContent(sourceDir, destDir string) error {
	// Create destination directory with streaming structure
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	// Create streaming subdirectories
	streamingDirs := []string{"segments", "manifests", "init", "video", "audio"}
	for _, dir := range streamingDirs {
		if err := os.MkdirAll(filepath.Join(destDir, dir), 0755); err != nil {
			return err
		}
	}

	// Walk source directory and move files
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Calculate relative path
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		// Organize into streaming structure
		destPath := cs.organizeStreamingFile(destDir, relPath, info.Name())
		destSubDir := filepath.Dir(destPath)

		// Ensure destination subdirectory exists
		if err := os.MkdirAll(destSubDir, 0755); err != nil {
			return err
		}

		// Move file
		return os.Rename(path, destPath)
	})
}

// organizeStreamingFile determines the correct location for a streaming file
func (cs *ContentStore) organizeStreamingFile(baseDir, relPath, filename string) string {
	// Manifest files
	if strings.HasSuffix(filename, ".mpd") || strings.HasSuffix(filename, ".m3u8") {
		return filepath.Join(baseDir, "manifests", filename)
	}

	// Initialization segments
	if strings.Contains(filename, "init") {
		return filepath.Join(baseDir, "init", filename)
	}

	// Video segments
	if strings.Contains(filename, "video") || strings.HasSuffix(filename, ".m4s") {
		return filepath.Join(baseDir, "video", filename)
	}

	// Audio segments
	if strings.Contains(filename, "audio") || strings.HasSuffix(filename, ".ts") {
		return filepath.Join(baseDir, "audio", filename)
	}

	// Default to segments directory
	return filepath.Join(baseDir, "segments", filename)
}

// AddSegment adds a new segment to existing streaming content
func (cs *ContentStore) AddSegment(contentHash string, segmentPath string, segmentInfo interface{}) error {
	// Convert interface to SegmentInfo
	var info SegmentInfo
	if si, ok := segmentInfo.(SegmentInfo); ok {
		info = si
	} else if siMap, ok := segmentInfo.(map[string]interface{}); ok {
		// Handle map format from events
		if index, ok := siMap["index"].(int); ok {
			info.Index = index
		}
		if profile, ok := siMap["profile"].(string); ok {
			info.Profile = profile
		}
		if segType, ok := siMap["type"].(string); ok {
			info.Type = segType
		}
		if duration, ok := siMap["duration"].(int); ok {
			info.Duration = duration
		}
	} else {
		// Default values
		info = SegmentInfo{
			Index:    1,
			Profile:  "default",
			Type:     "video",
			Duration: 4,
		}
	}
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Get existing metadata
	metadata, err := cs.metadata.Load(contentHash)
	if err != nil {
		return fmt.Errorf("content not found: %w", err)
	}

	// Get content path
	contentPath := cs.getContentPath(contentHash)

	// Organize and move the segment
	segmentFilename := filepath.Base(segmentPath)
	destPath := cs.organizeStreamingFile(contentPath, "", segmentFilename)
	destDir := filepath.Dir(destPath)

	// Ensure destination directory exists
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create segment directory: %w", err)
	}

	// Move segment file
	if err := os.Rename(segmentPath, destPath); err != nil {
		return fmt.Errorf("failed to move segment: %w", err)
	}

	// Update metadata
	metadata.SegmentCount++
	metadata.LastSegmentTime = time.Now()
	metadata.LastAccessed = time.Now()

	// Calculate new total duration
	if metadata.SegmentDuration > 0 {
		metadata.TotalDuration = time.Duration(metadata.SegmentCount*metadata.SegmentDuration) * time.Second
	}

	// Save updated metadata
	if err := cs.metadata.Save(contentHash, *metadata); err != nil {
		return fmt.Errorf("failed to update metadata: %w", err)
	}

	cs.logger.Debug("Added segment to content",
		"hash", contentHash,
		"segment", segmentFilename,
		"total_segments", metadata.SegmentCount,
	)

	return nil
}

// GetSegments returns all segments for streaming content
func (cs *ContentStore) GetSegments(contentHash string) ([]string, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	contentPath := cs.getContentPath(contentHash)
	var segments []string

	// Check video segments
	videoDir := filepath.Join(contentPath, "video")
	if videoSegments, err := cs.listSegmentsInDir(videoDir); err == nil {
		segments = append(segments, videoSegments...)
	}

	// Check audio segments
	audioDir := filepath.Join(contentPath, "audio")
	if audioSegments, err := cs.listSegmentsInDir(audioDir); err == nil {
		segments = append(segments, audioSegments...)
	}

	// Check general segments
	segmentsDir := filepath.Join(contentPath, "segments")
	if generalSegments, err := cs.listSegmentsInDir(segmentsDir); err == nil {
		segments = append(segments, generalSegments...)
	}

	return segments, nil
}

// listSegmentsInDir lists all segment files in a directory
func (cs *ContentStore) listSegmentsInDir(dir string) ([]string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var segments []string
	for _, file := range files {
		if !file.IsDir() {
			name := file.Name()
			// Check for segment file extensions
			if strings.HasSuffix(name, ".m4s") || strings.HasSuffix(name, ".ts") || strings.HasSuffix(name, ".mp4") {
				segments = append(segments, filepath.Join(dir, name))
			}
		}
	}

	return segments, nil
}

// SegmentInfo represents information about a streaming segment
type SegmentInfo struct {
	Index    int    `json:"index"`
	Profile  string `json:"profile"`
	Type     string `json:"type"` // "video" or "audio"
	Duration int    `json:"duration"`
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

		if info.IsDir() || !isMetadataFile(path) {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			ms.logger.Warn("Failed to read metadata file", "path", path, "error", err)
			return nil
		}

		var metadata ContentMetadata
		if err := json.Unmarshal(data, &metadata); err != nil {
			ms.logger.Warn("Failed to unmarshal metadata", "path", path, "error", err)
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

// isMetadataFile checks if a file is a metadata JSON file
func isMetadataFile(path string) bool {
	return filepath.Ext(path) == ".json"
}

// URLGenerator creates CDN-friendly URLs for content
type URLGenerator struct {
	baseURL    string
	cdnEnabled bool
	cdnDomain  string
}

// NewURLGenerator creates a new URL generator
func NewURLGenerator(baseURL string, cdnEnabled bool, cdnDomain string) *URLGenerator {
	return &URLGenerator{
		baseURL:    baseURL,
		cdnEnabled: cdnEnabled,
		cdnDomain:  cdnDomain,
	}
}

// GenerateManifestURL creates a URL for accessing the manifest
func (ug *URLGenerator) GenerateManifestURL(contentHash string, format string) string {
	var manifestFile string
	switch format {
	case "dash":
		manifestFile = "manifest.mpd"
	case "hls":
		manifestFile = "playlist.m3u8"
	default:
		manifestFile = "output.mp4"
	}

	if ug.cdnEnabled && ug.cdnDomain != "" {
		// CDN URL: https://cdn.example.com/content/ab/abcdef123456/manifest.mpd
		return fmt.Sprintf("https://%s/content/%s/%s/%s",
			ug.cdnDomain,
			contentHash[:2],
			contentHash,
			manifestFile,
		)
	}

	// Direct URL: /api/v1/content/abcdef123456/manifest.mpd
	return fmt.Sprintf("%s/api/v1/content/%s/%s",
		ug.baseURL,
		contentHash,
		manifestFile,
	)
}

// GenerateSegmentURL creates a URL for accessing segments
func (ug *URLGenerator) GenerateSegmentURL(contentHash string, segmentPath string) string {
	if ug.cdnEnabled && ug.cdnDomain != "" {
		return fmt.Sprintf("https://%s/content/%s/%s/%s",
			ug.cdnDomain,
			contentHash[:2],
			contentHash,
			segmentPath,
		)
	}

	return fmt.Sprintf("%s/api/v1/content/%s/%s",
		ug.baseURL,
		contentHash,
		segmentPath,
	)
}
