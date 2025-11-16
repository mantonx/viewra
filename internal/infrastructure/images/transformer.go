package images

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
	"github.com/viewra/viewra/internal/domain/images"
)

// TransformOptions contains parameters for image transformation
// All images are converted to WebP format
type TransformOptions struct {
	Width   int
	Height  int
	Quality int // 1-100 for WebP quality
}

// ImagePreset defines a specific size/quality variant to generate
type ImagePreset struct {
	Name    string // e.g., "thumb", "medium", "large"
	Width   int
	Height  int
	Quality int
}

// ImagePresets defines all size variants to pre-generate during scan
// This eliminates on-demand transformation overhead
var ImagePresets = map[images.ImageType][]ImagePreset{
	// Posters (2:3 aspect ratio - vertical)
	images.ImageTypePoster: {
		{Name: "thumb", Width: 200, Height: 300, Quality: 100},    // Grid thumbnail
		{Name: "medium", Width: 400, Height: 600, Quality: 100},   // List view
		{Name: "large", Width: 600, Height: 900, Quality: 100},    // Detail modal
		{Name: "xlarge", Width: 800, Height: 1200, Quality: 100}, // Full detail view
	},

	// Fanart/Backdrops (16:9 aspect ratio - horizontal)
	images.ImageTypeFanart: {
		{Name: "thumb", Width: 320, Height: 180, Quality: 100},    // Small preview
		{Name: "medium", Width: 854, Height: 480, Quality: 100},   // Standard view (480p)
		{Name: "large", Width: 1280, Height: 720, Quality: 100},   // HD view (720p)
		{Name: "xlarge", Width: 1920, Height: 1080, Quality: 100}, // Full HD (1080p)
	},
	images.ImageTypeBackdrop: {
		{Name: "thumb", Width: 320, Height: 180, Quality: 100},
		{Name: "medium", Width: 854, Height: 480, Quality: 100},
		{Name: "large", Width: 1280, Height: 720, Quality: 100},
		{Name: "xlarge", Width: 1920, Height: 1080, Quality: 100},
	},
	images.ImageTypeExtraFanart: {
		{Name: "thumb", Width: 320, Height: 180, Quality: 100},
		{Name: "medium", Width: 854, Height: 480, Quality: 100},
		{Name: "large", Width: 1280, Height: 720, Quality: 100},
		{Name: "xlarge", Width: 1920, Height: 1080, Quality: 100},
	},

	// Episode Thumbnails (16:9 aspect ratio)
	images.ImageTypeThumb: {
		{Name: "thumb", Width: 320, Height: 180, Quality: 100},  // List view
		{Name: "medium", Width: 640, Height: 360, Quality: 100}, // Hover preview
		{Name: "large", Width: 854, Height: 480, Quality: 100},  // Detail view
	},

	// Landscape Posters (16:9 aspect ratio)
	images.ImageTypeLandscape: {
		{Name: "thumb", Width: 320, Height: 180, Quality: 100},
		{Name: "medium", Width: 640, Height: 360, Quality: 100},
		{Name: "large", Width: 1280, Height: 720, Quality: 100},
	},

	// Banners (wide horizontal - approximately 7:1 aspect ratio)
	images.ImageTypeBanner: {
		{Name: "medium", Width: 758, Height: 140, Quality: 100},  // Standard banner
		{Name: "large", Width: 1000, Height: 185, Quality: 100}, // Large banner
	},

	// Logos (variable aspect ratio, preserve transparency)
	images.ImageTypeClearLogo: {
		{Name: "medium", Width: 400, Height: 0, Quality: 100}, // Width-based, maintain aspect
		{Name: "large", Width: 800, Height: 0, Quality: 100},
	},
	images.ImageTypeLogo: {
		{Name: "medium", Width: 400, Height: 0, Quality: 100},
		{Name: "large", Width: 800, Height: 0, Quality: 100},
	},

	// Clear Art and Character Art (variable aspect ratio)
	images.ImageTypeClearArt: {
		{Name: "medium", Width: 500, Height: 0, Quality: 100},
		{Name: "large", Width: 1000, Height: 0, Quality: 100},
	},
	images.ImageTypeCharacterArt: {
		{Name: "medium", Width: 500, Height: 0, Quality: 100},
		{Name: "large", Width: 1000, Height: 0, Quality: 100},
	},

	// Actor Headshots (2:3 aspect ratio - vertical)
	images.ImageTypeActor: {
		{Name: "thumb", Width: 100, Height: 150, Quality: 100},
		{Name: "medium", Width: 200, Height: 300, Quality: 100},
		{Name: "large", Width: 300, Height: 450, Quality: 100},
	},

	// Music Album Covers (1:1 aspect ratio - square)
	images.ImageTypeCover: {
		{Name: "thumb", Width: 150, Height: 150, Quality: 100},    // Grid thumbnail
		{Name: "medium", Width: 300, Height: 300, Quality: 100},   // List view
		{Name: "large", Width: 500, Height: 500, Quality: 100},    // Detail view
		{Name: "xlarge", Width: 1000, Height: 1000, Quality: 100}, // Full resolution
	},
	images.ImageTypeFolder: {
		{Name: "thumb", Width: 150, Height: 150, Quality: 100},
		{Name: "medium", Width: 300, Height: 300, Quality: 100},
		{Name: "large", Width: 500, Height: 500, Quality: 100},
	},

	// Disc Art (1:1 aspect ratio - square, preserve transparency)
	images.ImageTypeDiscArt: {
		{Name: "medium", Width: 300, Height: 300, Quality: 100},
		{Name: "large", Width: 500, Height: 500, Quality: 100},
	},
}

