package tv

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/domain/media"
)

// ListTVEpisodesUseCase handles the business logic for listing TV episodes
type ListTVEpisodesUseCase struct {
	repo media.TVRepository
}

// NewListTVEpisodesUseCase creates a new instance of ListTVEpisodesUseCase
func NewListTVEpisodesUseCase(repo media.TVRepository) *ListTVEpisodesUseCase {
	return &ListTVEpisodesUseCase{
		repo: repo,
	}
}

// ExecuteByShow retrieves all episodes for a specific show by title
func (uc *ListTVEpisodesUseCase) ExecuteByShow(ctx context.Context, libraryID int64, showTitle string) (ListTVEpisodesResponse, error) {
	episodes, err := uc.repo.ListTVEpisodesByShow(ctx, libraryID, showTitle)
	if err != nil {
		return ListTVEpisodesResponse{}, fmt.Errorf("failed to list episodes for show %s: %w", showTitle, err)
	}

	return ToListTVEpisodesResponse(episodes), nil
}

// ExecuteByShowID retrieves all episodes for a specific show by ID
func (uc *ListTVEpisodesUseCase) ExecuteByShowID(ctx context.Context, showID int64) (ListTVEpisodesResponse, error) {
	episodes, err := uc.repo.ListTVEpisodesByShowID(ctx, showID)
	if err != nil {
		return ListTVEpisodesResponse{}, fmt.Errorf("failed to list episodes for show ID %d: %w", showID, err)
	}

	return ToListTVEpisodesResponse(episodes), nil
}

// ExecuteByLibrary retrieves all episodes in a library
func (uc *ListTVEpisodesUseCase) ExecuteByLibrary(ctx context.Context, libraryID int64) (ListTVEpisodesResponse, error) {
	episodes, err := uc.repo.ListTVEpisodesByLibrary(ctx, libraryID)
	if err != nil {
		return ListTVEpisodesResponse{}, fmt.Errorf("failed to list episodes: %w", err)
	}

	return ToListTVEpisodesResponse(episodes), nil
}
