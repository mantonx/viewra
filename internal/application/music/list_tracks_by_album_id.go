package music

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/domain/media"
)

// ListTracksByAlbumIDUseCase handles listing tracks using an album's representative track ID
type ListTracksByAlbumIDUseCase struct {
	repo media.MusicRepository
}

// NewListTracksByAlbumIDUseCase creates a new instance
func NewListTracksByAlbumIDUseCase(repo media.MusicRepository) *ListTracksByAlbumIDUseCase {
	return &ListTracksByAlbumIDUseCase{
		repo: repo,
	}
}

// Execute retrieves all tracks for an album identified by a representative track ID
func (uc *ListTracksByAlbumIDUseCase) Execute(ctx context.Context, albumTrackID int64) (ListTracksResponse, error) {
	// Get the representative track to find the album name
	track, err := uc.repo.GetMusicTrackByID(ctx, albumTrackID)
	if err != nil {
		return ListTracksResponse{}, fmt.Errorf("failed to get album track: %w", err)
	}

	if track.Album == "" {
		return ListTracksResponse{}, fmt.Errorf("track %d has no album information", albumTrackID)
	}

	// Get all tracks for this album
	tracks, err := uc.repo.ListMusicTracksByAlbum(ctx, track.LibraryID, track.Album)
	if err != nil {
		return ListTracksResponse{}, fmt.Errorf("failed to list tracks for album %s: %w", track.Album, err)
	}

	return ToListTracksResponse(tracks), nil
}
