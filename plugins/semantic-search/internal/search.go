package internal

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

// SearchService handles semantic search operations.
type SearchService struct {
	embeddingService *EmbeddingService
	vector           *sdk.VectorClient
	defaultLimit     int
	maxLimit         int
	minSimilarity    float32
	logger           *slog.Logger
	rankingConfig    *RankingConfig
}

// NewSearchService creates a new search service.
func NewSearchService(
	embeddingService *EmbeddingService,
	vector *sdk.VectorClient,
	config SearchConfig,
	logger *slog.Logger,
) *SearchService {
	defaultLimit := config.DefaultLimit
	if defaultLimit <= 0 {
		defaultLimit = 20
	}
	maxLimit := config.MaxLimit
	if maxLimit <= 0 {
		maxLimit = 100
	}
	minSimilarity := config.MinSimilarity
	if minSimilarity <= 0 {
		minSimilarity = 0.3
	}

	return &SearchService{
		embeddingService: embeddingService,
		vector:           vector,
		defaultLimit:     defaultLimit,
		maxLimit:         maxLimit,
		minSimilarity:    minSimilarity,
		logger:           logger,
		rankingConfig:    nil, // Set via SetRankingConfig
	}
}

// SetRankingConfig sets the ranking configuration for the search service.
func (s *SearchService) SetRankingConfig(config *RankingConfig) {
	s.rankingConfig = config
}

// getBoosts returns the boost weights, falling back to defaults if not configured.
func (s *SearchService) getBoosts() *BoostWeights {
	if s.rankingConfig != nil && s.rankingConfig.Boosts != nil {
		return &s.rankingConfig.Boosts.Boosts
	}
	defaults := getDefaultBoostConfig()
	return &defaults.Boosts
}

// getQualityConfig returns the quality boost config, falling back to defaults if not configured.
func (s *SearchService) getQualityConfig() *QualityBoost {
	if s.rankingConfig != nil && s.rankingConfig.Boosts != nil {
		return &s.rankingConfig.Boosts.Quality
	}
	defaults := getDefaultBoostConfig()
	return &defaults.Quality
}

// getDiversityConfig returns the diversity config, falling back to defaults if not configured.
func (s *SearchService) getDiversityConfig() *Diversity {
	if s.rankingConfig != nil && s.rankingConfig.Boosts != nil {
		return &s.rankingConfig.Boosts.Diversity
	}
	defaults := getDefaultBoostConfig()
	return &defaults.Diversity
}

// getKnownStudios returns the set of known studios for intent detection.
func (s *SearchService) getKnownStudios() map[string]bool {
	if s.rankingConfig != nil && s.rankingConfig.Studios != nil {
		return s.rankingConfig.Studios
	}
	return studiosToMap(getDefaultStudios())
}

// getLanguageMap returns the language name mappings for intent detection.
func (s *SearchService) getLanguageMap() map[string]string {
	if s.rankingConfig != nil && s.rankingConfig.Languages != nil {
		return s.rankingConfig.Languages
	}
	return getDefaultLanguages()
}

// SearchParams defines parameters for semantic search.
type SearchParams struct {
	Query               string
	EntityTypes         []EntityType
	Limit               int
	ExcludeIntents      []string             // Intent types to exclude from results (e.g., "decade", "genre")
	PlaybackConstraints *PlaybackConstraints // Optional playback filters (4K, HDR, subtitles, etc.)
}

