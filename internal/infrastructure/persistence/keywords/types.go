package keywords

import (
	"database/sql"

	"github.com/mantonx/viewra/internal/domain/media"
	sqlc_postgres "github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	sqlc_sqlite "github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
)

// SQLite mappers

func sqliteKeywordRowToDomain(row sqlc_sqlite.GetKeywordsByEntityRow) *media.Keyword {
	return &media.Keyword{
		KeywordID:  int(row.KeywordID),
		Name:       row.Keyword,
		IsLocation: nullBoolToBool(row.IsLocation),
	}
}

func sqliteLocationKeywordRowToDomain(row sqlc_sqlite.GetLocationKeywordsByEntityRow) *media.Keyword {
	return &media.Keyword{
		KeywordID:  int(row.KeywordID),
		Name:       row.Keyword,
		IsLocation: true,
	}
}

// PostgreSQL mappers

func postgresKeywordRowToDomain(row sqlc_postgres.GetKeywordsByEntityRow) *media.Keyword {
	return &media.Keyword{
		KeywordID:  int(row.KeywordID),
		Name:       row.Keyword,
		IsLocation: nullBoolToBool(row.IsLocation),
	}
}

func postgresLocationKeywordRowToDomain(row sqlc_postgres.GetLocationKeywordsByEntityRow) *media.Keyword {
	return &media.Keyword{
		KeywordID:  int(row.KeywordID),
		Name:       row.Keyword,
		IsLocation: true,
	}
}

// Helper functions

func boolToNullBool(b bool) sql.NullBool {
	return sql.NullBool{Bool: b, Valid: true}
}

func nullBoolToBool(nb sql.NullBool) bool {
	if !nb.Valid {
		return false
	}
	return nb.Bool
}
