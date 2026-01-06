package internal

import (
	"context"
	"errors"
	"log/slog"
	"sort"

	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

var (
	// ErrNoInteractions is returned when a user has no interactions to build recommendations from.
	ErrNoInteractions = errors.New("user has no interactions")
	// ErrVectorSearchUnavailable is returned when vector search is not available.
	ErrVectorSearchUnavailable = errors.New("vector search not available")
)

// UserEmbeddingService generates personalized recommendations from user interaction history.
// It uses the semantic-search plugin's FindSimilar capability to find items similar to
// the user's positively rated items, then aggregates and ranks the results.
type UserEmbeddingService struct {
	ratings  *sdk.RatingsClient
	progress *sdk.ProgressClient
	plugins  *sdk.PluginsClient
	data     *sdk.DataClient
	logger   *slog.Logger
}

// NewUserEmbeddingService creates a new user embedding service.
func NewUserEmbeddingService(
	ratings *sdk.RatingsClient,
	progress *sdk.ProgressClient,
	plugins *sdk.PluginsClient,
	data *sdk.DataClient,
	logger *slog.Logger,
) *UserEmbeddingService {
	return &UserEmbeddingService{
		ratings:  ratings,
		progress: progress,
		plugins:  plugins,
		data:     data,
		logger:   logger,
	}
}

// scoredItem holds an item with its aggregated recommendation score.
type scoredItem struct {
	EntityType string
	EntityID   int64
	Score      float32
	Sources    int // Number of seed items that led to this recommendation
}

// GetUserTasteRecommendations finds items similar to what the user likes.
// It aggregates results from FindSimilar queries for each seed item, giving higher
// scores to items that appear similar to multiple liked items.
//
// The algorithm:
// 1. Collect seed items from ratings (favorites, upvotes) and watch history
// 2. For each seed, find similar items using semantic search
// 3. Aggregate scores - items similar to multiple seeds get boosted
// 4. Filter out already-interacted items and return top results
func (s *UserEmbeddingService) GetUserTasteRecommendations(
	ctx context.Context,
	userID string,
	exclude map[int64]bool,
	limit int,
) ([]sdk.MediaItem, error) {
	if s.plugins == nil || !s.plugins.IsVectorSearchAvailable(ctx) {
		return nil, ErrVectorSearchUnavailable
	}

	// Collect seed items from ratings and watch history
	seedItems, err := s.collectSeedItems(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(seedItems) == 0 {
		return nil, ErrNoInteractions
	}

	s.logger.Debug("collected seed items for user taste recommendations",
		"user_id", userID,
		"num_seeds", len(seedItems),
	)

	// Aggregate similar items across all seeds
	scores := make(map[int64]*scoredItem)

	// Weight seeds by recency (index 0 = most recent)
	for i, seed := range seedItems {
		if i >= 10 {
			break // Limit to top 10 seeds to avoid too many API calls
		}

		// Recency weight: 1.0, 0.9, 0.8, ...
		seedWeight := 1.0 - float32(i)*0.1
		if seedWeight < 0.1 {
			seedWeight = 0.1
		}

		// Find similar items for this seed
		results, _, err := s.plugins.FindSimilar(ctx, seed.EntityType, seed.EntityID, 20)
		if err != nil {
			s.logger.Debug("FindSimilar failed for seed",
				"entity_type", seed.EntityType,
				"entity_id", seed.EntityID,
				"error", err,
			)
			continue
		}

		for _, r := range results {
			// Skip if this is the seed itself or excluded
			if r.EntityID == seed.EntityID || exclude[r.EntityID] {
				continue
			}

			// Aggregate score: similarity * seed weight
			score := r.Similarity * seedWeight

			if existing, ok := scores[r.EntityID]; ok {
				existing.Score += score
				existing.Sources++
			} else {
				scores[r.EntityID] = &scoredItem{
					EntityType: r.EntityType,
					EntityID:   r.EntityID,
					Score:      score,
					Sources:    1,
				}
			}
		}
	}

	// Convert to slice and sort by score (descending)
	items := make([]*scoredItem, 0, len(scores))
	for _, item := range scores {
		// Boost items that appear similar to multiple seeds
		if item.Sources > 1 {
			item.Score *= 1.0 + float32(item.Sources-1)*0.2 // +20% per additional source
		}
		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Score > items[j].Score
	})

	// Convert to MediaItem slice
	result := make([]sdk.MediaItem, 0, limit)
	for _, item := range items {
		if len(result) >= limit {
			break
		}
		result = append(result, sdk.MediaItem{
			EntityType: item.EntityType,
			EntityID:   item.EntityID,
			Reason:     "Matches your taste",
		})
	}

	s.logger.Debug("user taste recommendations generated",
		"user_id", userID,
		"total_candidates", len(scores),
		"returned", len(result),
	)

	return result, nil
}

// seedItem represents an item to base recommendations on.
type seedItem struct {
	EntityType string
	EntityID   int64
}

// collectSeedItems gathers items the user has positively interacted with.
func (s *UserEmbeddingService) collectSeedItems(ctx context.Context, userID string) ([]seedItem, error) {
	var seeds []seedItem
	seenIDs := make(map[int64]bool)

	// Collect from ratings (favorites and upvotes)
	if s.ratings != nil {
		likedIDs, err := s.ratings.GetPositivelyRatedIDs(ctx, userID, "", 50)
		if err != nil {
			s.logger.Debug("failed to get positively rated IDs", "user_id", userID, "error", err)
		} else {
			for _, id := range likedIDs {
				if !seenIDs[id] {
					seenIDs[id] = true
					// Determine entity type by querying data
					entityType := s.getEntityType(ctx, id)
					seeds = append(seeds, seedItem{
						EntityType: entityType,
						EntityID:   id,
					})
				}
			}
		}
	}

	// Collect from watch history (completed watches)
	if s.progress != nil {
		watches, err := s.progress.ListWatchedItems(ctx, userID, "", 50, 0)
		if err != nil {
			s.logger.Debug("failed to get watched items", "user_id", userID, "error", err)
		} else {
			for _, w := range watches {
				if !seenIDs[w.MediaID] {
					seenIDs[w.MediaID] = true
					seeds = append(seeds, seedItem{
						EntityType: w.MediaType,
						EntityID:   w.MediaID,
					})
				}
			}
		}
	}

	return seeds, nil
}

// getEntityType determines the entity type for a media ID.
func (s *UserEmbeddingService) getEntityType(ctx context.Context, mediaID int64) string {
	if s.data == nil {
		return "movie" // Default fallback
	}

	// Try movie first
	media, err := s.data.GetMedia(ctx, mediaID, "movie")
	if err == nil && media != nil {
		return media.MediaType
	}

	// Try tv_show
	media, err = s.data.GetMedia(ctx, mediaID, "tv_show")
	if err == nil && media != nil {
		return media.MediaType
	}

	return "movie" // Default fallback
}

// uniqueIDs returns a deduplicated slice of IDs while preserving order.
func uniqueIDs(ids []int64) []int64 {
	seen := make(map[int64]bool)
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}

// weightedAverage computes the weighted average of multiple vectors.
// Each vector is multiplied by its corresponding weight, then summed and normalized.
func weightedAverage(vectors [][]float32, weights []float32) []float32 {
	if len(vectors) == 0 || len(vectors) != len(weights) {
		return nil
	}

	dims := len(vectors[0])
	result := make([]float32, dims)
	var totalWeight float32

	for i, v := range vectors {
		w := weights[i]
		totalWeight += w
		for j := range result {
			result[j] += v[j] * w
		}
	}

	// Normalize by total weight
	if totalWeight > 0 {
		for i := range result {
			result[i] /= totalWeight
		}
	}

	return result
}
