package internal

import (
	"strings"
	"testing"
	"time"
)

// TestTimeOfDay tests the time of day detection.
func TestTimeOfDay(t *testing.T) {
	tests := []struct {
		hour     int
		expected string
	}{
		{5, "morning"},
		{8, "morning"},
		{11, "morning"},
		{12, "afternoon"},
		{14, "afternoon"},
		{16, "afternoon"},
		{17, "evening"},
		{19, "evening"},
		{20, "evening"},
		{21, "night"},
		{23, "night"},
		{0, "night"},
		{3, "night"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			testTime := time.Date(2024, 6, 15, tt.hour, 30, 0, 0, time.UTC)
			result := getTimeOfDay(testTime)
			if result != tt.expected {
				t.Errorf("hour %d: got %q, want %q", tt.hour, result, tt.expected)
			}
		})
	}
}

// TestSeasonDetection tests season detection based on month and hemisphere.
func TestSeasonDetection(t *testing.T) {
	tests := []struct {
		name     string
		month    time.Month
		latitude float64
		expected string
	}{
		// Northern hemisphere
		{"march north", time.March, 40.0, "spring"},
		{"june north", time.June, 40.0, "summer"},
		{"september north", time.September, 40.0, "fall"},
		{"december north", time.December, 40.0, "winter"},

		// Southern hemisphere (reversed)
		{"march south", time.March, -40.0, "fall"},
		{"june south", time.June, -40.0, "winter"},
		{"september south", time.September, -40.0, "spring"},
		{"december south", time.December, -40.0, "summer"},

		// Equator (uses northern convention)
		{"june equator", time.June, 0.0, "summer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testTime := time.Date(2024, tt.month, 15, 12, 0, 0, 0, time.UTC)
			result := getSeason(testTime, tt.latitude)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestIsSpecificQuery tests detection of specific vs vague queries.
func TestIsSpecificQuery(t *testing.T) {
	// Specific queries - should NOT be enriched with context
	specificQueries := []string{
		// Director/creator
		"directed by Christopher Nolan",
		"Spielberg movies",
		"Tarantino films",

		// Actor
		"starring Tom Hanks",
		"movies with Leonardo DiCaprio",

		// Genre
		"action movies",
		"horror films",
		"romantic comedy",

		// Location
		"set in Tokyo",
		"filmed in New York",

		// Reference
		"movies like Inception",
		"similar to The Matrix",

		// Nationality/language
		"French films",
		"Korean thriller",
		"Japanese anime",

		// Era/decade
		"80s action movies",
		"1990s comedy",
		"classic films",

		// Studio
		"Pixar movies",
		"A24 horror",
		"Marvel films",

		// Specific subject
		"zombie movies",
		"heist films",
		"spy thriller",
		"superhero movies",
	}

	for _, query := range specificQueries {
		t.Run("specific: "+query, func(t *testing.T) {
			if !isSpecificQuery(query) {
				t.Errorf("expected query to be specific: %q", query)
			}
		})
	}

	// Vague queries - SHOULD be enriched with context
	vagueQueries := []string{
		"something to watch tonight",
		"what should I watch",
		"recommend me a movie",
		"I'm feeling sad",
		"in the mood for something fun",
		"good movie for date night",
		"something relaxing",
		"I feel bored",
	}

	for _, query := range vagueQueries {
		t.Run("vague: "+query, func(t *testing.T) {
			if isSpecificQuery(query) {
				t.Errorf("expected query to be vague: %q", query)
			}
		})
	}
}

// TestWeatherEnrichments tests weather-based query enrichments.
func TestWeatherEnrichments(t *testing.T) {
	enricher := NewContextEnricher(nil, nil)

	tests := []struct {
		name       string
		weather    *WeatherContext
		shouldHave []string
	}{
		{
			name: "rainy cold",
			weather: &WeatherContext{
				Available:   true,
				Condition:   "rainy",
				Temperature: 10,
			},
			shouldHave: []string{"cozy", "indoor"},
		},
		{
			name: "snowy",
			weather: &WeatherContext{
				Available: true,
				Condition: "snowy",
			},
			shouldHave: []string{"winter", "cozy"},
		},
		{
			name: "stormy",
			weather: &WeatherContext{
				Available: true,
				Condition: "stormy",
			},
			shouldHave: []string{"intense", "dramatic"},
		},
		{
			name: "foggy",
			weather: &WeatherContext{
				Available: true,
				Condition: "foggy",
			},
			shouldHave: []string{"mysterious", "atmospheric"},
		},
		{
			name: "sunny day",
			weather: &WeatherContext{
				Available:   true,
				Condition:   "sunny",
				IsDay:       true,
				Temperature: 22,
			},
			shouldHave: []string{"uplifting", "bright"},
		},
		{
			name: "hot sunny day",
			weather: &WeatherContext{
				Available:   true,
				Condition:   "sunny",
				IsDay:       true,
				Temperature: 32,
			},
			shouldHave: []string{"summer", "adventure"},
		},
		{
			name: "clear night",
			weather: &WeatherContext{
				Available: true,
				Condition: "clear",
				IsDay:     false,
			},
			shouldHave: []string{"romantic evening"},
		},
		{
			name: "freezing",
			weather: &WeatherContext{
				Available:   true,
				Condition:   "cloudy",
				Temperature: -5,
			},
			shouldHave: []string{"warming", "fireside"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enrichments := enricher.getWeatherEnrichments(tt.weather)
			enrichmentStr := strings.Join(enrichments, " ")

			for _, expected := range tt.shouldHave {
				if !strings.Contains(enrichmentStr, expected) {
					t.Errorf("expected enrichment %q not found in %v", expected, enrichments)
				}
			}
		})
	}
}

// TestTimeEnrichments tests time-based query enrichments.
func TestTimeEnrichments(t *testing.T) {
	enricher := NewContextEnricher(nil, nil)

	tests := []struct {
		name       string
		context    *QueryContext
		shouldHave []string
	}{
		{
			name: "morning",
			context: &QueryContext{
				TimeOfDay: "morning",
				DayOfWeek: "monday",
			},
			shouldHave: []string{"light", "uplifting"},
		},
		{
			name: "afternoon",
			context: &QueryContext{
				TimeOfDay: "afternoon",
				DayOfWeek: "wednesday",
			},
			shouldHave: []string{"adventure", "action"},
		},
		{
			name: "evening",
			context: &QueryContext{
				TimeOfDay: "evening",
				DayOfWeek: "tuesday",
			},
			shouldHave: []string{"relaxing", "entertainment"},
		},
		{
			name: "friday night",
			context: &QueryContext{
				TimeOfDay: "night",
				DayOfWeek: "friday",
			},
			shouldHave: []string{"atmospheric", "party", "exciting"},
		},
		{
			name: "saturday night",
			context: &QueryContext{
				TimeOfDay: "night",
				DayOfWeek: "saturday",
			},
			shouldHave: []string{"binge-worthy", "marathon"},
		},
		{
			name: "sunday afternoon",
			context: &QueryContext{
				TimeOfDay: "afternoon",
				DayOfWeek: "sunday",
			},
			shouldHave: []string{"binge-worthy"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enrichments := enricher.getTimeEnrichments(tt.context)
			enrichmentStr := strings.Join(enrichments, " ")

			for _, expected := range tt.shouldHave {
				if !strings.Contains(enrichmentStr, expected) {
					t.Errorf("expected enrichment %q not found in %v", expected, enrichments)
				}
			}
		})
	}
}

// TestQueryEnrichment tests the full query enrichment flow.
func TestQueryEnrichment(t *testing.T) {
	enricher := NewContextEnricher(nil, nil)

	tests := []struct {
		name          string
		query         string
		context       *QueryContext
		shouldContain string
		shouldSkip    bool // If true, query should NOT be enriched
	}{
		{
			name:  "vague query gets enriched",
			query: "something to watch",
			context: &QueryContext{
				TimeOfDay: "night",
				Weather: &WeatherContext{
					Available: true,
					Condition: "rainy",
				},
			},
			shouldContain: "Context:",
		},
		{
			name:       "specific query not enriched",
			query:      "Christopher Nolan movies",
			context:    &QueryContext{TimeOfDay: "night"},
			shouldSkip: true,
		},
		{
			name:       "genre query not enriched",
			query:      "action movies",
			context:    &QueryContext{TimeOfDay: "night"},
			shouldSkip: true,
		},
		{
			name:       "nil context returns original",
			query:      "something fun",
			context:    nil,
			shouldSkip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := enricher.EnrichQuery(tt.query, tt.context)

			if tt.shouldSkip {
				if result != tt.query {
					t.Errorf("expected query to be unchanged, got %q", result)
				}
			} else {
				if !strings.Contains(result, tt.shouldContain) {
					t.Errorf("expected result to contain %q, got %q", tt.shouldContain, result)
				}
			}
		})
	}
}

// TestWeatherSuggestions tests weather-based genre/mood suggestions.
func TestWeatherSuggestions(t *testing.T) {
	enricher := NewContextEnricher(nil, nil)

	tests := []struct {
		name           string
		weather        *WeatherContext
		expectedGenres []string
		expectedMoods  []string
	}{
		{
			name:           "rainy",
			weather:        &WeatherContext{Available: true, Condition: "rainy"},
			expectedGenres: []string{"drama", "romance", "mystery"},
			expectedMoods:  []string{"cozy", "introspective"},
		},
		{
			name:           "snowy",
			weather:        &WeatherContext{Available: true, Condition: "snowy"},
			expectedGenres: []string{"family", "holiday"},
			expectedMoods:  []string{"cozy", "magical", "festive"},
		},
		{
			name:           "stormy",
			weather:        &WeatherContext{Available: true, Condition: "stormy"},
			expectedGenres: []string{"thriller", "horror"},
			expectedMoods:  []string{"intense", "suspenseful"},
		},
		{
			name:           "foggy",
			weather:        &WeatherContext{Available: true, Condition: "foggy"},
			expectedGenres: []string{"mystery", "thriller", "noir"},
			expectedMoods:  []string{"atmospheric", "mysterious"},
		},
		{
			name:           "sunny day",
			weather:        &WeatherContext{Available: true, Condition: "sunny", IsDay: true},
			expectedGenres: []string{"comedy", "adventure", "action"},
			expectedMoods:  []string{"uplifting", "fun"},
		},
		{
			name:           "clear night",
			weather:        &WeatherContext{Available: true, Condition: "clear", IsDay: false},
			expectedGenres: []string{"romance", "drama", "sci-fi"},
			expectedMoods:  []string{"romantic", "contemplative"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := enricher.getWeatherSuggestions(tt.weather)

			for _, genre := range tt.expectedGenres {
				found := false
				for _, g := range suggestions.Genres {
					if g == genre {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected genre %q not found in %v", genre, suggestions.Genres)
				}
			}

			for _, mood := range tt.expectedMoods {
				found := false
				for _, m := range suggestions.Moods {
					if m == mood {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected mood %q not found in %v", mood, suggestions.Moods)
				}
			}
		})
	}
}

// TestTimeSuggestions tests time-based genre/mood suggestions.
func TestTimeSuggestions(t *testing.T) {
	enricher := NewContextEnricher(nil, nil)

	tests := []struct {
		timeOfDay      string
		expectedGenres []string
		expectedMoods  []string
	}{
		{
			timeOfDay:      "morning",
			expectedGenres: []string{"documentary", "comedy", "animation"},
			expectedMoods:  []string{"light", "educational", "uplifting"},
		},
		{
			timeOfDay:      "afternoon",
			expectedGenres: []string{"action", "adventure", "family"},
			expectedMoods:  []string{"exciting", "fun"},
		},
		{
			timeOfDay:      "evening",
			expectedGenres: []string{"drama", "comedy", "romance"},
			expectedMoods:  []string{"relaxing", "engaging", "heartwarming"},
		},
		{
			timeOfDay:      "night",
			expectedGenres: []string{"thriller", "horror", "sci-fi", "noir"},
			expectedMoods:  []string{"atmospheric", "intense", "immersive"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.timeOfDay, func(t *testing.T) {
			suggestions := enricher.getTimeSuggestions(tt.timeOfDay)

			for _, genre := range tt.expectedGenres {
				found := false
				for _, g := range suggestions.Genres {
					if g == genre {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected genre %q not found in %v", genre, suggestions.Genres)
				}
			}

			for _, mood := range tt.expectedMoods {
				found := false
				for _, m := range suggestions.Moods {
					if m == mood {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected mood %q not found in %v", mood, suggestions.Moods)
				}
			}
		})
	}
}

// TestSeasonalSuggestions tests season-based content suggestions.
func TestSeasonalSuggestions(t *testing.T) {
	tests := []struct {
		season         string
		expectedGenres []string
		expectedMoods  []string
	}{
		{
			season:         "spring",
			expectedGenres: []string{"romance", "comedy", "adventure"},
			expectedMoods:  []string{"uplifting", "fresh", "hopeful"},
		},
		{
			season:         "summer",
			expectedGenres: []string{"action", "adventure", "comedy", "blockbuster"},
			expectedMoods:  []string{"exciting", "fun", "escapist"},
		},
		{
			season:         "fall",
			expectedGenres: []string{"drama", "thriller", "horror", "mystery"},
			expectedMoods:  []string{"atmospheric", "cozy", "mysterious"},
		},
		{
			season:         "winter",
			expectedGenres: []string{"family", "drama", "romance", "fantasy"},
			expectedMoods:  []string{"cozy", "heartwarming", "magical"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.season, func(t *testing.T) {
			suggestions := getSeasonalSuggestions(tt.season)

			for _, genre := range tt.expectedGenres {
				found := false
				for _, g := range suggestions.Genres {
					if g == genre {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected genre %q not found in %v", genre, suggestions.Genres)
				}
			}

			for _, mood := range tt.expectedMoods {
				found := false
				for _, m := range suggestions.Moods {
					if m == mood {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected mood %q not found in %v", mood, suggestions.Moods)
				}
			}
		})
	}
}

// TestNamePatternDetection tests detection of "Name movies" patterns.
func TestNamePatternDetection(t *testing.T) {
	tests := []struct {
		query    string
		expected bool
	}{
		{"Spielberg movies", true},
		{"Tom Hanks films", true},
		{"Wes Anderson movies", true},
		{"DiCaprio films", true},
		{"McQueen movies", true},
		{"O'Brien films", true},

		{"action movies", false},
		{"good movies", false},
		{"horror films", false},
		{"something fun", false},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := hasNamePlusMediaPattern(tt.query)
			if result != tt.expected {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestCamelCaseNameDetection tests detection of CamelCase names.
func TestCamelCaseNameDetection(t *testing.T) {
	tests := []struct {
		query    string
		expected bool
	}{
		{"DiCaprio movies", true},
		{"McQueen films", true},
		{"LaBeouf movies", true},
		{"DeMille films", true},

		{"spielberg movies", false},
		{"action films", false},
		{"The Matrix", false},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := hasCamelCaseName(tt.query)
			if result != tt.expected {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestSurnameDetection tests detection of surname patterns.
func TestSurnameDetection(t *testing.T) {
	tests := []struct {
		word     string
		expected bool
	}{
		// Germanic
		{"Spielberg", true},
		{"Goldstein", true},
		{"Hoffman", true},

		// Italian
		{"Scorsese", false}, // Doesn't match suffix patterns
		{"Tarantino", true},
		{"Fellini", true},

		// Spanish
		{"Rodriguez", true},
		{"Fernandez", true},

		// Slavic
		{"Kubrick", false}, // -ick not in patterns
		{"Polanski", true},

		// Irish/Scottish
		{"McCarthy", true},
		{"MacDonald", true},
		{"O'Brien", true},

		// Common words that aren't surnames
		{"Action", false},
		{"Comedy", false},
		{"Movie", false},
	}

	for _, tt := range tests {
		t.Run(tt.word, func(t *testing.T) {
			result := looksLikeSurname(tt.word)
			if result != tt.expected {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestDeduplicate tests the string deduplication helper.
func TestDeduplicate(t *testing.T) {
	tests := []struct {
		input    []string
		expected int // expected length after dedup
	}{
		{[]string{"a", "b", "c"}, 3},
		{[]string{"a", "a", "b"}, 2},
		{[]string{"A", "a", "B"}, 2}, // case-insensitive
		{[]string{}, 0},
		{[]string{"cozy", "Cozy", "COZY"}, 1},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := deduplicate(tt.input)
			if len(result) != tt.expected {
				t.Errorf("got %d items, want %d", len(result), tt.expected)
			}
		})
	}
}
