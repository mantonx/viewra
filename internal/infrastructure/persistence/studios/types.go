package studios

import (
	"database/sql"

	"github.com/mantonx/viewra/internal/domain/media"
	sqlc_postgres "github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	sqlc_sqlite "github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
)

// SQLite mappers

func sqliteStudioToDomain(s sqlc_sqlite.Studio) *media.Studio {
	studio := &media.Studio{
		ID:   s.ID,
		Name: s.Name,
	}
	if s.LogoPath.Valid {
		studio.LogoPath = s.LogoPath.String
	}
	if s.TmdbID.Valid {
		studio.TMDbID = int(s.TmdbID.Int64)
	}
	return studio
}

func buildSQLiteCreateStudioParams(studio *media.Studio) sqlc_sqlite.CreateStudioParams {
	params := sqlc_sqlite.CreateStudioParams{
		Name: studio.Name,
	}
	if studio.LogoPath != "" {
		params.LogoPath = sql.NullString{String: studio.LogoPath, Valid: true}
	}
	if studio.TMDbID != 0 {
		params.TmdbID = sql.NullInt64{Int64: int64(studio.TMDbID), Valid: true}
	}
	return params
}

func buildSQLiteUpdateStudioParams(studio *media.Studio) sqlc_sqlite.UpdateStudioParams {
	params := sqlc_sqlite.UpdateStudioParams{
		ID:   studio.ID,
		Name: studio.Name,
	}
	if studio.LogoPath != "" {
		params.LogoPath = sql.NullString{String: studio.LogoPath, Valid: true}
	}
	if studio.TMDbID != 0 {
		params.TmdbID = sql.NullInt64{Int64: int64(studio.TMDbID), Valid: true}
	}
	return params
}

// PostgreSQL mappers

func postgresStudioToDomain(s sqlc_postgres.Studio) *media.Studio {
	studio := &media.Studio{
		ID:   int64(s.ID),
		Name: s.Name,
	}
	if s.LogoPath.Valid {
		studio.LogoPath = s.LogoPath.String
	}
	if s.TmdbID.Valid {
		studio.TMDbID = int(s.TmdbID.Int32)
	}
	return studio
}

func buildPostgresCreateStudioParams(studio *media.Studio) sqlc_postgres.CreateStudioParams {
	params := sqlc_postgres.CreateStudioParams{
		Name: studio.Name,
	}
	if studio.LogoPath != "" {
		params.LogoPath = sql.NullString{String: studio.LogoPath, Valid: true}
	}
	if studio.TMDbID != 0 {
		params.TmdbID = sql.NullInt32{Int32: int32(studio.TMDbID), Valid: true}
	}
	return params
}

func buildPostgresUpdateStudioParams(studio *media.Studio) sqlc_postgres.UpdateStudioParams {
	params := sqlc_postgres.UpdateStudioParams{
		ID:   int32(studio.ID),
		Name: studio.Name,
	}
	if studio.LogoPath != "" {
		params.LogoPath = sql.NullString{String: studio.LogoPath, Valid: true}
	}
	if studio.TMDbID != 0 {
		params.TmdbID = sql.NullInt32{Int32: int32(studio.TMDbID), Valid: true}
	}
	return params
}
