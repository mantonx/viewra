package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	appenrich "github.com/mantonx/viewra/internal/application/enrichment"
	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/infrastructure/events"
)

// Manager orchestrates the enrichment pipeline.
// It manages job queuing and coordinates worker pools for each stage.
type Manager struct {
	deps             *Deps
	typedRepos       *TypedMediaRepos
	pipelineCache    *PipelineCache
	entityCache      *EntityCache
	circuitBreakers  *CircuitBreakerRegistry
	workerPools      map[string]*WorkerPool
	enrichers        map[string]appenrich.Enricher
	mu               sync.RWMutex
	running          bool
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
}

// PipelineCacheTTL is the default TTL for pipeline configuration cache.
// 5 minutes provides good balance between freshness and performance.
const PipelineCacheTTL = 5 * time.Minute

// EntityCacheMaxSize is the default max entries per entity type in the cache.
const EntityCacheMaxSize = 10000

// NewManager creates a new pipeline manager.
func NewManager(deps *Deps, typedRepos *TypedMediaRepos) *Manager {
	return &Manager{
		deps:            deps,
		typedRepos:      typedRepos,
		pipelineCache:   NewPipelineCache(deps.PipelineRepo, PipelineCacheTTL),
		entityCache:     NewEntityCache(EntityCacheMaxSize),
		circuitBreakers: NewCircuitBreakerRegistry(),
		workerPools:     make(map[string]*WorkerPool),
		enrichers:       make(map[string]appenrich.Enricher),
	}
}

// RegisterEnricher registers an enricher for a stage.
// Must be called before Start().
func (m *Manager) RegisterEnricher(enricher appenrich.Enricher) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enrichers[enricher.Stage()] = enricher
}

// RegisterExternalEnricher registers an external plugin enricher and ensures
// pipeline configuration exists for it (disabled by default).
// This is called when external plugins are loaded.
func (m *Manager) RegisterExternalEnricher(ctx context.Context, enricher appenrich.Enricher) error {
	// First register the enricher
	m.RegisterEnricher(enricher)

	stage := enricher.Stage()
	caps := enricher.Capabilities()

	// For each supported media type, ensure a pipeline stage exists
	for _, mt := range caps.MediaTypes {
		mediaType := enrichment.MediaType(mt)

		// Check if stage already exists
		existing, err := m.deps.PipelineRepo.GetStageByName(ctx, mediaType, stage)
		if err != nil {
			m.deps.Logger.Warn("failed to check for existing pipeline stage",
				"stage", stage,
				"media_type", mt,
				"error", err)
			continue
		}

		if existing != nil {
			m.deps.Logger.Debug("pipeline stage already exists",
				"stage", stage,
				"media_type", mt,
				"enabled", existing.Enabled)
			continue
		}

		// Find the next available position (after existing stages)
		allStages, err := m.deps.PipelineRepo.GetAllStages(ctx, mediaType)
		if err != nil {
			m.deps.Logger.Warn("failed to get existing stages",
				"media_type", mt,
				"error", err)
			continue
		}

		// Position is one after the last stage
		position := len(allStages) + 1

		// Create new stage (enabled by default for external plugins)
		newStage := &enrichment.PipelineStage{
			MediaType: mediaType,
			PluginID:  stage,
			StageName: stage,
			Position:  position,
			Enabled:   true,
		}

		if _, err := m.deps.PipelineRepo.Create(ctx, newStage); err != nil {
			m.deps.Logger.Warn("failed to create pipeline stage",
				"stage", stage,
				"media_type", mt,
				"position", position,
				"error", err)
			continue
		}

		m.deps.Logger.Info("registered external plugin pipeline stage",
			"stage", stage,
			"media_type", mt,
			"position", position,
			"enabled", false)
	}

	return nil
}

