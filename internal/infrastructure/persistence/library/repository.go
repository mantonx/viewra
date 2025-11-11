package library

import (
	"context"
	"database/sql"
	"errors"

	"github.com/viewra/viewra/internal/domain/library"
	"github.com/viewra/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/viewra/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/viewra/viewra/internal/infrastructure/persistence/adapters"
	"github.com/viewra/viewra/internal/infrastructure/persistence/common"
)

// NewRepository creates a new library repository with the appropriate database driver.
// The driver parameter should be "sqlite", "sqlite3", "postgres", or "postgresql".
func NewRepository(db *sql.DB, driver string) *Repository {
	r := &Repository{
		db:      db,
		dbType:  driver,
		adapter: adapters.NewTypeAdapter(),
		router:  common.NewQueryRouter(driver),
	}

	if common.IsPostgres(driver) {
		r.postgres = sqlc_postgres.New(db)
	} else {
		r.sqlite = sqlc_sqlite.New(db)
	}

	return r
}

// Create adds a new library to the database.
func (r *Repository) Create(ctx context.Context, lib *library.Library) error {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.CreateLibrary(ctx, sqlc_postgres.CreateLibraryParams{
				Name: lib.Name,
				Path: lib.Path,
				Type: string(lib.Type),
			})
		},
		func() (any, error) {
			return r.sqlite.CreateLibrary(ctx, sqlc_sqlite.CreateLibraryParams{
				Name: lib.Name,
				Path: lib.Path,
				Type: string(lib.Type),
			})
		},
	)
	if err != nil {
		return err
	}

	// Convert result to domain library
	if r.router.IsPostgresDB() {
		pgResult := result.(sqlc_postgres.Library)
		lib.ID = int64(pgResult.ID)
		lib.CreatedAt = common.ParseNullTime(pgResult.CreatedAt)
		lib.UpdatedAt = common.ParseNullTime(pgResult.UpdatedAt)
	} else {
		sqResult := result.(sqlc_sqlite.Library)
		lib.ID = sqResult.ID
		lib.CreatedAt = common.ParseNullTime(sqResult.CreatedAt)
		lib.UpdatedAt = common.ParseNullTime(sqResult.UpdatedAt)
	}

	return nil
}

// GetByID retrieves a library by its ID.
func (r *Repository) GetByID(ctx context.Context, id int64) (*library.Library, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetLibraryByID(ctx, int32(id))
		},
		func() (any, error) {
			return r.sqlite.GetLibraryByID(ctx, id)
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, library.ErrLibraryNotFound
		}
		return nil, err
	}

	// Convert to domain library
	if r.router.IsPostgresDB() {
		pgResult := result.(sqlc_postgres.Library)
		return &library.Library{
			ID:        int64(pgResult.ID),
			Name:      pgResult.Name,
			Path:      pgResult.Path,
			Type:      library.LibraryType(pgResult.Type),
			CreatedAt: common.ParseNullTime(pgResult.CreatedAt),
			UpdatedAt: common.ParseNullTime(pgResult.UpdatedAt),
		}, nil
	}

	sqResult := result.(sqlc_sqlite.Library)
	return &library.Library{
		ID:        sqResult.ID,
		Name:      sqResult.Name,
		Path:      sqResult.Path,
		Type:      library.LibraryType(sqResult.Type),
		CreatedAt: common.ParseNullTime(sqResult.CreatedAt),
		UpdatedAt: common.ParseNullTime(sqResult.UpdatedAt),
	}, nil
}

