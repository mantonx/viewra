package internal

import (
	"testing"
)

// TestDetectQueryIntent tests intent detection from natural language queries.
func TestDetectQueryIntent(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected queryIntent
	}{
		// Director search patterns
		{
			name:  "directed by pattern",
			query: "movies directed by Christopher Nolan",
			expected: queryIntent{
				isDirectorSearch: true,
				directorName:     "christopher nolan",
			},
		},
		{
			name:  "director pattern",
			query: "director Quentin Tarantino films",
			expected: queryIntent{
				isDirectorSearch: true,
				directorName:     "quentin tarantino",
			},
		},
		{
			name:  "films by pattern",
			query: "films by Denis Villeneuve",
			expected: queryIntent{
				isDirectorSearch: true,
				directorName:     "denis villeneuve",
			},
		},

		// Actor search patterns
		{
			name:  "starring pattern",
			query: "movies starring Tom Hanks",
			expected: queryIntent{
				isActorSearch: true,
				actorName:     "tom hanks",
			},
		},
		{
			name:  "with actor pattern",
			query: "thrillers with Leonardo DiCaprio",
			expected: queryIntent{
				isActorSearch: true,
				actorName:     "leonardo dicaprio",
			},
		},
		{
			name:  "featuring pattern",
			query: "comedies featuring Jim Carrey",
			expected: queryIntent{
				isActorSearch: true,
				actorName:     "jim carrey",
			},
		},

		// Writer search patterns
		{
			name:  "written by pattern",
			query: "movies written by Aaron Sorkin",
			expected: queryIntent{
				isWriterSearch: true,
				writerName:     "aaron sorkin",
			},
		},
		{
			name:  "screenplay by pattern",
			query: "screenplay by Charlie Kaufman",
			expected: queryIntent{
				isWriterSearch: true,
				writerName:     "charlie kaufman",
			},
		},

		// Producer search patterns
		{
			name:  "produced by pattern",
			query: "produced by Jerry Bruckheimer",
			expected: queryIntent{
				isProducerSearch: true,
				producerName:     "jerry bruckheimer",
			},
		},

		// Studio search patterns
		{
			name:  "by pixar pattern",
			query: "animated movies by Pixar",
			expected: queryIntent{
				isStudioSearch: true,
				studioName:     "pixar",
			},
		},
		{
			name:  "by a24 pattern",
			query: "horror by A24",
			expected: queryIntent{
				isStudioSearch: true,
				studioName:     "a24",
			},
		},
		{
			name:  "by ghibli pattern",
			query: "anime by Ghibli",
			expected: queryIntent{
				isStudioSearch: true,
				studioName:     "ghibli",
			},
		},

		// Person name + movies pattern (generic)
		{
			name:  "name movies pattern - spielberg",
			query: "Spielberg movies",
			expected: queryIntent{
				isPersonSearch: true,
				personName:     "spielberg",
			},
		},
		{
			name:  "name movies pattern - wes anderson",
			query: "Wes Anderson films",
			expected: queryIntent{
				isPersonSearch: true,
				personName:     "wes anderson",
			},
		},
		{
			name:  "name movies pattern - tarantino",
			query: "Tarantino films",
			expected: queryIntent{
				isPersonSearch: true,
				personName:     "tarantino",
			},
		},
		{
			name:  "name movies pattern - meryl streep",
			query: "Meryl Streep movies",
			expected: queryIntent{
				isPersonSearch: true,
				personName:     "meryl streep",
			},
		},

		// Should NOT match as person search
		{
			name:  "genre search should not be person",
			query: "horror movies",
			expected: queryIntent{
				isPersonSearch: false,
			},
		},
		{
			name:  "nationality should not be person",
			query: "Korean movies",
			expected: queryIntent{
				isPersonSearch: false,
			},
		},
		{
			name:  "adjective should not be person",
			query: "good movies",
			expected: queryIntent{
				isPersonSearch: false,
			},
		},
		{
			name:  "action movies should not be person",
			query: "action movies",
			expected: queryIntent{
				isPersonSearch: false,
			},
		},

		// Location patterns
		{
			name:  "set in pattern",
			query: "movies set in Tokyo",
			expected: queryIntent{
				isLocationSearch: true,
			},
		},
		{
			name:  "filmed in pattern",
			query: "filmed in New York",
			expected: queryIntent{
				isLocationSearch: true,
			},
		},

		// Language/nationality patterns
		{
			name:  "French films",
			query: "French films",
			expected: queryIntent{
				isLanguageSearch: true,
				languageName:     "french",
			},
		},
		{
			name:  "Korean thriller",
			query: "Korean thriller movies",
			expected: queryIntent{
				isLanguageSearch: true,
				languageName:     "korean",
			},
		},
		{
			name:  "Japanese anime",
			query: "Japanese anime",
			expected: queryIntent{
				isLanguageSearch: true,
				languageName:     "japanese",
			},
		},
		{
			name:  "K-drama",
			query: "K-drama recommendations",
			expected: queryIntent{
				isLanguageSearch: true,
				languageName:     "korean",
			},
		},
		{
			name:  "Bollywood",
			query: "Bollywood musical",
			expected: queryIntent{
				isLanguageSearch: true,
				languageName:     "hindi",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intent := detectQueryIntent(tt.query)

			if intent.isDirectorSearch != tt.expected.isDirectorSearch {
				t.Errorf("isDirectorSearch: got %v, want %v", intent.isDirectorSearch, tt.expected.isDirectorSearch)
			}
			if intent.directorName != tt.expected.directorName && tt.expected.isDirectorSearch {
				t.Errorf("directorName: got %q, want %q", intent.directorName, tt.expected.directorName)
			}

			if intent.isActorSearch != tt.expected.isActorSearch {
				t.Errorf("isActorSearch: got %v, want %v", intent.isActorSearch, tt.expected.isActorSearch)
			}
			if intent.actorName != tt.expected.actorName && tt.expected.isActorSearch {
				t.Errorf("actorName: got %q, want %q", intent.actorName, tt.expected.actorName)
			}

			if intent.isWriterSearch != tt.expected.isWriterSearch {
				t.Errorf("isWriterSearch: got %v, want %v", intent.isWriterSearch, tt.expected.isWriterSearch)
			}
			if intent.writerName != tt.expected.writerName && tt.expected.isWriterSearch {
				t.Errorf("writerName: got %q, want %q", intent.writerName, tt.expected.writerName)
			}

			if intent.isProducerSearch != tt.expected.isProducerSearch {
				t.Errorf("isProducerSearch: got %v, want %v", intent.isProducerSearch, tt.expected.isProducerSearch)
			}
			if intent.producerName != tt.expected.producerName && tt.expected.isProducerSearch {
				t.Errorf("producerName: got %q, want %q", intent.producerName, tt.expected.producerName)
			}

			if intent.isStudioSearch != tt.expected.isStudioSearch {
				t.Errorf("isStudioSearch: got %v, want %v", intent.isStudioSearch, tt.expected.isStudioSearch)
			}
			if intent.studioName != tt.expected.studioName && tt.expected.isStudioSearch {
				t.Errorf("studioName: got %q, want %q", intent.studioName, tt.expected.studioName)
			}

			if intent.isPersonSearch != tt.expected.isPersonSearch {
				t.Errorf("isPersonSearch: got %v, want %v", intent.isPersonSearch, tt.expected.isPersonSearch)
			}
			if intent.personName != tt.expected.personName && tt.expected.isPersonSearch {
				t.Errorf("personName: got %q, want %q", intent.personName, tt.expected.personName)
			}

			if intent.isLocationSearch != tt.expected.isLocationSearch {
				t.Errorf("isLocationSearch: got %v, want %v", intent.isLocationSearch, tt.expected.isLocationSearch)
			}
		})
	}
}

