package music

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/domain/common"
	musicRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/music"
)

// OptimizedMusicRepository defines the interface for optimized music queries
type OptimizedMusicRepository interface {
	CountArtistsByLibrary(ctx context.Context, libraryID int64) (int64, error)
	GetArtistsWithCountsByLibraryPaginated(ctx context.Context, libraryID int64, pagination *common.PaginationParams) ([]musicRepo.ArtistWithCounts, error)
	CountSearchArtistsByName(ctx context.Context, libraryID int64, query string) (int64, error)
	SearchArtistsWithCountsByNamePaginated(ctx context.Context, libraryID int64, query string, pagination *common.PaginationParams) ([]musicRepo.ArtistWithCounts, error)
}

// ListArtistsUseCase handles the business logic for listing music artists with aggregated data
type ListArtistsUseCase struct {
	repo OptimizedMusicRepository
}

// NewListArtistsUseCase creates a new instance of ListArtistsUseCase
func NewListArtistsUseCase(repo OptimizedMusicRepository) *ListArtistsUseCase {
	return &ListArtistsUseCase{
		repo: repo,
	}
}

// Execute retrieves all artists in a library (uses paginated query with high limit)
func (uc *ListArtistsUseCase) Execute(ctx context.Context, libraryID int64) (ListArtistsResponse, error) {
	// Get total count first
	total, err := uc.repo.CountArtistsByLibrary(ctx, libraryID)
	if err != nil {
		return ListArtistsResponse{}, fmt.Errorf("failed to count artists: %w", err)
	}

	// Use the optimized query with a high limit to get all
	pagination := &common.PaginationParams{
		Limit:  int(total) + 100, // Add buffer to ensure we get all
		Offset: 0,
	}

	artists, err := uc.repo.GetArtistsWithCountsByLibraryPaginated(ctx, libraryID, pagination)
	if err != nil {
		return ListArtistsResponse{}, fmt.Errorf("failed to list artists: %w", err)
	}

	// Convert to response
	responses := make([]ArtistSummary, len(artists))
	for i, artist := range artists {
		responses[i] = ArtistSummary{
			ID:         artist.ID,
			Name:       artist.Name,
			AlbumCount: artist.AlbumCount,
			TrackCount: artist.TrackCount,
		}
	}

	return ListArtistsResponse{
		Artists: responses,
		Total:   len(responses),
	}, nil
}

// ExecuteWithPagination retrieves artists in a library with proper database-level pagination
func (uc *ListArtistsUseCase) ExecuteWithPagination(ctx context.Context, libraryID int64, pagination *common.PaginationParams) (ListArtistsResponse, error) {
	if pagination == nil {
		pagination = common.DefaultPaginationParams()
	}

	// Get total count
	total, err := uc.repo.CountArtistsByLibrary(ctx, libraryID)
	if err != nil {
		return ListArtistsResponse{}, fmt.Errorf("failed to count artists: %w", err)
	}

	// Get paginated artists with counts using optimized query
	artists, err := uc.repo.GetArtistsWithCountsByLibraryPaginated(ctx, libraryID, pagination)
	if err != nil {
		return ListArtistsResponse{}, fmt.Errorf("failed to list artists: %w", err)
	}

	// Convert to response
	responses := make([]ArtistSummary, len(artists))
	for i, artist := range artists {
		responses[i] = ArtistSummary{
			ID:         artist.ID,
			Name:       artist.Name,
			AlbumCount: artist.AlbumCount,
			TrackCount: artist.TrackCount,
		}
	}

	return ListArtistsResponse{
		Artists:    responses,
		Total:      len(responses),
		Pagination: common.NewPaginationMetadata(total, pagination),
	}, nil
}

// ExecuteWithSearch searches for artists by name with pagination
func (uc *ListArtistsUseCase) ExecuteWithSearch(ctx context.Context, libraryID int64, query string, pagination *common.PaginationParams) (ListArtistsResponse, error) {
	if pagination == nil {
		pagination = common.DefaultPaginationParams()
	}

	// Get total count of matching artists
	total, err := uc.repo.CountSearchArtistsByName(ctx, libraryID, query)
	if err != nil {
		return ListArtistsResponse{}, fmt.Errorf("failed to count artists: %w", err)
	}

	// Get paginated search results with counts using optimized query
	artists, err := uc.repo.SearchArtistsWithCountsByNamePaginated(ctx, libraryID, query, pagination)
	if err != nil {
		return ListArtistsResponse{}, fmt.Errorf("failed to search artists: %w", err)
	}

	// Convert to response
	responses := make([]ArtistSummary, len(artists))
	for i, artist := range artists {
		responses[i] = ArtistSummary{
			ID:         artist.ID,
			Name:       artist.Name,
			AlbumCount: artist.AlbumCount,
			TrackCount: artist.TrackCount,
		}
	}

	return ListArtistsResponse{
		Artists:    responses,
		Total:      len(responses),
		Pagination: common.NewPaginationMetadata(total, pagination),
	}, nil
}
