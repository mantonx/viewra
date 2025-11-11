package media

// Movie represents a movie media item with movie-specific metadata
type Movie struct {
	Media // Embedded base media fields

	// Movie-specific fields
	Year        int
	IMDbID      string
	TMDbID      int
	Director    string
	Rating      float32 // e.g., 7.5
	Genres      []string
	Description string
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

	if m.Rating < 0 || m.Rating > 10 {
		if m.Rating != 0 { // 0 is acceptable for no rating
			return ErrInvalidRating
		}
	}

	return nil
}
