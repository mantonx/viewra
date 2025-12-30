package querier

import (
	"testing"
)

func TestTitleMatchScore(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		search   string
		minScore int
		maxScore int
	}{
		{
			name:     "exact match",
			title:    "the matrix",
			search:   "the matrix",
			minScore: 100,
			maxScore: 100,
		},
		{
			name:     "exact match ignoring the",
			title:    "the matrix",
			search:   "matrix",
			minScore: 90,
			maxScore: 100,
		},
		{
			name:     "starts with",
			title:    "the matrix reloaded",
			search:   "the matrix",
			minScore: 70,
			maxScore: 90,
		},
		{
			name:     "contains as word",
			title:    "enter the matrix",
			search:   "matrix",
			minScore: 50,
			maxScore: 70,
		},
		{
			name:     "contains anywhere",
			title:    "thematrix",
			search:   "matrix",
			minScore: 30,
			maxScore: 50,
		},
		{
			name:     "no match pattern",
			title:    "inception",
			search:   "matrix",
			minScore: 0,
			maxScore: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := titleMatchScore(tt.title, tt.search)
			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("titleMatchScore(%q, %q) = %d, want between %d and %d",
					tt.title, tt.search, score, tt.minScore, tt.maxScore)
			}
		})
	}
}

func TestRankByTitleMatch(t *testing.T) {
	results := []*MediaInfo{
		{ID: 1, Title: "The Matrix Reloaded", MediaType: "movie"},
		{ID: 2, Title: "Enter The Matrix", MediaType: "movie"},
		{ID: 3, Title: "The Matrix", MediaType: "movie"},
		{ID: 4, Title: "Matrix", MediaType: "tv_show"},
	}

	rankByTitleMatch(results, "The Matrix")

	// Exact match should be first
	if results[0].Title != "The Matrix" {
		t.Errorf("Expected 'The Matrix' first, got %q", results[0].Title)
	}

	// "Matrix" (exact without "the") should be second
	if results[1].Title != "Matrix" {
		t.Errorf("Expected 'Matrix' second, got %q", results[1].Title)
	}

	// Prefix match should come before contains
	if results[2].Title != "The Matrix Reloaded" {
		t.Errorf("Expected 'The Matrix Reloaded' third, got %q", results[2].Title)
	}
}