// Start begins all worker pools.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return errors.New("pipeline manager already running")
	}

	// Recover from any previous crashes by resetting stuck jobs
	if err := m.recoverStuckJobs(ctx); err != nil {
		m.deps.Logger.Warn("failed to recover stuck jobs", slog.Any("error", err))
		// Continue anyway - this shouldn't prevent startup
	}

	m.ctx, m.cancel = context.WithCancel(ctx)
	m.running = true

	// Wire circuit breaker state change callback to event bus
	m.circuitBreakers.SetOnStateChange(func(stage string, oldState, newState CircuitState) {
		m.deps.EventBus.Publish(events.NewEvent("enrichment.circuit_state", "pipeline").
			WithData("stage", stage).
			WithData("old_state", string(oldState)).
			WithData("new_state", string(newState)).
			Build())
		m.deps.Logger.Info("circuit breaker state changed",
			slog.String("stage", stage),
			slog.String("old_state", string(oldState)),
			slog.String("new_state", string(newState)))
	})

	// Start worker pools for each registered enricher
	for stage, enricher := range m.enrichers {
		config := m.getStageConfig(stage, enricher.Capabilities())
		circuitBreaker := m.circuitBreakers.Get(stage)
		pool := NewWorkerPool(m.deps, enricher, m.typedRepos, m.pipelineCache, m.entityCache, circuitBreaker, config)
		pool.SetEnqueueNext(m.EnqueueNextStage)
		m.workerPools[stage] = pool

		m.wg.Add(1)
		go func(p *WorkerPool) {
			defer m.wg.Done()
			p.Run(m.ctx)
		}(pool)

		m.deps.Logger.Info("started worker pool",
			slog.String("stage", stage),
			slog.Int("concurrency", config.Concurrency))
	}

	m.deps.Logger.Info("pipeline manager started",
		slog.Int("stages", len(m.enrichers)))

	return nil
}

// Stop gracefully stops all worker pools.
func (m *Manager) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	m.cancel()
	m.mu.Unlock()

	// Wait for all worker pools to finish
	m.wg.Wait()

	m.deps.Logger.Info("pipeline manager stopped")
}

// recoverStuckJobs resets jobs that were stuck in 'processing' status from a previous crash.
// This ensures that jobs orphaned by worker crashes are re-queued for processing.
// It also recovers orphaned pipeline states where a stage completed but the next was never enqueued.
func (m *Manager) recoverStuckJobs(ctx context.Context) error {
	// Reset stuck queue jobs (processing for > 10 minutes)
	const maxLockSeconds = 600 // 10 minutes
	if err := m.deps.QueueRepo.ReleaseStuck(ctx, maxLockSeconds); err != nil {
		return fmt.Errorf("release stuck queue jobs: %w", err)
	}

	// Reset stuck status records
	count, err := m.deps.StatusRepo.ResetStuck(ctx)
	if err != nil {
		return fmt.Errorf("reset stuck status records: %w", err)
	}

	if count > 0 {
		m.deps.Logger.Info("recovered stuck enrichment jobs",
			slog.Int64("status_records_reset", count))
	}

	// Recover orphaned pipeline states - where a stage completed but next stage was never enqueued.
	// This happens when the server crashes between marking complete and enqueuing next.
	orphaned, err := m.deps.QueueRepo.GetOrphanedPipelineStates(ctx)
	if err != nil {
		return fmt.Errorf("get orphaned pipeline states: %w", err)
	}

	for _, state := range orphaned {
		job := &enrichment.QueueJob{
			MediaID:     state.MediaID,
			MediaType:   state.MediaType,
			Stage:       state.NextStage,
			Priority:    0,
			Status:      enrichment.JobStatusPending,
			MaxAttempts: 3,
		}

		if _, err := m.deps.QueueRepo.Enqueue(ctx, job); err != nil {
			m.deps.Logger.Warn("failed to enqueue orphaned pipeline job",
				slog.String("media_type", string(state.MediaType)),
				slog.Int64("media_id", state.MediaID),
				slog.String("stage", state.NextStage),
				slog.Any("error", err))
			continue
		}
	}

	if len(orphaned) > 0 {
		m.deps.Logger.Info("recovered orphaned pipeline states",
			slog.Int("count", len(orphaned)))
	}

	return nil
}

