package internal

import (
	"context"
	"log/slog"
	"math/rand"
	"sort"
	"sync"

	"github.com/mantonx/viewra/pkg/plugin/sdk"
	"github.com/mantonx/viewra/plugins/recommendations/internal/cf"
)

// HybridWeights configures the balance between recommendation strategies.
type HybridWeights struct {
	// Collaborative is the weight for SAR collaborative filtering (0-100)
	Collaborative int `yaml:"collaborative" json:"collaborative"`

	// Semantic is the weight for semantic/content-based similarity (0-100)
	Semantic int `yaml:"semantic" json:"semantic"`

	// Exploration is the weight for discovery/exploration items (0-100)
	Exploration int `yaml:"exploration" json:"exploration"`
}

// DefaultHybridWeights returns balanced default weights.
func DefaultHybridWeights() HybridWeights {
	return HybridWeights{
		Collaborative: 50, // 50% collaborative filtering
		Semantic:      30, // 30% semantic similarity
		Exploration:   20, // 20% exploration/discovery
	}
}

// Normalize ensures weights sum to 100.
func (w HybridWeights) Normalize() HybridWeights {
	total := w.Collaborative + w.Semantic + w.Exploration
	if total == 0 {
		return DefaultHybridWeights()
	}
	return HybridWeights{
		Collaborative: w.Collaborative * 100 / total,
		Semantic:      w.Semantic * 100 / total,
		Exploration:   w.Exploration * 100 / total,
	}
}

// scoredCandidate holds a recommendation candidate with its score and source.
type scoredCandidate struct {
	EntityType string
	EntityID   int64
	Title      string
	Year       int
	Score      float32
	Source     string // "collaborative", "semantic", "exploration"
	Reason     string
}

// HybridScorer combines multiple recommendation strategies.
type HybridScorer struct {
	sarService           *cf.SARService
	userEmbeddingService *UserEmbeddingService
	ratings              *sdk.RatingsClient
	progress             *sdk.ProgressClient
	data                 *sdk.DataClient
	plugins              *sdk.PluginsClient
	weights              HybridWeights
	logger               *slog.Logger
	mu                   sync.RWMutex
}

// NewHybridScorer creates a new hybrid scorer.
func NewHybridScorer(
	sarService *cf.SARService,
	userEmbeddingService *UserEmbeddingService,
	ratings *sdk.RatingsClient,
	progress *sdk.ProgressClient,
	data *sdk.DataClient,
	plugins *sdk.PluginsClient,
	weights HybridWeights,
	logger *slog.Logger,
) *HybridScorer {
	return &HybridScorer{
		sarService:           sarService,
		userEmbeddingService: userEmbeddingService,
		ratings:              ratings,
		progress:             progress,
		data:                 data,
		plugins:              plugins,
		weights:              weights.Normalize(),
		logger:               logger,
	}
}

// UpdateWeights updates the hybrid scoring weights.
func (h *HybridScorer) UpdateWeights(weights HybridWeights) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.weights = weights.Normalize()
}

// GetRecommendations returns hybrid recommendations combining multiple strategies.
// For cold start users (no history), it returns exploration items only.
func (h *HybridScorer) GetRecommendations(ctx context.Context, userID string, limit int) ([]sdk.MediaItem, error) {
	h.mu.RLock()
	weights := h.weights
	h.mu.RUnlock()

	// Build exclude set
	excludeSet := h.buildExcludeSet(ctx, userID)

	// Check if user has any history
	hasHistory := h.userHasHistory(ctx, userID)

	if !hasHistory {
		// Cold start: return exploration items only
		h.logger.Debug("cold start user, returning exploration items only", "user_id", userID)
		return h.getExplorationItems(ctx, excludeSet, limit)
	}

	// Calculate how many items from each source
	collabCount := limit * weights.Collaborative / 100
	semanticCount := limit * weights.Semantic / 100
	explorationCount := limit - collabCount - semanticCount // Remainder to exploration

	h.logger.Debug("hybrid scoring allocation",
		"user_id", userID,
		"collab", collabCount,
		"semantic", semanticCount,
		"exploration", explorationCount,
	)

	// Collect candidates from each source
	var candidates []scoredCandidate
	seen := make(map[int64]bool)

	// 1. Collaborative filtering (SAR)
	if collabCount > 0 && h.sarService != nil && h.sarService.HasUser(userID) {
		collabCandidates := h.getCollaborativeCandidates(ctx, userID, excludeSet, collabCount*2)
		for _, c := range collabCandidates {
			if !seen[c.EntityID] {
				seen[c.EntityID] = true
				candidates = append(candidates, c)
			}
		}
	}

	// 2. Semantic/content-based similarity
	if semanticCount > 0 && h.userEmbeddingService != nil {
		semanticCandidates := h.getSemanticCandidates(ctx, userID, excludeSet, seen, semanticCount*2)
		for _, c := range semanticCandidates {
			if !seen[c.EntityID] {
				seen[c.EntityID] = true
				candidates = append(candidates, c)
			}
		}
	}

	// 3. Exploration (genre-based discovery)
	if explorationCount > 0 {
		exploreCandidates := h.getExplorationCandidates(ctx, userID, excludeSet, seen, explorationCount*2)
		for _, c := range exploreCandidates {
			if !seen[c.EntityID] {
				seen[c.EntityID] = true
				candidates = append(candidates, c)
			}
		}
	}

	// Score and rank all candidates
	ranked := h.rankCandidates(candidates, weights, limit)

	// Convert to MediaItems
	result := make([]sdk.MediaItem, 0, len(ranked))
	for _, c := range ranked {
		result = append(result, sdk.MediaItem{
			EntityType: c.EntityType,
			EntityID:   c.EntityID,
			Title:      c.Title,
			Year:       c.Year,
			Reason:     c.Reason,
		})
	}

	h.logger.Debug("hybrid recommendations generated",
		"user_id", userID,
		"total_candidates", len(candidates),
		"returned", len(result),
	)

	return result, nil
}

