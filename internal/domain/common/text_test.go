package common

import (
	"testing"
)

func TestNormalizeSortTitle(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "accented characters",
			input:    "À Nous la Liberté",
			expected: "nous la liberte",
		},
		{
			name:     "leading article The",
			input:    "The Matrix",
			expected: "matrix",
		},
		{
			name:     "leading article A",
			input:    "A Beautiful Mind",
			expected: "beautiful mind",
		},
		{
			name:     "leading article An",
			input:    "An American Tail",
			expected: "american tail",
		},
		{
			name:     "french article Le",
			input:    "Le Fabuleux Destin d'Amélie Poulain",
			expected: "fabuleux destin d'amelie poulain",
		},
		{
			name:     "spanish article El",
			input:    "El Laberinto del Fauno",
			expected: "laberinto del fauno",
		},
		{
			name:     "mixed accents and case",
			input:    "Café Société",
			expected: "cafe societe",
		},
		{
			name:     "no article or accents",
			input:    "Zombieland",
			expected: "zombieland",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "various accented vowels",
			input:    "àáâãäåèéêëìíîïòóôõöùúûü",
			expected: "aaaaaaeeeeiiiiooooouuuu",
		},
		{
			name:     "leading apostrophe",
			input:    "'Round Midnight",
			expected: "round midnight",
		},
		{
			name:     "leading parentheses",
			input:    "(500) Days of Summer",
			expected: "days of summer",
		},
		{
			name:     "leading ellipsis",
			input:    "...And Justice for All",
			expected: "and justice for all",
		},
		{
			name:     "multiple leading special chars",
			input:    "...'Bout Time",
			expected: "bout time",
		},
		{
			name:     "numeric title only",
			input:    "300",
			expected: "300",
		},
		{
			name:     "numeric with year",
			input:    "1917",
			expected: "1917",
		},
		{
			name:     "numeric with slash",
			input:    "50/50",
			expected: "50/50",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeSortTitle(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeSortTitle(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeTitle(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "colon removed",
			input:    "Star Trek: Voyager",
			expected: "Star Trek Voyager",
		},
		{
			name:     "hyphen to space",
			input:    "Spider-Man",
			expected: "Spider Man",
		},
		{
			name:     "slash to space",
			input:    "Giri/Haji",
			expected: "Giri Haji",
		},
		{
			name:     "plus to space",
			input:    "Giri+Haji",
			expected: "Giri Haji",
		},
		{
			name:     "ampersand to and",
			input:    "Tom & Jerry",
			expected: "Tom and Jerry",
		},
		{
			name:     "apostrophe removed",
			input:    "Ocean's Eleven",
			expected: "Oceans Eleven",
		},
		{
			name:     "smart apostrophe removed",
			input:    "Ocean's Eleven",
			expected: "Oceans Eleven",
		},
		{
			name:     "question mark removed",
			input:    "Are You Afraid of the Dark?",
			expected: "Are You Afraid of the Dark",
		},
		{
			name:     "exclamation mark removed",
			input:    "Wow!",
			expected: "Wow",
		},
		{
			name:     "parentheses removed",
			input:    "(500) Days of Summer",
			expected: "500 Days of Summer",
		},
		{
			name:     "multiple punctuation",
			input:    "What's Up, Doc?",
			expected: "Whats Up Doc",
		},
		{
			name:     "collapses multiple spaces",
			input:    "The   Quick   Brown   Fox",
			expected: "The Quick Brown Fox",
		},
		{
			name:     "complex title",
			input:    "Dr. Strangelove or: How I Learned to Stop Worrying & Love the Bomb",
			expected: "Dr Strangelove or How I Learned to Stop Worrying and Love the Bomb",
		},
		{
			name:     "en-dash to space",
			input:    "2001–2020",
			expected: "2001 2020",
		},
		{
			name:     "em-dash to space",
			input:    "hello—world",
			expected: "hello world",
		},
		{
			name:     "double quotes removed",
			input:    "\"Hello World\"",
			expected: "Hello World",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeTitle(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeTitle(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRemoveLeadingArticles(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"The", "The Matrix", "Matrix"},
		{"A", "A Beautiful Mind", "Beautiful Mind"},
		{"An", "An American Tail", "American Tail"},
		{"Le (French)", "Le Fabuleux", "Fabuleux"},
		{"La (French)", "La Vie en Rose", "Vie en Rose"},
		{"Les (French)", "Les Misérables", "Misérables"},
		{"El (Spanish)", "El Mariachi", "Mariachi"},
		{"Los (Spanish)", "Los Amantes", "Amantes"},
		{"Der (German)", "Der Untergang", "Untergang"},
		{"Die (German)", "Die Hard", "Hard"},
		{"Das (German)", "Das Boot", "Boot"},
		{"Il (Italian)", "Il Postino", "Postino"},
		{"lowercase the", "the thing", "thing"},
		{"no article", "Matrix", "Matrix"},
		{"article in middle", "This The That", "This The That"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeLeadingArticles(tt.input)
			if result != tt.expected {
				t.Errorf("removeLeadingArticles(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRemoveLeadingSpecialChars(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"leading apostrophe", "'Round Midnight", "Round Midnight"},
		{"leading parenthesis", "(500) Days", "Days"},
		{"leading ellipsis", "...And Justice", "And Justice"},
		{"numeric only", "300", "300"},
		{"numeric year", "1917", "1917"},
		{"numeric with slash", "50/50", "50/50"},
		{"no special chars", "Hello World", "Hello World"},
		{"all special chars no letters", "123", "123"},
		{"empty string", "", ""},
		{"multiple leading special chars", "...'Bout Time", "Bout Time"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeLeadingSpecialChars(tt.input)
			if result != tt.expected {
				t.Errorf("removeLeadingSpecialChars(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRemoveAccents(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"no accents", "Hello World", "Hello World"},
		{"french accents", "Café", "Cafe"},
		{"grave accent", "à", "a"},
		{"acute accent", "é", "e"},
		{"circumflex", "ô", "o"},
		{"tilde", "ñ", "n"},
		{"umlaut", "ü", "u"},
		{"cedilla", "ç", "c"},
		{"multiple accents", "Amélie", "Amelie"},
		{"mixed", "naïve résumé", "naive resume"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeAccents(tt.input)
			if result != tt.expected {
				t.Errorf("removeAccents(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
