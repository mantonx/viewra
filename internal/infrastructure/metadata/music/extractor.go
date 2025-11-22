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
		// Basic metadata
		Title:       strings.TrimSpace(metadata.Title),
		Artist:      strings.TrimSpace(metadata.Artist),
		Album:       strings.TrimSpace(metadata.Album),
		AlbumArtist: strings.TrimSpace(metadata.AlbumArtist),
		Genre:       strings.TrimSpace(metadata.Genre),
		TrackNumber: metadata.TrackNumber,
		DiscNumber:  metadata.DiscNumber,
		Year:        metadata.Year,
		Duration:    int(metadata.Duration.Seconds()),
		Composer:    strings.TrimSpace(metadata.Composer),
		// Extended metadata
		TotalTracks:         metadata.TrackTotal,
		TotalDiscs:          metadata.DiscTotal,
		ReleaseDate:         formatReleaseDate(metadata.ReleaseDate),
		Lyricist:            "", // Not available in current ID3 parser
		ISRC:                strings.TrimSpace(metadata.ISRC),
		ReleaseType:         "", // Not available in current ID3 parser
		Compilation:         false, // Not available in current ID3 parser
		OriginalTitle:       "", // Not available in current ID3 parser
		Publisher:           strings.TrimSpace(metadata.Publisher),
		MusicBrainzTrackID:  strings.TrimSpace(metadata.MusicBrainzTrackID),
		MusicBrainzAlbumID:  strings.TrimSpace(metadata.MusicBrainzReleaseID),
		MusicBrainzArtistID: strings.TrimSpace(metadata.MusicBrainzArtistID),
	}

	return info, nil
}

// formatReleaseDate converts a time.Time to ISO 8601 date string (YYYY-MM-DD)
func formatReleaseDate(t interface{}) string {
	// Handle time.Time from metadata
	if releaseTime, ok := t.(interface{ IsZero() bool; Format(string) string }); ok {
		if !releaseTime.IsZero() {
			return releaseTime.Format("2006-01-02")
		}
	}
	return ""
}
