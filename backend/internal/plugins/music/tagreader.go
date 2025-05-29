package music

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dhowden/tag"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/metadata"
)

// TagReader handles reading metadata from music files using dhowden/tag
type TagReader struct {
	supportedFormats map[string]bool
}

// NewTagReader creates a new tag reader instance
func NewTagReader() *TagReader {
	return &TagReader{
		supportedFormats: map[string]bool{
			"mp3":  true,
			"flac": true,
			"wav":  true,
			"m4a":  true,
			"aac":  true,
			"ogg":  true,
			"wma":  true,
			"opus": true,
			"aiff": true,
			"ape":  true,
			"wv":   true,
		},
	}
}

// CanReadFile checks if the tag reader can handle the given file extension
func (tr *TagReader) CanReadFile(path string) bool {
	ext := getFileExtension(path)
	return tr.supportedFormats[ext]
}

// ReadMetadata extracts metadata from a music file
func (tr *TagReader) ReadMetadata(path string) (*database.MusicMetadata, error) {
	// Check if file exists and is readable
	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	
	if fileInfo.Size() == 0 {
		return nil, fmt.Errorf("file is empty: %s", path)
	}
	
	// Open the file
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	
	// Extract metadata using dhowden/tag
	tagMetadata, err := tag.ReadFrom(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata from file: %w", err)
	}
	
	if tagMetadata == nil {
		return nil, fmt.Errorf("no metadata found in file")
	}
	
	// Extract technical information using FFprobe (if available)
	var technicalInfo *metadata.AudioTechnicalInfo
	if metadata.IsFFProbeAvailable() {
		technicalInfo, err = metadata.ExtractAudioTechnicalInfo(path)
		if err != nil {
			fmt.Printf("WARNING: [TagReader] FFprobe extraction failed for %s: %v\n", path, err)
			// Continue with fallback approach
		}
	}
	
	// Convert to our database model
	musicMeta := &database.MusicMetadata{
		Title:       cleanString(tagMetadata.Title()),
		Artist:      cleanString(tagMetadata.Artist()),
		Album:       cleanString(tagMetadata.Album()),
		AlbumArtist: cleanString(tagMetadata.AlbumArtist()),
		Genre:       cleanString(tagMetadata.Genre()),
	}

	// Use FFprobe data if available, otherwise fall back to file extension
	if technicalInfo != nil {
		musicMeta.Format = technicalInfo.Format
		musicMeta.Bitrate = technicalInfo.Bitrate
		musicMeta.SampleRate = technicalInfo.SampleRate
		musicMeta.Channels = technicalInfo.Channels
		if technicalInfo.Duration > 0 {
			musicMeta.Duration = time.Duration(technicalInfo.Duration * float64(time.Second))
		}
	} else {
		// Fallback to file extension
		musicMeta.Format = getFileExtension(path)
		musicMeta.Bitrate = 0 // No bitrate available without FFprobe
		musicMeta.SampleRate = 0 // No sample rate available without FFprobe
		musicMeta.Channels = 0 // No channel info available without FFprobe
	}
	
	// Handle year
	if year := tagMetadata.Year(); year != 0 {
		musicMeta.Year = year
	}
	
	// Handle track number
	if track, total := tagMetadata.Track(); track != 0 {
		musicMeta.Track = track
		musicMeta.TrackTotal = total
	}
	
	// Handle disc number 
	if disc, total := tagMetadata.Disc(); disc != 0 {
		musicMeta.Disc = disc
		musicMeta.DiscTotal = total
	}
	
	// Check for artwork
	if picture := tagMetadata.Picture(); picture != nil && len(picture.Data) > 0 {
		musicMeta.HasArtwork = true
		// Store artwork data temporarily for later processing
		musicMeta.ArtworkData = picture.Data
		if picture.Ext != "" {
			musicMeta.ArtworkExt = picture.Ext
		} else {
			// Determine extension from MIME type
			switch picture.MIMEType {
			case "image/jpeg":
				musicMeta.ArtworkExt = "jpg"
			case "image/png":
				musicMeta.ArtworkExt = "png"
			case "image/gif":
				musicMeta.ArtworkExt = "gif"
			default:
				musicMeta.ArtworkExt = "jpg" // Default
			}
		}
	}
	
	return musicMeta, nil
}

// GetSupportedExtensions returns the file extensions supported by this reader
func (tr *TagReader) GetSupportedExtensions() []string {
	exts := make([]string, 0, len(tr.supportedFormats))
	for ext := range tr.supportedFormats {
		exts = append(exts, "."+ext)
	}
	return exts
}

// cleanString trims whitespace and handles empty strings
func cleanString(s string) string {
	cleaned := strings.TrimSpace(s)
	if cleaned == "" {
		return cleaned
	}
	// Remove extra whitespace
	fields := strings.Fields(cleaned)
	return strings.Join(fields, " ")
}

// getFileExtension returns the file extension in lowercase without the dot
func getFileExtension(path string) string {
	// Find the last dot
	lastDot := -1
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			lastDot = i
			break
		}
		if path[i] == '/' || path[i] == '\\' {
			break // Hit path separator before finding dot
		}
	}
	
	if lastDot == -1 || lastDot == len(path)-1 {
		return ""
	}
	
	// Convert to lowercase and remove the dot
	ext := path[lastDot+1:]
	result := make([]byte, len(ext))
	for i, b := range []byte(ext) {
		if b >= 'A' && b <= 'Z' {
			result[i] = b + 32 // Convert to lowercase
		} else {
			result[i] = b
		}
	}
	
	return string(result)
} 