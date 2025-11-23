package common

import (
	"context"
	"database/sql"
	"errors"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/adapters"
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

// QuerySingle executes a single-row query with automatic routing and mapping.
// This eliminates the boilerplate of Router().Route() + type assertion + error handling.
//
// Usage:
//
//	movie, err := QuerySingle(r.BaseRepository, ctx,
//	    func() (sqlc_postgres.GetMovieByMediaIDRow, error) {
//	        return r.Postgres().GetMovieByMediaID(ctx, int32(id))
//	    },
//	    func() (sqlc_sqlite.GetMovieByMediaIDRow, error) {
//	        return r.SQLite().GetMovieByMediaID(ctx, id)
//	    },
//	    postgresMovieToDomain,
//	    sqliteMovieToDomain,
//	)
func QuerySingle[TPostgresRow, TSQLiteRow, TDomain any](
	r *BaseRepository,
	ctx context.Context,
	pgQuery func() (TPostgresRow, error),
	sqQuery func() (TSQLiteRow, error),
	pgMapper func(TPostgresRow) TDomain,
	sqMapper func(TSQLiteRow) TDomain,
) (TDomain, error) {
	var zero TDomain

	result, err := r.Router().Route(
		func() (any, error) {
			row, err := pgQuery()
			if err != nil {
				return nil, err
			}
			return pgMapper(row), nil
		},
		func() (any, error) {
			row, err := sqQuery()
			if err != nil {
				return nil, err
			}
			return sqMapper(row), nil
		},
	)

	if err != nil {
		return zero, r.ConvertNotFoundError(err)
	}

	return result.(TDomain), nil
}

// QueryMany executes a multi-row query with automatic routing and mapping.
// This eliminates the boilerplate of Router().Route() + slice mapping + type assertion.
//
// Usage:
//
//	movies, err := QueryMany(r.BaseRepository, ctx,
//	    func() ([]sqlc_postgres.ListMoviesByLibraryRow, error) {
//	        return r.Postgres().ListMoviesByLibrary(ctx, int32(libraryID))
//	    },
//	    func() ([]sqlc_sqlite.ListMoviesByLibraryRow, error) {
//	        return r.SQLite().ListMoviesByLibrary(ctx, libraryID)
//	    },
//	    postgresListMovieToDomain,
//	    sqliteListMovieToDomain,
//	)
func QueryMany[TPostgresRow, TSQLiteRow, TDomain any](
	r *BaseRepository,
	ctx context.Context,
	pgQuery func() ([]TPostgresRow, error),
	sqQuery func() ([]TSQLiteRow, error),
	pgMapper func(TPostgresRow) TDomain,
	sqMapper func(TSQLiteRow) TDomain,
) ([]TDomain, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			rows, err := pgQuery()
			if err != nil {
				return nil, err
			}
			results := make([]TDomain, len(rows))
			for i, row := range rows {
				results[i] = pgMapper(row)
			}
			return results, nil
		},
		func() (any, error) {
			rows, err := sqQuery()
			if err != nil {
				return nil, err
			}
			results := make([]TDomain, len(rows))
			for i, row := range rows {
				results[i] = sqMapper(row)
			}
			return results, nil
		},
	)

	if err != nil {
		return nil, err
	}

	return result.([]TDomain), nil
}

// QueryScalar executes a scalar query (count, sum, etc.) with automatic routing.
// Handles type conversion between PostgreSQL (int64) and SQLite (int64) scalar results.
//
// Usage:
//
//	count, err := QueryScalar(r.BaseRepository, ctx,
//	    func() (int64, error) {
//	        return r.Postgres().CountMoviesByLibrary(ctx, int32(libraryID))
//	    },
//	    func() (int64, error) {
//	        return r.SQLite().CountMoviesByLibrary(ctx, libraryID)
//	    },
//	)
func QueryScalar[T any](
	r *BaseRepository,
	ctx context.Context,
	pgQuery func() (T, error),
	sqQuery func() (T, error),
) (T, error) {
	var zero T

	result, err := r.Router().Route(
		func() (any, error) {
			return pgQuery()
		},
		func() (any, error) {
			return sqQuery()
		},
	)

	if err != nil {
		return zero, err
	}

	return result.(T), nil
}

// ExecuteCommand executes a write command (insert, update, delete) with automatic routing.
// Returns only an error, suitable for :exec queries.
//
// Usage:
//
//	err := ExecuteCommand(r.BaseRepository, ctx,
//	    func() error {
//	        return r.Postgres().CreateMovie(ctx, pgParams)
//	    },
//	    func() error {
//	        return r.SQLite().CreateMovie(ctx, sqParams)
//	    },
//	)
func ExecuteCommand(
	r *BaseRepository,
	ctx context.Context,
	pgCommand func() error,
	sqCommand func() error,
) error {
	return r.Router().RouteVoid(pgCommand, sqCommand)
}

// MediaRepository interface for base media operations (to avoid import cycles)
