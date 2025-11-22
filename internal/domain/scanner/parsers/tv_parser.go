package parsers

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mantonx/viewra/internal/domain/scanner"
)

// TV show filename patterns in order of preference (most specific to least specific)
var tvPatterns = []*regexp.Regexp{
	// S01E01 format with optional episode title
	// Examples: "Show.Name.S01E01.mkv", "Show Name - S01E01 - Episode Title.mp4"
	regexp.MustCompile(`(?i)^(.+?)[\s._-]+[Ss](\d{1,2})[Ee](\d{1,3})(?:[\s._-]*(?:[Ee](\d{1,3}))?)?(?:[\s._-]+(.+?))?(?:\.\w+)?$`),

	// Season X Episode Y format
	// Examples: "Show.Name.Season.1.Episode.01.mkv"
	regexp.MustCompile(`(?i)^(.+?)[\s._-]+Season[\s._-]*(\d{1,2})[\s._-]+Episode[\s._-]*(\d{1,3})(?:[\s._-]+(.+?))?(?:\.\w+)?$`),

	// 1x01 format
	// Examples: "Show Name 1x01.mkv", "Show.Name.1x01.Episode.Title.mp4"
	regexp.MustCompile(`(?i)^(.+?)[\s._-]+(\d{1,2})x(\d{1,3})(?:[\s._-]+(.+?))?(?:\.\w+)?$`),

	// Episode number only (less reliable, requires directory context)
	// Examples: "Show Name/Season 1/01 - Episode Title.mkv"
	regexp.MustCompile(`(?i)^(.+?)[\s._-]+(\d{1,3})(?:[\s._-]+(.+?))?(?:\.\w+)?$`),
}

