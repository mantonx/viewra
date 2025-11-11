package library

import (
	"context"
	"fmt"

	"github.com/viewra/viewra/internal/domain/library"
)

// GetLibraryUseCase handles the business logic for retrieving a single library
type GetLibraryUseCase struct {
	repo library.Repository
}

// NewGetLibraryUseCase creates a new instance of GetLibraryUseCase
func NewGetLibraryUseCase(repo library.Repository) *GetLibraryUseCase {
	return &GetLibraryUseCase{
		repo: repo,
	}
}

// Execute retrieves a library by its ID
func (uc *GetLibraryUseCase) Execute(ctx context.Context, id int64) (LibraryResponse, error) {
	lib, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return LibraryResponse{}, fmt.Errorf("failed to get library: %w", err)
	}

	return ToLibraryResponse(lib), nil
}
