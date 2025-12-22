package nfo

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// episodePatternRegex matches TV episode patterns like S05E13, s05e13, S5E13, etc.
// Captures season and episode numbers for normalization.
var episodePatternRegex = regexp.MustCompile(`(?i)S(\d+)E(\d+)`)

// extractEpisodePattern extracts and normalizes the episode pattern from a filename.
// Returns a normalized pattern like "S05E13" or empty string if no pattern found.
// This allows matching "Star Trek Voyager - S05E13" with "Star Trek - Voyager (1995) - S05E13 - Gravity".
func extractEpisodePattern(filename string) string {
	match := episodePatternRegex.FindStringSubmatch(filename)
	if match == nil {
		return ""
	}
	// Normalize to uppercase with zero-padded numbers (e.g., "S05E13")
	season, _ := strconv.Atoi(match[1])
	episode, _ := strconv.Atoi(match[2])
	return fmt.Sprintf("S%02dE%02d", season, episode)
}

// Common helper functions shared across all NFO parsers

// fileExists checks if a file exists at the given path
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// parseDate attempts to parse a date string using common formats
func parseDate(dateStr string) (time.Time, error) {
	// Try common date formats
	formats := []string{
		"2006-01-02",
		"2006/01/02",
		"02-01-2006",
		"02/01/2006",
		"January 2, 2006",
		"Jan 2, 2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}

// parseRuntime extracts minutes from various runtime formats
// Handles: "90", "90 min", "90 minutes", etc.
func parseRuntime(runtime string) int {
	// Remove non-numeric characters (order matters: remove "minutes" before "min")
	runtime = strings.TrimSpace(runtime)
	runtime = strings.ReplaceAll(runtime, "minutes", "")
	runtime = strings.ReplaceAll(runtime, "min", "")
	runtime = strings.TrimSpace(runtime)

	minutes, _ := strconv.Atoi(runtime)
	return minutes
}

// cleanIMDbID normalizes IMDb IDs to standard format (ttXXXXXXX)
func cleanIMDbID(id string) string {
	id = strings.TrimSpace(id)

	// Remove imdb:// prefix and URL paths
	id = strings.TrimPrefix(id, "imdb://")
	id = strings.TrimPrefix(id, "https://www.imdb.com/title/")
	id = strings.TrimPrefix(id, "http://www.imdb.com/title/")
	id = strings.TrimSuffix(id, "/")

	// Ensure it starts with tt
	if id != "" && !strings.HasPrefix(id, "tt") {
		id = "tt" + id
	}

	return id
}

// parseIntSafe safely parses a string to int, returning 0 on error
func parseIntSafe(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	val, _ := strconv.Atoi(s)
	return val
}

// parseInt64Safe safely parses a string to int64, returning 0 on error
func parseInt64Safe(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	val, _ := strconv.ParseInt(s, 10, 64)
	return val
}

// parseMoney strips currency symbols and parses monetary values
func parseMoney(money string) int64 {
	// Remove currency symbols and commas
	money = strings.TrimSpace(money)
	money = strings.ReplaceAll(money, "$", "")
	money = strings.ReplaceAll(money, "€", "")
	money = strings.ReplaceAll(money, "£", "")
	money = strings.ReplaceAll(money, ",", "")
	money = strings.TrimSpace(money)

	if money == "" {
		return 0
	}

	val, _ := strconv.ParseInt(money, 10, 64)
	return val
}

// parseFloat32Safe safely parses a string to float32, returning 0 on error
func parseFloat32Safe(s string) float32 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	val, _ := strconv.ParseFloat(s, 32)
	return float32(val)
}

// Actor represents a cast/crew member (shared across all media types)
type Actor struct {
	Name  string `xml:"name"`
	Role  string `xml:"role"`
	Order int    `xml:"order"`
	Thumb string `xml:"thumb"`
}

// UniqueID represents the modern uniqueid format
type UniqueID struct {
	Type    string `xml:"type,attr"`
	Default bool   `xml:"default,attr"`
	Value   string `xml:",chardata"`
}

// Rating represents the modern ratings format
type Rating struct {
	Name    string  `xml:"name,attr"`
	Max     float32 `xml:"max,attr"`
	Default bool    `xml:"default,attr"`
	Value   float32 `xml:"value"`
	Votes   int     `xml:"votes"`
}

// FindNFOFile searches for an NFO file with various naming conventions
// baseName is the media file name without extension
// dir is the directory to search in
// specificNames are specific NFO filenames to check (e.g., "tvshow.nfo", "movie.nfo")
func FindNFOFile(mediaPath string, specificNames ...string) (string, error) {
	dir := filepath.Dir(mediaPath)
	base := strings.TrimSuffix(filepath.Base(mediaPath), filepath.Ext(mediaPath))

	// Try exact match: filename.nfo
	nfoPath := filepath.Join(dir, base+".nfo")
	if fileExists(nfoPath) {
		return nfoPath, nil
	}

	// Try alternate: filename-nfo.xml
	nfoPath = filepath.Join(dir, base+"-nfo.xml")
	if fileExists(nfoPath) {
		return nfoPath, nil
	}

	// Try specific names (e.g., "movie.nfo", "tvshow.nfo")
	for _, name := range specificNames {
		nfoPath = filepath.Join(dir, name)
		if fileExists(nfoPath) {
			return nfoPath, nil
		}
	}

	// Extract episode pattern from media filename (e.g., "S05E13", "s05e13")
	// This prevents matching S05E01's NFO to S05E13's media file
	episodePattern := extractEpisodePattern(base)

	// Look for any .nfo file in the same directory as last resort
	// If the media file has an episode pattern, only match NFOs with the same pattern
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".nfo") {
			continue
		}

		// If media file has an episode pattern, NFO must have the same pattern
		if episodePattern != "" {
			nfoBase := strings.TrimSuffix(name, filepath.Ext(name))
			nfoPattern := extractEpisodePattern(nfoBase)
			if nfoPattern != episodePattern {
				continue // Skip NFOs with different episode numbers
			}
		}

		return filepath.Join(dir, name), nil
	}

	return "", fmt.Errorf("no NFO file found for: %s", mediaPath)
}

