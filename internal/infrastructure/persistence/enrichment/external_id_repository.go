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

// NewExternalIDRepository creates a new external ID repository with the appropriate database driver.
func NewExternalIDRepository(db *sql.DB, driver string) *ExternalIDRepository {
	r := &ExternalIDRepository{
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

// Upsert creates or updates an external ID.
func (r *ExternalIDRepository) Upsert(ctx context.Context, id *enrichment.ExternalID) error {
	return r.router.RouteVoid(
		func() error {
			return r.postgres.UpsertExternalID(ctx, sqlc_postgres.UpsertExternalIDParams{
				MediaID:    int32(id.MediaID),
				Provider:   id.Provider,
				ExternalID: id.ExternalID,
			})
		},
		func() error {
			return r.sqlite.UpsertExternalID(ctx, sqlc_sqlite.UpsertExternalIDParams{
				MediaID:    id.MediaID,
				Provider:   id.Provider,
				ExternalID: id.ExternalID,
			})
		},
	)
}

// GetByMedia returns all external IDs for a media item.
func (r *ExternalIDRepository) GetByMedia(ctx context.Context, mediaID int64) ([]*enrichment.ExternalID, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetExternalIDsByMedia(ctx, int32(mediaID))
		},
		func() (any, error) {
			return r.sqlite.GetExternalIDsByMedia(ctx, mediaID)
		},
	)
	if err != nil {
		return nil, err
	}

	if r.router.IsPostgresDB() {
		pgIDs := result.([]sqlc_postgres.MediaExternalID)
		ids := make([]*enrichment.ExternalID, len(pgIDs))
		for i, pgID := range pgIDs {
			ids[i] = r.convertToExternalID(pgID)
		}
		return ids, nil
	}

	sqIDs := result.([]sqlc_sqlite.MediaExternalID)
	ids := make([]*enrichment.ExternalID, len(sqIDs))
	for i, sqID := range sqIDs {
		ids[i] = r.convertToExternalID(sqID)
	}
	return ids, nil
}

// GetMediaByExternalID finds a media item by provider and external ID.
func (r *ExternalIDRepository) GetMediaByExternalID(ctx context.Context, provider, externalID string) (int64, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetMediaByExternalID(ctx, sqlc_postgres.GetMediaByExternalIDParams{
				Provider:   provider,
				ExternalID: externalID,
			})
		},
		func() (any, error) {
			return r.sqlite.GetMediaByExternalID(ctx, sqlc_sqlite.GetMediaByExternalIDParams{
				Provider:   provider,
				ExternalID: externalID,
			})
		},
	)
	if err != nil {
		return 0, err
	}

	if r.router.IsPostgresDB() {
		return int64(result.(int32)), nil
	}
	return result.(int64), nil
}

// DeleteByMedia removes all external IDs for a media item.
func (r *ExternalIDRepository) DeleteByMedia(ctx context.Context, mediaID int64) error {
	return r.router.RouteVoid(
		func() error {
			return r.postgres.DeleteExternalIDsByMedia(ctx, int32(mediaID))
		},
		func() error {
			return r.sqlite.DeleteExternalIDsByMedia(ctx, mediaID)
		},
	)
}

// convertToExternalID converts sqlc result to domain ExternalID.
func (r *ExternalIDRepository) convertToExternalID(result any) *enrichment.ExternalID {
	if r.router.IsPostgresDB() {
		pgID := result.(sqlc_postgres.MediaExternalID)
		return &enrichment.ExternalID{
			MediaID:    int64(pgID.MediaID),
			Provider:   pgID.Provider,
			ExternalID: pgID.ExternalID,
			CreatedAt:  common.ParseNullTime(pgID.CreatedAt),
			UpdatedAt:  common.ParseNullTime(pgID.UpdatedAt),
		}
	}

	sqID := result.(sqlc_sqlite.MediaExternalID)
	return &enrichment.ExternalID{
		MediaID:    sqID.MediaID,
		Provider:   sqID.Provider,
		ExternalID: sqID.ExternalID,
		CreatedAt:  parseTimeString(sqID.CreatedAt),
		UpdatedAt:  parseTimeString(sqID.UpdatedAt),
	}
}
