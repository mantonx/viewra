package tv

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/infrastructure/persistence/tvshow"
)

// GetTVShowUseCase handles the business logic for getting a single TV show
type GetTVShowUseCase struct {
	repo *tvshow.Repository
}

// NewGetTVShowUseCase creates a new instance of GetTVShowUseCase
func NewGetTVShowUseCase(repo *tvshow.Repository) *GetTVShowUseCase {
	return &GetTVShowUseCase{
		repo: repo,
	}
}

// Execute retrieves a TV show by ID with aggregated data
func (uc *GetTVShowUseCase) Execute(ctx context.Context, showID int64) (*TVShowDetailResponse, error) {
	show, err := uc.repo.GetTVShowByID(ctx, showID)
	if err != nil {
		return nil, fmt.Errorf("failed to get TV show: %w", err)
	}

	// Get seasons for the show to count them
	seasons, err := uc.repo.ListTVSeasonsByShow(ctx, show.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get seasons for show %d: %w", show.ID, err)
	}

	// Calculate total episode count
	totalEpisodes := 0
	for _, season := range seasons {
		totalEpisodes += season.EpisodeCount
	}

	return &TVShowDetailResponse{
		ID:            show.ID,
		LibraryID:     show.LibraryID,
		Title:         show.Title,
		Year:          show.Year,
		Genre:         show.Genre,
		Plot:          show.Plot,
		IMDbID:        show.IMDbID,
		TMDbID:        show.TMDbID,
		ContentRating: show.ContentRating,
		SeasonCount:   len(seasons),
		EpisodeCount:  totalEpisodes,
	}, nil
}
