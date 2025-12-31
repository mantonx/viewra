package keywords

import (
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// Unified mappers - work for both SQLite and PostgreSQL since types are identical

func keywordRowToDomain(row unified.GetKeywordsByEntityRow) *media.Keyword {
	return &media.Keyword{
		KeywordID:  int(row.KeywordID),
		Name:       row.Keyword,
		IsLocation: common.NullInt64ToBool(row.IsLocation),
	}
}

func locationKeywordRowToDomain(row unified.GetLocationKeywordsByEntityRow) *media.Keyword {
	return &media.Keyword{
		KeywordID:  int(row.KeywordID),
		Name:       row.Keyword,
		IsLocation: true,
	}
}
