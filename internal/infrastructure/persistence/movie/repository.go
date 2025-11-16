package movie

import (
	"context"
	"fmt"

	"github.com/viewra/viewra/internal/domain/media"
	"github.com/viewra/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/viewra/viewra/internal/infrastructure/persistence/common"
)

// Repository implements media.MovieRepository using sqlc.
// It embeds BaseRepository for dual-database support.
type Repository struct {
	*common.BaseRepository
	mediaRepo media.Repository
}

// NewRepository creates a new movie repository with the specified database driver.
// The driver parameter should be "sqlite", "sqlite3", "postgres", or "postgresql".
func NewRepository(db *common.BaseRepository, mediaRepo media.Repository) *Repository {
	return &Repository{
		BaseRepository: db,
		mediaRepo:      mediaRepo,
	}
}

// CreateMovie adds a new movie to the repository
func (r *Repository) CreateMovie(ctx context.Context, movie *media.Movie) error {
	// First, create the base media record
	if err := r.mediaRepo.Create(ctx, &movie.Media); err != nil {
		return fmt.Errorf("failed to create base media record: %w", err)
	}

	// Then, create the movie-specific record
	_, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return nil, r.SQLite().CreateMovie(ctx, buildSQLiteCreateMovieParams(movie))
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create movie record: %w", err)
	}

	return nil
}

// GetMovieByID retrieves a movie by its media ID
func (r *Repository) GetMovieByID(ctx context.Context, id int64) (*media.Movie, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().GetMovieByMediaID(ctx, id)
		},
	)
	if err != nil {
		return nil, r.ConvertNotFoundError(err)
	}

	// Convert to domain movie
	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}
	return sqliteMovieToDomain(result.(sqlc_sqlite.GetMovieByMediaIDRow)), nil
}

// ListMoviesByLibrary retrieves all movies in a specific library
func (r *Repository) ListMoviesByLibrary(ctx context.Context, libraryID int64) ([]*media.Movie, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().ListMoviesByLibrary(ctx, libraryID)
		},
	)
	if err != nil {
		return nil, err
	}

	// Convert to domain movies
	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	sqResults := result.([]sqlc_sqlite.ListMoviesByLibraryRow)
	movies := make([]*media.Movie, len(sqResults))
	for i, row := range sqResults {
		movies[i] = sqliteListMovieToDomain(row)
	}
	return movies, nil
}

// UpdateMovie modifies an existing movie
func (r *Repository) UpdateMovie(ctx context.Context, movie *media.Movie) error {
	// First, update the base media record
	if err := r.mediaRepo.Update(ctx, &movie.Media); err != nil {
		return fmt.Errorf("failed to update base media record: %w", err)
	}

	// Then, update the movie-specific record
	_, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return nil, r.SQLite().UpdateMovie(ctx, buildSQLiteUpdateMovieParams(movie))
		},
	)
	if err != nil {
		return fmt.Errorf("failed to update movie record: %w", err)
	}

	return nil
}

// SearchMovies searches for movies by title
func (r *Repository) SearchMovies(ctx context.Context, libraryID int64, query string) ([]*media.Movie, error) {
	// Add wildcards for LIKE search
	searchPattern := "%" + query + "%"

	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().SearchMoviesByTitle(ctx, sqlc_sqlite.SearchMoviesByTitleParams{
				LibraryID:     libraryID,
				Title:         searchPattern,
				OriginalTitle: common.NullString(searchPattern),
			})
		},
	)
	if err != nil {
		return nil, err
	}

	// Convert to domain movies
	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	sqResults := result.([]sqlc_sqlite.SearchMoviesByTitleRow)
	movies := make([]*media.Movie, len(sqResults))
	for i, row := range sqResults {
		movies[i] = sqliteSearchMovieToDomain(row)
	}
	return movies, nil
}

// ListMoviesByGenre retrieves all movies in a specific library with a given genre
func (r *Repository) ListMoviesByGenre(ctx context.Context, libraryID int64, genre string) ([]*media.Movie, error) {
	// Add wildcards for LIKE search
	genrePattern := "%" + genre + "%"

	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().ListMoviesByGenre(ctx, sqlc_sqlite.ListMoviesByGenreParams{
				LibraryID: libraryID,
				Genre:     common.NullString(genrePattern),
			})
		},
	)
	if err != nil {
		return nil, err
	}

	// Convert to domain movies
	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	sqResults := result.([]sqlc_sqlite.ListMoviesByGenreRow)
	movies := make([]*media.Movie, len(sqResults))
	for i, row := range sqResults {
		movies[i] = sqliteGenreMovieToDomain(row)
	}
	return movies, nil
}

// ListMoviesByYear retrieves all movies in a specific library from a given year
func (r *Repository) ListMoviesByYear(ctx context.Context, libraryID int64, year int) ([]*media.Movie, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().ListMoviesByYear(ctx, sqlc_sqlite.ListMoviesByYearParams{
				LibraryID: libraryID,
				Year:      common.NullInt64(int64(year)),
			})
		},
	)
	if err != nil {
		return nil, err
	}

	// Convert to domain movies
	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	sqResults := result.([]sqlc_sqlite.ListMoviesByYearRow)
	movies := make([]*media.Movie, len(sqResults))
	for i, row := range sqResults {
		movies[i] = sqliteYearMovieToDomain(row)
	}
	return movies, nil
}

// DeleteMovie removes a movie from the database (does not delete the file)
func (r *Repository) DeleteMovie(ctx context.Context, mediaID int64) error {
	_, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return nil, r.SQLite().DeleteMovie(ctx, mediaID)
		},
	)
	if err != nil {
		return fmt.Errorf("failed to delete movie record: %w", err)
	}

	// Also delete the base media record
	if err := r.mediaRepo.Delete(ctx, mediaID); err != nil {
		return fmt.Errorf("failed to delete base media record: %w", err)
	}

	return nil
}
