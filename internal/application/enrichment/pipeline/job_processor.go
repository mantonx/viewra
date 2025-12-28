package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	appenrich "github.com/mantonx/viewra/internal/application/enrichment"
	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/infrastructure/events"
)

// JobProcessor handles the execution of a single enrichment job.
type JobProcessor struct {
	deps              *Deps
	enricher          appenrich.Enricher
	requestBuilder    *RequestBuilder
	responseApplier   *ResponseApplier
	pipelineCache     *PipelineCache
	throughputTracker *ThroughputTracker
	config            StageWorkerConfig
	logger            *slog.Logger

	// enqueueNext is called to enqueue the next stage after successful completion.
	enqueueNext func(ctx context.Context, mediaID int64, libraryID int64, mediaType enrichment.MediaType, currentPosition int) error

	// Circuit breaker callbacks for recording enricher results.
	// These are called after enricher.Enrich() returns.
	onEnricherSuccess func()
	onEnricherFailure func()
}

// NewJobProcessor creates a new JobProcessor.
func NewJobProcessor(
	deps *Deps,
	enricher appenrich.Enricher,
	requestBuilder *RequestBuilder,
	responseApplier *ResponseApplier,
	pipelineCache *PipelineCache,
	config StageWorkerConfig,
	logger *slog.Logger,
) *JobProcessor {
	return &JobProcessor{
		deps:            deps,
		enricher:        enricher,
		requestBuilder:  requestBuilder,
		responseApplier: responseApplier,
		pipelineCache:   pipelineCache,
		config:          config,
		logger:          logger,
	}
}

// SetEnqueueNext sets the callback for enqueueing the next pipeline stage.
func (p *JobProcessor) SetEnqueueNext(fn func(ctx context.Context, mediaID int64, libraryID int64, mediaType enrichment.MediaType, currentPosition int) error) {
	p.enqueueNext = fn
}

// SetCircuitBreakerCallbacks sets callbacks for recording success/failure to the circuit breaker.
func (p *JobProcessor) SetCircuitBreakerCallbacks(onSuccess, onFailure func()) {
	p.onEnricherSuccess = onSuccess
	p.onEnricherFailure = onFailure
}

// SetThroughputTracker sets the throughput tracker for recording completion metrics.
func (p *JobProcessor) SetThroughputTracker(tracker *ThroughputTracker) {
	p.throughputTracker = tracker
}

// Process handles a single enrichment job with timeout and error handling.
// This method fetches external IDs on-demand. For batch processing, use ProcessWithExternalIDs.
func (p *JobProcessor) Process(ctx context.Context, job *enrichment.QueueJob) {
	p.ProcessWithExternalIDs(ctx, job, nil)
}

// ProcessWithExternalIDs handles a single enrichment job with prefetched external IDs.
// If prefetchedIDs is nil, external IDs will be fetched on-demand.
func (p *JobProcessor) ProcessWithExternalIDs(ctx context.Context, job *enrichment.QueueJob, prefetchedIDs map[string]string) {
	logger := p.logger.With(
		slog.Int64("job_id", job.ID),
		slog.Int64("media_id", job.MediaID),
		slog.Int("attempt", job.Attempts+1),
	)

	// Create timeout context for this job
	jobCtx, cancel := context.WithTimeout(ctx, time.Duration(p.config.Timeout)*time.Second)
	defer cancel()

	// Publish start event with library_id for SSE filtering
	p.deps.EventBus.Publish(events.NewEvent(events.EventEnrichmentStarted, "worker").
		WithMediaID(job.MediaID).
		WithLibraryID(job.LibraryID).
		WithStage(p.config.Stage).
		Build())

	// Update status to processing
	if err := p.deps.StatusRepo.Upsert(jobCtx, &enrichment.Status{
		MediaType: job.MediaType,
		MediaID:   job.MediaID,
		Stage:     job.Stage,
		Status:    enrichment.JobStatusProcessing,
	}); err != nil {
		logger.Error("failed to update status", slog.Any("error", err))
	}

	// Build EnrichRequest from job (using prefetched external IDs if available)
	startTime := time.Now()
	req, mediaType, err := p.requestBuilder.BuildWithExternalIDs(jobCtx, job, prefetchedIDs)
	if err != nil {
		p.handleFailure(ctx, logger, job, job.MediaType, fmt.Errorf("build request: %w", err), time.Since(startTime))
		return
	}

	// Call the enricher
	resp, err := p.enricher.Enrich(jobCtx, req)
	duration := time.Since(startTime)

	if err != nil {
		// Record failure for circuit breaker (only for enricher errors, not build errors)
		if p.onEnricherFailure != nil {
			p.onEnricherFailure()
		}
		p.handleFailure(ctx, logger, job, mediaType, err, duration)
		return
	}

	// Record success for circuit breaker
	if p.onEnricherSuccess != nil {
		p.onEnricherSuccess()
	}

	// Handle skipped jobs (not an error, just nothing to do)
	if resp.Skipped {
		logger.Debug("job skipped", slog.String("reason", resp.SkipReason))
		p.handleSkipped(ctx, logger, job, mediaType, resp, duration)
		return
	}

	// Apply enrichment results
	if err := p.responseApplier.Apply(jobCtx, job, mediaType, resp); err != nil {
		p.handleFailure(ctx, logger, job, mediaType, fmt.Errorf("apply response: %w", err), duration)
		return
	}

	p.handleSuccess(ctx, logger, job, mediaType, resp, duration)
}

