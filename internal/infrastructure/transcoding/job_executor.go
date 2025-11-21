package transcoding

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/viewra/viewra/internal/domain/transcode"
	"github.com/viewra/viewra/internal/infrastructure/filesystem"
)

// jobExecutor handles the execution of transcode jobs with validation, progress tracking, and cleanup.
type jobExecutor struct {
	repo     transcode.Repository
	executor *ffmpegExecutor
	logger   *slog.Logger
}

// newJobExecutor creates a new job executor.
func newJobExecutor(repo transcode.Repository, executor *ffmpegExecutor, logger *slog.Logger) *jobExecutor {
	return &jobExecutor{
		repo:     repo,
		executor: executor,
		logger:   logger,
	}
}

// execute runs a transcoding/remuxing job with complete lifecycle management.
// It handles validation, progress tracking, error handling, and cleanup for all job types.
func (e *jobExecutor) execute(
	ctx context.Context,
	job *transcode.TranscodeJob,
	inputPath, outputBaseDir, operationName string,
	executorFunc func(context.Context, TranscodeOptions) error,
) error {
	// Validate job state
	if job == nil {
		return fmt.Errorf("transcode job is nil")
	}

	if !job.IsQueued() && !job.IsProcessing() {
		return fmt.Errorf("job must be in queued or processing status, got: %s", job.Status)
	}

	// Get quality profile
	profile, err := GetQualityProfile(job.Quality)
	if err != nil {
		return e.failJob(ctx, job, fmt.Errorf("invalid quality profile: %w", err))
	}

	// Build output directory path: <outputBaseDir>/hls/<mediaID>/<quality>/
	outputDir := GetHLSOutputPath(outputBaseDir, job.MediaID, job.Quality)

	// Create output directory before validation so disk space check works
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return e.failJob(ctx, job, fmt.Errorf("failed to create output directory: %w", err))
	}

	// Comprehensive validation: path security, file checks, disk space
	// Note: We skip codec analysis for remux operations as they're chosen based on strategy
	if err := ValidateTranscodeRequest(inputPath, outputDir, profile, nil); err != nil {
		// For remux operations, we may skip some validation errors
		// since the strategy selection already determined compatibility
		if operationName == "transcode" {
			return e.failJob(ctx, job, fmt.Errorf("validation failed: %w", err))
		}
	}

	// Get video info for size estimation and audio track selection
	videoInfo, err := e.getVideoInfo(ctx, job, inputPath)
	if err != nil {
		// Log warning but continue
		e.logger.Warn("failed to get video info",
			slog.Int64("job_id", job.ID),
			slog.String("error", err.Error()),
		)
	}

	// Estimate output size and verify disk space
	if err := e.checkDiskSpace(ctx, job, outputDir, profile, videoInfo); err != nil {
		return e.failJob(ctx, job, err)
	}

	// Log operation start
	e.logger.Info(fmt.Sprintf("starting %s", operationName),
		slog.Int64("job_id", job.ID),
		slog.Int64("media_id", job.MediaID),
		slog.String("quality", job.Quality),
		slog.String("input", inputPath),
		slog.String("output", outputDir),
	)

	// Mark job as processing
	job.MarkAsProcessing()
	if err := e.repo.Update(ctx, job); err != nil {
		e.logger.Error("failed to mark job as processing",
			slog.Int64("job_id", job.ID),
			slog.String("error", err.Error()),
		)
		// Continue anyway - the operation can still proceed
	}

	// Prepare transcoding options
	opts := e.prepareOptions(job, inputPath, outputDir, profile, videoInfo)

	// Execute the operation
	if err := executorFunc(ctx, opts); err != nil {
		// Clean up partial output on failure
		if cleanupErr := CleanupOutputDir(outputDir); cleanupErr != nil {
			e.logger.Warn("failed to cleanup output directory after failure",
				slog.String("output_dir", outputDir),
				slog.String("error", cleanupErr.Error()),
			)
		}
		return e.failJob(ctx, job, fmt.Errorf("%s failed: %w", operationName, err))
	}

	// Complete the job
	return e.completeJob(ctx, job, outputDir, operationName)
}

// getVideoInfo fetches video metadata for the input file.
func (e *jobExecutor) getVideoInfo(ctx context.Context, job *transcode.TranscodeJob, inputPath string) (*VideoInfo, error) {
	videoInfo, err := GetVideoInfo(inputPath)
	if err != nil {
		return nil, err
	}
	return videoInfo, nil
}

// checkDiskSpace estimates output size and verifies sufficient disk space is available.
func (e *jobExecutor) checkDiskSpace(
	ctx context.Context,
	job *transcode.TranscodeJob,
	outputDir string,
	profile *QualityProfile,
	videoInfo *VideoInfo,
) error {
	if videoInfo == nil || videoInfo.Duration <= 0 {
		return nil // Can't estimate without video info
	}

	estimatedSize, err := EstimateOutputSize(profile.VideoBitrate, profile.AudioBitrate, videoInfo.Duration)
	if err != nil {
		e.logger.Warn("failed to estimate output size",
			slog.Int64("job_id", job.ID),
			slog.String("error", err.Error()),
		)
		return nil
	}

	// Add 30% safety margin for HLS overhead (m3u8, multiple segments, etc.)
	requiredBytes := uint64(float64(estimatedSize) * 1.3)
	requiredGB := int64(requiredBytes / (1024 * 1024 * 1024))

	// Check if we have enough space for the estimated output
	if err := CheckDiskSpace(outputDir, requiredGB); err != nil {
		return fmt.Errorf(
			"insufficient disk space for estimated output (%.2f GB required): %w",
			float64(requiredBytes)/(1024*1024*1024), err)
	}

	e.logger.Info("estimated output size",
		slog.Int64("job_id", job.ID),
		slog.String("estimated_size", FormatBytes(estimatedSize)),
		slog.String("with_margin", FormatBytes(requiredBytes)),
		slog.Float64("duration_seconds", videoInfo.Duration),
	)

	return nil
}

