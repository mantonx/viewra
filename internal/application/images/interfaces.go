package images

import (
	"context"

	"github.com/mantonx/viewra/internal/domain/images"
)

// GetImageExecutor defines the interface for getting a single image
type GetImageExecutor interface {
	Execute(ctx context.Context, id int) (*ImageResponse, error)
}

// GetMediaImagesExecutor defines the interface for getting all images for a media item
type GetMediaImagesExecutor interface {
	Execute(ctx context.Context, mediaID int) (*ListImagesResponse, error)
}

// GetEntityImagesExecutor defines the interface for getting all images for an entity
type GetEntityImagesExecutor interface {
	Execute(ctx context.Context, mediaType images.MediaType, entityID int) (*ListImagesResponse, error)
}

// ExtractMovieImagesExecutor defines the interface for extracting movie images
type ExtractMovieImagesExecutor interface {
	Execute(ctx context.Context, movieFilePath string, mediaType images.MediaType, entityID int, mediaID *int) error
}

// ExtractTVEpisodeImagesExecutor defines the interface for extracting TV episode images
type ExtractTVEpisodeImagesExecutor interface {
	Execute(ctx context.Context, episodeFilePath string, mediaType images.MediaType, entityID int, mediaID *int) error
}

// ExtractMusicAlbumImagesExecutor defines the interface for extracting music album images
type ExtractMusicAlbumImagesExecutor interface {
	Execute(ctx context.Context, albumDir string, mediaType images.MediaType, entityID int) error
}

// GetBatchMediaImagesExecutor defines the interface for getting images for multiple media items or entities
type GetBatchMediaImagesExecutor interface {
	Execute(ctx context.Context, mediaIDs []int, mediaType string, entityIDs []int) (*BatchImagesResponse, error)
}
