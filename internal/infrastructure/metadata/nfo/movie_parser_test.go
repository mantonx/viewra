package nfo

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseMovieNFO(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	// Create a sample NFO file
	nfoContent := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<movie>
	<title>The Matrix</title>
	<originaltitle>The Matrix</originaltitle>
	<sorttitle>Matrix</sorttitle>
	<year>1999</year>
	<releasedate>1999-03-31</releasedate>
	<plot>A computer hacker learns about the true nature of reality.</plot>
	<tagline>Welcome to the Real World</tagline>
	<runtime>136</runtime>
	<director>Lana Wachowski</director>
	<mpaa>R</mpaa>
	<imdb>tt0133093</imdb>
	<tmdbid>603</tmdbid>
	<genre>Action</genre>
	<genre>Science Fiction</genre>
	<actor>
		<name>Keanu Reeves</name>
		<role>Neo</role>
	</actor>
	<actor>
		<name>Laurence Fishburne</name>
		<role>Morpheus</role>
	</actor>
	<actor>
		<name>Carrie-Anne Moss</name>
		<role>Trinity</role>
	</actor>
	<country>USA</country>
	<originallanguage>en</originallanguage>
	<budget>63000000</budget>
	<revenue>466364845</revenue>
</movie>`

	nfoPath := filepath.Join(tmpDir, "movie.nfo")
	if err := os.WriteFile(nfoPath, []byte(nfoContent), 0644); err != nil {
		t.Fatalf("Failed to create test NFO file: %v", err)
	}

	// Parse the NFO
	metadata, err := ParseMovieNFO(nfoPath)
	if err != nil {
		t.Fatalf("ParseMovieNFO failed: %v", err)
	}

	// Verify basic fields
	if metadata.Title != "The Matrix" {
		t.Errorf("Expected title 'The Matrix', got '%s'", metadata.Title)
	}

	if metadata.Year != 1999 {
		t.Errorf("Expected year 1999, got %d", metadata.Year)
	}

	if metadata.RuntimeMinutes != 136 {
		t.Errorf("Expected runtime 136, got %d", metadata.RuntimeMinutes)
	}

	if metadata.Director != "Lana Wachowski" {
		t.Errorf("Expected director 'Lana Wachowski', got '%s'", metadata.Director)
	}

	// Verify IDs
	if metadata.IMDbID != "tt0133093" {
		t.Errorf("Expected IMDb ID 'tt0133093', got '%s'", metadata.IMDbID)
	}

	if metadata.TMDbID != 603 {
		t.Errorf("Expected TMDb ID 603, got %d", metadata.TMDbID)
	}

	// Verify cast
	if len(metadata.Cast) != 3 {
		t.Errorf("Expected 3 cast members, got %d", len(metadata.Cast))
	}

	expectedCast := []string{"Keanu Reeves", "Laurence Fishburne", "Carrie-Anne Moss"}
	for i, name := range expectedCast {
		if i >= len(metadata.Cast) || metadata.Cast[i].Name != name {
			castName := ""
			if i < len(metadata.Cast) {
				castName = metadata.Cast[i].Name
			}
			t.Errorf("Expected cast[%d] to be '%s', got '%s'", i, name, castName)
		}
	}

	// Verify genres
	if len(metadata.Genre) != 2 {
		t.Errorf("Expected 2 genres, got %d", len(metadata.Genre))
	}

	// Verify release date
	expectedDate := time.Date(1999, 3, 31, 0, 0, 0, 0, time.UTC)
	if !metadata.ReleaseDate.Equal(expectedDate) {
		t.Errorf("Expected release date %v, got %v", expectedDate, metadata.ReleaseDate)
	}

	// Verify financial data
	if metadata.Budget != 63000000 {
		t.Errorf("Expected budget 63000000, got %d", metadata.Budget)
	}

	if metadata.Revenue != 466364845 {
		t.Errorf("Expected revenue 466364845, got %d", metadata.Revenue)
	}
}

func TestParseMovieNFO_WithUniqueIDs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create NFO with new UniqueID format
	nfoContent := `<?xml version="1.0" encoding="UTF-8"?>
<movie>
	<title>Inception</title>
	<year>2010</year>
	<uniqueid type="imdb" default="true">tt1375666</uniqueid>
	<uniqueid type="tmdb">27205</uniqueid>
	<runtime>148 min</runtime>
</movie>`

	nfoPath := filepath.Join(tmpDir, "movie.nfo")
	if err := os.WriteFile(nfoPath, []byte(nfoContent), 0644); err != nil {
		t.Fatalf("Failed to create test NFO file: %v", err)
	}

	metadata, err := ParseMovieNFO(nfoPath)
	if err != nil {
		t.Fatalf("ParseMovieNFO failed: %v", err)
	}

	if metadata.IMDbID != "tt1375666" {
		t.Errorf("Expected IMDb ID 'tt1375666', got '%s'", metadata.IMDbID)
	}

	if metadata.TMDbID != 27205 {
		t.Errorf("Expected TMDb ID 27205, got %d", metadata.TMDbID)
	}

	// Runtime with "min" suffix should be parsed correctly
	if metadata.RuntimeMinutes != 148 {
		t.Errorf("Expected runtime 148, got %d", metadata.RuntimeMinutes)
	}
}

func TestFindMovieNFO(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a movie file
	moviePath := filepath.Join(tmpDir, "The Matrix (1999).mkv")
	if err := os.WriteFile(moviePath, []byte("fake movie"), 0644); err != nil {
		t.Fatalf("Failed to create test movie file: %v", err)
	}

	// Test 1: No NFO file exists
	_, err := FindMovieNFO(moviePath)
	if err == nil {
		t.Error("Expected error when no NFO exists, got nil")
	}

	// Test 2: Exact match NFO
	nfoPath := filepath.Join(tmpDir, "The Matrix (1999).nfo")
	if err := os.WriteFile(nfoPath, []byte("<movie></movie>"), 0644); err != nil {
		t.Fatalf("Failed to create NFO file: %v", err)
	}

	foundPath, err := FindMovieNFO(moviePath)
	if err != nil {
		t.Fatalf("FindMovieNFO failed: %v", err)
	}

	if foundPath != nfoPath {
		t.Errorf("Expected to find '%s', got '%s'", nfoPath, foundPath)
	}
}

func TestCleanIMDbID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"tt0133093", "tt0133093"},
		{"0133093", "tt0133093"},
		{"imdb://tt0133093", "tt0133093"},
		{"https://www.imdb.com/title/tt0133093/", "tt0133093"},
		{"http://www.imdb.com/title/tt0133093", "tt0133093"},
		{"", ""},
	}

	for _, tt := range tests {
		result := cleanIMDbID(tt.input)
		if result != tt.expected {
			t.Errorf("cleanIMDbID(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestParseRuntime(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"136", 136},
		{"136 min", 136},
		{"136 minutes", 136},
		{"148 min", 148},
		{"  90  ", 90},
		{"", 0},
	}

	for _, tt := range tests {
		result := parseRuntime(tt.input)
		if result != tt.expected {
			t.Errorf("parseRuntime(%q) = %d, expected %d", tt.input, result, tt.expected)
		}
	}
}

func TestParseMoney(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"63000000", 63000000},
		{"$63,000,000", 63000000},
		{"€63000000", 63000000},
		{"£63,000,000", 63000000},
		{"", 0},
	}

	for _, tt := range tests {
		result := parseMoney(tt.input)
		if result != tt.expected {
			t.Errorf("parseMoney(%q) = %d, expected %d", tt.input, result, tt.expected)
		}
	}
}
