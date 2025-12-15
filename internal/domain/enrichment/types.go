// Package enrichment defines domain types and interfaces for the media enrichment system.
// Enrichment is the process of fetching and merging metadata from various sources
// (NFO files, TMDb, MusicBrainz, etc.) after media files are discovered by the scanner.
package enrichment

import (
	"context"
	"time"
)

// MediaType represents the type of media being enriched.
type MediaType string

const (
	MediaTypeMovie  MediaType = "movie"
	MediaTypeTV     MediaType = "tv"      // TV episode
	MediaTypeTVShow MediaType = "tv_show" // TV show (parent entity)
	MediaTypeMusic  MediaType = "music"
)

// JobStatus represents the status of an enrichment queue job.
type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusProcessing JobStatus = "processing"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
	JobStatusSkipped    JobStatus = "skipped"
)

// ErrorCategory classifies errors for retry logic and reporting.
type ErrorCategory string

const (
	ErrorCategoryNetwork   ErrorCategory = "network"    // Connection failed, timeout
	ErrorCategoryRateLimit ErrorCategory = "rate_limit" // API rate limited
	ErrorCategoryNotFound  ErrorCategory = "not_found"  // No match in external DB
	ErrorCategoryParsing   ErrorCategory = "parsing"    // Failed to parse file/response
	ErrorCategoryPlugin    ErrorCategory = "plugin"     // Plugin crashed or returned error
	ErrorCategoryDatabase  ErrorCategory = "database"   // Database operation failed
)

