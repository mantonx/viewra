package media

import "time"

// Movie represents a movie media item with movie-specific metadata
type Movie struct {
	Media // Embedded base media fields

	// Basic movie information
	Year             int
	ReleaseDate      time.Time
	OriginalTitle    string
	SortTitle        string
	RuntimeMinutes   int

	// External IDs
	IMDbID string
	TMDbID int

	// People
	Director string
	Cast     []string

	// Content information
	Genre             []string
	Plot              string
	Tagline           string
	ContentRating     string // e.g., "PG-13", "R"
	MaturityRating    int    // Numeric rating 0-10
	ContentAdvisories []string

	// Ratings
	Rating      float32 // Average rating (0.0 - 10.0)
	RatingVotes int     // Number of votes

	// Production details
	Budget           int64
	Revenue          int64
	OriginalLanguage string
	CountryOfOrigin  string
	AwardsSummary    string
}

// IsValid validates the movie entity including base media validation
func (m *Movie) IsValid() error {
	// Validate base media fields first
	if err := m.Media.IsValid(); err != nil {
		return err
	}

	// Movie-specific validation
	if m.Year < 1800 || m.Year > 2100 {
		if m.Year != 0 { // 0 is acceptable for unknown year
			return ErrInvalidYear
		}
	}

	if m.MaturityRating < 0 || m.MaturityRating > 10 {
		if m.MaturityRating != 0 { // 0 is acceptable for no rating
			return ErrInvalidRating
		}
	}

	if m.RuntimeMinutes < 0 {
		return ErrInvalidDuration
	}

	return nil
}
