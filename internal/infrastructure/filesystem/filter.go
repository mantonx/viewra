package filesystem

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/viewra/viewra/internal/domain/scanner"
)

// NewFilter creates a new Filter with default settings
func NewFilter() *Filter {
	return &Filter{
		skipHidden: true,
	}
}

// ShouldProcess returns true if the file should be processed as media
func (f *Filter) ShouldProcess(path string, info os.FileInfo) bool {
	// Skip directories
	if info.IsDir() {
		return false
	}

	// Skip hidden files if configured
	if f.skipHidden && f.isHidden(path) {
		return false
	}

	fileName := strings.ToLower(filepath.Base(path))

	// Skip artwork files (poster, banner, fanart, etc.)
	if f.isArtworkFile(fileName) {
		return false
	}

	// Skip metadata files (.nfo, .xml, subtitles)
	if f.isMetadataFile(fileName) {
		return false
	}

	// Skip system files
	if f.isSystemFile(fileName) {
		return false
	}

	// Check if it's a media file
	return f.IsMediaFile(path)
}

// IsMediaFile returns true if the file extension indicates media content
func (f *Filter) IsMediaFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return f.GetMediaType(ext) != scanner.MediaTypeUnknown
}

// GetMediaType determines the media type based on file extension
func (f *Filter) GetMediaType(extension string) scanner.MediaType {
	// Video extensions
	videoExts := []string{
		".mkv", ".mp4", ".avi", ".mov", ".wmv",
		".flv", ".webm", ".m4v", ".3gp", ".ts",
		".mpg", ".mpeg", ".rm", ".rmvb", ".asf",
		".divx", ".vob", ".ogv", ".mts", ".m2ts",
	}

	// Audio extensions
	audioExts := []string{
		".mp3", ".flac", ".wav", ".m4a", ".aac",
		".ogg", ".wma", ".opus", ".aiff", ".ape",
		".wv", ".alac", ".m4p", ".dsf", ".dff",
	}

	if hasExtension(extension, videoExts) {
		// Default to movie for video files (will be refined later with filename parsing)
		return scanner.MediaTypeMovie
	}

	if hasExtension(extension, audioExts) {
		return scanner.MediaTypeTrack
	}

	return scanner.MediaTypeUnknown
}

// isArtworkFile checks if the file is artwork (poster, banner, fanart, etc.)
func (f *Filter) isArtworkFile(fileName string) bool {
	artworkPatterns := []string{
		"poster.", "banner.", "thumb.", "thumbnail.", "cover.", "artwork.",
		"fanart.", "background.", "backdrop.", "clearlogo.", "clearart.",
		"landscape.", "disc.", "folder.", "albumart.", "front.", "back.",
		"-poster.", "-banner.", "-thumb.", "-thumbnail.", "-cover.",
		"-fanart.", "-background.", "-backdrop.",
		"season01-poster.", "season02-poster.", "season-poster.",
		"show-poster.", "show-banner.",
	}

	if containsAny(fileName, artworkPatterns) {
		return true
	}

	// Check image extensions (might be artwork)
	imageExts := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".webp", ".svg"}
	return hasExtension(fileName, imageExts)
}

// isMetadataFile checks if the file is metadata (subtitles, .nfo, etc.)
func (f *Filter) isMetadataFile(fileName string) bool {
	metadataExts := []string{
		".nfo", ".xml", ".srt", ".vtt", ".ass", ".ssa", ".sub", ".idx",
		".txt", ".info", ".cue", ".m3u", ".m3u8", ".pls",
	}
	return hasExtension(fileName, metadataExts)
}

// isSystemFile checks if the file is a system file
func (f *Filter) isSystemFile(fileName string) bool {
	fileName = strings.ToLower(fileName)
	systemPatterns := []string{
		".ds_store", "thumbs.db", ".tmp", ".temp", ".bak",
		".backup", ".old", ".orig", "desktop.ini", ".directory",
	}
	return containsAny(fileName, systemPatterns)
}

// isHidden checks if the file or directory is hidden
func (f *Filter) isHidden(path string) bool {
	base := filepath.Base(path)
	return base != "" && base[0] == '.'
}