// TestExtractSearchTerms tests extraction of meaningful search terms.
func TestExtractSearchTerms(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected []string
	}{
		{
			name:     "simple genre query",
			query:    "action movies",
			expected: []string{"action"},
		},
		{
			name:     "complex query",
			query:    "dark psychological thriller",
			expected: []string{"dark", "psychological", "thriller"},
		},
		{
			name:     "query with stopwords",
			query:    "a movie with explosions and car chases",
			expected: []string{"explosions", "car", "chases"},
		},
		{
			name:     "query with filler words",
			query:    "I want to watch something scary",
			expected: []string{"scary"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			terms := extractSearchTerms(tt.query)

			if len(terms) != len(tt.expected) {
				t.Errorf("got %d terms, want %d. Got: %v", len(terms), len(tt.expected), terms)
				return
			}

			for i, term := range terms {
				if term != tt.expected[i] {
					t.Errorf("term[%d]: got %q, want %q", i, term, tt.expected[i])
				}
			}
		})
	}
}

// TestExtractGenresFromQuery tests genre extraction from queries.
func TestExtractGenresFromQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected []string
	}{
		{
			name:     "horror query",
			query:    "scary horror movies",
			expected: []string{"horror"},
		},
		{
			name:     "sci-fi query",
			query:    "sci-fi action film",
			expected: []string{"science fiction", "action"},
		},
		{
			name:     "romantic comedy",
			query:    "romantic comedy with happy ending",
			expected: []string{"romance", "comedy"},
		},
		{
			name:     "animated fantasy",
			query:    "animated fantasy adventure",
			expected: []string{"animation", "fantasy", "adventure"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			genres := extractGenresFromQuery(tt.query)

			// Check that all expected genres are present
			for _, expected := range tt.expected {
				found := false
				for _, got := range genres {
					if got == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected genre %q not found in %v", expected, genres)
				}
			}
		})
	}
}

