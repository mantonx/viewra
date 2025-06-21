package pluginmodule

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"gorm.io/gorm"
)

// FallbackManager handles graceful degradation when external plugins fail
type FallbackManager struct {
	logger     hclog.Logger
	db         *gorm.DB
	cache      map[string]*FallbackCacheEntry
	cacheMutex sync.RWMutex
	config     *FallbackConfig

	// Strategy handlers
	strategies map[string]FallbackStrategy
}

// FallbackConfig contains configuration for fallback behavior
type FallbackConfig struct {
	Enabled            bool                `json:"enabled"`
	DefaultStrategy    string              `json:"default_strategy"` // "cache", "alternative", "minimal", "fail"
	CacheDuration      time.Duration       `json:"cache_duration"`
	MaxCacheSize       int64               `json:"max_cache_size"`
	CleanupInterval    time.Duration       `json:"cleanup_interval"`
	RetryAfterDuration time.Duration       `json:"retry_after_duration"`
	AlternativePlugins map[string][]string `json:"alternative_plugins"` // plugin_id -> list of alternatives
	MinimalDataEnabled bool                `json:"minimal_data_enabled"`
}

// FallbackCacheEntry represents a cached response for fallback scenarios
type FallbackCacheEntry struct {
	Key          string                 `json:"key"`
	Data         map[string]interface{} `json:"data"`
	ExpiresAt    time.Time              `json:"expires_at"`
	CreatedAt    time.Time              `json:"created_at"`
	Source       string                 `json:"source"`     // Original plugin that created this data
	Confidence   float64                `json:"confidence"` // Confidence score of the data
	AccessCount  int64                  `json:"access_count"`
	LastAccessAt time.Time              `json:"last_access_at"`
}

// FallbackStrategy defines the interface for fallback strategies
type FallbackStrategy interface {
	Execute(ctx context.Context, request *FallbackRequest) (*FallbackResponse, error)
	GetName() string
}

// FallbackRequest contains the request information for fallback handling
type FallbackRequest struct {
	PluginID      string                 `json:"plugin_id"`
	Operation     string                 `json:"operation"`
	Parameters    map[string]interface{} `json:"parameters"`
	MediaFileID   string                 `json:"media_file_id,omitempty"`
	MediaType     string                 `json:"media_type,omitempty"`
	OriginalError error                  `json:"-"`
	RequestTime   time.Time              `json:"request_time"`
}

// FallbackResponse contains the response from fallback handling
type FallbackResponse struct {
	Success    bool                   `json:"success"`
	Data       map[string]interface{} `json:"data,omitempty"`
	Source     string                 `json:"source"`   // Source of the fallback data
	Strategy   string                 `json:"strategy"` // Strategy used
	FromCache  bool                   `json:"from_cache"`
	Confidence float64                `json:"confidence"`
	Message    string                 `json:"message,omitempty"`
	RetryAfter *time.Time             `json:"retry_after,omitempty"`
}

// NewFallbackManager creates a new fallback manager
func NewFallbackManager(logger hclog.Logger, db *gorm.DB, config *FallbackConfig) *FallbackManager {
	if config == nil {
		config = DefaultFallbackConfig()
	}

	fm := &FallbackManager{
		logger:     logger,
		db:         db,
		cache:      make(map[string]*FallbackCacheEntry),
		config:     config,
		strategies: make(map[string]FallbackStrategy),
	}

	// Register default strategies
	fm.RegisterStrategy(&CacheFallbackStrategy{manager: fm})
	fm.RegisterStrategy(&AlternativeFallbackStrategy{manager: fm})
	fm.RegisterStrategy(&MinimalDataFallbackStrategy{manager: fm})
	fm.RegisterStrategy(&FailFallbackStrategy{manager: fm})

	return fm
}

// RegisterStrategy registers a fallback strategy
func (fm *FallbackManager) RegisterStrategy(strategy FallbackStrategy) {
	fm.strategies[strategy.GetName()] = strategy
}

