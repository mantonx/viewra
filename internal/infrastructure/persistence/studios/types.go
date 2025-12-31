package studios

import (
	"database/sql"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
)

// Unified mappers - work for both SQLite and PostgreSQL since types are identical

func studioToDomain(s unified.Studio) *media.Studio {
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

func buildCreateStudioParams(studio *media.Studio) unified.CreateStudioParams {
	params := unified.CreateStudioParams{
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

func buildUpdateStudioParams(studio *media.Studio) unified.UpdateStudioParams {
	params := unified.UpdateStudioParams{
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
