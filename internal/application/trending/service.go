package trending

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/infrastructure/plugins/registry"
	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

// TrendingDataFetcher fetches trending data from a plugin.
type TrendingDataFetcher interface {
	// FetchTrending fetches trending data from a plugin.
	FetchTrending(ctx context.Context, pluginID string, req *sdk.TrendingRequest) (*sdk.TrendingResponse, error)
}

// MediaMatcher matches external IDs to local library items.
type MediaMatcher interface {
	// FindByExternalID finds a local media item by external source and ID.
	// Returns the local ID or nil if not found.
	FindByExternalID(ctx context.Context, source, externalID, mediaType string) (*int64, error)
}

// Result is the processed trending data with local library matches.
type Result struct {
	// Items contains trending items matched to the local library.
	Items []*sdk.TrendingItem `json:"items"`

	// Source is the provider that served the original data.
	Source string `json:"source"`

	// Window is the time window used.
	Window string `json:"window"`

	// TotalMatched is the number of items matched to local library.
	TotalMatched int `json:"total_matched"`

	// TotalTrending is the total trending items from the provider.
	TotalTrending int `json:"total_trending"`
}

// cacheEntry represents a cached trending response.
type cacheEntry struct {
	response  *sdk.TrendingResponse
	fetchedAt time.Time
}

// Service aggregates trending data and matches against the local library.
type Service struct {
	providerRegistry *registry.TrendingProviderRegistry
	trendingFetcher  TrendingDataFetcher
	mediaMatcher     MediaMatcher
	logger           *slog.Logger

	// Simple in-memory cache
	mu    sync.RWMutex
	cache map[string]*cacheEntry
}

// NewService creates a new trending service.
func NewService(
	providerRegistry *registry.TrendingProviderRegistry,
	trendingFetcher TrendingDataFetcher,
	mediaMatcher MediaMatcher,
	logger *slog.Logger,
) *Service {
	return &Service{
		providerRegistry: providerRegistry,
		trendingFetcher:  trendingFetcher,
		mediaMatcher:     mediaMatcher,
		logger:           logger,
		cache:            make(map[string]*cacheEntry),
	}
}

// GetTrending returns trending items matched to the local library.
func (s *Service) GetTrending(ctx context.Context, mediaType string, limit int) (*Result, error) {
	// Get the default provider
	provider := s.providerRegistry.GetDefault()
	if provider == nil {
		return nil, fmt.Errorf("no trending provider available")
	}

	// Normalize media type
	if mediaType == "" {
		mediaType = "all"
	}

	// Normalize limit
	if limit <= 0 {
		limit = 20
	}

	// Check cache
	cacheKey := fmt.Sprintf("%s:%s:%s", provider.PluginID, provider.Info.ID, mediaType)
	if cached := s.getFromCache(cacheKey); cached != nil {
		return s.matchToLibrary(ctx, cached, limit)
	}

	// Fetch from provider
	trending, err := s.trendingFetcher.FetchTrending(ctx, provider.PluginID, &sdk.TrendingRequest{
		MediaType: mediaType,
		Window:    "week",
		Limit:     limit * 3, // Get extra to account for non-matches
	})
	if err != nil {
		return nil, fmt.Errorf("fetch trending: %w", err)
	}

	// Cache for 1 hour
	s.setCache(cacheKey, trending, time.Hour)

	// Match against local library
	return s.matchToLibrary(ctx, trending, limit)
}

