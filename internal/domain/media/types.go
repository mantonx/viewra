package media

// MediaType represents the type of media (movie, tv episode, music track)
type MediaType string

const (
	// MediaTypeMovie represents a movie file
	MediaTypeMovie MediaType = "movie"

	// MediaTypeTV represents a TV episode file
	MediaTypeTV MediaType = "tv_episode"

	// MediaTypeMusic represents a music track file
	MediaTypeMusic MediaType = "music_track"
)

// IsValid checks if the media type is a valid value
func (mt MediaType) IsValid() bool {
	switch mt {
	case MediaTypeMovie, MediaTypeTV, MediaTypeMusic:
		return true
	default:
		return false
	}
}

// String returns the string representation of the media type
func (mt MediaType) String() string {
	return string(mt)
}
