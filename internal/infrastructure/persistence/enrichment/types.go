package enrichment

import (
	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// QueueRepository implements enrichment.QueueRepository using sqlc.
// It embeds BaseRepository for dual-database support.
type QueueRepository struct {
	*common.BaseRepository
}

// StatusRepository implements enrichment.StatusRepository using sqlc.
// It embeds BaseRepository for dual-database support.
type StatusRepository struct {
	*common.BaseRepository
}

// PipelineRepository implements enrichment.PipelineRepository using sqlc.
// It embeds BaseRepository for dual-database support.
type PipelineRepository struct {
	*common.BaseRepository
}

// ExternalIDRepository implements enrichment.ExternalIDRepository using sqlc.
// It embeds BaseRepository for dual-database support.
type ExternalIDRepository struct {
	*common.BaseRepository
}

// MetadataSourceRepository implements enrichment.MetadataSourceRepository using sqlc.
// It embeds BaseRepository for dual-database support.
type MetadataSourceRepository struct {
	*common.BaseRepository
}

// ========================================
// Unified Row Converters
// ========================================
// Since PostgreSQL and SQLite types are now structurally identical,
// we use a single converter per type using unified type aliases.

// convertExternalID converts a unified MediaExternalID to domain ExternalID.
func convertExternalID(row unified.MediaExternalID) *enrichment.ExternalID {
	var mediaID *int64
	if row.MediaID.Valid {
		v := row.MediaID.Int64
		mediaID = &v
	}

	return &enrichment.ExternalID{
		ID:         row.ID,
		MediaID:    mediaID,
		MediaType:  enrichment.MediaType(row.MediaType),
		EntityID:   row.EntityID,
		Provider:   row.Provider,
		ExternalID: row.ExternalID,
		CreatedAt:  common.ParseNullTime(row.CreatedAt),
		UpdatedAt:  common.ParseNullTime(row.UpdatedAt),
	}
}

// convertMetadataSource converts a unified MediaMetadataSource to domain MetadataSource.
func convertMetadataSource(row unified.MediaMetadataSource) *enrichment.MetadataSource {
	return &enrichment.MetadataSource{
		MediaID:   row.MediaID,
		FieldName: row.FieldName,
		PluginID:  row.PluginID,
		RawValue:  common.ParseNullString(row.RawValue),
		UpdatedAt: common.ParseNullTime(row.UpdatedAt),
	}
}

// convertPipelineStage converts a unified EnrichmentPipeline to domain PipelineStage.
func convertPipelineStage(row unified.EnrichmentPipeline) *enrichment.PipelineStage {
	return &enrichment.PipelineStage{
		ID:         row.ID,
		MediaType:  enrichment.MediaType(row.MediaType),
		PluginID:   row.PluginID,
		StageName:  row.StageName,
		Position:   int(row.Position),
		Enabled:    row.Enabled.Int64 == 1,
		ConfigJSON: common.ParseNullString(row.ConfigJson),
		CreatedAt:  common.ParseNullTime(row.CreatedAt),
		UpdatedAt:  common.ParseNullTime(row.UpdatedAt),
	}
}

// convertQueueJob converts a unified EnrichmentQueue to domain QueueJob.
func convertQueueJob(row unified.EnrichmentQueue) *enrichment.QueueJob {
	return &enrichment.QueueJob{
		ID:            row.ID,
		MediaID:       row.MediaID,
		LibraryID:     row.LibraryID.Int64,
		MediaType:     enrichment.MediaType(row.MediaType),
		Stage:         row.Stage,
		Priority:      int(row.Priority.Int64),
		Status:        enrichment.JobStatus(common.ParseNullString(row.Status)),
		Attempts:      int(row.Attempts.Int64),
		MaxAttempts:   int(row.MaxAttempts.Int64),
		ErrorMessage:  common.ParseNullString(row.ErrorMessage),
		ErrorCategory: enrichment.ErrorCategory(common.ParseNullString(row.ErrorCategory)),
		NextRetryAt:   common.ParseNullTimePtr(row.NextRetryAt),
		LockedBy:      common.ParseNullString(row.LockedBy),
		LockedAt:      common.ParseNullTimePtr(row.LockedAt),
		CreatedAt:     common.ParseNullTime(row.CreatedAt),
		UpdatedAt:     common.ParseNullTime(row.UpdatedAt),
	}
}

// convertEnrichmentStatus converts a unified EnrichmentStatus to domain Status.
func convertEnrichmentStatus(row unified.EnrichmentStatus) *enrichment.Status {
	return &enrichment.Status{
		MediaType:    enrichment.MediaType(row.MediaType),
		MediaID:      row.MediaID,
		Stage:        row.Stage,
		Status:       enrichment.JobStatus(common.ParseNullString(row.Status)),
		PluginID:     common.ParseNullString(row.PluginID),
		CompletedAt:  common.ParseNullTimePtr(row.CompletedAt),
		ErrorMessage: common.ParseNullString(row.ErrorMessage),
		MetadataJSON: common.ParseNullString(row.MetadataJson),
	}
}

// convertQueueStats converts a unified GetEnrichmentQueueStatsRow to domain QueueStats.
func convertQueueStats(row unified.GetEnrichmentQueueStatsRow) *enrichment.QueueStats {
	return &enrichment.QueueStats{
		Stage:           row.Stage,
		PendingCount:    row.PendingCount,
		ProcessingCount: row.ProcessingCount,
		CompletedCount:  row.CompletedCount,
		FailedCount:     row.FailedCount,
		SkippedCount:    row.SkippedCount,
		TotalCount:      row.TotalCount,
	}
}

// convertLibraryProgress converts a unified GetLibraryEnrichmentProgressRow to domain QueueStats.
func convertLibraryProgress(row unified.GetLibraryEnrichmentProgressRow) *enrichment.QueueStats {
	return &enrichment.QueueStats{
		Stage:           row.Stage,
		PendingCount:    row.PendingCount,
		ProcessingCount: row.ProcessingCount,
		CompletedCount:  row.CompletedCount,
		FailedCount:     row.FailedCount,
		SkippedCount:    row.SkippedCount,
		TotalCount:      row.TotalCount,
	}
}

// mapSlice converts a slice of one type to another using the provided mapper function.
func mapSlice[TFrom, TTo any](from []TFrom, mapper func(TFrom) TTo) []TTo {
	result := make([]TTo, len(from))
	for i, v := range from {
		result[i] = mapper(v)
	}
	return result
}
