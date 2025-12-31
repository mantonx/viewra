package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

// GetSearchProviderInfo returns metadata about this search provider.
// This is used by the home service to understand provider capabilities.
func (p *SemanticSearchPlugin) GetSearchProviderInfo(ctx context.Context) (*sdk.SearchProviderInfo, error) {
	return &sdk.SearchProviderInfo{
		ID:          "semantic-search",
		Name:        "AI-Powered Search",
		Description: "Semantic search using AI embeddings with contextual suggestions",
		Icon:        "sparkles",
		Priority:    100, // High priority - AI-powered
		Capabilities: []string{
			"natural_language",
			"suggestions",
			"context_aware",
		},
	}, nil
}

// GetSuggestions returns contextual search suggestions for the search hero widget.
// Suggestions are personalized based on time, weather, and other context factors.
func (p *SemanticSearchPlugin) GetSuggestions(ctx context.Context, req *sdk.SuggestionRequest) (*sdk.SuggestionResponse, error) {
	p.mu.RLock()
	contextEnricher := p.contextEnricher
	p.mu.RUnlock()

	if contextEnricher == nil {
		return &sdk.SuggestionResponse{
			Suggestions: getDefaultSuggestions(),
		}, nil
	}

	// Get context for this user
	qc, err := contextEnricher.GetContext(ctx, req.UserID)
	if err != nil {
		p.Log().Debug("failed to get context for suggestions", "error", err)
		return &sdk.SuggestionResponse{
			Suggestions: getDefaultSuggestions(),
		}, nil
	}

	suggestions := buildContextualSuggestions(qc, req.Limit)

	return &sdk.SuggestionResponse{
		Suggestions: suggestions,
	}, nil
}

// buildContextualSuggestions creates suggestions based on the current context.
func buildContextualSuggestions(qc *QueryContext, limit int) []sdk.Suggestion {
	if limit <= 0 {
		limit = 6
	}

	var suggestions []sdk.Suggestion

	// Add weather-based suggestion if available
	if qc.Weather != nil && qc.Weather.Available {
		weatherSuggestion := getWeatherSuggestion(qc.Weather, qc.Season)
		if weatherSuggestion != nil {
			suggestions = append(suggestions, *weatherSuggestion)
		}
	}

	// Add time-based suggestion
	timeSuggestion := getTimeSuggestion(qc.TimeOfDay, qc.DayOfWeek)
	if timeSuggestion != nil {
		suggestions = append(suggestions, *timeSuggestion)
	}

	// Add holiday suggestion if active
	if len(qc.ActiveHolidays) > 0 {
		holiday := qc.ActiveHolidays[0]
		suggestions = append(suggestions, sdk.Suggestion{
			ID:          fmt.Sprintf("holiday-%s", strings.ToLower(strings.ReplaceAll(holiday.Name, " ", "-"))),
			Label:       holiday.Name + " picks",
			Icon:        getHolidayIcon(holiday.Name),
			Description: "Seasonal favorites",
			Style:       "accent",
			Action: sdk.SuggestionAction{
				Type:  "search",
				Query: strings.Join(holiday.SearchKeywords[:min(3, len(holiday.SearchKeywords))], " "),
			},
		})
	}

	// Add genre-based suggestions from context
	genreSuggestions := getGenreSuggestions(qc.SuggestedGenres)
	suggestions = append(suggestions, genreSuggestions...)

	// Add mood-based suggestions
	moodSuggestions := getMoodSuggestions(qc.SuggestedMoods)
	suggestions = append(suggestions, moodSuggestions...)

	// Add fallback suggestions if we don't have enough
	if len(suggestions) < limit {
		defaults := getDefaultSuggestions()
		for _, d := range defaults {
			if len(suggestions) >= limit {
				break
			}
			// Only add if not already present
			exists := false
			for _, s := range suggestions {
				if s.ID == d.ID {
					exists = true
					break
				}
			}
			if !exists {
				suggestions = append(suggestions, d)
			}
		}
	}

	// Limit results
	if len(suggestions) > limit {
		suggestions = suggestions[:limit]
	}

	return suggestions
}

