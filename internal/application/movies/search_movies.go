package movies

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/media"
)

// SearchMoviesUseCase handles the business logic for searching movies
type SearchMoviesUseCase struct {
	repo media.MovieRepository
}

// NewSearchMoviesUseCase creates a new instance of SearchMoviesUseCase
func NewSearchMoviesUseCase(repo media.MovieRepository) *SearchMoviesUseCase {
	return &SearchMoviesUseCase{
		repo: repo,
	}
}

// Execute searches for movies by title
func (uc *SearchMoviesUseCase) Execute(ctx context.Context, libraryID int64, query string) (ListMoviesResponse, error) {
	if query == "" {
		return ListMoviesResponse{}, fmt.Errorf("search query cannot be empty")
	}

	movies, err := uc.repo.SearchMovies(ctx, libraryID, query)
	if err != nil {
		return ListMoviesResponse{}, fmt.Errorf("failed to search movies: %w", err)
	}

	return ToListMoviesResponse(movies), nil
}

// ExecuteWithPagination searches for movies by title with pagination
func (uc *SearchMoviesUseCase) ExecuteWithPagination(ctx context.Context, libraryID int64, query string, pagination *common.PaginationParams) (ListMoviesResponse, error) {
	if query == "" {
		return ListMoviesResponse{}, fmt.Errorf("search query cannot be empty")
	}

	if pagination == nil {
		pagination = common.DefaultPaginationParams()
	}

	// Get total count
	total, err := uc.repo.CountSearchMoviesByTitle(ctx, libraryID, query)
	if err != nil {
		return ListMoviesResponse{}, fmt.Errorf("failed to count search results: %w", err)
	}

	// Get paginated results
	movies, err := uc.repo.SearchMoviesByTitlePaginated(ctx, libraryID, query, pagination)
	if err != nil {
		return ListMoviesResponse{}, fmt.Errorf("failed to search movies: %w", err)
	}

	return ToListMoviesResponseWithPagination(movies, total, pagination), nil
}
