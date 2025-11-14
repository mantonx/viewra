package transcode

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/viewra/viewra/internal/domain/transcode"
	transcoding "github.com/viewra/viewra/internal/infrastructure/transcoding"
)

// QueueConfig configures the transcode job queue.
type QueueConfig struct {
	// WorkerCount is the number of concurrent transcode workers
	WorkerCount int

	// PollInterval is how often to check for new queued jobs
	PollInterval time.Duration

	// IdleTimeout is how long a transcode can run without segment requests before being cancelled
	// Set to 0 to disable idle timeout. Recommended: 5 minutes
	IdleTimeout time.Duration

	// OutputBaseDir is the base directory for DASH output files
	OutputBaseDir string

	// MediaFileGetter is a function to get the file path for a media ID
	MediaFileGetter func(ctx context.Context, mediaID int64) (string, error)
}

// DefaultQueueConfig returns default queue configuration.
func DefaultQueueConfig() *QueueConfig {
	return &QueueConfig{
		WorkerCount:   2,                                 // 2 concurrent transcodes by default
		PollInterval:  10 * time.Second,                  // Check for new jobs every 10 seconds
		IdleTimeout:   5 * time.Minute,                   // Cancel transcode after 5 minutes of no activity
		OutputBaseDir: transcoding.GetDefaultOutputDir(), // /data/dash or ./data/dash
	}
}

// Queue manages a worker pool for processing transcode jobs.
type Queue struct {
	config  *QueueConfig
	repo    transcode.Repository
	service transcoding.Service
	logger  *slog.Logger

	jobChan chan *transcode.TranscodeJob
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running bool
	mu      sync.Mutex

	// activeJobs tracks currently processing jobs with their cancel functions
	// Key: job ID, Value: cancel function to stop the transcode
	activeJobs   map[int64]context.CancelFunc
	activeJobsMu sync.RWMutex

	// lastAccessTime tracks the last time each job had a segment requested
	// Key: "mediaID:quality", Value: last access timestamp
	lastAccessTime   map[string]time.Time
	lastAccessTimeMu sync.RWMutex
}

// NewQueue creates a new transcode job queue.
func NewQueue(config *QueueConfig, repo transcode.Repository, service transcoding.Service, logger *slog.Logger) *Queue {
	if config == nil {
		config = DefaultQueueConfig()
	}

	if logger == nil {
		logger = slog.Default()
	}

	if config.MediaFileGetter == nil {
		// Default implementation that requires manual setting
		config.MediaFileGetter = func(_ context.Context, mediaID int64) (string, error) {
			return "", fmt.Errorf("MediaFileGetter not configured for media ID %d", mediaID)
		}
	}

	return &Queue{
		config:         config,
		repo:           repo,
		service:        service,
		logger:         logger,
		jobChan:        make(chan *transcode.TranscodeJob, config.WorkerCount*2), // Buffer for smoother operation
		activeJobs:     make(map[int64]context.CancelFunc),
		lastAccessTime: make(map[string]time.Time),
	}
}

// Start starts the queue and worker pool.
func (q *Queue) Start(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.running {
		return fmt.Errorf("queue is already running")
	}

	q.ctx, q.cancel = context.WithCancel(ctx)
	q.running = true

	// Start worker goroutines
	for i := 0; i < q.config.WorkerCount; i++ {
		q.wg.Add(1)
		go q.worker(i)
	}

	// Start job poller
	q.wg.Add(1)
	go q.poller()

	// Start idle monitor if idle timeout is configured
	if q.config.IdleTimeout > 0 {
		q.wg.Add(1)
		go q.idleMonitor()
	}

	q.logger.Info("transcode queue started",
		slog.Int("workers", q.config.WorkerCount),
		slog.Duration("poll_interval", q.config.PollInterval),
		slog.Duration("idle_timeout", q.config.IdleTimeout),
	)

	return nil
}

