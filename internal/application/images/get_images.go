package images

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/domain/images"
)

// GetImageUseCase handles retrieving a single image
type GetImageUseCase struct {
	repo images.Repository
}

// NewGetImageUseCase creates a new instance of GetImageUseCase
func NewGetImageUseCase(repo images.Repository) *GetImageUseCase {
	return &GetImageUseCase{repo: repo}
}

// Execute retrieves an image by its ID
func (uc *GetImageUseCase) Execute(ctx context.Context, id int) (*ImageResponse, error) {
	img, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get image: %w", err)
	}

	response := ToImageResponse(img)
	return &response, nil
}

// GetMediaImagesUseCase handles retrieving all images for a media item
type GetMediaImagesUseCase struct {
	repo images.Repository
}

// NewGetMediaImagesUseCase creates a new instance of GetMediaImagesUseCase
func NewGetMediaImagesUseCase(repo images.Repository) *GetMediaImagesUseCase {
	return &GetMediaImagesUseCase{repo: repo}
}

// Execute retrieves all images for a media item
func (uc *GetMediaImagesUseCase) Execute(ctx context.Context, mediaID int) (*ListImagesResponse, error) {
	imgs, err := uc.repo.GetByMediaID(ctx, mediaID)
	if err != nil {
		return nil, fmt.Errorf("failed to get media images: %w", err)
	}

	response := ToListImagesResponse(imgs)
	return &response, nil
}

// GetEntityImagesUseCase handles retrieving all images for an entity
type GetEntityImagesUseCase struct {
	repo images.Repository
}

// NewGetEntityImagesUseCase creates a new instance of GetEntityImagesUseCase
func NewGetEntityImagesUseCase(repo images.Repository) *GetEntityImagesUseCase {
	return &GetEntityImagesUseCase{repo: repo}
}

// Execute retrieves all images for an entity
func (uc *GetEntityImagesUseCase) Execute(ctx context.Context, mediaType images.MediaType, entityID int) (*ListImagesResponse, error) {
	imgs, err := uc.repo.GetByEntity(ctx, mediaType, entityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get entity images: %w", err)
	}

	response := ToListImagesResponse(imgs)
	return &response, nil
}

// GetBatchMediaImagesUseCase handles retrieving images for multiple media items
type GetBatchMediaImagesUseCase struct {
	repo images.Repository
}

// NewGetBatchMediaImagesUseCase creates a new instance of GetBatchMediaImagesUseCase
func NewGetBatchMediaImagesUseCase(repo images.Repository) *GetBatchMediaImagesUseCase {
	return &GetBatchMediaImagesUseCase{repo: repo}
}

// Execute retrieves images for multiple media items or entities
// For media-based lookup: provide mediaIDs and leave mediaType/entityIDs empty
// For entity-based lookup: provide mediaType and entityIDs, leave mediaIDs empty
func (uc *GetBatchMediaImagesUseCase) Execute(ctx context.Context, mediaIDs []int, mediaType string, entityIDs []int) (*BatchImagesResponse, error) {
	result := make(map[int][]ImageResponse)

	// Media-based batch lookup (movies, episodes, tracks with media files)
	if len(mediaIDs) > 0 {
		for _, mediaID := range mediaIDs {
			imgs, err := uc.repo.GetByMediaID(ctx, mediaID)
			if err != nil {
				// Continue on error - some media items might not have images
				result[mediaID] = []ImageResponse{}
				continue
			}

			// Convert to response format
			responses := make([]ImageResponse, len(imgs))
			for i, img := range imgs {
				responses[i] = ToImageResponse(img)
			}
			result[mediaID] = responses
		}
		return &BatchImagesResponse{MediaImages: result}, nil
	}

	// Entity-based batch lookup (TV shows, seasons, music artists/albums)
	if len(entityIDs) > 0 && mediaType != "" {
		// Convert string media type to domain type
		var domainMediaType images.MediaType
		switch mediaType {
		case "tv_show":
			domainMediaType = images.MediaTypeTVShow
		case "tv_season":
			domainMediaType = images.MediaTypeTVSeason
		case "music_artist":
			domainMediaType = images.MediaTypeMusicArtist
		case "music_album":
			domainMediaType = images.MediaTypeMusicAlbum
		default:
			return &BatchImagesResponse{MediaImages: make(map[int][]ImageResponse)}, nil
		}

		for _, entityID := range entityIDs {
			imgs, err := uc.repo.GetByEntity(ctx, domainMediaType, entityID)
			if err != nil {
				// Continue on error - some entities might not have images
				result[entityID] = []ImageResponse{}
				continue
			}

			// Convert to response format
			responses := make([]ImageResponse, len(imgs))
			for i, img := range imgs {
				responses[i] = ToImageResponse(img)
			}
			result[entityID] = responses
		}
		return &BatchImagesResponse{MediaImages: result}, nil
	}

	// Empty request
	return &BatchImagesResponse{MediaImages: make(map[int][]ImageResponse)}, nil
}
