package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	appenrich "github.com/mantonx/viewra/internal/application/enrichment"
	"github.com/mantonx/viewra/internal/domain/enrichment"
	"golang.org/x/time/rate"
)

// WorkerPool manages concurrent workers for a single enrichment stage.
type WorkerPool struct {
	deps           *Deps
	enricher       appenrich.Enricher
	typedRepos     *TypedMediaRepos
	pipelineCache  *PipelineCache
	config         StageWorkerConfig
	limiter        *rate.Limiter
	circuitBreaker *CircuitBreaker
	jobProcessor   *JobProcessor
	wg             sync.WaitGroup

	// enqueueNext is called to enqueue the next stage after successful completion.
	// Set by Manager after creation.
	enqueueNext func(ctx context.Context, mediaID int64, libraryID int64, mediaType enrichment.MediaType, currentPosition int) error
}

// NewWorkerPool creates a new worker pool for a stage.
func NewWorkerPool(deps *Deps, enricher appenrich.Enricher, typedRepos *TypedMediaRepos, pipelineCache *PipelineCache, entityCache *EntityCache, circuitBreaker *CircuitBreaker, config StageWorkerConfig) *WorkerPool {
	var limiter *rate.Limiter
	if config.RateLimit > 0 {
		// Burst size matches concurrency to allow all workers to acquire tokens in parallel.
		// This prevents idle workers when the rate limit hasn't been exceeded.
		burst := config.Concurrency
		if burst < 1 {
			burst = 1
		}
		limiter = rate.NewLimiter(rate.Limit(config.RateLimit), burst)
	}

	// Create the component chain
	logger := deps.Logger.With(slog.String("stage", config.Stage))
	metadataApplier := NewMetadataApplier(typedRepos, logger)
	creditsApplier := NewCreditsApplier(deps.PeopleRepo, logger)
	studiosApplier := NewStudiosApplier(deps.StudioRepo, logger)
	keywordsApplier := NewKeywordsApplier(deps.KeywordRepo, logger)

	// Build ImageProcessor with optional dependencies
	var imgOpts []ImageProcessorOption
	if deps.MetadataExtractor != nil {
		imgOpts = append(imgOpts, WithMetadataExtractor(deps.MetadataExtractor))
	}
	if deps.Transformer != nil {
		imgOpts = append(imgOpts, WithTransformer(deps.Transformer))
	}
	if deps.Downloader != nil {
		imgOpts = append(imgOpts, WithDownloader(deps.Downloader))
	}
	imageProcessor := NewImageProcessor(deps.ImageRepo, logger, imgOpts...)

	requestBuilder := NewRequestBuilder(deps, typedRepos, entityCache, logger)
	responseApplier := NewResponseApplier(deps, metadataApplier, creditsApplier, studiosApplier, keywordsApplier, imageProcessor, logger)
	jobProcessor := NewJobProcessor(deps, enricher, requestBuilder, responseApplier, pipelineCache, config, logger)

	pool := &WorkerPool{
		deps:           deps,
		enricher:       enricher,
		typedRepos:     typedRepos,
		pipelineCache:  pipelineCache,
		config:         config,
		limiter:        limiter,
		circuitBreaker: circuitBreaker,
		jobProcessor:   jobProcessor,
	}

	// Wire up circuit breaker callbacks
	if circuitBreaker != nil {
		jobProcessor.SetCircuitBreakerCallbacks(
			circuitBreaker.RecordSuccess,
			circuitBreaker.RecordFailure,
		)
	}

	return pool
}

// SetEnqueueNext sets the callback for enqueueing the next pipeline stage.
func (p *WorkerPool) SetEnqueueNext(fn func(ctx context.Context, mediaID int64, libraryID int64, mediaType enrichment.MediaType, currentPosition int) error) {
	p.enqueueNext = fn
	p.jobProcessor.SetEnqueueNext(fn)
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

		// Check circuit breaker before attempting to claim jobs
		if p.circuitBreaker != nil && !p.circuitBreaker.Allow() {
			// Circuit is open - wait before retrying
			status := p.circuitBreaker.Status()
			logger.Debug("circuit breaker open, waiting",
				slog.Duration("retry_after", status.RetryAfter))
			select {
			case <-ctx.Done():
				return
			case <-time.After(min(status.RetryAfter, 30*time.Second)):
				// Check again after waiting
			}
			continue
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

		// Prefetch external IDs for all claimed jobs (batch optimization)
		externalIDsMap := p.prefetchExternalIDs(ctx, jobs)

		// Process each job with prefetched external IDs
		for _, job := range jobs {
			if ctx.Err() != nil {
				return
			}
			p.jobProcessor.ProcessWithExternalIDs(ctx, job, externalIDsMap[job.MediaID])
		}
	}
}

// prefetchExternalIDs fetches external IDs for all jobs in a single batch query.
// Returns a map of mediaID -> map[provider]externalID for efficient lookup.
func (p *WorkerPool) prefetchExternalIDs(ctx context.Context, jobs []*enrichment.QueueJob) map[int64]map[string]string {
	result := make(map[int64]map[string]string)

	// Collect unique media IDs
	mediaIDs := make([]int64, 0, len(jobs))
	seen := make(map[int64]bool)
	for _, job := range jobs {
		if !seen[job.MediaID] {
			seen[job.MediaID] = true
			mediaIDs = append(mediaIDs, job.MediaID)
		}
	}

	if len(mediaIDs) == 0 {
		return result
	}

	// Batch fetch external IDs
	extIDsMap, err := p.deps.ExternalIDRepo.GetByMediaBatch(ctx, mediaIDs)
	if err != nil {
		// Non-fatal - individual jobs will fall back to their own lookup
		p.deps.Logger.Debug("failed to prefetch external IDs",
			slog.Int("job_count", len(jobs)),
			slog.Any("error", err))
		return result
	}

	// Convert to map[mediaID]map[provider]externalID
	for mediaID, extIDs := range extIDsMap {
		providerMap := make(map[string]string)
		for _, extID := range extIDs {
			providerMap[extID.Provider] = extID.ExternalID
		}
		result[mediaID] = providerMap
	}

	return result
}