// userHasHistory checks if the user has any ratings or watch history.
func (h *HybridScorer) userHasHistory(ctx context.Context, userID string) bool {
	// Check ratings
	if h.ratings != nil {
		hasRatings, err := h.ratings.HasRatings(ctx, userID)
		if err == nil && hasRatings {
			return true
		}
	}

	// Check watch history
	if h.progress != nil {
		hasWatch, err := h.progress.HasWatchHistory(ctx, userID)
		if err == nil && hasWatch {
			return true
		}
	}

	return false
}

// buildExcludeSet creates a set of item IDs to exclude.
func (h *HybridScorer) buildExcludeSet(ctx context.Context, userID string) map[int64]bool {
	excludeSet := make(map[int64]bool)

	// Exclude downvoted items
	if h.ratings != nil {
		downvotedIDs, _ := h.ratings.GetDownvotedIDs(ctx, userID, "", 100)
		for _, id := range downvotedIDs {
			excludeSet[id] = true
		}
	}

	// Exclude already watched items
	if h.progress != nil {
		watchedIDs, _ := h.progress.GetWatchedEntityIDs(ctx, userID, "", 500)
		for _, id := range watchedIDs {
			excludeSet[id] = true
		}
	}

	return excludeSet
}

// getCollaborativeCandidates gets SAR-based recommendations.
func (h *HybridScorer) getCollaborativeCandidates(ctx context.Context, userID string, exclude map[int64]bool, limit int) []scoredCandidate {
	scored := h.sarService.RecommendWithScores(userID, limit, exclude)
	if len(scored) == 0 {
		return nil
	}

	candidates := make([]scoredCandidate, 0, len(scored))
	for _, s := range scored {
		c := scoredCandidate{
			EntityID: s.ItemID,
			Score:    s.Score,
			Source:   "collaborative",
			Reason:   "Popular with similar viewers",
		}

		// Get media details
		if h.data != nil {
			if details, err := h.data.GetMediaDetails(ctx, s.ItemID, ""); err == nil {
				c.EntityType = details.MediaType
				c.Title = details.Title
				c.Year = details.Year
			}
		}

		candidates = append(candidates, c)
	}

	return candidates
}

// getSemanticCandidates gets content-based similarity recommendations.
func (h *HybridScorer) getSemanticCandidates(ctx context.Context, userID string, exclude, seen map[int64]bool, limit int) []scoredCandidate {
	if h.userEmbeddingService == nil {
		return nil
	}

	items, err := h.userEmbeddingService.GetUserTasteRecommendations(ctx, userID, exclude, limit)
	if err != nil {
		h.logger.Debug("semantic candidates failed", "error", err)
		return nil
	}

	candidates := make([]scoredCandidate, 0, len(items))
	for i, item := range items {
		if seen[item.EntityID] {
			continue
		}

		c := scoredCandidate{
			EntityType: item.EntityType,
			EntityID:   item.EntityID,
			Title:      item.Title,
			Year:       item.Year,
			Score:      1.0 - float32(i)*0.05, // Decay by position
			Source:     "semantic",
			Reason:     "Matches your taste",
		}
		candidates = append(candidates, c)
	}

	return candidates
}

