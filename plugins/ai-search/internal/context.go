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
func (e *ContextEnricher) GetContext(ctx context.Context, userID string) (*QueryContext, error) {
	now := time.Now()
	qc := &QueryContext{
		TimeOfDay: getTimeOfDay(now),
		DayOfWeek: strings.ToLower(now.Weekday().String()),
	}

	// Try to get weather context
	if e.weatherClient != nil && userID != "" {
		weather, err := e.weatherClient.GetCurrentWeather(ctx, &pluginv1.WeatherRequest{
			UserId: userID,
		})
		if err != nil {
			e.logger.Debug("failed to get weather context", "error", err)
		} else if weather.Available {
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
			qc.TimeOfDay = weather.TimeOfDay // Use weather-derived time of day if available
		}
	}

	// If no weather-derived season, calculate from date
	if qc.Season == "" {
		// Default to northern hemisphere
		qc.Season = getSeason(now, 40.0) // Assume ~40° latitude as default
	}

	// Get holiday context
	qc.ActiveHolidays = GetActiveHolidays(now)
	qc.UpcomingHoliday = GetUpcomingHoliday(now, 14) // Look 2 weeks ahead
	qc.IsHolidaySeason = len(qc.ActiveHolidays) > 0

	// Generate suggestions based on context
	e.generateSuggestions(qc)

	return qc, nil
}

// EnrichQuery adds contextual terms to a search query.
func (e *ContextEnricher) EnrichQuery(query string, qc *QueryContext) string {
	if qc == nil {
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
func (e *ContextEnricher) generateSuggestions(qc *QueryContext) {
	// Combine suggestions from multiple sources

	// Weather-based suggestions
	if qc.Weather != nil && qc.Weather.Available {
		weatherSuggestions := e.getWeatherSuggestions(qc.Weather)
		qc.SuggestedGenres = append(qc.SuggestedGenres, weatherSuggestions.Genres...)
		qc.SuggestedMoods = append(qc.SuggestedMoods, weatherSuggestions.Moods...)
	}

	// Holiday-based suggestions
	for _, h := range qc.ActiveHolidays {
		qc.SuggestedGenres = append(qc.SuggestedGenres, h.Genres...)
		qc.SuggestedMoods = append(qc.SuggestedMoods, h.Moods...)
		qc.SuggestedKeywords = append(qc.SuggestedKeywords, h.SearchKeywords...)
	}

	// Season-based suggestions
	seasonal := GetSeasonalContent(time.Now(), 40.0) // Default latitude
	qc.SuggestedGenres = append(qc.SuggestedGenres, seasonal.Genres...)
	qc.SuggestedMoods = append(qc.SuggestedMoods, seasonal.Moods...)
	qc.SuggestedKeywords = append(qc.SuggestedKeywords, seasonal.Keywords...)

	// Time-of-day suggestions
	timeSuggestions := e.getTimeSuggestions(qc.TimeOfDay)
	qc.SuggestedGenres = append(qc.SuggestedGenres, timeSuggestions.Genres...)
	qc.SuggestedMoods = append(qc.SuggestedMoods, timeSuggestions.Moods...)

	// Deduplicate
	qc.SuggestedGenres = deduplicate(qc.SuggestedGenres)
	qc.SuggestedMoods = deduplicate(qc.SuggestedMoods)
	qc.SuggestedKeywords = deduplicate(qc.SuggestedKeywords)
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
