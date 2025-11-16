package tv

import (
	"context"
	"fmt"

	"github.com/viewra/viewra/internal/domain/media"
)

// SearchTVEpisodesUseCase handles the business logic for searching TV episodes
type SearchTVEpisodesUseCase struct {
	repo media.TVRepository
}

// NewSearchTVEpisodesUseCase creates a new instance of SearchTVEpisodesUseCase
func NewSearchTVEpisodesUseCase(repo media.TVRepository) *SearchTVEpisodesUseCase {
	return &SearchTVEpisodesUseCase{
		repo: repo,
	}
}

// Execute searches for TV episodes by show title or episode title
func (uc *SearchTVEpisodesUseCase) Execute(ctx context.Context, libraryID int64, query string) (ListTVEpisodesResponse, error) {
	if query == "" {
		return ListTVEpisodesResponse{}, fmt.Errorf("search query cannot be empty")
	}

	episodes, err := uc.repo.SearchTVEpisodes(ctx, libraryID, query)
	if err != nil {
		return ListTVEpisodesResponse{}, fmt.Errorf("failed to search TV episodes: %w", err)
	}

	return ToListTVEpisodesResponse(episodes), nil
}
