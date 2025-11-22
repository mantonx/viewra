package images

import "github.com/mantonx/viewra/internal/domain/images"

// ImageInfo represents extracted image information
type ImageInfo struct {
	// File path (absolute or relative to media file)
	Path string

	// Image type (poster, fanart, etc.)
	Type images.ImageType

	// Priority (0 = highest) for multiple images of same type
	Priority int

	// Language code (if applicable, e.g., "en", "fr")
	Language *string

	// Metadata (populated after extraction)
	Width         *int
	Height        *int
	FileSizeBytes *int64
	MimeType      *string
	FileHash      *string
}

// ExtractedImages contains all images found for a media item
type ExtractedImages struct {
	// All discovered images
	Images []ImageInfo
}
