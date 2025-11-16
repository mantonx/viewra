package common

import (
	"database/sql"
	"errors"

	"github.com/viewra/viewra/internal/domain/media"
	"github.com/viewra/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/viewra/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/viewra/viewra/internal/infrastructure/persistence/adapters"
)

// BaseRepository provides common repository functionality for all media type repositories.
// It handles dual-database support (SQLite and PostgreSQL) and common patterns.
type BaseRepository struct {
	db       *sql.DB
	dbType   string
	sqlite   *sqlc_sqlite.Queries
	postgres *sqlc_postgres.Queries
	adapter  *adapters.TypeAdapter
	router   *QueryRouter
}

// NewBaseRepository creates a new base repository with the specified database driver.
// The driver parameter should be "sqlite", "sqlite3", "postgres", or "postgresql".
func NewBaseRepository(db *sql.DB, driver string) *BaseRepository {
	r := &BaseRepository{
		db:      db,
		dbType:  driver,
		adapter: adapters.NewTypeAdapter(),
		router:  NewQueryRouter(driver),
	}

	if IsPostgres(driver) {
		r.postgres = sqlc_postgres.New(db)
	} else {
		r.sqlite = sqlc_sqlite.New(db)
	}

	return r
}

// DB returns the underlying database connection
func (r *BaseRepository) DB() *sql.DB {
	return r.db
}

// DBType returns the database driver type
func (r *BaseRepository) DBType() string {
	return r.dbType
}

// SQLite returns the SQLite queries instance
func (r *BaseRepository) SQLite() *sqlc_sqlite.Queries {
	return r.sqlite
}

// Postgres returns the PostgreSQL queries instance
func (r *BaseRepository) Postgres() *sqlc_postgres.Queries {
	return r.postgres
}

// Adapter returns the type adapter
func (r *BaseRepository) Adapter() *adapters.TypeAdapter {
	return r.adapter
}

// Router returns the query router
func (r *BaseRepository) Router() *QueryRouter {
	return r.router
}

// PostgresNotImplemented returns a standard error for unimplemented PostgreSQL features
func (r *BaseRepository) PostgresNotImplemented() error {
	return errors.New("PostgreSQL support not yet implemented")
}

// ConvertNotFoundError converts sql.ErrNoRows to domain-specific not found error
func (r *BaseRepository) ConvertNotFoundError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return media.ErrMediaNotFound
	}
	return err
}

// MediaRepository interface for base media operations (to avoid import cycles)
