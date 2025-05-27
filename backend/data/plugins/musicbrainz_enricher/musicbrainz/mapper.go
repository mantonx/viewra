package musicbrainz

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// EnrichedMetadata represents enriched metadata ready for storage
type EnrichedMetadata struct {
	MusicBrainzRecordingID string
	MusicBrainzReleaseID   string
	MusicBrainzArtistID    string
	EnrichedTitle          string
	EnrichedArtist         string
	EnrichedAlbum          string
	EnrichedAlbumArtist    string
	EnrichedYear           int
	EnrichedGenre          string
	EnrichedTrackNumber    int
	EnrichedDiscNumber     int
	MatchScore             float64
	ArtworkURL             string
	ArtworkPath            string
}

// Matcher handles matching logic between MusicBrainz data and existing metadata
type Matcher struct {
	threshold float64
}

// NewMatcher creates a new matcher with the given threshold
func NewMatcher(threshold float64) *Matcher {
	return &Matcher{threshold: threshold}
}

// FindBestMatch finds the best matching recording from search results
func (m *Matcher) FindBestMatch(recordings []Recording, title, artist, album string) *MatchResult {
	var bestMatch *Recording
	var bestScore float64

	for i := range recordings {
		score := m.calculateMatchScore(&recordings[i], title, artist, album)
		if score > bestScore {
			bestScore = score
			bestMatch = &recordings[i]
		}
	}

	if bestMatch == nil || bestScore < m.threshold {
		return &MatchResult{
			Recording: bestMatch,
			Score:     bestScore,
			Matched:   false,
		}
	}

	return &MatchResult{
		Recording: bestMatch,
		Score:     bestScore,
		Matched:   true,
	}
}

// calculateMatchScore calculates a similarity score between a recording and metadata
func (m *Matcher) calculateMatchScore(recording *Recording, title, artist, album string) float64 {
	score := 0.0

	// Title match (40% weight)
	titleScore := m.calculateStringScore(recording.Title, title)
	score += titleScore * 0.4

	// Artist match (30% weight)
	if len(recording.Artists) > 0 {
		artistScore := m.calculateStringScore(recording.Artists[0].Name, artist)
		score += artistScore * 0.3
	}

	// Album match (30% weight)
	if len(recording.Releases) > 0 {
		albumScore := m.calculateStringScore(recording.Releases[0].Title, album)
		score += albumScore * 0.3
	}

	return score
}

// calculateStringScore calculates similarity between two strings
func (m *Matcher) calculateStringScore(s1, s2 string) float64 {
	if s1 == "" || s2 == "" {
		return 0.0
	}

	// Normalize strings for comparison
	norm1 := m.normalizeString(s1)
	norm2 := m.normalizeString(s2)

	// Exact match
	if norm1 == norm2 {
		return 1.0
	}

	// Contains match
	if strings.Contains(norm1, norm2) || strings.Contains(norm2, norm1) {
		return 0.7
	}

	// Simple word overlap scoring
	words1 := strings.Fields(norm1)
	words2 := strings.Fields(norm2)
	
	if len(words1) == 0 || len(words2) == 0 {
		return 0.0
	}

	matches := 0
	for _, word1 := range words1 {
		for _, word2 := range words2 {
			if word1 == word2 && len(word1) > 2 { // Only count meaningful words
				matches++
				break
			}
		}
	}

	// Calculate overlap ratio
	maxWords := len(words1)
	if len(words2) > maxWords {
		maxWords = len(words2)
	}

	return float64(matches) / float64(maxWords)
}

// normalizeString normalizes a string for comparison
func (m *Matcher) normalizeString(s string) string {
	// Convert to lowercase
	s = strings.ToLower(s)
	
	// Remove common punctuation and extra spaces
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "!", "")
	s = strings.ReplaceAll(s, "?", "")
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, "\"", "")
	s = strings.ReplaceAll(s, "(", "")
	s = strings.ReplaceAll(s, ")", "")
	s = strings.ReplaceAll(s, "[", "")
	s = strings.ReplaceAll(s, "]", "")
	
	// Normalize whitespace
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

// MapToEnrichedMetadata converts a MusicBrainz recording to enriched metadata
func MapToEnrichedMetadata(recording *Recording, matchScore float64) *EnrichedMetadata {
	metadata := &EnrichedMetadata{
		MusicBrainzRecordingID: recording.ID,
		EnrichedTitle:          recording.Title,
		MatchScore:             matchScore,
	}

	// Extract artist information
	if len(recording.Artists) > 0 {
		metadata.MusicBrainzArtistID = recording.Artists[0].Artist.ID
		metadata.EnrichedArtist = recording.Artists[0].Name
	}

	// Extract release information
	if len(recording.Releases) > 0 {
		release := recording.Releases[0]
		metadata.MusicBrainzReleaseID = release.ID
		metadata.EnrichedAlbum = release.Title

		// Extract year from date
		if release.Date != "" {
			if year, err := extractYear(release.Date); err == nil {
				metadata.EnrichedYear = year
			}
		}

		// Extract album artist (may be different from track artist)
		if len(release.Artists) > 0 {
			metadata.EnrichedAlbumArtist = release.Artists[0].Name
		}

		// Extract track and disc information
		for _, medium := range release.Media {
			for _, track := range medium.Tracks {
				if track.ID == recording.ID {
					metadata.EnrichedTrackNumber = track.Position
					metadata.EnrichedDiscNumber = medium.Position
					break
				}
			}
		}

		// Extract genre from release group
		if release.ReleaseGroup.PrimaryType != "" {
			metadata.EnrichedGenre = release.ReleaseGroup.PrimaryType
		}
	}

	return metadata
}

// extractYear extracts a year from a date string (YYYY-MM-DD or YYYY)
func extractYear(dateStr string) (int, error) {
	// Handle different date formats
	if len(dateStr) >= 4 {
		yearStr := dateStr[:4]
		return strconv.Atoi(yearStr)
	}
	return 0, strconv.ErrSyntax
}

// MapArtworkInfo maps cover art information to artwork metadata
func MapArtworkInfo(coverArt *CoverArtResponse, bestImage *CoverArtImage, mediaFileID uint, releaseID string) (string, string) {
	if bestImage == nil {
		return "", ""
	}

	artworkURL := bestImage.Image
	artworkPath := generateArtworkPath(mediaFileID, releaseID)

	return artworkURL, artworkPath
}

// generateArtworkPath generates a local path for storing artwork
func generateArtworkPath(mediaFileID uint, releaseID string) string {
	timestamp := time.Now().Unix()
	return fmt.Sprintf("artwork/musicbrainz_%d_%s_%d.jpg", mediaFileID, releaseID, timestamp)
} 