package media

// MusicTrack represents a music track media item with music-specific metadata
type MusicTrack struct {
	Media // Embedded base media fields

	// Music-specific fields
	Artist      string
	Album       string
	TrackNumber int
	Year        int
	Genre       string
	Composer    string
	Publisher   string
	Bitrate     int // In kbps
}

// IsValid validates the music track entity including base media validation
func (m *MusicTrack) IsValid() error {
	// Validate base media fields first
	if err := m.Media.IsValid(); err != nil {
		return err
	}

	// Music-specific validation
	if m.TrackNumber < 0 {
		return ErrInvalidTrackNumber
	}

	if m.Year < 0 || m.Year > 2100 {
		if m.Year != 0 { // 0 is acceptable for unknown year
			return ErrInvalidYear
		}
	}

	if m.Bitrate < 0 {
		if m.Bitrate != 0 { // 0 is acceptable for unknown bitrate
			return ErrInvalidRating // Reuse rating error for negative values
		}
	}

	return nil
}
