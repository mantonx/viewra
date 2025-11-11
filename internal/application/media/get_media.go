package media

import (
	"context"
	"fmt"

	"github.com/viewra/viewra/internal/domain/media"
)

// GetMediaUseCase handles the business logic for retrieving a single media item
type GetMediaUseCase struct {
	repo media.Repository
}

// NewGetMediaUseCase creates a new instance of GetMediaUseCase
func NewGetMediaUseCase(repo media.Repository) *GetMediaUseCase {
	return &GetMediaUseCase{
		repo: repo,
	}
}

// Execute retrieves a media item by its ID
func (uc *GetMediaUseCase) Execute(ctx context.Context, id int64) (MediaResponse, error) {
	m, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return MediaResponse{}, fmt.Errorf("failed to get media: %w", err)
	}

	return ToMediaResponse(m), nil
}
