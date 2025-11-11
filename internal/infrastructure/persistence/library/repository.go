package library

import (
	"context"
	"database/sql"
	"errors"

	"github.com/viewra/viewra/internal/domain/library"
	"github.com/viewra/viewra/internal/infrastructure/database/sqlc"
	"github.com/viewra/viewra/internal/infrastructure/persistence/common"
)

// Repository implements the library.Repository interface using sqlc
type Repository struct {
	queries *sqlc.Queries
}

// NewRepository creates a new library repository
func NewRepository(db *sql.DB) *Repository {
	return &Repository{
		queries: sqlc.New(db),
	}
}

// Create adds a new library to the database
func (r *Repository) Create(ctx context.Context, lib *library.Library) error {
	result, err := r.queries.CreateLibrary(ctx, sqlc.CreateLibraryParams{
		Name: lib.Name,
		Path: lib.Path,
		Type: string(lib.Type),
	})
	if err != nil {
		return err
	}

	// Update the library with generated values
	lib.ID = result.ID
	lib.CreatedAt = common.ParseTimeInterface(result.CreatedAt)
	lib.UpdatedAt = common.ParseTimeInterface(result.UpdatedAt)

	return nil
}

// GetByID retrieves a library by its ID
func (r *Repository) GetByID(ctx context.Context, id int64) (*library.Library, error) {
	result, err := r.queries.GetLibraryByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, library.ErrLibraryNotFound
		}
		return nil, err
	}

	return &library.Library{
		ID:        result.ID,
		Name:      result.Name,
		Path:      result.Path,
		Type:      library.LibraryType(result.Type),
		CreatedAt: common.ParseTimeInterface(result.CreatedAt),
		UpdatedAt: common.ParseTimeInterface(result.UpdatedAt),
	}, nil
}

// GetByPath retrieves a library by its path
func (r *Repository) GetByPath(ctx context.Context, path string) (*library.Library, error) {
	result, err := r.queries.GetLibraryByPath(ctx, path)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, library.ErrLibraryNotFound
		}
		return nil, err
	}

	return &library.Library{
		ID:        result.ID,
		Name:      result.Name,
		Path:      result.Path,
		Type:      library.LibraryType(result.Type),
		CreatedAt: common.ParseTimeInterface(result.CreatedAt),
		UpdatedAt: common.ParseTimeInterface(result.UpdatedAt),
	}, nil
}

// List retrieves all libraries
func (r *Repository) List(ctx context.Context) ([]*library.Library, error) {
	results, err := r.queries.ListLibraries(ctx)
	if err != nil {
		return nil, err
	}

	libraries := make([]*library.Library, len(results))
	for i, result := range results {
		libraries[i] = &library.Library{
			ID:        result.ID,
			Name:      result.Name,
			Path:      result.Path,
			Type:      library.LibraryType(result.Type),
			CreatedAt: common.ParseTimeInterface(result.CreatedAt),
			UpdatedAt: common.ParseTimeInterface(result.UpdatedAt),
		}
	}

	return libraries, nil
}

// Update modifies an existing library
func (r *Repository) Update(ctx context.Context, lib *library.Library) error {
	result, err := r.queries.UpdateLibrary(ctx, sqlc.UpdateLibraryParams{
		Name: lib.Name,
		Path: lib.Path,
		Type: string(lib.Type),
		ID:   lib.ID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.ErrLibraryNotFound
		}
		return err
	}

	// Update timestamps
	lib.UpdatedAt = common.ParseTimeInterface(result.UpdatedAt)

	return nil
}

// Delete removes a library from the database
func (r *Repository) Delete(ctx context.Context, id int64) error {
	return r.queries.DeleteLibrary(ctx, id)
}

// Exists checks if a library with the given path already exists
func (r *Repository) Exists(ctx context.Context, path string) (bool, error) {
	count, err := r.queries.LibraryExistsByPath(ctx, path)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
