package internal

import (
	"container/list"
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

// EmbeddingCacheConfig holds configuration for the query embedding cache.
type EmbeddingCacheConfig struct {
	MaxEntries int           // Maximum number of cached embeddings (default: 1000)
	TTL        time.Duration // Time-to-live for cache entries (default: 1 hour)
	Enabled    bool          // Whether caching is enabled (default: true)
}

// DefaultEmbeddingCacheConfig returns sensible defaults for the cache.
func DefaultEmbeddingCacheConfig() EmbeddingCacheConfig {
	return EmbeddingCacheConfig{
		MaxEntries: 1000,
		TTL:        time.Hour,
		Enabled:    true,
	}
}

// embeddingCacheEntry holds a cached embedding with metadata.
type embeddingCacheEntry struct {
	embedding []float32
	createdAt time.Time
	key       string // For LRU list reference
}

// EmbeddingCacheStats holds cache performance metrics.
type EmbeddingCacheStats struct {
	Hits      uint64
	Misses    uint64
	Evictions uint64
	Size      int
	MaxSize   int
	HitRate   float64
	Enabled   bool
}

// EmbeddingService handles embedding generation via the AI capability.
// Uses sdk.AIClient to invoke embedding methods on a provider plugin.
type EmbeddingService struct {
	ai               *sdk.AIClient
	targetDimensions int
	logger           *slog.Logger

	// LRU cache for query embeddings
	cacheMu     sync.RWMutex
	cache       map[string]*list.Element // key -> list element containing *embeddingCacheEntry
	cacheOrder  *list.List               // LRU order (front = most recent)
	cacheConfig EmbeddingCacheConfig

	// Cache metrics
	cacheHits   atomic.Uint64
	cacheMisses atomic.Uint64
	evictions   atomic.Uint64
}

// NewEmbeddingService creates a new embedding service.
func NewEmbeddingService(plugins *sdk.PluginsClient, logger *slog.Logger) *EmbeddingService {
	return &EmbeddingService{
		ai:               sdk.NewAIClient(plugins),
		targetDimensions: 768, // Standard dimension for most embedding models
		logger:           logger,
		cache:            make(map[string]*list.Element),
		cacheOrder:       list.New(),
		cacheConfig:      DefaultEmbeddingCacheConfig(),
	}
}

// NewEmbeddingServiceWithConfig creates an embedding service with custom cache config.
func NewEmbeddingServiceWithConfig(
	plugins *sdk.PluginsClient,
	logger *slog.Logger,
	config EmbeddingCacheConfig,
) *EmbeddingService {
	svc := NewEmbeddingService(plugins, logger)
	svc.cacheConfig = config
	return svc
}

// SetCacheConfig updates the cache configuration.
func (s *EmbeddingService) SetCacheConfig(config EmbeddingCacheConfig) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.cacheConfig = config

	// If cache is disabled, clear it
	if !config.Enabled {
		s.cache = make(map[string]*list.Element)
		s.cacheOrder.Init()
	}
}

// CacheStats returns current cache statistics.
func (s *EmbeddingService) CacheStats() EmbeddingCacheStats {
	s.cacheMu.RLock()
	size := len(s.cache)
	s.cacheMu.RUnlock()

	hits := s.cacheHits.Load()
	misses := s.cacheMisses.Load()
	total := hits + misses

	var hitRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	return EmbeddingCacheStats{
		Hits:      hits,
		Misses:    misses,
		Evictions: s.evictions.Load(),
		Size:      size,
		MaxSize:   s.cacheConfig.MaxEntries,
		HitRate:   hitRate,
		Enabled:   s.cacheConfig.Enabled,
	}
}

// ClearCache removes all cached embeddings.
func (s *EmbeddingService) ClearCache() {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.cache = make(map[string]*list.Element)
	s.cacheOrder.Init()
	s.logger.Info("embedding cache cleared")
}

// normalizeQuery creates a cache key from a query string.
// Normalizes to lowercase and collapses whitespace for better hit rate.
func normalizeQuery(text string) string {
	return strings.ToLower(strings.Join(strings.Fields(text), " "))
}

// cacheGet attempts to retrieve an embedding from the cache.
// Returns the embedding and true if found and not expired, nil and false otherwise.
func (s *EmbeddingService) cacheGet(key string) ([]float32, bool) {
	if !s.cacheConfig.Enabled {
		return nil, false
	}

	s.cacheMu.RLock()
	elem, exists := s.cache[key]
	s.cacheMu.RUnlock()

	if !exists {
		return nil, false
	}

	entry, ok := elem.Value.(*embeddingCacheEntry)
	if !ok {
		return nil, false
	}

	// Check TTL
	if time.Since(entry.createdAt) > s.cacheConfig.TTL {
		// Entry expired, remove it
		s.cacheMu.Lock()
		if elem, exists := s.cache[key]; exists {
			s.cacheOrder.Remove(elem)
			delete(s.cache, key)
		}
		s.cacheMu.Unlock()
		return nil, false
	}

	// Move to front (most recently used)
	s.cacheMu.Lock()
	s.cacheOrder.MoveToFront(elem)
	s.cacheMu.Unlock()

	// Return a copy to prevent mutation
	result := make([]float32, len(entry.embedding))
	copy(result, entry.embedding)
	return result, true
}