// TestExtractNegativeTerms tests extraction of negative terms.
func TestExtractNegativeTerms(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected []string
	}{
		{
			name:     "no horror",
			query:    "comedy movies no horror",
			expected: []string{"horror"},
		},
		{
			name:     "without violence",
			query:    "family movies without violence",
			expected: []string{"violence"},
		},
		{
			name:     "non-scary",
			query:    "non-scary thrillers",
			expected: []string{"scary"},
		},
		{
			name:     "not animated",
			query:    "not animated films",
			expected: []string{"animated"},
		},
		{
			name:     "avoid action",
			query:    "avoid action movies",
			expected: []string{"action"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			negatives := extractNegativeTerms(tt.query)

			for _, expected := range tt.expected {
				found := false
				for _, got := range negatives {
					if got == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected negative term %q not found in %v", expected, negatives)
				}
			}
		})
	}
}

// TestExtractNegativeGenres tests negative genre extraction including mood-implied negatives.
func TestExtractNegativeGenres(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected []string
	}{
		{
			name:     "cozy implies no horror/thriller",
			query:    "cozy movies for a rainy day",
			expected: []string{"horror", "thriller"},
		},
		{
			name:     "feel-good implies no horror/thriller/drama",
			query:    "feel-good comedy",
			expected: []string{"horror", "thriller", "drama"},
		},
		{
			name:     "heartwarming implies no horror/thriller",
			query:    "heartwarming family story",
			expected: []string{"horror", "thriller"},
		},
		{
			name:     "kids movies implies no horror",
			query:    "movies for kids",
			expected: []string{"horror"},
		},
		{
			name:     "explicit no horror",
			query:    "thriller no horror",
			expected: []string{"horror"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			negatives := extractNegativeGenres(tt.query)

			for _, expected := range tt.expected {
				found := false
				for _, got := range negatives {
					if got == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected negative genre %q not found in %v", expected, negatives)
				}
			}
		})
	}
}

