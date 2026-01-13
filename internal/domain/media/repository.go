package media

import (
	"context"
	"time"

	"github.com/mantonx/viewra/internal/domain/common"
)

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

	// GetFilePathCache retrieves a map of file_path -> id for all media in a library
	// Memory-efficient: only loads the columns needed for cache lookup
	GetFilePathCache(ctx context.Context, libraryID int64) (map[string]int64, error)

	// Transaction-aware methods
	DeleteWithTx(ctx context.Context, tx common.Transaction, id int64) error
	ListByLibraryWithTx(ctx context.Context, tx common.Transaction, libraryID int64) ([]*Media, error)

	// Audio and subtitle track methods
	InsertAudioTrack(ctx context.Context, track *AudioTrack) error
	GetAudioTracksByMediaID(ctx context.Context, mediaID int64) ([]*AudioTrack, error)
	DeleteAudioTracksByMediaID(ctx context.Context, mediaID int64) error
	InsertSubtitleTrack(ctx context.Context, track *SubtitleTrack) error
	GetSubtitleTracksByMediaID(ctx context.Context, mediaID int64) ([]*SubtitleTrack, error)
	DeleteSubtitleTracksByMediaID(ctx context.Context, mediaID int64) error
	DeleteExternalSubtitlesByMediaID(ctx context.Context, mediaID int64) error
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

	// Pagination support
	// CountMoviesByLibrary returns the total count of movies in a library
	CountMoviesByLibrary(ctx context.Context, libraryID int64) (int64, error)

	// ListMoviesByLibraryPaginated retrieves movies in a library with pagination
	ListMoviesByLibraryPaginated(ctx context.Context, libraryID int64, pagination *common.PaginationParams) ([]*Movie, error)

	// CountSearchMoviesByTitle returns the count of movies matching a search query
	CountSearchMoviesByTitle(ctx context.Context, libraryID int64, query string) (int64, error)

	// SearchMoviesByTitlePaginated searches for movies by title with pagination
	SearchMoviesByTitlePaginated(ctx context.Context, libraryID int64, query string, pagination *common.PaginationParams) ([]*Movie, error)

	// ListMovieIDsByLibraryPaginated retrieves only movie IDs in a library with pagination (for prefetching)
	ListMovieIDsByLibraryPaginated(ctx context.Context, libraryID int64, pagination *common.PaginationParams) ([]int64, error)

	// ListRecentlyAdded returns recently added movies across all libraries
	ListRecentlyAdded(ctx context.Context, limit int) ([]*Movie, error)

	// ListDistinctGenres returns distinct genres across all movies
	ListDistinctGenres(ctx context.Context, limit int) ([]string, error)
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

	// GetTVShowByID retrieves a TV show by its ID
	GetTVShowByID(ctx context.Context, id int64) (TVShow, error)

	// GetTVShowByTitle retrieves a TV show by library ID and title
	GetTVShowByTitle(ctx context.Context, libraryID int64, title string) (TVShow, error)

	// UpdateTVShow updates an existing TV show's metadata
	UpdateTVShow(ctx context.Context, show TVShow) error

	// GetTVSeasonByShowAndNumber retrieves a TV season by show ID and season number
	GetTVSeasonByShowAndNumber(ctx context.Context, showID int64, seasonNumber int64) (TVSeason, error)

	// Pagination support for TV shows
	// CountTVShowsByLibrary returns the total count of TV shows in a library
	CountTVShowsByLibrary(ctx context.Context, libraryID int64) (int64, error)

	// ListTVShowsByLibraryPaginated retrieves TV shows in a library with pagination
	ListTVShowsByLibraryPaginated(ctx context.Context, libraryID int64, pagination *common.PaginationParams) ([]TVShowWithCounts, error)

	// CountSearchTVShowsByTitle returns the count of TV shows matching a search query
	CountSearchTVShowsByTitle(ctx context.Context, libraryID int64, query string) (int64, error)

	// SearchTVShowsByTitlePaginated searches for TV shows by title with pagination
	SearchTVShowsByTitlePaginated(ctx context.Context, libraryID int64, query string, pagination *common.PaginationParams) ([]TVShow, error)

	// SearchTVShowsWithCountsByTitlePaginated searches for TV shows by title with pagination and includes counts
	SearchTVShowsWithCountsByTitlePaginated(ctx context.Context, libraryID int64, query string, pagination *common.PaginationParams) ([]TVShowWithCounts, error)

	// ListTVShowIDsByLibraryPaginated retrieves only TV show IDs in a library with pagination (for prefetching)
	ListTVShowIDsByLibraryPaginated(ctx context.Context, libraryID int64, pagination *common.PaginationParams) ([]int64, error)

	// ListTVSeasonsByShow retrieves all seasons for a specific show
	ListTVSeasonsByShow(ctx context.Context, showID int64) ([]TVSeason, error)

	// ListRecentlyAddedShows returns recently added TV shows across all libraries
	ListRecentlyAddedShows(ctx context.Context, limit int) ([]TVShow, error)
}

