package enrichment

import (
	"context"
	"database/sql"

	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// NewExternalIDRepository creates a new external ID repository with the appropriate database driver.
func NewExternalIDRepository(db *common.BaseRepository) *ExternalIDRepository {
	return &ExternalIDRepository{
		BaseRepository: db,
	}
}

// Upsert creates or updates an external ID.
// The ExternalID must have MediaType and EntityID set.
func (r *ExternalIDRepository) Upsert(ctx context.Context, id *enrichment.ExternalID) error {
	// Convert optional MediaID to sql.NullInt64
	var mediaID sql.NullInt64
	if id.MediaID != nil {
		mediaID = sql.NullInt64{Int64: *id.MediaID, Valid: true}
	}

	return r.Q().UpsertExternalID(ctx, unified.UpsertExternalIDParams{
		MediaID:    mediaID,
		MediaType:  string(id.MediaType),
		EntityID:   id.EntityID,
		Provider:   id.Provider,
		ExternalID: id.ExternalID,
	})
}

// GetByEntity returns all external IDs for an entity by type and ID.
func (r *ExternalIDRepository) GetByEntity(ctx context.Context, mediaType enrichment.MediaType, entityID int64) ([]*enrichment.ExternalID, error) {
	rows, err := r.Q().GetExternalIDsByMedia(ctx, unified.GetExternalIDsByMediaParams{
		MediaType: string(mediaType),
		EntityID:  entityID,
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, convertExternalID), nil
}

// GetByMedia returns all external IDs for a media table entry (legacy).
func (r *ExternalIDRepository) GetByMedia(ctx context.Context, mediaID int64) ([]*enrichment.ExternalID, error) {
	rows, err := r.Q().GetExternalIDsByMediaID(ctx, sql.NullInt64{Int64: mediaID, Valid: true})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, convertExternalID), nil
}

// GetByMediaBatch returns external IDs for multiple media IDs in a single query.
// Returns a map of mediaID -> []*ExternalID for efficient batch processing.
func (r *ExternalIDRepository) GetByMediaBatch(ctx context.Context, mediaIDs []int64) (map[int64][]*enrichment.ExternalID, error) {
	if len(mediaIDs) == 0 {
		return make(map[int64][]*enrichment.ExternalID), nil
	}

	rows, err := r.Q().GetExternalIDsByMediaIDBatch(ctx, mediaIDs)
	if err != nil {
		return nil, err
	}

	// Group results by media ID
	resultMap := make(map[int64][]*enrichment.ExternalID)
	for _, row := range rows {
		if row.MediaID.Valid {
			mediaID := row.MediaID.Int64
			resultMap[mediaID] = append(resultMap[mediaID], convertExternalID(row))
		}
	}

	return resultMap, nil
}

// GetEntityByExternalID finds an entity by provider and external ID.
func (r *ExternalIDRepository) GetEntityByExternalID(ctx context.Context, provider, externalID string) (enrichment.MediaType, int64, error) {
	row, err := r.Q().GetEntityByExternalID(ctx, unified.GetEntityByExternalIDParams{
		Provider:   provider,
		ExternalID: externalID,
	})
	if err != nil {
		return "", 0, err
	}
	return enrichment.MediaType(row.MediaType), row.EntityID, nil
}

// GetMediaByExternalID finds a media item by provider and external ID (legacy).
func (r *ExternalIDRepository) GetMediaByExternalID(ctx context.Context, provider, externalID string) (int64, error) {
	return r.Q().GetMediaByExternalID(ctx, unified.GetMediaByExternalIDParams{
		Provider:   provider,
		ExternalID: externalID,
	})
}

// DeleteByEntity removes all external IDs for an entity.
func (r *ExternalIDRepository) DeleteByEntity(ctx context.Context, mediaType enrichment.MediaType, entityID int64) error {
	return r.Q().DeleteExternalIDsByMedia(ctx, unified.DeleteExternalIDsByMediaParams{
		MediaType: string(mediaType),
		EntityID:  entityID,
	})
}

// DeleteByMedia removes all external IDs for a media table entry (legacy).
func (r *ExternalIDRepository) DeleteByMedia(ctx context.Context, mediaID int64) error {
	return r.Q().DeleteExternalIDsByMediaID(ctx, sql.NullInt64{Int64: mediaID, Valid: true})
}
