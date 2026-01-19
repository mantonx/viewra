package internal

import (
	"context"
	"log/slog"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

// SARInteraction represents a user-item interaction for collaborative filtering.
// This is the base data structure used to build the SAR model.
type SARInteraction struct {
	UserID    string
	ItemID    int64
	Timestamp time.Time
	Weight    float32 // Optional explicit weight (1.0 if not specified)
}

// NewSARInteraction creates a new interaction with default weight.
func NewSARInteraction(userID string, itemID int64, timestamp time.Time) SARInteraction {
	return SARInteraction{
		UserID:    userID,
		ItemID:    itemID,
		Timestamp: timestamp,
		Weight:    1.0,
	}
}

// NewWeightedSARInteraction creates a new interaction with explicit weight.
func NewWeightedSARInteraction(userID string, itemID int64, timestamp time.Time, weight float32) SARInteraction {
	return SARInteraction{
		UserID:    userID,
		ItemID:    itemID,
		Timestamp: timestamp,
		Weight:    weight,
	}
}

// SARScoredItem holds an item ID and its recommendation score.
type SARScoredItem struct {
	ItemID int64
	Score  float32
}

// SARModel implements the Simple Algorithm for Recommendation (SAR).
// SAR uses item co-occurrence patterns from user histories to compute
// item-item similarity, combined with time-decayed user affinity to
// generate personalized recommendations.
//
// The algorithm:
// 1. Build item-item similarity using Jaccard coefficient on co-occurrence
// 2. Build user-item affinity with exponential time decay
// 3. Recommendations = Affinity × Similarity
type SARModel struct {
	// similarity[itemA][itemB] = Jaccard similarity coefficient
	similarity map[int64]map[int64]float32

	// affinity[userID][itemID] = time-weighted interaction score
	affinity map[string]map[int64]float32

	// itemCounts[itemID] = number of users who interacted with item
	itemCounts map[int64]int

	// Configuration
	halfLife time.Duration // Half-life for time decay (default 30 days)

	mu sync.RWMutex
}

// SARConfig holds configuration for the SAR model.
type SARConfig struct {
	HalfLife time.Duration // Half-life for time decay
}

// DefaultSARConfig returns the default SAR configuration.
func DefaultSARConfig() SARConfig {
	return SARConfig{
		HalfLife: 30 * 24 * time.Hour, // 30 days
	}
}

// NewSARModel creates a new SAR model with default configuration.
func NewSARModel() *SARModel {
	return NewSARModelWithConfig(DefaultSARConfig())
}

// NewSARModelWithConfig creates a new SAR model with custom configuration.
func NewSARModelWithConfig(config SARConfig) *SARModel {
	return &SARModel{
		similarity: make(map[int64]map[int64]float32),
		affinity:   make(map[string]map[int64]float32),
		itemCounts: make(map[int64]int),
		halfLife:   config.HalfLife,
	}
}

// Build constructs the SAR model from interaction data.
// This builds both the item-item similarity matrix and user-item affinity matrix.
func (m *SARModel) Build(interactions []SARInteraction) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Reset model state
	m.similarity = make(map[int64]map[int64]float32)
	m.affinity = make(map[string]map[int64]float32)
	m.itemCounts = make(map[int64]int)

	if len(interactions) == 0 {
		return
	}

	// Group interactions by user
	userItems := m.groupByUser(interactions)

	// Count item occurrences
	m.countItems(userItems)

	// Build item-item similarity from co-occurrence
	m.buildSimilarity(userItems)

	// Build user-item affinity with time decay
	m.buildAffinity(interactions)
}

// groupByUser groups interactions by user ID.
func (m *SARModel) groupByUser(interactions []SARInteraction) map[string][]int64 {
	userItems := make(map[string][]int64)
	userSeen := make(map[string]map[int64]bool)

	for _, inter := range interactions {
		if userSeen[inter.UserID] == nil {
			userSeen[inter.UserID] = make(map[int64]bool)
		}
		if !userSeen[inter.UserID][inter.ItemID] {
			userSeen[inter.UserID][inter.ItemID] = true
			userItems[inter.UserID] = append(userItems[inter.UserID], inter.ItemID)
		}
	}

	return userItems
}

// countItems counts how many users interacted with each item.
func (m *SARModel) countItems(userItems map[string][]int64) {
	for _, items := range userItems {
		for _, itemID := range items {
			m.itemCounts[itemID]++
		}
	}
}

