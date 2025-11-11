package library

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/viewra/viewra/internal/domain/library"
)

// UpdateLibraryUseCase handles the business logic for updating a library
type UpdateLibraryUseCase struct {
	repo library.Repository
}

// NewUpdateLibraryUseCase creates a new instance of UpdateLibraryUseCase
func NewUpdateLibraryUseCase(repo library.Repository) *UpdateLibraryUseCase {
	return &UpdateLibraryUseCase{
		repo: repo,
	}
}

// Execute updates an existing library with the given request data
func (uc *UpdateLibraryUseCase) Execute(ctx context.Context, id int64, req UpdateLibraryRequest) (UpdateLibraryResponse, error) {
	// Get existing library
	lib, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return UpdateLibraryResponse{}, fmt.Errorf("failed to get library: %w", err)
	}

	// Update fields if provided
	if req.Name != "" {
		lib.Name = req.Name
	}
	if req.Path != "" {
		lib.Path = req.Path
	}
	if req.Type != "" {
		lib.Type = library.LibraryType(req.Type)
	}

	lib.UpdatedAt = time.Now()

	// Validate domain entity
	if err := lib.IsValid(); err != nil {
		return UpdateLibraryResponse{}, fmt.Errorf("validation failed: %w", err)
	}

	// Check if path exists on filesystem (if path was updated)
	if req.Path != "" {
		if _, err := os.Stat(lib.Path); os.IsNotExist(err) {
			return UpdateLibraryResponse{}, library.ErrPathNotFound
		}

		// Check if another library with this path already exists
		existing, err := uc.repo.GetByPath(ctx, lib.Path)
		if err != nil && err != library.ErrLibraryNotFound {
			return UpdateLibraryResponse{}, fmt.Errorf("failed to check library path: %w", err)
		}
		if existing != nil && existing.ID != id {
			return UpdateLibraryResponse{}, library.ErrDuplicatePath
		}
	}

	// Update the library
	if err := uc.repo.Update(ctx, lib); err != nil {
		return UpdateLibraryResponse{}, fmt.Errorf("failed to update library: %w", err)
	}

	return ToUpdateLibraryResponse(lib), nil
}