// handleSuccess marks a job as completed and enqueues the next stage.
func (p *JobProcessor) handleSuccess(ctx context.Context, logger *slog.Logger, job *enrichment.QueueJob, mediaType enrichment.MediaType, resp *pluginv1.EnrichResponse, duration time.Duration) {
	logger.Debug("job completed",
		slog.Duration("duration", duration),
		slog.Bool("matched", resp != nil && resp.GetMatched()),
		slog.Bool("skipped", resp != nil && resp.GetSkipped()))

	// Record throughput for ETA calculation
	if p.throughputTracker != nil {
		p.throughputTracker.RecordCompletion(job.Stage)
	}

	// Mark job as completed
	if err := p.deps.QueueRepo.Complete(ctx, job.ID); err != nil {
		logger.Error("failed to mark job complete", slog.Any("error", err))
	}

	// Update status
	if err := p.deps.StatusRepo.MarkComplete(ctx, mediaType, job.MediaID, job.Stage, "", ""); err != nil {
		logger.Error("failed to update status", slog.Any("error", err))
	}

	// Publish completion event with library_id for SSE filtering
	p.deps.EventBus.Publish(events.NewEvent(events.EventEnrichmentStageComplete, "worker").
		WithMediaID(job.MediaID).
		WithLibraryID(job.LibraryID).
		WithStage(p.config.Stage).
		Build())

	// Enqueue next stage if callback is set
	if p.enqueueNext != nil && mediaType != "" {
		// Get current stage position from pipeline config (cached)
		currentPosition := 0
		if stage, err := p.pipelineCache.GetStageByName(ctx, mediaType, job.Stage); err != nil {
			logger.Warn("failed to get stage position, using 0",
				slog.String("stage", job.Stage),
				slog.Any("error", err))
		} else if stage != nil {
			currentPosition = stage.Position
		}

		if err := p.enqueueNext(ctx, job.MediaID, job.LibraryID, mediaType, currentPosition); err != nil {
			logger.Error("failed to enqueue next stage", slog.Any("error", err))
		}
	}
}

