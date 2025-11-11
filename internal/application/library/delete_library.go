package library

import (
	"context"
	"fmt"

	"github.com/viewra/viewra/internal/domain/library"
)

// DeleteLibraryUseCase handles the business logic for deleting a library
type DeleteLibraryUseCase struct {
	repo library.Repository
}

// NewDeleteLibraryUseCase creates a new instance of DeleteLibraryUseCase
func NewDeleteLibraryUseCase(repo library.Repository) *DeleteLibraryUseCase {
	return &DeleteLibraryUseCase{
		repo: repo,
	}
}

// Execute deletes a library by its ID
// Note: This only removes the library record, not the actual media files
func (uc *DeleteLibraryUseCase) Execute(ctx context.Context, id int64) error {
	// Verify library exists
	_, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get library: %w", err)
	}

	// Delete the library
	if err := uc.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete library: %w", err)
	}

	return nil
}
