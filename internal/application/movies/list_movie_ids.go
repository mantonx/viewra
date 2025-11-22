package movies

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/media"
)

// ListMovieIDsUseCase handles the business logic for listing movie IDs only
type ListMovieIDsUseCase struct {
	repo media.MovieRepository
}

// NewListMovieIDsUseCase creates a new instance of ListMovieIDsUseCase
func NewListMovieIDsUseCase(repo media.MovieRepository) *ListMovieIDsUseCase {
	return &ListMovieIDsUseCase{
		repo: repo,
	}
}

// Execute retrieves movie IDs in a library with pagination
func (uc *ListMovieIDsUseCase) Execute(ctx context.Context, libraryID int64, pagination *common.PaginationParams) (ListIDsResponse, error) {
	if pagination == nil {
		pagination = common.DefaultPaginationParams()
	}

	// Get total count
	total, err := uc.repo.CountMoviesByLibrary(ctx, libraryID)
	if err != nil {
		return ListIDsResponse{}, fmt.Errorf("failed to count movies: %w", err)
	}

	// Get paginated IDs
	ids, err := uc.repo.ListMovieIDsByLibraryPaginated(ctx, libraryID, pagination)
	if err != nil {
		return ListIDsResponse{}, fmt.Errorf("failed to list movie IDs: %w", err)
	}

	return ListIDsResponse{
		IDs:        ids,
		Total:      int(total),
		Pagination: common.NewPaginationMetadata(total, pagination),
	}, nil
}
