package media

import (
	"context"
	"fmt"

	"github.com/viewra/viewra/internal/domain/media"
)

// ListMediaUseCase handles the business logic for listing media items
type ListMediaUseCase struct {
	repo media.Repository
}

// NewListMediaUseCase creates a new instance of ListMediaUseCase
func NewListMediaUseCase(repo media.Repository) *ListMediaUseCase {
	return &ListMediaUseCase{
		repo: repo,
	}
}

// Execute retrieves all media items in a library
func (uc *ListMediaUseCase) Execute(ctx context.Context, libraryID int64) (ListMediaResponse, error) {
	mediaList, err := uc.repo.ListByLibrary(ctx, libraryID)
	if err != nil {
		return ListMediaResponse{}, fmt.Errorf("failed to list media: %w", err)
	}

	return ToListMediaResponse(mediaList), nil
}

// ExecuteByType retrieves all media items of a specific type in a library
func (uc *ListMediaUseCase) ExecuteByType(ctx context.Context, libraryID int64, mediaType media.MediaType) (ListMediaResponse, error) {
	mediaList, err := uc.repo.ListByType(ctx, libraryID, mediaType)
	if err != nil {
		return ListMediaResponse{}, fmt.Errorf("failed to list media by type: %w", err)
	}

	return ToListMediaResponse(mediaList), nil
}