// getWeatherSuggestion creates a suggestion based on weather conditions.
func getWeatherSuggestion(w *WeatherContext, season string) *sdk.Suggestion {
	switch w.Condition {
	case "rainy":
		return &sdk.Suggestion{
			ID:          "weather-rainy",
			Label:       "Cozy rainy day movies",
			Icon:        "cloud-rain",
			Description: "Perfect for indoor viewing",
			Style:       "primary",
			Action: sdk.SuggestionAction{
				Type:  "search",
				Query: "cozy comfort movie indoor",
			},
		}
	case "snowy":
		return &sdk.Suggestion{
			ID:          "weather-snowy",
			Label:       "Snowy day classics",
			Icon:        "snowflake",
			Description: "Winter warmth",
			Style:       "primary",
			Action: sdk.SuggestionAction{
				Type:  "search",
				Query: "winter cozy heartwarming",
			},
		}
	case "stormy":
		return &sdk.Suggestion{
			ID:          "weather-stormy",
			Label:       "Storm watching thrillers",
			Icon:        "cloud-lightning",
			Description: "Intense atmospheric films",
			Style:       "primary",
			Action: sdk.SuggestionAction{
				Type:  "search",
				Query: "intense thriller dramatic atmospheric",
			},
		}
	case "sunny", "clear":
		if w.IsDay {
			return &sdk.Suggestion{
				ID:          "weather-sunny",
				Label:       "Feel-good adventures",
				Icon:        "sun",
				Description: "Bright uplifting stories",
				Style:       "primary",
				Action: sdk.SuggestionAction{
					Type:  "search",
					Query: "uplifting adventure fun",
				},
			}
		}
		return &sdk.Suggestion{
			ID:          "weather-clear-night",
			Label:       "Starlit cinema",
			Icon:        "moon-star",
			Description: "Night sky vibes",
			Style:       "primary",
			Action: sdk.SuggestionAction{
				Type:  "search",
				Query: "romantic epic night stars",
			},
		}
	case "cloudy", "overcast":
		return &sdk.Suggestion{
			ID:          "weather-cloudy",
			Label:       "Thoughtful dramas",
			Icon:        "cloud",
			Description: "Contemplative stories",
			Style:       "secondary",
			Action: sdk.SuggestionAction{
				Type:  "search",
				Query: "thoughtful drama contemplative",
			},
		}
	case "foggy":
		return &sdk.Suggestion{
			ID:          "weather-foggy",
			Label:       "Mysterious & moody",
			Icon:        "cloud-fog",
			Description: "Atmospheric mysteries",
			Style:       "secondary",
			Action: sdk.SuggestionAction{
				Type:  "search",
				Query: "mysterious atmospheric noir",
			},
		}
	default:
		return nil
	}
}