// HandleFailure handles a plugin failure and returns fallback data if available
func (fm *FallbackManager) HandleFailure(ctx context.Context, request *FallbackRequest) (*FallbackResponse, error) {
	if !fm.config.Enabled {
		return &FallbackResponse{
			Success:  false,
			Strategy: "disabled",
			Message:  "Fallback handling is disabled",
		}, request.OriginalError
	}

	fm.logger.Debug("handling plugin failure with fallback",
		"plugin_id", request.PluginID,
		"operation", request.Operation,
		"error", request.OriginalError)

	// Determine which strategy to use
	strategyName := fm.determineStrategy(request)
	strategy, exists := fm.strategies[strategyName]
	if !exists {
		return &FallbackResponse{
			Success:  false,
			Strategy: strategyName,
			Message:  fmt.Sprintf("Strategy '%s' not found", strategyName),
		}, fmt.Errorf("fallback strategy not found: %s", strategyName)
	}

	// Execute the strategy
	response, err := strategy.Execute(ctx, request)
	if err != nil {
		fm.logger.Error("fallback strategy failed",
			"strategy", strategyName,
			"plugin_id", request.PluginID,
			"error", err)
		return response, err
	}

	fm.logger.Info("fallback strategy executed successfully",
		"strategy", strategyName,
		"plugin_id", request.PluginID,
		"from_cache", response.FromCache,
		"confidence", response.Confidence)

	return response, nil
}

// determineStrategy determines which fallback strategy to use
func (fm *FallbackManager) determineStrategy(request *FallbackRequest) string {
	// Check if we have cached data for this request
	cacheKey := fm.generateCacheKey(request)
	if fm.hasCachedData(cacheKey) {
		return "cache"
	}

	// Check if alternative plugins are available
	if alternatives, exists := fm.config.AlternativePlugins[request.PluginID]; exists && len(alternatives) > 0 {
		return "alternative"
	}

	// Check if minimal data strategy is enabled
	if fm.config.MinimalDataEnabled {
		return "minimal"
	}

	// Default strategy or fail
	if fm.config.DefaultStrategy != "" {
		return fm.config.DefaultStrategy
	}

	return "fail"
}

// Cache management methods
func (fm *FallbackManager) StoreCacheEntry(key string, data map[string]interface{}, source string, confidence float64) {
	fm.cacheMutex.Lock()
	defer fm.cacheMutex.Unlock()

	entry := &FallbackCacheEntry{
		Key:          key,
		Data:         data,
		ExpiresAt:    time.Now().Add(fm.config.CacheDuration),
		CreatedAt:    time.Now(),
		Source:       source,
		Confidence:   confidence,
		AccessCount:  0,
		LastAccessAt: time.Now(),
	}

	fm.cache[key] = entry
	fm.logger.Debug("stored fallback cache entry", "key", key, "source", source, "confidence", confidence)
}

func (fm *FallbackManager) GetCacheEntry(key string) (*FallbackCacheEntry, bool) {
	fm.cacheMutex.RLock()
	defer fm.cacheMutex.RUnlock()

	entry, exists := fm.cache[key]
	if !exists {
		return nil, false
	}

	// Check if expired
	if time.Now().After(entry.ExpiresAt) {
		delete(fm.cache, key)
		return nil, false
	}

	// Update access statistics
	entry.AccessCount++
	entry.LastAccessAt = time.Now()

	return entry, true
}

func (fm *FallbackManager) hasCachedData(key string) bool {
	_, exists := fm.GetCacheEntry(key)
	return exists
}

func (fm *FallbackManager) generateCacheKey(request *FallbackRequest) string {
	// Create a unique key based on plugin, operation, and parameters
	paramsJson, _ := json.Marshal(request.Parameters)
	return fmt.Sprintf("%s:%s:%s", request.PluginID, request.Operation, string(paramsJson))
}

// StartCleanupRoutine starts a routine to clean up expired cache entries
func (fm *FallbackManager) StartCleanupRoutine(ctx context.Context) {
	ticker := time.NewTicker(fm.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fm.cleanupExpiredEntries()
		}
	}
}

