package media

import (
	"context"
	"database/sql"
	"errors"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/adapters"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// NewRepository creates a new media repository with the specified database driver.
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

// Create adds a new media item to the database
func (r *Repository) Create(ctx context.Context, m *media.Media) error {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.CreateMedia(ctx, buildPostgresCreateParams(m))
		},
		func() (any, error) {
			return r.sqlite.CreateMedia(ctx, buildSQLiteCreateParams(m))
		},
	)
	if err != nil {
		return err
	}

	// Convert result to domain media
	if r.router.IsPostgresDB() {
		pgResult := result.(sqlc_postgres.Medium)
		m.ID = int64(pgResult.ID)
		m.CreatedAt = common.ParseNullTime(pgResult.CreatedAt)
		m.UpdatedAt = common.ParseNullTime(pgResult.UpdatedAt)
	} else {
		sqResult := result.(sqlc_sqlite.Medium)
		m.ID = sqResult.ID
		m.CreatedAt = common.ParseNullTime(sqResult.CreatedAt)
		m.UpdatedAt = common.ParseNullTime(sqResult.UpdatedAt)
	}

	return nil
}

// GetByID retrieves a media item by its ID
func (r *Repository) GetByID(ctx context.Context, id int64) (*media.Media, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetMediaByID(ctx, int32(id))
		},
		func() (any, error) {
			return r.sqlite.GetMediaByID(ctx, id)
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, media.ErrMediaNotFound
		}
		return nil, err
	}

	// Convert to domain media
	if r.router.IsPostgresDB() {
		return pgMediumToDomain(result.(sqlc_postgres.Medium)), nil
	}
	return sqliteMediumToDomain(result.(sqlc_sqlite.Medium)), nil
}

// GetByFilePath retrieves a media item by its file path within a library
func (r *Repository) GetByFilePath(ctx context.Context, libraryID int64, filePath string) (*media.Media, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetMediaByFilePath(ctx, sqlc_postgres.GetMediaByFilePathParams{
				LibraryID: int32(libraryID),
				FilePath:  filePath,
			})
		},
		func() (any, error) {
			return r.sqlite.GetMediaByFilePath(ctx, sqlc_sqlite.GetMediaByFilePathParams{
				LibraryID: libraryID,
				FilePath:  filePath,
			})
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, media.ErrMediaNotFound
		}
		return nil, err
	}

	// Convert to domain media
	if r.router.IsPostgresDB() {
		return pgMediumToDomain(result.(sqlc_postgres.Medium)), nil
	}
	return sqliteMediumToDomain(result.(sqlc_sqlite.Medium)), nil
}

// ListAll retrieves all media items across all libraries
func (r *Repository) ListAll(ctx context.Context) ([]*media.Media, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.ListAllMedia(ctx)
		},
		func() (any, error) {
			return r.sqlite.ListAllMedia(ctx)
		},
	)
	if err != nil {
		return nil, err
	}

	// Convert to domain media
	if r.router.IsPostgresDB() {
		pgResults := result.([]sqlc_postgres.Medium)
		mediaList := make([]*media.Media, len(pgResults))
		for i, pgResult := range pgResults {
			mediaList[i] = pgMediumToDomain(pgResult)
		}
		return mediaList, nil
	}

	sqResults := result.([]sqlc_sqlite.Medium)
	mediaList := make([]*media.Media, len(sqResults))
	for i, sqResult := range sqResults {
		mediaList[i] = sqliteMediumToDomain(sqResult)
	}
	return mediaList, nil
}

// ListByLibrary retrieves all media items in a specific library
func (r *Repository) ListByLibrary(ctx context.Context, libraryID int64) ([]*media.Media, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.ListMediaByLibrary(ctx, int32(libraryID))
		},
		func() (any, error) {
			return r.sqlite.ListMediaByLibrary(ctx, libraryID)
		},
	)
	if err != nil {
		return nil, err
	}

	// Convert to domain media
	if r.router.IsPostgresDB() {
		pgResults := result.([]sqlc_postgres.Medium)
		mediaList := make([]*media.Media, len(pgResults))
		for i, pgResult := range pgResults {
			mediaList[i] = pgMediumToDomain(pgResult)
		}
		return mediaList, nil
	}

	sqResults := result.([]sqlc_sqlite.Medium)
	mediaList := make([]*media.Media, len(sqResults))
	for i, sqResult := range sqResults {
		mediaList[i] = sqliteMediumToDomain(sqResult)
	}
	return mediaList, nil
}

// GetFilePathCache retrieves a map of file_path -> id for all media in a library.
// Memory-efficient: only loads the columns needed for cache lookup.
//
// Memory estimate: ~200 bytes per file (150 byte avg path + map overhead).
// For 100K files: ~20MB. For 500K files: ~100MB.
//
// The database/sql package streams rows, so we only hold one row in memory at a time
// during iteration. The map itself is the main memory consumer.
func (r *Repository) GetFilePathCache(ctx context.Context, libraryID int64) (map[string]int64, error) {
	// First, get count to pre-allocate map (avoids repeated reallocation)
	var count int64
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM media WHERE library_id = ?
	`, libraryID).Scan(&count)
	if err != nil {
		// Non-fatal - continue with default allocation
		count = 0
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, file_path
		FROM media
		WHERE library_id = ?
	`, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Pre-allocate map based on count
	cache := make(map[string]int64, count)
	for rows.Next() {
		var id int64
		var filePath string
		if err := rows.Scan(&id, &filePath); err != nil {
			return nil, err
		}
		cache[filePath] = id
	}

	return cache, rows.Err()
}

