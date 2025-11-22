package tv

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/media"
)

// ListTVShowIDsUseCase handles the business logic for listing TV show IDs only
type ListTVShowIDsUseCase struct {
	repo media.TVRepository
}

// NewListTVShowIDsUseCase creates a new instance of ListTVShowIDsUseCase
func NewListTVShowIDsUseCase(repo media.TVRepository) *ListTVShowIDsUseCase {
	return &ListTVShowIDsUseCase{
		repo: repo,
	}
}

// Execute retrieves TV show IDs in a library with pagination
func (uc *ListTVShowIDsUseCase) Execute(ctx context.Context, libraryID int64, pagination *common.PaginationParams) (ListIDsResponse, error) {
	if pagination == nil {
		pagination = common.DefaultPaginationParams()
	}

	// Get total count
	total, err := uc.repo.CountTVShowsByLibrary(ctx, libraryID)
	if err != nil {
		return ListIDsResponse{}, fmt.Errorf("failed to count TV shows: %w", err)
	}

	// Get paginated IDs
	ids, err := uc.repo.ListTVShowIDsByLibraryPaginated(ctx, libraryID, pagination)
	if err != nil {
		return ListIDsResponse{}, fmt.Errorf("failed to list TV show IDs: %w", err)
	}

	return ListIDsResponse{
		IDs:        ids,
		Total:      int(total),
		Pagination: common.NewPaginationMetadata(total, pagination),
	}, nil
}
