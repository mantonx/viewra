// Package images provides image processing and caching services.
package images

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mantonx/viewra/internal/domain/images"
	"github.com/mantonx/viewra/internal/infrastructure/ffmpeg/thumbnail"
)

// VideoThumbnailService extracts thumbnails from video files and stores them
// in the image cache with all preset sizes generated.
type VideoThumbnailService struct {
	thumbClient  *thumbnail.Client
	cacheService *CacheService
	transformer  *Transformer
	tempDir      string
}

// NewVideoThumbnailService creates a new video thumbnail service.
func NewVideoThumbnailService(
	thumbClient *thumbnail.Client,
	cacheService *CacheService,
	transformer *Transformer,
	tempDir string,
) *VideoThumbnailService {
	return &VideoThumbnailService{
		thumbClient:  thumbClient,
		cacheService: cacheService,
		transformer:  transformer,
		tempDir:      tempDir,
	}
}

// GenerateOptions configures thumbnail generation.
type GenerateOptions struct {
	// Duration of the video in seconds (used to calculate timestamp)
	Duration int
	// TimestampPercent is the percentage into the video to extract (default 10%)
	TimestampPercent float64
}

// GenerateResult contains the result of thumbnail generation.
type GenerateResult struct {
	// FileHash is the hash of the extracted frame (used as cache key)
	FileHash string
	// Presets maps preset names to their cache paths
	Presets map[string]string
}

// Generate extracts a thumbnail from a video and generates all preset sizes.
// Returns nil if thumbnails already exist in cache.
// The fileHash parameter is used as the cache key for deduplication.
func (s *VideoThumbnailService) Generate(
	ctx context.Context,
	videoPath string,
	fileHash string,
	opts GenerateOptions,
) (*GenerateResult, error) {
	// Check if thumbnails already exist for this hash
	if s.thumbnailsExist(fileHash) {
		// Return existing preset paths
		return s.getExistingPresets(fileHash)
	}

	// Calculate timestamp (default 10% into video)
	percent := opts.TimestampPercent
	if percent == 0 {
		percent = 0.10
	}
	timestamp := time.Duration(float64(opts.Duration)*percent) * time.Second

	// Create temp file for extracted frame
	tempPath := filepath.Join(s.tempDir, fmt.Sprintf("thumb_%s.png", fileHash))
	defer os.Remove(tempPath) // Clean up temp file

	// Ensure temp directory exists
	if err := os.MkdirAll(s.tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Extract frame using FFmpeg
	err := s.thumbClient.Generate(ctx, videoPath, tempPath, thumbnail.Options{
		Timestamp: timestamp,
		Quality:   2, // High quality for source frame
	})
	if err != nil {
		return nil, fmt.Errorf("failed to extract video frame: %w", err)
	}

	// Generate all preset sizes using the image transformer
	presets, err := s.transformer.TransformAllPresets(tempPath, fileHash, images.ImageTypeThumb)
	if err != nil {
		return nil, fmt.Errorf("failed to generate thumbnail presets: %w", err)
	}

	return &GenerateResult{
		FileHash: fileHash,
		Presets:  presets,
	}, nil
}

// thumbnailsExist checks if thumbnail presets already exist for this hash.
func (s *VideoThumbnailService) thumbnailsExist(fileHash string) bool {
	// Check if the smallest preset exists (thumb)
	// If it does, we assume all presets were generated
	presets := ImagePresets[images.ImageTypeThumb]
	if len(presets) == 0 {
		return false
	}

	// Build cache key for first preset
	opts := TransformOptions{
		Width:   presets[0].Width,
		Height:  presets[0].Height,
		Quality: presets[0].Quality,
	}
	cacheKey := buildCacheKeyWithName(fileHash, opts, presets[0].Name, images.ImageTypeThumb)

	return s.cacheService.FileExists(cacheKey)
}

// getExistingPresets returns the preset paths for an existing thumbnail.
func (s *VideoThumbnailService) getExistingPresets(fileHash string) (*GenerateResult, error) {
	presets := ImagePresets[images.ImageTypeThumb]
	result := &GenerateResult{
		FileHash: fileHash,
		Presets:  make(map[string]string),
	}

	for _, preset := range presets {
		opts := TransformOptions{
			Width:   preset.Width,
			Height:  preset.Height,
			Quality: preset.Quality,
		}
		cacheKey := buildCacheKeyWithName(fileHash, opts, preset.Name, images.ImageTypeThumb)
		result.Presets[preset.Name] = cacheKey
	}

	return result, nil
}

// GetThumbnailPath returns the cache path for a specific preset.
// Preset names are: "thumb", "medium", "large"
func (s *VideoThumbnailService) GetThumbnailPath(fileHash, preset string) (string, error) {
	presets := ImagePresets[images.ImageTypeThumb]
	for _, p := range presets {
		if p.Name == preset {
			opts := TransformOptions{
				Width:   p.Width,
				Height:  p.Height,
				Quality: p.Quality,
			}
			return buildCacheKeyWithName(fileHash, opts, p.Name, images.ImageTypeThumb), nil
		}
	}
	return "", fmt.Errorf("unknown preset: %s", preset)
}
