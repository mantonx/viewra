package music

import (
	"fmt"
	"os"
	"time"

	"github.com/dhowden/tag"
)

// TrackMetadata represents extracted metadata from audio files
type TrackMetadata struct {
	// Basic information
	Title       string
	Artist      string
	Album       string
	AlbumArtist string
	Composer    string
	Genre       string
	Year        int

	// Track information
	TrackNumber int
	TrackTotal  int
	DiscNumber  int
	DiscTotal   int

	// Additional metadata
	Comment     string
	Lyrics      string
	Copyright   string
	Publisher   string
	ISRC        string // International Standard Recording Code
	Barcode     string // Album barcode/UPC
	CatalogNum  string // Catalog number
	ReleaseDate time.Time

	// Technical information
	Duration   time.Duration
	Bitrate    int // in kbps
	SampleRate int // in Hz
	Channels   int
	Codec      string // MP3, FLAC, AAC, etc.

	// IDs
	MusicBrainzTrackID       string
	MusicBrainzRecordingID   string
	MusicBrainzReleaseID     string
	MusicBrainzArtistID      string
	MusicBrainzAlbumArtistID string
}

// ParseAudioFile extracts metadata from an audio file using ID3/Vorbis/APE tags
func ParseAudioFile(filePath string) (*TrackMetadata, error) {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Parse the tags
	m, err := tag.ReadFrom(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read tags: %w", err)
	}

	// Extract metadata
	metadata := &TrackMetadata{
		Title:       m.Title(),
		Artist:      m.Artist(),
		Album:       m.Album(),
		AlbumArtist: m.AlbumArtist(),
		Composer:    m.Composer(),
		Genre:       m.Genre(),
		Year:        m.Year(),
		Comment:     m.Comment(),
		Lyrics:      m.Lyrics(),
		Codec:       string(m.FileType()),
	}

	// Extract track/disc numbers
	trackNum, trackTotal := m.Track()
	metadata.TrackNumber = trackNum
	metadata.TrackTotal = trackTotal

	discNum, discTotal := m.Disc()
	metadata.DiscNumber = discNum
	metadata.DiscTotal = discTotal

	// TODO: Extract custom tags (MusicBrainz IDs, etc.) when tag library supports it
	// The tag.Custom interface is not exported in the current version
	// For now, we only extract the basic metadata fields that are directly available

	return metadata, nil
}

// ParseAudioFileWithTechnicalInfo extracts both metadata and technical information
// This requires additional processing beyond basic tag reading
func ParseAudioFileWithTechnicalInfo(filePath string) (*TrackMetadata, error) {
	metadata, err := ParseAudioFile(filePath)
	if err != nil {
		return nil, err
	}

	// Get file info for additional details
	fileInfo, err := os.Stat(filePath)
	if err == nil {
		// File size is available but we don't store it in TrackMetadata
		// Technical audio properties (bitrate, sample rate, etc.) would need
		// a more specialized library like ffprobe or a format-specific parser
		_ = fileInfo
	}

	return metadata, nil
}

// HasMetadata checks if an audio file has parseable metadata tags
func HasMetadata(filePath string) bool {
	file, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer file.Close()

	_, err = tag.ReadFrom(file)
	return err == nil
}
