package library

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/viewra/viewra/internal/domain/library"
)

// CreateLibraryUseCase handles the business logic for creating a new library
type CreateLibraryUseCase struct {
	repo library.Repository
}

// NewCreateLibraryUseCase creates a new instance of CreateLibraryUseCase
func NewCreateLibraryUseCase(repo library.Repository) *CreateLibraryUseCase {
	return &CreateLibraryUseCase{
		repo: repo,
	}
}

// Execute creates a new library with the given request data
func (uc *CreateLibraryUseCase) Execute(ctx context.Context, req CreateLibraryRequest) (CreateLibraryResponse, error) {
	// Convert DTO to domain entity
	lib := &library.Library{
		Name:      req.Name,
		Path:      req.Path,
		Type:      library.LibraryType(req.Type),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Validate domain entity
	if err := lib.IsValid(); err != nil {
		return CreateLibraryResponse{}, fmt.Errorf("validation failed: %w", err)
	}

	// Check if path exists on filesystem
	if _, err := os.Stat(lib.Path); os.IsNotExist(err) {
		return CreateLibraryResponse{}, library.ErrPathNotFound
	}

	// Check if library with this path already exists
	exists, err := uc.repo.Exists(ctx, lib.Path)
	if err != nil {
		return CreateLibraryResponse{}, fmt.Errorf("failed to check library existence: %w", err)
	}
	if exists {
		return CreateLibraryResponse{}, library.ErrDuplicatePath
	}

	// Create the library
	if err := uc.repo.Create(ctx, lib); err != nil {
		return CreateLibraryResponse{}, fmt.Errorf("failed to create library: %w", err)
	}

	return ToCreateLibraryResponse(lib), nil
}
