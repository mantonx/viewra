// Package types - Metadata types
package types

import (
	"time"
)

// MetadataUpdate represents a metadata update request
type MetadataUpdate struct {
	FileID   string            `json:"file_id"`
	Metadata map[string]string `json:"metadata"`
}

// EnrichmentRequest represents a request to enrich media metadata
type EnrichmentRequest struct {
	MediaType   string   `json:"media_type"` // movie, episode, track
	MediaIDs    []string `json:"media_ids"`
	Providers   []string `json:"providers,omitempty"`
	ForceUpdate bool     `json:"force_update"`
}

// EnrichmentResult represents the result of metadata enrichment
type EnrichmentResult struct {
	MediaID    string                 `json:"media_id"`
	Success    bool                   `json:"success"`
	Provider   string                 `json:"provider"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Error      string                 `json:"error,omitempty"`
	EnrichedAt time.Time              `json:"enriched_at"`
}

// MovieMetadata represents enriched movie metadata
type MovieMetadata struct {
	Title         string    `json:"title"`
	OriginalTitle string    `json:"original_title,omitempty"`
	Year          int       `json:"year"`
	ReleaseDate   time.Time `json:"release_date"`
	Runtime       int       `json:"runtime"` // minutes
	Overview      string    `json:"overview"`
	Tagline       string    `json:"tagline,omitempty"`
	Genres        []string  `json:"genres"`
	Rating        float64   `json:"rating"`
	VoteCount     int       `json:"vote_count"`
	Poster        string    `json:"poster,omitempty"`
	Backdrop      string    `json:"backdrop,omitempty"`
	IMDBID        string    `json:"imdb_id,omitempty"`
	TMDBID        int       `json:"tmdb_id,omitempty"`
}

// EpisodeMetadata represents enriched episode metadata
type EpisodeMetadata struct {
	Title         string    `json:"title"`
	EpisodeNumber int       `json:"episode_number"`
	SeasonNumber  int       `json:"season_number"`
	AirDate       time.Time `json:"air_date"`
	Overview      string    `json:"overview"`
	Runtime       int       `json:"runtime"` // minutes
	Rating        float64   `json:"rating"`
	StillImage    string    `json:"still_image,omitempty"`
	ShowTitle     string    `json:"show_title"`
	ShowID        string    `json:"show_id"`
}

// TrackMetadata represents enriched music track metadata
type TrackMetadata struct {
	Title        string   `json:"title"`
	Artist       string   `json:"artist"`
	Album        string   `json:"album"`
	AlbumArtist  string   `json:"album_artist,omitempty"`
	TrackNumber  int      `json:"track_number"`
	DiscNumber   int      `json:"disc_number,omitempty"`
	Year         int      `json:"year,omitempty"`
	Genre        string   `json:"genre,omitempty"`
	Genres       []string `json:"genres,omitempty"`
	Duration     int      `json:"duration"` // seconds
	AlbumArt     string   `json:"album_art,omitempty"`
	MusicBrainzID string  `json:"musicbrainz_id,omitempty"`
}