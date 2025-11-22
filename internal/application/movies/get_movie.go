package movies

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/domain/media"
)

// GetMovieUseCase handles the business logic for getting a single movie
type GetMovieUseCase struct {
	repo media.MovieRepository
}

// NewGetMovieUseCase creates a new instance of GetMovieUseCase
func NewGetMovieUseCase(repo media.MovieRepository) *GetMovieUseCase {
	return &GetMovieUseCase{
		repo: repo,
	}
}

// Execute retrieves a movie by its ID
func (uc *GetMovieUseCase) Execute(ctx context.Context, id int64) (*MovieResponse, error) {
	movie, err := uc.repo.GetMovieByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get movie: %w", err)
	}

	response := ToMovieResponse(movie)
	return &response, nil
}
