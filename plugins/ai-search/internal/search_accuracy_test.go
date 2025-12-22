package internal

import (
	"strings"
	"testing"
)

// SearchAccuracyTestCase defines a test case for search accuracy.
type SearchAccuracyTestCase struct {
	Name        string
	Query       string
	EntityTypes []EntityType
	// ExpectInTop5 lists substrings that should appear in the top 5 results' text
	ExpectInTop5 []string
	// ExpectNotInTop5 lists substrings that should NOT appear in the top 5 results
	ExpectNotInTop5 []string
	// MinSimilarity is the minimum similarity score expected for top result
	MinSimilarity float32
}

// mockSearchResult simulates a search result for testing keyword boost logic.
type mockSearchResult struct {
	text       string
	similarity float32
}

// TestIntentDetectionAccuracy tests that query intents are correctly detected.
//
// Known limitations:
//   - Person searches match on surname only, so "Spielberg films" may match movies
//     with other Spielberg family members (e.g., Anne Spielberg wrote "Big")
//   - Director matches are boosted higher than cast/producer matches to mitigate this
func TestIntentDetectionAccuracy(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		wantType string // "director", "actor", "writer", "producer", "studio", "person", "language", "location", "none"
		wantName string
	}{
		// Director patterns
		{"director - directed by", "movies directed by Christopher Nolan", "director", "christopher nolan"},
		{"director - director prefix", "director Quentin Tarantino films", "director", "quentin tarantino"},
		{"director - films by", "films by Denis Villeneuve", "director", "denis villeneuve"},
		{"director - movies by person", "movies by Martin Scorsese", "director", "martin scorsese"},

		// Actor patterns
		{"actor - starring", "movies starring Tom Hanks", "actor", "tom hanks"},
		{"actor - with", "thrillers with Leonardo DiCaprio", "actor", "leonardo dicaprio"},
		{"actor - featuring", "comedies featuring Jim Carrey", "actor", "jim carrey"},

		// Writer patterns
		{"writer - written by", "movies written by Aaron Sorkin", "writer", "aaron sorkin"},
		{"writer - screenplay by", "screenplay by Charlie Kaufman", "writer", "charlie kaufman"},

		// Producer patterns
		{"producer - produced by", "produced by Jerry Bruckheimer", "producer", "jerry bruckheimer"},

		// Studio patterns
		{"studio - by pixar", "animated movies by Pixar", "studio", "pixar"},
		{"studio - by a24", "horror by A24", "studio", "a24"},
		{"studio - by ghibli", "anime by Ghibli", "studio", "ghibli"},
		{"studio - by marvel", "superhero movies by Marvel", "studio", "marvel"},
		{"studio - by disney", "family films by Disney", "studio", "disney"},
		{"studio - movies by netflix", "movies by Netflix", "studio", "netflix"},

		// Person patterns (Name + movies)
		{"person - single name", "Spielberg movies", "person", "spielberg"},
		{"person - full name", "Wes Anderson films", "person", "wes anderson"},
		{"person - actor name", "Meryl Streep movies", "person", "meryl streep"},

		// Language/nationality patterns
		{"language - french", "French films", "language", "french"},
		{"language - korean", "Korean movies", "language", "korean"},
		{"language - japanese", "Japanese cinema", "language", "japanese"},
		{"language - italian", "Italian neorealism", "language", "italian"},
		{"language - german", "German expressionism", "language", "german"},
		{"language - spanish", "Spanish thrillers", "language", "spanish"},
		{"language - k-drama", "K-drama recommendations", "language", "korean"},
		{"language - anime", "best anime movies", "language", "japanese"},
		{"language - bollywood", "Bollywood musical", "language", "hindi"},

		// Location patterns
		{"location - set in", "movies set in Tokyo", "location", ""},
		{"location - filmed in", "filmed in New York", "location", ""},
		{"location - takes place", "takes place in Paris", "location", ""},

		// Should NOT match person pattern
		{"not person - genre", "horror movies", "none", ""},
		{"not person - nationality alone", "Korean movies", "language", "korean"}, // Should be language, not person
		{"not person - adjective", "good movies", "none", ""},
		{"not person - action", "action movies", "none", ""},
		{"not person - best", "best movies", "none", ""},
		{"not person - similar", "similar movies", "none", ""},

		// Complex queries - should detect primary intent
		{"complex - korean thriller", "Korean thriller movies", "language", "korean"},
		{"complex - french romance", "French romantic comedy", "language", "french"},
		{"complex - 80s action", "80s action movies", "none", ""}, // No specific person/language intent
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intent := detectQueryIntent(tt.query)

			var gotType, gotName string
			switch {
			case intent.isDirectorSearch:
				gotType = "director"
				gotName = intent.directorName
			case intent.isActorSearch:
				gotType = "actor"
				gotName = intent.actorName
			case intent.isWriterSearch:
				gotType = "writer"
				gotName = intent.writerName
			case intent.isProducerSearch:
				gotType = "producer"
				gotName = intent.producerName
			case intent.isStudioSearch:
				gotType = "studio"
				gotName = intent.studioName
			case intent.isPersonSearch:
				gotType = "person"
				gotName = intent.personName
			case intent.isLanguageSearch:
				gotType = "language"
				gotName = intent.languageName
			case intent.isLocationSearch:
				gotType = "location"
				gotName = ""
			default:
				gotType = "none"
				gotName = ""
			}

			if gotType != tt.wantType {
				t.Errorf("intent type: got %q, want %q", gotType, tt.wantType)
			}
			if tt.wantName != "" && gotName != tt.wantName {
				t.Errorf("extracted name: got %q, want %q", gotName, tt.wantName)
			}
		})
	}
}