// cacheSet stores an embedding in the cache, evicting old entries if needed.
func (s *EmbeddingService) cacheSet(key string, embedding []float32) {
	if !s.cacheConfig.Enabled {
		return
	}

	// Make a copy to store
	stored := make([]float32, len(embedding))
	copy(stored, embedding)

	entry := &embeddingCacheEntry{
		embedding: stored,
		createdAt: time.Now(),
		key:       key,
	}

	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	// If key already exists, update it
	if elem, exists := s.cache[key]; exists {
		elem.Value = entry
		s.cacheOrder.MoveToFront(elem)
		return
	}

	// Evict oldest entries if at capacity
	for s.cacheOrder.Len() >= s.cacheConfig.MaxEntries {
		oldest := s.cacheOrder.Back()
		if oldest != nil {
			if oldEntry, ok := oldest.Value.(*embeddingCacheEntry); ok {
				delete(s.cache, oldEntry.key)
			}
			s.cacheOrder.Remove(oldest)
			s.evictions.Add(1)
		}
	}

	// Add new entry at front
	elem := s.cacheOrder.PushFront(entry)
	s.cache[key] = elem
}

// EmbedSingle generates an embedding for a single text.
// This method does NOT use the cache - use EmbedSingleCached for query embeddings.
func (s *EmbeddingService) EmbedSingle(ctx context.Context, text string) ([]float32, error) {
	if s.ai == nil {
		return nil, fmt.Errorf("AI client not available")
	}

	embedding, err := s.ai.GenerateEmbedding(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("generate embedding: %w", err)
	}

	if len(embedding) != s.targetDimensions {
		embedding = s.normalizeEmbedding(embedding, s.targetDimensions)
	}

	return embedding, nil
}

// EmbedSingleCached generates an embedding with LRU caching.
// Use this for search queries where the same query may be repeated.
// The text is normalized (lowercase, collapsed whitespace) for cache key matching.
func (s *EmbeddingService) EmbedSingleCached(ctx context.Context, text string) ([]float32, error) {
	key := normalizeQuery(text)

	// Check cache first
	if cached, found := s.cacheGet(key); found {
		s.cacheHits.Add(1)
		s.logger.Debug("embedding cache hit", "query", text)
		return cached, nil
	}

	s.cacheMisses.Add(1)

	// Generate embedding
	embedding, err := s.EmbedSingle(ctx, text)
	if err != nil {
		return nil, err
	}

	// Store in cache
	s.cacheSet(key, embedding)
	s.logger.Debug("embedding cached", "query", text, "cache_size", s.cacheOrder.Len())

	return embedding, nil
}

// EmbedBatch generates embeddings for multiple texts.
func (s *EmbeddingService) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if s.ai == nil {
		return nil, fmt.Errorf("AI client not available")
	}

	embeddings, err := s.ai.GenerateEmbeddingBatch(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("generate embeddings: %w", err)
	}

	results := make([][]float32, len(embeddings))
	for i, embedding := range embeddings {
		if len(embedding) != s.targetDimensions {
			embedding = s.normalizeEmbedding(embedding, s.targetDimensions)
		}
		results[i] = embedding
	}

	return results, nil
}

// normalizeEmbedding adjusts embedding to target dimensions.
// Uses truncation (if larger) or zero-padding (if smaller).
func (s *EmbeddingService) normalizeEmbedding(embedding []float32, targetDim int) []float32 {
	if len(embedding) == targetDim {
		return embedding
	}

	normalized := make([]float32, targetDim)

	if len(embedding) > targetDim {
		// Truncate and renormalize
		copy(normalized, embedding[:targetDim])
		s.l2Normalize(normalized)
	} else {
		// Zero-pad (embedding stays the same, zeros at the end)
		copy(normalized, embedding)
	}

	return normalized
}

// l2Normalize normalizes a vector to unit length.
func (s *EmbeddingService) l2Normalize(v []float32) {
	var sum float64
	for _, val := range v {
		sum += float64(val) * float64(val)
	}
	if sum == 0 {
		return
	}
	norm := float32(math.Sqrt(sum))
	for i := range v {
		v[i] /= norm
	}
}
