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
	ratings *sdk.RatingsClient
	data    *sdk.DataClient
	plugins *sdk.PluginsClient
	config  Config
	logger  *slog.Logger
	mu      sync.RWMutex
}

// NewRecommendationsService creates a new recommendations service.
func NewRecommendationsService(
	ratings *sdk.RatingsClient,
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

	// Get user's positively rated items (favorites + upvotes)
	likedIDs, err := s.ratings.GetPositivelyRatedIDs(ctx, userID, "", limit)
	if err != nil {
		return nil, fmt.Errorf("get positively rated IDs: %w", err)
	}

	if len(likedIDs) == 0 {
		// No ratings yet - return empty or could return popular items
		s.logger.Debug("no ratings for user, returning empty recommendations", "user_id", userID)
		return []sdk.MediaItem{}, nil
	}

	// Get downvoted items to exclude
	excludeIDs, _ := s.ratings.GetDownvotedIDs(ctx, userID, "", 100)
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
	favoriteIDs, err := s.ratings.GetFavoriteIDs(ctx, userID, "", 10)
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
	downvoted, _ := s.ratings.GetDownvotedIDs(ctx, userID, "", 100)
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

// getSimilarItems uses semantic search (if available) to find similar items.
// Falls back to genre-based recommendations if semantic search is unavailable.
func (s *RecommendationsService) getSimilarItems(ctx context.Context, baseIDs []int64, exclude map[int64]bool, limit int) ([]sdk.MediaItem, error) {
	if s.data == nil {
		return nil, fmt.Errorf("data client not available")
	}

	var allSimilar []sdk.MediaItem
	seen := make(map[int64]bool)

	// Check if vector search is available via the plugins client
	vectorSearchAvailable := s.plugins != nil && s.plugins.IsVectorSearchAvailable(ctx)

	// For each base item, find similar items
	for _, baseID := range baseIDs {
		if len(allSimilar) >= limit {
			break
		}

		// Get the base item details to determine entity type
		baseDetails, err := s.data.GetMediaDetails(ctx, baseID, "")
		if err != nil {
			s.logger.Debug("failed to get base item details", "base_id", baseID, "error", err)
			continue
		}

		var similar []sdk.MediaItem

		if vectorSearchAvailable {
			// Use semantic search to find similar items
			results, _, err := s.plugins.FindSimilar(ctx, baseDetails.MediaType, baseID, limit-len(allSimilar))
			if err != nil {
				s.logger.Debug("semantic search failed, falling back to genre-based",
					"base_id", baseID, "error", err)
				// Fall back to genre-based
				similar, _ = s.getGenreBasedRecommendationsForItem(ctx, baseDetails, exclude, limit-len(allSimilar))
			} else {
				// Convert semantic search results to MediaItems
				for _, r := range results {
					if exclude[r.EntityID] || seen[r.EntityID] || r.EntityID == baseID {
						continue
					}
					similar = append(similar, sdk.MediaItem{
						EntityType: r.EntityType,
						EntityID:   r.EntityID,
						Reason:     fmt.Sprintf("Similar to %s", baseDetails.Title),
					})
				}
			}
		} else {
			// No vector search available, use genre-based fallback
			similar, err = s.getGenreBasedRecommendationsForItem(ctx, baseDetails, exclude, limit-len(allSimilar))
			if err != nil {
				s.logger.Debug("genre-based recommendations failed",
					"base_id", baseID, "error", err)
				continue
			}
		}

		for _, item := range similar {
			if !seen[item.EntityID] && !exclude[item.EntityID] {
				if item.Reason == "" {
					item.Reason = fmt.Sprintf("Similar to %s", baseDetails.Title)
				}
				allSimilar = append(allSimilar, item)
				seen[item.EntityID] = true
			}
		}
	}

	return allSimilar, nil
}

// getGenreBasedRecommendations finds items with matching genres for a list of base IDs.
// This is called from GetForYou when semantic search is unavailable.
func (s *RecommendationsService) getGenreBasedRecommendations(ctx context.Context, baseIDs []int64, exclude map[int64]bool, limit int) ([]sdk.MediaItem, error) {
	if s.data == nil {
		return nil, fmt.Errorf("data client not available")
	}

	var allItems []sdk.MediaItem
	seen := make(map[int64]bool)

	for _, baseID := range baseIDs {
		if len(allItems) >= limit {
			break
		}

		baseDetails, err := s.data.GetMediaDetails(ctx, baseID, "")
		if err != nil {
			s.logger.Debug("failed to get base item details", "base_id", baseID, "error", err)
			continue
		}

		items, err := s.getGenreBasedRecommendationsForItem(ctx, baseDetails, exclude, limit-len(allItems))
		if err != nil {
			continue
		}

		for _, item := range items {
			if !seen[item.EntityID] {
				seen[item.EntityID] = true
				allItems = append(allItems, item)
			}
		}
	}

	return allItems, nil
}

// getGenreBasedRecommendationsForItem finds items with matching genres for a single base item.
// Uses the host's ListMediaByGenre RPC for database-level genre matching.
func (s *RecommendationsService) getGenreBasedRecommendationsForItem(ctx context.Context, baseItem *sdk.MediaDetails, exclude map[int64]bool, limit int) ([]sdk.MediaItem, error) {
	if s.data == nil {
		return nil, fmt.Errorf("data client not available")
	}

	if len(baseItem.Genres) == 0 {
		s.logger.Debug("base item has no genres, cannot find genre-based recommendations",
			"base_id", baseItem.ID)
		return []sdk.MediaItem{}, nil
	}

	// Build list of IDs to exclude
	excludeIDs := make([]int64, 0, len(exclude)+1)
	excludeIDs = append(excludeIDs, baseItem.ID) // Exclude the base item itself
	for id := range exclude {
		excludeIDs = append(excludeIDs, id)
	}

	// Determine media type for query
	mediaType := baseItem.MediaType
	if mediaType == "tv_episode" {
		mediaType = "tv_show" // For episodes, recommend shows
	}
	if mediaType != "movie" && mediaType != "tv_show" {
		s.logger.Debug("unsupported media type for genre recommendations",
			"media_type", mediaType)
		return []sdk.MediaItem{}, nil
	}

	var allItems []sdk.MediaItem
	seen := make(map[int64]bool)

	// Search using the first few genres (primary genres are usually most relevant)
	maxGenres := 2
	if len(baseItem.Genres) < maxGenres {
		maxGenres = len(baseItem.Genres)
	}

	for i := 0; i < maxGenres && len(allItems) < limit; i++ {
		genre := baseItem.Genres[i]

		items, err := s.data.ListMediaByGenre(ctx, mediaType, genre, 0, excludeIDs, limit-len(allItems))
		if err != nil {
			s.logger.Debug("ListMediaByGenre failed", "genre", genre, "error", err)
			continue
		}

		for _, item := range items {
			if seen[item.ID] {
				continue
			}
			seen[item.ID] = true

			allItems = append(allItems, sdk.MediaItem{
				EntityType: mediaType,
				EntityID:   item.ID,
				Title:      item.Title,
				Year:       item.Year,
				Reason:     fmt.Sprintf("Also in %s", genre),
			})
		}
	}

	return allItems, nil
}