// TestKeywordBoostAccuracy tests that keyword boosting works correctly.
func TestKeywordBoostAccuracy(t *testing.T) {
	service := &SearchService{
		minSimilarity: 0.3,
	}

	tests := []struct {
		name        string
		query       string
		results     []SearchResult
		expectFirst string // Expected title pattern in first result after boost
		expectBoost bool   // Whether we expect a boost to change ordering
	}{
		{
			name:  "director boost - nolan",
			query: "movies directed by Christopher Nolan",
			results: []SearchResult{
				{Similarity: 0.8, Text: "Title: Inception (2010)\nGenre: Sci-Fi\nDirected by: Christopher Nolan\n"},
				{Similarity: 0.85, Text: "Title: The Matrix (1999)\nGenre: Sci-Fi\nDirected by: Lana Wachowski\n"},
			},
			expectFirst: "Inception",
			expectBoost: true,
		},
		{
			name:  "actor boost - hanks",
			query: "movies starring Tom Hanks",
			results: []SearchResult{
				{Similarity: 0.8, Text: "Title: Forrest Gump (1994)\nGenre: Drama\nCast: Tom Hanks, Robin Wright\n"},
				{Similarity: 0.85, Text: "Title: Rain Man (1988)\nGenre: Drama\nCast: Dustin Hoffman, Tom Cruise\n"},
			},
			expectFirst: "Forrest Gump",
			expectBoost: true,
		},
		{
			name:  "studio boost - pixar",
			query: "Pixar movies",
			results: []SearchResult{
				{Similarity: 0.8, Text: "Title: Toy Story (1995)\nGenre: Animation\nStudios: Pixar Animation Studios\n"},
				{Similarity: 0.85, Text: "Title: Shrek (2001)\nGenre: Animation\nStudios: DreamWorks Animation\n"},
			},
			expectFirst: "Toy Story",
			expectBoost: true,
		},
		{
			name:  "language boost - french",
			query: "French films",
			results: []SearchResult{
				{Similarity: 0.8, Text: "Title: Amélie (2001)\nGenre: Comedy\nLanguage: French\nCountry: France\n"},
				{Similarity: 0.85, Text: "Title: The Artist (2011)\nGenre: Comedy\nLanguage: English\nCountry: USA\n"},
			},
			expectFirst: "Amélie",
			expectBoost: true,
		},
		{
			name:  "genre boost",
			query: "horror movies",
			results: []SearchResult{
				{Similarity: 0.8, Text: "Title: The Shining (1980)\nGenre: Horror, Drama\n"},
				{Similarity: 0.82, Text: "Title: The Godfather (1972)\nGenre: Crime, Drama\n"},
			},
			expectFirst: "The Shining",
			expectBoost: true,
		},
		{
			name:  "negative genre penalty - cozy no horror",
			query: "cozy movies",
			results: []SearchResult{
				{Similarity: 0.85, Text: "Title: A Nightmare on Elm Street (1984)\nGenre: Horror\n"},
				{Similarity: 0.8, Text: "Title: You've Got Mail (1998)\nGenre: Comedy, Romance\n"},
			},
			expectFirst: "You've Got Mail",
			expectBoost: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryLower := strings.ToLower(tt.query)
			boosted := service.applyKeywordBoost(tt.results, tt.query, queryLower)

			if len(boosted) == 0 {
				t.Fatal("no results after boost")
			}

			// Sort by similarity (highest first)
			for i := 0; i < len(boosted)-1; i++ {
				for j := i + 1; j < len(boosted); j++ {
					if boosted[j].Similarity > boosted[i].Similarity {
						boosted[i], boosted[j] = boosted[j], boosted[i]
					}
				}
			}

			firstTitle := boosted[0].Text
			if !strings.Contains(firstTitle, tt.expectFirst) {
				t.Errorf("expected first result to contain %q, got %q", tt.expectFirst, firstTitle)
			}
		})
	}
}

