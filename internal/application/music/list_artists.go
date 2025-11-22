package music

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/media"
)

// ListArtistsUseCase handles the business logic for listing music artists with aggregated data
type ListArtistsUseCase struct {
	repo media.MusicRepository
}

// NewListArtistsUseCase creates a new instance of ListArtistsUseCase
func NewListArtistsUseCase(repo media.MusicRepository) *ListArtistsUseCase {
	return &ListArtistsUseCase{
		repo: repo,
	}
}

// Execute retrieves all artists in a library
func (uc *ListArtistsUseCase) Execute(ctx context.Context, libraryID int64) (ListArtistsResponse, error) {
	// Get all artist entities from music_artists table
	artists, err := uc.repo.ListArtistsByLibrary(ctx, libraryID)
	if err != nil {
		return ListArtistsResponse{}, fmt.Errorf("failed to list artists: %w", err)
	}

	// Get all albums and tracks to count per artist
	albums, err := uc.repo.ListAlbumsByLibrary(ctx, libraryID)
	if err != nil {
		return ListArtistsResponse{}, fmt.Errorf("failed to list albums: %w", err)
	}

	tracks, err := uc.repo.ListMusicTracksByLibrary(ctx, libraryID)
	if err != nil {
		return ListArtistsResponse{}, fmt.Errorf("failed to list tracks: %w", err)
	}

	// Count albums and tracks per artist
	albumCounts := make(map[int64]int)
	trackCounts := make(map[int64]int)

	for _, album := range albums {
		if album.ArtistID > 0 {
			albumCounts[album.ArtistID]++
		}
	}

	for _, track := range tracks {
		if track.ArtistID > 0 {
			trackCounts[track.ArtistID]++
		}
	}

	// Convert to response
	responses := make([]ArtistSummary, len(artists))
	for i, artist := range artists {
		responses[i] = ArtistSummary{
			ID:         artist.ID,
			Name:       artist.Name,
			AlbumCount: albumCounts[artist.ID],
			TrackCount: trackCounts[artist.ID],
		}
	}

	return ListArtistsResponse{
		Artists: responses,
		Total:   len(responses),
	}, nil
}

// ExecuteWithPagination retrieves artists in a library with pagination
// For now, this doesn't paginate properly - it gets all artists and counts
// TODO: Optimize with database-level pagination and aggregation
func (uc *ListArtistsUseCase) ExecuteWithPagination(ctx context.Context, libraryID int64, pagination *common.PaginationParams) (ListArtistsResponse, error) {
	if pagination == nil {
		pagination = common.DefaultPaginationParams()
	}

	// For now, just call Execute and return all results
	// TODO: Add proper pagination at database level
	resp, err := uc.Execute(ctx, libraryID)
	if err != nil {
		return ListArtistsResponse{}, err
	}

	// Add pagination metadata
	total := int64(len(resp.Artists))
	resp.Pagination = common.NewPaginationMetadata(total, pagination)

	return resp, nil
}
