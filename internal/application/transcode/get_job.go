package transcode

import (
	"context"

	"github.com/viewra/viewra/internal/domain/transcode"
)

// GetJob retrieves a transcode job by its ID.
func GetJob(ctx context.Context, repo transcode.Repository, jobID int64) (*transcode.TranscodeJob, error) {
	return repo.GetByID(ctx, jobID)
}

// GetJobForMedia retrieves a transcode job for a specific media and quality.
func GetJobForMedia(ctx context.Context, repo transcode.Repository, mediaID int64, quality string) (*transcode.TranscodeJob, error) {
	return repo.GetByMediaIDAndQuality(ctx, mediaID, quality)
}

// ListJobsByStatus retrieves all transcode jobs with a specific status.
func ListJobsByStatus(ctx context.Context, repo transcode.Repository, status string) ([]*transcode.TranscodeJob, error) {
	return repo.ListByStatus(ctx, status)
}

// ListProcessingJobs retrieves all currently processing jobs.
func ListProcessingJobs(ctx context.Context, repo transcode.Repository) ([]*transcode.TranscodeJob, error) {
	return repo.ListProcessingJobs(ctx)
}