// buildSimilarity computes Jaccard similarity from item co-occurrence.
func (m *SARModel) buildSimilarity(userItems map[string][]int64) {
	// Count co-occurrences: items appearing in same user's history
	cooccurrence := make(map[int64]map[int64]int)

	for _, items := range userItems {
		// For each pair of items in user's history
		for i := 0; i < len(items); i++ {
			itemA := items[i]
			if cooccurrence[itemA] == nil {
				cooccurrence[itemA] = make(map[int64]int)
			}
			for j := i + 1; j < len(items); j++ {
				itemB := items[j]
				// Increment both directions
				cooccurrence[itemA][itemB]++
				if cooccurrence[itemB] == nil {
					cooccurrence[itemB] = make(map[int64]int)
				}
				cooccurrence[itemB][itemA]++
			}
		}
	}

	// Normalize by item popularity using Jaccard: |A ∩ B| / |A ∪ B|
	for itemA, neighbors := range cooccurrence {
		if m.similarity[itemA] == nil {
			m.similarity[itemA] = make(map[int64]float32)
		}
		countA := m.itemCounts[itemA]

		for itemB, intersectionCount := range neighbors {
			countB := m.itemCounts[itemB]
			// Jaccard: intersection / union
			// union = countA + countB - intersection
			union := countA + countB - intersectionCount
			if union > 0 {
				m.similarity[itemA][itemB] = float32(intersectionCount) / float32(union)
			}
		}
	}
}

// buildAffinity computes time-decayed user-item affinity.
func (m *SARModel) buildAffinity(interactions []SARInteraction) {
	now := time.Now()

	for _, inter := range interactions {
		age := now.Sub(inter.Timestamp)
		// Exponential time decay: weight = 0.5^(age/halfLife)
		decayFactor := math.Pow(0.5, float64(age)/float64(m.halfLife))
		weight := float32(decayFactor) * inter.Weight

		if m.affinity[inter.UserID] == nil {
			m.affinity[inter.UserID] = make(map[int64]float32)
		}
		// Sum affinities for repeated interactions
		m.affinity[inter.UserID][inter.ItemID] += weight
	}
}

// Recommend generates recommendations for a user.
// Returns up to `limit` item IDs, excluding items in the exclude set.
// Returns nil for cold-start users with no affinity data.
func (m *SARModel) Recommend(userID string, limit int, exclude map[int64]bool) []int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	userAffinity := m.affinity[userID]
	if len(userAffinity) == 0 {
		return nil // Cold start - no history for this user
	}

	// Score = Σ affinity(user, itemJ) × similarity(itemJ, itemI)
	scores := make(map[int64]float32)

	for itemJ, affinity := range userAffinity {
		// Add weighted similarity to all items similar to itemJ
		for itemI, sim := range m.similarity[itemJ] {
			if exclude != nil && exclude[itemI] {
				continue
			}
			// Skip items user already has affinity with
			if _, alreadyInteracted := userAffinity[itemI]; alreadyInteracted {
				continue
			}
			scores[itemI] += affinity * sim
		}
	}

	return m.topK(scores, limit)
}

// RecommendWithScores returns recommendations with their scores.
func (m *SARModel) RecommendWithScores(userID string, limit int, exclude map[int64]bool) []SARScoredItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	userAffinity := m.affinity[userID]
	if len(userAffinity) == 0 {
		return nil
	}

	scores := make(map[int64]float32)

	for itemJ, affinity := range userAffinity {
		for itemI, sim := range m.similarity[itemJ] {
			if exclude != nil && exclude[itemI] {
				continue
			}
			if _, alreadyInteracted := userAffinity[itemI]; alreadyInteracted {
				continue
			}
			scores[itemI] += affinity * sim
		}
	}

	return m.topKWithScores(scores, limit)
}

// topK returns the top K items by score.
func (m *SARModel) topK(scores map[int64]float32, k int) []int64 {
	items := m.topKWithScores(scores, k)
	result := make([]int64, len(items))
	for i, item := range items {
		result[i] = item.ItemID
	}
	return result
}

// topKWithScores returns the top K items with their scores.
func (m *SARModel) topKWithScores(scores map[int64]float32, k int) []SARScoredItem {
	if len(scores) == 0 {
		return nil
	}

	items := make([]SARScoredItem, 0, len(scores))
	for itemID, score := range scores {
		items = append(items, SARScoredItem{ItemID: itemID, Score: score})
	}

	// Sort by score descending
	sort.Slice(items, func(i, j int) bool {
		return items[i].Score > items[j].Score
	})

	if k > len(items) {
		k = len(items)
	}
	return items[:k]
}

// HasUser returns whether the model has affinity data for a user.
func (m *SARModel) HasUser(userID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.affinity[userID]
	return ok
}

// UserCount returns the number of users in the model.
func (m *SARModel) UserCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.affinity)
}

// ItemCount returns the number of items in the model.
func (m *SARModel) ItemCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.itemCounts)
}

// SARService wraps the SAR model and provides methods to build and query it.
// It collects interactions from ratings and watch history to build collaborative
// filtering recommendations.
type SARService struct {
	model    *SARModel
	ratings  *sdk.RatingsClient
	progress *sdk.ProgressClient
	data     *sdk.DataClient
	logger   *slog.Logger

	// Track when model was last built
	lastBuildTime time.Time
	buildInterval time.Duration

	mu sync.RWMutex
}

// NewSARService creates a new SAR service.
func NewSARService(
	ratings *sdk.RatingsClient,
	progress *sdk.ProgressClient,
	data *sdk.DataClient,
	logger *slog.Logger,
) *SARService {
	return &SARService{
		model:         NewSARModel(),
		ratings:       ratings,
		progress:      progress,
		data:          data,
		logger:        logger,
		buildInterval: 1 * time.Hour, // Rebuild at most once per hour
	}
}