// EnqueueFirstStage enqueues a media item for the first stage of its pipeline.
// Called after media is discovered/saved during scanning.
// libraryID is used for SSE event filtering so clients can subscribe to a specific library's progress.
// priority determines processing order (higher = processed sooner). Use CalculatePriorityFromMetadata().
func (m *Manager) EnqueueFirstStage(ctx context.Context, mediaID int64, libraryID int64, mediaType enrichment.MediaType, priority int) error {
	// Get the first enabled stage for this media type (cached)
	firstStage, err := m.pipelineCache.GetFirstStage(ctx, mediaType)
	if err != nil {
		return fmt.Errorf("get first stage: %w", err)
	}

	if firstStage == nil {
		// No pipeline configured for this media type - nothing to do
		m.deps.Logger.Debug("no pipeline configured",
			slog.Int64("media_id", mediaID),
			slog.String("media_type", string(mediaType)))
		return nil
	}

	// Enqueue for the first stage
	job := &enrichment.QueueJob{
		MediaID:     mediaID,
		LibraryID:   libraryID,
		MediaType:   mediaType,
		Stage:       firstStage.StageName,
		Priority:    priority,
		Status:      enrichment.JobStatusPending,
		MaxAttempts: 3,
	}

	if _, err := m.deps.QueueRepo.Enqueue(ctx, job); err != nil {
		return fmt.Errorf("enqueue first stage: %w", err)
	}

	// Publish event with library_id for SSE filtering
	m.deps.EventBus.Publish(events.NewEvent(events.EventEnrichmentQueued, "pipeline").
		WithMediaID(mediaID).
		WithLibraryID(libraryID).
		WithStage(firstStage.StageName).
		Build())

	return nil
}

// EnqueueFirstStageBatch enqueues multiple media items for their first pipeline stages.
// This is much more efficient than individual EnqueueFirstStage calls during library scans.
// Returns the number of successfully enqueued jobs.
func (m *Manager) EnqueueFirstStageBatch(ctx context.Context, items []EnqueueItem) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}

	// Group items by media type for efficient stage lookup
	byType := make(map[enrichment.MediaType][]EnqueueItem)
	for _, item := range items {
		byType[item.MediaType] = append(byType[item.MediaType], item)
	}

	// Build jobs for each media type
	var jobs []*enrichment.QueueJob
	for mediaType, typeItems := range byType {
		// Get first stage for this media type (cached)
		firstStage, err := m.pipelineCache.GetFirstStage(ctx, mediaType)
		if err != nil {
			m.deps.Logger.Warn("failed to get first stage for batch",
				slog.String("media_type", string(mediaType)),
				slog.Any("error", err))
			continue
		}

		if firstStage == nil {
			// No pipeline for this media type - skip
			continue
		}

		for _, item := range typeItems {
			jobs = append(jobs, &enrichment.QueueJob{
				MediaID:     item.MediaID,
				LibraryID:   item.LibraryID,
				MediaType:   mediaType,
				Stage:       firstStage.StageName,
				Priority:    item.Priority,
				Status:      enrichment.JobStatusPending,
				MaxAttempts: 3,
			})
		}
	}

	if len(jobs) == 0 {
		return 0, nil
	}

	// Batch enqueue
	count, err := m.deps.QueueRepo.EnqueueBatch(ctx, jobs)
	if err != nil {
		return 0, fmt.Errorf("batch enqueue: %w", err)
	}

	// Publish batch event (single event for all items)
	if count > 0 {
		m.deps.EventBus.Publish(events.NewEvent(events.EventEnrichmentQueued, "pipeline").
			WithData("batch_size", count).
			Build())
	}

	return count, nil
}

// EnqueueItem represents a media item to be enqueued for enrichment.
type EnqueueItem struct {
	MediaID   int64
	LibraryID int64
	MediaType enrichment.MediaType
	Priority  int
}

