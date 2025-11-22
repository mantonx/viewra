package music

import (
	"strings"

	"github.com/mantonx/viewra/internal/domain/scanner"
)

// Extractor adapts the infrastructure ID3 parser to the domain's MusicMetadataExtractor interface
type Extractor struct{}

// NewExtractor creates a new music metadata extractor
func NewExtractor() scanner.MusicMetadataExtractor {
	return &Extractor{}
}

// ExtractMetadata extracts metadata from an audio file using ID3/Vorbis/APE tags
func (e *Extractor) ExtractMetadata(filePath string) (*scanner.MusicInfo, error) {
	// Use the existing ParseAudioFile function
	metadata, err := ParseAudioFile(filePath)
	if err != nil {
		return nil, err
	}

	// Convert from TrackMetadata to scanner.MusicInfo
	info := &scanner.MusicInfo{
		Title:       strings.TrimSpace(metadata.Title),
		Artist:      strings.TrimSpace(metadata.Artist),
		Album:       strings.TrimSpace(metadata.Album),
		AlbumArtist: strings.TrimSpace(metadata.AlbumArtist),
		Genre:       strings.TrimSpace(metadata.Genre),
		TrackNumber: metadata.TrackNumber,
		DiscNumber:  metadata.DiscNumber,
		Year:        metadata.Year,
		Duration:    int(metadata.Duration.Seconds()),
	}

	return info, nil
}
