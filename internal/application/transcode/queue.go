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

	// OutputBaseDir is the base directory for DASH output files
	OutputBaseDir string

	// MediaFileGetter is a function to get the file path for a media ID
	MediaFileGetter func(ctx context.Context, mediaID int64) (string, error)
}

// DefaultQueueConfig returns default queue configuration.
func DefaultQueueConfig() *QueueConfig {
	return &QueueConfig{
		WorkerCount:   2,                // 2 concurrent transcodes by default
		PollInterval:  10 * time.Second, // Check for new jobs every 10 seconds
		OutputBaseDir: "./data/dash",    // Default output directory
	}
}

// Queue manages a worker pool for processing transcode jobs.
type Queue struct {
	config    *QueueConfig
	repo      transcode.Repository
	service   transcoding.Service
	logger    *slog.Logger

	jobChan   chan *transcode.TranscodeJob
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	running   bool
	mu        sync.Mutex
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
		config.MediaFileGetter = func(ctx context.Context, mediaID int64) (string, error) {
			return "", fmt.Errorf("MediaFileGetter not configured for media ID %d", mediaID)
		}
	}

	return &Queue{
		config:  config,
		repo:    repo,
		service: service,
		logger:  logger,
		jobChan: make(chan *transcode.TranscodeJob, config.WorkerCount*2), // Buffer for smoother operation
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

	q.logger.Info("transcode queue started",
		slog.Int("workers", q.config.WorkerCount),
		slog.Duration("poll_interval", q.config.PollInterval),
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

	// Execute transcode
	startTime := time.Now()
	err = q.service.TranscodeToDASH(ctx, job, inputPath, q.config.OutputBaseDir)
	duration := time.Since(startTime)

	if err != nil {
		q.logger.Error("transcode failed",
			slog.Int("worker_id", workerID),
			slog.Int64("job_id", job.ID),
			slog.Duration("duration", duration),
			slog.String("error", err.Error()),
		)
	} else {
		q.logger.Info("transcode completed successfully",
			slog.Int("worker_id", workerID),
			slog.Int64("job_id", job.ID),
			slog.Int64("media_id", job.MediaID),
			slog.String("quality", job.Quality),
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

// QueueStats contains queue statistics.
type QueueStats struct {
	QueuedJobs     int64
	ProcessingJobs int64
	CompletedJobs  int64
	FailedJobs     int64
	WorkerCount    int
}
