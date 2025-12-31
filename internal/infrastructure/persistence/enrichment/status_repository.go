package enrichment

import (
	"context"
	"database/sql"

	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// NewStatusRepository creates a new enrichment status repository with the appropriate database driver.
func NewStatusRepository(db *common.BaseRepository) *StatusRepository {
	return &StatusRepository{
		BaseRepository: db,
	}
}

// Upsert creates or updates the status for a media/stage pair.
func (r *StatusRepository) Upsert(ctx context.Context, status *enrichment.Status) error {
	var completedAt sql.NullTime
	if status.CompletedAt != nil {
		completedAt = sql.NullTime{Time: *status.CompletedAt, Valid: true}
	}
	return r.Q().UpsertEnrichmentStatus(ctx, unified.UpsertEnrichmentStatusParams{
		MediaType:    string(status.MediaType),
		MediaID:      status.MediaID,
		Stage:        status.Stage,
		Status:       sql.NullString{String: string(status.Status), Valid: status.Status != ""},
		PluginID:     sql.NullString{String: status.PluginID, Valid: status.PluginID != ""},
		CompletedAt:  completedAt,
		ErrorMessage: sql.NullString{String: status.ErrorMessage, Valid: status.ErrorMessage != ""},
		MetadataJson: sql.NullString{String: status.MetadataJSON, Valid: status.MetadataJSON != ""},
	})
}

// GetByMedia returns all enrichment statuses for a media item.
func (r *StatusRepository) GetByMedia(ctx context.Context, mediaType enrichment.MediaType, mediaID int64) ([]*enrichment.Status, error) {
	rows, err := r.Q().GetEnrichmentStatusByMedia(ctx, unified.GetEnrichmentStatusByMediaParams{
		MediaType: string(mediaType),
		MediaID:   mediaID,
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, convertEnrichmentStatus), nil
}

// MarkComplete marks a stage as completed for a media item.
func (r *StatusRepository) MarkComplete(ctx context.Context, mediaType enrichment.MediaType, mediaID int64, stage, pluginID, metadataJSON string) error {
	return r.Q().MarkEnrichmentComplete(ctx, unified.MarkEnrichmentCompleteParams{
		PluginID:     sql.NullString{String: pluginID, Valid: pluginID != ""},
		MetadataJson: sql.NullString{String: metadataJSON, Valid: metadataJSON != ""},
		MediaType:    string(mediaType),
		MediaID:      mediaID,
		Stage:        stage,
	})
}

// MarkFailed marks a stage as failed for a media item.
func (r *StatusRepository) MarkFailed(ctx context.Context, mediaType enrichment.MediaType, mediaID int64, stage, pluginID, errorMsg string) error {
	return r.Q().MarkEnrichmentFailed(ctx, unified.MarkEnrichmentFailedParams{
		PluginID:     sql.NullString{String: pluginID, Valid: pluginID != ""},
		ErrorMessage: sql.NullString{String: errorMsg, Valid: errorMsg != ""},
		MediaType:    string(mediaType),
		MediaID:      mediaID,
		Stage:        stage,
	})
}

// MarkSkipped marks a stage as skipped for a media item.
func (r *StatusRepository) MarkSkipped(ctx context.Context, mediaType enrichment.MediaType, mediaID int64, stage, pluginID string) error {
	return r.Q().MarkEnrichmentSkipped(ctx, unified.MarkEnrichmentSkippedParams{
		PluginID:  sql.NullString{String: pluginID, Valid: pluginID != ""},
		MediaType: string(mediaType),
		MediaID:   mediaID,
		Stage:     stage,
	})
}

// GetLibraryProgress returns enrichment progress for a library.
func (r *StatusRepository) GetLibraryProgress(ctx context.Context, libraryID int64) (map[string]*enrichment.QueueStats, error) {
	rows, err := r.Q().GetLibraryEnrichmentProgress(ctx, libraryID)
	if err != nil {
		return nil, err
	}

	progress := make(map[string]*enrichment.QueueStats)
	for _, row := range rows {
		progress[row.Stage] = convertLibraryProgress(row)
	}
	return progress, nil
}

// DeleteByMedia removes all status records for a media item.
func (r *StatusRepository) DeleteByMedia(ctx context.Context, mediaType enrichment.MediaType, mediaID int64) error {
	return r.Q().DeleteEnrichmentStatusByMedia(ctx, unified.DeleteEnrichmentStatusByMediaParams{
		MediaType: string(mediaType),
		MediaID:   mediaID,
	})
}

// ResetStuck resets all 'processing' status records to 'pending'.
// Called at startup to recover from crashed workers.
func (r *StatusRepository) ResetStuck(ctx context.Context) (int64, error) {
	return r.Q().ResetStuckEnrichmentStatus(ctx)
}

// OverallProgress represents the overall enrichment progress for a library.
// This counts unique media items rather than stage entries.
type OverallProgress struct {
	// TotalItems is the number of unique media items that have entered enrichment.
	TotalItems int64
	// RemainingItems is the number of unique media items still being processed.
	RemainingItems int64
	// CompletedItems is TotalItems - RemainingItems.
	CompletedItems int64
}

// GetOverallProgress returns the overall enrichment progress for a library.
// Unlike per-stage progress, this counts unique media items to avoid inflating totals.
func (r *StatusRepository) GetOverallProgress(ctx context.Context, libraryID int64) (*OverallProgress, error) {
	row, err := r.Q().GetLibraryEnrichmentOverallProgress(ctx, sql.NullInt64{Int64: libraryID, Valid: true})
	if err != nil {
		return nil, err
	}
	return &OverallProgress{
		TotalItems:     row.TotalItems,
		RemainingItems: row.RemainingItems,
		CompletedItems: row.TotalItems - row.RemainingItems,
	}, nil
}
