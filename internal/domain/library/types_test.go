package library

import (
	"testing"
)

func TestLibraryTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		libType  LibraryType
		expected string
	}{
		{"movies", LibraryTypeMovies, "movies"},
		{"tv", LibraryTypeTV, "tv"},
		{"music", LibraryTypeMusic, "music"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.libType) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, tt.libType)
			}
		})
	}
}

func TestLibraryTypeIsValid(t *testing.T) {
	tests := []struct {
		name     string
		libType  LibraryType
		expected bool
	}{
		{"valid movies", LibraryTypeMovies, true},
		{"valid tv", LibraryTypeTV, true},
		{"valid music", LibraryTypeMusic, true},
		{"invalid empty", LibraryType(""), false},
		{"invalid unknown", LibraryType("unknown"), false},
		{"invalid random", LibraryType("random"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.libType.IsValid()
			if result != tt.expected {
				t.Errorf("Expected IsValid()=%v for %s, got %v", tt.expected, tt.libType, result)
			}
		})
	}
}

func TestLibraryTypeString(t *testing.T) {
	tests := []struct {
		name     string
		libType  LibraryType
		expected string
	}{
		{"movies to string", LibraryTypeMovies, "movies"},
		{"tv to string", LibraryTypeTV, "tv"},
		{"music to string", LibraryTypeMusic, "music"},
		{"empty to string", LibraryType(""), ""},
		{"custom to string", LibraryType("custom"), "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.libType.String()
			if result != tt.expected {
				t.Errorf("Expected String()=%s, got %s", tt.expected, result)
			}
		})
	}
}