// Stop gracefully stops the queue and all workers.
// It waits for current jobs to complete before returning.
func (q *Queue) Stop() error {
	q.mu.Lock()
	if !q.running {
		q.mu.Unlock()
		return fmt.Errorf("queue is not running")
	}
	q.running = false
	q.mu.Unlock()

	q.logger.Info("stopping transcode queue...")

	// Cancel context to stop poller and workers
	q.cancel()

	// Close job channel to signal workers to exit after draining
	close(q.jobChan)

	// Wait for all workers to finish
	q.wg.Wait()

	q.logger.Info("transcode queue stopped")
	return nil
}

// EnqueueJob adds a job to the queue immediately (bypasses polling).
// Useful for triggering jobs on-demand.
func (q *Queue) EnqueueJob(job *transcode.TranscodeJob) error {
	q.mu.Lock()
	if !q.running {
		q.mu.Unlock()
		return fmt.Errorf("queue is not running")
	}
	q.mu.Unlock()

	select {
	case q.jobChan <- job:
		q.logger.Debug("job enqueued",
			slog.Int64("job_id", job.ID),
			slog.Int64("media_id", job.MediaID),
			slog.String("quality", job.Quality),
		)
		return nil
	case <-q.ctx.Done():
		return fmt.Errorf("queue is shutting down")
	default:
		return fmt.Errorf("job queue is full")
	}
}

// poller periodically checks the database for queued jobs.
func (q *Queue) poller() {
	defer q.wg.Done()

	ticker := time.NewTicker(q.config.PollInterval)
	defer ticker.Stop()

	// Do an initial poll immediately
	q.pollOnce()

	for {
		select {
		case <-q.ctx.Done():
			q.logger.Debug("poller stopping")
			return
		case <-ticker.C:
			q.pollOnce()
		}
	}
}

// pollOnce performs a single poll for queued jobs.
func (q *Queue) pollOnce() {
	ctx, cancel := context.WithTimeout(q.ctx, 30*time.Second)
	defer cancel()

	// Fetch queued jobs (limit to worker count to avoid overwhelming the queue)
	jobs, err := q.repo.ListQueuedJobs(ctx, q.config.WorkerCount)
	if err != nil {
		q.logger.Error("failed to fetch queued jobs", slog.String("error", err.Error()))
		return
	}

	if len(jobs) == 0 {
		return
	}

	q.logger.Debug("found queued jobs", slog.Int("count", len(jobs)))

	// Add jobs to channel
	for _, job := range jobs {
		select {
		case q.jobChan <- job:
			q.logger.Debug("job added to queue",
				slog.Int64("job_id", job.ID),
				slog.Int64("media_id", job.MediaID),
				slog.String("quality", job.Quality),
			)
		case <-q.ctx.Done():
			return
		default:
			// Queue is full, skip this job for now (will be picked up in next poll)
			q.logger.Warn("job queue full, skipping job",
				slog.Int64("job_id", job.ID),
			)
		}
	}
}

// worker processes jobs from the queue.
func (q *Queue) worker(id int) {
	defer q.wg.Done()

	q.logger.Debug("worker started", slog.Int("worker_id", id))

	for {
		select {
		case <-q.ctx.Done():
			q.logger.Debug("worker stopping", slog.Int("worker_id", id))
			return

		case job, ok := <-q.jobChan:
			if !ok {
				// Channel closed, exit
				q.logger.Debug("worker exiting, channel closed", slog.Int("worker_id", id))
				return
			}

			q.processJob(id, job)
		}
	}
}