// getExplorationCandidates gets discovery items from different genres.
func (h *HybridScorer) getExplorationCandidates(ctx context.Context, userID string, exclude, seen map[int64]bool, limit int) []scoredCandidate {
	if h.data == nil {
		return nil
	}

	// Get user's preferred genres to explore adjacent ones
	preferredGenres := h.getUserPreferredGenres(ctx, userID)

	// Exploration genres: mix of user's genres + random popular genres
	explorationGenres := []string{"Action", "Drama", "Comedy", "Thriller", "Sci-Fi", "Documentary"}
	for _, g := range preferredGenres {
		// Add genres user hasn't explored much
		found := false
		for _, eg := range explorationGenres {
			if eg == g {
				found = true
				break
			}
		}
		if !found {
			explorationGenres = append(explorationGenres, g)
		}
	}

	// Shuffle for variety
	rand.Shuffle(len(explorationGenres), func(i, j int) {
		explorationGenres[i], explorationGenres[j] = explorationGenres[j], explorationGenres[i]
	})

	var candidates []scoredCandidate
	excludeIDs := make([]int64, 0, len(exclude)+len(seen))
	for id := range exclude {
		excludeIDs = append(excludeIDs, id)
	}
	for id := range seen {
		excludeIDs = append(excludeIDs, id)
	}

	// Get items from various genres
	for _, genre := range explorationGenres {
		if len(candidates) >= limit {
			break
		}

		// Try movies first, then TV shows
		for _, mediaType := range []string{"movie", "tv_show"} {
			items, err := h.data.ListMediaByGenre(ctx, mediaType, genre, 0, excludeIDs, 5)
			if err != nil {
				continue
			}

			for _, item := range items {
				if seen[item.ID] || exclude[item.ID] {
					continue
				}

				candidates = append(candidates, scoredCandidate{
					EntityType: item.MediaType,
					EntityID:   item.ID,
					Title:      item.Title,
					Year:       item.Year,
					Score:      0.5 + rand.Float32()*0.3, // Random score 0.5-0.8
					Source:     "exploration",
					Reason:     "Discover something new",
				})
				excludeIDs = append(excludeIDs, item.ID)

				if len(candidates) >= limit {
					break
				}
			}
		}
	}

	return candidates
}

// getUserPreferredGenres returns the user's most-watched genres.
func (h *HybridScorer) getUserPreferredGenres(ctx context.Context, userID string) []string {
	genreCounts := make(map[string]int)

	// Count genres from ratings
	if h.ratings != nil {
		likedIDs, _ := h.ratings.GetPositivelyRatedIDs(ctx, userID, "", 50)
		for _, id := range likedIDs {
			if h.data != nil {
				if details, err := h.data.GetMediaDetails(ctx, id, ""); err == nil {
					for _, g := range details.Genres {
						genreCounts[g]++
					}
				}
			}
		}
	}

	// Sort by count
	type genreCount struct {
		genre string
		count int
	}
	var sorted []genreCount
	for g, c := range genreCounts {
		sorted = append(sorted, genreCount{g, c})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})

	// Return top genres
	result := make([]string, 0, 5)
	for i, gc := range sorted {
		if i >= 5 {
			break
		}
		result = append(result, gc.genre)
	}

	return result
}

// rankCandidates scores and ranks all candidates.
func (h *HybridScorer) rankCandidates(candidates []scoredCandidate, weights HybridWeights, limit int) []scoredCandidate {
	if len(candidates) == 0 {
		return nil
	}

	// Apply source-specific weight multipliers
	for i := range candidates {
		switch candidates[i].Source {
		case "collaborative":
			candidates[i].Score *= float32(weights.Collaborative) / 100.0
		case "semantic":
			candidates[i].Score *= float32(weights.Semantic) / 100.0
		case "exploration":
			candidates[i].Score *= float32(weights.Exploration) / 100.0
		}
	}

	// Sort by score descending
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	// Limit results
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	// Shuffle slightly for variety (swap adjacent items with 30% probability)
	for i := 1; i < len(candidates)-1; i++ {
		if rand.Float32() < 0.3 {
			candidates[i], candidates[i+1] = candidates[i+1], candidates[i]
		}
	}

	return candidates
}

// getExplorationItems returns items for cold start users.
func (h *HybridScorer) getExplorationItems(ctx context.Context, exclude map[int64]bool, limit int) ([]sdk.MediaItem, error) {
	candidates := h.getExplorationCandidates(ctx, "", exclude, nil, limit*2)

	// Sort by score and limit
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	result := make([]sdk.MediaItem, 0, len(candidates))
	for _, c := range candidates {
		result = append(result, sdk.MediaItem{
			EntityType: c.EntityType,
			EntityID:   c.EntityID,
			Title:      c.Title,
			Year:       c.Year,
			Reason:     "Popular in your library",
		})
	}

	return result, nil
}