// ListByType retrieves all media items of a specific type in a library
func (r *Repository) ListByType(
	ctx context.Context,
	libraryID int64,
	mediaType media.MediaType,
) ([]*media.Media, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.ListMediaByType(ctx, sqlc_postgres.ListMediaByTypeParams{
				LibraryID: int32(libraryID),
				Type:      string(mediaType),
			})
		},
		func() (any, error) {
			return r.sqlite.ListMediaByType(ctx, sqlc_sqlite.ListMediaByTypeParams{
				LibraryID: libraryID,
				Type:      string(mediaType),
			})
		},
	)
	if err != nil {
		return nil, err
	}

	// Convert to domain media
	if r.router.IsPostgresDB() {
		pgResults := result.([]sqlc_postgres.Medium)
		mediaList := make([]*media.Media, len(pgResults))
		for i, pgResult := range pgResults {
			mediaList[i] = pgMediumToDomain(pgResult)
		}
		return mediaList, nil
	}

	sqResults := result.([]sqlc_sqlite.Medium)
	mediaList := make([]*media.Media, len(sqResults))
	for i, sqResult := range sqResults {
		mediaList[i] = sqliteMediumToDomain(sqResult)
	}
	return mediaList, nil
}

// Update modifies an existing media item
func (r *Repository) Update(ctx context.Context, m *media.Media) error {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.UpdateMedia(ctx, buildPostgresUpdateParams(m))
		},
		func() (any, error) {
			return r.sqlite.UpdateMedia(ctx, buildSQLiteUpdateParams(m))
		},
	)
	if err != nil {
		return err
	}

	// Update timestamps
	if r.router.IsPostgresDB() {
		pgResult := result.(sqlc_postgres.Medium)
		m.UpdatedAt = common.ParseNullTime(pgResult.UpdatedAt)
	} else {
		sqResult := result.(sqlc_sqlite.Medium)
		m.UpdatedAt = common.ParseNullTime(sqResult.UpdatedAt)
	}

	return nil
}

// Delete removes a media item from the repository
func (r *Repository) Delete(ctx context.Context, id int64) error {
	return r.router.RouteVoid(
		func() error {
			return r.postgres.DeleteMedia(ctx, int32(id))
		},
		func() error {
			return r.sqlite.DeleteMedia(ctx, id)
		},
	)
}

// ExistsInLibrary checks if a media item with the given file path exists in the library
func (r *Repository) ExistsInLibrary(ctx context.Context, libraryID int64, filePath string) (bool, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.MediaExistsInLibrary(ctx, sqlc_postgres.MediaExistsInLibraryParams{
				LibraryID: int32(libraryID),
				FilePath:  filePath,
			})
		},
		func() (any, error) {
			count, err := r.sqlite.MediaExistsInLibrary(ctx, sqlc_sqlite.MediaExistsInLibraryParams{
				LibraryID: libraryID,
				FilePath:  filePath,
			})
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

// Count returns the total number of media items in a library
func (r *Repository) Count(ctx context.Context, libraryID int64) (int64, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.CountMediaInLibrary(ctx, int32(libraryID))
		},
		func() (any, error) {
			return r.sqlite.CountMediaInLibrary(ctx, libraryID)
		},
	)
	if err != nil {
		return 0, err
	}

	return result.(int64), nil
}

// CountByType returns the number of media items of a specific type in a library
func (r *Repository) CountByType(ctx context.Context, libraryID int64, mediaType media.MediaType) (int64, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.CountMediaByType(ctx, sqlc_postgres.CountMediaByTypeParams{
				LibraryID: int32(libraryID),
				Type:      string(mediaType),
			})
		},
		func() (any, error) {
			return r.sqlite.CountMediaByType(ctx, sqlc_sqlite.CountMediaByTypeParams{
				LibraryID: libraryID,
				Type:      string(mediaType),
			})
		},
	)
	if err != nil {
		return 0, err
	}

	return result.(int64), nil
}

// DeleteWithTx removes a media item from the repository within a transaction
func (r *Repository) DeleteWithTx(ctx context.Context, tx *sql.Tx, id int64) error {
	_, err := r.router.Route(
		func() (any, error) {
			return nil, r.postgres.WithTx(tx).DeleteMedia(ctx, int32(id))
		},
		func() (any, error) {
			return nil, r.sqlite.WithTx(tx).DeleteMedia(ctx, id)
		},
	)
	return err
}

// ListByLibraryWithTx retrieves all media items in a specific library within a transaction
func (r *Repository) ListByLibraryWithTx(ctx context.Context, tx *sql.Tx, libraryID int64) ([]*media.Media, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.WithTx(tx).ListMediaByLibrary(ctx, int32(libraryID))
		},
		func() (any, error) {
			return r.sqlite.WithTx(tx).ListMediaByLibrary(ctx, libraryID)
		},
	)
	if err != nil {
		return nil, err
	}

	// Convert to domain media
	if r.router.IsPostgresDB() {
		pgResults := result.([]sqlc_postgres.Medium)
		mediaList := make([]*media.Media, len(pgResults))
		for i, pgResult := range pgResults {
			mediaList[i] = pgMediumToDomain(pgResult)
		}
		return mediaList, nil
	}

	sqResults := result.([]sqlc_sqlite.Medium)
	mediaList := make([]*media.Media, len(sqResults))
	for i, sqResult := range sqResults {
		mediaList[i] = sqliteMediumToDomain(sqResult)
	}
	return mediaList, nil
}
