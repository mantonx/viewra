package library

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/mantonx/viewra/internal/application/common"
	"github.com/mantonx/viewra/internal/domain/library"
)

// CreateLibraryUseCase handles the business logic for creating a new library
type CreateLibraryUseCase struct {
	repo      library.Repository
	txManager *common.TxManager
}

// NewCreateLibraryUseCase creates a new instance of CreateLibraryUseCase
func NewCreateLibraryUseCase(repo library.Repository, txManager *common.TxManager) *CreateLibraryUseCase {
	return &CreateLibraryUseCase{
		repo:      repo,
		txManager: txManager,
	}
}

// Execute creates a new library with the given request data
func (uc *CreateLibraryUseCase) Execute(ctx context.Context, req CreateLibraryRequest) (LibraryResponse, error) {
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
		return LibraryResponse{}, fmt.Errorf("validation failed: %w", err)
	}

	// Check if path exists on filesystem (before transaction)
	if _, err := os.Stat(lib.Path); os.IsNotExist(err) {
		return LibraryResponse{}, library.ErrPathNotFound
	}

	// Execute within transaction to ensure atomicity of check + create
	err := uc.txManager.WithTransaction(ctx, func(tx *sql.Tx) error {
		// Check if library with this path already exists
		exists, err := uc.repo.ExistsWithTx(ctx, tx, lib.Path)
		if err != nil {
			return fmt.Errorf("failed to check library existence: %w", err)
		}
		if exists {
			return library.ErrDuplicatePath
		}

		// Create the library
		if err := uc.repo.CreateWithTx(ctx, tx, lib); err != nil {
			return fmt.Errorf("failed to create library: %w", err)
		}

		return nil
	})

	if err != nil {
		return LibraryResponse{}, err
	}

	return ToLibraryResponse(lib), nil
}
