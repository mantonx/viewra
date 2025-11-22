package movie

import (
	"context"
	"fmt"

	domainCommon "github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
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

// CountMoviesByLibrary returns the total count of movies in a library
func (r *Repository) CountMoviesByLibrary(ctx context.Context, libraryID int64) (int64, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().CountMoviesByLibrary(ctx, libraryID)
		},
	)
	if err != nil {
		return 0, err
	}

	if r.Router().IsPostgresDB() {
		return 0, r.PostgresNotImplemented()
	}

	return result.(int64), nil
}

// ListMoviesByLibraryPaginated retrieves movies in a library with pagination
func (r *Repository) ListMoviesByLibraryPaginated(ctx context.Context, libraryID int64, pagination *domainCommon.PaginationParams) ([]*media.Movie, error) {
	if pagination == nil {
		pagination = domainCommon.DefaultPaginationParams()
	}

	// Default to title_asc if not specified
	sortBy := pagination.SortBy
	if sortBy == "" {
		sortBy = "title_asc"
	}

	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			if sortBy == "title_desc" {
				return r.SQLite().ListMoviesByLibraryPaginatedDesc(ctx, sqlc_sqlite.ListMoviesByLibraryPaginatedDescParams{
					LibraryID: libraryID,
					Limit:     int64(pagination.Limit),
					Offset:    int64(pagination.Offset),
				})
			}
			return r.SQLite().ListMoviesByLibraryPaginated(ctx, sqlc_sqlite.ListMoviesByLibraryPaginatedParams{
				LibraryID: libraryID,
				Limit:     int64(pagination.Limit),
				Offset:    int64(pagination.Offset),
			})
		},
	)
	if err != nil {
		return nil, err
	}

	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	// Handle different row types based on sort order
	var movies []*media.Movie
	if sortBy == "title_desc" {
		sqResults := result.([]sqlc_sqlite.ListMoviesByLibraryPaginatedDescRow)
		movies = make([]*media.Movie, len(sqResults))
		for i, descRow := range sqResults {
			// Convert DescRow to regular Row (they have identical fields)
			row := sqlc_sqlite.ListMoviesByLibraryRow(descRow)
			movies[i] = sqliteListMovieToDomain(row)
		}
	} else {
		sqResults := result.([]sqlc_sqlite.ListMoviesByLibraryPaginatedRow)
		movies = make([]*media.Movie, len(sqResults))
		for i, row := range sqResults {
			movies[i] = sqliteListMovieToDomain(sqlc_sqlite.ListMoviesByLibraryRow(row))
		}
	}
	return movies, nil
}

// CountSearchMoviesByTitle returns the count of movies matching a search query
func (r *Repository) CountSearchMoviesByTitle(ctx context.Context, libraryID int64, query string) (int64, error) {
	searchPattern := "%" + query + "%"

	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().CountSearchMoviesByTitle(ctx, sqlc_sqlite.CountSearchMoviesByTitleParams{
				LibraryID:     libraryID,
				Title:         searchPattern,
				OriginalTitle: common.NullString(searchPattern),
			})
		},
	)
	if err != nil {
		return 0, err
	}

	if r.Router().IsPostgresDB() {
		return 0, r.PostgresNotImplemented()
	}

	return result.(int64), nil
}

// SearchMoviesByTitlePaginated searches for movies by title with pagination
func (r *Repository) SearchMoviesByTitlePaginated(ctx context.Context, libraryID int64, query string, pagination *domainCommon.PaginationParams) ([]*media.Movie, error) {
	if pagination == nil {
		pagination = domainCommon.DefaultPaginationParams()
	}

	searchPattern := "%" + query + "%"

	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().SearchMoviesByTitlePaginated(ctx, sqlc_sqlite.SearchMoviesByTitlePaginatedParams{
				LibraryID:     libraryID,
				Title:         searchPattern,
				OriginalTitle: common.NullString(searchPattern),
				Limit:         int64(pagination.Limit),
				Offset:        int64(pagination.Offset),
			})
		},
	)
	if err != nil {
		return nil, err
	}

	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	sqResults := result.([]sqlc_sqlite.SearchMoviesByTitlePaginatedRow)
	movies := make([]*media.Movie, len(sqResults))
	for i, row := range sqResults {
		movies[i] = sqliteSearchMovieToDomain(sqlc_sqlite.SearchMoviesByTitleRow(row))
	}
	return movies, nil
}

// ListMovieIDsByLibraryPaginated retrieves only movie IDs in a library with pagination
func (r *Repository) ListMovieIDsByLibraryPaginated(ctx context.Context, libraryID int64, pagination *domainCommon.PaginationParams) ([]int64, error) {
	if pagination == nil {
		pagination = domainCommon.DefaultPaginationParams()
	}

	// Determine sort order from pagination params
	sortBy := pagination.SortBy
	if sortBy == "" {
		sortBy = "title_asc"
	}

	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			// Choose the appropriate query based on sort order
			if sortBy == "title_desc" {
				return r.SQLite().ListMovieIDsByLibraryPaginatedDesc(ctx, sqlc_sqlite.ListMovieIDsByLibraryPaginatedDescParams{
					LibraryID: libraryID,
					Limit:     int64(pagination.Limit),
					Offset:    int64(pagination.Offset),
				})
			}
			return r.SQLite().ListMovieIDsByLibraryPaginated(ctx, sqlc_sqlite.ListMovieIDsByLibraryPaginatedParams{
				LibraryID: libraryID,
				Limit:     int64(pagination.Limit),
				Offset:    int64(pagination.Offset),
			})
		},
	)
	if err != nil {
		return nil, err
	}

	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	return result.([]int64), nil
}
