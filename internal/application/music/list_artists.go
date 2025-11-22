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

// Execute retrieves all artists in a library with track and album counts
// Uses efficient database aggregation instead of loading all tracks into memory
func (uc *ListArtistsUseCase) Execute(ctx context.Context, libraryID int64) (ListArtistsResponse, error) {
	// Get total count
	total, err := uc.repo.CountArtistsByLibrary(ctx, libraryID)
	if err != nil {
		return ListArtistsResponse{}, fmt.Errorf("failed to count artists: %w", err)
	}

	// Get all artists without pagination (use very large limit)
	pagination := &common.PaginationParams{
		Limit:  int(total), // Get all artists
		Offset: 0,
	}

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
		Artists: responses,
		Total:   len(responses),
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
