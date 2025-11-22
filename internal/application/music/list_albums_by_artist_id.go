package music

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/domain/media"
)

// ListAlbumsByArtistIDUseCase handles listing albums by artist ID
type ListAlbumsByArtistIDUseCase struct {
	repo media.MusicRepository
}

// NewListAlbumsByArtistIDUseCase creates a new instance
func NewListAlbumsByArtistIDUseCase(repo media.MusicRepository) *ListAlbumsByArtistIDUseCase {
	return &ListAlbumsByArtistIDUseCase{
		repo: repo,
	}
}

// Execute retrieves all albums for an artist by artist entity ID
func (uc *ListAlbumsByArtistIDUseCase) Execute(ctx context.Context, artistID int64) (ListAlbumsResponse, error) {
	artist, err := uc.repo.GetArtistByID(ctx, artistID)
	if err != nil {
		return ListAlbumsResponse{}, fmt.Errorf("failed to get artist: %w", err)
	}

	albums, err := uc.repo.ListAlbumsByLibrary(ctx, artist.LibraryID)
	if err != nil {
		return ListAlbumsResponse{}, fmt.Errorf("failed to list albums for artist %s: %w", artist.Name, err)
	}

	// Filter albums by artist_id and convert to response format
	var albumSummaries []AlbumSummary
	for _, album := range albums {
		if album.ArtistID == artist.ID {
			albumSummaries = append(albumSummaries, AlbumSummary{
				ID:         album.ID,
				Album:      album.Title,
				Artist:     album.AlbumArtist,
				Year:       album.Year,
				TrackCount: album.TotalTracks,
			})
		}
	}

	return ListAlbumsResponse{
		Albums: albumSummaries,
		Total:  len(albumSummaries),
	}, nil
}
