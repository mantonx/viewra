package cache

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mantonx/viewra/plugins/tmdb_enricher_v2/internal/config"
	"github.com/mantonx/viewra/plugins/tmdb_enricher_v2/internal/models"
	"github.com/mantonx/viewra/plugins/tmdb_enricher_v2/internal/types"
	plugins "github.com/mantonx/viewra/sdk"
	"gorm.io/gorm"
)

// CacheManager handles caching operations for TMDb API responses
type CacheManager struct {
	db     *gorm.DB
	config *config.Config
	logger plugins.Logger
}

// NewCacheManager creates a new cache manager
func NewCacheManager(db *gorm.DB, cfg *config.Config, logger plugins.Logger) *CacheManager {
	return &CacheManager{
		db:     db,
		config: cfg,
		logger: logger,
	}
}

// Get retrieves a cached response
func (cm *CacheManager) Get(queryType, query string) (interface{}, bool, error) {
	queryHash := cm.generateHash(query)

	var cache models.TMDbCache
	err := cm.db.Where("query_type = ? AND query_hash = ?", queryType, queryHash).First(&cache).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, false, nil // Cache miss
		}
		return nil, false, fmt.Errorf("failed to query cache: %w", err)
	}

	// Check if expired
	if cache.IsExpired() {
		cm.logger.Debug("cache entry expired", "query_type", queryType, "hash", queryHash[:8])
		// Clean up expired entry
		go cm.deleteExpiredEntry(cache.ID)
		return nil, false, nil
	}

	// Parse the cached response based on query type
	var result interface{}
	switch queryType {
	case "search":
		var searchResults []types.Result
		if err := json.Unmarshal([]byte(cache.Response), &searchResults); err != nil {
			cm.logger.Warn("failed to unmarshal cached search results", "error", err)
			return nil, false, nil
		}
		result = searchResults

	case "movie_details":
		var movieDetails types.MovieDetails
		if err := json.Unmarshal([]byte(cache.Response), &movieDetails); err != nil {
			cm.logger.Warn("failed to unmarshal cached movie details", "error", err)
			return nil, false, nil
		}
		result = movieDetails

	case "tv_details":
		var tvDetails types.TVSeriesDetails
		if err := json.Unmarshal([]byte(cache.Response), &tvDetails); err != nil {
			cm.logger.Warn("failed to unmarshal cached TV details", "error", err)
			return nil, false, nil
		}
		result = tvDetails

	case "season_details":
		var seasonDetails types.TVSeasonDetails
		if err := json.Unmarshal([]byte(cache.Response), &seasonDetails); err != nil {
			cm.logger.Warn("failed to unmarshal cached season details", "error", err)
			return nil, false, nil
		}
		result = seasonDetails

	case "episode_details":
		var episodeDetails types.TVEpisodeDetails
		if err := json.Unmarshal([]byte(cache.Response), &episodeDetails); err != nil {
			cm.logger.Warn("failed to unmarshal cached episode details", "error", err)
			return nil, false, nil
		}
		result = episodeDetails

	case "movie_images":
		var images types.ImagesResponse
		if err := json.Unmarshal([]byte(cache.Response), &images); err != nil {
			cm.logger.Warn("failed to unmarshal cached movie images", "error", err)
			return nil, false, nil
		}
		result = images

	case "tv_images":
		var images types.ImagesResponse
		if err := json.Unmarshal([]byte(cache.Response), &images); err != nil {
			cm.logger.Warn("failed to unmarshal cached TV images", "error", err)
			return nil, false, nil
		}
		result = images

	default:
		cm.logger.Warn("unknown cache query type", "query_type", queryType)
		return nil, false, nil
	}

	cm.logger.Debug("cache hit", "query_type", queryType, "hash", queryHash[:8])
	return result, true, nil
}

// Set stores a response in the cache
func (cm *CacheManager) Set(queryType, query string, response interface{}) error {
	queryHash := cm.generateHash(query)

	// Marshal the response to JSON
	responseBytes, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	// Calculate expiry time
	expiresAt := time.Now().Add(cm.config.Cache.GetCacheDuration())

	// Create or update cache entry
	cache := models.TMDbCache{
		QueryHash: queryHash,
		QueryType: queryType,
		Response:  string(responseBytes),
		ExpiresAt: expiresAt,
	}

	// Use UPSERT to handle duplicates
	err = cm.db.Where("query_type = ? AND query_hash = ?", queryType, queryHash).
		Assign(&cache).
		FirstOrCreate(&cache).Error

	if err != nil {
		return fmt.Errorf("failed to save cache entry: %w", err)
	}

	cm.logger.Debug("cache entry saved",
		"query_type", queryType,
		"hash", queryHash[:8],
		"expires_at", expiresAt.Format(time.RFC3339))

	return nil
}

