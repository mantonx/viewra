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
	deps         *Deps
	enricher     appenrich.Enricher
	typedRepos   *TypedMediaRepos
	config       StageWorkerConfig
	limiter      *rate.Limiter
	jobProcessor *JobProcessor
	wg           sync.WaitGroup

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

	// Create the component chain
	logger := deps.Logger.With(slog.String("stage", config.Stage))
	metadataApplier := NewMetadataApplier(typedRepos, logger)
	creditsApplier := NewCreditsApplier(deps.PeopleRepo, logger)
	studiosApplier := NewStudiosApplier(deps.StudioRepo, logger)

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

	requestBuilder := NewRequestBuilder(deps, typedRepos, logger)
	responseApplier := NewResponseApplier(deps, metadataApplier, creditsApplier, studiosApplier, imageProcessor, logger)
	jobProcessor := NewJobProcessor(deps, enricher, requestBuilder, responseApplier, config, logger)

	return &WorkerPool{
		deps:         deps,
		enricher:     enricher,
		typedRepos:   typedRepos,
		config:       config,
		limiter:      limiter,
		jobProcessor: jobProcessor,
	}
}

// SetEnqueueNext sets the callback for enqueueing the next pipeline stage.
func (p *WorkerPool) SetEnqueueNext(fn func(ctx context.Context, mediaID int64, mediaType enrichment.MediaType, currentPosition int) error) {
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
			p.jobProcessor.Process(ctx, job)
		}
	}
}