// handleSkipped marks a job as skipped and enqueues the next stage.
// Skipped jobs are not failures - they just had nothing to do (e.g., no NFO file found).
func (p *JobProcessor) handleSkipped(ctx context.Context, logger *slog.Logger, job *enrichment.QueueJob, mediaType enrichment.MediaType, resp *pluginv1.EnrichResponse, duration time.Duration) {
	logger.Debug("job skipped",
		slog.Duration("duration", duration),
		slog.String("reason", resp.GetSkipReason()))

	// Record throughput for ETA calculation (skipped jobs still count as completed)
	if p.throughputTracker != nil {
		p.throughputTracker.RecordCompletion(job.Stage)
	}

	// Mark job as completed (skipped jobs are considered complete)
	if err := p.deps.QueueRepo.Complete(ctx, job.ID); err != nil {
		logger.Error("failed to mark job complete", slog.Any("error", err))
	}

	// Update status to skipped
	if err := p.deps.StatusRepo.MarkSkipped(ctx, mediaType, job.MediaID, job.Stage, ""); err != nil {
		logger.Error("failed to update status", slog.Any("error", err))
	}

	// Publish skipped event with library_id for SSE filtering
	p.deps.EventBus.Publish(events.NewEvent(events.EventEnrichmentSkipped, "worker").
		WithMediaID(job.MediaID).
		WithLibraryID(job.LibraryID).
		WithStage(p.config.Stage).
		WithData("reason", resp.GetSkipReason()).
		Build())

	// Enqueue next stage if callback is set (skipped jobs still advance the pipeline)
	if p.enqueueNext != nil && mediaType != "" {
		// Get current stage position from pipeline config (cached)
		currentPosition := 0
		if stage, err := p.pipelineCache.GetStageByName(ctx, mediaType, job.Stage); err != nil {
			logger.Warn("failed to get stage position, using 0",
				slog.String("stage", job.Stage),
				slog.Any("error", err))
		} else if stage != nil {
			currentPosition = stage.Position
		}

		if err := p.enqueueNext(ctx, job.MediaID, job.LibraryID, mediaType, currentPosition); err != nil {
			logger.Error("failed to enqueue next stage", slog.Any("error", err))
		}
	}
}

// handleFailure handles job failure with retry logic.
func (p *JobProcessor) handleFailure(ctx context.Context, logger *slog.Logger, job *enrichment.QueueJob, mediaType enrichment.MediaType, err error, duration time.Duration) {
	// Categorize the error
	category := categorizeError(err)

	// Check if this is a transient connection error (plugin restart, server restart, etc.)
	// These should not count against the retry limit - just re-queue immediately
	isConnError := IsConnectionError(err)

	if isConnError {
		// Re-queue the job without incrementing attempts
		// Use longer delay (30s) to avoid log spam during plugin restarts
		retryTime := time.Now().Add(30 * time.Second)
		if dbErr := p.deps.QueueRepo.FailWithoutPenalty(ctx, job.ID, err.Error(), category, &retryTime); dbErr != nil {
			logger.Error("failed to re-queue job", slog.Any("error", dbErr))
		}

		// Log at Debug level to avoid spam - connection errors during plugin reload are expected
		logger.Debug("job deferred due to connection error, will retry in 30s",
			slog.Any("error", err),
			slog.Duration("duration", duration))

		// Don't update status to failed for transient errors - keep it as processing
		// so the UI doesn't flash failed/retry states

		return
	}

	logger.Error("job failed",
		slog.Any("error", err),
		slog.String("category", string(category)),
		slog.Duration("duration", duration))

	// Mark job as failed (with retry if appropriate)
	// Calculate retry time based on error category
	var nextRetryAt *time.Time
	if category.IsRetryable() && job.ShouldRetry() {
		// Exponential backoff: 30s, 60s, 120s, etc.
		delay := time.Duration(30*(1<<job.Attempts)) * time.Second
		retryTime := time.Now().Add(delay)
		nextRetryAt = &retryTime
	}
	if dbErr := p.deps.QueueRepo.Fail(ctx, job.ID, err.Error(), category, nextRetryAt); dbErr != nil {
		logger.Error("failed to mark job failed", slog.Any("error", dbErr))
	}

	// Update status
	if dbErr := p.deps.StatusRepo.MarkFailed(ctx, mediaType, job.MediaID, job.Stage, "", err.Error()); dbErr != nil {
		logger.Error("failed to update status", slog.Any("error", dbErr))
	}

	// Publish failure event with library_id for SSE filtering
	p.deps.EventBus.Publish(events.NewEvent(events.EventEnrichmentFailed, "worker").
		WithMediaID(job.MediaID).
		WithLibraryID(job.LibraryID).
		WithStage(p.config.Stage).
		WithError(err).
		Build())
}
