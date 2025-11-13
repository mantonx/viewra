package transcode

import "context"

// Repository defines the interface for transcode job persistence.
type Repository interface {
	// Create creates a new transcode job.
	Create(ctx context.Context, job *TranscodeJob) error

	// Update updates an existing transcode job.
	Update(ctx context.Context, job *TranscodeJob) error

	// GetByID retrieves a transcode job by its ID.
	GetByID(ctx context.Context, id int64) (*TranscodeJob, error)

	// GetByMediaIDAndQuality retrieves a transcode job by media ID and quality.
	GetByMediaIDAndQuality(ctx context.Context, mediaID int64, quality string) (*TranscodeJob, error)

	// ListByStatus retrieves all transcode jobs with a specific status.
	ListByStatus(ctx context.Context, status string) ([]*TranscodeJob, error)

	// ListQueuedJobs retrieves queued jobs up to a limit, ordered by creation time.
	ListQueuedJobs(ctx context.Context, limit int) ([]*TranscodeJob, error)

	// ListProcessingJobs retrieves all currently processing jobs.
	ListProcessingJobs(ctx context.Context) ([]*TranscodeJob, error)

	// CountByStatus counts jobs with a specific status.
	CountByStatus(ctx context.Context, status string) (int64, error)

	// Delete deletes a transcode job by ID.
	Delete(ctx context.Context, id int64) error

	// DeleteByMediaID deletes all transcode jobs for a media item.
	DeleteByMediaID(ctx context.Context, mediaID int64) error
}
