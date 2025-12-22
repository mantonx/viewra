package internal

import (
	"context"
	"log/slog"
	"strings"
	"time"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

// QueryContext holds contextual information used to enrich search queries.
type QueryContext struct {
	// Weather context (if available)
	Weather *WeatherContext

	// Time context
	TimeOfDay string // morning, afternoon, evening, night
	DayOfWeek string // monday, tuesday, etc.
	Season    string // spring, summer, fall, winter

	// Holiday context
	ActiveHolidays  []Holiday
	UpcomingHoliday *Holiday
	IsHolidaySeason bool

	// Derived suggestions
	SuggestedGenres   []string
	SuggestedMoods    []string
	SuggestedKeywords []string
}

// WeatherContext holds weather-related context.
type WeatherContext struct {
	Available     bool
	Temperature   float64
	Humidity      int
	IsDay         bool
	Condition     string // sunny, cloudy, rainy, snowy, stormy, foggy
	Precipitation float64
	CloudCover    int
}

// ContextEnricher enriches search queries with contextual information.
type ContextEnricher struct {
	weatherClient pluginv1.HostWeatherClient
	logger        *slog.Logger
}

// NewContextEnricher creates a new context enricher.
func NewContextEnricher(weatherClient pluginv1.HostWeatherClient, logger *slog.Logger) *ContextEnricher {
	return &ContextEnricher{
		weatherClient: weatherClient,
		logger:        logger,
	}
}

// GetContext retrieves the current context for a user.
// Privacy: Location-based context (weather, season, holidays) is ONLY included
// when the user has explicitly enabled location sharing AND weather data is available.
// Time-based context (time of day, day of week) uses server time and is always safe.
func (e *ContextEnricher) GetContext(ctx context.Context, userID string) (*QueryContext, error) {
	now := time.Now()
	qc := &QueryContext{
		TimeOfDay: getTimeOfDay(now),
		DayOfWeek: strings.ToLower(now.Weekday().String()),
	}

	// Try to get weather context - ONLY if user has enabled location sharing
	// The weather service returns Available=false if location is disabled
	locationEnabled := false
	if e.weatherClient != nil && userID != "" {
		weather, err := e.weatherClient.GetCurrentWeather(ctx, &pluginv1.WeatherRequest{
			UserId: userID,
		})
		if err != nil {
			if e.logger != nil {
				e.logger.Debug("failed to get weather context", "error", err)
			}
		} else if weather.Available {
			// User has explicitly opted in to location sharing
			locationEnabled = true
			qc.Weather = &WeatherContext{
				Available:     true,
				Temperature:   float64(weather.Temperature),
				Humidity:      int(weather.Humidity),
				IsDay:         weather.IsDay,
				Condition:     weather.Condition,
				Precipitation: float64(weather.Precipitation),
				CloudCover:    int(weather.CloudCover),
			}
			qc.Season = weather.Season
			qc.TimeOfDay = weather.TimeOfDay // Use user's local time of day
		}
	}

	// PRIVACY: Only include location-derived context if user explicitly enabled it
	// This prevents leaking assumptions about user's location (hemisphere, country, etc.)
	if locationEnabled {
		// Get holiday context - only for users who opted in
		// TODO: In the future, determine holidays based on user's actual country
		qc.ActiveHolidays = GetActiveHolidays(now)
		qc.UpcomingHoliday = GetUpcomingHoliday(now, 14) // Look 2 weeks ahead
		qc.IsHolidaySeason = len(qc.ActiveHolidays) > 0
	}
	// Note: Season is already set from weather response when location is enabled.
	// We intentionally do NOT fall back to a default season/hemisphere when location
	// is disabled, as that would leak an assumption about the user's location.

	// Generate suggestions based on available context
	e.generateSuggestions(qc)

	return qc, nil
}

// EnrichQuery adds contextual terms to a search query.
// Only enriches vague/mood-based queries; skips specific queries about
// directors, actors, locations, titles, etc.
func (e *ContextEnricher) EnrichQuery(query string, qc *QueryContext) string {
	if qc == nil {
		return query
	}

	// Skip enrichment for specific queries - they don't need context
	if isSpecificQuery(query) {
		if e.logger != nil {
			e.logger.Debug("skipping context enrichment for specific query", "query", query)
		}
		return query
	}

	var enrichments []string

	// Add weather-based enrichments
	if qc.Weather != nil && qc.Weather.Available {
		weatherTerms := e.getWeatherEnrichments(qc.Weather)
		enrichments = append(enrichments, weatherTerms...)
	}

	// Add time-based enrichments
	timeTerms := e.getTimeEnrichments(qc)
	enrichments = append(enrichments, timeTerms...)

	// Add holiday enrichments (subtle boost)
	if len(qc.ActiveHolidays) > 0 {
		for _, h := range qc.ActiveHolidays {
			// Only add a few keywords to avoid overwhelming
			if len(h.SearchKeywords) > 0 {
				enrichments = append(enrichments, h.SearchKeywords[0])
			}
		}
	}

	if len(enrichments) == 0 {
		return query
	}

	// Combine original query with enrichments
	// Format: "original query. Context: enrichment1, enrichment2"
	return query + ". Context: " + strings.Join(enrichments, ", ")
}

// isSpecificQuery returns true if the query is specific enough that context
// enrichment would likely hurt rather than help results.
//
// Specific queries include:
//   - Director/actor references: "directed by spielberg", "starring tom hanks"
//   - Location-based: "set in tokyo", "filmed in new york"
//   - Title references: "like inception", "similar to the matrix"
//   - Specific genres with modifiers: "french new wave", "japanese horror"
//   - Year/decade references: "from the 80s", "1990s action"
//   - Specific subject matter: "mafia", "heist", "war"
//   - Simple genre queries: "action movie", "comedy", "horror films"
//
// Vague queries that SHOULD be enriched:
//   - Mood-based: "something fun", "I'm feeling sad"
//   - Time-based: "something to watch tonight"
//   - Open-ended: "recommend me a movie", "what should I watch"
func isSpecificQuery(query string) bool {
	q := strings.ToLower(query)

	// First check if query is explicitly vague/open-ended
	vaguePatterns := []string{
		"something to watch", "something for",
		"what should i", "what to watch",
		"recommend", "suggestion",
		"i'm feeling", "i feel", "im feeling",
		"in the mood", "mood for",
		"tonight", "this evening", "right now",
		"good movie", "nice film",
	}
	for _, pattern := range vaguePatterns {
		if strings.Contains(q, pattern) {
			return false // Explicitly vague, should be enriched
		}
	}

	// Simple genre queries should NOT be enriched with holiday/weather context
	// These are specific enough that the user knows what they want
	genrePatterns := []string{
		"action", "comedy", "drama", "horror", "thriller", "romance",
		"sci-fi", "scifi", "science fiction", "fantasy", "animation",
		"documentary", "crime", "mystery", "musical", "adventure",
		"war film", "war movie", "western", "noir", "indie",
	}
	for _, genre := range genrePatterns {
		if strings.Contains(q, genre) {
			return true // Genre query = specific, don't add holiday context
		}
	}

	// Patterns that indicate a specific query
	specificPatterns := []string{
		// Director/creator patterns
		"directed by", "director", "by director",
		"written by", "screenplay by",
		"produced by", "from producer",
		"created by",

		// Actor/cast patterns
		"starring", "with actor", "featuring",
		"played by", "acted by",

		// Location patterns
		"set in", "filmed in", "takes place in",
		"shot in", "located in", "based in",

		// Reference patterns
		"like the movie", "similar to", "movies like",
		"based on the book", "based on the novel",
		"remake of", "sequel to", "prequel to",

		// Nationality/origin patterns (usually specific)
		"french ", "japanese ", "korean ", "italian ",
		"german ", "spanish ", "british ", "indian ",
		"chinese ", "swedish ", "danish ", "russian ",

		// Era/decade patterns
		"from the ", "from 19", "from 20",
		"1950s", "1960s", "1970s", "1980s", "1990s", "2000s", "2010s", "2020s",
		"50s ", "60s ", "70s ", "80s ", "90s ",
		"classic ", "vintage ", "golden age",

		// Specific film movements/styles
		"new wave", "film noir", "neorealism", "expressionist",
		"dogme 95", "mumblecore", "giallo",

		// Award patterns
		"oscar", "academy award", "cannes", "sundance",
		"golden globe", "bafta", "palme d'or",

		// Studio patterns
		"from studio", "disney", "pixar", "marvel", "dc comics",
		"a24 ", "blumhouse", "ghibli",

		// Specific subject matter that implies focused search
		"mafia", "gangster", "mob ",
		"heist", "robbery", "bank job",
		"serial killer", "slasher",
		"zombie", "vampire", "werewolf",
		"superhero", "supervillain",
		"world war", "vietnam", "civil war",
		"spy ", "espionage", "secret agent",
		"martial arts", "kung fu", "samurai",
		"western ", "cowboy",
		"space ", "astronaut", "alien ",
		"robot", "android", "cyborg",
		"dinosaur", "monster ",
		"wizard", "witch", "magic ",
		"pirate", "treasure",
	}

	for _, pattern := range specificPatterns {
		if strings.Contains(q, pattern) {
			return true
		}
	}

	// Check for "[Name] movies/films" pattern (e.g., "Spielberg movies", "Hanks films")
	if hasNamePlusMediaPattern(query) {
		return true
	}

	// Check for CamelCase names (e.g., "DiCaprio", "McQueen", "LaBeouf")
	if hasCamelCaseName(query) {
		return true
	}

	// Check for specific proper nouns (names with capital letters in original)
	// This catches queries like "Hitchcock thriller" or "Tarantino film"
	words := strings.Fields(query)
	properNounCount := 0
	hasMovieKeyword := false
	for _, word := range words {
		lower := strings.ToLower(word)
		// Track if query mentions movies/films
		if lower == "movie" || lower == "movies" || lower == "film" || lower == "films" {
			hasMovieKeyword = true
		}
		// Skip common title words
		if len(word) > 2 && word[0] >= 'A' && word[0] <= 'Z' {
			// Skip common non-name capitalized words
			if lower != "the" && lower != "movie" && lower != "film" &&
				lower != "movies" && lower != "films" &&
				lower != "something" && lower != "looking" && lower != "want" &&
				lower != "need" && lower != "feeling" && lower != "watch" &&
				lower != "for" && lower != "with" && lower != "show" &&
				lower != "shows" && lower != "series" {
				// Check if this word looks like a surname
				if looksLikeSurname(word) {
					return true // Single surname is specific enough
				}
				properNounCount++
			}
		}
	}

	// If query has a proper noun + movie keyword, it's likely a person search
	// (e.g., "Spielberg movies" = 1 proper noun + movie keyword)
	if properNounCount >= 1 && hasMovieKeyword {
		return true
	}

	// If query has 2+ proper nouns, it's likely specific (e.g., "Alfred Hitchcock")
	if properNounCount >= 2 {
		return true
	}

	return false
}

// hasNamePlusMediaPattern detects queries like "Spielberg movies", "Tom Hanks films"
func hasNamePlusMediaPattern(query string) bool {
	q := strings.ToLower(query)
	words := strings.Fields(q)
	if len(words) < 2 {
		return false
	}

	// Check if last word is a media type
	lastWord := words[len(words)-1]
	mediaWords := []string{"movies", "films", "movie", "film", "shows", "series"}
	isMediaWord := false
	for _, mw := range mediaWords {
		if lastWord == mw {
			isMediaWord = true
			break
		}
	}
	if !isMediaWord {
		return false
	}

	// Check if any preceding word is capitalized in the original query
	originalWords := strings.Fields(query)
	for i := 0; i < len(originalWords)-1; i++ {
		word := originalWords[i]
		if len(word) > 1 && word[0] >= 'A' && word[0] <= 'Z' {
			return true // Found "Name movies" pattern
		}
	}
	return false
}

// hasCamelCaseName detects CamelCase names like "DiCaprio", "McQueen", "LaBeouf"
func hasCamelCaseName(query string) bool {
	words := strings.Fields(query)
	for _, word := range words {
		if len(word) < 3 {
			continue
		}
		// Check for capital letter after lowercase (e.g., "DiCaprio")
		for i := 1; i < len(word); i++ {
			if word[i] >= 'A' && word[i] <= 'Z' && word[i-1] >= 'a' && word[i-1] <= 'z' {
				return true
			}
		}
	}
	return false
}

// looksLikeSurname detects words that appear to be surnames based on common suffixes
func looksLikeSurname(word string) bool {
	if len(word) < 4 {
		return false
	}
	lower := strings.ToLower(word)

	// Common surname suffixes from various origins
	surnameSuffixes := []string{
		// Germanic
		"-berg", "-stein", "-man", "-mann", "-son", "-sen",
		// Italian
		"-ini", "-ino", "-elli", "-etti", "-oni", "-ucci",
		// Spanish/Portuguese
		"-ez", "-es", "-os",
		// Slavic
		"-ski", "-sky", "-ovich", "-ova", "-enko",
		// French
		"-eau", "-eux",
		// Irish/Scottish
		"-ley", "-ly",
		// Others
		"-ton", "-ford", "-wood",
	}

	for _, suffix := range surnameSuffixes {
		if strings.HasSuffix(lower, suffix[1:]) { // Remove leading dash for comparison
			return true
		}
	}

	// Check for "Mc" or "Mac" prefix (Scottish/Irish)
	if strings.HasPrefix(lower, "mc") || strings.HasPrefix(lower, "mac") {
		return true
	}

	// Check for "O'" prefix (Irish)
	if len(word) > 2 && word[0] == 'O' && word[1] == '\'' {
		return true
	}

	return false
}

// getWeatherEnrichments returns search terms based on weather conditions.
func (e *ContextEnricher) getWeatherEnrichments(w *WeatherContext) []string {
	var terms []string

	switch w.Condition {
	case "rainy":
		terms = append(terms, "cozy", "indoor", "comfort viewing")
		if w.Temperature < 15 {
			terms = append(terms, "warm", "hygge")
		}
	case "snowy":
		terms = append(terms, "winter", "cozy", "warm", "holiday")
	case "stormy":
		terms = append(terms, "intense", "dramatic", "atmospheric")
	case "foggy":
		terms = append(terms, "mysterious", "atmospheric", "moody")
	case "sunny", "clear":
		if w.IsDay {
			terms = append(terms, "uplifting", "bright")
			if w.Temperature > 25 {
				terms = append(terms, "summer", "adventure")
			}
		} else {
			terms = append(terms, "stargazing", "night sky", "romantic evening")
		}
	case "cloudy", "overcast":
		terms = append(terms, "contemplative", "thoughtful")
	}

	// Temperature-based suggestions
	if w.Temperature < 5 {
		terms = append(terms, "warming", "fireside")
	} else if w.Temperature > 30 {
		terms = append(terms, "cooling", "escapist", "beach")
	}

	return terms
}

// getTimeEnrichments returns search terms based on time of day and day of week.
func (e *ContextEnricher) getTimeEnrichments(qc *QueryContext) []string {
	var terms []string

	switch qc.TimeOfDay {
	case "morning":
		terms = append(terms, "light", "uplifting", "energizing")
	case "afternoon":
		terms = append(terms, "adventure", "action")
	case "evening":
		terms = append(terms, "relaxing", "entertainment")
	case "night":
		terms = append(terms, "atmospheric", "immersive", "late night")
		if qc.DayOfWeek == "friday" || qc.DayOfWeek == "saturday" {
			terms = append(terms, "party", "exciting")
		}
	}

	// Weekend-specific
	if qc.DayOfWeek == "saturday" || qc.DayOfWeek == "sunday" {
		terms = append(terms, "binge-worthy", "marathon")
	}

	return terms
}

// generateSuggestions populates suggested genres/moods based on context.
// Privacy: Only uses location-derived suggestions (weather, season, holidays) when
// the user has explicitly opted in (indicated by qc.Weather.Available or qc.Season being set).
func (e *ContextEnricher) generateSuggestions(qc *QueryContext) {
	// Combine suggestions from multiple sources

	// Weather-based suggestions - only when user opted in
	if qc.Weather != nil && qc.Weather.Available {
		weatherSuggestions := e.getWeatherSuggestions(qc.Weather)
		qc.SuggestedGenres = append(qc.SuggestedGenres, weatherSuggestions.Genres...)
		qc.SuggestedMoods = append(qc.SuggestedMoods, weatherSuggestions.Moods...)
	}

	// Holiday-based suggestions - only populated when user opted in (see GetContext)
	for _, h := range qc.ActiveHolidays {
		qc.SuggestedGenres = append(qc.SuggestedGenres, h.Genres...)
		qc.SuggestedMoods = append(qc.SuggestedMoods, h.Moods...)
		qc.SuggestedKeywords = append(qc.SuggestedKeywords, h.SearchKeywords...)
	}

	// Season-based suggestions - ONLY when season is explicitly known from location
	// We do NOT assume a default hemisphere/latitude as that leaks location assumptions
	if qc.Season != "" {
		seasonal := getSeasonalSuggestions(qc.Season)
		qc.SuggestedGenres = append(qc.SuggestedGenres, seasonal.Genres...)
		qc.SuggestedMoods = append(qc.SuggestedMoods, seasonal.Moods...)
		qc.SuggestedKeywords = append(qc.SuggestedKeywords, seasonal.Keywords...)
	}

	// Time-of-day suggestions - safe to use (based on server time, not location)
	timeSuggestions := e.getTimeSuggestions(qc.TimeOfDay)
	qc.SuggestedGenres = append(qc.SuggestedGenres, timeSuggestions.Genres...)
	qc.SuggestedMoods = append(qc.SuggestedMoods, timeSuggestions.Moods...)

	// Deduplicate
	qc.SuggestedGenres = deduplicate(qc.SuggestedGenres)
	qc.SuggestedMoods = deduplicate(qc.SuggestedMoods)
	qc.SuggestedKeywords = deduplicate(qc.SuggestedKeywords)
}

// getSeasonalSuggestions returns content suggestions for a known season.
// This is only called when the user has opted in to location sharing.
func getSeasonalSuggestions(season string) struct {
	Genres   []string
	Moods    []string
	Keywords []string
} {
	switch season {
	case "spring":
		return struct {
			Genres   []string
			Moods    []string
			Keywords []string
		}{
			Genres:   []string{"romance", "comedy", "adventure"},
			Moods:    []string{"uplifting", "fresh", "hopeful", "light"},
			Keywords: []string{"spring", "renewal", "bloom", "new beginnings"},
		}
	case "summer":
		return struct {
			Genres   []string
			Moods    []string
			Keywords []string
		}{
			Genres:   []string{"action", "adventure", "comedy", "blockbuster"},
			Moods:    []string{"exciting", "fun", "escapist", "adventurous"},
			Keywords: []string{"summer", "beach", "vacation", "road trip", "outdoor"},
		}
	case "fall":
		return struct {
			Genres   []string
			Moods    []string
			Keywords []string
		}{
			Genres:   []string{"drama", "thriller", "horror", "mystery"},
			Moods:    []string{"atmospheric", "cozy", "mysterious", "contemplative"},
			Keywords: []string{"autumn", "fall", "harvest", "change"},
		}
	case "winter":
		return struct {
			Genres   []string
			Moods    []string
			Keywords []string
		}{
			Genres:   []string{"family", "drama", "romance", "fantasy"},
			Moods:    []string{"cozy", "heartwarming", "magical", "introspective"},
			Keywords: []string{"winter", "snow", "cold", "fireplace", "hygge"},
		}
	default:
		return struct {
			Genres   []string
			Moods    []string
			Keywords []string
		}{}
	}
}

type suggestions struct {
	Genres []string
	Moods  []string
}

func (e *ContextEnricher) getWeatherSuggestions(w *WeatherContext) suggestions {
	s := suggestions{}

	switch w.Condition {
	case "rainy":
		s.Genres = []string{"drama", "romance", "mystery"}
		s.Moods = []string{"cozy", "introspective", "romantic"}
	case "snowy":
		s.Genres = []string{"family", "holiday", "adventure"}
		s.Moods = []string{"cozy", "magical", "festive"}
	case "stormy":
		s.Genres = []string{"thriller", "horror", "disaster"}
		s.Moods = []string{"intense", "suspenseful", "dramatic"}
	case "sunny", "clear":
		if w.IsDay {
			s.Genres = []string{"comedy", "adventure", "action"}
			s.Moods = []string{"uplifting", "fun", "exciting"}
		} else {
			s.Genres = []string{"romance", "drama", "sci-fi"}
			s.Moods = []string{"romantic", "contemplative", "epic"}
		}
	case "cloudy", "overcast":
		s.Genres = []string{"drama", "indie", "documentary"}
		s.Moods = []string{"thoughtful", "artistic", "contemplative"}
	case "foggy":
		s.Genres = []string{"mystery", "thriller", "noir"}
		s.Moods = []string{"atmospheric", "mysterious", "moody"}
	default:
		s.Genres = []string{"drama", "comedy"}
		s.Moods = []string{"entertaining"}
	}

	return s
}

func (e *ContextEnricher) getTimeSuggestions(timeOfDay string) suggestions {
	s := suggestions{}

	switch timeOfDay {
	case "morning":
		s.Genres = []string{"documentary", "comedy", "animation"}
		s.Moods = []string{"light", "educational", "uplifting"}
	case "afternoon":
		s.Genres = []string{"action", "adventure", "family"}
		s.Moods = []string{"exciting", "fun", "entertaining"}
	case "evening":
		s.Genres = []string{"drama", "comedy", "romance"}
		s.Moods = []string{"relaxing", "engaging", "heartwarming"}
	case "night":
		s.Genres = []string{"thriller", "horror", "sci-fi", "noir"}
		s.Moods = []string{"atmospheric", "intense", "immersive"}
	default:
		s.Genres = []string{}
		s.Moods = []string{}
	}

	return s
}

func getTimeOfDay(t time.Time) string {
	hour := t.Hour()
	switch {
	case hour >= 5 && hour < 12:
		return "morning"
	case hour >= 12 && hour < 17:
		return "afternoon"
	case hour >= 17 && hour < 21:
		return "evening"
	default:
		return "night"
	}
}

func getSeason(t time.Time, latitude float64) string {
	month := t.Month()
	isNorthern := latitude >= 0

	switch {
	case month >= 3 && month <= 5:
		if isNorthern {
			return "spring"
		}
		return "fall"
	case month >= 6 && month <= 8:
		if isNorthern {
			return "summer"
		}
		return "winter"
	case month >= 9 && month <= 11:
		if isNorthern {
			return "fall"
		}
		return "spring"
	default:
		if isNorthern {
			return "winter"
		}
		return "summer"
	}
}

func deduplicate(items []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(items))
	for _, item := range items {
		lower := strings.ToLower(item)
		if !seen[lower] {
			seen[lower] = true
			result = append(result, item)
		}
	}
	return result
}
