package transcoding

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/viewra/viewra/internal/domain/transcode"
)

// Service provides video transcoding functionality for DASH streaming.
type Service interface {
	// TranscodeToDASH transcodes a video file to DASH format at the specified quality level.
	// It updates the transcode job in the repository throughout the process.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control
	//   - job: The transcode job to execute (must be in queued status)
	//   - inputPath: Absolute path to the input video file
	//   - outputBaseDir: Base directory where DASH output will be stored
	//
	// The output structure will be:
	//   outputBaseDir/
	//     <mediaID>/
	//       <quality>/
	//         manifest.mpd
	//         init_*.m4s
	//         segment_*.m4s
	//
	// Returns an error if transcoding fails at any stage.
	TranscodeToDASH(ctx context.Context, job *transcode.TranscodeJob, inputPath, outputBaseDir string) error
}

// service implements the Service interface.
type service struct {
	repo     transcode.Repository
	executor *ffmpegExecutor
	logger   *slog.Logger
}

// NewService creates a new transcoding service.
// It verifies that FFmpeg is available in the system PATH.
func NewService(repo transcode.Repository, logger *slog.Logger) (Service, error) {
	executor, err := newFFmpegExecutor()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ffmpeg executor: %w", err)
	}

	if logger == nil {
		logger = slog.Default()
	}

	return &service{
		repo:     repo,
		executor: executor,
		logger:   logger,
	}, nil
}

// TranscodeToDASH implements the Service interface.
func (s *service) TranscodeToDASH(ctx context.Context, job *transcode.TranscodeJob, inputPath, outputBaseDir string) error {
	// Validate job state
	if job == nil {
		return fmt.Errorf("transcode job is nil")
	}

	if !job.IsQueued() && !job.IsProcessing() {
		return fmt.Errorf("job must be in queued or processing status, got: %s", job.Status)
	}

	// Validate input file
	if err := ValidateInputFile(inputPath); err != nil {
		return s.failJob(ctx, job, fmt.Errorf("invalid input file: %w", err))
	}

	// Get quality profile
	profile, err := GetQualityProfile(job.Quality)
	if err != nil {
		return s.failJob(ctx, job, fmt.Errorf("invalid quality profile: %w", err))
	}

	// Build output directory path: <outputBaseDir>/<mediaID>/<quality>/
	outputDir := s.buildOutputPath(outputBaseDir, job.MediaID, job.Quality)

	// Log transcode start
	s.logger.Info("starting transcode",
		slog.Int64("job_id", job.ID),
		slog.Int64("media_id", job.MediaID),
		slog.String("quality", job.Quality),
		slog.String("input", inputPath),
		slog.String("output", outputDir),
	)

	// Mark job as processing
	job.MarkAsProcessing()
	if err := s.repo.Update(ctx, job); err != nil {
		s.logger.Error("failed to mark job as processing",
			slog.Int64("job_id", job.ID),
			slog.String("error", err.Error()),
		)
		// Continue anyway - the transcode can still proceed
	}

	// Create progress handler that updates the job in the database
	progressHandler := s.createProgressHandler(ctx, job)

	// Prepare transcode options
	opts := TranscodeOptions{
		InputPath:       inputPath,
		OutputDir:       outputDir,
		Profile:         profile,
		ProgressHandler: progressHandler,
	}

	// Execute transcode
	if err := s.executor.Transcode(ctx, opts); err != nil {
		// Clean up partial output on failure
		if cleanupErr := CleanupOutputDir(outputDir); cleanupErr != nil {
			s.logger.Warn("failed to cleanup output directory after transcode failure",
				slog.String("output_dir", outputDir),
				slog.String("error", cleanupErr.Error()),
			)
		}
		return s.failJob(ctx, job, fmt.Errorf("transcode failed: %w", err))
	}

	// Mark job as completed
	job.MarkAsCompleted()
	if err := s.repo.Update(ctx, job); err != nil {
		s.logger.Error("failed to mark job as completed",
			slog.Int64("job_id", job.ID),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("transcode succeeded but failed to update job status: %w", err)
	}

	s.logger.Info("transcode completed successfully",
		slog.Int64("job_id", job.ID),
		slog.Int64("media_id", job.MediaID),
		slog.String("quality", job.Quality),
		slog.String("output", outputDir),
	)

	return nil
}

// createProgressHandler creates a progress callback function that updates the job.
func (s *service) createProgressHandler(ctx context.Context, job *transcode.TranscodeJob) func(int) {
	// Track last reported progress to avoid unnecessary database updates
	lastReported := 0

	return func(progress int) {
		// Only update if progress changed by at least 5% to reduce database load
		if progress-lastReported < 5 && progress != 100 {
			return
		}

		// Update job progress
		if err := job.UpdateProgress(progress); err != nil {
			s.logger.Warn("invalid progress value",
				slog.Int64("job_id", job.ID),
				slog.Int("progress", progress),
				slog.String("error", err.Error()),
			)
			return
		}

		// Save to database
		if err := s.repo.Update(ctx, job); err != nil {
			s.logger.Warn("failed to update job progress",
				slog.Int64("job_id", job.ID),
				slog.Int("progress", progress),
				slog.String("error", err.Error()),
			)
			return
		}

		lastReported = progress

		s.logger.Debug("transcode progress",
			slog.Int64("job_id", job.ID),
			slog.Int("progress", progress),
		)
	}
}

// failJob marks a job as failed and updates it in the repository.
// It returns the original error wrapped with context.
func (s *service) failJob(ctx context.Context, job *transcode.TranscodeJob, err error) error {
	errMsg := err.Error()

	// Truncate error message if too long (database constraints)
	if len(errMsg) > 1000 {
		errMsg = errMsg[:997] + "..."
	}

	job.MarkAsFailed(errMsg)

	if updateErr := s.repo.Update(ctx, job); updateErr != nil {
		s.logger.Error("failed to mark job as failed",
			slog.Int64("job_id", job.ID),
			slog.String("error", updateErr.Error()),
		)
		// Return combined error
		return fmt.Errorf("transcode failed: %w (also failed to update job: %v)", err, updateErr)
	}

	s.logger.Error("transcode failed",
		slog.Int64("job_id", job.ID),
		slog.Int64("media_id", job.MediaID),
		slog.String("quality", job.Quality),
		slog.String("error", errMsg),
	)

	return err
}

// buildOutputPath constructs the output directory path for DASH files.
// Format: <baseDir>/<mediaID>/<quality>/
func (s *service) buildOutputPath(baseDir string, mediaID int64, quality string) string {
	return filepath.Join(
		baseDir,
		fmt.Sprintf("%d", mediaID),
		strings.ToLower(quality),
	)
}

// GetManifestPath returns the path to the DASH manifest file for a given media and quality.
// This is a utility function for consumers of the transcoding service.
func GetManifestPath(baseDir string, mediaID int64, quality string) string {
	return filepath.Join(
		baseDir,
		fmt.Sprintf("%d", mediaID),
		strings.ToLower(quality),
		"manifest.mpd",
	)
}

// GetOutputDirectory returns the output directory path for a given media and quality.
// This is a utility function for consumers of the transcoding service.
func GetOutputDirectory(baseDir string, mediaID int64, quality string) string {
	return filepath.Join(
		baseDir,
		fmt.Sprintf("%d", mediaID),
		strings.ToLower(quality),
	)
}
