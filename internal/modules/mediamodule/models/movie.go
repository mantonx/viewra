// Package models - Movie models
// Package models - Movie models for future migration
//
// IMPORTANT: These models are NOT currently in use.
// See media.go for full migration plan.
//
package models

import (
	"time"
)

// Movie represents a movie
type Movie struct {
	ID            string     `gorm:"type:varchar(36);primaryKey" json:"id"`
	Title         string     `gorm:"not null;index" json:"title"`
	OriginalTitle string     `json:"original_title"`
	Overview      string     `gorm:"type:text" json:"overview"`
	Tagline       string     `json:"tagline"`
	ReleaseDate   *time.Time `json:"release_date"`
	Runtime       int        `json:"runtime"` // In minutes

	// Ratings & Compliance
	Rating     string  `json:"rating"`      // e.g. PG-13, R, etc.
	TmdbRating float64 `json:"tmdb_rating"` // TMDb vote average
	VoteCount  int     `json:"vote_count"`
	Popularity float64 `json:"popularity"`

	// Status & Production Info
	Status string `json:"status"` // Released, In Production, Post Production, etc.
	Adult  bool   `json:"adult"`  // Adult content flag
	Video  bool   `json:"video"`  // Video release flag

	// Financial Data
	Budget  int64 `json:"budget"`  // Production budget in USD
	Revenue int64 `json:"revenue"` // Global box office in USD

	// Artwork & Media
	Poster   string `json:"poster"`
	Backdrop string `json:"backdrop"`

	// Text Data & Metadata (stored as JSON for flexibility)
	Genres              string `gorm:"type:text" json:"genres"`               // JSON array of genres
	ProductionCompanies string `gorm:"type:text" json:"production_companies"` // JSON array of companies
	ProductionCountries string `gorm:"type:text" json:"production_countries"` // JSON array of countries
	SpokenLanguages     string `gorm:"type:text" json:"spoken_languages"`     // JSON array of languages
	Keywords            string `gorm:"type:text" json:"keywords"`             // JSON array of keywords/tags

	// Cast & Crew (summary data)
	MainCast string `gorm:"type:text" json:"main_cast"` // JSON array of main cast
	MainCrew string `gorm:"type:text" json:"main_crew"` // JSON array of main crew

	// External IDs & Links
	TmdbID      string `gorm:"index" json:"tmdb_id"`
	ImdbID      string `gorm:"index" json:"imdb_id"`
	ExternalIDs string `gorm:"type:text" json:"external_ids"` // JSON object with all external IDs

	// Language & Region
	OriginalLanguage string `json:"original_language"`

	// Franchise & Collection
	Collection string `gorm:"type:text" json:"collection"` // JSON object if part of collection/franchise

	// Awards & Recognition
	Awards string `gorm:"type:text" json:"awards"` // JSON array of awards

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}