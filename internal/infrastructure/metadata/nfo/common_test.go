package nfo

import "testing"

func TestExtractEpisodePattern(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		// Standard patterns
		{"standard S05E13", "Star Trek Voyager - S05E13 - Gravity", "S05E13"},
		{"lowercase s05e13", "show - s05e13 - title", "S05E13"},
		{"mixed case S5e13", "show - S5e13 - title", "S05E13"},

		// Different naming conventions
		{"Plex style", "Star Trek - Voyager (1995) - S05E13 - Gravity [Bluray-1080p][AC3 5.1][x265]", "S05E13"},
		{"simple format", "ShowS01E01Title", "S01E01"},
		{"with dots", "Show.S02E05.Title", "S02E05"},

		// Single digit episode numbers
		{"single digit episode", "show - S01E1 - pilot", "S01E01"},
		{"single digit season", "show - S1E01 - pilot", "S01E01"},
		{"both single digit", "show - S1E1 - pilot", "S01E01"},

		// High numbers
		{"high season", "show - S15E99 - finale", "S15E99"},
		{"three digit episode", "anime - S01E100 - special", "S01E100"},

		// No pattern
		{"no pattern - movie", "The Matrix (1999)", ""},
		{"no pattern - just text", "random text", ""},
		{"no pattern - season only", "Season 05", ""},
		{"no pattern - episode only", "Episode 13", ""},

		// Edge cases
		{"empty string", "", ""},
		{"partial pattern SE", "show - SE13", ""},
		{"partial pattern S0E", "show - S0E", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractEpisodePattern(tt.filename)
			if got != tt.want {
				t.Errorf("extractEpisodePattern(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}

func TestExtractEpisodePattern_NFOMatching(t *testing.T) {
	// Test that media and NFO files with different naming conventions
	// but same episode numbers are matched correctly
	mediaFilenames := []string{
		"Star Trek Voyager - S05E13 - Gravity.avi",
		"Star Trek Voyager - S05E13 - Gravity",
	}
	nfoFilenames := []string{
		"Star Trek - Voyager (1995) - S05E13 - Gravity [Bluray-1080p][AC3 5.1][x265].nfo",
		"Star Trek - Voyager (1995) - S05E13 - Gravity [Bluray-1080p][AC3 5.1][x265]",
	}

	for _, media := range mediaFilenames {
		mediaPattern := extractEpisodePattern(media)
		if mediaPattern != "S05E13" {
			t.Errorf("Media %q should have pattern S05E13, got %q", media, mediaPattern)
		}

		for _, nfo := range nfoFilenames {
			nfoPattern := extractEpisodePattern(nfo)
			if nfoPattern != "S05E13" {
				t.Errorf("NFO %q should have pattern S05E13, got %q", nfo, nfoPattern)
			}

			// Verify they match
			if mediaPattern != nfoPattern {
				t.Errorf("Media %q (pattern %q) should match NFO %q (pattern %q)",
					media, mediaPattern, nfo, nfoPattern)
			}
		}
	}

	// Test that different episodes don't match
	media := "Star Trek Voyager - S05E13 - Gravity.avi"
	wrongNFO := "Star Trek - Voyager (1995) - S05E01 - Night [Bluray-1080p][AC3 5.1][x265].nfo"

	mediaPattern := extractEpisodePattern(media)
	wrongPattern := extractEpisodePattern(wrongNFO)

	if mediaPattern == wrongPattern {
		t.Errorf("Media S05E13 should NOT match NFO S05E01: %q == %q", mediaPattern, wrongPattern)
	}
}
