package internal

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

// SearchService handles semantic search operations.
type SearchService struct {
	embeddingService *EmbeddingService
	embeddingsClient pluginv1.HostEmbeddingsClient
	defaultLimit     int
	maxLimit         int
	minSimilarity    float32
	logger           *slog.Logger
}

// NewSearchService creates a new search service.
func NewSearchService(
	embeddingService *EmbeddingService,
	embeddingsClient pluginv1.HostEmbeddingsClient,
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
		embeddingsClient: embeddingsClient,
		defaultLimit:     defaultLimit,
		maxLimit:         maxLimit,
		minSimilarity:    minSimilarity,
		logger:           logger,
	}
}

// SearchParams defines parameters for semantic search.
type SearchParams struct {
	Query       string
	EntityTypes []EntityType
	Limit       int
}

// Search performs semantic search using the query text.
func (s *SearchService) Search(ctx context.Context, params SearchParams) ([]SearchResult, error) {
	if s.embeddingService == nil || s.embeddingsClient == nil {
		return nil, fmt.Errorf("search service not properly initialized")
	}

	// Detect query intent early to determine search strategy
	intent := detectQueryIntent(params.Query)

	// Generate embedding for the query
	queryEmbedding, err := s.embeddingService.EmbedSingle(ctx, params.Query)
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

	// Determine minimum similarity - lower for person/studio searches
	// to ensure we can find and boost relevant results
	minSim := s.minSimilarity
	if intent.isPersonSearch || intent.isStudioSearch || intent.isDirectorSearch ||
		intent.isActorSearch || intent.isWriterSearch || intent.isProducerSearch {
		minSim = 0.15 // Lower threshold for person/studio searches
	}

	// Search using the embeddings client
	resp, err := s.embeddingsClient.Search(ctx, &pluginv1.EmbeddingSearchRequest{
		QueryEmbedding: queryEmbedding,
		EntityTypes:    entityTypeStrs,
		Limit:          int32(fetchLimit),
		MinSimilarity:  minSim,
	})
	if err != nil {
		return nil, fmt.Errorf("search embeddings: %w", err)
	}

	// Convert results to map for merging
	resultMap := make(map[string]SearchResult)
	for _, r := range resp.Results {
		key := fmt.Sprintf("%s:%d", r.EntityType, r.EntityId)
		resultMap[key] = SearchResult{
			EntityType: EntityType(r.EntityType),
			EntityID:   r.EntityId,
			Similarity: r.Similarity,
			Text:       r.Text,
		}
	}

	// For person/studio searches, also do a text search to find exact matches
	// This ensures movies containing the person/studio are included even if
	// semantic similarity is low
	if searchTerm := s.getTextSearchTerm(intent); searchTerm != "" {
		textResults, err := s.embeddingsClient.SearchText(ctx, &pluginv1.TextSearchRequest{
			Query:       searchTerm,
			EntityTypes: entityTypeStrs,
			Limit:       100, // Fetch up to 100 text matches
		})
		if err != nil {
			s.logger.Warn("text search failed, continuing with semantic only", "error", err)
		} else {
			// Merge text search results - give them a base similarity score
			for _, r := range textResults.Results {
				key := fmt.Sprintf("%s:%d", r.EntityType, r.EntityId)
				if existing, found := resultMap[key]; found {
					// Result already in map - keep the higher similarity
					if r.Similarity > existing.Similarity {
						resultMap[key] = SearchResult{
							EntityType: EntityType(r.EntityType),
							EntityID:   r.EntityId,
							Similarity: r.Similarity,
							Text:       r.Text,
						}
					}
				} else {
					// New result from text search - give it a base score of 0.5
					// (will be boosted further by applyKeywordBoost)
					resultMap[key] = SearchResult{
						EntityType: EntityType(r.EntityType),
						EntityID:   r.EntityId,
						Similarity: 0.5, // Base score for text matches
						Text:       r.Text,
					}
				}
			}
			s.logger.Debug("text search found results",
				"term", searchTerm,
				"count", len(textResults.Results),
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

	// Final sort after diversity adjustment
	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	// Apply final limit
	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
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
	return ""
}

// detectQueryIntent identifies what type of search the user is performing.
type queryIntent struct {
	isDirectorSearch bool
	isActorSearch    bool
	isWriterSearch   bool
	isProducerSearch bool
	isStudioSearch   bool
	isGenreSearch    bool
	isLocationSearch bool
	isPersonSearch   bool   // Generic person search (name + movies)
	isLanguageSearch bool   // Searching by language (French films, Korean movies)
	directorName     string // extracted director name if searching by director
	actorName        string // extracted actor name if searching by actor
	writerName       string // extracted writer name
	producerName     string // extracted producer name
	studioName       string // extracted studio name
	personName       string // generic person name (from "Name movies" pattern)
	languageName     string // language being searched for (e.g., "french", "korean")
}

func detectQueryIntent(query string) queryIntent {
	intent := queryIntent{}
	q := strings.ToLower(query)

	// Known studio names for disambiguation - check these first before director patterns
	knownStudios := map[string]bool{
		"pixar": true, "disney": true, "marvel": true, "a24": true,
		"ghibli": true, "dreamworks": true, "warner": true, "universal": true,
		"paramount": true, "sony": true, "fox": true, "mgm": true,
		"lionsgate": true, "miramax": true, "blumhouse": true, "netflix": true,
		"hbo": true, "amazon": true, "apple": true, "hulu": true,
		"lucasfilm": true, "dc": true, "legendary": true, "annapurna": true,
		"neon": true, "searchlight": true, "focus": true, "studio ghibli": true,
	}

	// Studio patterns - check BEFORE director patterns to avoid "movies by Pixar" matching director
	studioPatterns := []string{"from studio ", "studio ", " by pixar", " by disney", " by marvel",
		" by a24", " by ghibli", " by dreamworks", " by warner", " by universal", " by paramount",
		" by sony", " by fox", " by mgm", " by lionsgate", " by miramax", " by blumhouse"}
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

	// Location patterns
	locationPatterns := []string{"set in ", "filmed in ", "takes place in "}
	for _, p := range locationPatterns {
		if strings.Contains(q, p) {
			intent.isLocationSearch = true
			break
		}
	}

	// Language/nationality patterns - detect queries like "French films", "Korean movies", "Japanese anime"
	languageMap := map[string]string{
		"french":     "french",
		"korean":     "korean",
		"japanese":   "japanese",
		"chinese":    "chinese",
		"spanish":    "spanish",
		"italian":    "italian",
		"german":     "german",
		"russian":    "russian",
		"indian":     "hindi",
		"bollywood":  "hindi",
		"swedish":    "swedish",
		"danish":     "danish",
		"norwegian":  "norwegian",
		"thai":       "thai",
		"turkish":    "turkish",
		"portuguese": "portuguese",
		"arabic":     "arabic",
		"polish":     "polish",
		"dutch":      "dutch",
		"greek":      "greek",
		"czech":      "czech",
		"hungarian":  "hungarian",
		"romanian":   "romanian",
		"vietnamese": "vietnamese",
		"indonesian": "indonesian",
		"filipino":   "filipino",
		"k-drama":    "korean",
		"kdrama":     "korean",
		"j-drama":    "japanese",
		"jdrama":     "japanese",
		"c-drama":    "chinese",
		"cdrama":     "chinese",
		"anime":      "japanese",
	}
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

	return intent
}

// extractPersonFromNameMoviesPattern extracts a person name from "Name movies" pattern
func extractPersonFromNameMoviesPattern(query string) (string, bool) {
	q := strings.ToLower(query)
	words := strings.Fields(query) // Keep original case for name extraction

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

	// Check if any name word is capitalized (proper noun)
	hasCapital := false
	for _, w := range nameWords {
		if len(w) > 0 && w[0] >= 'A' && w[0] <= 'Z' {
			hasCapital = true
			break
		}
	}

	if !hasCapital {
		return "", false
	}

	// Skip if it contains a nationality pattern like "korean movies" or "korean thriller movies"
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

	// Check for genre words that shouldn't be treated as names
	genreWords := map[string]bool{
		"action": true, "comedy": true, "drama": true, "horror": true,
		"thriller": true, "romance": true, "fantasy": true, "animation": true,
		"documentary": true, "crime": true, "mystery": true, "adventure": true,
		"sci-fi": true, "western": true, "musical": true, "war": true,
		"indie": true, "classic": true, "old": true, "new": true,
		"good": true, "best": true, "top": true, "great": true,
	}
	if len(nameWords) == 1 && genreWords[strings.ToLower(nameWords[0])] {
		return "", false
	}

	name := strings.ToLower(strings.Join(nameWords, " "))

	// Skip very short names that are likely not person names
	if len(name) < 3 {
		return "", false
	}

	// Check if this looks like a known query pattern we should ignore
	// e.g., "similar movies", "more movies", "other movies"
	skipWords := map[string]bool{
		"similar": true, "more": true, "other": true, "some": true,
		"any": true, "random": true, "popular": true, "trending": true,
		"recent": true, "latest": true, "upcoming": true, "recommended": true,
	}
	firstWord := strings.ToLower(nameWords[0])
	if skipWords[firstWord] {
		return "", false
	}

	_ = q // Silence unused warning
	return name, true
}

// applyKeywordBoost boosts results that contain query keywords in their text.
// This helps surface exact matches for directors, actors, locations, genres.
// Also applies penalties for negative signals (e.g., "without violence").
// queryOriginal is the original query with case preserved (for proper noun detection).
// queryLower is the lowercased query for matching.
func (s *SearchService) applyKeywordBoost(results []SearchResult, queryOriginal, queryLower string) []SearchResult {
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
	intent := detectQueryIntent(queryOriginal)

	// Track if we have a strong intent-based search (needs higher boost cap)
	hasStrongIntent := intent.isDirectorSearch || intent.isActorSearch ||
		intent.isWriterSearch || intent.isProducerSearch ||
		intent.isStudioSearch || intent.isPersonSearch || intent.isLanguageSearch

	for i := range results {
		textLower := strings.ToLower(results[i].Text)
		boost := float32(0.0)
		penalty := float32(0.0)

		// For director searches, apply VERY strong boost/penalty
		if intent.isDirectorSearch && intent.directorName != "" {
			directorLine := extractLine(textLower, "directed by:")
			if strings.Contains(directorLine, intent.directorName) {
				boost += 0.55 // Very strong boost for matching director
			} else {
				penalty += 0.35 // Strong penalty for non-matching director
			}
		}

		// For actor searches, apply strong boost/penalty
		if intent.isActorSearch && intent.actorName != "" {
			castLine := extractLine(textLower, "cast:")
			if strings.Contains(castLine, intent.actorName) {
				boost += 0.50 // Strong boost for matching actor
			} else {
				penalty += 0.30 // Penalty for non-matching actor
			}
		}

		// For writer searches
		if intent.isWriterSearch && intent.writerName != "" {
			writerLine := extractLine(textLower, "written by:")
			if strings.Contains(writerLine, intent.writerName) {
				boost += 0.45 // Strong boost for matching writer
			} else {
				penalty += 0.25 // Penalty for non-matching writer
			}
		}

		// For producer searches
		if intent.isProducerSearch && intent.producerName != "" {
			producerLine := extractLine(textLower, "produced by:")
			if strings.Contains(producerLine, intent.producerName) {
				boost += 0.40 // Boost for matching producer
			} else {
				penalty += 0.20 // Penalty for non-matching producer
			}
		}

		// For studio searches
		if intent.isStudioSearch && intent.studioName != "" {
			studioLine := extractLine(textLower, "studios:")
			if studioLine == "" {
				studioLine = extractLine(textLower, "network:")
			}
			if strings.Contains(studioLine, intent.studioName) {
				boost += 0.50 // Strong boost for matching studio
			} else {
				penalty += 0.30 // Penalty for non-matching studio
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
				boost += 0.60 // Strongest boost - "Name films" usually means director
			}
			if writerMatch {
				boost += 0.50
			}
			if studioMatch {
				boost += 0.55 // Strong for "Pixar movies" type queries
			}
			if producerMatch && !directorMatch && !writerMatch {
				// Only boost producer if not already director/writer
				boost += 0.35
			}
			if castMatch && !directorMatch && !writerMatch {
				// Cast-only match gets smaller boost and penalty for not being primary
				// This helps "Spielberg films" prioritize directed films over cameos
				boost += 0.25
				penalty += 0.15 // Slight penalty for cast-only match
			}

			// Strong penalty for not finding the person at all
			foundInPeople := directorMatch || writerMatch || producerMatch || castMatch || studioMatch
			if !foundInPeople {
				penalty += 0.45
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
				boost += 0.55 // Strong boost for matching language/country
			} else {
				penalty += 0.35 // Strong penalty for non-matching language
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

		// Genre matching boost
		if len(queryGenres) > 0 {
			resultGenres := extractLine(textLower, "genre:")
			genreMatches := 0
			for _, g := range queryGenres {
				if strings.Contains(resultGenres, g) {
					genreMatches++
				}
			}
			if genreMatches > 0 {
				boost += float32(genreMatches) * 0.08
			} else {
				// Penalize results that don't match the requested genre
				boost -= 0.05
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
		"drama":           "drama",
		"horror":          "horror",
		"thriller":        "thriller",
		"romance":         "romance",
		"romantic":        "romance",
		"sci-fi":          "science fiction",
		"scifi":           "science fiction",
		"science fiction": "science fiction",
		"fantasy":         "fantasy",
		"animation":       "animation",
		"animated":        "animation",
		"documentary":     "documentary",
		"crime":           "crime",
		"mystery":         "mystery",
		"western":         "western",
		"war":             "war",
		"musical":         "music",
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
			penalty += float32(selectedDirectors[director]) * 0.03
		}

		// Penalize if we already have movies from this decade
		if decade != "" && selectedDecades[decade] > 1 {
			penalty += float32(selectedDecades[decade]-1) * 0.01
		}

		// Light penalty for repeated genres (we want some genre coherence)
		for genre := range selectedGenres {
			if strings.Contains(genres, genre) && selectedGenres[genre] > 2 {
				penalty += 0.01
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
	if s.embeddingsClient == nil {
		return nil, fmt.Errorf("embeddings client not available")
	}

	// Get the embedding for the source entity
	resp, err := s.embeddingsClient.Get(ctx, &pluginv1.EmbeddingQuery{
		EntityType: string(entityType),
		EntityId:   entityID,
	})
	if err != nil {
		return nil, fmt.Errorf("get embedding: %w", err)
	}
	if !resp.Exists {
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
	searchResp, err := s.embeddingsClient.Search(ctx, &pluginv1.EmbeddingSearchRequest{
		QueryEmbedding: resp.Embedding,
		EntityTypes:    []string{string(entityType)},
		Limit:          int32(limit + 1), // +1 to account for self
		MinSimilarity:  s.minSimilarity,
	})
	if err != nil {
		return nil, fmt.Errorf("search similar: %w", err)
	}

	// Filter out the source entity
	results := make([]SearchResult, 0, len(searchResp.Results))
	for _, r := range searchResp.Results {
		if r.EntityType == string(entityType) && r.EntityId == entityID {
			continue // Skip self
		}
		results = append(results, SearchResult{
			EntityType: EntityType(r.EntityType),
			EntityID:   r.EntityId,
			Similarity: r.Similarity,
			Text:       r.Text,
		})
		if len(results) >= limit {
			break
		}
	}

	return results, nil
}

// GetStatus returns the current search/indexing status.
func (s *SearchService) GetStatus(ctx context.Context) (*IndexingStatus, error) {
	if s.embeddingsClient == nil {
		return nil, fmt.Errorf("embeddings client not available")
	}

	status := &IndexingStatus{
		Stats: make(map[EntityType]EntityStats),
	}

	// Get counts for each entity type
	for _, entityType := range []EntityType{
		EntityMovie, EntityTVShow, EntityTVEpisode,
		EntityMusicArtist, EntityMusicAlbum, EntityMusicTrack,
	} {
		resp, err := s.embeddingsClient.CountByType(ctx, &pluginv1.EntityTypeQuery{
			EntityType: string(entityType),
		})
		if err != nil {
			s.logger.Warn("failed to count embeddings", "type", entityType, "error", err)
			continue
		}
		status.Stats[entityType] = EntityStats{
			Indexed: resp.Count,
		}
	}

	return status, nil
}
