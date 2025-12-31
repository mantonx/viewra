package internal

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

// RecommendationsService generates personalized recommendations.
type RecommendationsService struct {
	ratings *RatingsService
	data    *sdk.DataClient
	plugins *sdk.PluginsClient
	config  Config
	logger  *slog.Logger
	mu      sync.RWMutex
}

// NewRecommendationsService creates a new recommendations service.
func NewRecommendationsService(
	ratings *RatingsService,
	data *sdk.DataClient,
	plugins *sdk.PluginsClient,
	config Config,
	logger *slog.Logger,
) *RecommendationsService {
	return &RecommendationsService{
		ratings: ratings,
		data:    data,
		plugins: plugins,
		config:  config,
		logger:  logger,
	}
}

// UpdateConfig updates the service configuration.
func (s *RecommendationsService) UpdateConfig(config Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = config
}

// GetForYou returns personalized recommendations based on user ratings.
func (s *RecommendationsService) GetForYou(ctx context.Context, userID string, limit int) ([]sdk.MediaItem, error) {
	s.mu.RLock()
	config := s.config
	s.mu.RUnlock()

	if limit <= 0 {
		limit = config.MaxRecommendations
	}

	// Get user's liked items
	likedIDs, err := s.ratings.GetUpvotedEntityIDs(ctx, userID, "", limit)
	if err != nil {
		return nil, fmt.Errorf("get upvoted IDs: %w", err)
	}

	if len(likedIDs) == 0 {
		// No ratings yet - return empty or could return popular items
		s.logger.Debug("no ratings for user, returning empty recommendations", "user_id", userID)
		return []sdk.MediaItem{}, nil
	}

	// Get downvoted items to exclude
	excludeIDs, _ := s.ratings.GetDownvotedEntityIDs(ctx, userID, 100)
	excludeSet := make(map[int64]bool)
	for _, id := range excludeIDs {
		excludeSet[id] = true
	}
	// Also exclude already liked items
	for _, id := range likedIDs {
		excludeSet[id] = true
	}

	// Try to use semantic search for similar items if available
	var recommendations []sdk.MediaItem
	if s.plugins != nil && s.plugins.IsAvailable(ctx, "embedding") {
		recommendations, err = s.getSimilarItems(ctx, likedIDs, excludeSet, limit)
		if err != nil {
			s.logger.Debug("semantic similar search failed, falling back to genre-based", "error", err)
		}
	}

	// Fall back to genre-based recommendations if semantic search not available or failed
	if len(recommendations) == 0 {
		recommendations, err = s.getGenreBasedRecommendations(ctx, likedIDs, excludeSet, limit)
		if err != nil {
			return nil, fmt.Errorf("genre-based recommendations: %w", err)
		}
	}

	// Add recommendation reasons
	for i := range recommendations {
		if recommendations[i].Reason == "" {
			recommendations[i].Reason = "Based on your ratings"
		}
	}

	return recommendations, nil
}

// GetBecauseYouLiked returns a recommendation row based on a specific favorite item.
func (s *RecommendationsService) GetBecauseYouLiked(ctx context.Context, userID string, limit int) (map[string]any, error) {
	s.mu.RLock()
	config := s.config
	s.mu.RUnlock()

	if limit <= 0 {
		limit = config.MaxRecommendations
	}

	// Get a random favorite to base recommendations on
	favoriteIDs, err := s.ratings.GetFavoriteEntityIDs(ctx, userID, "", 10)
	if err != nil || len(favoriteIDs) == 0 {
		return map[string]any{
			"title":    "Because You Liked...",
			"subtitle": "",
			"items":    []sdk.MediaItem{},
		}, nil
	}

	// Pick the most recent favorite
	baseID := favoriteIDs[0]

	// Get the base item's details
	baseItem, err := s.data.GetMediaDetails(ctx, baseID, "")
	if err != nil {
		s.logger.Debug("failed to get base item details", "id", baseID, "error", err)
		return map[string]any{
			"title":    "Because You Liked...",
			"subtitle": "",
			"items":    []sdk.MediaItem{},
		}, nil
	}

	// Build exclude set
	excludeSet := make(map[int64]bool)
	for _, id := range favoriteIDs {
		excludeSet[id] = true
	}
	downvoted, _ := s.ratings.GetDownvotedEntityIDs(ctx, userID, 100)
	for _, id := range downvoted {
		excludeSet[id] = true
	}

	// Get similar items
	var items []sdk.MediaItem
	if s.plugins != nil && s.plugins.IsAvailable(ctx, "embedding") {
		items, err = s.getSimilarItems(ctx, []int64{baseID}, excludeSet, limit)
		if err != nil {
			s.logger.Debug("semantic search failed", "error", err)
		}
	}

	// Fall back to genre-based
	if len(items) == 0 {
		items, err = s.getGenreBasedRecommendations(ctx, []int64{baseID}, excludeSet, limit)
		if err != nil {
			s.logger.Debug("genre-based recommendations failed", "error", err)
		}
	}

	// Add reason to each item
	reason := fmt.Sprintf("Similar to %s", baseItem.Title)
	for i := range items {
		items[i].Reason = reason
	}

	return map[string]any{
		"title":    "Because You Liked...",
		"subtitle": baseItem.Title,
		"items":    items,
	}, nil
}

