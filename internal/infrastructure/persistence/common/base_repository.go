package common

import (
	"database/sql"
	"errors"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
)

// BaseRepository provides common repository functionality for all media type repositories.
// It handles dual-database support (SQLite and PostgreSQL) via the unified Querier.
type BaseRepository struct {
	db      *sql.DB
	dbType  string
	querier *unified.Querier
}

// NewBaseRepository creates a new base repository with the specified database driver.
// The driver parameter should be "sqlite", "sqlite3", "postgres", or "postgresql".
func NewBaseRepository(db *sql.DB, driver string) *BaseRepository {
	return &BaseRepository{
		db:      db,
		dbType:  driver,
		querier: unified.NewQuerier(db, driver),
	}
}

// DB returns the underlying database connection
func (r *BaseRepository) DB() *sql.DB {
	return r.db
}

// DBType returns the database driver type
func (r *BaseRepository) DBType() string {
	return r.dbType
}

// Q returns the unified Querier that automatically routes to the correct backend.
// This is the preferred way to access the database - it eliminates the need for
// separate Postgres()/SQLite() calls and duplicate code paths.
func (r *BaseRepository) Q() *unified.Querier {
	return r.querier
}

// QWithTx returns a unified Querier that uses the provided transaction.
func (r *BaseRepository) QWithTx(tx *sql.Tx) *unified.Querier {
	return unified.NewQuerierWithTx(tx, r.dbType)
}

// ConvertNotFoundError converts sql.ErrNoRows to domain-specific not found error
func (r *BaseRepository) ConvertNotFoundError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return media.ErrMediaNotFound
	}
	return err
}