// getTimeSuggestion creates a suggestion based on time of day.
func getTimeSuggestion(timeOfDay, dayOfWeek string) *sdk.Suggestion {
	isWeekend := dayOfWeek == "saturday" || dayOfWeek == "sunday"
	isFridayNight := dayOfWeek == "friday" && (timeOfDay == "evening" || timeOfDay == "night")

	switch timeOfDay {
	case "morning":
		return &sdk.Suggestion{
			ID:          "time-morning",
			Label:       "Morning watch",
			Icon:        "sunrise",
			Description: "Light & energizing",
			Style:       "secondary",
			Action: sdk.SuggestionAction{
				Type:  "search",
				Query: "light uplifting documentary",
			},
		}
	case "afternoon":
		if isWeekend {
			return &sdk.Suggestion{
				ID:          "time-weekend-afternoon",
				Label:       "Weekend adventures",
				Icon:        "compass",
				Description: "Epic journeys",
				Style:       "secondary",
				Action: sdk.SuggestionAction{
					Type:  "search",
					Query: "adventure action epic journey",
				},
			}
		}
		return nil // No specific afternoon suggestion during weekdays
	case "evening":
		return &sdk.Suggestion{
			ID:          "time-evening",
			Label:       "Evening relaxation",
			Icon:        "sofa",
			Description: "Unwind after your day",
			Style:       "secondary",
			Action: sdk.SuggestionAction{
				Type:  "search",
				Query: "relaxing entertaining heartwarming",
			},
		}
	case "night":
		if isFridayNight || isWeekend {
			return &sdk.Suggestion{
				ID:          "time-weekend-night",
				Label:       "Late night gems",
				Icon:        "moon",
				Description: "Night owl favorites",
				Style:       "secondary",
				Action: sdk.SuggestionAction{
					Type:  "search",
					Query: "exciting cult classic late night",
				},
			}
		}
		return &sdk.Suggestion{
			ID:          "time-night",
			Label:       "Night watching",
			Icon:        "moon",
			Description: "Atmospheric & immersive",
			Style:       "secondary",
			Action: sdk.SuggestionAction{
				Type:  "search",
				Query: "atmospheric immersive thriller",
			},
		}
	default:
		return nil
	}
}

// getGenreSuggestions creates suggestions from suggested genres.
func getGenreSuggestions(genres []string) []sdk.Suggestion {
	if len(genres) == 0 {
		return nil
	}

	// Map genres to suggestion configs
	genreConfigs := map[string]struct {
		label string
		icon  string
	}{
		"action":      {"Action movies", "zap"},
		"adventure":   {"Adventures", "compass"},
		"comedy":      {"Comedies", "laugh"},
		"drama":       {"Dramas", "theater"},
		"horror":      {"Horror", "ghost"},
		"thriller":    {"Thrillers", "alert-triangle"},
		"romance":     {"Romance", "heart"},
		"sci-fi":      {"Sci-Fi", "rocket"},
		"fantasy":     {"Fantasy", "wand"},
		"mystery":     {"Mysteries", "search"},
		"documentary": {"Documentaries", "film"},
		"animation":   {"Animation", "palette"},
		"family":      {"Family films", "users"},
		"indie":       {"Indie films", "star"},
	}

	var suggestions []sdk.Suggestion
	seen := make(map[string]bool)

	for _, genre := range genres {
		if seen[genre] {
			continue
		}
		seen[genre] = true

		config, ok := genreConfigs[strings.ToLower(genre)]
		if !ok {
			continue
		}

		suggestions = append(suggestions, sdk.Suggestion{
			ID:    fmt.Sprintf("genre-%s", strings.ToLower(genre)),
			Label: config.label,
			Icon:  config.icon,
			Style: "secondary",
			Action: sdk.SuggestionAction{
				Type: "filter",
				Filter: map[string]string{
					"genre": genre,
				},
			},
		})

		if len(suggestions) >= 2 { // Limit genre suggestions
			break
		}
	}

	return suggestions
}

// getMoodSuggestions creates suggestions from suggested moods.
func getMoodSuggestions(moods []string) []sdk.Suggestion {
	if len(moods) == 0 {
		return nil
	}

	// Map moods to suggestion configs
	moodConfigs := map[string]struct {
		label       string
		icon        string
		description string
	}{
		"cozy":          {"Cozy vibes", "coffee", "Warm & comforting"},
		"uplifting":     {"Feel good", "smile", "Mood boosters"},
		"exciting":      {"Exciting", "zap", "Adrenaline rush"},
		"relaxing":      {"Chill out", "wind", "Easy watching"},
		"atmospheric":   {"Atmospheric", "cloud", "Immersive experiences"},
		"heartwarming":  {"Heartwarming", "heart", "Touching stories"},
		"intense":       {"Intense", "flame", "Edge of your seat"},
		"contemplative": {"Thought-provoking", "lightbulb", "Makes you think"},
		"fun":           {"Fun times", "party-popper", "Entertainment"},
		"romantic":      {"Romantic", "heart", "Love stories"},
	}

	var suggestions []sdk.Suggestion
	seen := make(map[string]bool)

	for _, mood := range moods {
		if seen[mood] {
			continue
		}
		seen[mood] = true

		config, ok := moodConfigs[strings.ToLower(mood)]
		if !ok {
			continue
		}

		suggestions = append(suggestions, sdk.Suggestion{
			ID:          fmt.Sprintf("mood-%s", strings.ToLower(mood)),
			Label:       config.label,
			Icon:        config.icon,
			Description: config.description,
			Style:       "secondary",
			Action: sdk.SuggestionAction{
				Type:  "search",
				Query: mood,
			},
		})

		if len(suggestions) >= 2 { // Limit mood suggestions
			break
		}
	}

	return suggestions
}

