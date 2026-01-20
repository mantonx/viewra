package internal

import (
	"container/list"
	"context"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

const queryRewriteSystemPrompt = `You are a search query optimizer for a media library. Your job is to rewrite user queries to better match content in a movie/TV database.

IMPORTANT RULES:
1. Detect user INTENT, not just literal words
2. If user says they're feeling sad/down/depressed and want cheering up, they want HAPPY/UPLIFTING content
3. If user is stressed/anxious, they likely want RELAXING/CALMING content
4. If user is bored, they want EXCITING/ENGAGING content
5. Expand abbreviations and colloquialisms
6. Add relevant genre/mood keywords that match the intent
7. Keep the output concise (under 20 words)
8. Output ONLY the rewritten query, nothing else

Examples:
- "feeling sad need cheering up" → "uplifting heartwarming feel-good comedy happy ending"
- "stressed out need to relax" → "relaxing calm peaceful slow-paced soothing"
- "something scary" → "horror scary terrifying thriller suspense"
- "80s action" → "1980s action movie eighties explosive"
- "like Die Hard" → "action thriller hostage hero one-man-army skyscraper"
- "Korean thriller" → "Korean South Korea thriller suspense mystery Language: Korean"`

// rewriteCacheEntry holds a cached query rewrite with metadata.
type rewriteCacheEntry struct {
	original  string
	rewritten string
	createdAt time.Time
}

// QueryRewriter uses an LLM to rewrite search queries for better semantic matching.
// Includes LRU caching to reduce API calls for repeated queries.
type QueryRewriter struct {
	ai      *sdk.AIClient
	logger  *slog.Logger
	enabled bool

	// LRU cache for rewritten queries
	cacheMu    sync.RWMutex
	cache      map[string]*list.Element // normalized query -> list element
	cacheOrder *list.List               // LRU order (front = most recent)
	maxEntries int
	cacheTTL   time.Duration

	// Cache metrics
	cacheHits   atomic.Uint64
	cacheMisses atomic.Uint64
}

// NewQueryRewriter creates a new query rewriter with LRU caching.
func NewQueryRewriter(plugins *sdk.PluginsClient, logger *slog.Logger) *QueryRewriter {
	ai := sdk.NewAIClient(plugins)
	return &QueryRewriter{
		ai:         ai,
		logger:     logger,
		enabled:    ai != nil,
		cache:      make(map[string]*list.Element),
		cacheOrder: list.New(),
		maxEntries: 500,              // Cache up to 500 query rewrites
		cacheTTL:   2 * time.Hour,    // Rewrites valid for 2 hours
	}
}

// Rewrite rewrites a query using the LLM to better match user intent.
// Returns the original query if rewriting fails or is disabled.
// Results are cached to reduce API calls for repeated queries.
func (r *QueryRewriter) Rewrite(ctx context.Context, query string) string {
	if !r.enabled || r.ai == nil {
		return query
	}

	// Skip very short queries or queries that look like direct searches
	if len(query) < 10 || !needsRewriting(query) {
		return query
	}

	// Normalize query for cache key
	cacheKey := normalizeQueryForCache(query)

	// Check cache first
	if cached, found := r.cacheGet(cacheKey); found {
		r.cacheHits.Add(1)
		r.logger.Debug("query rewrite cache hit", "query", query, "rewritten", cached)
		return cached
	}

	r.cacheMisses.Add(1)

	// Use AIClient for chat
	resp, err := r.ai.Chat(ctx, sdk.ChatRequest{
		Messages: []sdk.ChatMessage{
			{Role: "system", Content: queryRewriteSystemPrompt},
			{Role: "user", Content: query},
		},
		Temperature: 0.3,
		MaxTokens:   50,
	})
	if err != nil {
		r.logger.Debug("query rewrite failed", "error", err)
		return query
	}

	rewritten := strings.TrimSpace(resp.Content)
	if rewritten == "" || len(rewritten) > 200 {
		return query
	}

	// Cache the rewrite
	r.cacheSet(cacheKey, query, rewritten)

	r.logger.Debug("rewrote query",
		"original", query,
		"rewritten", rewritten,
		"cache_size", r.cacheOrder.Len(),
	)

	return rewritten
}

// normalizeQueryForCache creates a normalized cache key from a query.
func normalizeQueryForCache(query string) string {
	return strings.ToLower(strings.Join(strings.Fields(query), " "))
}

// cacheGet retrieves a cached rewrite if it exists and hasn't expired.
func (r *QueryRewriter) cacheGet(key string) (string, bool) {
	r.cacheMu.RLock()
	elem, exists := r.cache[key]
	r.cacheMu.RUnlock()

	if !exists {
		return "", false
	}

	entry, ok := elem.Value.(*rewriteCacheEntry)
	if !ok {
		return "", false
	}

	// Check TTL
	if time.Since(entry.createdAt) > r.cacheTTL {
		// Entry expired, remove it
		r.cacheMu.Lock()
		if elem, exists := r.cache[key]; exists {
			r.cacheOrder.Remove(elem)
			delete(r.cache, key)
		}
		r.cacheMu.Unlock()
		return "", false
	}

	// Move to front (most recently used)
	r.cacheMu.Lock()
	r.cacheOrder.MoveToFront(elem)
	r.cacheMu.Unlock()

	return entry.rewritten, true
}

// cacheSet stores a query rewrite in the cache.
func (r *QueryRewriter) cacheSet(key, original, rewritten string) {
	entry := &rewriteCacheEntry{
		original:  original,
		rewritten: rewritten,
		createdAt: time.Now(),
	}

	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()

	// If key already exists, update it
	if elem, exists := r.cache[key]; exists {
		elem.Value = entry
		r.cacheOrder.MoveToFront(elem)
		return
	}

	// Evict oldest entries if at capacity
	for r.cacheOrder.Len() >= r.maxEntries {
		oldest := r.cacheOrder.Back()
		if oldest != nil {
			if oldEntry, ok := oldest.Value.(*rewriteCacheEntry); ok {
				delete(r.cache, normalizeQueryForCache(oldEntry.original))
			}
			r.cacheOrder.Remove(oldest)
		}
	}

	// Add new entry at front
	elem := r.cacheOrder.PushFront(entry)
	r.cache[key] = elem
}

// CacheStats returns cache statistics for monitoring.
func (r *QueryRewriter) CacheStats() map[string]any {
	r.cacheMu.RLock()
	size := len(r.cache)
	r.cacheMu.RUnlock()

	hits := r.cacheHits.Load()
	misses := r.cacheMisses.Load()
	total := hits + misses

	var hitRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	return map[string]any{
		"hits":       hits,
		"misses":     misses,
		"size":       size,
		"max_size":   r.maxEntries,
		"hit_rate":   hitRate,
		"ttl_hours":  r.cacheTTL.Hours(),
	}
}

// ClearCache removes all cached rewrites.
func (r *QueryRewriter) ClearCache() {
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()
	r.cache = make(map[string]*list.Element)
	r.cacheOrder.Init()
	r.logger.Info("query rewrite cache cleared")
}

// needsRewriting returns true if the query might benefit from rewriting.
func needsRewriting(query string) bool {
	q := strings.ToLower(query)

	// Queries expressing emotional state or needs
	emotionalIndicators := []string{
		"feeling", "i'm", "im ", "i am", "need", "want",
		"something", "mood", "like", "similar",
		"cheer", "relax", "stressed", "bored", "sad",
		"happy", "scared", "excited",
	}

	for _, indicator := range emotionalIndicators {
		if strings.Contains(q, indicator) {
			return true
		}
	}

	return false
}
