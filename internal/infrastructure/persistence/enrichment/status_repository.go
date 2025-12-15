package enrichment

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/adapters"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
	"github.com/sqlc-dev/pqtype"
)

// NewStatusRepository creates a new enrichment status repository with the appropriate database driver.
func NewStatusRepository(db *sql.DB, driver string) *StatusRepository {
	r := &StatusRepository{
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

// Upsert creates or updates the status for a media/stage pair.
func (r *StatusRepository) Upsert(ctx context.Context, status *enrichment.Status) error {
	return r.router.RouteVoid(
		func() error {
			var completedAt sql.NullTime
			if status.CompletedAt != nil {
				completedAt = sql.NullTime{Time: *status.CompletedAt, Valid: true}
			}
			return r.postgres.UpsertEnrichmentStatus(ctx, sqlc_postgres.UpsertEnrichmentStatusParams{
				MediaID:      int32(status.MediaID),
				Stage:        status.Stage,
				Status:       sql.NullString{String: string(status.Status), Valid: status.Status != ""},
				PluginID:     sql.NullString{String: status.PluginID, Valid: status.PluginID != ""},
				CompletedAt:  completedAt,
				ErrorMessage: sql.NullString{String: status.ErrorMessage, Valid: status.ErrorMessage != ""},
				MetadataJson: statusStringToNullRawMessage(status.MetadataJSON),
			})
		},
		func() error {
			var completedAtStr sql.NullString
			if status.CompletedAt != nil {
				completedAtStr = sql.NullString{String: status.CompletedAt.Format("2006-01-02 15:04:05"), Valid: true}
			}
			return r.sqlite.UpsertEnrichmentStatus(ctx, sqlc_sqlite.UpsertEnrichmentStatusParams{
				MediaID:      status.MediaID,
				Stage:        status.Stage,
				Status:       sql.NullString{String: string(status.Status), Valid: status.Status != ""},
				PluginID:     sql.NullString{String: status.PluginID, Valid: status.PluginID != ""},
				CompletedAt:  completedAtStr,
				ErrorMessage: sql.NullString{String: status.ErrorMessage, Valid: status.ErrorMessage != ""},
				MetadataJson: sql.NullString{String: status.MetadataJSON, Valid: status.MetadataJSON != ""},
			})
		},
	)
}

// GetByMedia returns all enrichment statuses for a media item.
func (r *StatusRepository) GetByMedia(ctx context.Context, mediaID int64) ([]*enrichment.Status, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetEnrichmentStatusByMedia(ctx, int32(mediaID))
		},
		func() (any, error) {
			return r.sqlite.GetEnrichmentStatusByMedia(ctx, mediaID)
		},
	)
	if err != nil {
		return nil, err
	}

	if r.router.IsPostgresDB() {
		pgStatuses := result.([]sqlc_postgres.EnrichmentStatus)
		statuses := make([]*enrichment.Status, len(pgStatuses))
		for i, pgStatus := range pgStatuses {
			statuses[i] = r.convertToStatus(pgStatus)
		}
		return statuses, nil
	}

	sqStatuses := result.([]sqlc_sqlite.EnrichmentStatus)
	statuses := make([]*enrichment.Status, len(sqStatuses))
	for i, sqStatus := range sqStatuses {
		statuses[i] = r.convertToStatus(sqStatus)
	}
	return statuses, nil
}

// MarkComplete marks a stage as completed for a media item.
func (r *StatusRepository) MarkComplete(ctx context.Context, mediaID int64, stage, pluginID, metadataJSON string) error {
	return r.router.RouteVoid(
		func() error {
			return r.postgres.MarkEnrichmentComplete(ctx, sqlc_postgres.MarkEnrichmentCompleteParams{
				PluginID:     sql.NullString{String: pluginID, Valid: pluginID != ""},
				MetadataJson: statusStringToNullRawMessage(metadataJSON),
				MediaID:      int32(mediaID),
				Stage:        stage,
			})
		},
		func() error {
			return r.sqlite.MarkEnrichmentComplete(ctx, sqlc_sqlite.MarkEnrichmentCompleteParams{
				PluginID:     sql.NullString{String: pluginID, Valid: pluginID != ""},
				MetadataJson: sql.NullString{String: metadataJSON, Valid: metadataJSON != ""},
				MediaID:      mediaID,
				Stage:        stage,
			})
		},
	)
}

// MarkFailed marks a stage as failed for a media item.
func (r *StatusRepository) MarkFailed(ctx context.Context, mediaID int64, stage, pluginID, errorMsg string) error {
	return r.router.RouteVoid(
		func() error {
			return r.postgres.MarkEnrichmentFailed(ctx, sqlc_postgres.MarkEnrichmentFailedParams{
				PluginID:     sql.NullString{String: pluginID, Valid: pluginID != ""},
				ErrorMessage: sql.NullString{String: errorMsg, Valid: errorMsg != ""},
				MediaID:      int32(mediaID),
				Stage:        stage,
			})
		},
		func() error {
			return r.sqlite.MarkEnrichmentFailed(ctx, sqlc_sqlite.MarkEnrichmentFailedParams{
				PluginID:     sql.NullString{String: pluginID, Valid: pluginID != ""},
				ErrorMessage: sql.NullString{String: errorMsg, Valid: errorMsg != ""},
				MediaID:      mediaID,
				Stage:        stage,
			})
		},
	)
}

