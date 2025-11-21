package media

import (
	"context"
	"database/sql"

	"github.com/viewra/viewra/internal/domain/common"
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

	// Transaction-aware methods
	DeleteWithTx(ctx context.Context, tx *sql.Tx, id int64) error
	ListByLibraryWithTx(ctx context.Context, tx *sql.Tx, libraryID int64) ([]*Media, error)
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

	// GetTVShowByTitle retrieves a TV show by library ID and title
	GetTVShowByTitle(ctx context.Context, libraryID int64, title string) (TVShow, error)

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

	// ListTVShowIDsByLibraryPaginated retrieves only TV show IDs in a library with pagination (for prefetching)
	ListTVShowIDsByLibraryPaginated(ctx context.Context, libraryID int64, pagination *common.PaginationParams) ([]int64, error)
}

// TVShow represents a TV show for use in repository operations
type TVShow struct {
	ID        int64
	LibraryID int64
	Title     string
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
