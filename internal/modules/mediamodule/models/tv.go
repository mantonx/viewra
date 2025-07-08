// Package models - TV show models
// Package models - TV show models for future migration
//
// IMPORTANT: These models are NOT currently in use.
// See media.go for full migration plan.
//
package models

import (
	"time"
)

// TVShow represents a TV show
type TVShow struct {
	ID           string     `gorm:"type:varchar(36);primaryKey" json:"id"`
	Title        string     `gorm:"not null;index" json:"title"`
	Description  string     `gorm:"type:text" json:"description"`
	FirstAirDate *time.Time `json:"first_air_date"`
	Status       string     `json:"status"` // e.g., Running, Ended
	Poster       string     `json:"poster"`
	Backdrop     string     `json:"backdrop"`
	TmdbID       string     `gorm:"index" json:"tmdb_id"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`

	// Relationships
	Seasons []Season `gorm:"foreignKey:TVShowID" json:"seasons,omitempty"`
}

// Season represents a season of a TV show
type Season struct {
	ID           string     `gorm:"type:varchar(36);primaryKey" json:"id"`
	TVShowID     string     `gorm:"type:varchar(36);not null;index" json:"tv_show_id"`
	TVShow       TVShow     `gorm:"foreignKey:TVShowID" json:"tv_show,omitempty"`
	SeasonNumber int        `gorm:"not null;index" json:"season_number"`
	Description  string     `gorm:"type:text" json:"description"`
	Poster       string     `json:"poster"`
	AirDate      *time.Time `json:"air_date"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`

	// Relationships
	Episodes []Episode `gorm:"foreignKey:SeasonID" json:"episodes,omitempty"`
}

// Episode represents an episode of a TV show
type Episode struct {
	ID            string     `gorm:"type:varchar(36);primaryKey" json:"id"`
	SeasonID      string     `gorm:"type:varchar(36);not null;index" json:"season_id"`
	Season        Season     `gorm:"foreignKey:SeasonID" json:"season,omitempty"`
	Title         string     `gorm:"not null;index" json:"title"`
	EpisodeNumber int        `gorm:"not null;index" json:"episode_number"`
	AirDate       *time.Time `json:"air_date"`
	Description   string     `gorm:"type:text" json:"description"`
	Duration      int        `json:"duration"` // In seconds
	StillImage    string     `json:"still_image"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}