// Transformer handles image transformations (resizing, format conversion)
type Transformer struct {
	cacheService *CacheService
}

// NewTransformer creates a new image transformer
func NewTransformer(cacheService *CacheService) *Transformer {
	return &Transformer{
		cacheService: cacheService,
	}
}

// Transform resizes and/or converts an image format
// Returns the path to the transformed image (cached)
func (t *Transformer) Transform(sourcePath, fileHash string, opts TransformOptions) (string, error) {
	// Build cache key for transformed image (includes sharding path)
	cacheKey := t.buildCacheKey(fileHash, opts)

	// Check if transformed version already exists in cache
	if t.cacheService.FileExists(cacheKey) {
		return cacheKey, nil
	}

	// Load source image
	src, err := imaging.Open(sourcePath)
	if err != nil {
		return "", fmt.Errorf("failed to open image: %w", err)
	}

	// Apply transformations
	transformed := src

	// Resize if dimensions specified
	if opts.Width > 0 || opts.Height > 0 {
		transformed = t.resize(src, opts.Width, opts.Height)
	}

	// Get full cache path and ensure directory structure exists
	cachePath := t.cacheService.GetCachedPath(cacheKey)
	cacheDir := filepath.Dir(cachePath)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	if err := t.saveImageAsWebP(transformed, cachePath, opts.Quality); err != nil {
		return "", fmt.Errorf("failed to save transformed image: %w", err)
	}

	return cacheKey, nil
}

// TransformAllPresets generates all preset sizes for a given image type
// Returns a map of preset names to cache paths
func (t *Transformer) TransformAllPresets(sourcePath, fileHash string, imageType images.ImageType) (map[string]string, error) {
	// Get presets for this image type
	presets, exists := ImagePresets[imageType]
	if !exists || len(presets) == 0 {
		// No presets defined for this image type, return empty
		return map[string]string{}, nil
	}

	// Load source image once
	src, err := imaging.Open(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open image: %w", err)
	}

	results := make(map[string]string)

	// Generate each preset
	for _, preset := range presets {
		opts := TransformOptions{
			Width:   preset.Width,
			Height:  preset.Height,
			Quality: preset.Quality,
		}

		// Build cache key with preset name and image type
		cacheKey := buildCacheKeyWithName(fileHash, opts, preset.Name, imageType)

		// Skip if already exists
		if t.cacheService.FileExists(cacheKey) {
			results[preset.Name] = cacheKey
			continue
		}

		// Apply transformations
		transformed := src
		if opts.Width > 0 || opts.Height > 0 {
			transformed = t.resize(src, opts.Width, opts.Height)
		}

		// Get full cache path and ensure directory structure exists
		cachePath := t.cacheService.GetCachedPath(cacheKey)
		cacheDir := filepath.Dir(cachePath)
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create cache directory: %w", err)
		}

		// Save as WebP
		if err := t.saveImageAsWebP(transformed, cachePath, opts.Quality); err != nil {
			return nil, fmt.Errorf("failed to save preset %s: %w", preset.Name, err)
		}

		results[preset.Name] = cacheKey
	}

	return results, nil
}

// resize resizes an image preserving aspect ratio
func (t *Transformer) resize(src image.Image, width, height int) image.Image {
	srcBounds := src.Bounds()
	srcWidth := srcBounds.Dx()
	srcHeight := srcBounds.Dy()

	// If both width and height are specified, use them as max bounds
	if width > 0 && height > 0 {
		return imaging.Fit(src, width, height, imaging.Lanczos)
	}

	// If only width specified, calculate height maintaining aspect ratio
	if width > 0 {
		aspectRatio := float64(srcHeight) / float64(srcWidth)
		height = int(float64(width) * aspectRatio)
		return imaging.Resize(src, width, height, imaging.Lanczos)
	}

	// If only height specified, calculate width maintaining aspect ratio
	if height > 0 {
		aspectRatio := float64(srcWidth) / float64(srcHeight)
		width = int(float64(height) * aspectRatio)
		return imaging.Resize(src, width, height, imaging.Lanczos)
	}

	// No resize needed
	return src
}

