package transcode

import (
	"context"
	"fmt"

	"github.com/viewra/viewra/internal/domain/transcode"
)

// CreateJobRequest represents a request to create a new transcode job.
type CreateJobRequest struct {
	MediaID int64
	Quality string
	Type    string // Optional: remux, remux_audio, or transcode (defaults to transcode)
}

// CreateJob creates a new transcode job for the specified media and quality.
// Returns the created job or an error if the job already exists or validation fails.
func CreateJob(ctx context.Context, repo transcode.Repository, req CreateJobRequest) (*transcode.TranscodeJob, error) {
	// Validate quality
	if !isValidQuality(req.Quality) {
		return nil, transcode.ErrInvalidQuality
	}

	// Check if job already exists for this media/quality combination
	existingJob, err := repo.GetByMediaIDAndQuality(ctx, req.MediaID, req.Quality)
	if err == nil && existingJob != nil {
		// Job exists - check its status
		if existingJob.IsQueued() ||
			existingJob.IsProcessing() ||
			existingJob.IsCompleted() {
			return nil, transcode.ErrJobAlreadyExists
		}
		// If failed, allow recreation by deleting old job
		if err := repo.Delete(ctx, existingJob.ID); err != nil {
			return nil, fmt.Errorf("failed to delete previous failed job: %w", err)
		}
	}

	// Determine job type - default to transcode if not specified
	jobType := req.Type
	if jobType == "" {
		jobType = transcode.TypeTranscode
	}

	// Create new job
	job, err := transcode.NewTranscodeJob(req.MediaID, req.Quality, jobType)
	if err != nil {
		return nil, fmt.Errorf("failed to create transcode job: %w", err)
	}

	// Save to repository
	if err := repo.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("failed to save transcode job: %w", err)
	}

	return job, nil
}

// isValidQuality checks if the quality string is valid.
func isValidQuality(quality string) bool {
	switch quality {
	case transcode.Quality360p, transcode.Quality720p, transcode.Quality1080p, transcode.Quality4K:
		return true
	default:
		return false
	}
}
