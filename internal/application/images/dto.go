package images

import (
	"time"

	"github.com/viewra/viewra/internal/domain/images"
)

// ImageResponse represents an image in the API response
type ImageResponse struct {
	ID             int       `json:"id"`
	MediaID        *int      `json:"media_id,omitempty"`
	MediaType      string    `json:"media_type"`
	EntityID       int       `json:"entity_id"`
	ImageType      string    `json:"image_type"`
	SourceType     string    `json:"source_type"`
	FilePath       string    `json:"file_path,omitempty"`
	ExternalURL    *string   `json:"external_url,omitempty"`
	LocalCachePath *string   `json:"local_cache_path,omitempty"`
	Width          *int      `json:"width,omitempty"`
	Height         *int      `json:"height,omitempty"`
	FileSizeBytes  *int64    `json:"file_size_bytes,omitempty"`
	MimeType       *string   `json:"mime_type,omitempty"`
	FileHash       *string   `json:"file_hash,omitempty"`
	Language       *string   `json:"language,omitempty"`
	Priority       int       `json:"priority"`
	AspectRatio    *float64  `json:"aspect_ratio,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ListImagesResponse represents a list of images
type ListImagesResponse struct {
	Images []ImageResponse `json:"images"`
	Total  int             `json:"total"`
}

// ToImageResponse converts a domain Image to an ImageResponse
func ToImageResponse(img *images.Image) ImageResponse {
	return ImageResponse{
		ID:             img.ID,
		MediaID:        img.MediaID,
		MediaType:      string(img.MediaType),
		EntityID:       img.EntityID,
		ImageType:      string(img.ImageType),
		SourceType:     string(img.SourceType),
		FilePath:       img.FilePath,
		ExternalURL:    img.ExternalURL,
		LocalCachePath: img.LocalCachePath,
		Width:          img.Width,
		Height:         img.Height,
		FileSizeBytes:  img.FileSizeBytes,
		MimeType:       img.MimeType,
		FileHash:       img.FileHash,
		Language:       img.Language,
		Priority:       img.Priority,
		AspectRatio:    img.GetAspectRatio(),
		CreatedAt:      img.CreatedAt,
		UpdatedAt:      img.UpdatedAt,
	}
}

// ToListImagesResponse converts a slice of domain Images to ListImagesResponse
func ToListImagesResponse(imgs []*images.Image) ListImagesResponse {
	responses := make([]ImageResponse, len(imgs))
	for i, img := range imgs {
		responses[i] = ToImageResponse(img)
	}
	return ListImagesResponse{
		Images: responses,
		Total:  len(responses),
	}
}
