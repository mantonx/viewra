package models

import (
	"time"
)

// MusicBrainzCache represents cached API responses to avoid repeated requests
type MusicBrainzCache struct {
	ID        uint32    `gorm:"primaryKey" json:"id"`
	QueryHash string    `gorm:"uniqueIndex;not null" json:"query_hash"`
	QueryType string    `gorm:"not null" json:"query_type"` // "artist", "release", "recording", "work"
	Response  string    `gorm:"type:text;not null" json:"response"`
	ExpiresAt time.Time `gorm:"not null;index" json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName specifies the table name for MusicBrainzCache
func (MusicBrainzCache) TableName() string {
	return "musicbrainz_caches"
}

// MusicBrainzEnrichment stores comprehensive music metadata enrichment
type MusicBrainzEnrichment struct {
	ID          uint32 `gorm:"primaryKey" json:"id"`
	MediaFileID string `gorm:"uniqueIndex;not null" json:"media_file_id"`

	// MusicBrainz IDs
	RecordingMBID string `gorm:"index" json:"recording_mbid,omitempty"` // Track/recording ID
	ReleaseMBID   string `gorm:"index" json:"release_mbid,omitempty"`   // Album/release ID
	ArtistMBID    string `gorm:"index" json:"artist_mbid,omitempty"`    // Main artist ID
	WorkMBID      string `gorm:"index" json:"work_mbid,omitempty"`      // Composition/work ID

	// Basic metadata
	EnrichedTitle  string `json:"enriched_title,omitempty"`
	EnrichedArtist string `json:"enriched_artist,omitempty"`
	EnrichedAlbum  string `json:"enriched_album,omitempty"`
	EnrichedYear   int    `json:"enriched_year,omitempty"`
	TrackNumber    int    `json:"track_number,omitempty"`
	DiscNumber     int    `json:"disc_number,omitempty"`
	TotalTracks    int    `json:"total_tracks,omitempty"`

	// Audio metadata
	Duration int    `json:"duration,omitempty"` // in milliseconds
	ISRC     string `json:"isrc,omitempty"`     // International Standard Recording Code
	Barcode  string `json:"barcode,omitempty"`  // Album barcode (EAN/UPC)

	// Release information
	ReleaseDate    string `json:"release_date,omitempty"`    // YYYY-MM-DD format
	ReleaseCountry string `json:"release_country,omitempty"` // ISO country code
	ReleaseStatus  string `json:"release_status,omitempty"`  // Official, Promotion, Bootleg, etc.
	ReleaseType    string `json:"release_type,omitempty"`    // Album, Single, EP, etc.

	// Label information
	LabelName     string `json:"label_name,omitempty"`
	CatalogNumber string `json:"catalog_number,omitempty"`
	LabelMBID     string `json:"label_mbid,omitempty"`

	// Additional artists and credits
	AlbumArtist string `json:"album_artist,omitempty"`                // Main album artist
	Artists     string `gorm:"type:text" json:"artists,omitempty"`    // JSON array of all artists
	Composers   string `gorm:"type:text" json:"composers,omitempty"`  // JSON array of composers
	Performers  string `gorm:"type:text" json:"performers,omitempty"` // JSON array of performers
	Producers   string `gorm:"type:text" json:"producers,omitempty"`  // JSON array of producers

	// Classification
	Genres string `gorm:"type:text" json:"genres,omitempty"` // JSON array of genres
	Tags   string `gorm:"type:text" json:"tags,omitempty"`   // JSON array of user tags
	Styles string `gorm:"type:text" json:"styles,omitempty"` // JSON array of musical styles

	// Work information (for classical music, compositions)
	WorkTitle    string `json:"work_title,omitempty"`    // Title of the musical work
	WorkComposer string `json:"work_composer,omitempty"` // Composer of the work
	WorkType     string `json:"work_type,omitempty"`     // Symphony, Sonata, etc.

	// Relationships and associations
	Relationships string `gorm:"type:text" json:"relationships,omitempty"` // JSON array of artist relationships
	Aliases       string `gorm:"type:text" json:"aliases,omitempty"`       // JSON array of alternative names

	// Technical metadata
	AcoustID string `json:"acoust_id,omitempty"` // AcoustID fingerprint

	// Cover art
	CoverArtURL   string `json:"cover_art_url,omitempty"`                    // Primary cover art URL
	CoverArtPath  string `json:"cover_art_path,omitempty"`                   // Local path to downloaded cover
	ThumbnailURL  string `json:"thumbnail_url,omitempty"`                    // Thumbnail cover art URL
	ThumbnailPath string `json:"thumbnail_path,omitempty"`                   // Local path to thumbnail
	CoverArtTypes string `gorm:"type:text" json:"cover_art_types,omitempty"` // JSON array of cover art types

	// Quality and confidence
	MatchScore  float64 `json:"match_score"`            // Confidence of the match (0-1)
	MatchMethod string  `json:"match_method,omitempty"` // How the match was made (title, ISRC, etc.)

	// Annotations and additional info
	Disambiguation string `gorm:"type:text" json:"disambiguation,omitempty"` // MusicBrainz disambiguation
	Annotation     string `gorm:"type:text" json:"annotation,omitempty"`     // Additional notes

	// Timestamps
	EnrichedAt time.Time `gorm:"not null" json:"enriched_at"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// TableName specifies the table name for MusicBrainzEnrichment
func (MusicBrainzEnrichment) TableName() string {
	return "musicbrainz_enrichments"
}

// MusicBrainzArtwork stores cover art metadata and download information
type MusicBrainzArtwork struct {
	ID          uint32 `gorm:"primaryKey" json:"id"`
	ReleaseMBID string `gorm:"index;not null" json:"release_mbid"`
	MediaFileID string `gorm:"index" json:"media_file_id,omitempty"` // Optional link to specific file

	// Cover Art Archive information
	CoverArtID   string `json:"cover_art_id,omitempty"`  // Cover Art Archive ID
	ImageURL     string `json:"image_url,omitempty"`     // Original image URL
	ThumbnailURL string `json:"thumbnail_url,omitempty"` // Thumbnail URL (250px)
	SmallURL     string `json:"small_url,omitempty"`     // Small URL (500px)
	LargeURL     string `json:"large_url,omitempty"`     // Large URL (1200px)

	// Local storage paths
	LocalPath     string `json:"local_path,omitempty"`     // Path to downloaded image
	ThumbnailPath string `json:"thumbnail_path,omitempty"` // Path to thumbnail

	// Metadata
	ImageType string `json:"image_type,omitempty"` // Front, Back, Booklet, etc.
	MimeType  string `json:"mime_type,omitempty"`  // image/jpeg, image/png
	FileSize  int64  `json:"file_size,omitempty"`  // File size in bytes
	Width     int    `json:"width,omitempty"`      // Image width
	Height    int    `json:"height,omitempty"`     // Image height

	// Source and quality
	Source   string `json:"source,omitempty"`   // Cover Art Archive, Amazon, etc.
	Approved bool   `json:"approved,omitempty"` // If from Cover Art Archive, is it approved
	IsFront  bool   `json:"is_front,omitempty"` // Is this the front cover
	IsBack   bool   `json:"is_back,omitempty"`  // Is this the back cover

	// Download status
	Downloaded    bool   `json:"downloaded,omitempty"`     // Has been successfully downloaded
	DownloadError string `json:"download_error,omitempty"` // Last download error

	// Timestamps
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DownloadedAt *time.Time `json:"downloaded_at,omitempty"` // When successfully downloaded
}

// TableName specifies the table name for MusicBrainzArtwork
func (MusicBrainzArtwork) TableName() string {
	return "musicbrainz_artworks"
}

// HasValidMBID returns true if the enrichment has at least one valid MusicBrainz ID
func (e *MusicBrainzEnrichment) HasValidMBID() bool {
	return e.RecordingMBID != "" || e.ReleaseMBID != "" || e.ArtistMBID != "" || e.WorkMBID != ""
}

// GetPrimaryMBID returns the most relevant MusicBrainz ID (prioritizing recording > release > artist)
func (e *MusicBrainzEnrichment) GetPrimaryMBID() string {
	if e.RecordingMBID != "" {
		return e.RecordingMBID
	}
	if e.ReleaseMBID != "" {
		return e.ReleaseMBID
	}
	if e.ArtistMBID != "" {
		return e.ArtistMBID
	}
	return e.WorkMBID
}

// IsHighConfidenceMatch returns true if the match score indicates high confidence
func (e *MusicBrainzEnrichment) IsHighConfidenceMatch() bool {
	return e.MatchScore >= 0.85
}

// HasCoverArt returns true if cover art information is available
func (e *MusicBrainzEnrichment) HasCoverArt() bool {
	return e.CoverArtURL != "" || e.CoverArtPath != ""
}

// GetDisplayTitle returns the best available title for display
func (e *MusicBrainzEnrichment) GetDisplayTitle() string {
	if e.EnrichedTitle != "" {
		return e.EnrichedTitle
	}
	if e.WorkTitle != "" {
		return e.WorkTitle
	}
	return "Unknown Title"
}

// GetDisplayArtist returns the best available artist name for display
func (e *MusicBrainzEnrichment) GetDisplayArtist() string {
	if e.EnrichedArtist != "" {
		return e.EnrichedArtist
	}
	if e.WorkComposer != "" {
		return e.WorkComposer
	}
	return "Unknown Artist"
}
