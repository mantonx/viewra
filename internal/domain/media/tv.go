package media

// TVEpisode represents a TV episode media item with TV-specific metadata
type TVEpisode struct {
	Media // Embedded base media fields

	// TV-specific fields
	ShowID       int64  // Foreign key to tv_shows
	ShowTitle    string // Populated from show lookup for display, not stored in tv_episodes
	SeasonID     int64  // Foreign key to tv_seasons
	Season       int
	Episode      int
	EpisodeTitle string // Different from Media.Title which is the filename
	TVDbID       int
	IMDbID       string
	AirDate      string // YYYY-MM-DD format
	Description  string

	// Ratings and runtime
	RuntimeMinutes int     // Episode runtime in minutes
	Rating         float32 // Average rating (0.0 - 10.0)
	RatingVotes    int     // Number of votes
}

// IsValid validates the TV episode entity including base media validation
func (tv *TVEpisode) IsValid() error {
	// Validate base media fields first
	if err := tv.Media.IsValid(); err != nil {
		return err
	}

	// TV-specific validation
	if tv.Season < 0 {
		return ErrInvalidSeason
	}

	if tv.Episode < 1 {
		return ErrInvalidEpisode
	}

	return nil
}
