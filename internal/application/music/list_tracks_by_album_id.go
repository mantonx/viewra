package music

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/domain/media"
)

// ListTracksByAlbumIDUseCase handles listing tracks by album ID
type ListTracksByAlbumIDUseCase struct {
	repo media.MusicRepository
}

// NewListTracksByAlbumIDUseCase creates a new instance
func NewListTracksByAlbumIDUseCase(repo media.MusicRepository) *ListTracksByAlbumIDUseCase {
	return &ListTracksByAlbumIDUseCase{
		repo: repo,
	}
}

// Execute retrieves all tracks for an album by album entity ID
func (uc *ListTracksByAlbumIDUseCase) Execute(ctx context.Context, albumID int64) (ListTracksResponse, error) {
	tracks, err := uc.repo.ListMusicTracksByAlbumID(ctx, albumID)
	if err != nil {
		return ListTracksResponse{}, fmt.Errorf("failed to list tracks for album: %w", err)
	}

	return ToListTracksResponse(tracks), nil
}
