package images

import (
	"context"
	"fmt"

	"github.com/viewra/viewra/internal/domain/images"
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
