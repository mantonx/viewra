package enrichment

import (
	"context"
	"database/sql"

	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/adapters"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// MetadataSourceRepository implements enrichment.MetadataSourceRepository using sqlc.
// It supports both SQLite and PostgreSQL through database-specific queriers.
type MetadataSourceRepository struct {
	db       *sql.DB
	dbType   string
	sqlite   *sqlc_sqlite.Queries
	postgres *sqlc_postgres.Queries
	adapter  *adapters.TypeAdapter
	router   *common.QueryRouter
}

// NewMetadataSourceRepository creates a new metadata source repository with the appropriate database driver.
func NewMetadataSourceRepository(db *sql.DB, driver string) *MetadataSourceRepository {
	r := &MetadataSourceRepository{
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

// Upsert creates or updates a metadata source record.
func (r *MetadataSourceRepository) Upsert(ctx context.Context, source *enrichment.MetadataSource) error {
	return r.router.RouteVoid(
		func() error {
			return r.postgres.UpsertMetadataSource(ctx, sqlc_postgres.UpsertMetadataSourceParams{
				MediaID:   int32(source.MediaID),
				FieldName: source.FieldName,
				PluginID:  source.PluginID,
				RawValue:  sql.NullString{String: source.RawValue, Valid: source.RawValue != ""},
			})
		},
		func() error {
			return r.sqlite.UpsertMetadataSource(ctx, sqlc_sqlite.UpsertMetadataSourceParams{
				MediaID:   source.MediaID,
				FieldName: source.FieldName,
				PluginID:  source.PluginID,
				RawValue:  sql.NullString{String: source.RawValue, Valid: source.RawValue != ""},
			})
		},
	)
}

// GetByMedia returns all metadata sources for a media item.
func (r *MetadataSourceRepository) GetByMedia(ctx context.Context, mediaID int64) ([]*enrichment.MetadataSource, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetMetadataSourcesByMedia(ctx, int32(mediaID))
		},
		func() (any, error) {
			return r.sqlite.GetMetadataSourcesByMedia(ctx, mediaID)
		},
	)
	if err != nil {
		return nil, err
	}

	if r.router.IsPostgresDB() {
		pgSources := result.([]sqlc_postgres.MediaMetadataSource)
		sources := make([]*enrichment.MetadataSource, len(pgSources))
		for i, pgSource := range pgSources {
			sources[i] = r.convertToMetadataSource(pgSource)
		}
		return sources, nil
	}

	sqSources := result.([]sqlc_sqlite.MediaMetadataSource)
	sources := make([]*enrichment.MetadataSource, len(sqSources))
	for i, sqSource := range sqSources {
		sources[i] = r.convertToMetadataSource(sqSource)
	}
	return sources, nil
}

// DeleteByMedia removes all metadata sources for a media item.
func (r *MetadataSourceRepository) DeleteByMedia(ctx context.Context, mediaID int64) error {
	return r.router.RouteVoid(
		func() error {
			return r.postgres.DeleteMetadataSourcesByMedia(ctx, int32(mediaID))
		},
		func() error {
			return r.sqlite.DeleteMetadataSourcesByMedia(ctx, mediaID)
		},
	)
}

// DeleteByPlugin removes all metadata sources for a plugin.
func (r *MetadataSourceRepository) DeleteByPlugin(ctx context.Context, pluginID string) error {
	return r.router.RouteVoid(
		func() error {
			return r.postgres.DeleteMetadataSourcesByPlugin(ctx, pluginID)
		},
		func() error {
			return r.sqlite.DeleteMetadataSourcesByPlugin(ctx, pluginID)
		},
	)
}

// convertToMetadataSource converts sqlc result to domain MetadataSource.
func (r *MetadataSourceRepository) convertToMetadataSource(result any) *enrichment.MetadataSource {
	if r.router.IsPostgresDB() {
		pgSource := result.(sqlc_postgres.MediaMetadataSource)
		return &enrichment.MetadataSource{
			MediaID:   int64(pgSource.MediaID),
			FieldName: pgSource.FieldName,
			PluginID:  pgSource.PluginID,
			RawValue:  common.ParseNullString(pgSource.RawValue),
			UpdatedAt: common.ParseNullTime(pgSource.UpdatedAt),
		}
	}

	sqSource := result.(sqlc_sqlite.MediaMetadataSource)
	return &enrichment.MetadataSource{
		MediaID:   sqSource.MediaID,
		FieldName: sqSource.FieldName,
		PluginID:  sqSource.PluginID,
		RawValue:  common.ParseNullString(sqSource.RawValue),
		UpdatedAt: parseTimeString(sqSource.UpdatedAt),
	}
}
