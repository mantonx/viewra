package filter

import (
	"path/filepath"
	"strings"
)

// Video file extensions
var videoExtensions = []string{
	".mkv", ".mp4", ".avi", ".mov", ".wmv",
	".flv", ".webm", ".m4v", ".3gp", ".ts",
	".mpg", ".mpeg", ".rm", ".rmvb", ".asf",
	".divx", ".vob", ".ogv", ".mts", ".m2ts",
}

// Audio file extensions
var audioExtensions = []string{
	".mp3", ".flac", ".wav", ".m4a", ".aac",
	".ogg", ".wma", ".opus", ".aiff", ".ape",
	".wv", ".alac", ".m4p", ".dsf", ".dff",
}

// Image file extensions (for artwork detection)
var imageExtensions = []string{
	".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".webp", ".svg",
}

// Metadata file extensions
var metadataExtensions = []string{
	".nfo", ".xml", ".srt", ".vtt", ".ass", ".ssa", ".sub", ".idx",
	".txt", ".info", ".cue", ".m3u", ".m3u8", ".pls",
}

// Artwork filename patterns
var artworkPatterns = []string{
	"poster.", "banner.", "thumb.", "thumbnail.", "cover.", "artwork.",
	"fanart.", "background.", "backdrop.", "clearlogo.", "clearart.",
	"landscape.", "disc.", "folder.", "albumart.", "front.", "back.",
	"-poster.", "-banner.", "-thumb.", "-thumbnail.", "-cover.",
	"-fanart.", "-background.", "-backdrop.",
	"season01-poster.", "season02-poster.", "season-poster.",
	"show-poster.", "show-banner.",
}

// System filename patterns
var systemPatterns = []string{
	".ds_store", "thumbs.db", ".tmp", ".temp", ".bak",
	".backup", ".old", ".orig", "desktop.ini", ".directory",
}

// normalizeExtension returns the lowercase extension with leading dot
func normalizeExtension(path string) string {
	return strings.ToLower(filepath.Ext(path))
}

// hasExtension checks if a file path has any of the given extensions
func hasExtension(path string, extensions []string) bool {
	ext := normalizeExtension(path)
	for _, validExt := range extensions {
		if ext == validExt {
			return true
		}
	}
	return false
}

// containsAny checks if a string contains any of the given substrings
func containsAny(s string, substrings []string) bool {
	for _, substr := range substrings {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}