// GetSearchResults retrieves cached search results
func (cm *CacheManager) GetSearchResults(query string) ([]types.Result, bool, error) {
	result, found, err := cm.Get("search", query)
	if err != nil || !found {
		return nil, found, err
	}

	if searchResults, ok := result.([]types.Result); ok {
		return searchResults, true, nil
	}

	return nil, false, fmt.Errorf("cached result is not search results")
}

// SetSearchResults stores search results in cache
func (cm *CacheManager) SetSearchResults(query string, results []types.Result) error {
	return cm.Set("search", query, results)
}

// GetMovieDetails retrieves cached movie details
func (cm *CacheManager) GetMovieDetails(tmdbID int) (*types.MovieDetails, bool, error) {
	query := fmt.Sprintf("movie:%d", tmdbID)
	result, found, err := cm.Get("movie_details", query)
	if err != nil || !found {
		return nil, found, err
	}

	if movieDetails, ok := result.(types.MovieDetails); ok {
		return &movieDetails, true, nil
	}

	return nil, false, fmt.Errorf("cached result is not movie details")
}

// SetMovieDetails stores movie details in cache
func (cm *CacheManager) SetMovieDetails(tmdbID int, details *types.MovieDetails) error {
	query := fmt.Sprintf("movie:%d", tmdbID)
	return cm.Set("movie_details", query, details)
}

// GetTVDetails retrieves cached TV series details
func (cm *CacheManager) GetTVDetails(tmdbID int) (*types.TVSeriesDetails, bool, error) {
	query := fmt.Sprintf("tv:%d", tmdbID)
	result, found, err := cm.Get("tv_details", query)
	if err != nil || !found {
		return nil, found, err
	}

	if tvDetails, ok := result.(types.TVSeriesDetails); ok {
		return &tvDetails, true, nil
	}

	return nil, false, fmt.Errorf("cached result is not TV details")
}

// SetTVDetails stores TV series details in cache
func (cm *CacheManager) SetTVDetails(tmdbID int, details *types.TVSeriesDetails) error {
	query := fmt.Sprintf("tv:%d", tmdbID)
	return cm.Set("tv_details", query, details)
}

// GetSeasonDetails retrieves cached season details
func (cm *CacheManager) GetSeasonDetails(tmdbID, seasonNumber int) (*types.TVSeasonDetails, bool, error) {
	query := fmt.Sprintf("season:%d:%d", tmdbID, seasonNumber)
	result, found, err := cm.Get("season_details", query)
	if err != nil || !found {
		return nil, found, err
	}

	if seasonDetails, ok := result.(types.TVSeasonDetails); ok {
		return &seasonDetails, true, nil
	}

	return nil, false, fmt.Errorf("cached result is not season details")
}

// SetSeasonDetails stores season details in cache
func (cm *CacheManager) SetSeasonDetails(tmdbID, seasonNumber int, details *types.TVSeasonDetails) error {
	query := fmt.Sprintf("season:%d:%d", tmdbID, seasonNumber)
	return cm.Set("season_details", query, details)
}

// GetEpisodeDetails retrieves cached episode details
func (cm *CacheManager) GetEpisodeDetails(tmdbID, seasonNumber, episodeNumber int) (*types.TVEpisodeDetails, bool, error) {
	query := fmt.Sprintf("episode:%d:%d:%d", tmdbID, seasonNumber, episodeNumber)
	result, found, err := cm.Get("episode_details", query)
	if err != nil || !found {
		return nil, found, err
	}

	if episodeDetails, ok := result.(types.TVEpisodeDetails); ok {
		return &episodeDetails, true, nil
	}

	return nil, false, fmt.Errorf("cached result is not episode details")
}

// SetEpisodeDetails stores episode details in cache
func (cm *CacheManager) SetEpisodeDetails(tmdbID, seasonNumber, episodeNumber int, details *types.TVEpisodeDetails) error {
	query := fmt.Sprintf("episode:%d:%d:%d", tmdbID, seasonNumber, episodeNumber)
	return cm.Set("episode_details", query, details)
}

// GetImages retrieves cached images
func (cm *CacheManager) GetImages(mediaType string, tmdbID int) (*types.ImagesResponse, bool, error) {
	query := fmt.Sprintf("%s_images:%d", mediaType, tmdbID)
	result, found, err := cm.Get(fmt.Sprintf("%s_images", mediaType), query)
	if err != nil || !found {
		return nil, found, err
	}

	if images, ok := result.(types.ImagesResponse); ok {
		return &images, true, nil
	}

	return nil, false, fmt.Errorf("cached result is not images")
}