// processJob processes a single transcode job.
func (q *Queue) processJob(workerID int, job *transcode.TranscodeJob) {
	q.logger.Info("worker processing job",
		slog.Int("worker_id", workerID),
		slog.Int64("job_id", job.ID),
		slog.Int64("media_id", job.MediaID),
		slog.String("quality", job.Quality),
	)

	ctx, cancel := context.WithTimeout(q.ctx, 2*time.Hour) // Generous timeout for large files
	defer cancel()

	// Register this job as active so it can be cancelled on demand
	q.activeJobsMu.Lock()
	q.activeJobs[job.ID] = cancel
	q.activeJobsMu.Unlock()

	// Ensure cleanup when done
	defer func() {
		q.activeJobsMu.Lock()
		delete(q.activeJobs, job.ID)
		q.activeJobsMu.Unlock()
	}()

	// Get input file path
	inputPath, err := q.config.MediaFileGetter(ctx, job.MediaID)
	if err != nil {
		q.logger.Error("failed to get media file path",
			slog.Int64("job_id", job.ID),
			slog.Int64("media_id", job.MediaID),
			slog.String("error", err.Error()),
		)
		// Mark job as failed
		job.MarkAsFailed(fmt.Sprintf("Failed to get media file: %v", err))
		if updateErr := q.repo.Update(ctx, job); updateErr != nil {
			q.logger.Error("failed to update job status",
				slog.Int64("job_id", job.ID),
				slog.String("error", updateErr.Error()),
			)
		}
		return
	}

	// Execute the appropriate operation based on job type
	startTime := time.Now()
	var operationName string

	switch job.Type {
	case transcode.TypeRemux:
		operationName = "remux"
		err = q.service.RemuxToDASH(ctx, job, inputPath, q.config.OutputBaseDir)
	case transcode.TypeRemuxAudio:
		operationName = "remux with audio downmix"
		err = q.service.RemuxWithAudioDownmix(ctx, job, inputPath, q.config.OutputBaseDir)
	case transcode.TypeTranscode:
		operationName = "transcode"
		err = q.service.TranscodeToDASH(ctx, job, inputPath, q.config.OutputBaseDir)
	default:
		// Default to transcode for backward compatibility or unknown types
		operationName = "transcode (default)"
		err = q.service.TranscodeToDASH(ctx, job, inputPath, q.config.OutputBaseDir)
	}

	duration := time.Since(startTime)

	if err != nil {
		q.logger.Error(fmt.Sprintf("%s failed", operationName),
			slog.Int("worker_id", workerID),
			slog.Int64("job_id", job.ID),
			slog.String("job_type", job.Type),
			slog.Duration("duration", duration),
			slog.String("error", err.Error()),
		)
	} else {
		q.logger.Info(fmt.Sprintf("%s completed successfully", operationName),
			slog.Int("worker_id", workerID),
			slog.Int64("job_id", job.ID),
			slog.Int64("media_id", job.MediaID),
			slog.String("quality", job.Quality),
			slog.String("job_type", job.Type),
			slog.Duration("duration", duration),
		)
	}
}

// GetStats returns current queue statistics.
func (q *Queue) GetStats(ctx context.Context) (*QueueStats, error) {
	queuedCount, err := q.repo.CountByStatus(ctx, transcode.StatusQueued)
	if err != nil {
		return nil, fmt.Errorf("failed to count queued jobs: %w", err)
	}

	processingCount, err := q.repo.CountByStatus(ctx, transcode.StatusProcessing)
	if err != nil {
		return nil, fmt.Errorf("failed to count processing jobs: %w", err)
	}

	completedCount, err := q.repo.CountByStatus(ctx, transcode.StatusCompleted)
	if err != nil {
		return nil, fmt.Errorf("failed to count completed jobs: %w", err)
	}

	failedCount, err := q.repo.CountByStatus(ctx, transcode.StatusFailed)
	if err != nil {
		return nil, fmt.Errorf("failed to count failed jobs: %w", err)
	}

	return &QueueStats{
		QueuedJobs:     queuedCount,
		ProcessingJobs: processingCount,
		CompletedJobs:  completedCount,
		FailedJobs:     failedCount,
		WorkerCount:    q.config.WorkerCount,
	}, nil
}

