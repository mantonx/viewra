package enrichment

import (
	"context"
	"database/sql"

	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// NewMetadataSourceRepository creates a new metadata source repository with the appropriate database driver.
func NewMetadataSourceRepository(db *common.BaseRepository) *MetadataSourceRepository {
	return &MetadataSourceRepository{
		BaseRepository: db,
	}
}

// Upsert creates or updates a metadata source record.
func (r *MetadataSourceRepository) Upsert(ctx context.Context, source *enrichment.MetadataSource) error {
	return r.Q().UpsertMetadataSource(ctx, unified.UpsertMetadataSourceParams{
		MediaID:   source.MediaID,
		FieldName: source.FieldName,
		PluginID:  source.PluginID,
		RawValue:  sql.NullString{String: source.RawValue, Valid: source.RawValue != ""},
	})
}

// GetByMedia returns all metadata sources for a media item.
func (r *MetadataSourceRepository) GetByMedia(ctx context.Context, mediaID int64) ([]*enrichment.MetadataSource, error) {
	rows, err := r.Q().GetMetadataSourcesByMedia(ctx, mediaID)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, convertMetadataSource), nil
}

// DeleteByMedia removes all metadata sources for a media item.
func (r *MetadataSourceRepository) DeleteByMedia(ctx context.Context, mediaID int64) error {
	return r.Q().DeleteMetadataSourcesByMedia(ctx, mediaID)
}

// DeleteByPlugin removes all metadata sources for a plugin.
func (r *MetadataSourceRepository) DeleteByPlugin(ctx context.Context, pluginID string) error {
	return r.Q().DeleteMetadataSourcesByPlugin(ctx, pluginID)
}
