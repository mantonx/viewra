package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	appenrich "github.com/mantonx/viewra/internal/application/enrichment"
	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/infrastructure/events"
)

// Manager orchestrates the enrichment pipeline.
// It manages job queuing and coordinates worker pools for each stage.
type Manager struct {
	deps        *Deps
	typedRepos  *TypedMediaRepos
	workerPools map[string]*WorkerPool
	enrichers   map[string]appenrich.Enricher
	mu          sync.RWMutex
	running     bool
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// NewManager creates a new pipeline manager.
func NewManager(deps *Deps, typedRepos *TypedMediaRepos) *Manager {
	return &Manager{
		deps:        deps,
		typedRepos:  typedRepos,
		workerPools: make(map[string]*WorkerPool),
		enrichers:   make(map[string]appenrich.Enricher),
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

	m.ctx, m.cancel = context.WithCancel(ctx)
	m.running = true

	// Start worker pools for each registered enricher
	for stage, enricher := range m.enrichers {
		config := m.getStageConfig(stage, enricher.Capabilities())
		pool := NewWorkerPool(m.deps, enricher, m.typedRepos, config)
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

// EnqueueFirstStage enqueues a media item for the first stage of its pipeline.
// Called after media is discovered/saved during scanning.
func (m *Manager) EnqueueFirstStage(ctx context.Context, mediaID int64, mediaType enrichment.MediaType) error {
	// Get the first enabled stage for this media type
	firstStage, err := m.deps.PipelineRepo.GetFirstStage(ctx, mediaType)
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
		MediaType:   mediaType,
		Stage:       firstStage.StageName,
		Priority:    0, // Default priority
		Status:      enrichment.JobStatusPending,
		MaxAttempts: 3,
	}

	if _, err := m.deps.QueueRepo.Enqueue(ctx, job); err != nil {
		return fmt.Errorf("enqueue first stage: %w", err)
	}

	// Publish event
	m.deps.EventBus.Publish(events.NewEvent(events.EventEnrichmentQueued, "pipeline").
		WithMediaID(mediaID).
		WithStage(firstStage.StageName).
		Build())

	return nil
}

// EnqueueNextStage enqueues a media item for the next pipeline stage.
// Called after a stage completes successfully.
func (m *Manager) EnqueueNextStage(ctx context.Context, mediaID int64, mediaType enrichment.MediaType, currentPosition int) error {
	// Get the next enabled stage after the current position
	nextStage, err := m.deps.PipelineRepo.GetNextStage(ctx, mediaType, currentPosition)
	if err != nil {
		return fmt.Errorf("get next stage: %w", err)
	}

	if nextStage == nil {
		// No more stages - enrichment complete
		m.deps.EventBus.Publish(events.NewEvent(events.EventEnrichmentComplete, "pipeline").
			WithMediaID(mediaID).
			Build())
		return nil
	}

	// Enqueue for next stage
	job := &enrichment.QueueJob{
		MediaID:     mediaID,
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
		WithStage(nextStage.StageName).
		Build())

	return nil
}

// EnqueueStage enqueues a media item for a specific stage.
// Useful for manual re-processing or custom stage triggers.
func (m *Manager) EnqueueStage(ctx context.Context, mediaID int64, mediaType enrichment.MediaType, stage string, priority int) error {
	job := &enrichment.QueueJob{
		MediaID:     mediaID,
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