// CancelJob cancels an actively processing transcode job.
// This immediately stops the FFmpeg process and marks the job as cancelled.
// Returns error if the job is not currently processing.
func (q *Queue) CancelJob(ctx context.Context, mediaID int64, quality string) error {
	// Find the job
	job, err := q.repo.GetByMediaIDAndQuality(ctx, mediaID, quality)
	if err != nil {
		return fmt.Errorf("failed to find job: %w", err)
	}

	if job == nil {
		return fmt.Errorf("no transcode job found for media %d at quality %s", mediaID, quality)
	}

	// Check if job is actually processing
	if !job.IsProcessing() {
		return fmt.Errorf("job %d is not currently processing (status: %s)", job.ID, job.Status)
	}

	// Get the cancel function for this job
	q.activeJobsMu.RLock()
	cancelFunc, exists := q.activeJobs[job.ID]
	q.activeJobsMu.RUnlock()

	if !exists {
		return fmt.Errorf("job %d is not actively transcoding", job.ID)
	}

	// Cancel the job's context (this kills FFmpeg)
	q.logger.Info("cancelling transcode job",
		slog.Int64("job_id", job.ID),
		slog.Int64("media_id", mediaID),
		slog.String("quality", quality),
	)
	cancelFunc()

	// Mark job as cancelled in the database
	job.MarkAsFailed("Cancelled by user")
	if err := q.repo.Update(ctx, job); err != nil {
		q.logger.Error("failed to mark job as cancelled",
			slog.Int64("job_id", job.ID),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("failed to update job status: %w", err)
	}

	return nil
}

// RecordAccess updates the last access time for a transcode job.
// Call this whenever a segment is requested to prevent idle timeout.
func (q *Queue) RecordAccess(mediaID int64, quality string) {
	key := fmt.Sprintf("%d:%s", mediaID, quality)

	q.lastAccessTimeMu.Lock()
	q.lastAccessTime[key] = time.Now()
	q.lastAccessTimeMu.Unlock()
}

// idleMonitor periodically checks for idle transcode jobs and cancels them.
func (q *Queue) idleMonitor() {
	defer q.wg.Done()

	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-q.ctx.Done():
			q.logger.Debug("idle monitor stopping")
			return
		case <-ticker.C:
			q.checkIdleJobs()
		}
	}
}

// checkIdleJobs finds and cancels jobs that haven't had segment requests in IdleTimeout duration.
func (q *Queue) checkIdleJobs() {
	ctx, cancel := context.WithTimeout(q.ctx, 30*time.Second)
	defer cancel()

	// Get all currently processing jobs
	jobs, err := q.repo.ListProcessingJobs(ctx)
	if err != nil {
		q.logger.Error("failed to fetch processing jobs for idle check",
			slog.String("error", err.Error()),
		)
		return
	}

	now := time.Now()
	idleThreshold := now.Add(-q.config.IdleTimeout)

	for _, job := range jobs {
		key := fmt.Sprintf("%d:%s", job.MediaID, job.Quality)

		q.lastAccessTimeMu.RLock()
		lastAccess, exists := q.lastAccessTime[key]
		q.lastAccessTimeMu.RUnlock()

		// If no access record exists, use job start time
		if !exists {
			lastAccess = job.StartedAt
			// Record initial access time
			q.lastAccessTimeMu.Lock()
			q.lastAccessTime[key] = lastAccess
			q.lastAccessTimeMu.Unlock()
		}

		// Check if job has been idle too long
		if lastAccess.Before(idleThreshold) {
			q.logger.Info("cancelling idle transcode job",
				slog.Int64("job_id", job.ID),
				slog.Int64("media_id", job.MediaID),
				slog.String("quality", job.Quality),
				slog.Duration("idle_duration", now.Sub(lastAccess)),
			)

			// Cancel the job
			if err := q.CancelJob(ctx, job.MediaID, job.Quality); err != nil {
				q.logger.Error("failed to cancel idle job",
					slog.Int64("job_id", job.ID),
					slog.String("error", err.Error()),
				)
			}

			// Clean up access tracking
			q.lastAccessTimeMu.Lock()
			delete(q.lastAccessTime, key)
			q.lastAccessTimeMu.Unlock()
		}
	}
}

// QueueStats contains queue statistics.
type QueueStats struct {
	QueuedJobs     int64
	ProcessingJobs int64
	CompletedJobs  int64
	FailedJobs     int64
	WorkerCount    int
}
