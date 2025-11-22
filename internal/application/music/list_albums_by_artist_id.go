package music

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/domain/media"
)

// ListAlbumsByArtistIDUseCase handles listing albums using an artist's representative track ID
type ListAlbumsByArtistIDUseCase struct {
	repo media.MusicRepository
}

// NewListAlbumsByArtistIDUseCase creates a new instance
func NewListAlbumsByArtistIDUseCase(repo media.MusicRepository) *ListAlbumsByArtistIDUseCase {
	return &ListAlbumsByArtistIDUseCase{
		repo: repo,
	}
}

// Execute retrieves all albums for an artist identified by a representative track ID
func (uc *ListAlbumsByArtistIDUseCase) Execute(ctx context.Context, artistTrackID int64) (ListAlbumsResponse, error) {
	// Get the representative track to find the artist name
	track, err := uc.repo.GetMusicTrackByID(ctx, artistTrackID)
	if err != nil {
		return ListAlbumsResponse{}, fmt.Errorf("failed to get artist track: %w", err)
	}

	// Determine artist name (prefer album_artist)
	artistName := track.AlbumArtist
	if artistName == "" {
		artistName = track.Artist
	}
	if artistName == "" {
		return ListAlbumsResponse{}, fmt.Errorf("track %d has no artist information", artistTrackID)
	}

	// Get all tracks by this artist
	tracks, err := uc.repo.ListMusicTracksByArtist(ctx, track.LibraryID, artistName)
	if err != nil {
		return ListAlbumsResponse{}, fmt.Errorf("failed to list tracks for artist %s: %w", artistName, err)
	}

	// Aggregate by album
	albumMap := make(map[string]*AlbumSummary)
	for _, t := range tracks {
		if t.Album == "" {
			continue // Skip tracks with no album
		}

		if _, exists := albumMap[t.Album]; !exists {
			albumMap[t.Album] = &AlbumSummary{
				ID:         t.ID, // Use first track's media_id as representative ID
				Album:      t.Album,
				Artist:     t.AlbumArtist,
				Year:       t.Year,
				TrackCount: 0,
			}
			if albumMap[t.Album].Artist == "" {
				albumMap[t.Album].Artist = t.Artist
			}
		}

		albumMap[t.Album].TrackCount++
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