// MarkSkipped marks a stage as skipped for a media item.
func (r *StatusRepository) MarkSkipped(ctx context.Context, mediaID int64, stage, pluginID string) error {
	return r.router.RouteVoid(
		func() error {
			return r.postgres.MarkEnrichmentSkipped(ctx, sqlc_postgres.MarkEnrichmentSkippedParams{
				PluginID: sql.NullString{String: pluginID, Valid: pluginID != ""},
				MediaID:  int32(mediaID),
				Stage:    stage,
			})
		},
		func() error {
			return r.sqlite.MarkEnrichmentSkipped(ctx, sqlc_sqlite.MarkEnrichmentSkippedParams{
				PluginID: sql.NullString{String: pluginID, Valid: pluginID != ""},
				MediaID:  mediaID,
				Stage:    stage,
			})
		},
	)
}

// GetLibraryProgress returns enrichment progress for a library.
func (r *StatusRepository) GetLibraryProgress(ctx context.Context, libraryID int64) (map[string]*enrichment.QueueStats, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetLibraryEnrichmentProgress(ctx, int32(libraryID))
		},
		func() (any, error) {
			return r.sqlite.GetLibraryEnrichmentProgress(ctx, libraryID)
		},
	)
	if err != nil {
		return nil, err
	}

	progress := make(map[string]*enrichment.QueueStats)

	if r.router.IsPostgresDB() {
		pgRows := result.([]sqlc_postgres.GetLibraryEnrichmentProgressRow)
		for _, row := range pgRows {
			progress[row.Stage] = &enrichment.QueueStats{
				Stage:           row.Stage,
				PendingCount:    row.PendingCount,
				ProcessingCount: row.ProcessingCount,
				CompletedCount:  row.CompletedCount,
				FailedCount:     row.FailedCount,
				SkippedCount:    row.SkippedCount,
				TotalCount:      row.TotalCount,
			}
		}
		return progress, nil
	}

	sqRows := result.([]sqlc_sqlite.GetLibraryEnrichmentProgressRow)
	for _, row := range sqRows {
		progress[row.Stage] = &enrichment.QueueStats{
			Stage:           row.Stage,
			PendingCount:    int64(row.PendingCount.Float64),
			ProcessingCount: int64(row.ProcessingCount.Float64),
			CompletedCount:  int64(row.CompletedCount.Float64),
			FailedCount:     int64(row.FailedCount.Float64),
			SkippedCount:    int64(row.SkippedCount.Float64),
			TotalCount:      row.TotalCount,
		}
	}
	return progress, nil
}

// DeleteByMedia removes all status records for a media item.
func (r *StatusRepository) DeleteByMedia(ctx context.Context, mediaID int64) error {
	return r.router.RouteVoid(
		func() error {
			return r.postgres.DeleteEnrichmentStatusByMedia(ctx, int32(mediaID))
		},
		func() error {
			return r.sqlite.DeleteEnrichmentStatusByMedia(ctx, mediaID)
		},
	)
}

// convertToStatus converts sqlc result to domain Status.
func (r *StatusRepository) convertToStatus(result any) *enrichment.Status {
	if r.router.IsPostgresDB() {
		pgStatus := result.(sqlc_postgres.EnrichmentStatus)
		return &enrichment.Status{
			MediaID:      int64(pgStatus.MediaID),
			Stage:        pgStatus.Stage,
			Status:       enrichment.JobStatus(common.ParseNullString(pgStatus.Status)),
			PluginID:     common.ParseNullString(pgStatus.PluginID),
			CompletedAt:  common.ParseNullTimePtr(pgStatus.CompletedAt),
			ErrorMessage: common.ParseNullString(pgStatus.ErrorMessage),
			MetadataJSON: statusNullRawMessageToString(pgStatus.MetadataJson),
		}
	}

	sqStatus := result.(sqlc_sqlite.EnrichmentStatus)
	return &enrichment.Status{
		MediaID:      sqStatus.MediaID,
		Stage:        sqStatus.Stage,
		Status:       enrichment.JobStatus(common.ParseNullString(sqStatus.Status)),
		PluginID:     common.ParseNullString(sqStatus.PluginID),
		CompletedAt:  parseNullTimeString(sqStatus.CompletedAt),
		ErrorMessage: common.ParseNullString(sqStatus.ErrorMessage),
		MetadataJSON: common.ParseNullString(sqStatus.MetadataJson),
	}
}

// statusStringToNullRawMessage converts a string to pqtype.NullRawMessage for PostgreSQL.
func statusStringToNullRawMessage(s string) pqtype.NullRawMessage {
	if s == "" {
		return pqtype.NullRawMessage{Valid: false}
	}
	return pqtype.NullRawMessage{
		RawMessage: json.RawMessage(s),
		Valid:      true,
	}
}

// statusNullRawMessageToString converts pqtype.NullRawMessage to string.
func statusNullRawMessageToString(m pqtype.NullRawMessage) string {
	if !m.Valid {
		return ""
	}
	return string(m.RawMessage)
}