// QueueJob represents a job in the enrichment queue.
// Jobs are created when media is discovered and processed through the enrichment pipeline.
type QueueJob struct {
	ID            int64
	MediaID       int64
	MediaType     MediaType // Type of entity (movie, tv, tv_show, music) - determines which table MediaID references
	Stage         string
	Priority      int
	Status        JobStatus
	Attempts      int
	MaxAttempts   int
	ErrorMessage  string
	ErrorCategory ErrorCategory
	NextRetryAt   *time.Time
	LockedBy      string
	LockedAt      *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ShouldRetry returns true if this job can be retried.
func (j *QueueJob) ShouldRetry() bool {
	return j.Attempts < j.MaxAttempts
}

// IsRetryable returns true if the error category allows retries.
func (c ErrorCategory) IsRetryable() bool {
	switch c {
	case ErrorCategoryNetwork, ErrorCategoryRateLimit:
		return true
	default:
		return false
	}
}

// QueueStats provides aggregate statistics for a queue stage.
type QueueStats struct {
	Stage           string
	PendingCount    int64
	ProcessingCount int64
	CompletedCount  int64
	FailedCount     int64
	SkippedCount    int64
	TotalCount      int64
}

// Status represents the enrichment status for a media item at a specific stage.
type Status struct {
	MediaID      int64
	Stage        string
	Status       JobStatus
	PluginID     string
	CompletedAt  *time.Time
	ErrorMessage string
	MetadataJSON string
}

// PipelineStage represents a configured enrichment stage in a pipeline.
type PipelineStage struct {
	ID         int64
	MediaType  MediaType
	PluginID   string
	StageName  string
	Position   int
	Enabled    bool
	ConfigJSON string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ExternalID represents an external provider's ID for a media item.
type ExternalID struct {
	MediaID    int64
	Provider   string
	ExternalID string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// MetadataSource tracks which plugin provided a specific metadata field.
type MetadataSource struct {
	MediaID   int64
	FieldName string
	PluginID  string
	RawValue  string
	UpdatedAt time.Time
}

// QueueRepository defines operations for the enrichment queue.
type QueueRepository interface {
	// Enqueue adds or updates a job in the queue.
	// Uses upsert behavior: re-enqueues completed/skipped/failed jobs.
	Enqueue(ctx context.Context, job *QueueJob) (*QueueJob, error)

	// ClaimBatch claims a batch of pending jobs for processing.
	// Returns jobs locked to the specified worker.
	ClaimBatch(ctx context.Context, stage, workerID string, batchSize int) ([]*QueueJob, error)

	// Complete marks a job as successfully completed.
	Complete(ctx context.Context, jobID int64) error

	// Fail marks a job as failed with error details.
	// Automatically schedules retry if attempts < maxAttempts.
	Fail(ctx context.Context, jobID int64, errMsg string, category ErrorCategory, nextRetryAt *time.Time) error

	// Skip marks a job as skipped (no match found).
	Skip(ctx context.Context, jobID int64) error

	// GetStats returns queue statistics for a stage.
	GetStats(ctx context.Context, stage string) (*QueueStats, error)

	// ReleaseStuck releases jobs that have been locked too long.
	ReleaseStuck(ctx context.Context, maxLockSeconds int) error

	// DeleteByMedia removes all queue jobs for a media item of a specific type.
	DeleteByMedia(ctx context.Context, mediaID int64, mediaType MediaType) error
}

// StatusRepository defines operations for enrichment status tracking.
type StatusRepository interface {
	// Upsert creates or updates the status for a media/stage pair.
	Upsert(ctx context.Context, status *Status) error

	// GetByMedia returns all enrichment statuses for a media item.
	GetByMedia(ctx context.Context, mediaID int64) ([]*Status, error)

	// MarkComplete marks a stage as completed for a media item.
	MarkComplete(ctx context.Context, mediaID int64, stage, pluginID, metadataJSON string) error

	// MarkFailed marks a stage as failed for a media item.
	MarkFailed(ctx context.Context, mediaID int64, stage, pluginID, errorMsg string) error

	// MarkSkipped marks a stage as skipped for a media item.
	MarkSkipped(ctx context.Context, mediaID int64, stage, pluginID string) error

	// GetLibraryProgress returns enrichment progress for a library.
	GetLibraryProgress(ctx context.Context, libraryID int64) (map[string]*QueueStats, error)

	// DeleteByMedia removes all status records for a media item.
	DeleteByMedia(ctx context.Context, mediaID int64) error
}

// PipelineRepository defines operations for pipeline configuration.
type PipelineRepository interface {
	// Create adds a new stage to a pipeline.
	Create(ctx context.Context, stage *PipelineStage) (*PipelineStage, error)

	// GetEnabledStages returns all enabled stages for a media type, ordered by position.
	GetEnabledStages(ctx context.Context, mediaType MediaType) ([]*PipelineStage, error)

	// GetAllStages returns all stages for a media type, including disabled.
	GetAllStages(ctx context.Context, mediaType MediaType) ([]*PipelineStage, error)

	// GetFirstStage returns the first enabled stage for a media type.
	GetFirstStage(ctx context.Context, mediaType MediaType) (*PipelineStage, error)

	// GetNextStage returns the next enabled stage after the given position.
	GetNextStage(ctx context.Context, mediaType MediaType, currentPosition int) (*PipelineStage, error)

	// Update modifies a pipeline stage.
	Update(ctx context.Context, stage *PipelineStage) error

	// Delete removes a pipeline stage.
	Delete(ctx context.Context, stageID int64) error

	// Enable/Disable toggles a stage.
	Enable(ctx context.Context, stageID int64) error
	Disable(ctx context.Context, stageID int64) error
}

// ExternalIDRepository defines operations for external IDs.
type ExternalIDRepository interface {
	// Upsert creates or updates an external ID.
	Upsert(ctx context.Context, id *ExternalID) error

	// GetByMedia returns all external IDs for a media item.
	GetByMedia(ctx context.Context, mediaID int64) ([]*ExternalID, error)

	// GetMediaByExternalID finds a media item by provider and external ID.
	GetMediaByExternalID(ctx context.Context, provider, externalID string) (int64, error)

	// DeleteByMedia removes all external IDs for a media item.
	DeleteByMedia(ctx context.Context, mediaID int64) error
}

// MetadataSourceRepository defines operations for metadata source tracking.
type MetadataSourceRepository interface {
	// Upsert creates or updates a metadata source record.
	Upsert(ctx context.Context, source *MetadataSource) error

	// GetByMedia returns all metadata sources for a media item.
	GetByMedia(ctx context.Context, mediaID int64) ([]*MetadataSource, error)

	// DeleteByMedia removes all metadata sources for a media item.
	DeleteByMedia(ctx context.Context, mediaID int64) error

	// DeleteByPlugin removes all metadata sources for a plugin.
	DeleteByPlugin(ctx context.Context, pluginID string) error
}
