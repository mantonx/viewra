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
// The ExternalID must have MediaType and EntityID set.
func (r *ExternalIDRepository) Upsert(ctx context.Context, id *enrichment.ExternalID) error {
	// Convert optional MediaID to sql.NullInt64
	var mediaID sql.NullInt64
	if id.MediaID != nil {
		mediaID = sql.NullInt64{Int64: *id.MediaID, Valid: true}
	}

	return r.router.RouteVoid(
		func() error {
			var pgMediaID sql.NullInt32
			if id.MediaID != nil {
				pgMediaID = sql.NullInt32{Int32: int32(*id.MediaID), Valid: true}
			}
			return r.postgres.UpsertExternalID(ctx, sqlc_postgres.UpsertExternalIDParams{
				MediaID:    pgMediaID,
				MediaType:  string(id.MediaType),
				EntityID:   int32(id.EntityID),
				Provider:   id.Provider,
				ExternalID: id.ExternalID,
			})
		},
		func() error {
			return r.sqlite.UpsertExternalID(ctx, sqlc_sqlite.UpsertExternalIDParams{
				MediaID:    mediaID,
				MediaType:  string(id.MediaType),
				EntityID:   id.EntityID,
				Provider:   id.Provider,
				ExternalID: id.ExternalID,
			})
		},
	)
}

// GetByEntity returns all external IDs for an entity by type and ID.
func (r *ExternalIDRepository) GetByEntity(ctx context.Context, mediaType enrichment.MediaType, entityID int64) ([]*enrichment.ExternalID, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetExternalIDsByMedia(ctx, sqlc_postgres.GetExternalIDsByMediaParams{
				MediaType: string(mediaType),
				EntityID:  int32(entityID),
			})
		},
		func() (any, error) {
			return r.sqlite.GetExternalIDsByMedia(ctx, sqlc_sqlite.GetExternalIDsByMediaParams{
				MediaType: string(mediaType),
				EntityID:  entityID,
			})
		},
	)
	if err != nil {
		return nil, err
	}

	return r.convertResultList(result), nil
}

// GetByMedia returns all external IDs for a media table entry (legacy).
func (r *ExternalIDRepository) GetByMedia(ctx context.Context, mediaID int64) ([]*enrichment.ExternalID, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetExternalIDsByMediaID(ctx, sql.NullInt32{Int32: int32(mediaID), Valid: true})
		},
		func() (any, error) {
			return r.sqlite.GetExternalIDsByMediaID(ctx, sql.NullInt64{Int64: mediaID, Valid: true})
		},
	)
	if err != nil {
		return nil, err
	}

	return r.convertResultList(result), nil
}

// GetByMediaBatch returns external IDs for multiple media IDs in a single query.
// Returns a map of mediaID -> []*ExternalID for efficient batch processing.
func (r *ExternalIDRepository) GetByMediaBatch(ctx context.Context, mediaIDs []int64) (map[int64][]*enrichment.ExternalID, error) {
	if len(mediaIDs) == 0 {
		return make(map[int64][]*enrichment.ExternalID), nil
	}

	result, err := r.router.Route(
		func() (any, error) {
			// Convert to []int32 for PostgreSQL
			pgIDs := make([]int32, len(mediaIDs))
			for i, id := range mediaIDs {
				pgIDs[i] = int32(id)
			}
			return r.postgres.GetExternalIDsByMediaIDBatch(ctx, pgIDs)
		},
		func() (any, error) {
			// Convert to []sql.NullInt64 for SQLite
			sqliteIDs := make([]sql.NullInt64, len(mediaIDs))
			for i, id := range mediaIDs {
				sqliteIDs[i] = sql.NullInt64{Int64: id, Valid: true}
			}
			return r.sqlite.GetExternalIDsByMediaIDBatch(ctx, sqliteIDs)
		},
	)
	if err != nil {
		return nil, err
	}

	// Group results by media ID
	resultMap := make(map[int64][]*enrichment.ExternalID)

	if r.router.IsPostgresDB() {
		pgIDs := result.([]sqlc_postgres.MediaExternalID)
		for _, pgID := range pgIDs {
			if pgID.MediaID.Valid {
				mediaID := int64(pgID.MediaID.Int32)
				resultMap[mediaID] = append(resultMap[mediaID], r.convertPostgresExternalID(pgID))
			}
		}
	} else {
		sqIDs := result.([]sqlc_sqlite.MediaExternalID)
		for _, sqID := range sqIDs {
			if sqID.MediaID.Valid {
				mediaID := sqID.MediaID.Int64
				resultMap[mediaID] = append(resultMap[mediaID], r.convertSqliteExternalID(sqID))
			}
		}
	}

	return resultMap, nil
}

