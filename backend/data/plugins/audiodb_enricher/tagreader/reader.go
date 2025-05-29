// Package tagreader provides basic metadata reading capabilities for audio files.
// This is a placeholder implementation - in production you'd use libraries like taglib-go.
package tagreader

import (
	"fmt"
	"path/filepath"
	"strconv"
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
	FilePath    string `json:"file_path"`
	FileSize    int64  `json:"file_size"`
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
	// to read actual metadata from the file. For now, we extract what we can
	// from the filename and directory structure.
	metadata := r.ExtractBasicInfo(filePath)
	metadata.FilePath = filePath

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

// ExtractBasicInfo extracts basic information from filename and directory structure
func (r *Reader) ExtractBasicInfo(filePath string) *Metadata {
	filename := filepath.Base(filePath)
	nameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))
	dir := filepath.Dir(filePath)
	
	metadata := &Metadata{}

	// Try to extract album and artist from directory structure
	// Common patterns: /Artist/Album/Track or /Music/Artist/Album/Track
	pathParts := strings.Split(dir, string(filepath.Separator))
	if len(pathParts) >= 2 {
		// Get the last two directory parts as potential Artist/Album
		artist := pathParts[len(pathParts)-2]
		album := pathParts[len(pathParts)-1]
		
		// Clean up directory names
		metadata.Artist = cleanDirectoryName(artist)
		metadata.Album = cleanDirectoryName(album)
	}

	// Try to parse filename patterns
	metadata = r.parseFilename(nameWithoutExt, metadata)

	return metadata
}

// parseFilename attempts to extract metadata from filename
func (r *Reader) parseFilename(filename string, metadata *Metadata) *Metadata {
	if metadata == nil {
		metadata = &Metadata{}
	}

	// Pattern: "Track Number - Artist - Title"
	if parts := strings.Split(filename, " - "); len(parts) == 3 {
		if track, err := parseTrackNumber(parts[0]); err == nil && track > 0 {
			metadata.Track = track
		}
		if metadata.Artist == "" {
			metadata.Artist = strings.TrimSpace(parts[1])
		}
		metadata.Title = strings.TrimSpace(parts[2])
		return metadata
	}

	// Pattern: "Artist - Title"
	if parts := strings.Split(filename, " - "); len(parts) == 2 {
		if metadata.Artist == "" {
			metadata.Artist = strings.TrimSpace(parts[0])
		}
		metadata.Title = strings.TrimSpace(parts[1])
		return metadata
	}

	// Pattern: "Track Number. Title" or "Track Number Title"
	if parts := strings.Fields(filename); len(parts) >= 2 {
		firstPart := parts[0]
		// Remove trailing dot if present
		firstPart = strings.TrimSuffix(firstPart, ".")
		
		if track, err := parseTrackNumber(firstPart); err == nil && track > 0 {
			metadata.Track = track
			metadata.Title = strings.Join(parts[1:], " ")
			return metadata
		}
	}

	// Pattern: "Title (Year)" - extract year
	if strings.Contains(filename, "(") && strings.Contains(filename, ")") {
		start := strings.LastIndex(filename, "(")
		end := strings.LastIndex(filename, ")")
		if end > start {
			yearStr := filename[start+1 : end]
			if year, err := strconv.Atoi(yearStr); err == nil && year > 1900 && year < 3000 {
				metadata.Year = year
				metadata.Title = strings.TrimSpace(filename[:start])
				return metadata
			}
		}
	}

	// Fallback: use filename as title
	if metadata.Title == "" {
		metadata.Title = filename
	}

	return metadata
}

// parseTrackNumber attempts to parse a track number from a string
func parseTrackNumber(s string) (int, error) {
	// Remove common prefixes and suffixes
	s = strings.TrimLeft(s, "0")
	s = strings.TrimSuffix(s, ".")
	
	// Try to parse as integer
	if track, err := strconv.Atoi(s); err == nil {
		if track > 0 && track <= 999 {
			return track, nil
		}
	}
	
	return 0, fmt.Errorf("invalid track number: %s", s)
}

// cleanDirectoryName cleans up directory names for use as artist/album names
func cleanDirectoryName(name string) string {
	// Remove common prefixes that indicate directory structure
	prefixes := []string{"CD1", "CD2", "CD3", "Disc 1", "Disc 2", "Disc 3", "Disk 1", "Disk 2", "Disk 3"}
	
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			name = strings.TrimSpace(strings.TrimPrefix(name, prefix))
			break
		}
	}
	
	// Remove year patterns in parentheses at the end
	if strings.Contains(name, "(") && strings.HasSuffix(name, ")") {
		start := strings.LastIndex(name, "(")
		yearPart := name[start+1 : len(name)-1]
		if year, err := strconv.Atoi(yearPart); err == nil && year > 1900 && year < 3000 {
			name = strings.TrimSpace(name[:start])
		}
	}
	
	// Remove underscores and replace with spaces
	name = strings.ReplaceAll(name, "_", " ")
	
	// Clean up multiple spaces
	words := strings.Fields(name)
	return strings.Join(words, " ")
}

// ExtractMetadataMap converts metadata to a string map for compatibility
func (m *Metadata) ExtractMetadataMap() map[string]string {
	result := make(map[string]string)
	
	if m.Title != "" {
		result["title"] = m.Title
	}
	if m.Artist != "" {
		result["artist"] = m.Artist
	}
	if m.Album != "" {
		result["album"] = m.Album
	}
	if m.AlbumArtist != "" {
		result["album_artist"] = m.AlbumArtist
	}
	if m.Genre != "" {
		result["genre"] = m.Genre
	}
	if m.Year > 0 {
		result["year"] = strconv.Itoa(m.Year)
	}
	if m.Track > 0 {
		result["track"] = strconv.Itoa(m.Track)
	}
	if m.Disc > 0 {
		result["disc"] = strconv.Itoa(m.Disc)
	}
	if m.Duration > 0 {
		result["duration"] = strconv.Itoa(m.Duration)
	}
	
	return result
}

// HasBasicMetadata checks if the metadata has at least title and artist
func (m *Metadata) HasBasicMetadata() bool {
	return m.Title != "" && m.Artist != ""
}

// IsEmpty checks if the metadata is completely empty
func (m *Metadata) IsEmpty() bool {
	return m.Title == "" && m.Artist == "" && m.Album == "" && m.Genre == "" && m.Year == 0 && m.Track == 0
} 