func (fm *FallbackManager) cleanupExpiredEntries() {
	fm.cacheMutex.Lock()
	defer fm.cacheMutex.Unlock()

	now := time.Now()
	expiredKeys := make([]string, 0)

	for key, entry := range fm.cache {
		if now.After(entry.ExpiresAt) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, key := range expiredKeys {
		delete(fm.cache, key)
	}

	if len(expiredKeys) > 0 {
		fm.logger.Debug("cleaned up expired fallback cache entries", "count", len(expiredKeys))
	}
}

// Fallback strategy implementations

// CacheFallbackStrategy returns cached data if available
type CacheFallbackStrategy struct {
	manager *FallbackManager
}

func (s *CacheFallbackStrategy) GetName() string {
	return "cache"
}

func (s *CacheFallbackStrategy) Execute(ctx context.Context, request *FallbackRequest) (*FallbackResponse, error) {
	cacheKey := s.manager.generateCacheKey(request)
	entry, exists := s.manager.GetCacheEntry(cacheKey)

	if !exists {
		return &FallbackResponse{
			Success:  false,
			Strategy: "cache",
			Message:  "No cached data available",
		}, fmt.Errorf("no cached data for key: %s", cacheKey)
	}

	return &FallbackResponse{
		Success:    true,
		Data:       entry.Data,
		Source:     entry.Source,
		Strategy:   "cache",
		FromCache:  true,
		Confidence: entry.Confidence,
		Message:    fmt.Sprintf("Returned cached data from %s", entry.Source),
	}, nil
}

// AlternativeFallbackStrategy tries alternative plugins
type AlternativeFallbackStrategy struct {
	manager *FallbackManager
}

func (s *AlternativeFallbackStrategy) GetName() string {
	return "alternative"
}

func (s *AlternativeFallbackStrategy) Execute(ctx context.Context, request *FallbackRequest) (*FallbackResponse, error) {
	alternatives, exists := s.manager.config.AlternativePlugins[request.PluginID]
	if !exists || len(alternatives) == 0 {
		return &FallbackResponse{
			Success:  false,
			Strategy: "alternative",
			Message:  "No alternative plugins configured",
		}, fmt.Errorf("no alternatives for plugin: %s", request.PluginID)
	}

	// For now, return a graceful failure indicating alternatives could be tried
	// Full implementation would require integration with the external plugin manager

	return &FallbackResponse{
		Success:    false,
		Strategy:   "alternative",
		Message:    fmt.Sprintf("Alternative plugins available: %v", alternatives),
		RetryAfter: &[]time.Time{time.Now().Add(s.manager.config.RetryAfterDuration)}[0],
	}, fmt.Errorf("alternative plugin execution requires external plugin manager integration")
}

// MinimalDataFallbackStrategy returns minimal data based on available information
type MinimalDataFallbackStrategy struct {
	manager *FallbackManager
}

func (s *MinimalDataFallbackStrategy) GetName() string {
	return "minimal"
}

func (s *MinimalDataFallbackStrategy) Execute(ctx context.Context, request *FallbackRequest) (*FallbackResponse, error) {
	// Create minimal data based on what we know
	minimalData := map[string]interface{}{
		"source":     "minimal_fallback",
		"plugin_id":  request.PluginID,
		"media_type": request.MediaType,
		"timestamp":  time.Now(),
		"status":     "partial_data",
	}

	// Add any available basic information
	if request.MediaFileID != "" {
		minimalData["media_file_id"] = request.MediaFileID
	}

	return &FallbackResponse{
		Success:    true,
		Data:       minimalData,
		Source:     "fallback_system",
		Strategy:   "minimal",
		FromCache:  false,
		Confidence: 0.3, // Low confidence for minimal data
		Message:    "Returned minimal fallback data",
	}, nil
}

// FailFallbackStrategy fails gracefully
type FailFallbackStrategy struct {
	manager *FallbackManager
}

func (s *FailFallbackStrategy) GetName() string {
	return "fail"
}

func (s *FailFallbackStrategy) Execute(ctx context.Context, request *FallbackRequest) (*FallbackResponse, error) {
	return &FallbackResponse{
		Success:    false,
		Strategy:   "fail",
		Message:    "No fallback available, failing gracefully",
		RetryAfter: &[]time.Time{time.Now().Add(s.manager.config.RetryAfterDuration)}[0],
	}, request.OriginalError
}

// DefaultFallbackConfig returns default fallback configuration
func DefaultFallbackConfig() *FallbackConfig {
	return &FallbackConfig{
		Enabled:            true,
		DefaultStrategy:    "cache",
		CacheDuration:      24 * time.Hour,
		MaxCacheSize:       100 * 1024 * 1024, // 100MB
		CleanupInterval:    1 * time.Hour,
		RetryAfterDuration: 5 * time.Minute,
		AlternativePlugins: make(map[string][]string),
		MinimalDataEnabled: true,
	}
}
