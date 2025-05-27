// Package tagreader provides basic metadata reading capabilities for audio files.
// This is a placeholder implementation - in production you'd use libraries like taglib-go.
package tagreader

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Metadata represents basic audio file metadata
type Metadata struct {
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	Album       string `json:"album"`
	AlbumArtist string `json:"album_artist"`
	Year        int    `json:"year"`
	Track       int    `json:"track"`
	Disc        int    `json:"disc"`
	Genre       string `json:"genre"`
	Duration    int    `json:"duration"` // in seconds
}

// Reader provides metadata reading functionality
type Reader struct{}

// NewReader creates a new metadata reader
func NewReader() *Reader {
	return &Reader{}
}

// ReadMetadata reads metadata from an audio file
// This is a placeholder implementation that would normally use a proper audio library
func (r *Reader) ReadMetadata(filePath string) (*Metadata, error) {
	// Check if file is supported
	if !r.IsSupported(filePath) {
		return nil, fmt.Errorf("unsupported file format: %s", filepath.Ext(filePath))
	}

	// In a real implementation, this would use a library like taglib-go
	// to read actual metadata from the file. For now, we return empty metadata.
	metadata := &Metadata{
		Title:       "",
		Artist:      "",
		Album:       "",
		AlbumArtist: "",
		Year:        0,
		Track:       0,
		Disc:        0,
		Genre:       "",
		Duration:    0,
	}

	return metadata, nil
}

// IsSupported checks if the file format is supported for metadata reading
func (r *Reader) IsSupported(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	supportedFormats := []string{
		".mp3", ".flac", ".m4a", ".aac", ".ogg",
		".wav", ".wma", ".opus", ".ape", ".wv",
	}

	for _, format := range supportedFormats {
		if ext == format {
			return true
		}
	}
	return false
}

// GetSupportedFormats returns a list of supported audio formats
func (r *Reader) GetSupportedFormats() []string {
	return []string{
		".mp3", ".flac", ".m4a", ".aac", ".ogg",
		".wav", ".wma", ".opus", ".ape", ".wv",
	}
}

// ExtractBasicInfo extracts basic information from filename if metadata is not available
func (r *Reader) ExtractBasicInfo(filePath string) *Metadata {
	filename := filepath.Base(filePath)
	nameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Try to parse common filename patterns
	// Pattern: "Artist - Title"
	if parts := strings.Split(nameWithoutExt, " - "); len(parts) >= 2 {
		return &Metadata{
			Artist: strings.TrimSpace(parts[0]),
			Title:  strings.TrimSpace(strings.Join(parts[1:], " - ")),
		}
	}

	// Pattern: "Track Number Title"
	if parts := strings.Fields(nameWithoutExt); len(parts) >= 2 {
		// Check if first part is a number
		if track := parseTrackNumber(parts[0]); track > 0 {
			return &Metadata{
				Track: track,
				Title: strings.Join(parts[1:], " "),
			}
		}
	}

	// Fallback: use filename as title
	return &Metadata{
		Title: nameWithoutExt,
	}
}

// parseTrackNumber attempts to parse a track number from a string
func parseTrackNumber(s string) int {
	// Remove common prefixes
	s = strings.TrimLeft(s, "0")
	
	// Simple number parsing
	var track int
	if _, err := fmt.Sscanf(s, "%d", &track); err == nil {
		if track > 0 && track <= 999 {
			return track
		}
	}
	
	return 0
} 