// saveImageAsWebP saves an image as WebP format
func (t *Transformer) saveImageAsWebP(img image.Image, path string, quality int) error {
	// Default quality
	if quality == 0 {
		quality = 85 // Default to 85% quality
	}

	// Ensure quality is in valid range
	if quality < 1 {
		quality = 1
	}
	if quality > 100 {
		quality = 100
	}

	// Create output file
	outFile, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// Encode as WebP with specified quality
	// Use lossless encoding for quality=100, lossy for everything else
	var opts *webp.Options
	if quality == 100 {
		// Lossless encoding - perfect quality, larger file size
		opts = &webp.Options{
			Lossless: true,
		}
	} else {
		// Lossy encoding with high quality
		// Convert quality (1-100) to WebP quality (0.0-1.0)
		webpQuality := float32(quality) / 100.0
		opts = &webp.Options{
			Lossless: false,
			Quality:  webpQuality,
		}
	}

	err = webp.Encode(outFile, img, opts)

	if err != nil {
		// Clean up partial file on error
		os.Remove(path)
		return fmt.Errorf("failed to encode image: %w", err)
	}

	return nil
}

// buildCacheKey generates a cache filename for a transformed image
// Format: {first2}/{next2}/{hash}_{imageType}_{width}x{height}.webp
// Example: 04/c3/04c38ff...poster_thumb.webp or 04/c3/04c38ff...fanart_300x450.webp
// Note: This is kept for backwards compatibility but imageType should be provided when possible
func (t *Transformer) buildCacheKey(fileHash string, opts TransformOptions) string {
	// Without imageType, use empty string (will be omitted from filename)
	return buildCacheKeyWithName(fileHash, opts, "", "")
}

// buildCacheKeyWithName generates a cache filename with an optional preset name
// Format: {first2}/{next2}/{hash}_{imageType}_{name}.webp or {hash}_{imageType}_{width}x{height}.webp
// Example: 04/c3/04c38ff...poster_thumb.webp or 04/c3/04c38ff...fanart_large.webp
func buildCacheKeyWithName(fileHash string, opts TransformOptions, presetName string, imageType images.ImageType) string {
	// Validate hash length for sharding
	if len(fileHash) < 4 {
		// Fallback to non-sharded for short hashes (shouldn't happen in practice)
		parts := []string{fileHash}
		if imageType != "" {
			parts = append(parts, string(imageType))
		}
		if presetName != "" {
			parts = append(parts, presetName)
		} else {
			parts = append(parts, "original")
		}
		return strings.Join(parts, "_") + ".webp"
	}

	// Build sharded path: first2/next2/filename
	level1 := fileHash[0:2]
	level2 := fileHash[2:4]

	var parts []string
	parts = append(parts, fileHash)

	// Include image type in filename if provided
	if imageType != "" {
		parts = append(parts, string(imageType))
	}

	// Use preset name if provided, otherwise use dimensions
	if presetName != "" {
		parts = append(parts, presetName)
	} else if opts.Width > 0 || opts.Height > 0 {
		dimStr := fmt.Sprintf("%dx%d", opts.Width, opts.Height)
		parts = append(parts, dimStr)
	} else {
		parts = append(parts, "original")
	}

	// Build filename with .webp extension
	filename := strings.Join(parts, "_") + ".webp"

	// Return sharded path
	return filepath.Join(level1, level2, filename)
}

// ParseTransformOptions parses transform options from query parameters
// All images are converted to WebP format
func ParseTransformOptions(width, height, quality string) (TransformOptions, error) {
	opts := TransformOptions{
		Quality: 85, // Default quality
	}

	// Parse width
	if width != "" {
		w, err := strconv.Atoi(width)
		if err != nil || w < 0 {
			return opts, fmt.Errorf("invalid width parameter: %s", width)
		}
		opts.Width = w
	}

	// Parse height
	if height != "" {
		h, err := strconv.Atoi(height)
		if err != nil || h < 0 {
			return opts, fmt.Errorf("invalid height parameter: %s", height)
		}
		opts.Height = h
	}

	// Parse quality
	if quality != "" {
		q, err := strconv.Atoi(quality)
		if err != nil || q < 1 || q > 100 {
			return opts, fmt.Errorf("invalid quality parameter: %s (must be 1-100)", quality)
		}
		opts.Quality = q
	}

	return opts, nil
}
