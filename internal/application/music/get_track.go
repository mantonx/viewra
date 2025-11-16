package music

import (
	"context"
	"fmt"

	"github.com/viewra/viewra/internal/domain/media"
)

// GetTrackUseCase handles the business logic for getting a single music track
type GetTrackUseCase struct {
	repo media.MusicRepository
}

// NewGetTrackUseCase creates a new instance of GetTrackUseCase
func NewGetTrackUseCase(repo media.MusicRepository) *GetTrackUseCase {
	return &GetTrackUseCase{
		repo: repo,
	}
}

// Execute retrieves a music track by its ID
func (uc *GetTrackUseCase) Execute(ctx context.Context, id int64) (*MusicTrackResponse, error) {
	track, err := uc.repo.GetMusicTrackByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get music track: %w", err)
	}

	response := ToMusicTrackResponse(track)
	return &response, nil
}
