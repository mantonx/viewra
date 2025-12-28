package monitor

import (
	"path/filepath"
	"regexp"
	"strings"
)

// FileChangeType represents the type of file that changed.
type FileChangeType string

const (
	// FileChangeMedia indicates a media file changed (.mkv, .mp4, .flac, etc.)
	FileChangeMedia FileChangeType = "media"

	// FileChangeNFO indicates an NFO metadata file changed
	FileChangeNFO FileChangeType = "nfo"

	// FileChangeLocalImage indicates a local artwork file changed (poster.jpg, etc.)
	FileChangeLocalImage FileChangeType = "local_image"

	// FileChangeUnknown indicates an unrecognized file type
	FileChangeUnknown FileChangeType = "unknown"
)

// Video file extensions
var videoExtensions = map[string]bool{
	".mkv":  true,
	".mp4":  true,
	".avi":  true,
	".m4v":  true,
	".mov":  true,
	".wmv":  true,
	".flv":  true,
	".webm": true,
	".ts":   true,
	".m2ts": true,
	".vob":  true,
	".mpg":  true,
	".mpeg": true,
	".3gp":  true,
	".ogv":  true,
	".divx": true,
}

// Audio file extensions
var audioExtensions = map[string]bool{
	".mp3":  true,
	".flac": true,
	".m4a":  true,
	".aac":  true,
	".ogg":  true,
	".opus": true,
	".wav":  true,
	".wma":  true,
	".alac": true,
	".aiff": true,
	".ape":  true,
	".dsf":  true,
	".dff":  true,
}

// Image file extensions
var imageExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".webp": true,
	".bmp":  true,
	".tiff": true,
}

// Patterns for local artwork files
var localImagePatterns = []string{
	// Movie/TV posters and art
	"poster",
	"folder",
	"fanart",
	"backdrop",
	"clearlogo",
	"logo",
	"landscape",
	"banner",
	"thumb",
	"cover",
	"album",
	"front",
	"back",
	"discart",
	"disc",
	"cd",
	"artist",
	// TV-specific patterns
	"tvshow-poster",
	"tvshow-fanart",
	"tvshow-banner",
	"tvshow-clearlogo",
	"season-specials",
}

// Season pattern for TV season artwork (season01-poster.jpg, etc.)
var seasonPattern = regexp.MustCompile(`(?i)^season\d+-`)

// Episode thumb pattern (S01E01-thumb.jpg, etc.)
var episodeThumbPattern = regexp.MustCompile(`(?i)s\d+e\d+.*-thumb`)

// Directories containing artwork
var artworkDirs = map[string]bool{
	"actors":      true,
	"extrathumbs": true,
	"extrafanart": true,
	".actors":     true,
}

// ClassifyFile determines the type of file based on its path.
// This is used to determine which enrichment stage should be triggered.
func ClassifyFile(filePath string) FileChangeType {
	ext := strings.ToLower(filepath.Ext(filePath))
	baseName := strings.ToLower(filepath.Base(filePath))
	baseNoExt := strings.TrimSuffix(baseName, ext)

	// Check for NFO files first
	if IsNFOFile(filePath) {
		return FileChangeNFO
	}

	// Check for local image files
	if IsLocalImageFile(filePath) {
		return FileChangeLocalImage
	}

	// Check for media files
	if IsMediaFile(filePath) {
		return FileChangeMedia
	}

	// Unknown file type - ignore
	_ = baseNoExt // Used in future pattern matching
	return FileChangeUnknown
}

// IsNFOFile checks if the file is an NFO metadata file.
func IsNFOFile(filePath string) bool {
	lower := strings.ToLower(filePath)

	// Standard .nfo extension
	if strings.HasSuffix(lower, ".nfo") {
		return true
	}

	// Alternate -nfo.xml format (some tools use this)
	if strings.HasSuffix(lower, "-nfo.xml") {
		return true
	}

	return false
}

// IsLocalImageFile checks if the file is a local artwork file.
// These files trigger the local-images enrichment stage.
func IsLocalImageFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))

	// Must be an image file
	if !imageExtensions[ext] {
		return false
	}

	baseName := strings.ToLower(filepath.Base(filePath))
	baseNoExt := strings.TrimSuffix(baseName, ext)

	// Check for standard artwork patterns
	for _, pattern := range localImagePatterns {
		if strings.Contains(baseNoExt, pattern) {
			return true
		}
	}

	// Check for season artwork (season01-poster.jpg, etc.)
	if seasonPattern.MatchString(baseName) {
		return true
	}

	// Check for episode thumbnails (S01E01-thumb.jpg, etc.)
	if episodeThumbPattern.MatchString(baseName) {
		return true
	}

	// Check if file is in an artwork directory
	dir := strings.ToLower(filepath.Base(filepath.Dir(filePath)))
	if artworkDirs[dir] {
		return true
	}

	return false
}

// IsMediaFile checks if the file is a media file (video or audio).
func IsMediaFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return videoExtensions[ext] || audioExtensions[ext]
}

// IsVideoFile checks if the file is a video file.
func IsVideoFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return videoExtensions[ext]
}

// IsAudioFile checks if the file is an audio file.
func IsAudioFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return audioExtensions[ext]
}

// GetEnrichmentStage returns the enrichment stage to trigger for a file change type.
// Returns empty string if the change type doesn't map to a specific stage.
func GetEnrichmentStage(changeType FileChangeType) string {
	switch changeType {
	case FileChangeNFO:
		return "nfo"
	case FileChangeLocalImage:
		return "local-images"
	case FileChangeMedia:
		// Media files trigger the full pipeline from first stage
		return "" // Empty means use EnqueueFirstStage
	default:
		return ""
	}
}

// ShouldIgnoreFile checks if a file should be ignored by the monitor.
// This includes temporary files, hidden files, system files, etc.
func ShouldIgnoreFile(filePath string) bool {
	baseName := filepath.Base(filePath)

	// Ignore hidden files (Unix style)
	if strings.HasPrefix(baseName, ".") {
		return true
	}

	// Ignore common temporary file patterns
	tempPatterns := []string{
		".tmp",
		".temp",
		".partial",
		".part",
		".crdownload", // Chrome download
		".download",
		"~",           // Backup files
		".swp",        // Vim swap
		".swo",        // Vim swap
		".DS_Store",   // macOS
		"Thumbs.db",   // Windows
		"desktop.ini", // Windows
	}

	lowerBase := strings.ToLower(baseName)
	for _, pattern := range tempPatterns {
		if strings.HasSuffix(lowerBase, pattern) {
			return true
		}
	}

	// Ignore sample/trailer files
	samplePatterns := []string{
		"-sample.",
		".sample.",
		"-trailer.",
		".trailer.",
	}
	lowerPath := strings.ToLower(filePath)
	for _, pattern := range samplePatterns {
		if strings.Contains(lowerPath, pattern) {
			return true
		}
	}

	return false
}
