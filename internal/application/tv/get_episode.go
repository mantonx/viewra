package tv

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/domain/media"
)

// GetTVEpisodeUseCase handles the business logic for getting a single TV episode
type GetTVEpisodeUseCase struct {
	repo media.TVRepository
}

// NewGetTVEpisodeUseCase creates a new instance of GetTVEpisodeUseCase
func NewGetTVEpisodeUseCase(repo media.TVRepository) *GetTVEpisodeUseCase {
	return &GetTVEpisodeUseCase{
		repo: repo,
	}
}

// Execute retrieves a TV episode by its ID
func (uc *GetTVEpisodeUseCase) Execute(ctx context.Context, id int64) (*TVEpisodeResponse, error) {
	episode, err := uc.repo.GetTVEpisodeByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get TV episode: %w", err)
	}

	response := ToTVEpisodeResponse(episode)
	return &response, nil
}