// BuildModel collects interactions and builds the SAR model.
// This should be called periodically or when user data changes.
func (s *SARService) BuildModel(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if we've built recently
	if time.Since(s.lastBuildTime) < s.buildInterval {
		s.logger.Debug("SAR model build skipped (too recent)",
			"last_build", s.lastBuildTime,
			"interval", s.buildInterval,
		)
		return nil
	}

	s.logger.Debug("building SAR model from user interactions")

	interactions, err := s.collectAllInteractions(ctx)
	if err != nil {
		return err
	}

	if len(interactions) == 0 {
		s.logger.Debug("no interactions found, SAR model will be empty")
		return nil
	}

	s.model.Build(interactions)
	s.lastBuildTime = time.Now()

	s.logger.Info("SAR model built",
		"users", s.model.UserCount(),
		"items", s.model.ItemCount(),
		"interactions", len(interactions),
	)

	return nil
}

// collectAllInteractions gathers all user interactions for SAR.
func (s *SARService) collectAllInteractions(ctx context.Context) ([]SARInteraction, error) {
	var interactions []SARInteraction

	// For now, we collect from the default user (user "1")
	// In the future, we can iterate through all users
	userIDs := []string{"1"}

	for _, userID := range userIDs {
		userInteractions, err := s.collectUserInteractions(ctx, userID)
		if err != nil {
			s.logger.Warn("failed to collect interactions for user",
				"user_id", userID,
				"error", err,
			)
			continue
		}
		interactions = append(interactions, userInteractions...)
	}

	return interactions, nil
}

// collectUserInteractions gathers all interactions for a single user.
func (s *SARService) collectUserInteractions(ctx context.Context, userID string) ([]SARInteraction, error) {
	var interactions []SARInteraction
	now := time.Now()

	// Collect from ratings (favorites = weight 2.0, upvotes = weight 1.0)
	if s.ratings != nil {
		// Get favorites (higher weight)
		favoriteIDs, err := s.ratings.GetFavoriteIDs(ctx, userID, "", 1000)
		if err != nil {
			s.logger.Debug("failed to get favorite IDs", "user_id", userID, "error", err)
		} else {
			for _, id := range favoriteIDs {
				interactions = append(interactions, NewWeightedSARInteraction(userID, id, now, 2.0))
			}
		}

		// Get upvotes (normal weight)
		upvotedIDs, err := s.ratings.GetUpvotedIDs(ctx, userID, "", 1000)
		if err != nil {
			s.logger.Debug("failed to get upvoted IDs", "user_id", userID, "error", err)
		} else {
			for _, id := range upvotedIDs {
				// Skip if already added as favorite
				found := false
				for _, fav := range favoriteIDs {
					if fav == id {
						found = true
						break
					}
				}
				if !found {
					interactions = append(interactions, NewWeightedSARInteraction(userID, id, now, 1.0))
				}
			}
		}
	}

	// Collect from watch history (completed = weight 1.0, in-progress = weight 0.5)
	if s.progress != nil {
		// Get watched items
		watchedItems, err := s.progress.ListWatchedItems(ctx, userID, "", 1000, 0)
		if err != nil {
			s.logger.Debug("failed to get watched items", "user_id", userID, "error", err)
		} else {
			for _, item := range watchedItems {
				interactions = append(interactions, NewWeightedSARInteraction(
					userID,
					item.MediaID,
					item.LastWatchedAt,
					1.0,
				))
			}
		}

		// Get in-progress items (partial weight)
		inProgressItems, err := s.progress.ListInProgressItems(ctx, userID, "", 1000, 0)
		if err != nil {
			s.logger.Debug("failed to get in-progress items", "user_id", userID, "error", err)
		} else {
			for _, item := range inProgressItems {
				// Weight based on progress percentage (more progress = higher weight)
				weight := float32(0.3 + (item.ProgressPercent/100.0)*0.7)
				interactions = append(interactions, NewWeightedSARInteraction(
					userID,
					item.MediaID,
					item.LastWatchedAt,
					weight,
				))
			}
		}
	}

	return interactions, nil
}

// Recommend returns SAR-based recommendations for a user.
// Returns nil if the model hasn't been built or the user has no history.
func (s *SARService) Recommend(userID string, limit int, exclude map[int64]bool) []int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.model.Recommend(userID, limit, exclude)
}

// RecommendWithScores returns SAR recommendations with scores.
func (s *SARService) RecommendWithScores(userID string, limit int, exclude map[int64]bool) []SARScoredItem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.model.RecommendWithScores(userID, limit, exclude)
}

// HasUser returns whether the SAR model has data for a user.
func (s *SARService) HasUser(userID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.model.HasUser(userID)
}

// IsModelReady returns whether the SAR model has been built.
func (s *SARService) IsModelReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.model.ItemCount() > 0
}

// Stats returns model statistics.
func (s *SARService) Stats() (users, items int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.model.UserCount(), s.model.ItemCount()
}
