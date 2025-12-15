package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	appenrich "github.com/mantonx/viewra/internal/application/enrichment"
	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/infrastructure/events"
	"golang.org/x/time/rate"
)

// WorkerPool manages concurrent workers for a single enrichment stage.
type WorkerPool struct {
	deps       *Deps
	enricher   appenrich.Enricher
	typedRepos *TypedMediaRepos
	config     StageWorkerConfig
	limiter    *rate.Limiter
	wg         sync.WaitGroup

	// enqueueNext is called to enqueue the next stage after successful completion.
	// Set by Manager after creation.
	enqueueNext func(ctx context.Context, mediaID int64, mediaType enrichment.MediaType, currentPosition int) error
}

// NewWorkerPool creates a new worker pool for a stage.
func NewWorkerPool(deps *Deps, enricher appenrich.Enricher, typedRepos *TypedMediaRepos, config StageWorkerConfig) *WorkerPool {
	var limiter *rate.Limiter
	if config.RateLimit > 0 {
		limiter = rate.NewLimiter(rate.Limit(config.RateLimit), 1)
	}

	return &WorkerPool{
		deps:       deps,
		enricher:   enricher,
		typedRepos: typedRepos,
		config:     config,
		limiter:    limiter,
	}
}

// SetEnqueueNext sets the callback for enqueueing the next pipeline stage.
func (p *WorkerPool) SetEnqueueNext(fn func(ctx context.Context, mediaID int64, mediaType enrichment.MediaType, currentPosition int) error) {
	p.enqueueNext = fn
}

// Run starts the worker pool and processes jobs until context is cancelled.
func (p *WorkerPool) Run(ctx context.Context) {
	// Start worker goroutines
	for i := 0; i < p.config.Concurrency; i++ {
		p.wg.Add(1)
		go p.worker(ctx, i)
	}

	// Wait for all workers to finish
	p.wg.Wait()
}

// worker is the main processing loop for a single worker.
func (p *WorkerPool) worker(ctx context.Context, workerID int) {
	defer p.wg.Done()

	workerName := fmt.Sprintf("%s-worker-%d", p.config.Stage, workerID)
	logger := p.deps.Logger.With(
		slog.String("stage", p.config.Stage),
		slog.Int("worker_id", workerID),
	)

	pollInterval := 1 * time.Second

	for {
		select {
		case <-ctx.Done():
			logger.Debug("worker shutting down")
			return
		default:
		}

		// Apply rate limiting if configured
		if p.limiter != nil {
			if err := p.limiter.Wait(ctx); err != nil {
				if ctx.Err() != nil {
					return // Context cancelled
				}
				continue
			}
		}

		// Claim a batch of jobs
		jobs, err := p.deps.QueueRepo.ClaimBatch(ctx, p.config.Stage, workerName, p.config.BatchSize)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.Error("failed to claim jobs", slog.Any("error", err))
			time.Sleep(pollInterval)
			continue
		}

		if len(jobs) == 0 {
			// No jobs available, wait before polling again
			select {
			case <-ctx.Done():
				return
			case <-time.After(pollInterval):
			}
			continue
		}

		// Process each job
		for _, job := range jobs {
			if ctx.Err() != nil {
				return
			}
			p.processJob(ctx, logger, job)
		}
	}
}