// GetFavorites returns the user's favorited items.
func (s *RecommendationsService) GetFavorites(ctx context.Context, userID string, limit int) ([]sdk.MediaItem, error) {
	s.mu.RLock()
	config := s.config
	s.mu.RUnlock()

	if limit <= 0 {
		limit = config.MaxRecommendations
	}

	favoriteIDs, err := s.ratings.GetFavoriteEntityIDs(ctx, userID, "", limit)
	if err != nil {
		return nil, fmt.Errorf("get favorite IDs: %w", err)
	}

	if len(favoriteIDs) == 0 {
		return []sdk.MediaItem{}, nil
	}

	// Fetch details for each favorite
	var items []sdk.MediaItem
	for _, id := range favoriteIDs {
		details, err := s.data.GetMediaDetails(ctx, id, "")
		if err != nil {
			s.logger.Debug("failed to get media details", "id", id, "error", err)
			continue
		}

		rating := RatingFavorite
		items = append(items, sdk.MediaItem{
			EntityType: details.MediaType,
			EntityID:   id,
			Title:      details.Title,
			Year:       details.Year,
			Rating:     &rating,
		})
	}

	return items, nil
}

// getSimilarItems uses semantic search to find similar items.
func (s *RecommendationsService) getSimilarItems(ctx context.Context, baseIDs []int64, exclude map[int64]bool, limit int) ([]sdk.MediaItem, error) {
	if s.data == nil {
		return nil, fmt.Errorf("data client not available")
	}

	var allSimilar []sdk.MediaItem
	seen := make(map[int64]bool)

	// For each base item, find similar items
	for _, baseID := range baseIDs {
		if len(allSimilar) >= limit {
			break
		}

		// Get the base item details
		baseDetails, err := s.data.GetMediaDetails(ctx, baseID, "")
		if err != nil {
			continue
		}

		// Find similar items using semantic search
		// This would call the semantic-search plugin via the plugins client
		// For now, we'll fall back to genre-based since we don't have direct access
		// to the semantic search capability in this implementation

		// Use genre matching as a fallback
		similar, err := s.getGenreBasedRecommendations(ctx, []int64{baseID}, exclude, limit-len(allSimilar))
		if err != nil {
			continue
		}

		for _, item := range similar {
			if !seen[item.EntityID] && !exclude[item.EntityID] {
				item.Reason = fmt.Sprintf("Similar to %s", baseDetails.Title)
				allSimilar = append(allSimilar, item)
				seen[item.EntityID] = true
			}
		}
	}

	return allSimilar, nil
}

// getGenreBasedRecommendations finds items with matching genres.
// Note: This is a simplified implementation that returns empty results.
// A full implementation would use the semantic search plugin to find similar items.
func (s *RecommendationsService) getGenreBasedRecommendations(ctx context.Context, baseIDs []int64, exclude map[int64]bool, limit int) ([]sdk.MediaItem, error) {
	// TODO: Implement genre-based recommendations using semantic search plugin
	// For now, return empty list - the home screen will show other widgets
	s.logger.Debug("genre-based recommendations not yet implemented, returning empty list")
	return []sdk.MediaItem{}, nil
}
