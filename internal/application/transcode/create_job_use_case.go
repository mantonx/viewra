package transcode

import (
	"context"

	"github.com/viewra/viewra/internal/domain/transcode"
)

// CreateJobUseCase handles creating transcode jobs.
type CreateJobUseCase struct {
	repo  transcode.Repository
	queue *Queue
}

// NewCreateJobUseCase creates a new CreateJobUseCase.
func NewCreateJobUseCase(repo transcode.Repository, queue *Queue) *CreateJobUseCase {
	return &CreateJobUseCase{
		repo:  repo,
		queue: queue,
	}
}

// Execute creates a transcode job and enqueues it.
func (uc *CreateJobUseCase) Execute(ctx context.Context, req CreateJobRequest) (*transcode.TranscodeJob, error) {
	// Create the job using the existing CreateJob function
	job, err := CreateJob(ctx, uc.repo, req)
	if err != nil {
		return nil, err
	}

	// Enqueue the job for processing
	if uc.queue != nil {
		// Best effort - if enqueue fails, job will be picked up by the poller
		_ = uc.queue.EnqueueJob(job) //nolint:errcheck
	}

	return job, nil
}
