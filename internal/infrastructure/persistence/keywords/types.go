package keywords

import (
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// SQLite mappers

func sqliteKeywordRowToDomain(row sqlc_sqlite.GetKeywordsByEntityRow) *media.Keyword {
	return &media.Keyword{
		KeywordID:  int(row.KeywordID),
		Name:       row.Keyword,
		IsLocation: common.NullInt64ToBool(row.IsLocation),
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
		IsLocation: common.NullInt64ToBool(row.IsLocation),
	}
}

func postgresLocationKeywordRowToDomain(row sqlc_postgres.GetLocationKeywordsByEntityRow) *media.Keyword {
	return &media.Keyword{
		KeywordID:  int(row.KeywordID),
		Name:       row.Keyword,
		IsLocation: true,
	}
}
