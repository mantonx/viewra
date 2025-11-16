package media

import "context"

// Repository defines the interface for media data access operations.
// This follows the Repository pattern from DDD, keeping domain logic
// separate from infrastructure concerns.
type Repository interface {
	// Create adds a new media item to the repository
	Create(ctx context.Context, media *Media) error

	// GetByID retrieves a media item by its ID
	GetByID(ctx context.Context, id int64) (*Media, error)

	// GetByFilePath retrieves a media item by its file path within a library
	GetByFilePath(ctx context.Context, libraryID int64, filePath string) (*Media, error)

	// ListAll retrieves all media items across all libraries
	ListAll(ctx context.Context) ([]*Media, error)

	// ListByLibrary retrieves all media items in a specific library
	ListByLibrary(ctx context.Context, libraryID int64) ([]*Media, error)

	// ListByType retrieves all media items of a specific type in a library
	ListByType(ctx context.Context, libraryID int64, mediaType MediaType) ([]*Media, error)

	// Update modifies an existing media item
	Update(ctx context.Context, media *Media) error

	// Delete removes a media item from the repository
	Delete(ctx context.Context, id int64) error

	// ExistsInLibrary checks if a media item with the given file path exists in the library
	ExistsInLibrary(ctx context.Context, libraryID int64, filePath string) (bool, error)

	// Count returns the total number of media items in a library
	Count(ctx context.Context, libraryID int64) (int64, error)

	// CountByType returns the number of media items of a specific type in a library
	CountByType(ctx context.Context, libraryID int64, mediaType MediaType) (int64, error)
}

// MovieRepository extends Repository with movie-specific operations
type MovieRepository interface {
	// CreateMovie adds a new movie to the repository
	CreateMovie(ctx context.Context, movie *Movie) error

	// GetMovieByID retrieves a movie by its ID
	GetMovieByID(ctx context.Context, id int64) (*Movie, error)

	// ListMoviesByLibrary retrieves all movies in a specific library
	ListMoviesByLibrary(ctx context.Context, libraryID int64) ([]*Movie, error)

	// UpdateMovie modifies an existing movie
	UpdateMovie(ctx context.Context, movie *Movie) error

	// SearchMovies searches for movies by title
	SearchMovies(ctx context.Context, libraryID int64, query string) ([]*Movie, error)
}

// TVRepository extends Repository with TV-specific operations
type TVRepository interface {
	// CreateTVEpisode adds a new TV episode to the repository
	CreateTVEpisode(ctx context.Context, episode *TVEpisode) error

	// GetTVEpisodeByID retrieves a TV episode by its ID
	GetTVEpisodeByID(ctx context.Context, id int64) (*TVEpisode, error)

	// ListTVEpisodesByLibrary retrieves all TV episodes in a specific library
	ListTVEpisodesByLibrary(ctx context.Context, libraryID int64) ([]*TVEpisode, error)

	// ListTVEpisodesByShow retrieves all episodes of a specific show by title
	ListTVEpisodesByShow(ctx context.Context, libraryID int64, showTitle string) ([]*TVEpisode, error)

	// ListTVEpisodesByShowID retrieves all episodes of a specific show by ID
	ListTVEpisodesByShowID(ctx context.Context, showID int64) ([]*TVEpisode, error)

	// UpdateTVEpisode modifies an existing TV episode
	UpdateTVEpisode(ctx context.Context, episode *TVEpisode) error

	// SearchTVEpisodes searches for TV episodes by show title or episode title
	SearchTVEpisodes(ctx context.Context, libraryID int64, query string) ([]*TVEpisode, error)
}

// MusicRepository extends Repository with music-specific operations
type MusicRepository interface {
	// CreateMusicTrack adds a new music track to the repository
	CreateMusicTrack(ctx context.Context, track *MusicTrack) error

	// GetMusicTrackByID retrieves a music track by its ID
	GetMusicTrackByID(ctx context.Context, id int64) (*MusicTrack, error)

	// ListMusicTracksByLibrary retrieves all music tracks in a specific library
	ListMusicTracksByLibrary(ctx context.Context, libraryID int64) ([]*MusicTrack, error)

	// ListMusicTracksByAlbum retrieves all tracks from a specific album
	ListMusicTracksByAlbum(ctx context.Context, libraryID int64, album string) ([]*MusicTrack, error)

	// ListMusicTracksByArtist retrieves all tracks by a specific artist
	ListMusicTracksByArtist(ctx context.Context, libraryID int64, artist string) ([]*MusicTrack, error)

	// UpdateMusicTrack modifies an existing music track
	UpdateMusicTrack(ctx context.Context, track *MusicTrack) error

	// SearchMusicTracks searches for music tracks by title, artist, or album
	SearchMusicTracks(ctx context.Context, libraryID int64, query string) ([]*MusicTrack, error)
}
