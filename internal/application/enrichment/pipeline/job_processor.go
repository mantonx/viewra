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
	deps            *Deps
	enricher        appenrich.Enricher
	requestBuilder  *RequestBuilder
	responseApplier *ResponseApplier
	config          StageWorkerConfig
	logger          *slog.Logger

	// enqueueNext is called to enqueue the next stage after successful completion.
	enqueueNext func(ctx context.Context, mediaID int64, mediaType enrichment.MediaType, currentPosition int) error
}

// NewJobProcessor creates a new JobProcessor.
func NewJobProcessor(
	deps *Deps,
	enricher appenrich.Enricher,
	requestBuilder *RequestBuilder,
	responseApplier *ResponseApplier,
	config StageWorkerConfig,
	logger *slog.Logger,
) *JobProcessor {
	return &JobProcessor{
		deps:            deps,
		enricher:        enricher,
		requestBuilder:  requestBuilder,
		responseApplier: responseApplier,
		config:          config,
		logger:          logger,
	}
}

// SetEnqueueNext sets the callback for enqueueing the next pipeline stage.
func (p *JobProcessor) SetEnqueueNext(fn func(ctx context.Context, mediaID int64, mediaType enrichment.MediaType, currentPosition int) error) {
	p.enqueueNext = fn
}

// Process handles a single enrichment job with timeout and error handling.
func (p *JobProcessor) Process(ctx context.Context, job *enrichment.QueueJob) {
	logger := p.logger.With(
		slog.Int64("job_id", job.ID),
		slog.Int64("media_id", job.MediaID),
		slog.Int("attempt", job.Attempts+1),
	)

	// Create timeout context for this job
	jobCtx, cancel := context.WithTimeout(ctx, time.Duration(p.config.Timeout)*time.Second)
	defer cancel()

	// Publish start event
	p.deps.EventBus.Publish(events.NewEvent(events.EventEnrichmentStarted, "worker").
		WithMediaID(job.MediaID).
		WithStage(p.config.Stage).
		Build())

	// Update status to processing
	if err := p.deps.StatusRepo.Upsert(jobCtx, &enrichment.Status{
		MediaID: job.MediaID,
		Stage:   job.Stage,
		Status:  enrichment.JobStatusProcessing,
	}); err != nil {
		logger.Error("failed to update status", slog.Any("error", err))
	}

	// Build EnrichRequest from job
	startTime := time.Now()
	req, mediaType, err := p.requestBuilder.Build(jobCtx, job)
	if err != nil {
		p.handleFailure(ctx, logger, job, enrichment.MediaType(""), fmt.Errorf("build request: %w", err), time.Since(startTime))
		return
	}

	// Call the enricher
	resp, err := p.enricher.Enrich(jobCtx, req)
	duration := time.Since(startTime)

	if err != nil {
		p.handleFailure(ctx, logger, job, mediaType, err, duration)
		return
	}

	// Handle skipped jobs (not an error, just nothing to do)
	if resp.Skipped {
		logger.Debug("job skipped", slog.String("reason", resp.SkipReason))
		p.handleSuccess(ctx, logger, job, mediaType, resp, duration)
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
	logger.Info("job completed",
		slog.Duration("duration", duration),
		slog.Bool("matched", resp != nil && resp.GetMatched()),
		slog.Bool("skipped", resp != nil && resp.GetSkipped()))

	// Mark job as completed
	if err := p.deps.QueueRepo.Complete(ctx, job.ID); err != nil {
		logger.Error("failed to mark job complete", slog.Any("error", err))
	}

	// Update status
	if err := p.deps.StatusRepo.MarkComplete(ctx, job.MediaID, job.Stage, "", ""); err != nil {
		logger.Error("failed to update status", slog.Any("error", err))
	}

	// Publish completion event
	p.deps.EventBus.Publish(events.NewEvent(events.EventEnrichmentStageComplete, "worker").
		WithMediaID(job.MediaID).
		WithStage(p.config.Stage).
		Build())

	// Enqueue next stage if callback is set
	if p.enqueueNext != nil && mediaType != "" {
		// Get current stage position from pipeline config
		currentPosition := 0
		if stage, err := p.deps.PipelineRepo.GetStageByName(ctx, mediaType, job.Stage); err != nil {
			logger.Warn("failed to get stage position, using 0",
				slog.String("stage", job.Stage),
				slog.Any("error", err))
		} else if stage != nil {
			currentPosition = stage.Position
		}

		if err := p.enqueueNext(ctx, job.MediaID, mediaType, currentPosition); err != nil {
			logger.Error("failed to enqueue next stage", slog.Any("error", err))
		}
	}
}

// handleFailure handles job failure with retry logic.
func (p *JobProcessor) handleFailure(ctx context.Context, logger *slog.Logger, job *enrichment.QueueJob, mediaType enrichment.MediaType, err error, duration time.Duration) {
	// Categorize the error
	category := categorizeError(err)

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
	if dbErr := p.deps.StatusRepo.MarkFailed(ctx, job.MediaID, job.Stage, "", err.Error()); dbErr != nil {
		logger.Error("failed to update status", slog.Any("error", dbErr))
	}

	// Publish failure event
	p.deps.EventBus.Publish(events.NewEvent(events.EventEnrichmentFailed, "worker").
		WithMediaID(job.MediaID).
		WithStage(p.config.Stage).
		WithError(err).
		Build())
}
