package movies

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/media"
)

// ListMoviesUseCase handles the business logic for listing movies
type ListMoviesUseCase struct {
	repo media.MovieRepository
}

// NewListMoviesUseCase creates a new instance of ListMoviesUseCase
func NewListMoviesUseCase(repo media.MovieRepository) *ListMoviesUseCase {
	return &ListMoviesUseCase{
		repo: repo,
	}
}

// Execute retrieves all movies in a library
func (uc *ListMoviesUseCase) Execute(ctx context.Context, libraryID int64) (ListMoviesResponse, error) {
	movies, err := uc.repo.ListMoviesByLibrary(ctx, libraryID)
	if err != nil {
		return ListMoviesResponse{}, fmt.Errorf("failed to list movies: %w", err)
	}

	return ToListMoviesResponse(movies), nil
}

// ExecuteWithPagination retrieves movies in a library with pagination
func (uc *ListMoviesUseCase) ExecuteWithPagination(ctx context.Context, libraryID int64, pagination *common.PaginationParams) (ListMoviesResponse, error) {
	if pagination == nil {
		pagination = common.DefaultPaginationParams()
	}

	// Get total count
	total, err := uc.repo.CountMoviesByLibrary(ctx, libraryID)
	if err != nil {
		return ListMoviesResponse{}, fmt.Errorf("failed to count movies: %w", err)
	}

	// Get paginated results
	movies, err := uc.repo.ListMoviesByLibraryPaginated(ctx, libraryID, pagination)
	if err != nil {
		return ListMoviesResponse{}, fmt.Errorf("failed to list movies: %w", err)
	}

	return ToListMoviesResponseWithPagination(movies, total, pagination), nil
}
