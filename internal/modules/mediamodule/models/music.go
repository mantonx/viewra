// Package models - Music models for future migration
//
// IMPORTANT: These models are NOT currently in use.
// See media.go for full migration plan.
//
package models

import (
	"time"

	"gorm.io/gorm"
)

// Artist represents a music artist in the media library
type Artist struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"not null;index" json:"name"`
	Description string         `gorm:"type:text" json:"description"` // Changed from Biography
	Biography   string         `gorm:"type:text" json:"biography"`
	Country     string         `json:"country"`
	Genres      string         `json:"genres"` // JSON array as string
	Image       string         `json:"image"` // Changed from ImageURL
	ImageURL    string         `json:"image_url"`
	MusicBrainz string         `gorm:"uniqueIndex" json:"musicbrainz_id"`
	Spotify     string         `gorm:"uniqueIndex" json:"spotify_id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at"`

	// Relationships
	Albums []Album `gorm:"foreignKey:ArtistID" json:"albums,omitempty"`
	Tracks []Track `gorm:"foreignKey:ArtistID" json:"tracks,omitempty"`
}

// Album represents a music album in the media library
type Album struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Title       string         `gorm:"not null;index" json:"title"`
	ArtistID    uint           `gorm:"not null;index" json:"artist_id"`
	ReleaseDate *time.Time     `json:"release_date"`
	Genre       string         `json:"genre"`
	TrackCount  int            `json:"track_count"`
	Duration    int            `json:"duration"` // Duration in seconds
	Artwork     string         `json:"artwork"` // Changed from CoverArt
	CoverArt    string         `json:"cover_art"`
	MusicBrainz string         `gorm:"uniqueIndex" json:"musicbrainz_id"`
	Spotify     string         `gorm:"uniqueIndex" json:"spotify_id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at"`

	// Relationships
	Artist Artist  `gorm:"foreignKey:ArtistID" json:"artist,omitempty"`
	Tracks []Track `gorm:"foreignKey:AlbumID" json:"tracks,omitempty"`
}

// Track represents a music track in the media library
type Track struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Title        string         `gorm:"not null;index" json:"title"`
	ArtistID     uint           `gorm:"not null;index" json:"artist_id"`
	AlbumID      uint           `gorm:"index" json:"album_id"`
	TrackNumber  int            `json:"track_number"`
	DiscNumber   int            `json:"disc_number"`
	Duration     int            `json:"duration"` // Duration in seconds
	Genre        string         `json:"genre"`
	Year         int            `json:"year"`
	Bitrate      int            `json:"bitrate"`
	SampleRate   int            `json:"sample_rate"`
	Channels     int            `json:"channels"`
	MusicBrainz  string         `gorm:"uniqueIndex" json:"musicbrainz_id"`
	Spotify      string         `gorm:"uniqueIndex" json:"spotify_id"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at"`

	// Relationships
	Artist Artist `gorm:"foreignKey:ArtistID" json:"artist,omitempty"`
	Album  Album  `gorm:"foreignKey:AlbumID" json:"album,omitempty"`
}

// Playlist represents a user-created playlist
type Playlist struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"not null" json:"name"`
	Description string         `gorm:"type:text" json:"description"`
	UserID      string         `gorm:"not null;index" json:"user_id"`
	IsPublic    bool           `gorm:"default:false" json:"is_public"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at"`

	// Relationships
	Tracks []Track `gorm:"many2many:playlist_tracks;" json:"tracks,omitempty"`
}

// PlaylistTrack represents the many-to-many relationship between playlists and tracks
type PlaylistTrack struct {
	PlaylistID uint      `gorm:"primaryKey" json:"playlist_id"`
	TrackID    uint      `gorm:"primaryKey" json:"track_id"`
	Position   int       `gorm:"not null" json:"position"`
	AddedAt    time.Time `gorm:"autoCreateTime" json:"added_at"`

	// Relationships
	Playlist Playlist `gorm:"foreignKey:PlaylistID" json:"playlist,omitempty"`
	Track    Track    `gorm:"foreignKey:TrackID" json:"track,omitempty"`
}