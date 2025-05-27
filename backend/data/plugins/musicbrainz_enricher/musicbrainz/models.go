// Package musicbrainz provides types and utilities for interacting with the MusicBrainz API.
package musicbrainz

// SearchResponse represents a MusicBrainz search response
type SearchResponse struct {
	Recordings []Recording `json:"recordings,omitempty"`
	Count      int         `json:"count"`
	Offset     int         `json:"offset"`
}

// Recording represents a MusicBrainz recording
type Recording struct {
	ID       string         `json:"id"`
	Title    string         `json:"title"`
	Length   int            `json:"length,omitempty"`
	Releases []Release      `json:"releases,omitempty"`
	Artists  []ArtistCredit `json:"artist-credit,omitempty"`
	Score    int            `json:"score,omitempty"`
}

// Release represents a MusicBrainz release
type Release struct {
	ID           string         `json:"id"`
	Title        string         `json:"title"`
	Date         string         `json:"date,omitempty"`
	Country      string         `json:"country,omitempty"`
	Status       string         `json:"status,omitempty"`
	Artists      []ArtistCredit `json:"artist-credit,omitempty"`
	ReleaseGroup ReleaseGroup   `json:"release-group,omitempty"`
	Media        []Medium       `json:"media,omitempty"`
}

// ReleaseGroup represents a MusicBrainz release group
type ReleaseGroup struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	PrimaryType    string   `json:"primary-type,omitempty"`
	SecondaryTypes []string `json:"secondary-types,omitempty"`
}

// Medium represents a MusicBrainz medium (disc, vinyl, etc.)
type Medium struct {
	Position int     `json:"position"`
	Title    string  `json:"title,omitempty"`
	Format   string  `json:"format,omitempty"`
	Tracks   []Track `json:"tracks,omitempty"`
}

// Track represents a MusicBrainz track
type Track struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Position int    `json:"position"`
	Length   int    `json:"length,omitempty"`
}

// ArtistCredit represents a MusicBrainz artist credit
type ArtistCredit struct {
	Name   string `json:"name"`
	Artist Artist `json:"artist"`
}

// Artist represents a MusicBrainz artist
type Artist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

// CoverArtResponse represents a Cover Art Archive response
type CoverArtResponse struct {
	Images  []CoverArtImage `json:"images"`
	Release string          `json:"release"`
}

// CoverArtImage represents a single cover art image
type CoverArtImage struct {
	ID         string   `json:"id"`
	Image      string   `json:"image"`
	Thumbnails struct {
		Small string `json:"250"`
		Large string `json:"500"`
	} `json:"thumbnails"`
	Front    bool     `json:"front"`
	Back     bool     `json:"back"`
	Types    []string `json:"types"`
	Comment  string   `json:"comment,omitempty"`
	Approved bool     `json:"approved"`
	Edit     int      `json:"edit"`
}

// MatchResult represents the result of matching a recording against metadata
type MatchResult struct {
	Recording *Recording
	Score     float64
	Matched   bool
} 