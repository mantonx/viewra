package parsers

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/viewra/viewra/internal/domain/scanner"
)

// Movie filename patterns to extract title and year
var moviePatterns = []*regexp.Regexp{
	// Title with year in parentheses: "The Matrix (1999).mkv"
	regexp.MustCompile(`(?i)^(.+?)\s*[\(\[](\d{4})[\)\]]`),

	// Title with year separated by dots/spaces: "Inception.2010.1080p.mkv"
	regexp.MustCompile(`(?i)^(.+?)[\s._-]+(\d{4})[\s._-]`),

	// Title with year at the end: "The Godfather 1972.mkv"
	regexp.MustCompile(`(?i)^(.+?)\s+(\d{4})$`),
}

// Movie quality/resolution patterns
var resolutionPatterns = map[string]string{
	"2160p": "4K",
	"1080p": "1080p",
	"720p":  "720p",
	"480p":  "480p",
	"4K":    "4K",
	"UHD":   "4K",
}

// Movie quality source patterns
var qualityPatterns = []string{
	"Remux", "BluRay", "BRRip", "BDRip", "WEBRip", "WEB-DL", "HDTV", "DVDRip",
	"WEBDL", "Bluray", "BRRIP", "WEBRIP", "Bluray",
}

// ParseMovie extracts movie metadata from a filename.
// It attempts to extract the title, year, resolution, quality, and IMDb ID from the filename.
func ParseMovie(filename string) (*scanner.MovieInfo, error) {
	// Get the filename without extension
	filenameNoExt := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))

	info := &scanner.MovieInfo{}

	// Extract IMDb ID if present (e.g., [imdbid-tt1234567])
	info.ImdbID = extractImdbID(filenameNoExt)

	// Extract edition info (e.g., "Extended Cut", "Director's Cut", "Remastered")
	info.Edition = extractEdition(filenameNoExt)

	// Try each pattern to extract title and year
	var titleRaw string
	yearFound := false

	for _, pattern := range moviePatterns {
		matches := pattern.FindStringSubmatch(filenameNoExt)
		if len(matches) >= 3 {
			titleRaw = matches[1]
			year, err := strconv.Atoi(matches[2])
			if err == nil && isValidMovieYear(year) {
				info.Year = year
				yearFound = true
				break
			}
		}
	}

	// If no year found with patterns, extract title without year
	if !yearFound {
		titleRaw = extractTitleWithoutYear(filenameNoExt)
	}

	// Clean the title
	info.Title = cleanMovieTitle(titleRaw)
	if info.Title == "" {
		return nil, fmt.Errorf("unable to extract movie title from: %s", filename)
	}

	// Extract resolution and quality
	info.Resolution = extractResolution(filenameNoExt)
	info.Quality = extractQuality(filenameNoExt)

	return info, nil
}

// extractTitleWithoutYear extracts the title when no year pattern is found
// It removes everything after quality/resolution tags
func extractTitleWithoutYear(filename string) string {
	filenameLower := strings.ToLower(filename)

	// Find the earliest occurrence of quality tags
	earliestIdx := len(filename)

	// Check for resolution tags
	for tag := range resolutionPatterns {
		tagLower := strings.ToLower(tag)
		if idx := strings.Index(filenameLower, tagLower); idx != -1 && idx < earliestIdx {
			earliestIdx = idx
		}
	}

	// Check for quality tags
	for _, tag := range qualityPatterns {
		tagLower := strings.ToLower(tag)
		if idx := strings.Index(filenameLower, tagLower); idx != -1 && idx < earliestIdx {
			earliestIdx = idx
		}
	}

	// Check for codec tags
	for _, tag := range qualityTags {
		tagLower := strings.ToLower(tag)
		if idx := strings.Index(filenameLower, tagLower); idx != -1 && idx < earliestIdx {
			earliestIdx = idx
		}
	}

	// If we found a tag, extract everything before it
	if earliestIdx < len(filename) {
		return filename[:earliestIdx]
	}

	return filename
}

// cleanMovieTitle removes quality tags, resolution, and normalizes the title
func cleanMovieTitle(title string) string {
	// Replace dots and underscores with spaces
	title = strings.ReplaceAll(title, ".", " ")
	title = strings.ReplaceAll(title, "_", " ")

	// Remove resolution tags
	titleLower := strings.ToLower(title)
	for tag := range resolutionPatterns {
		tagLower := strings.ToLower(tag)
		if idx := strings.Index(titleLower, tagLower); idx != -1 {
			title = title[:idx]
			titleLower = titleLower[:idx]
		}
	}

	// Remove quality tags
	for _, tag := range qualityPatterns {
		tagLower := strings.ToLower(tag)
		if idx := strings.Index(titleLower, tagLower); idx != -1 {
			title = title[:idx]
			titleLower = titleLower[:idx]
		}
	}

	// Remove codec and other quality tags
	for _, tag := range qualityTags {
		tagLower := strings.ToLower(tag)
		if idx := strings.Index(titleLower, tagLower); idx != -1 {
			title = title[:idx]
			titleLower = titleLower[:idx]
		}
	}

	// Remove common brackets and their content
	title = regexp.MustCompile(`\[.*?\]`).ReplaceAllString(title, "")
	title = regexp.MustCompile(`\{.*?\}`).ReplaceAllString(title, "")

	// Clean up whitespace
	title = strings.TrimSpace(title)
	title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")

	// Remove trailing dashes or dots
	title = strings.TrimRight(title, " -.")

	return title
}

// extractResolution extracts the video resolution from the filename
func extractResolution(filename string) string {
	filenameLower := strings.ToLower(filename)

	for tag, resolution := range resolutionPatterns {
		tagLower := strings.ToLower(tag)
		if strings.Contains(filenameLower, tagLower) {
			return resolution
		}
	}

	return ""
}

// extractQuality extracts the source quality from the filename
func extractQuality(filename string) string {
	filenameLower := strings.ToLower(filename)

	for _, quality := range qualityPatterns {
		qualityLower := strings.ToLower(quality)
		if strings.Contains(filenameLower, qualityLower) {
			return quality
		}
	}

	return ""
}

// isValidMovieYear checks if a year is within a reasonable range for movies
func isValidMovieYear(year int) bool {
	return year >= 1880 && year <= 2100
}

// extractImdbID extracts the IMDb ID from the filename
// Looks for patterns like [imdbid-tt1234567] or [tt1234567]
func extractImdbID(filename string) string {
	// Pattern: [imdbid-tt1234567] or [tt1234567]
	imdbPattern := regexp.MustCompile(`\[(?:imdbid-)?([Tt][Tt]\d{7,8})\]`)
	matches := imdbPattern.FindStringSubmatch(filename)
	if len(matches) >= 2 {
		return strings.ToLower(matches[1])
	}
	return ""
}

// extractEdition extracts the movie edition from the filename
// Looks for edition markers like "Extended Cut", "Director's Cut", "Remastered", "Proper", etc.
func extractEdition(filename string) string {
	// Common edition patterns
	editionPatterns := []string{
		"Extended Cut",
		"Director's Cut",
		"Directors Cut",
		"Theatrical Cut",
		"Final Cut",
		"Remastered",
		"Restored",
		"Unrated",
		"Uncut",
		"Special Edition",
		"Limited Edition",
		"Criterion Collection",
		"Proper",
		"Hybrid",
	}

	filenameLower := strings.ToLower(filename)
	for _, edition := range editionPatterns {
		editionLower := strings.ToLower(edition)
		if strings.Contains(filenameLower, editionLower) {
			return edition
		}
	}

	return ""
}