// Search performs semantic search using the query text.
func (s *SearchService) Search(ctx context.Context, params SearchParams) ([]SearchResult, error) {
	if s.embeddingService == nil || s.vector == nil {
		return nil, fmt.Errorf("search service not properly initialized")
	}

	// Detect query intent early to determine search strategy
	intent := detectQueryIntent(params.Query)

	// Handle "similar to" queries by finding the source movie and using FindSimilar
	if intent.isSimilarSearch && intent.similarToTitle != "" {
		s.logger.Info("detected 'similar to' query",
			"query", params.Query,
			"extractedTitle", intent.similarToTitle)
		results, err := s.handleSimilarToSearch(ctx, intent.similarToTitle, params)
		if err != nil {
			// Log but fall back to regular search if we can't find the movie
			s.logger.Warn("similar-to search failed, falling back to semantic search",
				"title", intent.similarToTitle, "error", err)
		} else if len(results) > 0 {
			s.logger.Info("similar-to search succeeded",
				"title", intent.similarToTitle,
				"resultCount", len(results))
			return results, nil
		} else {
			s.logger.Warn("similar-to search returned no results, falling back to semantic search",
				"title", intent.similarToTitle)
		}
	}

	// Generate embedding for the query (with caching for repeated queries)
	queryEmbedding, err := s.embeddingService.EmbedSingleCached(ctx, params.Query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	// Apply limits
	limit := params.Limit
	if limit <= 0 {
		limit = s.defaultLimit
	}
	if limit > s.maxLimit {
		limit = s.maxLimit
	}

	// Convert entity types to strings
	entityTypeStrs := make([]string, len(params.EntityTypes))
	for i, et := range params.EntityTypes {
		entityTypeStrs[i] = string(et)
	}

	// For hybrid search, fetch more results initially so we can re-rank
	fetchLimit := limit * 5
	if fetchLimit > 200 {
		fetchLimit = 200
	}

	// Determine minimum similarity - lower for person/studio/collection searches
	// to ensure we can find and boost relevant results
	minSim := s.minSimilarity
	if intent.isPersonSearch || intent.isStudioSearch || intent.isDirectorSearch ||
		intent.isActorSearch || intent.isWriterSearch || intent.isProducerSearch ||
		intent.isCollectionSearch || intent.isComposerSearch || intent.isCinematographerSearch {
		minSim = 0.15 // Lower threshold for person/studio/collection/crew searches
	}

	// Search using the vector client
	searchResp, err := s.vector.Search(ctx, sdk.VectorSearchRequest{
		QueryVector:   queryEmbedding,
		EntityTypes:   entityTypeStrs,
		Limit:         fetchLimit,
		MinSimilarity: minSim,
	})
	if err != nil {
		return nil, fmt.Errorf("search embeddings: %w", err)
	}

	s.logger.Info("vector search completed",
		"query", params.Query,
		"minSimilarity", minSim,
		"resultCount", len(searchResp.Results))

	// Log top results for debugging
	for i, r := range searchResp.Results {
		if i >= 3 {
			break
		}
		s.logger.Debug("vector search result",
			"index", i,
			"entityID", r.EntityID,
			"similarity", r.Similarity)
	}

	// Convert results to map for merging
	resultMap := make(map[string]SearchResult)
	for _, r := range searchResp.Results {
		key := fmt.Sprintf("%s:%d", r.EntityType, r.EntityID)
		resultMap[key] = SearchResult{
			EntityType: EntityType(r.EntityType),
			EntityID:   r.EntityID,
			Similarity: r.Similarity,
			Text:       r.Text,
		}
	}

	// For person/studio searches, also do a text search to find exact matches
	// This ensures movies containing the person/studio are included even if
	// semantic similarity is low
	if searchTerm := s.getTextSearchTerm(intent); searchTerm != "" {
		textResp, err := s.vector.SearchText(ctx, searchTerm, entityTypeStrs, 100)
		if err != nil {
			s.logger.Warn("text search failed, continuing with semantic only", "error", err)
		} else {
			// Merge text search results - give them a base similarity score
			for _, r := range textResp.Results {
				key := fmt.Sprintf("%s:%d", r.EntityType, r.EntityID)
				if existing, found := resultMap[key]; found {
					// Result already in map - keep the higher similarity
					if r.Similarity > existing.Similarity {
						resultMap[key] = SearchResult{
							EntityType: EntityType(r.EntityType),
							EntityID:   r.EntityID,
							Similarity: r.Similarity,
							Text:       r.Text,
						}
					}
				} else {
					// New result from text search - give it a base score of 0.5
					// (will be boosted further by applyKeywordBoost)
					resultMap[key] = SearchResult{
						EntityType: EntityType(r.EntityType),
						EntityID:   r.EntityID,
						Similarity: 0.5, // Base score for text matches
						Text:       r.Text,
					}
				}
			}
			s.logger.Debug("text search found results",
				"term", searchTerm,
				"count", len(textResp.Results),
				"total_after_merge", len(resultMap))
		}
	}

	// Convert map back to slice
	results := make([]SearchResult, 0, len(resultMap))
	for _, r := range resultMap {
		results = append(results, r)
	}

	// Apply hybrid boosting: boost results that match query keywords in text
	queryLower := strings.ToLower(params.Query)
	results = s.applyKeywordBoost(results, params.Query, queryLower)

	// Deduplicate by title (handles cases where same movie has multiple media entries)
	results = s.deduplicateByTitle(results)

	// Re-sort by boosted similarity
	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	// Apply diversity penalty to avoid too many similar results
	results = s.applyDiversityPenalty(results, limit)

	// FINAL STAGE: Apply quality boost (with guardrails)
	// This must be the last ranking stage before final sort.
	// Quality boost is capped at ±15% and uses multiplicative scaling,
	// ensuring it can only shuffle within similarity tiers, not override relevance.
	results = s.applyQualityBoost(results)

	// Apply playback constraints if specified (4K, HDR, subtitles, etc.)
	// Use explicit constraints from params, or fall back to detected constraints from query
	playbackConstraints := params.PlaybackConstraints
	if (playbackConstraints == nil || playbackConstraints.IsEmpty()) && intent.isPlaybackSearch {
		playbackConstraints = intent.playbackConstraints
	}
	if playbackConstraints != nil && !playbackConstraints.IsEmpty() {
		results = s.applyPlaybackFilters(ctx, results, playbackConstraints)
	}

	// Final sort after all adjustments
	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	// Apply final limit
	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// Explain runs the search pipeline and returns detailed breakdown of how the query was processed.
// This is used for debugging search behavior via the /explain endpoint.
func (s *SearchService) Explain(ctx context.Context, query string, limit int) (*ExplainResult, error) {
	startTime := timeNow()

	if s.embeddingService == nil || s.vector == nil {
		return nil, fmt.Errorf("search service not properly initialized")
	}

	result := &ExplainResult{
		Query:           query,
		NormalizedQuery: query,
		DetectedIntents: make(map[string]interface{}),
		BoostsApplied:   []BoostExplain{},
		TopResults:      []ResultExplain{},
	}

	// Detect query intent
	intent := detectQueryIntent(query)

	// Build detected intents map
	if intent.isSimilarSearch {
		result.DetectedIntents["similar_to"] = map[string]interface{}{
			"title": intent.similarToTitle,
		}
		result.SearchMode = "similar_to"
	} else {
		result.SearchMode = "semantic"
	}

	if intent.isDirectorSearch && intent.directorName != "" {
		result.DetectedIntents["director"] = map[string]interface{}{
			"name": intent.directorName,
		}
		result.SearchMode = "semantic_with_filters"
	}
	if intent.isActorSearch && intent.actorName != "" {
		result.DetectedIntents["actor"] = map[string]interface{}{
			"name": intent.actorName,
		}
		result.SearchMode = "semantic_with_filters"
	}
	if intent.isWriterSearch && intent.writerName != "" {
		result.DetectedIntents["writer"] = map[string]interface{}{
			"name": intent.writerName,
		}
		result.SearchMode = "semantic_with_filters"
	}
	if intent.isProducerSearch && intent.producerName != "" {
		result.DetectedIntents["producer"] = map[string]interface{}{
			"name": intent.producerName,
		}
		result.SearchMode = "semantic_with_filters"
	}
	if intent.isStudioSearch && intent.studioName != "" {
		result.DetectedIntents["studio"] = map[string]interface{}{
			"name": intent.studioName,
		}
		result.SearchMode = "semantic_with_filters"
	}
	if intent.isPersonSearch && intent.personName != "" {
		result.DetectedIntents["person"] = map[string]interface{}{
			"name": intent.personName,
			"type": "director_or_actor",
		}
		result.SearchMode = "semantic_with_filters"
	}
	if intent.isLanguageSearch && intent.languageName != "" {
		result.DetectedIntents["language"] = map[string]interface{}{
			"name": intent.languageName,
		}
		result.SearchMode = "semantic_with_filters"
	}
	if intent.isCollectionSearch && intent.collectionName != "" {
		result.DetectedIntents["collection"] = map[string]interface{}{
			"name":                intent.collectionName,
			"wants_chronological": intent.wantsChronological,
		}
		result.SearchMode = "semantic_with_filters"
	}
	if intent.isComposerSearch && intent.composerName != "" {
		result.DetectedIntents["composer"] = map[string]interface{}{
			"name": intent.composerName,
		}
		result.SearchMode = "semantic_with_filters"
	}
	if intent.isCinematographerSearch && intent.cinematographerName != "" {
		result.DetectedIntents["cinematographer"] = map[string]interface{}{
			"name": intent.cinematographerName,
		}
		result.SearchMode = "semantic_with_filters"
	}
	if intent.isGenreSearch {
		result.DetectedIntents["genre"] = map[string]interface{}{
			"detected": true,
		}
	}

	// Check for decade patterns in query
	decadeInfo := extractDecadeFromQuery(query)
	if decadeInfo != nil {
		result.DetectedIntents["decade"] = decadeInfo
		result.SearchMode = "semantic_with_filters"
	}

	// Apply limits
	if limit <= 0 {
		limit = s.defaultLimit
	}
	if limit > s.maxLimit {
		limit = s.maxLimit
	}

	// Generate embedding (check cache status)
	queryEmbedding, err := s.embeddingService.EmbedSingleCached(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	// Check if embedding was cached (we can infer this from timing, but for now assume not cached)
	result.EmbeddingCached = s.embeddingService.IsCached(query)

	// Determine minimum similarity
	minSim := s.minSimilarity
	if intent.isPersonSearch || intent.isStudioSearch || intent.isDirectorSearch ||
		intent.isActorSearch || intent.isWriterSearch || intent.isProducerSearch ||
		intent.isCollectionSearch || intent.isComposerSearch || intent.isCinematographerSearch {
		minSim = 0.15
	}

	// Fetch more results for re-ranking
	fetchLimit := limit * 5
	if fetchLimit > 200 {
		fetchLimit = 200
	}

	// Vector search
	searchResp, err := s.vector.Search(ctx, sdk.VectorSearchRequest{
		QueryVector:   queryEmbedding,
		EntityTypes:   []string{string(EntityMovie), string(EntityTVShow)},
		Limit:         fetchLimit,
		MinSimilarity: minSim,
	})
	if err != nil {
		return nil, fmt.Errorf("search embeddings: %w", err)
	}

	result.VectorSearch = &VectorSearchExplain{
		MinSimilarity:       minSim,
		ResultsBeforeFilter: len(searchResp.Results),
	}

	// Convert to result map and track base similarities
	resultMap := make(map[string]SearchResult)
	baseSimilarities := make(map[string]float32)
	for _, r := range searchResp.Results {
		key := fmt.Sprintf("%s:%d", r.EntityType, r.EntityID)
		resultMap[key] = SearchResult{
			EntityType: EntityType(r.EntityType),
			EntityID:   r.EntityID,
			Similarity: r.Similarity,
			Text:       r.Text,
		}
		baseSimilarities[key] = r.Similarity
	}

	// Text search for person/studio queries
	if searchTerm := s.getTextSearchTerm(intent); searchTerm != "" {
		textResp, err := s.vector.SearchText(ctx, searchTerm, []string{string(EntityMovie), string(EntityTVShow)}, 100)
		if err == nil {
			for _, r := range textResp.Results {
				key := fmt.Sprintf("%s:%d", r.EntityType, r.EntityID)
				if existing, found := resultMap[key]; found {
					if r.Similarity > existing.Similarity {
						resultMap[key] = SearchResult{
							EntityType: EntityType(r.EntityType),
							EntityID:   r.EntityID,
							Similarity: r.Similarity,
							Text:       r.Text,
						}
						baseSimilarities[key] = r.Similarity
					}
				} else {
					resultMap[key] = SearchResult{
						EntityType: EntityType(r.EntityType),
						EntityID:   r.EntityID,
						Similarity: 0.5,
						Text:       r.Text,
					}
					baseSimilarities[key] = 0.5
				}
			}
		}
	}

	result.VectorSearch.ResultsAfterFilter = len(resultMap)

	// Convert map to slice
	results := make([]SearchResult, 0, len(resultMap))
	for _, r := range resultMap {
		results = append(results, r)
	}

	// Apply keyword boost and track boost breakdown per result
	queryLower := strings.ToLower(query)
	boostBreakdowns := make(map[string]map[string]float32)
	results, boostBreakdowns = s.applyKeywordBoostWithExplain(results, query, queryLower, baseSimilarities)

	// Aggregate boost statistics
	boostCounts := make(map[string]int)
	boostTotals := make(map[string]float32)
	boostTargets := make(map[string]string)

	for _, breakdown := range boostBreakdowns {
		for boostType, value := range breakdown {
			if value != 0 {
				boostCounts[boostType]++
				boostTotals[boostType] += value
			}
		}
	}

	// Set boost targets based on intent
	if intent.isDirectorSearch && intent.directorName != "" {
		boostTargets["director_match"] = intent.directorName
	}
	if intent.isActorSearch && intent.actorName != "" {
		boostTargets["actor_match"] = intent.actorName
	}
	if intent.isPersonSearch && intent.personName != "" {
		boostTargets["person_match"] = intent.personName
	}
	if intent.isStudioSearch && intent.studioName != "" {
		boostTargets["studio_match"] = intent.studioName
	}
	if intent.isLanguageSearch && intent.languageName != "" {
		boostTargets["language_match"] = intent.languageName
	}

	// Build boosts applied list
	for boostType, count := range boostCounts {
		if count > 0 {
			avgBoost := boostTotals[boostType] / float32(count)
			result.BoostsApplied = append(result.BoostsApplied, BoostExplain{
				Type:    boostType,
				Target:  boostTargets[boostType],
				Boost:   avgBoost,
				Matches: count,
			})
		}
	}

	// Deduplicate
	results = s.deduplicateByTitle(results)

	// Sort by similarity
	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	// Apply diversity penalty
	results = s.applyDiversityPenalty(results, limit)

	// Track similarity before quality boost for explain output
	preQualitySimilarities := make(map[string]float32)
	for _, r := range results {
		key := fmt.Sprintf("%s:%d", r.EntityType, r.EntityID)
		preQualitySimilarities[key] = r.Similarity
	}

	// FINAL STAGE: Apply quality boost (with guardrails)
	results = s.applyQualityBoost(results)

	// Track quality boost amounts
	qualityBoostCount := 0
	for _, r := range results {
		key := fmt.Sprintf("%s:%d", r.EntityType, r.EntityID)
		if r.Similarity != preQualitySimilarities[key] {
			qualityBoostCount++
			// Add quality boost to breakdown
			if boostBreakdowns[key] == nil {
				boostBreakdowns[key] = make(map[string]float32)
			}
			boostBreakdowns[key]["quality_boost"] = r.Similarity - preQualitySimilarities[key]
		}
	}
	if qualityBoostCount > 0 {
		result.BoostsApplied = append(result.BoostsApplied, BoostExplain{
			Type:    "quality_boost",
			Target:  "rating/votes",
			Boost:   0.15, // max possible boost
			Matches: qualityBoostCount,
		})
	}

	// Final sort
	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	// Apply limit
	if len(results) > limit {
		results = results[:limit]
	}

	result.FinalResults = len(results)

	// Build top results with breakdown
	for _, r := range results {
		key := fmt.Sprintf("%s:%d", r.EntityType, r.EntityID)
		title, year := extractTitleAndYear(r.Text)

		resultExplain := ResultExplain{
			Title:             title,
			Year:              year,
			BaseSimilarity:    baseSimilarities[key],
			BoostedSimilarity: r.Similarity,
			BoostBreakdown:    boostBreakdowns[key],
		}
		if resultExplain.BoostBreakdown == nil {
			resultExplain.BoostBreakdown = make(map[string]float32)
		}
		result.TopResults = append(result.TopResults, resultExplain)
	}

	result.TookMs = timeNow().Sub(startTime).Milliseconds()

	return result, nil
}

// applyKeywordBoostWithExplain is like applyKeywordBoost but also returns boost breakdown per result.
func (s *SearchService) applyKeywordBoostWithExplain(
	results []SearchResult,
	queryOriginal, queryLower string,
	baseSimilarities map[string]float32,
) ([]SearchResult, map[string]map[string]float32) {
	boostBreakdowns := make(map[string]map[string]float32)

	// Get configurable boost weights
	boosts := s.getBoosts()

	queryTerms := extractSearchTerms(queryLower)
	negativeTerms := extractNegativeTerms(queryLower)
	queryGenres := extractGenresFromQuery(queryLower)
	negativeGenres := extractNegativeGenres(queryLower)
	intent := detectQueryIntentWithConfig(queryOriginal, s.getKnownStudios(), s.getLanguageMap())

	hasStrongIntent := intent.isDirectorSearch || intent.isActorSearch ||
		intent.isWriterSearch || intent.isProducerSearch ||
		intent.isStudioSearch || intent.isPersonSearch || intent.isLanguageSearch ||
		intent.isCollectionSearch || intent.isComposerSearch || intent.isCinematographerSearch

	for i := range results {
		key := fmt.Sprintf("%s:%d", results[i].EntityType, results[i].EntityID)
		breakdown := make(map[string]float32)
		textLower := strings.ToLower(results[i].Text)
		boost := float32(0.0)
		penalty := float32(0.0)

		// Director boost
		if intent.isDirectorSearch && intent.directorName != "" {
			directorLine := extractLine(textLower, "directed by:")
			if strings.Contains(directorLine, intent.directorName) {
				boost += boosts.DirectorMatch
				breakdown["director_match"] = boosts.DirectorMatch
			} else {
				penalty += boosts.DirectorMismatchPenalty
				breakdown["director_mismatch"] = -boosts.DirectorMismatchPenalty
			}
		}

		// Actor boost
		if intent.isActorSearch && intent.actorName != "" {
			castLine := extractLine(textLower, "cast:")
			if strings.Contains(castLine, intent.actorName) {
				boost += boosts.ActorMatch
				breakdown["actor_match"] = boosts.ActorMatch
			} else {
				penalty += boosts.ActorMismatchPenalty
				breakdown["actor_mismatch"] = -boosts.ActorMismatchPenalty
			}
		}

		// Writer boost
		if intent.isWriterSearch && intent.writerName != "" {
			writerLine := extractLine(textLower, "written by:")
			if strings.Contains(writerLine, intent.writerName) {
				boost += boosts.WriterMatch
				breakdown["writer_match"] = boosts.WriterMatch
			} else {
				penalty += boosts.WriterMismatchPenalty
				breakdown["writer_mismatch"] = -boosts.WriterMismatchPenalty
			}
		}

		// Producer boost
		if intent.isProducerSearch && intent.producerName != "" {
			producerLine := extractLine(textLower, "produced by:")
			if strings.Contains(producerLine, intent.producerName) {
				boost += boosts.ProducerMatch
				breakdown["producer_match"] = boosts.ProducerMatch
			} else {
				penalty += boosts.ProducerMismatchPenalty
				breakdown["producer_mismatch"] = -boosts.ProducerMismatchPenalty
			}
		}

		// Studio boost
		if intent.isStudioSearch && intent.studioName != "" {
			studioLine := extractLine(textLower, "studios:")
			if studioLine == "" {
				studioLine = extractLine(textLower, "network:")
			}
			if strings.Contains(studioLine, intent.studioName) {
				boost += boosts.StudioMatch
				breakdown["studio_match"] = boosts.StudioMatch
			} else {
				penalty += boosts.StudioMismatchPenalty
				breakdown["studio_mismatch"] = -boosts.StudioMismatchPenalty
			}
		}

		// Person search boost
		if intent.isPersonSearch && intent.personName != "" {
			directorMatch := strings.Contains(extractLine(textLower, "directed by:"), intent.personName)
			writerMatch := strings.Contains(extractLine(textLower, "written by:"), intent.personName)
			producerMatch := strings.Contains(extractLine(textLower, "produced by:"), intent.personName)
			castMatch := strings.Contains(extractLine(textLower, "cast:"), intent.personName)
			studioMatch := strings.Contains(extractLine(textLower, "studios:"), intent.personName)

			if directorMatch {
				boost += boosts.PersonDirectorMatch
				breakdown["person_director"] = boosts.PersonDirectorMatch
			}
			if writerMatch {
				boost += boosts.PersonWriterMatch
				breakdown["person_writer"] = boosts.PersonWriterMatch
			}
			if studioMatch {
				boost += boosts.PersonStudioMatch
				breakdown["person_studio"] = boosts.PersonStudioMatch
			}
			if producerMatch && !directorMatch && !writerMatch {
				boost += boosts.PersonProducerMatch
				breakdown["person_producer"] = boosts.PersonProducerMatch
			}
			if castMatch && !directorMatch && !writerMatch {
				boost += boosts.PersonCastMatch
				penalty += boosts.PersonCastPenalty
				breakdown["person_cast"] = boosts.PersonCastMatch - boosts.PersonCastPenalty // net effect
			}

			foundInPeople := directorMatch || writerMatch || producerMatch || castMatch || studioMatch
			if !foundInPeople {
				penalty += boosts.PersonNotFoundPenalty
				breakdown["person_not_found"] = -boosts.PersonNotFoundPenalty
			}
		}

		// Language boost
		if intent.isLanguageSearch && intent.languageName != "" {
			languageLine := extractLine(textLower, "language:")
			countryLine := extractLine(textLower, "country:")
			typeLine := extractLine(textLower, "type:")

			matchedLanguage := strings.Contains(languageLine, intent.languageName) ||
				strings.Contains(countryLine, intent.languageName) ||
				strings.Contains(typeLine, intent.languageName)

			if matchedLanguage {
				boost += boosts.LanguageMatch
				breakdown["language_match"] = boosts.LanguageMatch
			} else {
				penalty += boosts.LanguageMismatchPenalty
				breakdown["language_mismatch"] = -boosts.LanguageMismatchPenalty
			}
		}

		// Composer boost (music by, score by, composed by)
		if intent.isComposerSearch && intent.composerName != "" {
			composerLine := extractLine(textLower, "music by:")
			if strings.Contains(composerLine, intent.composerName) {
				boost += boosts.ComposerMatch
				breakdown["composer_match"] = boosts.ComposerMatch
			} else {
				penalty += boosts.ComposerMismatchPenalty
				breakdown["composer_mismatch"] = -boosts.ComposerMismatchPenalty
			}
		}

		// Cinematographer boost (cinematography by, shot by, dp)
		if intent.isCinematographerSearch && intent.cinematographerName != "" {
			cinematographerLine := extractLine(textLower, "cinematography by:")
			if strings.Contains(cinematographerLine, intent.cinematographerName) {
				boost += boosts.CinematographerMatch
				breakdown["cinematographer_match"] = boosts.CinematographerMatch
			} else {
				penalty += boosts.CinematographerMismatchPenalty
				breakdown["cinematographer_mismatch"] = -boosts.CinematographerMismatchPenalty
			}
		}

		// Collection/franchise boost
		if intent.isCollectionSearch && intent.collectionName != "" {
			titleLine := extractLine(textLower, "title:")
			collectionLine := extractLine(textLower, "collection:")

			titleMatch := strings.Contains(titleLine, intent.collectionName)
			collectionMatch := strings.Contains(collectionLine, intent.collectionName)

			collectionWords := strings.Fields(intent.collectionName)
			wordMatchCount := 0
			for _, word := range collectionWords {
				if len(word) >= 3 && strings.Contains(titleLine, word) {
					wordMatchCount++
				}
			}
			partialMatch := len(collectionWords) > 0 && wordMatchCount >= len(collectionWords)/2+1

			if titleMatch || collectionMatch {
				boost += 0.60
				breakdown["collection_match"] = 0.60
			} else if partialMatch {
				boost += 0.40
				breakdown["collection_partial"] = 0.40
			} else {
				penalty += 0.50
				breakdown["collection_mismatch"] = -0.50
			}
		}

		// Keyword term boost
		matchCount := 0
		termBoost := float32(0.0)
		for _, term := range queryTerms {
			if strings.Contains(textLower, term) {
				matchCount++
				if strings.Contains(extractLine(textLower, "title:"), term) {
					termBoost += 0.15
				} else if strings.Contains(extractLine(textLower, "directed by:"), term) {
					termBoost += 0.12
				} else if strings.Contains(extractLine(textLower, "written by:"), term) {
					termBoost += 0.10
				} else if strings.Contains(extractLine(textLower, "produced by:"), term) {
					termBoost += 0.08
				} else if strings.Contains(extractLine(textLower, "cast:"), term) {
					termBoost += 0.10
				} else if strings.Contains(extractLine(textLower, "studios:"), term) {
					termBoost += 0.10
				} else if strings.Contains(extractLine(textLower, "setting:"), term) {
					termBoost += 0.10
				} else {
					termBoost += 0.05
				}
			}
		}
		if matchCount > 1 {
			termBoost += float32(matchCount-1) * 0.03
		}
		if termBoost > 0 {
			boost += termBoost
			breakdown["keyword_match"] = termBoost
		}

		// Genre boost
		if len(queryGenres) > 0 {
			resultGenres := extractLine(textLower, "genre:")
			genreMatches := 0
			for _, g := range queryGenres {
				if strings.Contains(resultGenres, g) {
					genreMatches++
				}
			}
			if genreMatches > 0 {
				genreBoost := float32(genreMatches) * boosts.GenreMatch
				boost += genreBoost
				breakdown["genre_match"] = genreBoost
			} else {
				penalty += boosts.GenreMismatchPenalty
				breakdown["genre_mismatch"] = -boosts.GenreMismatchPenalty
			}
		}

		// Negative term penalty
		for _, term := range negativeTerms {
			if strings.Contains(textLower, term) {
				penalty += 0.25
				breakdown["negative_term"] = -0.25
			}
		}

		// Negative genre penalty
		if len(negativeGenres) > 0 {
			resultGenres := extractLine(textLower, "genre:")
			for _, g := range negativeGenres {
				if strings.Contains(resultGenres, g) {
					penalty += 0.40
					breakdown["negative_genre"] = -0.40
				}
			}
		}

		// Apply boost with cap
		maxBoost := float32(0.3)
		if hasStrongIntent {
			maxBoost = 0.6
		}
		if boost > maxBoost {
			boost = maxBoost
		}
		results[i].Similarity += boost - penalty

		// Cap at 1.0 and floor at 0
		if results[i].Similarity > 1.0 {
			results[i].Similarity = 1.0
		}
		if results[i].Similarity < 0 {
			results[i].Similarity = 0
		}

		boostBreakdowns[key] = breakdown
	}

	return results, boostBreakdowns
}

// extractDecadeFromQuery extracts decade information from a query string.
func extractDecadeFromQuery(query string) map[string]interface{} {
	q := strings.ToLower(query)

	// Patterns like "90s", "1990s", "nineties"
	decadePatterns := map[string]struct {
		value     string
		yearStart int
		yearEnd   int
	}{
		"50s": {"1950s", 1950, 1959}, "1950s": {"1950s", 1950, 1959}, "fifties": {"1950s", 1950, 1959},
		"60s": {"1960s", 1960, 1969}, "1960s": {"1960s", 1960, 1969}, "sixties": {"1960s", 1960, 1969},
		"70s": {"1970s", 1970, 1979}, "1970s": {"1970s", 1970, 1979}, "seventies": {"1970s", 1970, 1979},
		"80s": {"1980s", 1980, 1989}, "1980s": {"1980s", 1980, 1989}, "eighties": {"1980s", 1980, 1989},
		"90s": {"1990s", 1990, 1999}, "1990s": {"1990s", 1990, 1999}, "nineties": {"1990s", 1990, 1999},
		"00s": {"2000s", 2000, 2009}, "2000s": {"2000s", 2000, 2009},
		"2010s": {"2010s", 2010, 2019},
		"2020s": {"2020s", 2020, 2029},
	}

	for pattern, info := range decadePatterns {
		if strings.Contains(q, pattern) {
			return map[string]interface{}{
				"value":      info.value,
				"year_start": info.yearStart,
				"year_end":   info.yearEnd,
			}
		}
	}

	return nil
}

// extractTitleAndYear extracts title and year from embedding text.
func extractTitleAndYear(text string) (string, int) {
	titleLine := extractLine(strings.ToLower(text), "title:")
	if titleLine == "" {
		return "", 0
	}

	// Remove "title:" prefix
	title := strings.TrimPrefix(titleLine, "title:")
	title = strings.TrimSpace(title)

	// Extract year from parentheses
	year := 0
	if start := strings.LastIndex(title, "("); start != -1 {
		if end := strings.LastIndex(title, ")"); end > start {
			yearStr := title[start+1 : end]
			if len(yearStr) == 4 {
				fmt.Sscanf(yearStr, "%d", &year)
			}
			// Remove year from title for cleaner display
			title = strings.TrimSpace(title[:start])
		}
	}

	return title, year
}

// getTextSearchTerm returns the text search term for person/studio searches.
// Returns empty string if no text search should be performed.
func (s *SearchService) getTextSearchTerm(intent queryIntent) string {
	if intent.isDirectorSearch && intent.directorName != "" {
		return intent.directorName
	}
	if intent.isActorSearch && intent.actorName != "" {
		return intent.actorName
	}
	if intent.isWriterSearch && intent.writerName != "" {
		return intent.writerName
	}
	if intent.isProducerSearch && intent.producerName != "" {
		return intent.producerName
	}
	if intent.isStudioSearch && intent.studioName != "" {
		return intent.studioName
	}
	if intent.isPersonSearch && intent.personName != "" {
		return intent.personName
	}
	if intent.isLanguageSearch && intent.languageName != "" {
		return intent.languageName
	}
	if intent.isCollectionSearch && intent.collectionName != "" {
		return intent.collectionName
	}
	if intent.isComposerSearch && intent.composerName != "" {
		return intent.composerName
	}
	if intent.isCinematographerSearch && intent.cinematographerName != "" {
		return intent.cinematographerName
	}
	return ""
}

// detectQueryIntent identifies what type of search the user is performing.
type queryIntent struct {
	isDirectorSearch        bool
	isActorSearch           bool
	isWriterSearch          bool
	isProducerSearch        bool
	isComposerSearch        bool // Searching by composer (music by, score by)
	isCinematographerSearch bool // Searching by cinematographer (shot by, cinematography by)
	isStudioSearch          bool
	isGenreSearch           bool
	isLocationSearch        bool
	isPersonSearch          bool                 // Generic person search (name + movies)
	isLanguageSearch        bool                 // Searching by language (French films, Korean movies)
	isSimilarSearch         bool                 // "movies like X", "similar to X" - find similar to a specific title
	isCollectionSearch      bool                 // "all X movies", "X franchise", "X series"
	isPlaybackSearch        bool                 // Searching by playback constraints (4K, HDR, subtitles)
	directorName            string               // extracted director name if searching by director
	actorName               string               // extracted actor name if searching by actor
	writerName              string               // extracted writer name
	producerName            string               // extracted producer name
	composerName            string               // extracted composer name (music by X)
	cinematographerName     string               // extracted cinematographer name (shot by X)
	studioName              string               // extracted studio name
	personName              string               // generic person name (from "Name movies" pattern)
	languageName            string               // language being searched for (e.g., "french", "korean")
	similarToTitle          string               // extracted title for "similar to" searches
	collectionName          string               // extracted collection/franchise name
	wantsChronological      bool                 // "in order", "chronologically"
	playbackConstraints     *PlaybackConstraints // extracted playback constraints
}

// convertToIntentChips converts a queryIntent to user-facing IntentChips.
// These chips show users what the system understood from their query.
func (intent queryIntent) convertToIntentChips(query string) []IntentChip {
	chips := []IntentChip{}
	chipID := 0

	// Priority 1: Similar-to queries (highest semantic intent)
	if intent.isSimilarSearch && intent.similarToTitle != "" {
		chipID++
		chips = append(chips, IntentChip{
			ID:        fmt.Sprintf("chip_%d", chipID),
			Type:      "similar_to",
			Value:     intent.similarToTitle,
			Display:   fmt.Sprintf("Like \"%s\"", intent.similarToTitle),
			Removable: true,
		})
	}

	// Priority 2: Person searches (director, actor, etc.)
	if intent.isDirectorSearch && intent.directorName != "" {
		chipID++
		chips = append(chips, IntentChip{
			ID:        fmt.Sprintf("chip_%d", chipID),
			Type:      "person",
			Value:     intent.directorName,
			Display:   intent.directorName,
			Role:      "director",
			Removable: true,
		})
	}
	if intent.isActorSearch && intent.actorName != "" {
		chipID++
		chips = append(chips, IntentChip{
			ID:        fmt.Sprintf("chip_%d", chipID),
			Type:      "person",
			Value:     intent.actorName,
			Display:   intent.actorName,
			Role:      "actor",
			Removable: true,
		})
	}
	if intent.isWriterSearch && intent.writerName != "" {
		chipID++
		chips = append(chips, IntentChip{
			ID:        fmt.Sprintf("chip_%d", chipID),
			Type:      "person",
			Value:     intent.writerName,
			Display:   intent.writerName,
			Role:      "writer",
			Removable: true,
		})
	}
	if intent.isProducerSearch && intent.producerName != "" {
		chipID++
		chips = append(chips, IntentChip{
			ID:        fmt.Sprintf("chip_%d", chipID),
			Type:      "person",
			Value:     intent.producerName,
			Display:   intent.producerName,
			Role:      "producer",
			Removable: true,
		})
	}
	if intent.isComposerSearch && intent.composerName != "" {
		chipID++
		chips = append(chips, IntentChip{
			ID:        fmt.Sprintf("chip_%d", chipID),
			Type:      "person",
			Value:     intent.composerName,
			Display:   intent.composerName,
			Role:      "composer",
			Removable: true,
		})
	}
	if intent.isCinematographerSearch && intent.cinematographerName != "" {
		chipID++
		chips = append(chips, IntentChip{
			ID:        fmt.Sprintf("chip_%d", chipID),
			Type:      "person",
			Value:     intent.cinematographerName,
			Display:   intent.cinematographerName,
			Role:      "cinematographer",
			Removable: true,
		})
	}
	if intent.isPersonSearch && intent.personName != "" {
		chipID++
		chips = append(chips, IntentChip{
			ID:        fmt.Sprintf("chip_%d", chipID),
			Type:      "person",
			Value:     intent.personName,
			Display:   intent.personName,
			Role:      "",
			Removable: true,
		})
	}

	// Priority 3: Structural filters (genre, decade, studio, language)
	// Extract decade from query
	decadeInfo := extractDecadeFromQuery(query)
	if decade, ok := decadeInfo["value"].(string); ok && decade != "" {
		chipID++
		// Extract display format (e.g., "90s" from "1990s")
		displayDecade := decade
		if strings.HasSuffix(decade, "s") && len(decade) == 5 {
			// "1990s" -> "90s"
			displayDecade = decade[2:]
		}
		// Get adjacent decades for refinements
		adjacentDecades := []string{}
		if strings.HasSuffix(decade, "s") {
			// Parse the decade start year
			baseYear := 0
			fmt.Sscanf(decade, "%d", &baseYear)
			if baseYear > 0 {
				if baseYear >= 1960 {
					prevDecade := baseYear - 10
					adjacentDecades = append(adjacentDecades, fmt.Sprintf("%ds", prevDecade%100))
				}
				if baseYear <= 2010 {
					nextDecade := baseYear + 10
					adjacentDecades = append(adjacentDecades, fmt.Sprintf("%ds", nextDecade%100))
				}
			}
		}
		chips = append(chips, IntentChip{
			ID:          fmt.Sprintf("chip_%d", chipID),
			Type:        "decade",
			Value:       decade,
			Display:     displayDecade,
			Removable:   true,
			Refinements: adjacentDecades,
		})
	}

	if intent.isStudioSearch && intent.studioName != "" {
		chipID++
		chips = append(chips, IntentChip{
			ID:        fmt.Sprintf("chip_%d", chipID),
			Type:      "studio",
			Value:     intent.studioName,
			Display:   intent.studioName,
			Removable: true,
		})
	}

	if intent.isLanguageSearch && intent.languageName != "" {
		chipID++
		chips = append(chips, IntentChip{
			ID:        fmt.Sprintf("chip_%d", chipID),
			Type:      "language",
			Value:     intent.languageName,
			Display:   intent.languageName,
			Removable: true,
		})
	}

	// Priority 4: Collection/franchise
	if intent.isCollectionSearch && intent.collectionName != "" {
		chipID++
		chips = append(chips, IntentChip{
			ID:        fmt.Sprintf("chip_%d", chipID),
			Type:      "collection",
			Value:     intent.collectionName,
			Display:   intent.collectionName,
			Removable: true,
		})
	}

	// Priority 5: Playback constraints
	if intent.isPlaybackSearch && intent.playbackConstraints != nil {
		pc := intent.playbackConstraints
		if pc.MinResolution != "" {
			chipID++
			chips = append(chips, IntentChip{
				ID:        fmt.Sprintf("chip_%d", chipID),
				Type:      "playback",
				Value:     pc.MinResolution,
				Display:   pc.MinResolution,
				Removable: true,
			})
		}
		if len(pc.HDRFormats) > 0 {
			chipID++
			chips = append(chips, IntentChip{
				ID:        fmt.Sprintf("chip_%d", chipID),
				Type:      "playback",
				Value:     strings.Join(pc.HDRFormats, ","),
				Display:   strings.Join(pc.HDRFormats, " / "),
				Removable: true,
			})
		}
		if pc.HasSubtitles != nil && *pc.HasSubtitles {
			chipID++
			chips = append(chips, IntentChip{
				ID:        fmt.Sprintf("chip_%d", chipID),
				Type:      "playback",
				Value:     "has_subtitles",
				Display:   "Has Subtitles",
				Removable: true,
			})
		}
	}

	// Extract negative terms as exclusion chips
	negativeTerms := extractNegativeTerms(query)
	for _, term := range negativeTerms {
		chipID++
		chips = append(chips, IntentChip{
			ID:        fmt.Sprintf("chip_%d", chipID),
			Type:      "exclusion",
			Value:     term,
			Display:   fmt.Sprintf("Not %s", term),
			Removable: true,
		})
	}

	return chips
}

func detectQueryIntent(query string) queryIntent {
	// Use default studios and languages for backward compatibility
	return detectQueryIntentWithConfig(query, studiosToMap(getDefaultStudios()), getDefaultLanguages())
}

// detectQueryIntentWithConfig identifies what type of search the user is performing,
// using the provided studios and languages maps for intent detection.
func detectQueryIntentWithConfig(query string, knownStudios map[string]bool, languageMap map[string]string) queryIntent {
	intent := queryIntent{}
	q := strings.ToLower(query)

	// Check for "similar to" / "movies like" patterns FIRST
	// These patterns indicate user wants movies similar to a specific title
	intent.similarToTitle, intent.isSimilarSearch = extractSimilarToTitle(q)
	if intent.isSimilarSearch {
		// If we found a "similar to" pattern, return early - this is the primary intent
		return intent
	}

	// Check for collection/franchise patterns
	// E.g., "all Mission Impossible movies", "Harry Potter in order", "Star Wars saga"
	intent.collectionName, intent.isCollectionSearch = extractCollectionName(q)
	intent.wantsChronological = detectChronologicalIntent(q)

	// Build studio patterns dynamically from known studios
	studioPatterns := []string{"from studio ", "studio "}
	for studio := range knownStudios {
		studioPatterns = append(studioPatterns, " by "+studio)
	}
	for _, p := range studioPatterns {
		if idx := strings.Index(q, p); idx != -1 {
			intent.isStudioSearch = true
			if strings.HasPrefix(p, " by ") {
				// Extract studio name from "by X" patterns
				intent.studioName = strings.TrimPrefix(p, " by ")
			} else {
				name := strings.TrimSpace(q[idx+len(p):])
				for _, suffix := range []string{" films", " movies", " film", " movie"} {
					name = strings.TrimSuffix(name, suffix)
				}
				intent.studioName = name
			}
			break
		}
	}

	// Also check for "movies by [studio]" or "films by [studio]" patterns
	if !intent.isStudioSearch {
		for _, p := range []string{"movies by ", "films by "} {
			if idx := strings.Index(q, p); idx != -1 {
				name := strings.TrimSpace(q[idx+len(p):])
				for _, suffix := range []string{" films", " movies", " film", " movie"} {
					name = strings.TrimSuffix(name, suffix)
				}
				// Check if this is a known studio name
				if knownStudios[name] {
					intent.isStudioSearch = true
					intent.studioName = name
					break
				}
			}
		}
	}

	// Director patterns - only match if not already identified as studio
	if !intent.isStudioSearch {
		directorPatterns := []string{"directed by ", "director ", "by director ", "films by ", "movies by "}
		for _, p := range directorPatterns {
			if idx := strings.Index(q, p); idx != -1 {
				intent.isDirectorSearch = true
				// Extract the name after the pattern
				name := strings.TrimSpace(q[idx+len(p):])
				// Clean up common suffixes
				for _, suffix := range []string{" films", " movies", " film", " movie"} {
					name = strings.TrimSuffix(name, suffix)
				}
				intent.directorName = name
				break
			}
		}
	}

	// Actor patterns
	actorPatterns := []string{"starring ", "with ", "featuring ", "acted by ", "played by "}
	for _, p := range actorPatterns {
		if idx := strings.Index(q, p); idx != -1 {
			intent.isActorSearch = true
			name := strings.TrimSpace(q[idx+len(p):])
			for _, suffix := range []string{" films", " movies", " film", " movie"} {
				name = strings.TrimSuffix(name, suffix)
			}
			intent.actorName = name
			break
		}
	}

	// Writer patterns
	writerPatterns := []string{"written by ", "screenplay by ", "script by ", "writer "}
	for _, p := range writerPatterns {
		if idx := strings.Index(q, p); idx != -1 {
			intent.isWriterSearch = true
			name := strings.TrimSpace(q[idx+len(p):])
			for _, suffix := range []string{" films", " movies", " film", " movie"} {
				name = strings.TrimSuffix(name, suffix)
			}
			intent.writerName = name
			break
		}
	}

	// Producer patterns
	producerPatterns := []string{"produced by ", "producer ", "from producer "}
	for _, p := range producerPatterns {
		if idx := strings.Index(q, p); idx != -1 {
			intent.isProducerSearch = true
			name := strings.TrimSpace(q[idx+len(p):])
			for _, suffix := range []string{" films", " movies", " film", " movie"} {
				name = strings.TrimSuffix(name, suffix)
			}
			intent.producerName = name
			break
		}
	}

	// Composer patterns (music by, score by, composed by)
	composerPatterns := []string{"music by ", "score by ", "composed by ", "composer "}
	for _, p := range composerPatterns {
		if idx := strings.Index(q, p); idx != -1 {
			intent.isComposerSearch = true
			name := strings.TrimSpace(q[idx+len(p):])
			for _, suffix := range []string{" films", " movies", " film", " movie"} {
				name = strings.TrimSuffix(name, suffix)
			}
			intent.composerName = name
			break
		}
	}

	// Cinematographer patterns (cinematography by, shot by, filmed by, dp)
	cinematographerPatterns := []string{"cinematography by ", "shot by ", "dp ", "director of photography "}
	for _, p := range cinematographerPatterns {
		if idx := strings.Index(q, p); idx != -1 {
			intent.isCinematographerSearch = true
			name := strings.TrimSpace(q[idx+len(p):])
			for _, suffix := range []string{" films", " movies", " film", " movie"} {
				name = strings.TrimSuffix(name, suffix)
			}
			intent.cinematographerName = name
			break
		}
	}

	// Location patterns
	locationPatterns := []string{"set in ", "filmed in ", "takes place in "}
	for _, p := range locationPatterns {
		if strings.Contains(q, p) {
			intent.isLocationSearch = true
			break
		}
	}

	// Language/nationality patterns - detect queries like "French films", "Korean movies", "Japanese anime"
	// Uses the languageMap passed as parameter for configurable language detection
	for keyword, lang := range languageMap {
		if strings.Contains(q, keyword) {
			intent.isLanguageSearch = true
			intent.languageName = lang
			break
		}
	}

	// Detect "[Name] movies/films" pattern when no other intent detected
	// E.g., "Spielberg movies", "Tom Hanks films", "Wes Anderson movies"
	if !intent.isDirectorSearch && !intent.isActorSearch && !intent.isWriterSearch &&
		!intent.isProducerSearch && !intent.isStudioSearch {
		intent.personName, intent.isPersonSearch = extractPersonFromNameMoviesPattern(query)
	}

	// Detect playback constraint patterns (4K, HDR, subtitles, etc.)
	intent.playbackConstraints, intent.isPlaybackSearch = extractPlaybackConstraints(q)

	return intent
}

// extractPlaybackConstraints extracts playback-related constraints from a query.
// Detects patterns like "4K movies", "Dolby Vision content", "movies with subtitles".
func extractPlaybackConstraints(query string) (*PlaybackConstraints, bool) {
	constraints := &PlaybackConstraints{}
	hasConstraints := false

	// Resolution patterns
	resolutionPatterns := map[string]string{
		"4k":      "4K",
		"uhd":     "4K",
		"2160p":   "4K",
		"1080p":   "1080p",
		"full hd": "1080p",
		"fhd":     "1080p",
		"720p":    "720p",
		"hd":      "720p",
		"8k":      "8K",
		"4320p":   "8K",
	}

	for pattern, resolution := range resolutionPatterns {
		if strings.Contains(query, pattern) {
			constraints.MinResolution = resolution
			hasConstraints = true
			break
		}
	}

	// HDR format patterns
	hdrPatterns := map[string]string{
		"dolby vision": "Dolby Vision",
		"dv":           "Dolby Vision",
		"hdr10+":       "HDR10+",
		"hdr10":        "HDR10",
		"hdr":          "HDR10", // Generic HDR defaults to HDR10
		"hlg":          "HLG",
	}

	for pattern, format := range hdrPatterns {
		if strings.Contains(query, pattern) {
			constraints.HDRFormats = append(constraints.HDRFormats, format)
			hasConstraints = true
		}
	}

	// Audio format patterns
	audioPatterns := map[string]string{
		"atmos":       "atmos",
		"dolby atmos": "atmos",
		"truehd":      "truehd",
		"dts-hd":      "dts-hd",
		"dts:x":       "dts:x",
		"5.1":         "5.1",
		"7.1":         "7.1",
		"surround":    "5.1",
	}

	for pattern, format := range audioPatterns {
		if strings.Contains(query, pattern) {
			constraints.AudioFormats = append(constraints.AudioFormats, format)
			hasConstraints = true
		}
	}

	// Subtitle patterns
	subtitlePatterns := []string{
		"with subtitles",
		"subtitled",
		"with subs",
		"has subtitles",
	}

	for _, pattern := range subtitlePatterns {
		if strings.Contains(query, pattern) {
			hasSubtitles := true
			constraints.HasSubtitles = &hasSubtitles
			hasConstraints = true
			break
		}
	}

	// Subtitle language patterns
	subtitleLangPatterns := map[string]string{
		"english subtitles":  "en",
		"english subs":       "en",
		"spanish subtitles":  "es",
		"spanish subs":       "es",
		"french subtitles":   "fr",
		"french subs":        "fr",
		"german subtitles":   "de",
		"german subs":        "de",
		"japanese subtitles": "ja",
		"japanese subs":      "ja",
		"korean subtitles":   "ko",
		"korean subs":        "ko",
		"chinese subtitles":  "zh",
		"chinese subs":       "zh",
	}

	for pattern, lang := range subtitleLangPatterns {
		if strings.Contains(query, pattern) {
			constraints.SubtitleLanguage = lang
			hasSubtitles := true
			constraints.HasSubtitles = &hasSubtitles
			hasConstraints = true
			break
		}
	}

	if !hasConstraints {
		return nil, false
	}

	return constraints, true
}

// extractPersonFromNameMoviesPattern extracts a person name from "Name movies" pattern
func extractPersonFromNameMoviesPattern(query string) (string, bool) {
	words := strings.Fields(query)

	// Check if query ends with movies/films
	mediaWords := map[string]bool{
		"movies": true, "films": true, "movie": true, "film": true,
	}
	if len(words) < 2 {
		return "", false
	}

	lastWord := strings.ToLower(words[len(words)-1])
	if !mediaWords[lastWord] {
		return "", false
	}

	// Extract everything before the media word as the potential name
	nameWords := words[:len(words)-1]
	if len(nameWords) == 0 {
		return "", false
	}

	// Skip if it contains a nationality pattern like "korean movies"
	nationalityWords := map[string]bool{
		"korean": true, "japanese": true, "french": true, "italian": true,
		"german": true, "spanish": true, "british": true, "american": true,
		"chinese": true, "indian": true, "russian": true, "swedish": true,
		"danish": true, "norwegian": true, "thai": true, "turkish": true,
	}
	for _, w := range nameWords {
		if nationalityWords[strings.ToLower(w)] {
			return "", false
		}
	}

	// Check for genre/adjective words that shouldn't be treated as names
	nonNameWords := map[string]bool{
		// Genres
		"action": true, "comedy": true, "drama": true, "horror": true,
		"thriller": true, "romance": true, "fantasy": true, "animation": true,
		"documentary": true, "crime": true, "mystery": true, "adventure": true,
		"sci-fi": true, "western": true, "musical": true, "war": true,
		"comedies": true, "dramas": true, "thrillers": true, "westerns": true,
		"anime": true, "animated": true, "cartoon": true, "manga": true,
		// Adjectives
		"indie": true, "classic": true, "old": true, "new": true,
		"good": true, "best": true, "top": true, "great": true,
		"funny": true, "scary": true, "romantic": true, "sad": true,
		"happy": true, "dark": true, "light": true, "serious": true,
		// Query patterns
		"similar": true, "more": true, "other": true, "some": true,
		"any": true, "random": true, "popular": true, "trending": true,
		"recent": true, "latest": true, "upcoming": true, "recommended": true,
		// Common words
		"with": true, "about": true, "like": true, "featuring": true,
		// Demographic/age descriptors (not person names)
		"teen": true, "teenage": true, "teenager": true, "teens": true,
		"kids": true, "children": true, "child": true, "family": true,
		"adult": true, "mature": true, "young": true, "youth": true,
		// Decade descriptors (not person names)
		"50s": true, "60s": true, "70s": true, "80s": true, "90s": true,
		"00s": true, "2000s": true, "2010s": true, "2020s": true,
		"fifties": true, "sixties": true, "seventies": true, "eighties": true,
		"nineties": true, "retro": true, "vintage": true, "modern": true,
	}

	// If all name words are non-name words, skip
	allNonName := true
	for _, w := range nameWords {
		if !nonNameWords[strings.ToLower(w)] {
			allNonName = false
			break
		}
	}
	if allNonName {
		return "", false
	}

	name := strings.ToLower(strings.Join(nameWords, " "))

	// Skip very short names that are likely not person names
	if len(name) < 3 {
		return "", false
	}

	return name, true
}

// extractSimilarToTitle extracts a movie title from "movies like X" or "similar to X" patterns.
// Returns the extracted title and whether a pattern was found.
func extractSimilarToTitle(query string) (string, bool) {
	q := strings.ToLower(query)

	// Patterns to detect (order matters - more specific first):
	// - "movies like X", "films like X", "something like X"
	// - "movies similar to X", "films similar to X"
	// - "similar to X"
	// - "like X" (when it starts with "like")
	// - "more like X"

	patterns := []struct {
		prefix string
		suffix string // optional suffix to strip from the end
	}{
		// "movies/films like X" patterns
		{"movies like ", ""},
		{"films like ", ""},
		{"movie like ", ""},
		{"film like ", ""},
		{"something like ", ""},
		{"anything like ", ""},
		{"more like ", ""},

		// "similar to X" patterns
		{"movies similar to ", ""},
		{"films similar to ", ""},
		{"movie similar to ", ""},
		{"film similar to ", ""},
		{"similar to ", ""},

		// "recommend me X" style patterns
		{"recommend something like ", ""},
		{"recommend movies like ", ""},
		{"recommend films like ", ""},

		// "find me movies like X"
		{"find movies like ", ""},
		{"find films like ", ""},
		{"find me movies like ", ""},
		{"find me films like ", ""},
		{"show me movies like ", ""},
		{"show me films like ", ""},
	}

	for _, p := range patterns {
		if idx := strings.Index(q, p.prefix); idx != -1 {
			// Extract everything after the pattern
			title := strings.TrimSpace(q[idx+len(p.prefix):])

			// Strip common suffixes that might be added
			suffixesToStrip := []string{
				" please", " thanks", " movie", " film",
				" recommendations", " recommendation",
			}
			for _, suffix := range suffixesToStrip {
				title = strings.TrimSuffix(title, suffix)
			}

			title = strings.TrimSpace(title)

			// Must have at least something as the title
			if len(title) >= 2 {
				return title, true
			}
		}
	}

	return "", false
}

// extractCollectionName extracts a collection/franchise name from queries like
// "all Mission Impossible movies", "Harry Potter in order", "Star Wars saga".
// Returns the extracted collection name (normalized) and whether a pattern was found.
func extractCollectionName(query string) (string, bool) {
	q := strings.ToLower(query)

	// Common abbreviations to expand for better matching
	abbreviations := map[string]string{
		"mcu":  "marvel cinematic universe",
		"dceu": "dc extended universe",
		"lotr": "lord of the rings",
		"potc": "pirates of the caribbean",
		"hp":   "harry potter",
		"sw":   "star wars",
		"mi":   "mission impossible",
		"f&f":  "fast and furious",
		"ff":   "fast and furious",
	}

	// Check for abbreviations first
	for abbr, full := range abbreviations {
		if strings.Contains(q, abbr) {
			return full, true
		}
	}

	// Patterns that indicate collection search with name BEFORE the keyword
	// E.g., "star wars saga", "harry potter series", "mission impossible franchise"
	suffixPatterns := []string{
		" saga",
		" trilogy",
		" quadrilogy",
		" pentalogy",
		" hexalogy",
		" franchise",
		" collection",
		" universe",
	}

	for _, suffix := range suffixPatterns {
		if idx := strings.Index(q, suffix); idx != -1 {
			name := strings.TrimSpace(q[:idx])
			// Remove leading "the " if present
			name = strings.TrimPrefix(name, "the ")
			// Remove leading "complete " if present
			name = strings.TrimPrefix(name, "complete ")
			if len(name) >= 2 {
				return name, true
			}
		}
	}

	// Patterns with name AFTER the keyword
	// E.g., "all mission impossible movies", "every james bond film", "complete mcu"
	prefixPatterns := []struct {
		prefix string
		suffix string // optional suffix to strip
	}{
		{"all ", " movies"},
		{"all ", " films"},
		{"all ", " movie"},
		{"all ", " film"},
		{"all ", ""},
		{"every ", " movie"},
		{"every ", " film"},
		{"every ", " movies"},
		{"every ", " films"},
		{"every ", ""},
		{"complete ", " movies"},
		{"complete ", " films"},
		{"complete ", ""},
		{"entire ", " series"},
		{"entire ", " franchise"},
		{"entire ", ""},
	}

	for _, p := range prefixPatterns {
		if strings.HasPrefix(q, p.prefix) {
			name := strings.TrimPrefix(q, p.prefix)
			if p.suffix != "" {
				name = strings.TrimSuffix(name, p.suffix)
			}
			// Also strip common trailing words
			for _, trail := range []string{" in order", " chronologically", " in sequence", " from first to last"} {
				name = strings.TrimSuffix(name, trail)
			}
			name = strings.TrimSpace(name)
			if len(name) >= 2 {
				return name, true
			}
		}
	}

	// Handle "X series" pattern but NOT for TV series context
	// "breaking bad series" = TV show, "mission impossible series" = collection
	// Heuristic: if it contains common TV show indicators, skip
	tvIndicators := []string{"season", "episode", "tv", "show", "watch"}
	isTVContext := false
	for _, indicator := range tvIndicators {
		if strings.Contains(q, indicator) {
			isTVContext = true
			break
		}
	}

	if !isTVContext {
		if idx := strings.Index(q, " series"); idx != -1 {
			// Check it's not "tv series" or "series finale" etc.
			before := strings.TrimSpace(q[:idx])
			if before != "tv" && before != "the" && len(before) >= 2 {
				// Remove leading "the " if present
				before = strings.TrimPrefix(before, "the ")
				return before, true
			}
		}
	}

	return "", false
}

// detectChronologicalIntent checks if the user wants results in chronological order.
func detectChronologicalIntent(query string) bool {
	q := strings.ToLower(query)

	chronoPatterns := []string{
		"in order",
		"chronologically",
		"chronological order",
		"in sequence",
		"from first to last",
		"in release order",
		"release order",
		"by release date",
		"oldest to newest",
		"oldest first",
	}

	for _, pattern := range chronoPatterns {
		if strings.Contains(q, pattern) {
			return true
		}
	}

	return false
}

// applyKeywordBoost boosts results that contain query keywords in their text.
// This helps surface exact matches for directors, actors, locations, genres.
// Also applies penalties for negative signals (e.g., "without violence").
// queryOriginal is the original query with case preserved (for proper noun detection).
// queryLower is the lowercased query for matching.
func (s *SearchService) applyKeywordBoost(results []SearchResult, queryOriginal, queryLower string) []SearchResult {
	// Get configurable boost weights
	boosts := s.getBoosts()

	// Extract meaningful terms from query
	queryTerms := extractSearchTerms(queryLower)

	// Extract negative terms (things to avoid)
	negativeTerms := extractNegativeTerms(queryLower)

	// Detect if query is looking for specific genres
	queryGenres := extractGenresFromQuery(queryLower)

	// Detect genres to avoid
	negativeGenres := extractNegativeGenres(queryLower)

	// Detect query intent for stronger boosting
	// Pass original query to preserve case for proper noun detection
	intent := detectQueryIntentWithConfig(queryOriginal, s.getKnownStudios(), s.getLanguageMap())

	// Track if we have a strong intent-based search (needs higher boost cap)
	hasStrongIntent := intent.isDirectorSearch || intent.isActorSearch ||
		intent.isWriterSearch || intent.isProducerSearch ||
		intent.isStudioSearch || intent.isPersonSearch || intent.isLanguageSearch ||
		intent.isCollectionSearch || intent.isComposerSearch || intent.isCinematographerSearch

	for i := range results {
		textLower := strings.ToLower(results[i].Text)
		boost := float32(0.0)
		penalty := float32(0.0)

		// For director searches, apply VERY strong boost/penalty
		if intent.isDirectorSearch && intent.directorName != "" {
			directorLine := extractLine(textLower, "directed by:")
			if strings.Contains(directorLine, intent.directorName) {
				boost += boosts.DirectorMatch
			} else {
				penalty += boosts.DirectorMismatchPenalty
			}
		}

		// For actor searches, apply strong boost/penalty
		if intent.isActorSearch && intent.actorName != "" {
			castLine := extractLine(textLower, "cast:")
			if strings.Contains(castLine, intent.actorName) {
				boost += boosts.ActorMatch
			} else {
				penalty += boosts.ActorMismatchPenalty
			}
		}

		// For writer searches
		if intent.isWriterSearch && intent.writerName != "" {
			writerLine := extractLine(textLower, "written by:")
			if strings.Contains(writerLine, intent.writerName) {
				boost += boosts.WriterMatch
			} else {
				penalty += boosts.WriterMismatchPenalty
			}
		}

		// For producer searches
		if intent.isProducerSearch && intent.producerName != "" {
			producerLine := extractLine(textLower, "produced by:")
			if strings.Contains(producerLine, intent.producerName) {
				boost += boosts.ProducerMatch
			} else {
				penalty += boosts.ProducerMismatchPenalty
			}
		}

		// For studio searches
		if intent.isStudioSearch && intent.studioName != "" {
			studioLine := extractLine(textLower, "studios:")
			if studioLine == "" {
				studioLine = extractLine(textLower, "network:")
			}
			if strings.Contains(studioLine, intent.studioName) {
				boost += boosts.StudioMatch
			} else {
				penalty += boosts.StudioMismatchPenalty
			}
		}

		// For generic person searches ("Name movies" pattern)
		// Check all people fields with priority: director > writer > cast (lead) > producer > cast (minor)
		// "Spielberg films" should prioritize films directed by Spielberg over films where
		// a Spielberg family member has a minor role
		if intent.isPersonSearch && intent.personName != "" {
			directorMatch := strings.Contains(extractLine(textLower, "directed by:"), intent.personName)
			writerMatch := strings.Contains(extractLine(textLower, "written by:"), intent.personName)
			producerMatch := strings.Contains(extractLine(textLower, "produced by:"), intent.personName)
			castMatch := strings.Contains(extractLine(textLower, "cast:"), intent.personName)
			studioMatch := strings.Contains(extractLine(textLower, "studios:"), intent.personName)

			// Determine the strength of the match
			// Primary roles (director, writer, studio) get strong boosts
			// Secondary roles (producer, cast) get moderate boosts
			// Cast-only matches (especially for common surnames) get weaker boosts
			if directorMatch {
				boost += boosts.PersonDirectorMatch
			}
			if writerMatch {
				boost += boosts.PersonWriterMatch
			}
			if studioMatch {
				boost += boosts.PersonStudioMatch
			}
			if producerMatch && !directorMatch && !writerMatch {
				// Only boost producer if not already director/writer
				boost += boosts.PersonProducerMatch
			}
			if castMatch && !directorMatch && !writerMatch {
				// Cast-only match gets smaller boost and penalty for not being primary
				// This helps "Spielberg films" prioritize directed films over cameos
				boost += boosts.PersonCastMatch
				penalty += boosts.PersonCastPenalty
			}

			// Strong penalty for not finding the person at all
			foundInPeople := directorMatch || writerMatch || producerMatch || castMatch || studioMatch
			if !foundInPeople {
				penalty += boosts.PersonNotFoundPenalty
			}
		}

		// For language/nationality searches (French films, Korean movies, etc.)
		if intent.isLanguageSearch && intent.languageName != "" {
			languageLine := extractLine(textLower, "language:")
			countryLine := extractLine(textLower, "country:")
			typeLine := extractLine(textLower, "type:") // For K-drama, J-drama hints

			matchedLanguage := strings.Contains(languageLine, intent.languageName) ||
				strings.Contains(countryLine, intent.languageName) ||
				strings.Contains(typeLine, intent.languageName)

			if matchedLanguage {
				boost += boosts.LanguageMatch
			} else {
				penalty += boosts.LanguageMismatchPenalty
			}
		}

		// Composer boost (music by, score by, composed by)
		if intent.isComposerSearch && intent.composerName != "" {
			composerLine := extractLine(textLower, "music by:")
			if strings.Contains(composerLine, intent.composerName) {
				boost += boosts.ComposerMatch
			} else {
				penalty += boosts.ComposerMismatchPenalty
			}
		}

		// Cinematographer boost (cinematography by, shot by, dp)
		if intent.isCinematographerSearch && intent.cinematographerName != "" {
			cinematographerLine := extractLine(textLower, "cinematography by:")
			if strings.Contains(cinematographerLine, intent.cinematographerName) {
				boost += boosts.CinematographerMatch
			} else {
				penalty += boosts.CinematographerMismatchPenalty
			}
		}

		// For collection/franchise searches ("all Mission Impossible movies", "Harry Potter in order")
		if intent.isCollectionSearch && intent.collectionName != "" {
			titleLine := extractLine(textLower, "title:")
			collectionLine := extractLine(textLower, "collection:")

			// Check if title contains the collection name
			titleMatch := strings.Contains(titleLine, intent.collectionName)
			// Check if there's a collection field that matches
			collectionMatch := strings.Contains(collectionLine, intent.collectionName)

			// Also check individual words of the collection name for partial matches
			// E.g., "mission impossible" should match "Mission: Impossible - Fallout"
			collectionWords := strings.Fields(intent.collectionName)
			wordMatchCount := 0
			for _, word := range collectionWords {
				if len(word) >= 3 && strings.Contains(titleLine, word) {
					wordMatchCount++
				}
			}
			partialMatch := len(collectionWords) > 0 && wordMatchCount >= len(collectionWords)/2+1

			if titleMatch || collectionMatch {
				boost += 0.60 // Very strong boost for exact collection match
			} else if partialMatch {
				boost += 0.40 // Good boost for partial match
			} else {
				penalty += 0.50 // Strong penalty for non-matching collection
			}
		}

		// Boost for each query term found in the result text
		matchCount := 0
		for _, term := range queryTerms {
			if strings.Contains(textLower, term) {
				matchCount++
				// Higher boost for matches in title or director line
				if strings.Contains(extractLine(textLower, "title:"), term) {
					boost += 0.15
				} else if strings.Contains(extractLine(textLower, "directed by:"), term) {
					boost += 0.12
				} else if strings.Contains(extractLine(textLower, "written by:"), term) {
					boost += 0.10
				} else if strings.Contains(extractLine(textLower, "produced by:"), term) {
					boost += 0.08
				} else if strings.Contains(extractLine(textLower, "cast:"), term) {
					boost += 0.10
				} else if strings.Contains(extractLine(textLower, "studios:"), term) {
					boost += 0.10
				} else if strings.Contains(extractLine(textLower, "setting:"), term) {
					boost += 0.10
				} else {
					boost += 0.05
				}
			}
		}

		// Bonus for matching multiple terms
		if matchCount > 1 {
			boost += float32(matchCount-1) * 0.03
		}

		// Genre matching boost - very strong boost/penalty for genre-specific searches
		if len(queryGenres) > 0 {
			resultGenres := extractLine(textLower, "genre:")
			genreMatches := 0
			for _, g := range queryGenres {
				if strings.Contains(resultGenres, g) {
					genreMatches++
				}
			}
			if genreMatches > 0 {
				boost += float32(genreMatches) * boosts.GenreMatch
			} else {
				// Very strong penalty for results that don't match the requested genre
				// This ensures non-matching genres are pushed to the bottom
				penalty += boosts.GenreMismatchPenalty
			}
		}

		// Apply negative term penalties
		for _, term := range negativeTerms {
			if strings.Contains(textLower, term) {
				penalty += 0.25 // Strong penalty for containing unwanted terms
			}
		}

		// Apply negative genre penalties - VERY strong to override semantic similarity
		if len(negativeGenres) > 0 {
			resultGenres := extractLine(textLower, "genre:")
			for _, g := range negativeGenres {
				if strings.Contains(resultGenres, g) {
					penalty += 0.40 // Very strong penalty for unwanted genres
				}
			}
		}

		// Apply boost and penalty (use higher cap for intent-based searches)
		maxBoost := float32(0.3)
		if hasStrongIntent {
			maxBoost = 0.6 // Higher cap for person/studio searches
		}
		if boost > maxBoost {
			boost = maxBoost
		}
		results[i].Similarity += boost - penalty

		// Cap at 1.0 and floor at 0
		if results[i].Similarity > 1.0 {
			results[i].Similarity = 1.0
		}
		if results[i].Similarity < 0 {
			results[i].Similarity = 0
		}
	}

	return results
}

// extractNegativeTerms extracts terms the user wants to avoid.
// Patterns: "without X", "no X", "non-X", "not X", "avoid X"
func extractNegativeTerms(query string) []string {
	var negatives []string

	// Patterns for negative signals
	patterns := []struct {
		prefix string
		suffix string
	}{
		{"without ", ""},
		{"no ", ""},
		{"non-", ""},
		{"non ", ""},
		{"not ", ""},
		{"avoid ", ""},
		{"excluding ", ""},
		{"except ", ""},
	}

	words := strings.Fields(query)
	for i, word := range words {
		for _, p := range patterns {
			if strings.HasPrefix(word, p.prefix) {
				// Extract the term after the prefix
				term := strings.TrimPrefix(word, p.prefix)
				if term != "" {
					negatives = append(negatives, term)
				}
			}
		}
		// Check for two-word patterns like "no violence"
		if i < len(words)-1 {
			twoWord := word + " " + words[i+1]
			for _, p := range patterns {
				if strings.HasPrefix(twoWord, p.prefix) {
					term := strings.TrimPrefix(twoWord, p.prefix)
					// Take just the next word
					termParts := strings.Fields(term)
					if len(termParts) > 0 {
						negatives = append(negatives, termParts[0])
					}
				}
			}
		}
	}

	return negatives
}

// extractMoodImpliedNegatives returns genres that should be penalized based on mood terms.
// For example, "cozy" implies not horror/thriller.
func extractMoodImpliedNegatives(query string) []string {
	moodMap := map[string][]string{
		// Cozy/comfort moods should avoid dark content
		"cozy":         {"horror", "thriller"},
		"comfy":        {"horror", "thriller"},
		"comfort":      {"horror", "thriller"},
		"heartwarming": {"horror", "thriller"},
		"feel good":    {"horror", "thriller", "drama"},
		"feel-good":    {"horror", "thriller", "drama"},
		"uplifting":    {"horror", "thriller"},
		"light":        {"horror", "thriller", "drama"},
		"lighthearted": {"horror", "thriller"},
		"fun":          {"horror", "drama"},
		"happy":        {"horror", "thriller", "drama"},
		"cheerful":     {"horror", "thriller"},
		"relaxing":     {"horror", "thriller"},
		// Kids/family implies no horror
		"family":   {"horror"},
		"kids":     {"horror"},
		"children": {"horror"},
	}

	var negatives []string
	seen := make(map[string]bool)
	for mood, genres := range moodMap {
		if strings.Contains(query, mood) {
			for _, g := range genres {
				if !seen[g] {
					negatives = append(negatives, g)
					seen[g] = true
				}
			}
		}
	}
	return negatives
}

// extractNegativeGenres extracts genres the user wants to avoid.
func extractNegativeGenres(query string) []string {
	negativeTerms := extractNegativeTerms(query)

	// Start with mood-implied negatives
	genres := extractMoodImpliedNegatives(query)
	seen := make(map[string]bool)
	for _, g := range genres {
		seen[g] = true
	}

	// If no explicit negatives, just return mood-implied ones
	if len(negativeTerms) == 0 {
		return genres
	}

	genreMap := map[string]string{
		"action":    "action",
		"violence":  "action", // violence often means action
		"violent":   "action",
		"comedy":    "comedy",
		"funny":     "comedy",
		"drama":     "drama",
		"horror":    "horror",
		"scary":     "horror",
		"thriller":  "thriller",
		"romance":   "romance",
		"romantic":  "romance",
		"love":      "romance",
		"sci-fi":    "science fiction",
		"fantasy":   "fantasy",
		"animation": "animation",
		"animated":  "animation",
		"cartoon":   "animation",
		"war":       "war",
	}

	for _, term := range negativeTerms {
		if genre, ok := genreMap[term]; ok && !seen[genre] {
			genres = append(genres, genre)
			seen[genre] = true
		}
	}

	return genres
}

// extractSearchTerms extracts meaningful search terms from the query.
func extractSearchTerms(query string) []string {
	// Words to skip
	skipWords := map[string]bool{
		"a": true, "an": true, "the": true, "and": true, "or": true,
		"in": true, "on": true, "at": true, "to": true, "for": true,
		"of": true, "with": true, "by": true, "from": true,
		"movie": true, "movies": true, "film": true, "films": true,
		"show": true, "shows": true, "series": true,
		"something": true, "anything": true, "want": true, "looking": true,
		"like": true, "similar": true, "good": true, "best": true, "great": true,
		"i": true, "me": true, "my": true, "watch": true, "see": true,
		"set": true, "directed": true, "starring": true, "featuring": true,
	}

	words := strings.Fields(query)
	var terms []string
	for _, word := range words {
		// Remove punctuation
		word = strings.Trim(word, ".,!?;:'\"")
		if len(word) < 2 {
			continue
		}
		if skipWords[word] {
			continue
		}
		terms = append(terms, word)
	}
	return terms
}

// extractGenresFromQuery detects genre keywords in the query.
func extractGenresFromQuery(query string) []string {
	genreMap := map[string]string{
		"action":          "action",
		"comedy":          "comedy",
		"comedies":        "comedy",
		"funny":           "comedy",
		"drama":           "drama",
		"dramas":          "drama",
		"horror":          "horror",
		"scary":           "horror",
		"thriller":        "thriller",
		"thrillers":       "thriller",
		"romance":         "romance",
		"romantic":        "romance",
		"romcom":          "comedy", // rom-coms are comedies
		"rom-com":         "comedy",
		"sci-fi":          "science fiction",
		"scifi":           "science fiction",
		"science fiction": "science fiction",
		"fantasy":         "fantasy",
		"animation":       "animation",
		"animated":        "animation",
		"documentary":     "documentary",
		"documentaries":   "documentary",
		"crime":           "crime",
		"mystery":         "mystery",
		"western":         "western",
		"westerns":        "western",
		"war":             "war",
		"musical":         "music",
		"musicals":        "music",
		"family":          "family",
		"adventure":       "adventure",
		"superhero":       "action", // superhero often tagged as action
	}

	var genres []string
	for keyword, genre := range genreMap {
		if strings.Contains(query, keyword) {
			// Avoid duplicates
			found := false
			for _, g := range genres {
				if g == genre {
					found = true
					break
				}
			}
			if !found {
				genres = append(genres, genre)
			}
		}
	}
	return genres
}

// extractLine extracts a specific line from text based on prefix.
func extractLine(text, prefix string) string {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		lineLower := strings.ToLower(line)
		if strings.HasPrefix(lineLower, prefix) {
			return lineLower
		}
	}
	return ""
}

// deduplicateByTitle removes duplicate results that have the same title.
// This handles cases where the same movie/show exists multiple times in the library
// (e.g., different files, different libraries). Keeps the result with highest similarity.
func (s *SearchService) deduplicateByTitle(results []SearchResult) []SearchResult {
	// Map from title key to best result
	bestByTitle := make(map[string]SearchResult)

	for _, r := range results {
		titleKey := extractTitleKey(r.Text)
		if titleKey == "" {
			// If we can't extract a title, use entity ID as key (fallback)
			titleKey = fmt.Sprintf("%s:%d", r.EntityType, r.EntityID)
		}

		existing, found := bestByTitle[titleKey]
		if !found || r.Similarity > existing.Similarity {
			bestByTitle[titleKey] = r
		}
	}

	// Rebuild results slice, maintaining original order for equal-scored items
	seen := make(map[string]bool)
	deduped := make([]SearchResult, 0, len(bestByTitle))

	for _, r := range results {
		titleKey := extractTitleKey(r.Text)
		if titleKey == "" {
			titleKey = fmt.Sprintf("%s:%d", r.EntityType, r.EntityID)
		}

		if seen[titleKey] {
			continue
		}
		seen[titleKey] = true

		// Use the best result for this title
		deduped = append(deduped, bestByTitle[titleKey])
	}

	return deduped
}

// applyDiversityPenalty penalizes results that are too similar to already-selected results.
// This helps ensure variety in recommendations (e.g., not all sequels, not all same director).
func (s *SearchService) applyDiversityPenalty(results []SearchResult, limit int) []SearchResult {
	if len(results) <= limit {
		return results
	}

	// Get configurable diversity settings
	diversity := s.getDiversityConfig()

	diverse := make([]SearchResult, 0, limit)
	selectedDirectors := make(map[string]int)
	selectedDecades := make(map[string]int)
	selectedGenres := make(map[string]int)

	for _, r := range results {
		if len(diverse) >= limit {
			break
		}

		textLower := strings.ToLower(r.Text)
		director := extractDirector(textLower)
		decade := extractDecade(r.Text)
		genres := extractLine(textLower, "genre:")

		// Calculate diversity penalty
		penalty := float32(0.0)

		// Penalize if we already have movies by this director
		if director != "" && selectedDirectors[director] > 0 {
			penalty += float32(selectedDirectors[director]) * diversity.SameDirectorPenalty
		}

		// Penalize if we already have movies from this decade
		if decade != "" && selectedDecades[decade] > 1 {
			penalty += float32(selectedDecades[decade]-1) * diversity.SameDecadePenalty
		}

		// Light penalty for repeated genres (we want some genre coherence)
		for genre := range selectedGenres {
			if strings.Contains(genres, genre) && selectedGenres[genre] > 2 {
				penalty += diversity.SameGenrePenalty
			}
		}

		// Apply penalty
		r.Similarity -= penalty
		if r.Similarity < s.minSimilarity {
			continue // Skip if penalty pushed it below threshold
		}

		diverse = append(diverse, r)

		// Track what we've selected
		if director != "" {
			selectedDirectors[director]++
		}
		if decade != "" {
			selectedDecades[decade]++
		}
		// Track primary genre
		if genres != "" {
			parts := strings.Split(genres, ",")
			if len(parts) > 0 {
				primaryGenre := strings.TrimSpace(strings.TrimPrefix(parts[0], "genre:"))
				selectedGenres[primaryGenre]++
			}
		}
	}

	return diverse
}

// extractDirector extracts director name from the text.
func extractDirector(textLower string) string {
	line := extractLine(textLower, "directed by:")
	if line == "" {
		return ""
	}
	// Remove "directed by:" prefix
	director := strings.TrimPrefix(line, "directed by:")
	director = strings.TrimSpace(director)
	return director
}

// extractDecade extracts the decade from the title line (e.g., "(1985)" -> "1980s").
func extractDecade(text string) string {
	titleLine := extractLine(strings.ToLower(text), "title:")
	// Find year in parentheses
	start := strings.LastIndex(titleLine, "(")
	end := strings.LastIndex(titleLine, ")")
	if start == -1 || end == -1 || end <= start {
		return ""
	}
	yearStr := titleLine[start+1 : end]
	if len(yearStr) == 4 && yearStr[0] >= '1' && yearStr[0] <= '2' {
		return yearStr[:3] + "0s"
	}
	return ""
}

// extractTitleKey extracts a normalized title from the embedding text for deduplication.
// The text format is "Title: Movie Name (Year)\n..."
func extractTitleKey(text string) string {
	// Find the first line which contains "Title: ..."
	lines := strings.SplitN(text, "\n", 2)
	if len(lines) == 0 {
		return ""
	}

	firstLine := lines[0]
	if !strings.HasPrefix(firstLine, "Title: ") {
		return ""
	}

	// Extract "Movie Name (Year)" part
	title := strings.TrimPrefix(firstLine, "Title: ")

	// Normalize: lowercase and remove extra spaces
	title = strings.ToLower(strings.TrimSpace(title))

	return title
}

// FindSimilar finds items similar to a given entity.
func (s *SearchService) FindSimilar(ctx context.Context, entityType EntityType, entityID int64, limit int) ([]SearchResult, error) {
	if s.vector == nil {
		return nil, fmt.Errorf("vector client not available")
	}

	// Get the embedding for the source entity
	stored, err := s.vector.Get(ctx, string(entityType), entityID)
	if err != nil {
		return nil, fmt.Errorf("get embedding: %w", err)
	}
	if stored == nil {
		return nil, fmt.Errorf("embedding not found for %s:%d", entityType, entityID)
	}

	// Apply limits
	if limit <= 0 {
		limit = s.defaultLimit
	}
	if limit > s.maxLimit {
		limit = s.maxLimit
	}

	// Search for similar items (of the same type by default)
	searchResp, err := s.vector.Search(ctx, sdk.VectorSearchRequest{
		QueryVector:   stored.Vector,
		EntityTypes:   []string{string(entityType)},
		Limit:         limit + 1,
		MinSimilarity: s.minSimilarity,
	})
	if err != nil {
		return nil, fmt.Errorf("search similar: %w", err)
	}

	// Filter out the source entity
	results := make([]SearchResult, 0, len(searchResp.Results))
	for _, r := range searchResp.Results {
		if r.EntityType == string(entityType) && r.EntityID == entityID {
			continue // Skip self
		}
		results = append(results, SearchResult{
			EntityType: EntityType(r.EntityType),
			EntityID:   r.EntityID,
			Similarity: r.Similarity,
			Text:       r.Text,
		})
		if len(results) >= limit {
			break
		}
	}

	return results, nil
}

// handleSimilarToSearch handles "movies like X" queries by finding the source movie and returning similar items.
func (s *SearchService) handleSimilarToSearch(ctx context.Context, title string, params SearchParams) ([]SearchResult, error) {
	// Determine which entity types to search for the source movie
	// Default to movie if no entity types specified or if movie is in the list
	sourceTypes := []string{string(EntityMovie)}
	if len(params.EntityTypes) > 0 {
		// Use the first entity type as the source type
		sourceTypes = []string{string(params.EntityTypes[0])}
	}

	// Search for the movie by title using text search
	s.logger.Debug("searching for source movie by title",
		"title", title,
		"sourceTypes", sourceTypes)
	textResults, err := s.vector.SearchText(ctx, title, sourceTypes, 20)
	if err != nil {
		s.logger.Warn("text search failed for source title",
			"title", title, "error", err)
		return nil, fmt.Errorf("search for source title: %w", err)
	}

	s.logger.Info("text search for source movie",
		"title", title,
		"resultCount", len(textResults.Results))

	if len(textResults.Results) == 0 {
		return nil, fmt.Errorf("no movie found matching '%s'", title)
	}

	// Log first few results for debugging
	for i, r := range textResults.Results {
		if i >= 3 {
			break
		}
		s.logger.Debug("text search result",
			"index", i,
			"entityID", r.EntityID,
			"titleKey", extractTitleKey(r.Text))
	}

	// Find the best match - prefer exact title matches
	var bestMatch *sdk.VectorSearchResult
	titleLower := strings.ToLower(title)

	for i, r := range textResults.Results {
		resultTitleKey := extractTitleKey(r.Text)
		// Check for exact match (title without year)
		resultTitle := resultTitleKey
		// Strip year in parentheses for matching
		if idx := strings.LastIndex(resultTitle, "("); idx > 0 {
			resultTitle = strings.TrimSpace(resultTitle[:idx])
		}

		if resultTitle == titleLower {
			bestMatch = &textResults.Results[i]
			break
		}
		// Also check if title contains the search term (for partial matches)
		if bestMatch == nil && strings.Contains(resultTitle, titleLower) {
			bestMatch = &textResults.Results[i]
		}
	}

	// If no exact match, use the first result
	if bestMatch == nil {
		bestMatch = &textResults.Results[0]
	}

	s.logger.Info("found source movie for similar search",
		"query", title,
		"matched", extractTitleKey(bestMatch.Text),
		"entityID", bestMatch.EntityID)

	// Apply limits
	limit := params.Limit
	if limit <= 0 {
		limit = s.defaultLimit
	}
	if limit > s.maxLimit {
		limit = s.maxLimit
	}

	// Use FindSimilar to get similar movies
	results, err := s.FindSimilar(ctx, EntityType(bestMatch.EntityType), bestMatch.EntityID, limit)
	if err != nil {
		return nil, fmt.Errorf("find similar to %s: %w", bestMatch.Text, err)
	}

	return results, nil
}

// GetStatus returns the current search/indexing status.
func (s *SearchService) GetStatus(ctx context.Context) (*IndexingStatus, error) {
	if s.vector == nil {
		return nil, fmt.Errorf("vector client not available")
	}

	status := &IndexingStatus{
		Stats: make(map[EntityType]EntityStats),
	}

	// Get counts for each entity type
	for _, entityType := range []EntityType{
		EntityMovie, EntityTVShow, EntityTVEpisode,
		EntityMusicArtist, EntityMusicAlbum, EntityMusicTrack,
	} {
		count, err := s.vector.Count(ctx, string(entityType))
		if err != nil {
			s.logger.Warn("failed to count embeddings", "type", entityType, "error", err)
			continue
		}
		status.Stats[entityType] = EntityStats{
			Indexed: count,
		}
	}

	return status, nil
}

// ratingPattern matches "Rating: X.X/10 (N votes)" or "Rating: X.X (N votes)" patterns in text.
var ratingPattern = regexp.MustCompile(`(?i)rating:\s*(\d+(?:\.\d+)?)\s*(?:/10)?\s*\((\d+)\s*votes?\)`)

// extractRatingFromText extracts rating and vote count from embedding text.
// Looks for patterns like "Rating: 8.5/10 (1234 votes)" or "Rating: 8.5 (1234 votes)".
func extractRatingFromText(text string) (rating float32, votes int64) {
	matches := ratingPattern.FindStringSubmatch(text)
	if len(matches) < 3 {
		return 0, 0
	}

	// Parse rating (first capture group)
	if r, err := strconv.ParseFloat(matches[1], 32); err == nil {
		rating = float32(r)
		// Normalize to 0-10 scale if needed (some sources use 0-100)
		if rating > 10 {
			rating = rating / 10.0
		}
	}

	// Parse vote count (second capture group)
	if v, err := strconv.ParseInt(matches[2], 10, 64); err == nil {
		votes = v
	}

	return rating, votes
}

// applyQualityBoost applies rating/popularity boost as the FINAL ranking stage.
// Guardrails ensure quality never overrides a significantly better semantic match.
//
// Guardrails:
//  1. Apply ONLY in final re-rank stage (after all other boosts)
//  2. Cap impact at ±15% per result
//  3. Require minimum vote threshold (100 votes) before applying
//  4. Use multiplicative boost, not additive
//
// Example of correct behavior:
//
//	Query: "obscure 1970s Italian giallo horror"
//	Deep Red (1975)      similarity=0.78, quality_boost=+3%  → 0.80 (wins)
//	The Godfather (1972) similarity=0.45, quality_boost=+15% → 0.52 (loses)
//
// Quality boost helps break ties, not override relevance.
func (s *SearchService) applyQualityBoost(results []SearchResult) []SearchResult {
	// Get configurable quality settings
	qualityConfig := s.getQualityConfig()

	// Skip if quality boost is disabled
	if !qualityConfig.Enabled {
		return results
	}

	for i := range results {
		// Extract rating info from text if not already populated
		if results[i].Rating == 0 && results[i].VoteCount == 0 {
			results[i].Rating, results[i].VoteCount = extractRatingFromText(results[i].Text)
		}

		rating := results[i].Rating
		votes := results[i].VoteCount

		// Guardrail 1: Only boost if we have enough confidence (minimum votes from config)
		if votes < qualityConfig.MinVotes {
			s.logger.Debug("quality boost skipped: insufficient votes",
				"entityID", results[i].EntityID,
				"votes", votes,
				"minRequired", qualityConfig.MinVotes)
			continue
		}

		// Guardrail 2: Normalized rating boost, max 10%
		// Rating is 0-10 scale, so rating/10 gives 0-1, then multiply by 0.10 for max 10%
		ratingBoost := (rating / 10.0) * 0.10

		// Guardrail 3: Confidence factor based on vote count (log scale), max contribution 5%
		// log10(100) = 2, log10(10000) = 4, so we divide by 4 to normalize
		// This means 100 votes = 0.5 factor, 10000 votes = 1.0 factor
		confidenceFactor := math.Min(math.Log10(float64(votes))/4.0, 1.0)

		// Guardrail 4: Combined boost capped at max boost from config
		totalBoost := math.Min(float64(ratingBoost)*confidenceFactor, float64(qualityConfig.MaxBoost))

		// Guardrail 5: Soft boost - multiplicative, not additive
		// A 0.4 similarity can become at most 0.46 (not 0.55)
		// This ensures a 0.78 match can't be beaten by a 0.45 match even with max boost:
		// 0.45 * 1.15 = 0.52 < 0.78
		originalSimilarity := results[i].Similarity
		results[i].Similarity *= (1 + float32(totalBoost))

		s.logger.Debug("quality boost applied",
			"entityID", results[i].EntityID,
			"rating", rating,
			"votes", votes,
			"ratingBoost", ratingBoost,
			"confidenceFactor", confidenceFactor,
			"totalBoost", totalBoost,
			"originalSimilarity", originalSimilarity,
			"boostedSimilarity", results[i].Similarity)
	}

	// Re-sort after boosting (quality can shuffle within tiers, not across)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	return results
}

// filterExcludedChips removes chips whose type is in the excluded set.
// This allows users to "remove" an intent chip and have it not appear in the UI.
func filterExcludedChips(chips []IntentChip, excluded map[string]bool) []IntentChip {
	if len(excluded) == 0 {
		return chips
	}

	filtered := make([]IntentChip, 0, len(chips))
	for _, chip := range chips {
		if !excluded[chip.Type] {
			filtered = append(filtered, chip)
		}
	}
	return filtered
}

// removeExcludedIntentsFromQuery modifies the query to remove terms associated with excluded intents.
// This allows the search to proceed without the excluded constraints.
//
// Intent type to query term mapping:
//   - similar_to: "like X", "similar to X", "movies like X"
//   - person: "directed by X", "starring X", "X movies" (name patterns)
//   - decade: "90s", "1990s", "from the 90s"
//   - studio: "pixar", "from studio X"
//   - language: "french", "korean", nationality adjectives
//   - collection: "all X movies", "X franchise"
//   - playback: "4K", "HDR", "with subtitles"
func (s *SearchService) removeExcludedIntentsFromQuery(query string, excluded map[string]bool) string {
	if len(excluded) == 0 {
		return query
	}

	result := query
	q := strings.ToLower(query)

	// Remove "similar to" patterns
	if excluded["similar_to"] {
		similarPatterns := []string{
			"movies like ", "films like ", "shows like ", "series like ",
			"similar to ", "something like ", "anything like ",
			"like ", // catch-all, but be careful
		}
		for _, pattern := range similarPatterns {
			if idx := strings.Index(q, pattern); idx != -1 {
				// Remove pattern and everything after it (the title)
				result = strings.TrimSpace(result[:idx])
				q = strings.ToLower(result)
				break
			}
		}
	}

	// Remove person-related patterns
	if excluded["person"] {
		personPatterns := []string{
			"directed by ", "director ", "by director ",
			"starring ", "with ", "featuring ", "acted by ", "played by ",
			"written by ", "screenplay by ", "script by ", "writer ",
			"produced by ", "producer ", "from producer ",
			"music by ", "score by ", "composed by ", "composer ",
			"cinematography by ", "shot by ", "dp ", "director of photography ",
			"films by ", "movies by ",
		}
		for _, pattern := range personPatterns {
			if idx := strings.Index(q, pattern); idx != -1 {
				// Remove pattern and the name after it
				afterPattern := idx + len(pattern)
				// Find the end of the name (before common suffixes or end of string)
				nameEnd := len(result)
				for _, suffix := range []string{" films", " movies", " film", " movie", " in ", " from ", " with "} {
					if suffixIdx := strings.Index(strings.ToLower(result[afterPattern:]), suffix); suffixIdx != -1 {
						candidateEnd := afterPattern + suffixIdx
						if candidateEnd < nameEnd {
							nameEnd = candidateEnd
						}
					}
				}
				result = strings.TrimSpace(result[:idx] + result[nameEnd:])
				q = strings.ToLower(result)
			}
		}
		// Also handle "Name movies/films" pattern at the end
		for _, suffix := range []string{" movies", " films"} {
			if strings.HasSuffix(q, suffix) {
				// Check if the part before the suffix looks like a name (capitalized words)
				beforeSuffix := result[:len(result)-len(suffix)]
				words := strings.Fields(beforeSuffix)
				if len(words) > 0 {
					// Check if last 1-3 words look like a name
					nameWordCount := 0
					for i := len(words) - 1; i >= 0 && i >= len(words)-3; i-- {
						if len(words[i]) > 0 && words[i][0] >= 'A' && words[i][0] <= 'Z' {
							nameWordCount++
						} else {
							break
						}
					}
					if nameWordCount > 0 {
						// Remove the name and suffix
						result = strings.TrimSpace(strings.Join(words[:len(words)-nameWordCount], " "))
						q = strings.ToLower(result)
					}
				}
			}
		}
	}

	// Remove decade patterns
	if excluded["decade"] {
		decadePatterns := []string{
			"1920s", "1930s", "1940s", "1950s", "1960s", "1970s", "1980s", "1990s", "2000s", "2010s", "2020s",
			"20s", "30s", "40s", "50s", "60s", "70s", "80s", "90s",
			"from the ", "of the ",
		}
		for _, pattern := range decadePatterns {
			result = strings.ReplaceAll(strings.ToLower(result), pattern, " ")
		}
		// Clean up extra spaces
		result = strings.Join(strings.Fields(result), " ")
		q = strings.ToLower(result)
	}

	// Remove studio patterns
	if excluded["studio"] {
		studioPatterns := []string{"from studio ", "studio "}
		for _, pattern := range studioPatterns {
			if idx := strings.Index(q, pattern); idx != -1 {
				afterPattern := idx + len(pattern)
				// Find end of studio name
				nameEnd := len(result)
				for _, suffix := range []string{" films", " movies", " film", " movie"} {
					if suffixIdx := strings.Index(strings.ToLower(result[afterPattern:]), suffix); suffixIdx != -1 {
						candidateEnd := afterPattern + suffixIdx
						if candidateEnd < nameEnd {
							nameEnd = candidateEnd
						}
					}
				}
				result = strings.TrimSpace(result[:idx] + result[nameEnd:])
				q = strings.ToLower(result)
			}
		}
	}

	// Remove language patterns
	if excluded["language"] {
		languagePatterns := getDefaultLanguages()
		for keyword := range languagePatterns {
			result = strings.ReplaceAll(strings.ToLower(result), keyword, " ")
		}
		result = strings.Join(strings.Fields(result), " ")
		q = strings.ToLower(result)
	}

	// Remove collection patterns
	if excluded["collection"] {
		collectionPatterns := []string{
			"all ", " franchise", " saga", " series", " trilogy", " in order", " chronologically",
		}
		for _, pattern := range collectionPatterns {
			result = strings.ReplaceAll(strings.ToLower(result), pattern, " ")
		}
		result = strings.Join(strings.Fields(result), " ")
		q = strings.ToLower(result)
	}

	// Remove playback patterns
	if excluded["playback"] {
		playbackPatterns := []string{
			"4k", "uhd", "2160p", "1080p", "full hd", "fhd", "720p", "hd", "8k", "4320p",
			"dolby vision", "dv", "hdr10+", "hdr10", "hdr", "hlg",
			"atmos", "dolby atmos", "truehd", "dts-hd", "dts:x", "5.1", "7.1", "surround",
			"with subtitles", "subtitled", "with subs", "has subtitles",
		}
		for _, pattern := range playbackPatterns {
			result = strings.ReplaceAll(strings.ToLower(result), pattern, " ")
		}
		result = strings.Join(strings.Fields(result), " ")
	}

	return strings.TrimSpace(result)
}

// SearchWithRecovery performs search with progressive relaxation on zero results.
// It tries the normal search first, then progressively relaxes constraints if no results are found.
// Recovery is NOT applied for "similar to" queries - they should fail gracefully.
func (s *SearchService) SearchWithRecovery(ctx context.Context, params SearchParams) (*SearchResultWithRecovery, error) {
	if s.embeddingService == nil || s.vector == nil {
		return nil, fmt.Errorf("search service not properly initialized")
	}

	// Build excluded intent set for quick lookup
	excludedIntents := make(map[string]bool)
	for _, intent := range params.ExcludeIntents {
		excludedIntents[intent] = true
	}

	// Modify query to remove excluded intent terms
	modifiedQuery := params.Query
	if len(excludedIntents) > 0 {
		modifiedQuery = s.removeExcludedIntentsFromQuery(params.Query, excludedIntents)
		s.logger.Debug("modified query for excluded intents",
			"original", params.Query,
			"modified", modifiedQuery,
			"excluded", params.ExcludeIntents)
	}

	// Create modified params with the adjusted query
	searchParams := SearchParams{
		Query:               modifiedQuery,
		EntityTypes:         params.EntityTypes,
		Limit:               params.Limit,
		ExcludeIntents:      params.ExcludeIntents,
		PlaybackConstraints: params.PlaybackConstraints,
	}

	// Detect query intent from the ORIGINAL query (for chip display)
	// but search with the modified query
	intent := detectQueryIntent(params.Query)

	// Convert intent to chips and filter out excluded ones
	allChips := intent.convertToIntentChips(params.Query)
	intentChips := filterExcludedChips(allChips, excludedIntents)

	// Don't apply recovery for "similar to" queries - they should fail gracefully
	if intent.isSimilarSearch && !excludedIntents["similar_to"] {
		results, err := s.Search(ctx, searchParams)
		if err != nil {
			return nil, err
		}
		return &SearchResultWithRecovery{
			Results:     results,
			Total:       len(results),
			IntentChips: intentChips,
		}, nil
	}

	// Try normal search first
	results, err := s.Search(ctx, searchParams)
	if err != nil {
		return nil, err
	}

	// If we got results, no recovery needed
	if len(results) > 0 {
		return &SearchResultWithRecovery{
			Results:     results,
			Total:       len(results),
			IntentChips: intentChips,
		}, nil
	}

	// Zero results - start progressive relaxation
	s.logger.Info("zero results, starting recovery",
		"query", params.Query,
		"original_threshold", s.minSimilarity)

	var recoveryActions []RecoveryAction
	originalQuery := params.Query

	// Step 1: Lower similarity threshold (from 0.3 to 0.2)
	results, action := s.tryLowerThreshold(ctx, params)
	if action != nil {
		recoveryActions = append(recoveryActions, *action)
		s.logger.Debug("recovery step: lowered threshold",
			"original", action.Original,
			"relaxed", action.Relaxed,
			"results", len(results))
	}
	if len(results) > 0 {
		return &SearchResultWithRecovery{
			Results:         results,
			Total:           len(results),
			RecoveryApplied: recoveryActions,
			OriginalQuery:   originalQuery,
		}, nil
	}

	// Step 2: Relax decade filter (±5 years)
	results, action = s.tryRelaxDecade(ctx, params)
	if action != nil {
		recoveryActions = append(recoveryActions, *action)
		s.logger.Debug("recovery step: relaxed decade",
			"original", action.Original,
			"relaxed", action.Relaxed,
			"results", len(results))
	}
	if len(results) > 0 {
		return &SearchResultWithRecovery{
			Results:         results,
			Total:           len(results),
			RecoveryApplied: recoveryActions,
			OriginalQuery:   originalQuery,
		}, nil
	}

	// Step 3: Relax genre filter (try parent genre or remove)
	results, action = s.tryRelaxGenre(ctx, params)
	if action != nil {
		recoveryActions = append(recoveryActions, *action)
		s.logger.Debug("recovery step: relaxed genre",
			"original", action.Original,
			"relaxed", action.Relaxed,
			"results", len(results))
	}
	if len(results) > 0 {
		return &SearchResultWithRecovery{
			Results:         results,
			Total:           len(results),
			RecoveryApplied: recoveryActions,
			OriginalQuery:   originalQuery,
		}, nil
	}

	// Step 4: Remove person/studio filters, keep only semantic
	results, action = s.tryRemovePersonFilters(ctx, params)
	if action != nil {
		recoveryActions = append(recoveryActions, *action)
		s.logger.Debug("recovery step: removed person/studio filters",
			"original", action.Original,
			"relaxed", action.Relaxed,
			"results", len(results))
	}

	return &SearchResultWithRecovery{
		Results:         results,
		Total:           len(results),
		RecoveryApplied: recoveryActions,
		OriginalQuery:   originalQuery,
		IntentChips:     intentChips,
	}, nil
}

// tryLowerThreshold attempts search with a lower similarity threshold.
func (s *SearchService) tryLowerThreshold(ctx context.Context, params SearchParams) ([]SearchResult, *RecoveryAction) {
	originalThreshold := s.minSimilarity
	relaxedThreshold := float32(0.2)

	// If already at or below relaxed threshold, skip
	if originalThreshold <= relaxedThreshold {
		return nil, nil
	}

	// Generate embedding for the query
	queryEmbedding, err := s.embeddingService.EmbedSingleCached(ctx, params.Query)
	if err != nil {
		s.logger.Warn("failed to embed query for threshold recovery", "error", err)
		return nil, nil
	}

	// Apply limits
	limit := params.Limit
	if limit <= 0 {
		limit = s.defaultLimit
	}
	if limit > s.maxLimit {
		limit = s.maxLimit
	}

	// Convert entity types to strings
	entityTypeStrs := make([]string, len(params.EntityTypes))
	for i, et := range params.EntityTypes {
		entityTypeStrs[i] = string(et)
	}

	fetchLimit := limit * 5
	if fetchLimit > 200 {
		fetchLimit = 200
	}

	// Search with lower threshold
	searchResp, err := s.vector.Search(ctx, sdk.VectorSearchRequest{
		QueryVector:   queryEmbedding,
		EntityTypes:   entityTypeStrs,
		Limit:         fetchLimit,
		MinSimilarity: relaxedThreshold,
	})
	if err != nil {
		s.logger.Warn("threshold recovery search failed", "error", err)
		return nil, nil
	}

	if len(searchResp.Results) == 0 {
		return nil, &RecoveryAction{
			Type:        "threshold",
			Description: "Lowered similarity threshold",
			Original:    fmt.Sprintf("%.2f", originalThreshold),
			Relaxed:     fmt.Sprintf("%.2f", relaxedThreshold),
		}
	}

	// Convert and process results
	results := make([]SearchResult, 0, len(searchResp.Results))
	for _, r := range searchResp.Results {
		results = append(results, SearchResult{
			EntityType: EntityType(r.EntityType),
			EntityID:   r.EntityID,
			Similarity: r.Similarity,
			Text:       r.Text,
		})
	}

	// Apply boosting and deduplication
	queryLower := strings.ToLower(params.Query)
	results = s.applyKeywordBoost(results, params.Query, queryLower)
	results = s.deduplicateByTitle(results)

	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results, &RecoveryAction{
		Type:        "threshold",
		Description: "Lowered similarity threshold",
		Original:    fmt.Sprintf("%.2f", originalThreshold),
		Relaxed:     fmt.Sprintf("%.2f", relaxedThreshold),
	}
}

// tryRelaxDecade attempts search with an expanded decade range.
func (s *SearchService) tryRelaxDecade(ctx context.Context, params SearchParams) ([]SearchResult, *RecoveryAction) {
	// Extract decade from query
	decadeInfo := extractDecadeFromQuery(params.Query)
	if decadeInfo == nil {
		return nil, nil // No decade filter to relax
	}

	yearStart, ok1 := decadeInfo["year_start"].(int)
	yearEnd, ok2 := decadeInfo["year_end"].(int)
	if !ok1 || !ok2 {
		return nil, nil
	}

	// Expand by ±5 years
	relaxedStart := yearStart - 5
	relaxedEnd := yearEnd + 5

	originalRange := fmt.Sprintf("%d-%d", yearStart, yearEnd)
	relaxedRange := fmt.Sprintf("%d-%d", relaxedStart, relaxedEnd)

	// Remove decade terms from query and search with broader semantic meaning
	relaxedQuery := removeDecadeFromQuery(params.Query)
	if relaxedQuery == params.Query {
		return nil, nil // Couldn't remove decade
	}

	relaxedParams := SearchParams{
		Query:       relaxedQuery,
		EntityTypes: params.EntityTypes,
		Limit:       params.Limit,
	}

	results, err := s.Search(ctx, relaxedParams)
	if err != nil {
		s.logger.Warn("decade recovery search failed", "error", err)
		return nil, nil
	}

	// Filter results to the expanded decade range
	filteredResults := make([]SearchResult, 0)
	for _, r := range results {
		_, year := extractTitleAndYear(r.Text)
		if year >= relaxedStart && year <= relaxedEnd {
			filteredResults = append(filteredResults, r)
		}
	}

	return filteredResults, &RecoveryAction{
		Type:        "decade",
		Description: "Expanded decade range",
		Original:    originalRange,
		Relaxed:     relaxedRange,
	}
}

// tryRelaxGenre attempts search with relaxed genre constraints.
func (s *SearchService) tryRelaxGenre(ctx context.Context, params SearchParams) ([]SearchResult, *RecoveryAction) {
	queryLower := strings.ToLower(params.Query)
	genres := extractGenresFromQuery(queryLower)
	if len(genres) == 0 {
		return nil, nil // No genre filter to relax
	}

	// Try parent/broader genres
	parentGenres := getParentGenres(genres)
	if len(parentGenres) == 0 {
		// No parent genres, try removing genre entirely
		relaxedQuery := removeGenreFromQuery(params.Query)
		if relaxedQuery == params.Query {
			return nil, nil
		}

		relaxedParams := SearchParams{
			Query:       relaxedQuery,
			EntityTypes: params.EntityTypes,
			Limit:       params.Limit,
		}

		results, err := s.Search(ctx, relaxedParams)
		if err != nil {
			s.logger.Warn("genre removal recovery search failed", "error", err)
			return nil, nil
		}

		return results, &RecoveryAction{
			Type:        "genre",
			Description: "Removed genre filter",
			Original:    strings.Join(genres, ", "),
			Relaxed:     "any genre",
		}
	}

	// Build query with parent genres
	relaxedQuery := replaceGenresInQuery(params.Query, parentGenres)
	relaxedParams := SearchParams{
		Query:       relaxedQuery,
		EntityTypes: params.EntityTypes,
		Limit:       params.Limit,
	}

	results, err := s.Search(ctx, relaxedParams)
	if err != nil {
		s.logger.Warn("parent genre recovery search failed", "error", err)
		return nil, nil
	}

	return results, &RecoveryAction{
		Type:        "genre",
		Description: "Expanded to related genres",
		Original:    strings.Join(genres, ", "),
		Relaxed:     strings.Join(parentGenres, ", "),
	}
}

// tryRemovePersonFilters attempts search with person/studio filters removed.
func (s *SearchService) tryRemovePersonFilters(ctx context.Context, params SearchParams) ([]SearchResult, *RecoveryAction) {
	intent := detectQueryIntent(params.Query)

	// Check if there are person/studio filters to remove
	var filterType string
	var filterValue string

	if intent.isDirectorSearch && intent.directorName != "" {
		filterType = "director"
		filterValue = intent.directorName
	} else if intent.isActorSearch && intent.actorName != "" {
		filterType = "actor"
		filterValue = intent.actorName
	} else if intent.isWriterSearch && intent.writerName != "" {
		filterType = "writer"
		filterValue = intent.writerName
	} else if intent.isProducerSearch && intent.producerName != "" {
		filterType = "producer"
		filterValue = intent.producerName
	} else if intent.isStudioSearch && intent.studioName != "" {
		filterType = "studio"
		filterValue = intent.studioName
	} else if intent.isPersonSearch && intent.personName != "" {
		filterType = "person"
		filterValue = intent.personName
	} else {
		return nil, nil // No person/studio filters to remove
	}

	// Remove person/studio references from query
	relaxedQuery := removePersonFromQuery(params.Query, filterValue)
	if relaxedQuery == "" || relaxedQuery == params.Query {
		// If we can't simplify, try a very generic search
		relaxedQuery = extractSemanticCore(params.Query)
		if relaxedQuery == "" {
			return nil, nil
		}
	}

	relaxedParams := SearchParams{
		Query:       relaxedQuery,
		EntityTypes: params.EntityTypes,
		Limit:       params.Limit,
	}

	results, err := s.Search(ctx, relaxedParams)
	if err != nil {
		s.logger.Warn("person filter removal recovery search failed", "error", err)
		return nil, nil
	}

	return results, &RecoveryAction{
		Type:        "filters",
		Description: fmt.Sprintf("Removed %s filter", filterType),
		Original:    filterValue,
		Relaxed:     "semantic search only",
	}
}

// removeDecadeFromQuery removes decade references from a query string.
func removeDecadeFromQuery(query string) string {
	q := strings.ToLower(query)

	// Patterns to remove
	decadePatterns := []string{
		"50s", "1950s", "fifties",
		"60s", "1960s", "sixties",
		"70s", "1970s", "seventies",
		"80s", "1980s", "eighties",
		"90s", "1990s", "nineties",
		"00s", "2000s",
		"2010s",
		"2020s",
	}

	result := q
	for _, pattern := range decadePatterns {
		result = strings.ReplaceAll(result, pattern, "")
	}

	// Clean up extra spaces
	result = strings.Join(strings.Fields(result), " ")
	return strings.TrimSpace(result)
}

// removeGenreFromQuery removes genre references from a query string.
func removeGenreFromQuery(query string) string {
	q := strings.ToLower(query)

	genreWords := []string{
		"action", "comedy", "comedies", "drama", "dramas", "horror", "thriller", "thrillers",
		"romance", "romantic", "sci-fi", "scifi", "science fiction", "fantasy", "animation",
		"animated", "documentary", "documentaries", "crime", "mystery", "western", "westerns",
		"war", "musical", "musicals", "family", "adventure", "superhero",
	}

	result := q
	for _, genre := range genreWords {
		result = strings.ReplaceAll(result, genre, "")
	}

	// Clean up extra spaces
	result = strings.Join(strings.Fields(result), " ")
	return strings.TrimSpace(result)
}

// replaceGenresInQuery replaces specific genres with broader ones.
func replaceGenresInQuery(query string, newGenres []string) string {
	// First remove existing genres
	result := removeGenreFromQuery(query)

	// Add new genres
	if len(newGenres) > 0 {
		result = result + " " + strings.Join(newGenres, " ")
	}

	return strings.TrimSpace(result)
}

// getParentGenres returns broader genre categories for specific genres.
func getParentGenres(genres []string) []string {
	parentMap := map[string]string{
		// Sub-genres to parent genres
		"slasher":          "horror",
		"psychological":    "thriller",
		"rom-com":          "comedy",
		"romcom":           "comedy",
		"dark comedy":      "comedy",
		"black comedy":     "comedy",
		"action comedy":    "action",
		"spy":              "thriller",
		"heist":            "crime",
		"noir":             "crime",
		"neo-noir":         "crime",
		"space opera":      "science fiction",
		"cyberpunk":        "science fiction",
		"dystopian":        "science fiction",
		"post-apocalyptic": "science fiction",
		"supernatural":     "horror",
		"zombie":           "horror",
		"vampire":          "horror",
		"found footage":    "horror",
		"martial arts":     "action",
		"war":              "drama",
		"historical":       "drama",
		"biographical":     "drama",
		"sports":           "drama",
		"musical":          "drama",
		"anime":            "animation",
		"cartoon":          "animation",
	}

	parentGenres := make([]string, 0)
	seen := make(map[string]bool)

	for _, genre := range genres {
		if parent, ok := parentMap[genre]; ok {
			if !seen[parent] {
				parentGenres = append(parentGenres, parent)
				seen[parent] = true
			}
		}
	}

	return parentGenres
}

// removePersonFromQuery removes person/studio name references from a query.
func removePersonFromQuery(query, personName string) string {
	q := strings.ToLower(query)
	personLower := strings.ToLower(personName)

	// Remove the person name
	result := strings.ReplaceAll(q, personLower, "")

	// Remove common patterns that would be left over
	patterns := []string{
		"directed by", "director", "by director",
		"starring", "with", "featuring", "acted by", "played by",
		"written by", "screenplay by", "script by", "writer",
		"produced by", "producer", "from producer",
		"from studio", "studio", "movies by", "films by",
	}

	for _, pattern := range patterns {
		result = strings.ReplaceAll(result, pattern, "")
	}

	// Clean up extra spaces
	result = strings.Join(strings.Fields(result), " ")
	return strings.TrimSpace(result)
}

// extractSemanticCore extracts the core semantic meaning from a query,
// removing all filter-like constraints.
func extractSemanticCore(query string) string {
	q := strings.ToLower(query)

	// Remove decade references
	q = removeDecadeFromQuery(q)

	// Remove genre references
	q = removeGenreFromQuery(q)

	// Remove common filter words
	filterWords := []string{
		"movies", "films", "movie", "film", "show", "shows", "series",
		"directed by", "director", "starring", "with", "featuring",
		"written by", "produced by", "from studio", "studio",
		"from the", "in the", "about", "like", "similar to",
	}

	for _, word := range filterWords {
		q = strings.ReplaceAll(q, word, "")
	}

	// Clean up
	q = strings.Join(strings.Fields(q), " ")
	q = strings.TrimSpace(q)

	// If we're left with very little, return empty
	if len(q) < 3 {
		return ""
	}

	return q
}

// =============================================================================
// Playback Constraint Filtering
// =============================================================================

// applyPlaybackFilters filters results based on playback constraints (resolution, HDR, subtitles, etc.).
// This requires fetching media details for each result, so it's applied after ranking.
func (s *SearchService) applyPlaybackFilters(ctx context.Context, results []SearchResult, constraints *PlaybackConstraints) []SearchResult {
	if constraints == nil || constraints.IsEmpty() {
		return results
	}

	filtered := make([]SearchResult, 0, len(results))
	for _, r := range results {
		// Skip non-video entity types (music doesn't have playback constraints)
		if r.EntityType == EntityMusicArtist || r.EntityType == EntityMusicAlbum || r.EntityType == EntityMusicTrack {
			filtered = append(filtered, r)
			continue
		}

		// For movies and TV episodes, we need to check playback info
		// TV shows don't have direct playback info (their episodes do)
		if r.EntityType == EntityTVShow {
			// For TV shows, we can't filter by playback constraints directly
			// Include them and let the user filter at episode level
			filtered = append(filtered, r)
			continue
		}

		// Check playback constraints from the embedding text
		// The embedding text contains resolution, HDR, and other info
		if s.matchesPlaybackConstraints(r.Text, constraints) {
			filtered = append(filtered, r)
		}
	}

	s.logger.Debug("playback filter applied",
		"before", len(results),
		"after", len(filtered),
		"constraints", constraints)

	return filtered
}

// matchesPlaybackConstraints checks if the embedding text matches the playback constraints.
// This parses the text to extract resolution, HDR format, audio, and subtitle info.
func (s *SearchService) matchesPlaybackConstraints(text string, constraints *PlaybackConstraints) bool {
	textLower := strings.ToLower(text)

	// Check resolution constraint
	if constraints.MinResolution != "" {
		resolutionLine := extractLine(textLower, "resolution:")
		if resolutionLine == "" {
			// Also check for resolution in other formats
			resolutionLine = extractLine(textLower, "quality:")
		}
		if !meetsMinResolution(resolutionLine, constraints.MinResolution) {
			return false
		}
	}

	if constraints.MaxResolution != "" {
		resolutionLine := extractLine(textLower, "resolution:")
		if resolutionLine == "" {
			resolutionLine = extractLine(textLower, "quality:")
		}
		if !meetsMaxResolution(resolutionLine, constraints.MaxResolution) {
			return false
		}
	}

	// Check HDR format constraint
	if len(constraints.HDRFormats) > 0 {
		hdrLine := extractLine(textLower, "hdr:")
		if hdrLine == "" {
			// Check if any HDR format is mentioned in the text
			hasHDR := false
			for _, format := range constraints.HDRFormats {
				if strings.Contains(textLower, strings.ToLower(format)) {
					hasHDR = true
					break
				}
			}
			if !hasHDR {
				return false
			}
		} else {
			// Check if the HDR line contains any of the required formats
			hasFormat := false
			for _, format := range constraints.HDRFormats {
				if strings.Contains(hdrLine, strings.ToLower(format)) {
					hasFormat = true
					break
				}
			}
			if !hasFormat {
				return false
			}
		}
	}

	// Check subtitle constraint
	if constraints.HasSubtitles != nil && *constraints.HasSubtitles {
		subtitleLine := extractLine(textLower, "subtitles:")
		if subtitleLine == "" || strings.Contains(subtitleLine, "none") {
			return false
		}
	}

	// Check subtitle language constraint
	if constraints.SubtitleLanguage != "" {
		subtitleLine := extractLine(textLower, "subtitles:")
		if !strings.Contains(subtitleLine, strings.ToLower(constraints.SubtitleLanguage)) {
			return false
		}
	}

	// Check audio format constraint
	if len(constraints.AudioFormats) > 0 {
		audioLine := extractLine(textLower, "audio:")
		hasFormat := false
		for _, format := range constraints.AudioFormats {
			if strings.Contains(audioLine, strings.ToLower(format)) ||
				strings.Contains(textLower, strings.ToLower(format)) {
				hasFormat = true
				break
			}
		}
		if !hasFormat {
			return false
		}
	}

	// Check minimum channels constraint
	if constraints.MinChannels > 0 {
		audioLine := extractLine(textLower, "audio:")
		if !meetsMinChannels(audioLine, constraints.MinChannels) {
			return false
		}
	}

	return true
}

// meetsMinResolution checks if the resolution meets the minimum requirement.
func meetsMinResolution(resolutionLine, minResolution string) bool {
	resolutionOrder := map[string]int{
		"sd":    1,
		"480p":  1,
		"720p":  2,
		"hd":    2,
		"1080p": 3,
		"fhd":   3,
		"4k":    4,
		"uhd":   4,
		"2160p": 4,
		"8k":    5,
		"4320p": 5,
	}

	minOrder, ok := resolutionOrder[strings.ToLower(minResolution)]
	if !ok {
		return true // Unknown resolution, don't filter
	}

	// Check if any resolution in the line meets the minimum
	for res, order := range resolutionOrder {
		if strings.Contains(resolutionLine, res) && order >= minOrder {
			return true
		}
	}

	return false
}

// meetsMaxResolution checks if the resolution meets the maximum requirement.
func meetsMaxResolution(resolutionLine, maxResolution string) bool {
	resolutionOrder := map[string]int{
		"sd":    1,
		"480p":  1,
		"720p":  2,
		"hd":    2,
		"1080p": 3,
		"fhd":   3,
		"4k":    4,
		"uhd":   4,
		"2160p": 4,
		"8k":    5,
		"4320p": 5,
	}

	maxOrder, ok := resolutionOrder[strings.ToLower(maxResolution)]
	if !ok {
		return true // Unknown resolution, don't filter
	}

	// Check if any resolution in the line exceeds the maximum
	for res, order := range resolutionOrder {
		if strings.Contains(resolutionLine, res) && order > maxOrder {
			return false
		}
	}

	return true
}

// meetsMinChannels checks if the audio has at least the minimum number of channels.
func meetsMinChannels(audioLine string, minChannels int) bool {
	// Check for common channel layouts
	channelPatterns := map[string]int{
		"atmos":  8,
		"7.1":    8,
		"5.1":    6,
		"stereo": 2,
		"2.0":    2,
		"mono":   1,
		"1.0":    1,
		"truehd": 8, // TrueHD is typically 7.1
		"dts-hd": 8, // DTS-HD MA is typically 7.1
		"dts:x":  8,
		"eac3":   6, // E-AC3 is typically 5.1
		"ac3":    6, // AC3 is typically 5.1
	}

	for pattern, channels := range channelPatterns {
		if strings.Contains(audioLine, pattern) && channels >= minChannels {
			return true
		}
	}

	// Try to extract channel count from patterns like "6 channels" or "8ch"
	// This is a simplified check
	if minChannels <= 2 && (strings.Contains(audioLine, "stereo") || strings.Contains(audioLine, "2.0")) {
		return true
	}
	if minChannels <= 6 && (strings.Contains(audioLine, "5.1") || strings.Contains(audioLine, "6ch")) {
		return true
	}
	if minChannels <= 8 && (strings.Contains(audioLine, "7.1") || strings.Contains(audioLine, "8ch") || strings.Contains(audioLine, "atmos")) {
		return true
	}

	return false
}
