package images

import (
	"fmt"
	"path/filepath"
	"time"
)

// Image represents an image asset for media items
type Image struct {
	ID        int
	MediaID   *int      // Nullable - for media files (movies, episodes, tracks)
	MediaType MediaType // Type of entity (movie, tv_show, tv_episode, etc.)
	EntityID  int       // ID in the specific entity table

	ImageType  ImageType
	SourceType SourceType

	// File locations
	FilePath       string  // For local files: absolute or relative path
	ExternalURL    *string // For downloaded images: original URL
	LocalCachePath *string // For external images: cached local path

	// Image metadata
	Width        *int    // Image width in pixels
	Height       *int    // Image height in pixels
	FileSizeBytes *int64  // File size for cleanup decisions
	MimeType     *string // image/jpeg, image/png, image/webp, etc.
	FileHash     *string // SHA256 hash for deduplication/integrity

	// Multi-language and priority
	Language *string // For multi-language posters (en, fr, ja, etc.)
	Priority int     // For multiple images of same type (0=highest priority)

	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate checks if the image entity is valid
func (i *Image) Validate() error {
	if !i.MediaType.IsValid() {
		return fmt.Errorf("invalid media type: %s", i.MediaType)
	}

	if !i.ImageType.IsValid() {
		return fmt.Errorf("invalid image type: %s", i.ImageType)
	}

	if !i.SourceType.IsValid() {
		return fmt.Errorf("invalid source type: %s", i.SourceType)
	}

	if i.EntityID <= 0 {
		return fmt.Errorf("entity_id must be positive")
	}

	// At least one location must be specified
	if i.FilePath == "" && (i.ExternalURL == nil || *i.ExternalURL == "") &&
		(i.LocalCachePath == nil || *i.LocalCachePath == "") {
		return fmt.Errorf("at least one file location must be specified")
	}

	return nil
}

// IsLocal returns true if this is a local file
func (i *Image) IsLocal() bool {
	return i.SourceType == SourceTypeLocal
}

// IsExternal returns true if this came from an external source
func (i *Image) IsExternal() bool {
	return i.SourceType != SourceTypeLocal
}

// GetActualPath returns the actual file path to serve
// Priority: LocalCachePath > FilePath > nil
func (i *Image) GetActualPath() string {
	// Prefer cached version for external images
	if i.LocalCachePath != nil && *i.LocalCachePath != "" {
		return *i.LocalCachePath
	}

	// Fall back to original file path
	if i.FilePath != "" {
		return i.FilePath
	}

	return ""
}

// GetFileName returns just the filename from the path
func (i *Image) GetFileName() string {
	path := i.GetActualPath()
	if path == "" {
		return ""
	}
	return filepath.Base(path)
}

// HasMetadata returns true if image metadata has been extracted
func (i *Image) HasMetadata() bool {
	return i.Width != nil && i.Height != nil && i.MimeType != nil
}

// GetAspectRatio returns the aspect ratio (width/height) if dimensions are known
func (i *Image) GetAspectRatio() *float64 {
	if i.Width == nil || i.Height == nil || *i.Height == 0 {
		return nil
	}

	ratio := float64(*i.Width) / float64(*i.Height)
	return &ratio
}

// IsPoster returns true if this is a poster-type image (2:3 aspect ratio expected)
func (i *Image) IsPoster() bool {
	return i.ImageType == ImageTypePoster
}

// IsFanart returns true if this is fanart/backdrop (16:9 aspect ratio expected)
func (i *Image) IsFanart() bool {
	return i.ImageType == ImageTypeFanart || i.ImageType == ImageTypeBackdrop
}

// IsLogo returns true if this is a logo type image
func (i *Image) IsLogo() bool {
	return i.ImageType == ImageTypeClearLogo || i.ImageType == ImageTypeLogo
}