// GetTrendingByWindow returns trending with a specific time window.
func (s *Service) GetTrendingByWindow(ctx context.Context, mediaType, window string, limit int) (*Result, error) {
	// Get the default provider
	provider := s.providerRegistry.GetDefault()
	if provider == nil {
		return nil, fmt.Errorf("no trending provider available")
	}

	// Validate window
	validWindow := false
	for _, w := range provider.Info.Windows {
		if w == window {
			validWindow = true
			break
		}
	}
	if !validWindow {
		window = "week" // Default to week
	}

	// Normalize
	if mediaType == "" {
		mediaType = "all"
	}
	if limit <= 0 {
		limit = 20
	}

	// Check cache
	cacheKey := fmt.Sprintf("%s:%s:%s:%s", provider.PluginID, provider.Info.ID, mediaType, window)
	if cached := s.getFromCache(cacheKey); cached != nil {
		return s.matchToLibrary(ctx, cached, limit)
	}

	// Fetch from provider
	trending, err := s.trendingFetcher.FetchTrending(ctx, provider.PluginID, &sdk.TrendingRequest{
		MediaType: mediaType,
		Window:    window,
		Limit:     limit * 3,
	})
	if err != nil {
		return nil, fmt.Errorf("fetch trending: %w", err)
	}

	// Cache duration based on window
	cacheDuration := time.Hour
	if window == "day" {
		cacheDuration = 30 * time.Minute
	}
	s.setCache(cacheKey, trending, cacheDuration)

	return s.matchToLibrary(ctx, trending, limit)
}

// GetProviders returns all available trending providers.
func (s *Service) GetProviders() []*registry.RegisteredTrendingProvider {
	return s.providerRegistry.GetAll()
}

// HasProvider returns true if at least one trending provider is available.
func (s *Service) HasProvider() bool {
	return s.providerRegistry.HasEnabled()
}

// matchToLibrary matches trending items to the local library.
func (s *Service) matchToLibrary(ctx context.Context, trending *sdk.TrendingResponse, limit int) (*Result, error) {
	if s.mediaMatcher == nil {
		// Return unmatched items
		return &Result{
			Items:         toPointers(trending.Items[:min(len(trending.Items), limit)]),
			Source:        trending.Source,
			Window:        trending.Window,
			TotalMatched:  0,
			TotalTrending: len(trending.Items),
		}, nil
	}

	matched := make([]*sdk.TrendingItem, 0)

	for _, item := range trending.Items {
		// Parse external ID (e.g., "tmdb:12345")
		parts := strings.SplitN(item.ExternalID, ":", 2)
		if len(parts) != 2 {
			continue
		}
		source, externalID := parts[0], parts[1]

		// Look up in local library
		localID, err := s.mediaMatcher.FindByExternalID(ctx, source, externalID, item.MediaType)
		if err != nil {
			s.logger.Debug("error matching trending item",
				"external_id", item.ExternalID,
				"error", err,
			)
			continue
		}
		if localID == nil {
			continue // Not in library
		}

		// Create a copy with local ID
		matchedItem := item
		matchedItem.LocalID = localID
		matchedItem.LocalMatched = true
		matched = append(matched, &matchedItem)

		if len(matched) >= limit {
			break
		}
	}

	return &Result{
		Items:         matched,
		Source:        trending.Source,
		Window:        trending.Window,
		TotalMatched:  len(matched),
		TotalTrending: len(trending.Items),
	}, nil
}

// getFromCache returns a cached response if still valid.
func (s *Service) getFromCache(key string) *sdk.TrendingResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.cache[key]
	if !ok {
		return nil
	}
	return entry.response
}

// setCache stores a response in the cache.
func (s *Service) setCache(key string, response *sdk.TrendingResponse, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache[key] = &cacheEntry{
		response:  response,
		fetchedAt: time.Now(),
	}

	// Schedule cleanup
	go func() {
		time.Sleep(ttl)
		s.mu.Lock()
		delete(s.cache, key)
		s.mu.Unlock()
	}()
}

// ClearCache clears the trending cache.
func (s *Service) ClearCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = make(map[string]*cacheEntry)
}

// toPointers converts a slice to a slice of pointers.
func toPointers(items []sdk.TrendingItem) []*sdk.TrendingItem {
	result := make([]*sdk.TrendingItem, len(items))
	for i := range items {
		result[i] = &items[i]
	}
	return result
}
