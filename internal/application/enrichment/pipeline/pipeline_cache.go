package pipeline

import (
	"context"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/domain/enrichment"
)

// PipelineCache provides in-memory caching for pipeline stage configuration.
// This eliminates per-job database queries for GetStageByName and GetNextStage,
// reducing ~200K queries for 100K items during enrichment.
type PipelineCache struct {
	repo enrichment.PipelineRepository
	ttl  time.Duration

	mu             sync.RWMutex
	stagesByName   map[stageKey]*enrichment.PipelineStage   // (mediaType, stageName) -> stage
	stagesByPos    map[posKey][]*enrichment.PipelineStage   // mediaType -> ordered stages
	lastRefresh    time.Time
	refreshPending bool
}

// stageKey is a composite key for stage lookup by name.
type stageKey struct {
	mediaType enrichment.MediaType
	stageName string
}

// posKey is a key for stage lookup by media type (for position-based queries).
type posKey struct {
	mediaType enrichment.MediaType
}

// NewPipelineCache creates a new pipeline cache with the given TTL.
// A TTL of 5 minutes is recommended for production.
func NewPipelineCache(repo enrichment.PipelineRepository, ttl time.Duration) *PipelineCache {
	return &PipelineCache{
		repo:         repo,
		ttl:          ttl,
		stagesByName: make(map[stageKey]*enrichment.PipelineStage),
		stagesByPos:  make(map[posKey][]*enrichment.PipelineStage),
	}
}

// GetStageByName returns a stage by media type and name, using cache when valid.
func (c *PipelineCache) GetStageByName(ctx context.Context, mediaType enrichment.MediaType, stageName string) (*enrichment.PipelineStage, error) {
	key := stageKey{mediaType: mediaType, stageName: stageName}

	// Check cache first
	c.mu.RLock()
	if c.isValid() {
		if stage, ok := c.stagesByName[key]; ok {
			c.mu.RUnlock()
			return stage, nil
		}
		// Key not in cache means stage doesn't exist
		c.mu.RUnlock()
		return nil, nil
	}
	c.mu.RUnlock()

	// Cache miss or expired - refresh
	if err := c.refresh(ctx); err != nil {
		// On refresh error, fall back to direct query
		return c.repo.GetStageByName(ctx, mediaType, stageName)
	}

	// Retry from cache
	c.mu.RLock()
	stage := c.stagesByName[key]
	c.mu.RUnlock()
	return stage, nil
}

// GetNextStage returns the next enabled stage after the given position, using cache when valid.
func (c *PipelineCache) GetNextStage(ctx context.Context, mediaType enrichment.MediaType, currentPosition int) (*enrichment.PipelineStage, error) {
	key := posKey{mediaType: mediaType}

	// Check cache first
	c.mu.RLock()
	if c.isValid() {
		stages, ok := c.stagesByPos[key]
		if ok {
			for _, stage := range stages {
				if stage.Position > currentPosition && stage.Enabled {
					c.mu.RUnlock()
					return stage, nil
				}
			}
			// No next stage found
			c.mu.RUnlock()
			return nil, nil
		}
		// Media type not in cache - might be new
		c.mu.RUnlock()
	} else {
		c.mu.RUnlock()
	}

	// Cache miss or expired - refresh
	if err := c.refresh(ctx); err != nil {
		// On refresh error, fall back to direct query
		return c.repo.GetNextStage(ctx, mediaType, currentPosition)
	}

	// Retry from cache
	c.mu.RLock()
	defer c.mu.RUnlock()
	stages, ok := c.stagesByPos[key]
	if !ok {
		return nil, nil
	}
	for _, stage := range stages {
		if stage.Position > currentPosition && stage.Enabled {
			return stage, nil
		}
	}
	return nil, nil
}

// GetFirstStage returns the first enabled stage for a media type, using cache when valid.
func (c *PipelineCache) GetFirstStage(ctx context.Context, mediaType enrichment.MediaType) (*enrichment.PipelineStage, error) {
	key := posKey{mediaType: mediaType}

	// Check cache first
	c.mu.RLock()
	if c.isValid() {
		stages, ok := c.stagesByPos[key]
		if ok {
			for _, stage := range stages {
				if stage.Enabled {
					c.mu.RUnlock()
					return stage, nil
				}
			}
			// No enabled stages
			c.mu.RUnlock()
			return nil, nil
		}
		c.mu.RUnlock()
	} else {
		c.mu.RUnlock()
	}

	// Cache miss or expired - refresh
	if err := c.refresh(ctx); err != nil {
		return c.repo.GetFirstStage(ctx, mediaType)
	}

	// Retry from cache
	c.mu.RLock()
	defer c.mu.RUnlock()
	stages, ok := c.stagesByPos[key]
	if !ok {
		return nil, nil
	}
	for _, stage := range stages {
		if stage.Enabled {
			return stage, nil
		}
	}
	return nil, nil
}

// Invalidate forces a cache refresh on the next access.
// Call this when pipeline configuration is modified via API.
func (c *PipelineCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastRefresh = time.Time{} // Zero time = invalid
}

// isValid returns true if the cache is still valid.
// Must be called with at least a read lock held.
func (c *PipelineCache) isValid() bool {
	if c.lastRefresh.IsZero() {
		return false
	}
	return time.Since(c.lastRefresh) < c.ttl
}

// refresh reloads all pipeline stages from the database.
func (c *PipelineCache) refresh(ctx context.Context) error {
	c.mu.Lock()
	// Double-check after acquiring write lock
	if c.isValid() {
		c.mu.Unlock()
		return nil
	}
	// Prevent concurrent refreshes
	if c.refreshPending {
		c.mu.Unlock()
		// Wait a bit and let the other goroutine finish
		time.Sleep(50 * time.Millisecond)
		return nil
	}
	c.refreshPending = true
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.refreshPending = false
		c.mu.Unlock()
	}()

	// Load all stages for all media types
	mediaTypes := []enrichment.MediaType{
		enrichment.MediaTypeMovie,
		enrichment.MediaTypeTV,
		enrichment.MediaTypeTVShow,
		enrichment.MediaTypeTVSeason,
		enrichment.MediaTypeMusic,
		enrichment.MediaTypeMusicAlbum,
		enrichment.MediaTypeMusicArtist,
	}

	newByName := make(map[stageKey]*enrichment.PipelineStage)
	newByPos := make(map[posKey][]*enrichment.PipelineStage)

	for _, mt := range mediaTypes {
		stages, err := c.repo.GetAllStages(ctx, mt)
		if err != nil {
			return err
		}

		key := posKey{mediaType: mt}
		newByPos[key] = stages

		for _, stage := range stages {
			nameKey := stageKey{mediaType: mt, stageName: stage.StageName}
			newByName[nameKey] = stage
		}
	}

	// Atomic swap
	c.mu.Lock()
	c.stagesByName = newByName
	c.stagesByPos = newByPos
	c.lastRefresh = time.Now()
	c.mu.Unlock()

	return nil
}
