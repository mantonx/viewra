package models

import (
	"fmt"
	"time"
)

// TMDbCache represents cached API responses from TMDb
type TMDbCache struct {
	ID        uint32    `gorm:"primaryKey" json:"id"`
	QueryHash string    `gorm:"uniqueIndex;not null" json:"query_hash"` // Hash of the query parameters
	QueryType string    `gorm:"not null;index" json:"query_type"`       // Type of query (search, movie, tv, etc.)
	Response  string    `gorm:"type:text;not null" json:"response"`     // JSON response from TMDb
	ExpiresAt time.Time `gorm:"not null;index" json:"expires_at"`       // When this cache entry expires
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// TableName returns the table name for TMDbCache
func (TMDbCache) TableName() string {
	return "tmdb_cache"
}

// IsExpired checks if the cache entry has expired
func (c *TMDbCache) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}

// TMDbEnrichment represents enrichment data from TMDb for media files
type TMDbEnrichment struct {
	ID          uint32 `gorm:"primaryKey" json:"id"`
	MediaFileID string `gorm:"not null;index" json:"media_file_id"` // Reference to media file
	TMDbID      int    `gorm:"not null;index" json:"tmdb_id"`       // TMDb ID
	TMDbType    string `gorm:"not null;index" json:"tmdb_type"`     // movie, tv, episode

	// Basic metadata
	Title         string     `gorm:"not null" json:"title"`
	OriginalTitle string     `json:"original_title,omitempty"`
	Overview      string     `gorm:"type:text" json:"overview,omitempty"`
	ReleaseDate   *time.Time `json:"release_date,omitempty"`

	// TV-specific fields
	SeasonNumber  *int `json:"season_number,omitempty"`
	EpisodeNumber *int `json:"episode_number,omitempty"`
	ShowTMDbID    *int `json:"show_tmdb_id,omitempty"` // For episodes, reference to show

	// Additional metadata (stored as JSON for flexibility)
	Genres      string `gorm:"type:text" json:"genres,omitempty"`       // JSON array of genres
	Cast        string `gorm:"type:text" json:"cast,omitempty"`         // JSON array of cast members
	Crew        string `gorm:"type:text" json:"crew,omitempty"`         // JSON array of crew members
	Keywords    string `gorm:"type:text" json:"keywords,omitempty"`     // JSON array of keywords
	ExternalIDs string `gorm:"type:text" json:"external_ids,omitempty"` // JSON object of external IDs

	// Quality and processing metadata
	ConfidenceScore float64   `gorm:"not null;default:0" json:"confidence_score"` // Match confidence (0-1)
	SourcePlugin    string    `gorm:"not null" json:"source_plugin"`              // Plugin that created this enrichment
	ProcessedAt     time.Time `gorm:"autoCreateTime" json:"processed_at"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName returns the table name for TMDbEnrichment
func (TMDbEnrichment) TableName() string {
	return "tmdb_enrichments"
}

// IsEpisode returns true if this enrichment is for an episode
func (e *TMDbEnrichment) IsEpisode() bool {
	return e.TMDbType == "episode"
}

// IsTVShow returns true if this enrichment is for a TV show
func (e *TMDbEnrichment) IsTVShow() bool {
	return e.TMDbType == "tv"
}

// IsMovie returns true if this enrichment is for a movie
func (e *TMDbEnrichment) IsMovie() bool {
	return e.TMDbType == "movie"
}

// TMDbArtwork represents artwork metadata downloaded from TMDb
type TMDbArtwork struct {
	ID          uint32 `gorm:"primaryKey" json:"id"`
	MediaFileID string `gorm:"not null;index" json:"media_file_id"` // Reference to media file
	TMDbID      int    `gorm:"not null;index" json:"tmdb_id"`       // TMDb ID

	// Artwork classification
	ArtworkType string `gorm:"not null;index" json:"artwork_type"` // poster, backdrop, logo, still
	Category    string `gorm:"not null;index" json:"category"`     // movie, tv, season, episode
	Subtype     string `gorm:"index" json:"subtype,omitempty"`     // season number, episode number, etc.

	// File information
	OriginalURL string `gorm:"not null" json:"original_url"`     // Original TMDb URL
	LocalPath   string `gorm:"not null;index" json:"local_path"` // Local file path
	FileName    string `gorm:"not null" json:"file_name"`        // Original filename
	MimeType    string `gorm:"not null" json:"mime_type"`        // MIME type
	FileSize    int64  `gorm:"not null" json:"file_size"`        // File size in bytes
	FileHash    string `gorm:"not null;index" json:"file_hash"`  // File hash for deduplication

	// TMDb metadata
	Width       int     `json:"width,omitempty"`        // Image width
	Height      int     `json:"height,omitempty"`       // Image height
	AspectRatio float64 `json:"aspect_ratio,omitempty"` // Aspect ratio
	Language    string  `json:"language,omitempty"`     // Language (ISO 639-1)
	VoteAverage float64 `json:"vote_average,omitempty"` // TMDb vote average
	VoteCount   int     `json:"vote_count,omitempty"`   // TMDb vote count

	// Processing metadata
	SourcePlugin string     `gorm:"not null" json:"source_plugin"` // Plugin that downloaded this
	DownloadedAt time.Time  `gorm:"autoCreateTime" json:"downloaded_at"`
	LastVerified *time.Time `json:"last_verified,omitempty"` // Last time file existence was verified
}

// TableName returns the table name for TMDbArtwork
func (TMDbArtwork) TableName() string {
	return "tmdb_artwork"
}

// IsPoster returns true if this is a poster image
func (a *TMDbArtwork) IsPoster() bool {
	return a.ArtworkType == "poster"
}

// IsBackdrop returns true if this is a backdrop image
func (a *TMDbArtwork) IsBackdrop() bool {
	return a.ArtworkType == "backdrop"
}

// IsLogo returns true if this is a logo image
func (a *TMDbArtwork) IsLogo() bool {
	return a.ArtworkType == "logo"
}

// IsStill returns true if this is a still image (episode/scene still)
func (a *TMDbArtwork) IsStill() bool {
	return a.ArtworkType == "still"
}

// GetDisplayName returns a human-readable name for the artwork
func (a *TMDbArtwork) GetDisplayName() string {
	if a.Category == "season" && a.Subtype != "" {
		return fmt.Sprintf("Season %s %s", a.Subtype, a.ArtworkType)
	}
	if a.Category == "episode" && a.Subtype != "" {
		return fmt.Sprintf("Episode %s %s", a.Subtype, a.ArtworkType)
	}
	return fmt.Sprintf("%s %s", a.Category, a.ArtworkType)
}

// GetSizeString returns a formatted size string
func (a *TMDbArtwork) GetSizeString() string {
	if a.Width > 0 && a.Height > 0 {
		return fmt.Sprintf("%dx%d", a.Width, a.Height)
	}
	return "unknown"
}

// GetFileSizeString returns a human-readable file size
func (a *TMDbArtwork) GetFileSizeString() string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	size := float64(a.FileSize)
	switch {
	case size >= GB:
		return fmt.Sprintf("%.2f GB", size/GB)
	case size >= MB:
		return fmt.Sprintf("%.2f MB", size/MB)
	case size >= KB:
		return fmt.Sprintf("%.2f KB", size/KB)
	default:
		return fmt.Sprintf("%d bytes", a.FileSize)
	}
}
