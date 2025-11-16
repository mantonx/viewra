package tv

import (
	"context"
	"fmt"

	"github.com/viewra/viewra/internal/infrastructure/persistence/tvshow"
)

// ListTVShowsUseCase handles the business logic for listing TV shows with aggregated data
type ListTVShowsUseCase struct {
	repo *tvshow.Repository
}

// NewListTVShowsUseCase creates a new instance of ListTVShowsUseCase
func NewListTVShowsUseCase(repo *tvshow.Repository) *ListTVShowsUseCase {
	return &ListTVShowsUseCase{
		repo: repo,
	}
}

// Execute retrieves all TV shows in a library with episode and season counts
func (uc *ListTVShowsUseCase) Execute(ctx context.Context, libraryID int64) (ListTVShowsResponse, error) {
	shows, err := uc.repo.ListTVShowsByLibrary(ctx, libraryID)
	if err != nil {
		return ListTVShowsResponse{}, fmt.Errorf("failed to list TV shows: %w", err)
	}

	// Build response with aggregated data
	responses := make([]TVShowSummary, len(shows))
	for i, show := range shows {
		// Get seasons for each show to count them
		seasons, err := uc.repo.ListTVSeasonsByShow(ctx, show.ID)
		if err != nil {
			return ListTVShowsResponse{}, fmt.Errorf("failed to get seasons for show %d: %w", show.ID, err)
		}

		// Calculate total episode count
		totalEpisodes := 0
		for _, season := range seasons {
			totalEpisodes += season.EpisodeCount
		}

		responses[i] = TVShowSummary{
			ID:           show.ID,
			LibraryID:    show.LibraryID,
			Title:        show.Title,
			SeasonCount:  len(seasons),
			EpisodeCount: totalEpisodes,
		}
	}

	return ListTVShowsResponse{
		Shows: responses,
		Total: len(responses),
	}, nil
}
