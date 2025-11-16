package music

import (
	"context"
	"fmt"

	"github.com/viewra/viewra/internal/domain/media"
)

// ListTracksUseCase handles the business logic for listing music tracks
type ListTracksUseCase struct {
	repo media.MusicRepository
}

// NewListTracksUseCase creates a new instance of ListTracksUseCase
func NewListTracksUseCase(repo media.MusicRepository) *ListTracksUseCase {
	return &ListTracksUseCase{
		repo: repo,
	}
}

// ExecuteByAlbum retrieves all tracks for a specific album
func (uc *ListTracksUseCase) ExecuteByAlbum(ctx context.Context, libraryID int64, album string) (ListTracksResponse, error) {
	tracks, err := uc.repo.ListMusicTracksByAlbum(ctx, libraryID, album)
	if err != nil {
		return ListTracksResponse{}, fmt.Errorf("failed to list tracks for album %s: %w", album, err)
	}

	return ToListTracksResponse(tracks), nil
}

// ExecuteByLibrary retrieves all tracks in a library
func (uc *ListTracksUseCase) ExecuteByLibrary(ctx context.Context, libraryID int64) (ListTracksResponse, error) {
	tracks, err := uc.repo.ListMusicTracksByLibrary(ctx, libraryID)
	if err != nil {
		return ListTracksResponse{}, fmt.Errorf("failed to list tracks: %w", err)
	}

	return ToListTracksResponse(tracks), nil
}
