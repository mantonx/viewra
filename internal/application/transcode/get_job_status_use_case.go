package transcode

import (
	"context"

	"github.com/mantonx/viewra/internal/domain/transcode"
)

// GetJobStatusRequest represents a request to get job status.
type GetJobStatusRequest struct {
	MediaID int64
	Quality string
}

// GetJobStatusUseCase handles getting transcode job status.
type GetJobStatusUseCase struct {
	repo transcode.Repository
}

// NewGetJobStatusUseCase creates a new GetJobStatusUseCase.
func NewGetJobStatusUseCase(repo transcode.Repository) *GetJobStatusUseCase {
	return &GetJobStatusUseCase{
		repo: repo,
	}
}

// Execute gets the status of a transcode job.
func (uc *GetJobStatusUseCase) Execute(ctx context.Context, req GetJobStatusRequest) (*transcode.TranscodeJob, error) {
	return GetJobForMedia(ctx, uc.repo, req.MediaID, req.Quality)
}
