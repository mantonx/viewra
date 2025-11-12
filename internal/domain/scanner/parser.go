package scanner

// MovieInfo contains metadata parsed from a movie filename
type MovieInfo struct {
	// Title is the movie name
	Title string

	// Year is the release year (optional, 0 if not found)
	Year int

	// Resolution is the video resolution (e.g., "720p", "1080p", "4K")
	Resolution string

	// Quality is the source quality (e.g., "BluRay", "WEB-DL", "HDTV")
	Quality string
}

// TVEpisodeInfo contains metadata parsed from a TV show filename
type TVEpisodeInfo struct {
	// ShowName is the TV show title
	ShowName string

	// Season is the season number (1-99)
	Season int

	// Episode is the episode number (1-999)
	Episode int

	// EpisodeEnd is the last episode number for multi-episode files (0 if single episode)
	EpisodeEnd int

	// EpisodeTitle is the episode name (optional)
	EpisodeTitle string

	// Year is the release year (optional, 0 if not found)
	Year int
}

// MusicInfo contains metadata for a music track
type MusicInfo struct {
	// Title is the track title
	Title string

	// Artist is the track artist
	Artist string

	// Album is the album name
	Album string

	// AlbumArtist is the album artist (may differ from track artist)
	AlbumArtist string

	// TrackNumber is the track number on the album (0 if not found)
	TrackNumber int

	// Year is the release year (0 if not found)
	Year int

	// Genre is the music genre
	Genre string

	// Duration is the track length in seconds (0 if not found)
	Duration int
}

// FilenameParser parses metadata from media filenames
type FilenameParser interface {
	// ParseMovie extracts movie metadata from a filename
	ParseMovie(filename string) (*MovieInfo, error)

	// ParseTVEpisode extracts TV show metadata from a filename
	ParseTVEpisode(filename string) (*TVEpisodeInfo, error)

	// ParseMusic extracts music metadata from a file (ID3 tags + filename fallback)
	ParseMusic(path string) (*MusicInfo, error)
}
