package movies

import (
	"context"
	"fmt"

	"github.com/viewra/viewra/internal/domain/media"
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
