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
