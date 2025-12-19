package tv

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/tvshow"
)

// ListTVShowsUseCase handles the business logic for listing TV shows with aggregated data
type ListTVShowsUseCase struct {
	repo *tvshow.Repository
}

// NewListTVShowsUseCase creates a new instance of ListTVShowsUseCase
func NewListTVShowsUseCase(repo *tvshow.Repository) *ListTVShowsUseCase {
	return &ListTVShowsUseCase{
		repo: repo,
	}
}

// Execute retrieves all TV shows in a library with episode and season counts
// Uses efficient database aggregation instead of N+1 queries
func (uc *ListTVShowsUseCase) Execute(ctx context.Context, libraryID int64) (ListTVShowsResponse, error) {
	// Get total count
	total, err := uc.repo.CountTVShowsByLibrary(ctx, libraryID)
	if err != nil {
		return ListTVShowsResponse{}, fmt.Errorf("failed to count TV shows: %w", err)
	}

	// Get all shows without pagination (use very large limit)
	pagination := &common.PaginationParams{
		Limit:  int(total), // Get all shows
		Offset: 0,
	}

	shows, err := uc.repo.ListTVShowsByLibraryPaginated(ctx, libraryID, pagination)
	if err != nil {
		return ListTVShowsResponse{}, fmt.Errorf("failed to list TV shows: %w", err)
	}

	// Convert to response
	responses := make([]TVShowSummary, len(shows))
	for i, show := range shows {
		responses[i] = TVShowSummary{
			ID:            show.ID,
			LibraryID:     show.LibraryID,
			Title:         show.Title,
			Year:          show.Year,
			Genre:         show.Genre,
			Plot:          show.Plot,
			IMDbID:        show.IMDbID,
			TMDbID:        show.TMDbID,
			ContentRating: show.ContentRating,
			SeasonCount:   int(show.SeasonCount),
			EpisodeCount:  int(show.EpisodeCount),
			CreatedAt:     show.CreatedAt,
		}
	}

	return ListTVShowsResponse{
		Shows: responses,
		Total: len(responses),
	}, nil
}

// ExecuteWithPagination retrieves TV shows in a library with pagination
func (uc *ListTVShowsUseCase) ExecuteWithPagination(ctx context.Context, libraryID int64, pagination *common.PaginationParams) (ListTVShowsResponse, error) {
	if pagination == nil {
		pagination = common.DefaultPaginationParams()
	}

	// Get total count
	total, err := uc.repo.CountTVShowsByLibrary(ctx, libraryID)
	if err != nil {
		return ListTVShowsResponse{}, fmt.Errorf("failed to count TV shows: %w", err)
	}

	// Get paginated results with counts
	shows, err := uc.repo.ListTVShowsByLibraryPaginated(ctx, libraryID, pagination)
	if err != nil {
		return ListTVShowsResponse{}, fmt.Errorf("failed to list TV shows: %w", err)
	}

	// Convert to response
	responses := make([]TVShowSummary, len(shows))
	for i, show := range shows {
		responses[i] = TVShowSummary{
			ID:            show.ID,
			LibraryID:     show.LibraryID,
			Title:         show.Title,
			Year:          show.Year,
			Genre:         show.Genre,
			Plot:          show.Plot,
			IMDbID:        show.IMDbID,
			TMDbID:        show.TMDbID,
			ContentRating: show.ContentRating,
			SeasonCount:   int(show.SeasonCount),
			EpisodeCount:  int(show.EpisodeCount),
			CreatedAt:     show.CreatedAt,
		}
	}

	return ListTVShowsResponse{
		Shows:      responses,
		Total:      len(responses),
		Pagination: common.NewPaginationMetadata(total, pagination),
	}, nil
}

// ExecuteWithSearch searches for TV shows by title with pagination
func (uc *ListTVShowsUseCase) ExecuteWithSearch(ctx context.Context, libraryID int64, query string, pagination *common.PaginationParams) (ListTVShowsResponse, error) {
	if pagination == nil {
		pagination = common.DefaultPaginationParams()
	}

	// Get total count of matching shows
	total, err := uc.repo.CountSearchTVShowsByTitle(ctx, libraryID, query)
	if err != nil {
		return ListTVShowsResponse{}, fmt.Errorf("failed to count TV shows: %w", err)
	}

	// Get paginated search results with counts
	shows, err := uc.repo.SearchTVShowsWithCountsByTitlePaginated(ctx, libraryID, query, pagination)
	if err != nil {
		return ListTVShowsResponse{}, fmt.Errorf("failed to search TV shows: %w", err)
	}

	// Convert to response
	responses := make([]TVShowSummary, len(shows))
	for i, show := range shows {
		responses[i] = TVShowSummary{
			ID:            show.ID,
			LibraryID:     show.LibraryID,
			Title:         show.Title,
			Year:          show.Year,
			Genre:         show.Genre,
			Plot:          show.Plot,
			IMDbID:        show.IMDbID,
			TMDbID:        show.TMDbID,
			ContentRating: show.ContentRating,
			SeasonCount:   int(show.SeasonCount),
			EpisodeCount:  int(show.EpisodeCount),
			CreatedAt:     show.CreatedAt,
		}
	}

	return ListTVShowsResponse{
		Shows:      responses,
		Total:      len(responses),
		Pagination: common.NewPaginationMetadata(total, pagination),
	}, nil
}
