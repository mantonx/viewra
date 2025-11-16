package nfo

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

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

	// Look for any .nfo file in the same directory as last resort
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".nfo") {
			return filepath.Join(dir, entry.Name()), nil
		}
	}

	return "", fmt.Errorf("no NFO file found for: %s", mediaPath)
}

// extractActorNames extracts just the names from a slice of Actor structs
func extractActorNames(actors []Actor) []string {
	names := make([]string, 0, len(actors))
	for _, actor := range actors {
		if actor.Name != "" {
			names = append(names, actor.Name)
		}
	}
	return names
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
