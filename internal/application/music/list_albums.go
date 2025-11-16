package music

import (
	"context"
	"fmt"

	"github.com/viewra/viewra/internal/domain/media"
)

// ListAlbumsUseCase handles the business logic for listing music albums
type ListAlbumsUseCase struct {
	repo media.MusicRepository
}

// NewListAlbumsUseCase creates a new instance of ListAlbumsUseCase
func NewListAlbumsUseCase(repo media.MusicRepository) *ListAlbumsUseCase {
	return &ListAlbumsUseCase{
		repo: repo,
	}
}

// ExecuteByArtist retrieves all albums for a specific artist
func (uc *ListAlbumsUseCase) ExecuteByArtist(ctx context.Context, libraryID int64, artist string) (ListAlbumsResponse, error) {
	// Get all tracks by this artist
	tracks, err := uc.repo.ListMusicTracksByArtist(ctx, libraryID, artist)
	if err != nil {
		return ListAlbumsResponse{}, fmt.Errorf("failed to list tracks for artist %s: %w", artist, err)
	}

	// Aggregate by album
	albumMap := make(map[string]*AlbumSummary)
	for _, track := range tracks {
		if track.Album == "" {
			continue // Skip tracks with no album
		}

		if _, exists := albumMap[track.Album]; !exists {
			albumMap[track.Album] = &AlbumSummary{
				Album:      track.Album,
				Artist:     track.AlbumArtist,
				Year:       track.Year,
				TrackCount: 0,
			}
			if albumMap[track.Album].Artist == "" {
				albumMap[track.Album].Artist = track.Artist
			}
		}

		albumMap[track.Album].TrackCount++
	}

	// Convert map to slice
	albums := make([]AlbumSummary, 0, len(albumMap))
	for _, album := range albumMap {
		albums = append(albums, *album)
	}

	return ListAlbumsResponse{
		Albums: albums,
		Total:  len(albums),
	}, nil
}