// TestExtractLine tests line extraction from embedding text.
func TestExtractLine(t *testing.T) {
	text := `Title: The Dark Knight (2008)
Genre: Action, Crime, Drama
Directed by: Christopher Nolan
Cast: Christian Bale, Heath Ledger
Studios: Warner Bros., Legendary Pictures`

	tests := []struct {
		prefix   string
		expected string
	}{
		{
			prefix:   "title:",
			expected: "title: the dark knight (2008)",
		},
		{
			prefix:   "genre:",
			expected: "genre: action, crime, drama",
		},
		{
			prefix:   "directed by:",
			expected: "directed by: christopher nolan",
		},
		{
			prefix:   "cast:",
			expected: "cast: christian bale, heath ledger",
		},
		{
			prefix:   "studios:",
			expected: "studios: warner bros., legendary pictures",
		},
		{
			prefix:   "not_found:",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.prefix, func(t *testing.T) {
			result := extractLine(text, tt.prefix)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestExtractDecade tests decade extraction from embedding text.
func TestExtractDecade(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected string
	}{
		{
			name:     "1980s movie",
			text:     "Title: Die Hard (1988)\nGenre: Action",
			expected: "1980s",
		},
		{
			name:     "1990s movie",
			text:     "Title: Pulp Fiction (1994)\nGenre: Crime",
			expected: "1990s",
		},
		{
			name:     "2000s movie",
			text:     "Title: The Dark Knight (2008)\nGenre: Action",
			expected: "2000s",
		},
		{
			name:     "2010s movie",
			text:     "Title: Inception (2010)\nGenre: Sci-Fi",
			expected: "2010s",
		},
		{
			name:     "no year",
			text:     "Title: Unknown Movie\nGenre: Drama",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDecade(tt.text)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestExtractTitleKey tests title key extraction for deduplication.
func TestExtractTitleKey(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected string
	}{
		{
			name:     "movie with year",
			text:     "Title: The Shawshank Redemption (1994)\nGenre: Drama",
			expected: "the shawshank redemption (1994)",
		},
		{
			name:     "movie without year",
			text:     "Title: Some Movie\nGenre: Comedy",
			expected: "some movie",
		},
		{
			name:     "no title prefix",
			text:     "The Movie Title\nGenre: Action",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTitleKey(tt.text)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestNeedsRewriting tests query rewriting detection.
func TestNeedsRewriting(t *testing.T) {
	tests := []struct {
		query    string
		expected bool
	}{
		// Should need rewriting
		{"I'm feeling sad and need cheering up", true},
		{"feeling stressed out", true},
		{"need something to relax", true},
		{"I want to watch something scary", true},
		{"in the mood for comedy", true},
		{"something like Die Hard", true},
		{"movies similar to Inception", true},

		// Should NOT need rewriting (direct searches)
		{"Christopher Nolan", false},
		{"action movies", false},
		{"The Dark Knight", false},
		{"sci-fi 2020", false},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := needsRewriting(tt.query)
			if result != tt.expected {
				t.Errorf("needsRewriting(%q): got %v, want %v", tt.query, result, tt.expected)
			}
		})
	}
}

// TestExtractPersonFromNameMoviesPattern tests the "Name movies" pattern detection.
func TestExtractPersonFromNameMoviesPattern(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		wantName  string
		wantMatch bool
	}{
		// Valid person names
		{
			name:      "single capitalized name",
			query:     "Spielberg movies",
			wantName:  "spielberg",
			wantMatch: true,
		},
		{
			name:      "full name",
			query:     "Wes Anderson films",
			wantName:  "wes anderson",
			wantMatch: true,
		},
		{
			name:      "actor name",
			query:     "Tom Hanks movies",
			wantName:  "tom hanks",
			wantMatch: true,
		},
		{
			name:      "three word name",
			query:     "George R Martin movies",
			wantName:  "george r martin",
			wantMatch: true,
		},

		// Should NOT match
		{
			name:      "genre word",
			query:     "horror movies",
			wantName:  "",
			wantMatch: false,
		},
		{
			name:      "nationality",
			query:     "Korean movies",
			wantName:  "",
			wantMatch: false,
		},
		{
			name:      "adjective - good",
			query:     "good movies",
			wantName:  "",
			wantMatch: false,
		},
		{
			name:      "adjective - best",
			query:     "best movies",
			wantName:  "",
			wantMatch: false,
		},
		{
			name:      "adjective - top",
			query:     "top movies",
			wantName:  "",
			wantMatch: false,
		},
		{
			name:      "lowercase only",
			query:     "spielberg movies",
			wantName:  "",
			wantMatch: false, // No capital letter
		},
		{
			name:      "similar keyword",
			query:     "similar movies",
			wantName:  "",
			wantMatch: false,
		},
		{
			name:      "popular keyword",
			query:     "popular movies",
			wantName:  "",
			wantMatch: false,
		},
		{
			name:      "too short",
			query:     "AI movies",
			wantName:  "",
			wantMatch: false, // "ai" is only 2 chars
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, match := extractPersonFromNameMoviesPattern(tt.query)
			if match != tt.wantMatch {
				t.Errorf("match: got %v, want %v", match, tt.wantMatch)
			}
			if name != tt.wantName {
				t.Errorf("name: got %q, want %q", name, tt.wantName)
			}
		})
	}
}

// BenchmarkDetectQueryIntent benchmarks the intent detection.
func BenchmarkDetectQueryIntent(b *testing.B) {
	queries := []string{
		"Christopher Nolan movies",
		"movies directed by Quentin Tarantino",
		"action movies starring Tom Hanks",
		"cozy romantic comedy for date night",
		"horror thriller set in Tokyo",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, q := range queries {
			detectQueryIntent(q)
		}
	}
}

// BenchmarkExtractSearchTerms benchmarks search term extraction.
func BenchmarkExtractSearchTerms(b *testing.B) {
	queries := []string{
		"dark psychological thriller with twist ending",
		"family friendly animated adventure comedy",
		"90s action movies with explosions",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, q := range queries {
			extractSearchTerms(q)
		}
	}
}
