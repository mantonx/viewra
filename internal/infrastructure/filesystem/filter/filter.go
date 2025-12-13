package filter

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mantonx/viewra/internal/domain/scanner"
)

// Filter implements scanner.FileFilter with smart file detection
type Filter struct {
	skipHidden bool
}

// New creates a new Filter with default settings
func New() *Filter {
	return &Filter{
		skipHidden: true,
	}
}

// ShouldProcess returns true if the file should be processed as media
func (f *Filter) ShouldProcess(path string, info os.FileInfo) bool {
	if info.IsDir() {
		return false
	}

	if f.skipHidden && f.isHidden(path) {
		return false
	}

	fileName := strings.ToLower(filepath.Base(path))

	if f.isArtworkFile(fileName) {
		return false
	}

	if f.isMetadataFile(fileName) {
		return false
	}

	if f.isSystemFile(fileName) {
		return false
	}

	return f.IsMediaFile(path)
}

// IsMediaFile returns true if the file extension indicates media content
func (f *Filter) IsMediaFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return f.GetMediaType(ext) != scanner.MediaTypeUnknown
}

// GetMediaType determines the media type based on file extension
func (f *Filter) GetMediaType(extension string) scanner.MediaType {
	if hasExtension(extension, videoExtensions) {
		return scanner.MediaTypeMovie
	}

	if hasExtension(extension, audioExtensions) {
		return scanner.MediaTypeTrack
	}

	return scanner.MediaTypeUnknown
}

// isArtworkFile checks if the file is artwork (poster, banner, fanart, etc.)
func (f *Filter) isArtworkFile(fileName string) bool {
	if containsAny(fileName, artworkPatterns) {
		return true
	}
	return hasExtension(fileName, imageExtensions)
}

// isMetadataFile checks if the file is metadata (subtitles, .nfo, etc.)
func (f *Filter) isMetadataFile(fileName string) bool {
	return hasExtension(fileName, metadataExtensions)
}

// isSystemFile checks if the file is a system file
func (f *Filter) isSystemFile(fileName string) bool {
	fileName = strings.ToLower(fileName)
	return containsAny(fileName, systemPatterns)
}

// isHidden checks if the file or directory is hidden
func (f *Filter) isHidden(path string) bool {
	base := filepath.Base(path)
	return base != "" && base[0] == '.'
}
