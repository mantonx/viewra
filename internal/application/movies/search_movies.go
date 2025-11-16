package movies

import (
	"context"
	"fmt"

	"github.com/viewra/viewra/internal/domain/media"
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