// TVShow represents a TV show for use in repository operations
type TVShow struct {
	ID               int64
	LibraryID        int64
	Title            string
	OriginalTitle    string
	SortTitle        string
	Year             int
	FirstAirDate     string // YYYY-MM-DD format (premiere date)
	LastAirDate      string // YYYY-MM-DD format
	Genre            []string
	Plot             string
	Tagline          string
	Status           string // "Returning Series", "Planned", "In Production", "Ended", "Cancelled", "Pilot"
	ContentRating    string
	Network          string
	OriginalLanguage string
	CountryOfOrigin  string
	IMDbID           string
	TMDbID           int
	TVDbID           int
	Directory        string // Path to the show directory (for enrichment/artwork)

	// Ratings
	Rating      float32 // Average rating (0.0 - 10.0)
	RatingVotes int     // Number of votes

	// Timestamps
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TVShowWithCounts represents a TV show with aggregated season and episode counts
type TVShowWithCounts struct {
	TVShow
	SeasonCount  int64
	EpisodeCount int64
}

// TVSeason represents a TV season for use in repository operations
type TVSeason struct {
	ID           int64
	ShowID       int64
	SeasonNumber int64
	EpisodeCount int
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

	// Pagination support for artists
	// CountArtistsByLibrary returns the total count of unique artists in a library
	CountArtistsByLibrary(ctx context.Context, libraryID int64) (int64, error)

	// ListArtistsByLibraryPaginated retrieves unique artists in a library with pagination
	ListArtistsByLibraryPaginated(ctx context.Context, libraryID int64, pagination *common.PaginationParams) ([]MusicArtist, error)

	// Pagination support for albums
	// CountAlbumsByLibrary returns the total count of unique albums in a library
	CountAlbumsByLibrary(ctx context.Context, libraryID int64) (int64, error)

	// ListAlbumsByLibraryPaginated retrieves unique albums in a library with pagination
	ListAlbumsByLibraryPaginated(ctx context.Context, libraryID int64, pagination *common.PaginationParams) ([]MusicAlbum, error)

	// Pagination support for tracks
	// CountMusicTracksByLibrary returns the total count of music tracks in a library
	CountMusicTracksByLibrary(ctx context.Context, libraryID int64) (int64, error)

	// ListMusicTracksByLibraryPaginated retrieves music tracks in a library with pagination
	ListMusicTracksByLibraryPaginated(ctx context.Context, libraryID int64, pagination *common.PaginationParams) ([]*MusicTrack, error)

	// ListArtistIDsByLibraryPaginated retrieves only artist representative IDs in a library with pagination (for prefetching)
	ListArtistIDsByLibraryPaginated(ctx context.Context, libraryID int64, pagination *common.PaginationParams) ([]int64, error)

	// Album entity operations
	// CreateAlbum creates a new album entity
	CreateAlbum(ctx context.Context, album *Album) error

	// GetAlbumByID retrieves an album by its ID
	GetAlbumByID(ctx context.Context, id int64) (*Album, error)

	// FindAlbumByTitle finds an album by library, title, and album artist
	FindAlbumByTitle(ctx context.Context, libraryID int64, title, albumArtist string) (*Album, error)

	// ListAlbumsByLibrary retrieves all album entities in a library
	ListAlbumsByLibrary(ctx context.Context, libraryID int64) ([]*Album, error)

	// ListMusicTracksByAlbumID retrieves all tracks for a specific album ID
	ListMusicTracksByAlbumID(ctx context.Context, albumID int64) ([]*MusicTrack, error)

	// Artist entity operations
	// CreateArtist creates a new artist entity
	CreateArtist(ctx context.Context, artist *Artist) error

	// GetArtistByID retrieves an artist by its ID
	GetArtistByID(ctx context.Context, id int64) (*Artist, error)

	// FindArtistByName finds an artist by library and name
	FindArtistByName(ctx context.Context, libraryID int64, name string) (*Artist, error)

	// ListArtistsByLibrary retrieves all artist entities in a library
	ListArtistsByLibrary(ctx context.Context, libraryID int64) ([]*Artist, error)

	// SearchArtistsByName searches for artists by name with pagination
	SearchArtistsByName(ctx context.Context, libraryID int64, query string, pagination *common.PaginationParams) ([]*Artist, error)

	// CountSearchArtistsByName returns the count of artists matching a search query
	CountSearchArtistsByName(ctx context.Context, libraryID int64, query string) (int64, error)

	// CreateMusicTrackWithEntities atomically creates a music track along with artist and album entities if needed
	// This operation is transactional - all entities are created or none are created
	// Parameters:
	//   - track: The music track to create (with Media, Artist, Album fields populated)
	//   - artist: Optional artist entity (can be nil if track.Artist is empty)
	//   - album: Optional album entity (can be nil if track.Album is empty)
	// Returns the created track with updated IDs for artist and album
	CreateMusicTrackWithEntities(ctx context.Context, track *MusicTrack, artist *Artist, album *Album) error
}

// MusicArtist represents an artist with aggregated track and album counts
type MusicArtist struct {
	RepresentativeID int64
	Artist           string
	AlbumCount       int64
	TrackCount       int64
}

// MusicAlbum represents an album with aggregated track information
type MusicAlbum struct {
	Album       string
	AlbumArtist string
	Year        int64
	TrackCount  int64
	Duration    int64 // Total duration in seconds
}