// extractActorNames extracts just the names from a slice of Actor structs.
// Actors are sorted by their Order field if specified in the NFO.
func extractActorNames(actors []Actor) []string {
	if len(actors) == 0 {
		return nil
	}

	// Sort actors by order (stable sort preserves original order for equal values)
	sorted := make([]Actor, len(actors))
	copy(sorted, actors)
	sortActorsByOrder(sorted)

	names := make([]string, 0, len(sorted))
	for _, actor := range sorted {
		if actor.Name != "" {
			names = append(names, actor.Name)
		}
	}
	return names
}

// CastMemberInfo holds cast member info extracted from NFO
type CastMemberInfo struct {
	Name  string
	Role  string
	Order int
}

// extractCastMembers extracts cast members with order info from a slice of Actor structs.
// Actors are sorted by their Order field if specified in the NFO.
func extractCastMembers(actors []Actor) []CastMemberInfo {
	if len(actors) == 0 {
		return nil
	}

	// Sort actors by order (stable sort preserves original order for equal values)
	sorted := make([]Actor, len(actors))
	copy(sorted, actors)
	sortActorsByOrder(sorted)

	cast := make([]CastMemberInfo, 0, len(sorted))
	for i, actor := range sorted {
		if actor.Name != "" {
			order := actor.Order
			if order == 0 {
				// If no order specified, use iteration index
				order = i
			}
			cast = append(cast, CastMemberInfo{
				Name:  actor.Name,
				Role:  actor.Role,
				Order: order,
			})
		}
	}
	return cast
}