// processJob handles a single enrichment job with timeout and error handling.
func (p *WorkerPool) processJob(ctx context.Context, logger *slog.Logger, job *enrichment.QueueJob) {
	logger = logger.With(
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
	req, mediaType, err := p.buildEnrichRequest(jobCtx, job)
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
	if err := p.applyEnrichResponse(jobCtx, job.MediaID, resp); err != nil {
		p.handleFailure(ctx, logger, job, mediaType, fmt.Errorf("apply response: %w", err), duration)
		return
	}

	p.handleSuccess(ctx, logger, job, mediaType, resp, duration)
}

// buildEnrichRequest fetches media data and builds an EnrichRequest.
// Uses job.MediaType for explicit routing - no fallback lookups needed.
func (p *WorkerPool) buildEnrichRequest(ctx context.Context, job *enrichment.QueueJob) (*pluginv1.EnrichRequest, enrichment.MediaType, error) {
	// Get existing external IDs for this media
	existingIDs := make(map[string]string)
	if extIDs, err := p.deps.ExternalIDRepo.GetByMedia(ctx, job.MediaID); err != nil {
		// Non-fatal - continue without existing IDs
		p.deps.Logger.Warn("failed to get external IDs", slog.Int64("media_id", job.MediaID), slog.Any("error", err))
	} else {
		for _, extID := range extIDs {
			existingIDs[extID.Provider] = extID.ExternalID
		}
	}

	// Route based on job.MediaType - this is now explicit, not inferred
	switch job.MediaType {
	case enrichment.MediaTypeTVShow:
		// Parent entity - TV show is in tv_shows table, not media table
		return p.buildTVShowRequest(ctx, job, existingIDs)

	case enrichment.MediaTypeMovie, enrichment.MediaTypeTV, enrichment.MediaTypeMusic:
		// Child entities - these are in the media table
		return p.buildMediaEntityRequest(ctx, job, existingIDs)

	default:
		return nil, "", fmt.Errorf("unsupported media type: %s", job.MediaType)
	}
}

// buildMediaEntityRequest builds a request for entities in the media table (movies, episodes, tracks).
func (p *WorkerPool) buildMediaEntityRequest(ctx context.Context, job *enrichment.QueueJob, existingIDs map[string]string) (*pluginv1.EnrichRequest, enrichment.MediaType, error) {
	media, err := p.deps.MediaRepo.GetByID(ctx, job.MediaID)
	if err != nil {
		return nil, "", fmt.Errorf("media ID %d not found: %w", job.MediaID, err)
	}

	req := &pluginv1.EnrichRequest{
		MediaId:     job.MediaID,
		MediaType:   string(job.MediaType),
		FilePath:    media.FilePath,
		Title:       media.Title,
		ExistingIds: existingIDs,
	}

	// Populate type-specific fields
	switch job.MediaType {
	case enrichment.MediaTypeMovie:
		if p.typedRepos != nil && p.typedRepos.Movie != nil {
			movie, err := p.typedRepos.Movie.GetMovieByID(ctx, job.MediaID)
			if err == nil && movie != nil {
				req.Year = int32(movie.Year)
			}
		}

	case enrichment.MediaTypeTV:
		if p.typedRepos != nil && p.typedRepos.TV != nil {
			episode, err := p.typedRepos.TV.GetTVEpisodeByID(ctx, job.MediaID)
			if err == nil && episode != nil {
				req.Tv = &pluginv1.TVMetadata{
					ShowTitle:     episode.ShowTitle,
					SeasonNumber:  int32(episode.Season),
					EpisodeNumber: int32(episode.Episode),
				}
			}
		}

	case enrichment.MediaTypeMusic:
		if p.typedRepos != nil && p.typedRepos.Music != nil {
			track, err := p.typedRepos.Music.GetMusicTrackByID(ctx, job.MediaID)
			if err == nil && track != nil {
				req.Music = &pluginv1.MusicMetadata{
					Artist:      track.Artist,
					Album:       track.Album,
					TrackNumber: int32(track.TrackNumber),
				}
			}
		}
	}

	return req, job.MediaType, nil
}

// buildTVShowRequest builds a request for TV shows (parent entities in tv_shows table).
func (p *WorkerPool) buildTVShowRequest(ctx context.Context, job *enrichment.QueueJob, existingIDs map[string]string) (*pluginv1.EnrichRequest, enrichment.MediaType, error) {
	if p.typedRepos == nil || p.typedRepos.TV == nil {
		return nil, "", fmt.Errorf("TV repository not configured")
	}

	show, err := p.typedRepos.TV.GetTVShowByID(ctx, job.MediaID)
	if err != nil {
		return nil, "", fmt.Errorf("TV show ID %d not found: %w", job.MediaID, err)
	}

	return &pluginv1.EnrichRequest{
		MediaId:     job.MediaID,
		MediaType:   string(enrichment.MediaTypeTVShow),
		FilePath:    show.Directory, // Use directory as file path for NFO lookup
		Title:       show.Title,
		Year:        int32(show.Year),
		ExistingIds: existingIDs,
	}, enrichment.MediaTypeTVShow, nil
}

// applyEnrichResponse applies enrichment results to the database.
func (p *WorkerPool) applyEnrichResponse(ctx context.Context, mediaID int64, resp *pluginv1.EnrichResponse) error {
	// Store discovered external IDs
	for provider, id := range resp.DiscoveredIds {
		extID := &enrichment.ExternalID{
			MediaID:    mediaID,
			Provider:   provider,
			ExternalID: id,
		}
		if err := p.deps.ExternalIDRepo.Upsert(ctx, extID); err != nil {
			return fmt.Errorf("store external ID %s: %w", provider, err)
		}
	}

	// TODO: Apply metadata updates to media record
	// This requires updating the media repository with the EnrichedMetadata fields
	// For Phase 2, we'll implement this when we have the NFO enricher working

	// TODO: Process images (local paths and remote URLs)
	// For Phase 2, LocalImages enricher will handle this

	return nil
}

// handleSuccess marks a job as completed and enqueues the next stage.
func (p *WorkerPool) handleSuccess(ctx context.Context, logger *slog.Logger, job *enrichment.QueueJob, mediaType enrichment.MediaType, resp *pluginv1.EnrichResponse, duration time.Duration) {
	logger.Info("job completed",
		slog.Duration("duration", duration),
		slog.Bool("matched", resp != nil && resp.Matched),
		slog.Bool("skipped", resp != nil && resp.Skipped))

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
		// TODO: Get current position from pipeline config
		// For now, use 0 - the manager will figure out the next stage
		if err := p.enqueueNext(ctx, job.MediaID, mediaType, 0); err != nil {
			logger.Error("failed to enqueue next stage", slog.Any("error", err))
		}
	}
}

// handleFailure handles job failure with retry logic.
func (p *WorkerPool) handleFailure(ctx context.Context, logger *slog.Logger, job *enrichment.QueueJob, mediaType enrichment.MediaType, err error, duration time.Duration) {
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

// categorizeError attempts to categorize an error for retry logic.
func categorizeError(err error) enrichment.ErrorCategory {
	if err == nil {
		return ""
	}

	errMsg := err.Error()

	// Network errors - should retry
	if containsAny(errMsg, "timeout", "connection refused", "no such host", "network") {
		return enrichment.ErrorCategoryNetwork
	}

	// Rate limiting - should retry with backoff
	if containsAny(errMsg, "rate limit", "429", "too many requests") {
		return enrichment.ErrorCategoryRateLimit
	}

	// Not found - probably shouldn't retry
	if containsAny(errMsg, "not found", "404") {
		return enrichment.ErrorCategoryNotFound
	}

	// Default to plugin error
	return enrichment.ErrorCategoryPlugin
}

// containsAny checks if s contains any of the substrings.
func containsAny(s string, substrings ...string) bool {
	for _, sub := range substrings {
		if len(sub) > 0 && len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
