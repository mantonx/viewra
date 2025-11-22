package music

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/domain/media"
)

// SearchTracksUseCase handles the business logic for searching music tracks
type SearchTracksUseCase struct {
	repo media.MusicRepository
}

// NewSearchTracksUseCase creates a new instance of SearchTracksUseCase
func NewSearchTracksUseCase(repo media.MusicRepository) *SearchTracksUseCase {
	return &SearchTracksUseCase{
		repo: repo,
	}
}

// Execute searches for music tracks by title, artist, or album
func (uc *SearchTracksUseCase) Execute(ctx context.Context, libraryID int64, query string) (ListTracksResponse, error) {
	if query == "" {
		return ListTracksResponse{}, fmt.Errorf("search query cannot be empty")
	}

	tracks, err := uc.repo.SearchMusicTracks(ctx, libraryID, query)
	if err != nil {
		return ListTracksResponse{}, fmt.Errorf("failed to search music tracks: %w", err)
	}

	return ToListTracksResponse(tracks), nil
}