// sortActorsByOrder sorts actors by their Order field.
// Actors without an order (Order == 0) are placed after those with explicit order.
func sortActorsByOrder(actors []Actor) {
	// Use a simple stable sort
	for i := 1; i < len(actors); i++ {
		for j := i; j > 0; j-- {
			// If both have order 0, keep original position
			if actors[j-1].Order == 0 && actors[j].Order == 0 {
				break
			}
			// If previous has no order but current does, swap
			if actors[j-1].Order == 0 && actors[j].Order > 0 {
				actors[j-1], actors[j] = actors[j], actors[j-1]
				continue
			}
			// If both have order, sort by order
			if actors[j-1].Order > 0 && actors[j].Order > 0 && actors[j-1].Order > actors[j].Order {
				actors[j-1], actors[j] = actors[j], actors[j-1]
				continue
			}
			break
		}
	}
}

// parseIDFromUniqueIDs extracts a specific ID type from UniqueID array
func parseIDFromUniqueIDs(uniqueIDs []UniqueID, idType string) string {
	for _, uid := range uniqueIDs {
		if strings.EqualFold(uid.Type, idType) {
			return uid.Value
		}
	}
	return ""
}

// getBestRating extracts the best rating from the ratings array (prefers default)
func getBestRating(ratings []Rating) float32 {
	if len(ratings) == 0 {
		return 0
	}

	// Prefer default rating
	for _, r := range ratings {
		if r.Default {
			return r.Value
		}
	}

	// Otherwise return first rating
	return ratings[0].Value
}

// ErrEmptyNFO is returned when an NFO file is empty
var ErrEmptyNFO = fmt.Errorf("NFO file is empty")

// ErrWrongNFOType is returned when an NFO file contains the wrong root element
type ErrWrongNFOType struct {
	Expected string
	Actual   string
}

func (e ErrWrongNFOType) Error() string {
	return fmt.Sprintf("NFO contains <%s> instead of expected <%s>", e.Actual, e.Expected)
}

// detectNFORootElement reads the start of an NFO file to determine its root element type.
// Returns the root element name (e.g., "movie", "tvshow", "episodedetails") or empty string if undetermined.
func detectNFORootElement(data []byte) string {
	// Skip BOM and whitespace
	content := strings.TrimSpace(string(data))

	// Skip XML declaration if present
	if strings.HasPrefix(content, "<?xml") {
		if idx := strings.Index(content, "?>"); idx != -1 {
			content = strings.TrimSpace(content[idx+2:])
		}
	}

	// Find the first opening tag (skip comments)
	for strings.HasPrefix(content, "<!--") {
		if idx := strings.Index(content, "-->"); idx != -1 {
			content = strings.TrimSpace(content[idx+3:])
		} else {
			break
		}
	}

	// Extract root element name
	if !strings.HasPrefix(content, "<") {
		return ""
	}

	content = content[1:] // Remove leading <

	// Find end of tag name (space, >, or /)
	endIdx := strings.IndexAny(content, " \t\n\r>/?")
	if endIdx == -1 {
		return ""
	}

	return strings.ToLower(content[:endIdx])
}

// validateNFOData checks if NFO data is valid and matches the expected type.
// Returns nil if valid, ErrEmptyNFO if empty, or ErrWrongNFOType if mismatched.
func validateNFOData(data []byte, expectedRoot string) error {
	if len(data) == 0 {
		return ErrEmptyNFO
	}

	// Check for only whitespace
	if len(strings.TrimSpace(string(data))) == 0 {
		return ErrEmptyNFO
	}

	// Detect actual root element
	actualRoot := detectNFORootElement(data)
	if actualRoot == "" {
		return ErrEmptyNFO
	}

	// Check if it matches expected
	if actualRoot != expectedRoot {
		return ErrWrongNFOType{Expected: expectedRoot, Actual: actualRoot}
	}

	return nil
}