// Season directory patterns to extract season number from directory path
var seasonDirPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)Season[\s._-]*(\d{1,2})`),
	regexp.MustCompile(`(?i)[Ss](\d{1,2})`),
	regexp.MustCompile(`(?i)^(\d{1,2})$`), // Just a number
}

// Quality/release artifacts to strip from show names and episode titles
var qualityTags = []string{
	"1080p", "720p", "480p", "2160p", "4K", "UHD",
	"BluRay", "BRRip", "BDRip", "WEBRip", "WEB-DL", "HDTV", "DVDRip",
	"x264", "x265", "h264", "h265", "HEVC", "XviD", "DivX",
	"AAC", "AC3", "DTS", "DD5.1", "DD2.0", "TrueHD", "FLAC",
	"PROPER", "REPACK", "EXTENDED", "UNRATED", "DC",
	"10bit", "8bit",
}

// ParseTVEpisode extracts TV show metadata from a filename or file path.
// It supports multiple naming patterns and can extract season information from directory structure.
func ParseTVEpisode(path string) (*scanner.TVEpisodeInfo, error) {
	// Get the filename without extension
	filename := filepath.Base(path)
	filenameNoExt := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Try each pattern until one matches
	for _, pattern := range tvPatterns {
		matches := pattern.FindStringSubmatch(filenameNoExt)
		if matches == nil {
			continue
		}

		// Extract components based on which pattern matched
		info, err := extractTVInfoFromMatches(matches, pattern)
		if err != nil {
			continue
		}

		// If we didn't extract a season from the filename, try the directory path
		if info.Season == 0 {
			season := extractSeasonFromPath(path)
			if season > 0 {
				info.Season = season
			}
		}

		// If show name is empty or looks like just a number, try to extract from path
		if info.ShowName == "" || regexp.MustCompile(`^\d+$`).MatchString(info.ShowName) {
			showFromPath := extractShowNameFromPath(path)
			if showFromPath != "" {
				info.ShowName = showFromPath
			}
		}

		// Clean up the show name and episode title
		info.ShowName = cleanShowName(info.ShowName)
		if info.EpisodeTitle != "" {
			info.EpisodeTitle = cleanEpisodeTitle(info.EpisodeTitle)
		}

		// Validate the extracted info
		if info.ShowName == "" || info.Episode == 0 {
			continue
		}

		return info, nil
	}

	return nil, fmt.Errorf("unable to parse TV episode information from: %s", filename)
}

// extractTVInfoFromMatches converts regex matches to TVEpisodeInfo
func extractTVInfoFromMatches(matches []string, pattern *regexp.Regexp) (*scanner.TVEpisodeInfo, error) {
	info := &scanner.TVEpisodeInfo{}

	// Different patterns have different group counts and meanings
	switch len(matches) {
	case 6: // S01E01 pattern with optional end episode and title
		info.ShowName = matches[1]
		season, _ := strconv.Atoi(matches[2])
		episode, _ := strconv.Atoi(matches[3])
		info.Season = season
		info.Episode = episode

		// Check for multi-episode format (S01E01E02)
		if matches[4] != "" {
			endEp, _ := strconv.Atoi(matches[4])
			info.EpisodeEnd = endEp
		}

		// Extract episode title if present
		if matches[5] != "" {
			info.EpisodeTitle = matches[5]
		}

	case 5: // Season X Episode Y or 1x01 format
		info.ShowName = matches[1]
		season, _ := strconv.Atoi(matches[2])
		episode, _ := strconv.Atoi(matches[3])
		info.Season = season
		info.Episode = episode

		// Extract episode title if present
		if matches[4] != "" {
			info.EpisodeTitle = matches[4]
		}

	case 4: // Episode number only (need season from directory)
		info.ShowName = matches[1]
		episode, _ := strconv.Atoi(matches[2])
		info.Episode = episode

		// Extract episode title if present
		if matches[3] != "" {
			info.EpisodeTitle = matches[3]
		}

	default:
		return nil, fmt.Errorf("unexpected match count: %d", len(matches))
	}

	return info, nil
}

// extractSeasonFromPath attempts to extract season number from the directory path
func extractSeasonFromPath(path string) int {
	dir := filepath.Dir(path)

	// Split the path and check each directory component
	parts := strings.Split(dir, string(filepath.Separator))

	// Check from the end backwards (most specific to least specific)
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]

		for _, pattern := range seasonDirPatterns {
			matches := pattern.FindStringSubmatch(part)
			if len(matches) >= 2 {
				season, err := strconv.Atoi(matches[1])
				if err == nil && season >= 0 && season <= 99 {
					return season
				}
			}
		}
	}

	return 0
}

// extractShowNameFromPath attempts to extract the show name from the directory path
// Typical structure: /TV Shows/Show Name/Season 1/Episode.mkv
func extractShowNameFromPath(path string) string {
	dir := filepath.Dir(path)
	parts := strings.Split(dir, string(filepath.Separator))

	// Look for the show name (typically 1-2 directories up from the file)
	// Skip the immediate parent if it looks like a season directory
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]

		// Skip if this looks like a season directory
		if regexp.MustCompile(`(?i)season|^s?\d+$`).MatchString(part) {
			continue
		}

		// Skip common library root names
		if regexp.MustCompile(`(?i)^(tv shows?|series|episodes?|television)$`).MatchString(part) {
			continue
		}

		// This should be the show name
		if part != "" && part != "." {
			return part
		}
	}

	return ""
}

// cleanShowName removes quality tags and normalizes the show name
func cleanShowName(name string) string {
	// Replace underscores with spaces
	name = strings.ReplaceAll(name, "_", " ")

	// Replace dots with spaces EXCEPT when they're part of abbreviations
	// An abbreviation is: single letter + dot (e.g., "P.D.")
	// Pattern matches sequences like "P.D." or just "D." at the end
	abbrPattern := regexp.MustCompile(`\b([A-Z])\.`)
	abbreviations := abbrPattern.FindAllString(name, -1)

	// Temporarily replace abbreviations with placeholders
	for i, abbr := range abbreviations {
		placeholder := fmt.Sprintf("__ABBR%d__", i)
		name = strings.Replace(name, abbr, placeholder, 1)
	}

	// Now replace all other dots with spaces
	name = strings.ReplaceAll(name, ".", " ")

	// Restore abbreviations
	for i, abbr := range abbreviations {
		placeholder := fmt.Sprintf("__ABBR%d__", i)
		name = strings.Replace(name, placeholder, abbr, 1)
	}

	// Remove quality tags
	nameLower := strings.ToLower(name)
	for _, tag := range qualityTags {
		tagLower := strings.ToLower(tag)
		nameLower = strings.ReplaceAll(nameLower, tagLower, "")
	}

	// Rebuild with proper casing by taking words that still exist in original
	words := strings.Fields(nameLower)
	origWords := strings.Fields(name)

	// Create a map of lowercase to original for case preservation
	wordMap := make(map[string]string)
	for _, w := range origWords {
		wordMap[strings.ToLower(w)] = w
	}

	result := make([]string, 0, len(words))
	for _, w := range words {
		if original, ok := wordMap[w]; ok && len(w) > 1 {
			result = append(result, original)
		}
	}

	name = strings.Join(result, " ")

	// Remove common brackets and extra whitespace
	name = strings.TrimSpace(name)
	name = regexp.MustCompile(`\s+`).ReplaceAllString(name, " ")

	// Remove year in parentheses if present (e.g., "Show Name (2020)")
	name = regexp.MustCompile(`\s*\(\d{4}\)\s*$`).ReplaceAllString(name, "")

	name = strings.TrimSpace(name)

	// Ensure trailing dots on abbreviations are preserved
	// (they might get trimmed, so we need to check the original)
	return name
}

// cleanEpisodeTitle removes quality tags and normalizes the episode title
func cleanEpisodeTitle(title string) string {
	// Replace dots and underscores with spaces
	title = strings.ReplaceAll(title, ".", " ")
	title = strings.ReplaceAll(title, "_", " ")

	// Remove common brackets content (quality tags, codecs, etc.)
	title = regexp.MustCompile(`\[.*?\]`).ReplaceAllString(title, "")

	// Remove release group tags that typically appear at the end after a dash
	// Pattern: " - ReleaseGroup" or "-ReleaseGroup" at the end
	// But be careful not to remove valid title words
	// Look for patterns like "-ETHEL", "-iVy", "-PSA" (all caps or mixed case single words)
	title = regexp.MustCompile(`\s*-\s*[A-Za-z][A-Za-z0-9]*\s*$`).ReplaceAllString(title, "")

	// Clean up whitespace
	title = strings.TrimSpace(title)
	title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")

	// Only remove quality tags that are very specific and unlikely to be in titles
	// Check for these patterns at the end of the title or as standalone sections
	qualityPattern := `(?i)\b(1080p|720p|480p|2160p|4K|UHD|BluRay|BRRip|BDRip|WEBRip|WEB-DL|HDTV|DVDRip|x264|x265|h264|h265|HEVC|XviD)\b.*$`
	title = regexp.MustCompile(qualityPattern).ReplaceAllString(title, "")

	// Final cleanup
	title = strings.TrimSpace(title)

	return title
}
