package parsers

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/viewra/viewra/internal/domain/scanner"
)

// ParseMusic extracts music metadata from an audio file.
// It first attempts to use the metadata extractor (if provided) to read ID3 tags,
// then falls back to filename parsing if tags are missing or extractor is nil.
func ParseMusic(path string, metadataExtractor scanner.MusicMetadataExtractor) (*scanner.MusicInfo, error) {
	info := &scanner.MusicInfo{}

	// Try to extract from ID3 tags first if extractor is available
	if metadataExtractor != nil {
		extractedInfo, err := metadataExtractor.ExtractMetadata(path)
		if err == nil && extractedInfo != nil && extractedInfo.Title != "" {
			// Successfully extracted from ID3 tags
			return extractedInfo, nil
		}
		// If extraction failed or returned incomplete data, fall through to filename parsing
	}

	// Fallback to filename parsing if ID3 tags are missing or incomplete
	err := extractFromFilename(path, info)
	if err != nil {
		return nil, fmt.Errorf("failed to extract music metadata from %s: %w", path, err)
	}

	return info, nil
}

// extractFromFilename parses music metadata from the filename as a fallback
// This handles cases where ID3 tags are missing or incomplete
func extractFromFilename(path string, info *scanner.MusicInfo) error {
	filename := getFilenameWithoutExt(path)
	dir := filepath.Dir(path)

	// Try to extract from common patterns:
	// 1. "Artist - Album - TrackNum - Title.mp3" (most common in our files)
	// 2. "Artist - Title.mp3"
	// 3. "01 - Artist - Title.mp3"
	// 4. "01 Title.mp3"

	// Pattern 1: Artist - Album - TrackNum - Title
	artistAlbumTrackPattern := `^(.+?)[\s._]*-[\s._]*(.+?)[\s._]*-[\s._]*(\d{1,3})[\s._]*-[\s._]*(.+)$`
	if matches := extractPattern(filename, artistAlbumTrackPattern); len(matches) == 5 {
		info.Artist = cleanString(matches[1])
		info.Album = cleanString(matches[2])
		trackNum, _ := strconv.Atoi(matches[3])
		info.TrackNumber = trackNum
		info.Title = cleanString(matches[4])

		// If album is empty from filename, try directory
		if info.Album == "" {
			info.Album = extractAlbumFromPath(dir)
		}

		return nil
	}

	// Pattern 2: TrackNum - Title (extract from beginning)
	trackPattern := `^(\d{1,3})[\s._-]+(.+)$`
	if matches := extractPattern(filename, trackPattern); len(matches) == 3 {
		trackNum, _ := strconv.Atoi(matches[1])
		info.TrackNumber = trackNum
		filename = matches[2]
	}

	// Pattern 3: Artist - Title
	artistTitlePattern := `^(.+?)[\s._-]+[\-–—]+[\s._-]+(.+)$`
	if matches := extractPattern(filename, artistTitlePattern); len(matches) == 3 {
		info.Artist = cleanString(matches[1])
		info.Title = cleanString(matches[2])
	} else {
		// Just use the filename as title
		info.Title = cleanString(filename)
	}

	// Try to extract album from directory name
	if info.Album == "" {
		info.Album = extractAlbumFromPath(dir)
	}

	// Try to extract artist from parent directories if not found
	if info.Artist == "" {
		info.Artist = extractArtistFromPath(dir)
	}

	if info.Title == "" {
		return fmt.Errorf("unable to extract title from filename")
	}

	return nil
}

// extractAlbumFromPath tries to extract album name from the directory structure
// Typical structure: /Music/Artist/Album (Year)[Format]/Track.mp3
// Examples: "Illinois (2005)[FLAC]", "Javelin (2023)[FLAC 24bit]", "Michigan (2003)[MP3-320]"
func extractAlbumFromPath(dir string) string {
	// Get the immediate parent directory as album
	album := filepath.Base(dir)

	// Skip multi-disc subdirectories (e.g., "CD 01", "CD 02", "Digital Media 01")
	if regexp.MustCompile(`(?i)^(CD|Disc|Digital Media)\s*\d+$`).MatchString(album) {
		// Go up one more level to get the actual album directory
		dir = filepath.Dir(dir)
		album = filepath.Base(dir)
	}

	// Remove format tags in brackets: [FLAC], [FLAC 24bit], [MP3-320], etc.
	album = regexp.MustCompile(`\[.*?\]`).ReplaceAllString(album, "")

	// Remove year in parentheses if present: "Album Name (2020)"
	album = regexp.MustCompile(`\(\d{4}\)`).ReplaceAllString(album, "")

	// Clean up common artifacts
	album = cleanString(album)

	return strings.TrimSpace(album)
}

// extractArtistFromPath tries to extract artist name from the directory structure
// Typical structure: /Music/Artist/Album/Track.mp3 or /Music/Artist/Album/CD 01/Track.mp3
func extractArtistFromPath(dir string) string {
	parts := strings.Split(dir, string(filepath.Separator))

	// Check if we're in a multi-disc subdirectory
	if len(parts) >= 1 {
		lastPart := parts[len(parts)-1]
		if regexp.MustCompile(`(?i)^(CD|Disc|Digital Media)\s*\d+$`).MatchString(lastPart) {
			// We're in a multi-disc subdirectory, go up one more level
			// Structure: /Artist/Album/CD 01/
			if len(parts) >= 3 {
				artist := parts[len(parts)-3]
				return cleanString(artist)
			}
		}
	}

	// Standard structure: /Artist/Album/
	if len(parts) >= 2 {
		// Assume the second-to-last directory is the artist
		artist := parts[len(parts)-2]
		return cleanString(artist)
	}

	return ""
}

// getFilenameWithoutExt returns the filename without the extension
func getFilenameWithoutExt(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

// extractPattern is a helper to extract regex pattern matches
func extractPattern(input, pattern string) []string {
	re := regexp.MustCompile(pattern)
	return re.FindStringSubmatch(input)
}

// cleanString removes common artifacts and normalizes strings
func cleanString(s string) string {
	// Replace dots and underscores with spaces
	s = strings.ReplaceAll(s, ".", " ")
	s = strings.ReplaceAll(s, "_", " ")

	// Remove common quality tags
	sLower := strings.ToLower(s)
	musicQualityTags := []string{
		"320kbps", "256kbps", "192kbps", "128kbps",
		"FLAC", "MP3", "AAC", "OGG", "WAV", "ALAC",
		"V0", "V2", "APE", "WMA",
	}

	for _, tag := range musicQualityTags {
		tagLower := strings.ToLower(tag)
		if idx := strings.Index(sLower, tagLower); idx != -1 {
			s = s[:idx]
			sLower = sLower[:idx]
		}
	}

	// Remove brackets with quality tags only, not all brackets
	// Only remove [FLAC], [MP3-320], etc., but keep parentheses for titles like "(To Have and to Hold)"
	s = regexp.MustCompile(`\[(?i)(FLAC|MP3|AAC|OGG|WAV|ALAC|V0|V2|APE|WMA)[^\]]*\]`).ReplaceAllString(s, "")

	// Clean up whitespace
	s = strings.TrimSpace(s)
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")

	return s
}