// SetImages stores images in cache
func (cm *CacheManager) SetImages(mediaType string, tmdbID int, images *types.ImagesResponse) error {
	query := fmt.Sprintf("%s_images:%d", mediaType, tmdbID)
	return cm.Set(fmt.Sprintf("%s_images", mediaType), query, images)
}

// StartCleanupRoutine starts a background routine to clean up expired cache entries
func (cm *CacheManager) StartCleanupRoutine(ctx context.Context) {
	ticker := time.NewTicker(cm.config.Cache.GetCleanupInterval())
	defer ticker.Stop()

	cm.logger.Info("cache cleanup routine started",
		"interval", cm.config.Cache.GetCleanupInterval().String())

	for {
		select {
		case <-ctx.Done():
			cm.logger.Info("cache cleanup routine stopped")
			return
		case <-ticker.C:
			cm.cleanupExpiredEntries()
		}
	}
}

// PrepareForScan prepares the cache for a scanning operation
func (cm *CacheManager) PrepareForScan() {
	cm.logger.Debug("preparing cache for scan")
	// Could implement cache warming, statistics reset, etc.
}

// cleanupExpiredEntries removes expired cache entries
func (cm *CacheManager) cleanupExpiredEntries() {
	result := cm.db.Where("expires_at < ?", time.Now()).Delete(&models.TMDbCache{})
	if result.Error != nil {
		cm.logger.Warn("failed to cleanup expired cache entries", "error", result.Error)
		return
	}

	if result.RowsAffected > 0 {
		cm.logger.Info("cleaned up expired cache entries", "count", result.RowsAffected)
	}
}

// deleteExpiredEntry deletes a specific expired cache entry
func (cm *CacheManager) deleteExpiredEntry(id uint32) {
	if err := cm.db.Delete(&models.TMDbCache{}, id).Error; err != nil {
		cm.logger.Warn("failed to delete expired cache entry", "error", err, "id", id)
	}
}

// GetCacheStats returns cache statistics
func (cm *CacheManager) GetCacheStats() (*CacheStats, error) {
	var total int64
	var expired int64

	// Count total entries
	if err := cm.db.Model(&models.TMDbCache{}).Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count total cache entries: %w", err)
	}

	// Count expired entries
	if err := cm.db.Model(&models.TMDbCache{}).Where("expires_at < ?", time.Now()).Count(&expired).Error; err != nil {
		return nil, fmt.Errorf("failed to count expired cache entries: %w", err)
	}

	// Get oldest and newest entries
	var oldest, newest models.TMDbCache
	cm.db.Model(&models.TMDbCache{}).Order("created_at ASC").First(&oldest)
	cm.db.Model(&models.TMDbCache{}).Order("created_at DESC").First(&newest)

	return &CacheStats{
		TotalEntries:   total,
		ExpiredEntries: expired,
		ActiveEntries:  total - expired,
		OldestEntry:    oldest.CreatedAt,
		NewestEntry:    newest.CreatedAt,
	}, nil
}

// ClearCache clears all cache entries
func (cm *CacheManager) ClearCache() error {
	result := cm.db.Where("1 = 1").Delete(&models.TMDbCache{})
	if result.Error != nil {
		return fmt.Errorf("failed to clear cache: %w", result.Error)
	}

	cm.logger.Info("cache cleared", "entries_deleted", result.RowsAffected)
	return nil
}

// ClearExpiredCache clears only expired cache entries
func (cm *CacheManager) ClearExpiredCache() error {
	result := cm.db.Where("expires_at < ?", time.Now()).Delete(&models.TMDbCache{})
	if result.Error != nil {
		return fmt.Errorf("failed to clear expired cache: %w", result.Error)
	}

	cm.logger.Info("expired cache cleared", "entries_deleted", result.RowsAffected)
	return nil
}

// generateHash generates a hash for the query
func (cm *CacheManager) generateHash(query string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(query)))
}

// CacheStats represents cache statistics
type CacheStats struct {
	TotalEntries   int64     `json:"total_entries"`
	ExpiredEntries int64     `json:"expired_entries"`
	ActiveEntries  int64     `json:"active_entries"`
	OldestEntry    time.Time `json:"oldest_entry"`
	NewestEntry    time.Time `json:"newest_entry"`
}

// Additional types that would be shared (these should ideally be in a separate types package)

// CacheStats represents cache statistics

// UpdateConfiguration updates the cache manager configuration at runtime
func (cm *CacheManager) UpdateConfiguration(newConfig *config.Config) {
	cm.config = newConfig
	cm.logger.Debug("cache manager configuration updated",
		"cache_duration_hours", newConfig.Cache.DurationHours,
		"cleanup_interval", newConfig.Cache.CleanupInterval)
}
