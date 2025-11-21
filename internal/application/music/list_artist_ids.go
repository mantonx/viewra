package music

import (
	"context"
	"fmt"

	"github.com/viewra/viewra/internal/domain/common"
	"github.com/viewra/viewra/internal/domain/media"
)

// ListArtistIDsUseCase handles the business logic for listing artist IDs only
type ListArtistIDsUseCase struct {
	repo media.MusicRepository
}

// NewListArtistIDsUseCase creates a new instance of ListArtistIDsUseCase
func NewListArtistIDsUseCase(repo media.MusicRepository) *ListArtistIDsUseCase {
	return &ListArtistIDsUseCase{
		repo: repo,
	}
}

// Execute retrieves artist representative IDs in a library with pagination
func (uc *ListArtistIDsUseCase) Execute(ctx context.Context, libraryID int64, pagination *common.PaginationParams) (ListIDsResponse, error) {
	if pagination == nil {
		pagination = common.DefaultPaginationParams()
	}

	// Get total count
	total, err := uc.repo.CountArtistsByLibrary(ctx, libraryID)
	if err != nil {
		return ListIDsResponse{}, fmt.Errorf("failed to count artists: %w", err)
	}

	// Get paginated IDs
	ids, err := uc.repo.ListArtistIDsByLibraryPaginated(ctx, libraryID, pagination)
	if err != nil {
		return ListIDsResponse{}, fmt.Errorf("failed to list artist IDs: %w", err)
	}

	return ListIDsResponse{
		IDs:        ids,
		Total:      int(total),
		Pagination: common.NewPaginationMetadata(total, pagination),
	}, nil
}