// TestNegativeTermExtraction tests extraction of negative/exclusion terms.
func TestNegativeTermExtraction(t *testing.T) {
	tests := []struct {
		query    string
		expected []string
	}{
		{"comedy no horror", []string{"horror"}},
		{"action without romance", []string{"romance"}},
		{"thriller non-violent", []string{"violent"}},
		{"drama not animated", []string{"animated"}},
		{"movies avoid scary", []string{"scary"}},
		{"family films excluding violence", []string{"violence"}},
		{"comedy except horror", []string{"horror"}},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			negatives := extractNegativeTerms(tt.query)

			for _, expected := range tt.expected {
				found := false
				for _, neg := range negatives {
					if neg == expected {
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

// TestMoodImpliedNegatives tests that mood words imply genre exclusions.
func TestMoodImpliedNegatives(t *testing.T) {
	tests := []struct {
		query          string
		expectedGenres []string
	}{
		{"cozy movie night", []string{"horror", "thriller"}},
		{"feel-good comedy", []string{"horror", "thriller", "drama"}},
		{"heartwarming family film", []string{"horror", "thriller"}},
		{"relaxing movies", []string{"horror", "thriller"}},
		{"movies for kids", []string{"horror"}},
		{"uplifting stories", []string{"horror", "thriller"}},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			negatives := extractMoodImpliedNegatives(tt.query)

			for _, expected := range tt.expectedGenres {
				found := false
				for _, neg := range negatives {
					if neg == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected mood-implied negative genre %q not found in %v", expected, negatives)
				}
			}
		})
	}
}

// TestGenreExtraction tests extraction of genre keywords from queries.
func TestGenreExtraction(t *testing.T) {
	tests := []struct {
		query    string
		expected []string
	}{
		{"action movies", []string{"action"}},
		{"romantic comedy", []string{"romance", "comedy"}},
		{"sci-fi thriller", []string{"science fiction", "thriller"}},
		{"animated fantasy adventure", []string{"animation", "fantasy", "adventure"}},
		{"horror mystery", []string{"horror", "mystery"}},
		{"documentary about nature", []string{"documentary"}},
		{"crime drama", []string{"crime", "drama"}},
		{"war film", []string{"war"}},
		{"musical comedy", []string{"music", "comedy"}},
		{"superhero action", []string{"action"}},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			genres := extractGenresFromQuery(tt.query)

			for _, expected := range tt.expected {
				found := false
				for _, g := range genres {
					if g == expected {
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

// TestSearchTermExtraction tests extraction of meaningful search terms.
func TestSearchTermExtraction(t *testing.T) {
	tests := []struct {
		query         string
		shouldHave    []string
		shouldNotHave []string
	}{
		{
			query:         "dark psychological thriller",
			shouldHave:    []string{"dark", "psychological", "thriller"},
			shouldNotHave: []string{},
		},
		{
			query:         "I want to watch something with explosions",
			shouldHave:    []string{"explosions"},
			shouldNotHave: []string{"i", "want", "to", "watch", "something", "with"},
		},
		{
			query:         "movies like Die Hard",
			shouldHave:    []string{"Die", "Hard"},
			shouldNotHave: []string{"movies", "like"},
		},
		{
			query:         "the best action films of the 90s",
			shouldHave:    []string{"action", "90s"},
			shouldNotHave: []string{"the", "of"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			terms := extractSearchTerms(tt.query)

			for _, expected := range tt.shouldHave {
				found := false
				for _, term := range terms {
					if term == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected term %q not found in %v", expected, terms)
				}
			}

			for _, notExpected := range tt.shouldNotHave {
				for _, term := range terms {
					if term == notExpected {
						t.Errorf("term %q should have been filtered out, but found in %v", notExpected, terms)
						break
					}
				}
			}
		})
	}
}

// TestDecadeExtraction tests extraction of decade from embedding text.
func TestDecadeExtraction(t *testing.T) {
	tests := []struct {
		text     string
		expected string
	}{
		{"Title: Die Hard (1988)\nGenre: Action", "1980s"},
		{"Title: Pulp Fiction (1994)\nGenre: Crime", "1990s"},
		{"Title: The Dark Knight (2008)\nGenre: Action", "2000s"},
		{"Title: Inception (2010)\nGenre: Sci-Fi", "2010s"},
		{"Title: Oppenheimer (2023)\nGenre: Drama", "2020s"},
		{"Title: Casablanca (1942)\nGenre: Drama", "1940s"},
		{"Title: The Wizard of Oz (1939)\nGenre: Fantasy", "1930s"},
		{"Title: Unknown Movie\nGenre: Drama", ""},
	}

	for _, tt := range tests {
		t.Run(tt.text[:20], func(t *testing.T) {
			result := extractDecade(tt.text)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestTitleKeyExtraction tests title extraction for deduplication.
func TestTitleKeyExtraction(t *testing.T) {
	tests := []struct {
		text     string
		expected string
	}{
		{"Title: The Shawshank Redemption (1994)\nGenre: Drama", "the shawshank redemption (1994)"},
		{"Title: WALL·E (2008)\nGenre: Animation", "wall·e (2008)"},
		{"Title: Some Movie\nGenre: Comedy", "some movie"},
		{"Genre: Action\nTitle: Wrong Order", ""}, // Title must be first line
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := extractTitleKey(tt.text)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestQueryNeedsRewriting tests detection of queries that need LLM rewriting.
func TestQueryNeedsRewriting(t *testing.T) {
	shouldRewrite := []string{
		"I'm feeling sad and need cheering up",
		"feeling stressed out",
		"need something to relax",
		"I want to watch something scary",
		"in the mood for comedy",
		"something like Die Hard",
		"movies similar to Inception",
		"feeling bored",
		"need to be happy",
	}

	shouldNotRewrite := []string{
		"Christopher Nolan",
		"action movies",
		"The Dark Knight",
		"sci-fi 2020",
		"horror",
		"French films",
		"Spielberg",
	}

	for _, query := range shouldRewrite {
		t.Run("rewrite: "+query, func(t *testing.T) {
			if !needsRewriting(query) {
				t.Errorf("expected query to need rewriting: %q", query)
			}
		})
	}

	for _, query := range shouldNotRewrite {
		t.Run("no rewrite: "+query, func(t *testing.T) {
			if needsRewriting(query) {
				t.Errorf("expected query to NOT need rewriting: %q", query)
			}
		})
	}
}

// TestLineExtraction tests extracting specific lines from embedding text.
func TestLineExtraction(t *testing.T) {
	text := `Title: The Dark Knight (2008)
Genre: Action, Crime, Drama
Language: English
Country: United States
Era: 2000s
Plot: Batman faces the Joker.
Directed by: Christopher Nolan
Written by: Jonathan Nolan, Christopher Nolan
Cast: Christian Bale, Heath Ledger, Aaron Eckhart
Studios: Warner Bros., Legendary Pictures
Mood: dark, intense, thrilling`

	tests := []struct {
		prefix   string
		contains string
	}{
		{"title:", "the dark knight"},
		{"genre:", "action"},
		{"language:", "english"},
		{"country:", "united states"},
		{"directed by:", "christopher nolan"},
		{"written by:", "jonathan nolan"},
		{"cast:", "heath ledger"},
		{"studios:", "warner bros"},
		{"mood:", "dark"},
		{"nonexistent:", ""},
	}

	for _, tt := range tests {
		t.Run(tt.prefix, func(t *testing.T) {
			line := extractLine(text, tt.prefix)
			if tt.contains == "" {
				if line != "" {
					t.Errorf("expected empty line for prefix %q, got %q", tt.prefix, line)
				}
			} else if !strings.Contains(line, tt.contains) {
				t.Errorf("expected line to contain %q, got %q", tt.contains, line)
			}
		})
	}
}

// TestDirectorExtraction tests director name extraction from text.
func TestDirectorExtraction(t *testing.T) {
	tests := []struct {
		text     string
		expected string
	}{
		{"Title: Inception\nDirected by: Christopher Nolan\n", "christopher nolan"},
		{"Title: Movie\nDirected by: The Wachowskis\n", "the wachowskis"},
		{"Title: Movie\nGenre: Action\n", ""},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := extractDirector(strings.ToLower(tt.text))
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

// BenchmarkIntentDetection benchmarks the intent detection function.
func BenchmarkIntentDetection(b *testing.B) {
	queries := []string{
		"Christopher Nolan movies",
		"movies directed by Quentin Tarantino",
		"action movies starring Tom Hanks",
		"cozy romantic comedy for date night",
		"horror thriller set in Tokyo",
		"French films from the 60s",
		"Korean thriller recommendations",
		"movies by Pixar",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, q := range queries {
			detectQueryIntent(q)
		}
	}
}

// BenchmarkKeywordBoost benchmarks the keyword boost function.
func BenchmarkKeywordBoost(b *testing.B) {
	service := &SearchService{minSimilarity: 0.3}
	results := make([]SearchResult, 100)
	for i := range results {
		results[i] = SearchResult{
			Similarity: 0.5 + float32(i)*0.005,
			Text:       "Title: Movie " + string(rune('A'+i%26)) + "\nGenre: Action\nDirected by: Director\nCast: Actor\n",
		}
	}

	queries := []string{
		"action movies",
		"directed by Nolan",
		"French films",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, q := range queries {
			service.applyKeywordBoost(results, q, strings.ToLower(q))
		}
	}
}

// TestLanguageNameMapping tests language code to name conversion.
func TestLanguageNameMapping(t *testing.T) {
	// This tests the helper in indexing.go
	tests := []struct {
		code     string
		expected string
	}{
		{"en", "English"},
		{"fr", "French"},
		{"ko", "Korean"},
		{"ja", "Japanese"},
		{"es", "Spanish"},
		{"de", "German"},
		{"it", "Italian"},
		{"zh", "Chinese"},
		{"hi", "Hindi"},
		{"xx", "xx"}, // Unknown code returns as-is
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			result := getLanguageName(tt.code)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestEraLabeling tests decade/era label generation.
func TestEraLabeling(t *testing.T) {
	tests := []struct {
		year     int32
		contains string
	}{
		{2023, "2020s"},
		{2015, "2010s"},
		{2005, "2000s"},
		{1995, "90s"},
		{1985, "80s"},
		{1975, "70s"},
		{1965, "60s"},
		{1955, "50s"},
		{1945, "40s"},
		{1935, "30s"},
		{1920, "silent"},
	}

	for _, tt := range tests {
		t.Run(tt.contains, func(t *testing.T) {
			result := getEraLabel(tt.year)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("year %d: got %q, expected to contain %q", tt.year, result, tt.contains)
			}
		})
	}
}