// GetByPath retrieves a library by its path.
func (r *Repository) GetByPath(ctx context.Context, path string) (*library.Library, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetLibraryByPath(ctx, path)
		},
		func() (any, error) {
			return r.sqlite.GetLibraryByPath(ctx, path)
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, library.ErrLibraryNotFound
		}
		return nil, err
	}

	// Convert to domain library
	if r.router.IsPostgresDB() {
		pgResult := result.(sqlc_postgres.Library)
		return &library.Library{
			ID:        int64(pgResult.ID),
			Name:      pgResult.Name,
			Path:      pgResult.Path,
			Type:      library.LibraryType(pgResult.Type),
			CreatedAt: common.ParseNullTime(pgResult.CreatedAt),
			UpdatedAt: common.ParseNullTime(pgResult.UpdatedAt),
		}, nil
	}

	sqResult := result.(sqlc_sqlite.Library)
	return &library.Library{
		ID:        sqResult.ID,
		Name:      sqResult.Name,
		Path:      sqResult.Path,
		Type:      library.LibraryType(sqResult.Type),
		CreatedAt: common.ParseNullTime(sqResult.CreatedAt),
		UpdatedAt: common.ParseNullTime(sqResult.UpdatedAt),
	}, nil
}

// List retrieves all libraries.
func (r *Repository) List(ctx context.Context) ([]*library.Library, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.ListLibraries(ctx)
		},
		func() (any, error) {
			return r.sqlite.ListLibraries(ctx)
		},
	)
	if err != nil {
		return nil, err
	}

	// Convert to domain libraries
	if r.router.IsPostgresDB() {
		pgResults := result.([]sqlc_postgres.Library)
		libraries := make([]*library.Library, len(pgResults))
		for i, pgResult := range pgResults {
			libraries[i] = &library.Library{
				ID:        int64(pgResult.ID),
				Name:      pgResult.Name,
				Path:      pgResult.Path,
				Type:      library.LibraryType(pgResult.Type),
				CreatedAt: common.ParseNullTime(pgResult.CreatedAt),
				UpdatedAt: common.ParseNullTime(pgResult.UpdatedAt),
			}
		}
		return libraries, nil
	}

	sqResults := result.([]sqlc_sqlite.Library)
	libraries := make([]*library.Library, len(sqResults))
	for i, sqResult := range sqResults {
		libraries[i] = &library.Library{
			ID:        sqResult.ID,
			Name:      sqResult.Name,
			Path:      sqResult.Path,
			Type:      library.LibraryType(sqResult.Type),
			CreatedAt: common.ParseNullTime(sqResult.CreatedAt),
			UpdatedAt: common.ParseNullTime(sqResult.UpdatedAt),
		}
	}
	return libraries, nil
}

// Update modifies an existing library.
func (r *Repository) Update(ctx context.Context, lib *library.Library) error {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.UpdateLibrary(ctx, sqlc_postgres.UpdateLibraryParams{
				Name: lib.Name,
				Path: lib.Path,
				Type: string(lib.Type),
				ID:   int32(lib.ID),
			})
		},
		func() (any, error) {
			return r.sqlite.UpdateLibrary(ctx, sqlc_sqlite.UpdateLibraryParams{
				Name: lib.Name,
				Path: lib.Path,
				Type: string(lib.Type),
				ID:   lib.ID,
			})
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.ErrLibraryNotFound
		}
		return err
	}

	// Update timestamps
	if r.router.IsPostgresDB() {
		pgResult := result.(sqlc_postgres.Library)
		lib.UpdatedAt = common.ParseNullTime(pgResult.UpdatedAt)
	} else {
		sqResult := result.(sqlc_sqlite.Library)
		lib.UpdatedAt = common.ParseNullTime(sqResult.UpdatedAt)
	}

	return nil
}

// Delete removes a library from the database.
func (r *Repository) Delete(ctx context.Context, id int64) error {
	return r.router.RouteVoid(
		func() error {
			return r.postgres.DeleteLibrary(ctx, int32(id))
		},
		func() error {
			return r.sqlite.DeleteLibrary(ctx, id)
		},
	)
}

// Exists checks if a library with the given path already exists.
func (r *Repository) Exists(ctx context.Context, path string) (bool, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.LibraryExistsByPath(ctx, path)
		},
		func() (any, error) {
			count, err := r.sqlite.LibraryExistsByPath(ctx, path)
			if err != nil {
				return false, err
			}
			return count > 0, nil
		},
	)
	if err != nil {
		return false, err
	}

	return result.(bool), nil
}