// EnqueueNextStage enqueues a media item for the next pipeline stage.
// Called after a stage completes successfully.
// libraryID is passed through from the completing job for SSE event filtering.
func (m *Manager) EnqueueNextStage(ctx context.Context, mediaID int64, libraryID int64, mediaType enrichment.MediaType, currentPosition int) error {
	// Get the next enabled stage after the current position (cached)
	nextStage, err := m.pipelineCache.GetNextStage(ctx, mediaType, currentPosition)
	if err != nil {
		return fmt.Errorf("get next stage: %w", err)
	}

	if nextStage == nil {
		// No more stages - enrichment complete
		m.deps.EventBus.Publish(events.NewEvent(events.EventEnrichmentComplete, "pipeline").
			WithMediaID(mediaID).
			WithLibraryID(libraryID).
			Build())
		return nil
	}

	// Enqueue for next stage
	job := &enrichment.QueueJob{
		MediaID:     mediaID,
		LibraryID:   libraryID,
		MediaType:   mediaType,
		Stage:       nextStage.StageName,
		Priority:    0,
		Status:      enrichment.JobStatusPending,
		MaxAttempts: 3,
	}

	if _, err := m.deps.QueueRepo.Enqueue(ctx, job); err != nil {
		return fmt.Errorf("enqueue next stage: %w", err)
	}

	m.deps.EventBus.Publish(events.NewEvent(events.EventEnrichmentQueued, "pipeline").
		WithMediaID(mediaID).
		WithLibraryID(libraryID).
		WithStage(nextStage.StageName).
		Build())

	return nil
}

// EnqueueStage enqueues a media item for a specific stage.
// Useful for manual re-processing or custom stage triggers.
// libraryID is used for SSE event filtering.
func (m *Manager) EnqueueStage(ctx context.Context, mediaID int64, libraryID int64, mediaType enrichment.MediaType, stage string, priority int) error {
	job := &enrichment.QueueJob{
		MediaID:     mediaID,
		LibraryID:   libraryID,
		MediaType:   mediaType,
		Stage:       stage,
		Priority:    priority,
		Status:      enrichment.JobStatusPending,
		MaxAttempts: 3,
	}

	if _, err := m.deps.QueueRepo.Enqueue(ctx, job); err != nil {
		return fmt.Errorf("enqueue stage %s: %w", stage, err)
	}

	m.deps.EventBus.Publish(events.NewEvent(events.EventEnrichmentQueued, "pipeline").
		WithMediaID(mediaID).
		WithLibraryID(libraryID).
		WithStage(stage).
		Build())

	return nil
}

// GetStats returns queue statistics for all stages.
func (m *Manager) GetStats(ctx context.Context) (map[string]*enrichment.QueueStats, error) {
	m.mu.RLock()
	stages := make([]string, 0, len(m.enrichers))
	for stage := range m.enrichers {
		stages = append(stages, stage)
	}
	m.mu.RUnlock()

	stats := make(map[string]*enrichment.QueueStats)
	for _, stage := range stages {
		s, err := m.deps.QueueRepo.GetStats(ctx, stage)
		if err != nil {
			return nil, fmt.Errorf("get stats for %s: %w", stage, err)
		}
		stats[stage] = s
	}

	return stats, nil
}

// InvalidatePipelineCache forces a refresh of the pipeline configuration cache.
// Call this after modifying pipeline stages via the API.
func (m *Manager) InvalidatePipelineCache() {
	m.pipelineCache.Invalidate()
}

// GetCircuitBreakerStatuses returns the current status of all circuit breakers.
func (m *Manager) GetCircuitBreakerStatuses() []CircuitBreakerStatus {
	return m.circuitBreakers.GetAll()
}

// ResetCircuitBreaker resets a circuit breaker for a specific stage.
func (m *Manager) ResetCircuitBreaker(stage string) {
	cb := m.circuitBreakers.Get(stage)
	cb.Reset()
}

// getStageConfig returns worker configuration for a stage based on enricher capabilities.
func (m *Manager) getStageConfig(stage string, caps appenrich.EnricherCapabilities) StageWorkerConfig {
	// Use enricher capabilities to determine configuration
	if caps.IsLocalEnricher() {
		config := DefaultLocalStageConfig(stage)
		return config
	}

	// Remote enricher - apply rate limiting from capabilities
	config := DefaultRemoteStageConfig(stage)
	if rateLimit := caps.GetRateLimitPerMinute(); rateLimit > 0 {
		// Convert requests per minute to requests per second
		config.RateLimit = float64(rateLimit) / 60.0
	}
	return config
}
