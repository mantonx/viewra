package keywords

import (
	"context"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// Repository implements media.KeywordRepository.
type Repository struct {
	*common.BaseRepository
}

// NewRepository creates a new keywords repository.
func NewRepository(base *common.BaseRepository) *Repository {
	return &Repository{BaseRepository: base}
}

// UpsertKeyword adds or updates a keyword for an entity.
func (r *Repository) UpsertKeyword(mediaType string, entityID int64, keyword *media.Keyword) error {
	ctx := context.Background()
	return common.ExecuteCommand(
		r.BaseRepository, ctx,
		func() error {
			return r.Postgres().InsertKeyword(ctx, sqlc_postgres.InsertKeywordParams{
				MediaType:  mediaType,
				EntityID:   int32(entityID),
				KeywordID:  int32(keyword.KeywordID),
				Keyword:    keyword.Name,
				IsLocation: boolToNullBool(keyword.IsLocation),
			})
		},
		func() error {
			return r.SQLite().InsertKeyword(ctx, sqlc_sqlite.InsertKeywordParams{
				MediaType:  mediaType,
				EntityID:   entityID,
				KeywordID:  int64(keyword.KeywordID),
				Keyword:    keyword.Name,
				IsLocation: boolToNullBool(keyword.IsLocation),
			})
		},
	)
}

// GetKeywordsForEntity retrieves all keywords for a media entity.
func (r *Repository) GetKeywordsForEntity(mediaType string, entityID int64) ([]*media.Keyword, error) {
	ctx := context.Background()
	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.GetKeywordsByEntityRow, error) {
			return r.Postgres().GetKeywordsByEntity(ctx, sqlc_postgres.GetKeywordsByEntityParams{
				MediaType: mediaType,
				EntityID:  int32(entityID),
			})
		},
		func() ([]sqlc_sqlite.GetKeywordsByEntityRow, error) {
			return r.SQLite().GetKeywordsByEntity(ctx, sqlc_sqlite.GetKeywordsByEntityParams{
				MediaType: mediaType,
				EntityID:  entityID,
			})
		},
		postgresKeywordRowToDomain,
		sqliteKeywordRowToDomain,
	)
}

// GetLocationKeywordsForEntity retrieves only location-related keywords.
func (r *Repository) GetLocationKeywordsForEntity(mediaType string, entityID int64) ([]*media.Keyword, error) {
	ctx := context.Background()
	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.GetLocationKeywordsByEntityRow, error) {
			return r.Postgres().GetLocationKeywordsByEntity(ctx, sqlc_postgres.GetLocationKeywordsByEntityParams{
				MediaType: mediaType,
				EntityID:  int32(entityID),
			})
		},
		func() ([]sqlc_sqlite.GetLocationKeywordsByEntityRow, error) {
			return r.SQLite().GetLocationKeywordsByEntity(ctx, sqlc_sqlite.GetLocationKeywordsByEntityParams{
				MediaType: mediaType,
				EntityID:  entityID,
			})
		},
		postgresLocationKeywordRowToDomain,
		sqliteLocationKeywordRowToDomain,
	)
}

// ClearKeywordsForEntity removes all keywords for a media entity.
func (r *Repository) ClearKeywordsForEntity(mediaType string, entityID int64) error {
	ctx := context.Background()
	return common.ExecuteCommand(
		r.BaseRepository, ctx,
		func() error {
			return r.Postgres().DeleteKeywordsByEntity(ctx, sqlc_postgres.DeleteKeywordsByEntityParams{
				MediaType: mediaType,
				EntityID:  int32(entityID),
			})
		},
		func() error {
			return r.SQLite().DeleteKeywordsByEntity(ctx, sqlc_sqlite.DeleteKeywordsByEntityParams{
				MediaType: mediaType,
				EntityID:  entityID,
			})
		},
	)
}

// ReplaceKeywordsForEntity clears existing keywords and adds new ones.
func (r *Repository) ReplaceKeywordsForEntity(mediaType string, entityID int64, keywords []*media.Keyword) error {
	// Clear existing keywords
	if err := r.ClearKeywordsForEntity(mediaType, entityID); err != nil {
		return err
	}

	// Add new keywords
	for _, keyword := range keywords {
		if err := r.UpsertKeyword(mediaType, entityID, keyword); err != nil {
			return err
		}
	}

	return nil
}

// Ensure Repository implements media.KeywordRepository
var _ media.KeywordRepository = (*Repository)(nil)
