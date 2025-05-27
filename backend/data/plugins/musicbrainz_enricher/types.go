package musicbrainz_enricher

import (
	"context"
	"time"
)

// Generic interfaces and types that can be shared across plugins

// Track represents a generic music track with enrichable metadata
type Track struct {
	ID          uint   `json:"id"`
	FilePath    string `json:"file_path"`
	Title       string `json:"title,omitempty"`
	Artist      string `json:"artist,omitempty"`
	Album       string `json:"album,omitempty"`
	AlbumArtist string `json:"album_artist,omitempty"`
	Year        int    `json:"year,omitempty"`
	TrackNumber int    `json:"track_number,omitempty"`
	DiscNumber  int    `json:"disc_number,omitempty"`
	Genre       string `json:"genre,omitempty"`
	Duration    int    `json:"duration,omitempty"` // in seconds
}

// EnrichmentResult represents the result of an enrichment operation
type EnrichmentResult struct {
	Success     bool                   `json:"success"`
	TrackID     uint                   `json:"track_id"`
	Source      string                 `json:"source"`      // e.g., "musicbrainz", "lastfm", "discogs"
	MatchScore  float64                `json:"match_score"` // 0.0 to 1.0
	EnrichedAt  time.Time              `json:"enriched_at"`
	Metadata    map[string]interface{} `json:"metadata"`
	ExternalIDs map[string]string      `json:"external_ids,omitempty"` // e.g., {"musicbrainz_recording_id": "abc123"}
	ArtworkURL  string                 `json:"artwork_url,omitempty"`
	Error       string                 `json:"error,omitempty"`
}

// EnrichmentRequest represents a request to enrich a track
type EnrichmentRequest struct {
	Track       *Track                 `json:"track"`
	Options     map[string]interface{} `json:"options,omitempty"`
	ForceUpdate bool                   `json:"force_update,omitempty"`
}

// MetadataEnricher is the generic interface that all enrichment plugins must implement
type MetadataEnricher interface {
	// GetID returns the unique identifier for this enricher
	GetID() string

	// GetName returns the human-readable name for this enricher
	GetName() string

	// GetSupportedMediaTypes returns the media types this enricher supports
	GetSupportedMediaTypes() []string

	// CanEnrich determines if this enricher can process the given track
	CanEnrich(track *Track) bool

	// Enrich performs the actual enrichment operation
	Enrich(ctx context.Context, request *EnrichmentRequest) (*EnrichmentResult, error)

	// GetPriority returns the priority of this enricher (higher = more priority)
	GetPriority() int

	// IsEnabled returns whether this enricher is currently enabled
	IsEnabled() bool

	// GetConfiguration returns the current configuration
	GetConfiguration() map[string]interface{}
}

// EnrichmentRegistry manages all registered enrichers
type EnrichmentRegistry interface {
	// RegisterEnricher registers a new enricher for the given media type
	RegisterEnricher(mediaType string, enricher MetadataEnricher) error

	// GetEnrichers returns all enrichers for a given media type
	GetEnrichers(mediaType string) []MetadataEnricher

	// GetEnricher returns a specific enricher by ID
	GetEnricher(enricherID string) (MetadataEnricher, bool)

	// EnrichTrack enriches a track using the best available enricher
	EnrichTrack(ctx context.Context, track *Track, options map[string]interface{}) (*EnrichmentResult, error)
}

// PluginInitializer is the interface that plugins must implement for initialization
type PluginInitializer interface {
	// Init initializes the plugin and registers it with the system
	Init() error
}

// MediaType constants for different types of media
const (
	MediaTypeMusic = "music"
	MediaTypeVideo = "video"
	MediaTypePhoto = "photo"
	MediaTypeBook  = "book"
)

// Common enrichment sources
const (
	SourceMusicBrainz = "musicbrainz"
	SourceLastFM      = "lastfm"
	SourceDiscogs     = "discogs"
	SourceSpotify     = "spotify"
	SourceITunes      = "itunes"
) 