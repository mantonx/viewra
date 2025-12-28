package pipeline

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/domain/enrichment"
)

// EnqueueBuffer provides buffered, batched enqueueing of enrichment jobs.
// Instead of individual INSERT per media item, it collects jobs and flushes
// them in batches, reducing DB round-trips by ~100x during library scans.
//
// Usage:
//
//	buffer := NewEnqueueBuffer(manager, logger, WithBatchSize(500), WithFlushInterval(2*time.Second))
//	buffer.Start(ctx)
//	defer buffer.Stop()
//
//	// Enqueue jobs - these are batched automatically
//	buffer.Enqueue(mediaID, libraryID, mediaType, priority)
type EnqueueBuffer struct {
	manager       EnqueueManager
	logger        *slog.Logger
	batchSize     int
	flushInterval time.Duration

	jobChan chan enqueueJob
	wg      sync.WaitGroup
	stopCh  chan struct{}
	stopped bool
	mu      sync.Mutex
}

// enqueueJob represents a job to be enqueued.
type enqueueJob struct {
	MediaID   int64
	LibraryID int64
	MediaType enrichment.MediaType
	Priority  int
}

// EnqueueManager defines the interface for enqueueing jobs.
// This is typically the pipeline Manager.
type EnqueueManager interface {
	EnqueueFirstStage(ctx context.Context, mediaID int64, libraryID int64, mediaType enrichment.MediaType) error
	EnqueueFirstStageBatch(ctx context.Context, items []EnqueueItem) (int, error)
}

// EnqueueBufferOption configures the buffer.
type EnqueueBufferOption func(*EnqueueBuffer)

// WithBatchSize sets the maximum number of jobs to batch before flushing.
// Default: 500
func WithBatchSize(size int) EnqueueBufferOption {
	return func(b *EnqueueBuffer) {
		if size > 0 {
			b.batchSize = size
		}
	}
}

// WithFlushInterval sets the maximum time to wait before flushing a partial batch.
// Default: 2 seconds
func WithFlushInterval(d time.Duration) EnqueueBufferOption {
	return func(b *EnqueueBuffer) {
		if d > 0 {
			b.flushInterval = d
		}
	}
}

// NewEnqueueBuffer creates a new buffered enqueue writer.
func NewEnqueueBuffer(manager EnqueueManager, logger *slog.Logger, opts ...EnqueueBufferOption) *EnqueueBuffer {
	b := &EnqueueBuffer{
		manager:       manager,
		logger:        logger,
		batchSize:     500,
		flushInterval: 2 * time.Second,
		jobChan:       make(chan enqueueJob, 10000), // Large buffer to avoid blocking scanners
		stopCh:        make(chan struct{}),
	}

	for _, opt := range opts {
		opt(b)
	}

	return b
}

// Start begins the background worker that processes enqueue requests.
func (b *EnqueueBuffer) Start(ctx context.Context) {
	b.wg.Add(1)
	go b.worker(ctx)
}

// Stop gracefully stops the buffer, flushing any remaining jobs.
func (b *EnqueueBuffer) Stop() {
	b.mu.Lock()
	if b.stopped {
		b.mu.Unlock()
		return
	}
	b.stopped = true
	b.mu.Unlock()

	close(b.stopCh)
	b.wg.Wait()
}

// Enqueue adds a job to the buffer for batched processing.
// This is non-blocking and safe to call from multiple goroutines.
func (b *EnqueueBuffer) Enqueue(mediaID int64, libraryID int64, mediaType enrichment.MediaType, priority int) {
	b.mu.Lock()
	stopped := b.stopped
	b.mu.Unlock()

	if stopped {
		return
	}

	select {
	case b.jobChan <- enqueueJob{
		MediaID:   mediaID,
		LibraryID: libraryID,
		MediaType: mediaType,
		Priority:  priority,
	}:
	default:
		// Channel full - log and drop (scan can still proceed)
		b.logger.Warn("enqueue buffer full, dropping job",
			slog.Int64("media_id", mediaID),
			slog.String("media_type", string(mediaType)))
	}
}

// worker processes jobs from the channel in batches.
func (b *EnqueueBuffer) worker(ctx context.Context) {
	defer b.wg.Done()

	batch := make([]enqueueJob, 0, b.batchSize)
	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}

		startTime := time.Now()

		// Convert to EnqueueItem slice for batch enqueue
		items := make([]EnqueueItem, len(batch))
		for i, job := range batch {
			items[i] = EnqueueItem{
				MediaID:   job.MediaID,
				LibraryID: job.LibraryID,
				MediaType: job.MediaType,
				Priority:  job.Priority,
			}
		}

		// Use batch enqueue for efficiency
		successCount, err := b.manager.EnqueueFirstStageBatch(ctx, items)
		if err != nil {
			b.logger.Warn("batch enqueue failed, falling back to individual",
				slog.Int("batch_size", len(batch)),
				slog.Any("error", err))

			// Fallback to individual enqueues
			successCount = 0
			for _, job := range batch {
				if err := b.manager.EnqueueFirstStage(ctx, job.MediaID, job.LibraryID, job.MediaType); err == nil {
					successCount++
				}
			}
		}

		failCount := len(batch) - successCount

		b.logger.Debug("flushed enqueue batch",
			slog.Int("batch_size", len(batch)),
			slog.Int("success", successCount),
			slog.Int("failed", failCount),
			slog.Duration("duration", time.Since(startTime)))

		batch = batch[:0] // Reset batch
	}

	for {
		select {
		case <-ctx.Done():
			flush() // Flush remaining on context cancellation
			return

		case <-b.stopCh:
			// Drain remaining jobs from channel
			draining := true
			for draining {
				select {
				case job := <-b.jobChan:
					batch = append(batch, job)
					if len(batch) >= b.batchSize {
						flush()
					}
				default:
					draining = false
				}
			}
			flush() // Final flush
			return

		case job := <-b.jobChan:
			batch = append(batch, job)
			if len(batch) >= b.batchSize {
				flush()
			}

		case <-ticker.C:
			flush() // Time-based flush for partial batches
		}
	}
}

// Pending returns the approximate number of jobs waiting to be processed.
func (b *EnqueueBuffer) Pending() int {
	return len(b.jobChan)
}