// GetEntityByExternalID finds an entity by provider and external ID.
func (r *ExternalIDRepository) GetEntityByExternalID(ctx context.Context, provider, externalID string) (enrichment.MediaType, int64, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetEntityByExternalID(ctx, sqlc_postgres.GetEntityByExternalIDParams{
				Provider:   provider,
				ExternalID: externalID,
			})
		},
		func() (any, error) {
			return r.sqlite.GetEntityByExternalID(ctx, sqlc_sqlite.GetEntityByExternalIDParams{
				Provider:   provider,
				ExternalID: externalID,
			})
		},
	)
	if err != nil {
		return "", 0, err
	}

	if r.router.IsPostgresDB() {
		row := result.(sqlc_postgres.GetEntityByExternalIDRow)
		// PostgreSQL enum comes through as interface{}, need type assertion
		mediaType := row.MediaType.(string)
		return enrichment.MediaType(mediaType), int64(row.EntityID), nil
	}

	row := result.(sqlc_sqlite.GetEntityByExternalIDRow)
	return enrichment.MediaType(row.MediaType), row.EntityID, nil
}

// GetMediaByExternalID finds a media item by provider and external ID (legacy).
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

// DeleteByEntity removes all external IDs for an entity.
func (r *ExternalIDRepository) DeleteByEntity(ctx context.Context, mediaType enrichment.MediaType, entityID int64) error {
	return r.router.RouteVoid(
		func() error {
			return r.postgres.DeleteExternalIDsByMedia(ctx, sqlc_postgres.DeleteExternalIDsByMediaParams{
				MediaType: string(mediaType),
				EntityID:  int32(entityID),
			})
		},
		func() error {
			return r.sqlite.DeleteExternalIDsByMedia(ctx, sqlc_sqlite.DeleteExternalIDsByMediaParams{
				MediaType: string(mediaType),
				EntityID:  entityID,
			})
		},
	)
}

// DeleteByMedia removes all external IDs for a media table entry (legacy).
func (r *ExternalIDRepository) DeleteByMedia(ctx context.Context, mediaID int64) error {
	return r.router.RouteVoid(
		func() error {
			return r.postgres.DeleteExternalIDsByMediaID(ctx, sql.NullInt32{Int32: int32(mediaID), Valid: true})
		},
		func() error {
			return r.sqlite.DeleteExternalIDsByMediaID(ctx, sql.NullInt64{Int64: mediaID, Valid: true})
		},
	)
}

// convertResultList converts sqlc result list to domain ExternalID list.
func (r *ExternalIDRepository) convertResultList(result any) []*enrichment.ExternalID {
	if r.router.IsPostgresDB() {
		pgIDs := result.([]sqlc_postgres.MediaExternalID)
		ids := make([]*enrichment.ExternalID, len(pgIDs))
		for i, pgID := range pgIDs {
			ids[i] = r.convertPostgresExternalID(pgID)
		}
		return ids
	}

	sqIDs := result.([]sqlc_sqlite.MediaExternalID)
	ids := make([]*enrichment.ExternalID, len(sqIDs))
	for i, sqID := range sqIDs {
		ids[i] = r.convertSqliteExternalID(sqID)
	}
	return ids
}

// convertPostgresExternalID converts a Postgres sqlc result to domain ExternalID.
func (r *ExternalIDRepository) convertPostgresExternalID(pgID sqlc_postgres.MediaExternalID) *enrichment.ExternalID {
	var mediaID *int64
	if pgID.MediaID.Valid {
		v := int64(pgID.MediaID.Int32)
		mediaID = &v
	}

	// PostgreSQL enum comes through as interface{}, need type assertion
	mediaType := pgID.MediaType.(string)

	return &enrichment.ExternalID{
		ID:         int64(pgID.ID),
		MediaID:    mediaID,
		MediaType:  enrichment.MediaType(mediaType),
		EntityID:   int64(pgID.EntityID),
		Provider:   pgID.Provider,
		ExternalID: pgID.ExternalID,
		CreatedAt:  common.ParseNullTime(pgID.CreatedAt),
		UpdatedAt:  common.ParseNullTime(pgID.UpdatedAt),
	}
}

// convertSqliteExternalID converts a SQLite sqlc result to domain ExternalID.
func (r *ExternalIDRepository) convertSqliteExternalID(sqID sqlc_sqlite.MediaExternalID) *enrichment.ExternalID {
	var mediaID *int64
	if sqID.MediaID.Valid {
		mediaID = &sqID.MediaID.Int64
	}

	return &enrichment.ExternalID{
		ID:         sqID.ID,
		MediaID:    mediaID,
		MediaType:  enrichment.MediaType(sqID.MediaType),
		EntityID:   sqID.EntityID,
		Provider:   sqID.Provider,
		ExternalID: sqID.ExternalID,
		CreatedAt:  parseTimeString(sqID.CreatedAt),
		UpdatedAt:  parseTimeString(sqID.UpdatedAt),
	}
}
