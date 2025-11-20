package music

import (
	"context"
	"fmt"

	"github.com/viewra/viewra/internal/domain/common"
	"github.com/viewra/viewra/internal/domain/media"
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

// Execute retrieves all artists in a library with track and album counts
// This aggregates data by artist name (from either artist or album_artist fields)
func (uc *ListArtistsUseCase) Execute(ctx context.Context, libraryID int64) (ListArtistsResponse, error) {
	// Get all tracks in the library
	tracks, err := uc.repo.ListMusicTracksByLibrary(ctx, libraryID)
	if err != nil {
		return ListArtistsResponse{}, fmt.Errorf("failed to list music tracks: %w", err)
	}

	// Aggregate by artist
	artistMap := make(map[string]*ArtistSummary)
	for _, track := range tracks {
		// Use album_artist if available, otherwise use artist
		artistName := track.AlbumArtist
		if artistName == "" {
			artistName = track.Artist
		}
		if artistName == "" {
			continue // Skip tracks with no artist information
		}

		if _, exists := artistMap[artistName]; !exists {
			artistMap[artistName] = &ArtistSummary{
				ID:         track.ID, // Use first track's media_id as representative ID
				Name:       artistName,
				Albums:     make(map[string]bool),
				TrackCount: 0,
			}
		}

		artistMap[artistName].TrackCount++
		if track.Album != "" {
			artistMap[artistName].Albums[track.Album] = true
		}
	}

	// Convert map to slice
	artists := make([]ArtistSummary, 0, len(artistMap))
	for _, artist := range artistMap {
		artist.AlbumCount = len(artist.Albums)
		artist.Albums = nil // Remove the map from the response
		artists = append(artists, *artist)
	}

	return ListArtistsResponse{
		Artists: artists,
		Total:   len(artists),
	}, nil
}

// ExecuteWithPagination retrieves artists in a library with pagination
func (uc *ListArtistsUseCase) ExecuteWithPagination(ctx context.Context, libraryID int64, pagination *common.PaginationParams) (ListArtistsResponse, error) {
	if pagination == nil {
		pagination = common.DefaultPaginationParams()
	}

	// Get total count
	total, err := uc.repo.CountArtistsByLibrary(ctx, libraryID)
	if err != nil {
		return ListArtistsResponse{}, fmt.Errorf("failed to count artists: %w", err)
	}

	// Get paginated results with counts
	artists, err := uc.repo.ListArtistsByLibraryPaginated(ctx, libraryID, pagination)
	if err != nil {
		return ListArtistsResponse{}, fmt.Errorf("failed to list artists: %w", err)
	}

	// Convert to response
	responses := make([]ArtistSummary, len(artists))
	for i, artist := range artists {
		responses[i] = ArtistSummary{
			ID:         artist.RepresentativeID,
			Name:       artist.Artist,
			AlbumCount: int(artist.AlbumCount),
			TrackCount: int(artist.TrackCount),
		}
	}

	return ListArtistsResponse{
		Artists:    responses,
		Total:      len(responses),
		Pagination: common.NewPaginationMetadata(total, pagination),
	}, nil
}
