package transcode

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/viewra/viewra/internal/domain/library"
	"github.com/viewra/viewra/internal/domain/media"
	"github.com/viewra/viewra/internal/domain/transcode"
	"github.com/viewra/viewra/internal/infrastructure/transcoding"
)

// ServeManifestRequest represents a request to serve an HLS playlist.
type ServeManifestRequest struct {
	MediaID   int64
	Quality   string
	OutputDir string
}

// ServeManifestResponse represents the result of a serve manifest request.
type ServeManifestResponse struct {
	// Strategy indicates what action to take
	Strategy ManifestStrategy

	// ManifestPath is the path to the manifest file (for Serve strategy)
	ManifestPath string

	// DirectPlayURL is the URL for direct playback (for DirectPlay strategy)
	DirectPlayURL string

	// Job contains the transcode job information (for Transcode strategy)
	Job *transcode.TranscodeJob

	// EstimatedTime is the estimated completion time for transcode jobs
	EstimatedTime string

	// Reason explains why this strategy was chosen
	Reason string
}

// ManifestStrategy indicates how to handle the manifest request.
type ManifestStrategy int

const (
	// StrategyServe means manifest exists and should be served directly
	StrategyServe ManifestStrategy = iota

	// StrategyDirectPlay means video is compatible and can be played directly
	StrategyDirectPlay

	// StrategyTranscode means a transcode job is needed (created or in progress)
	StrategyTranscode
)

// ServeManifestUseCase handles serving HLS playlists with on-demand transcoding.
type ServeManifestUseCase struct {
	transcodeRepo transcode.Repository
	mediaRepo     media.Repository
	libraryRepo   library.Repository
	createJobUC   *CreateJobUseCase
}

// NewServeManifestUseCase creates a new ServeManifestUseCase.
func NewServeManifestUseCase(
	transcodeRepo transcode.Repository,
	mediaRepo media.Repository,
	libraryRepo library.Repository,
	createJobUC *CreateJobUseCase,
) *ServeManifestUseCase {
	return &ServeManifestUseCase{
		transcodeRepo: transcodeRepo,
		mediaRepo:     mediaRepo,
		libraryRepo:   libraryRepo,
		createJobUC:   createJobUC,
	}
}

// Execute handles the playlist serving logic with on-demand transcoding.
func (uc *ServeManifestUseCase) Execute(ctx context.Context, req ServeManifestRequest) (*ServeManifestResponse, error) {
	// Step 1: Check if playlist exists (even for in-progress jobs)
	manifestPath := transcoding.GetHLSManifestPath(req.OutputDir, req.MediaID, req.Quality)
	if _, err := os.Stat(manifestPath); err == nil {
		// Playlist exists - check if we have enough segments for playback
		// With streaming HLS, segments are generated progressively
		segmentDir := filepath.Dir(manifestPath)
		segmentPattern := filepath.Join(segmentDir, "segment_*.ts")
		segments, err := filepath.Glob(segmentPattern)
		if err != nil {
			// If glob fails, assume no segments yet
			segments = []string{}
		}

		// If we have at least 2 segments, we can start playback
		// The playlist will update as more segments are generated
		if len(segments) >= 2 {
			return &ServeManifestResponse{
				Strategy:     StrategyServe,
				ManifestPath: manifestPath,
			}, nil
		}

		// Playlist exists but not enough segments yet (initializing)
		// Check if there's a job in progress
		existingJob, err := GetJobForMedia(ctx, uc.transcodeRepo, req.MediaID, req.Quality)
		if err == nil && existingJob != nil && existingJob.IsProcessing() {
			return &ServeManifestResponse{
				Strategy:      StrategyTranscode,
				Job:           existingJob,
				EstimatedTime: "Initializing (~10 seconds)",
				Reason:        "First segments generating, playback starting soon",
			}, nil
		}
	}

	// Step 2: Get media entity
	mediaEntity, err := uc.mediaRepo.GetByID(ctx, req.MediaID)
	if err != nil {
		return nil, fmt.Errorf("media not found: %w", err)
	}

	// Step 3: Get full file path (file_path in database is absolute)
	fullPath := mediaEntity.FilePath

	// Step 4: Analyze video
	videoInfo, err := transcoding.GetVideoInfo(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze video: %w", err)
	}

	// Step 5: Determine streaming strategy
	strategy, reason := transcoding.DetermineStreamStrategy(videoInfo)

	// Step 6: Handle based on strategy
	switch strategy {
	case transcoding.DirectPlay:
		return &ServeManifestResponse{
			Strategy:      StrategyDirectPlay,
			DirectPlayURL: fmt.Sprintf("/api/stream/%d", req.MediaID),
			Reason:        reason,
		}, nil

	case transcoding.Remux:
		return uc.createOrGetJob(ctx, req.MediaID, req.Quality, transcode.TypeRemux, reason, "2-5 minutes")

	case transcoding.RemuxWithAudioDownmix:
		return uc.createOrGetJob(ctx, req.MediaID, req.Quality, transcode.TypeRemuxAudio, reason, "5-10 minutes")

	case transcoding.Transcode:
		return uc.createOrGetJob(ctx, req.MediaID, req.Quality, transcode.TypeTranscode, reason, "20-60 minutes")

	default:
		return nil, fmt.Errorf("unknown streaming strategy: %v", strategy)
	}
}

// createOrGetJob creates a new transcode job or returns an existing one.
func (uc *ServeManifestUseCase) createOrGetJob(
	ctx context.Context,
	mediaID int64,
	quality string,
	jobType string,
	reason string,
	estimatedTime string,
) (*ServeManifestResponse, error) {
	// Check if job already exists
	existingJob, err := GetJobForMedia(ctx, uc.transcodeRepo, mediaID, quality)
	if err == nil && existingJob != nil {
		// Handle inconsistent state: completed job but no playlist
		if existingJob.IsCompleted() {
			uc.requeueCompletedJob(ctx, existingJob)
		}

		return &ServeManifestResponse{
			Strategy:      StrategyTranscode,
			Job:           existingJob,
			EstimatedTime: estimatedTime,
			Reason:        reason,
		}, nil
	}

	// Use CreateJobUseCase to create and enqueue the job (DRY principle)
	job, err := uc.createJobUC.Execute(ctx, CreateJobRequest{
		MediaID: mediaID,
		Quality: quality,
		Type:    jobType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	return &ServeManifestResponse{
		Strategy:      StrategyTranscode,
		Job:           job,
		EstimatedTime: estimatedTime,
		Reason:        reason,
	}, nil
}

// requeueCompletedJob requeues a completed job that has inconsistent state.
func (uc *ServeManifestUseCase) requeueCompletedJob(ctx context.Context, job *transcode.TranscodeJob) {
	job.Status = transcode.StatusQueued
	job.Progress = 0

	if err := uc.transcodeRepo.Update(ctx, job); err != nil {
		return // Best effort
	}

	if uc.createJobUC.queue != nil {
		_ = uc.createJobUC.queue.EnqueueJob(job) //nolint:errcheck // Best effort
	}
}