// getDefaultSuggestions returns fallback suggestions when context is unavailable.
func getDefaultSuggestions() []sdk.Suggestion {
	return []sdk.Suggestion{
		{
			ID:          "default-popular",
			Label:       "Popular now",
			Icon:        "trending-up",
			Description: "What everyone's watching",
			Style:       "primary",
			Action: sdk.SuggestionAction{
				Type: "navigate",
				URL:  "/browse/popular",
			},
		},
		{
			ID:          "default-recommend",
			Label:       "Recommended for you",
			Icon:        "sparkles",
			Description: "Based on your taste",
			Style:       "primary",
			Action: sdk.SuggestionAction{
				Type: "navigate",
				URL:  "/browse/recommended",
			},
		},
		{
			ID:          "default-new",
			Label:       "Recently added",
			Icon:        "plus-circle",
			Description: "Fresh additions",
			Style:       "secondary",
			Action: sdk.SuggestionAction{
				Type: "navigate",
				URL:  "/browse/recent",
			},
		},
		{
			ID:          "default-classics",
			Label:       "Classics",
			Icon:        "award",
			Description: "Timeless favorites",
			Style:       "secondary",
			Action: sdk.SuggestionAction{
				Type:  "search",
				Query: "classic acclaimed masterpiece",
			},
		},
	}
}

// getHolidayIcon returns an appropriate icon for a holiday.
func getHolidayIcon(holidayName string) string {
	name := strings.ToLower(holidayName)
	switch {
	case strings.Contains(name, "christmas"):
		return "gift"
	case strings.Contains(name, "halloween"):
		return "ghost"
	case strings.Contains(name, "valentine"):
		return "heart"
	case strings.Contains(name, "thanksgiving"):
		return "home"
	case strings.Contains(name, "easter"):
		return "egg"
	case strings.Contains(name, "independence") || strings.Contains(name, "july"):
		return "flag"
	case strings.Contains(name, "new year"):
		return "party-popper"
	default:
		return "calendar"
	}
}

// min returns the smaller of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// handleSuggestions handles HTTP requests for search suggestions.
func (p *SemanticSearchPlugin) handleSuggestions(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
	if req.Method != "GET" {
		return jsonResponse(http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}

	// Parse limit from query params
	limit := 6
	if limitStr := req.Query["limit"]; limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 20 {
			limit = l
		}
	}

	// Parse context from query params
	context := req.Query["context"]
	if context == "" {
		context = "homepage"
	}

	resp, err := p.GetSuggestions(ctx, &sdk.SuggestionRequest{
		UserID:  req.UserID,
		Context: context,
		Limit:   limit,
	})
	if err != nil {
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return jsonResponse(http.StatusOK, resp)
}

// handleSearchInfo handles HTTP requests for search provider info.
func (p *SemanticSearchPlugin) handleSearchInfo(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
	if req.Method != "GET" {
		return jsonResponse(http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}

	info, err := p.GetSearchProviderInfo(ctx)
	if err != nil {
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return jsonResponse(http.StatusOK, info)
}