// prepareOptions creates TranscodeOptions for the FFmpeg executor.
func (e *jobExecutor) prepareOptions(
	job *transcode.TranscodeJob,
	inputPath, outputDir string,
	profile *QualityProfile,
	videoInfo *VideoInfo,
) TranscodeOptions {
	opts := TranscodeOptions{
		InputPath:       inputPath,
		OutputDir:       outputDir,
		Profile:         profile,
		ProgressHandler: e.createProgressHandler(job),
	}

	// Use specific audio track if available
	if videoInfo != nil && videoInfo.SelectedAudioTrackIndex > 0 {
		opts.UseSpecificAudioTrack = true
		opts.AudioTrackIndex = videoInfo.SelectedAudioTrackIndex

		e.logger.Info("using selected audio track",
			slog.Int64("job_id", job.ID),
			slog.Int("track_index", videoInfo.SelectedAudioTrackIndex),
			slog.String("codec", videoInfo.AudioCodec),
			slog.Int("channels", videoInfo.AudioChannels),
		)
	}

	// Set start position if provided (for seek-based transcoding)
	if job.StartPosition > 0 {
		opts.UseStartPosition = true
		opts.StartPosition = job.StartPosition

		e.logger.Info("starting transcode from seek position",
			slog.Int64("job_id", job.ID),
			slog.Int("start_position", job.StartPosition),
		)
	}

	return opts
}

// createProgressHandler creates a progress callback that updates the job in the database.
func (e *jobExecutor) createProgressHandler(job *transcode.TranscodeJob) func(int) {
	// Track last reported progress to avoid unnecessary database updates
	lastReported := 0

	return func(progress int) {
		// Only update if progress changed by at least 5% to reduce database load
		if progress-lastReported < 5 && progress != 100 {
			return
		}

		// Update job progress
		if err := job.UpdateProgress(progress); err != nil {
			e.logger.Warn("invalid progress value",
				slog.Int64("job_id", job.ID),
				slog.Int("progress", progress),
				slog.String("error", err.Error()),
			)
			return
		}

		// Save to database
		if err := e.repo.Update(context.Background(), job); err != nil {
			e.logger.Warn("failed to update job progress",
				slog.Int64("job_id", job.ID),
				slog.Int("progress", progress),
				slog.String("error", err.Error()),
			)
			return
		}

		lastReported = progress

		e.logger.Debug("transcode progress",
			slog.Int64("job_id", job.ID),
			slog.Int("progress", progress),
		)
	}
}

// completeJob marks a job as completed with metadata and updates the repository.
func (e *jobExecutor) completeJob(
	ctx context.Context,
	job *transcode.TranscodeJob,
	outputDir, operationName string,
) error {
	// Calculate file size
	fileSize, err := filesystem.CalculateDirSize(outputDir)
	if err != nil {
		e.logger.Warn("failed to calculate output size",
			slog.Int64("job_id", job.ID),
			slog.String("error", err.Error()),
		)
	}

	// Mark job as completed with file metadata
	job.MarkAsCompleted()
	job.FilePath = outputDir
	job.FileSizeBytes = fileSize
	if err := e.repo.Update(ctx, job); err != nil {
		e.logger.Error("failed to mark job as completed",
			slog.Int64("job_id", job.ID),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("%s succeeded but failed to update job status: %w", operationName, err)
	}

	e.logger.Info(fmt.Sprintf("%s completed successfully", operationName),
		slog.Int64("job_id", job.ID),
		slog.Int64("media_id", job.MediaID),
		slog.String("quality", job.Quality),
		slog.String("output", outputDir),
	)

	return nil
}

// failJob marks a job as failed and updates it in the repository.
// It returns the original error wrapped with context.
func (e *jobExecutor) failJob(ctx context.Context, job *transcode.TranscodeJob, err error) error {
	errMsg := err.Error()

	// Truncate error message if too long (database constraints)
	if len(errMsg) > 1000 {
		errMsg = errMsg[:997] + "..."
	}

	job.MarkAsFailed(errMsg)

	if updateErr := e.repo.Update(ctx, job); updateErr != nil {
		e.logger.Error("failed to mark job as failed",
			slog.Int64("job_id", job.ID),
			slog.String("error", updateErr.Error()),
		)
		// Return combined error
		return fmt.Errorf("transcode failed: %w (also failed to update job: %v)", err, updateErr)
	}

	e.logger.Error("transcode failed",
		slog.Int64("job_id", job.ID),
		slog.Int64("media_id", job.MediaID),
		slog.String("quality", job.Quality),
		slog.String("error", errMsg),
	)

	return err
}
