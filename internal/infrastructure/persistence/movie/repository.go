package movie

import (
	"context"
	"fmt"

	domainCommon "github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
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
	err := common.ExecuteCommand(
		r.BaseRepository, ctx,
		func() error {
			return r.Postgres().CreateMovie(ctx, buildPostgresCreateMovieParams(movie))
		},
		func() error {
			return r.SQLite().CreateMovie(ctx, buildSQLiteCreateMovieParams(movie))
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create movie record: %w", err)
	}

	return nil
}

// GetMovieByID retrieves a movie by its media ID
func (r *Repository) GetMovieByID(ctx context.Context, id int64) (*media.Movie, error) {
	return common.QuerySingle(
		r.BaseRepository, ctx,
		func() (sqlc_postgres.GetMovieByMediaIDRow, error) {
			return r.Postgres().GetMovieByMediaID(ctx, int32(id))
		},
		func() (sqlc_sqlite.GetMovieByMediaIDRow, error) {
			return r.SQLite().GetMovieByMediaID(ctx, id)
		},
		postgresMovieToDomain,
		sqliteMovieToDomain,
	)
}

// ListMoviesByLibrary retrieves all movies in a specific library
func (r *Repository) ListMoviesByLibrary(ctx context.Context, libraryID int64) ([]*media.Movie, error) {
	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.ListMoviesByLibraryRow, error) {
			return r.Postgres().ListMoviesByLibrary(ctx, int32(libraryID))
		},
		func() ([]sqlc_sqlite.ListMoviesByLibraryRow, error) {
			return r.SQLite().ListMoviesByLibrary(ctx, libraryID)
		},
		postgresListMovieToDomain,
		sqliteListMovieToDomain,
	)
}

// UpdateMovie modifies an existing movie
func (r *Repository) UpdateMovie(ctx context.Context, movie *media.Movie) error {
	// First, update the base media record
	if err := r.mediaRepo.Update(ctx, &movie.Media); err != nil {
		return fmt.Errorf("failed to update base media record: %w", err)
	}

	// Then, update the movie-specific record
	err := common.ExecuteCommand(
		r.BaseRepository, ctx,
		func() error {
			return r.Postgres().UpdateMovie(ctx, buildPostgresUpdateMovieParams(movie))
		},
		func() error {
			return r.SQLite().UpdateMovie(ctx, buildSQLiteUpdateMovieParams(movie))
		},
	)
	if err != nil {
		return fmt.Errorf("failed to update movie record: %w", err)
	}

	return nil
}

// SearchMovies searches for movies by title
func (r *Repository) SearchMovies(ctx context.Context, libraryID int64, query string) ([]*media.Movie, error) {
	searchPattern := "%" + query + "%"

	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.SearchMoviesByTitleRow, error) {
			return r.Postgres().SearchMoviesByTitle(ctx, sqlc_postgres.SearchMoviesByTitleParams{
				LibraryID:     int32(libraryID),
				Title:         searchPattern,
				OriginalTitle: common.NullString(searchPattern),
			})
		},
		func() ([]sqlc_sqlite.SearchMoviesByTitleRow, error) {
			return r.SQLite().SearchMoviesByTitle(ctx, sqlc_sqlite.SearchMoviesByTitleParams{
				LibraryID:     libraryID,
				Title:         searchPattern,
				OriginalTitle: common.NullString(searchPattern),
			})
		},
		postgresSearchMovieToDomain,
		sqliteSearchMovieToDomain,
	)
}

// ListMoviesByGenre retrieves all movies in a specific library with a given genre
func (r *Repository) ListMoviesByGenre(ctx context.Context, libraryID int64, genre string) ([]*media.Movie, error) {
	genrePattern := "%" + genre + "%"

	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.ListMoviesByGenreRow, error) {
			return r.Postgres().ListMoviesByGenre(ctx, sqlc_postgres.ListMoviesByGenreParams{
				LibraryID: int32(libraryID),
				Genre:     common.NullString(genrePattern),
			})
		},
		func() ([]sqlc_sqlite.ListMoviesByGenreRow, error) {
			return r.SQLite().ListMoviesByGenre(ctx, sqlc_sqlite.ListMoviesByGenreParams{
				LibraryID: libraryID,
				Genre:     common.NullString(genrePattern),
			})
		},
		postgresGenreMovieToDomain,
		sqliteGenreMovieToDomain,
	)
}

// ListMoviesByYear retrieves all movies in a specific library from a given year
func (r *Repository) ListMoviesByYear(ctx context.Context, libraryID int64, year int) ([]*media.Movie, error) {
	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.ListMoviesByYearRow, error) {
			return r.Postgres().ListMoviesByYear(ctx, sqlc_postgres.ListMoviesByYearParams{
				LibraryID: int32(libraryID),
				Year:      common.NullInt32FromInt64(int64(year)),
			})
		},
		func() ([]sqlc_sqlite.ListMoviesByYearRow, error) {
			return r.SQLite().ListMoviesByYear(ctx, sqlc_sqlite.ListMoviesByYearParams{
				LibraryID: libraryID,
				Year:      common.NullInt64(int64(year)),
			})
		},
		postgresYearMovieToDomain,
		sqliteYearMovieToDomain,
	)
}

// DeleteMovie removes a movie from the database (does not delete the file)
func (r *Repository) DeleteMovie(ctx context.Context, mediaID int64) error {
	err := common.ExecuteCommand(
		r.BaseRepository, ctx,
		func() error {
			return r.Postgres().DeleteMovie(ctx, int32(mediaID))
		},
		func() error {
			return r.SQLite().DeleteMovie(ctx, mediaID)
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
	return common.QueryScalar(
		r.BaseRepository, ctx,
		func() (int64, error) {
			return r.Postgres().CountMoviesByLibrary(ctx, int32(libraryID))
		},
		func() (int64, error) {
			return r.SQLite().CountMoviesByLibrary(ctx, libraryID)
		},
	)
}

// ListMoviesByLibraryPaginated retrieves movies in a library with pagination
func (r *Repository) ListMoviesByLibraryPaginated(ctx context.Context, libraryID int64, pagination *domainCommon.PaginationParams) ([]*media.Movie, error) {
	if pagination == nil {
		pagination = domainCommon.DefaultPaginationParams()
	}

	sortBy := pagination.SortBy
	if sortBy == "" {
		sortBy = "title_asc"
	}

	if sortBy == "title_desc" {
		return common.QueryMany(
			r.BaseRepository, ctx,
			func() ([]sqlc_postgres.ListMoviesByLibraryPaginatedDescRow, error) {
				return r.Postgres().ListMoviesByLibraryPaginatedDesc(ctx, sqlc_postgres.ListMoviesByLibraryPaginatedDescParams{
					LibraryID: int32(libraryID),
					Limit:     int32(pagination.Limit),
					Offset:    int32(pagination.Offset),
				})
			},
			func() ([]sqlc_sqlite.ListMoviesByLibraryPaginatedDescRow, error) {
				return r.SQLite().ListMoviesByLibraryPaginatedDesc(ctx, sqlc_sqlite.ListMoviesByLibraryPaginatedDescParams{
					LibraryID: libraryID,
					Limit:     int64(pagination.Limit),
					Offset:    int64(pagination.Offset),
				})
			},
			func(row sqlc_postgres.ListMoviesByLibraryPaginatedDescRow) *media.Movie {
				return postgresListMovieToDomain(sqlc_postgres.ListMoviesByLibraryRow(row))
			},
			func(row sqlc_sqlite.ListMoviesByLibraryPaginatedDescRow) *media.Movie {
				return sqliteListMovieToDomain(sqlc_sqlite.ListMoviesByLibraryRow(row))
			},
		)
	}

	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.ListMoviesByLibraryPaginatedRow, error) {
			return r.Postgres().ListMoviesByLibraryPaginated(ctx, sqlc_postgres.ListMoviesByLibraryPaginatedParams{
				LibraryID: int32(libraryID),
				Limit:     int32(pagination.Limit),
				Offset:    int32(pagination.Offset),
			})
		},
		func() ([]sqlc_sqlite.ListMoviesByLibraryPaginatedRow, error) {
			return r.SQLite().ListMoviesByLibraryPaginated(ctx, sqlc_sqlite.ListMoviesByLibraryPaginatedParams{
				LibraryID: libraryID,
				Limit:     int64(pagination.Limit),
				Offset:    int64(pagination.Offset),
			})
		},
		func(row sqlc_postgres.ListMoviesByLibraryPaginatedRow) *media.Movie {
			return postgresListMovieToDomain(sqlc_postgres.ListMoviesByLibraryRow(row))
		},
		func(row sqlc_sqlite.ListMoviesByLibraryPaginatedRow) *media.Movie {
			return sqliteListMovieToDomain(sqlc_sqlite.ListMoviesByLibraryRow(row))
		},
	)
}

// CountSearchMoviesByTitle returns the count of movies matching a search query
func (r *Repository) CountSearchMoviesByTitle(ctx context.Context, libraryID int64, query string) (int64, error) {
	searchPattern := "%" + query + "%"

	return common.QueryScalar(
		r.BaseRepository, ctx,
		func() (int64, error) {
			return r.Postgres().CountSearchMoviesByTitle(ctx, sqlc_postgres.CountSearchMoviesByTitleParams{
				LibraryID:     int32(libraryID),
				Title:         searchPattern,
				OriginalTitle: common.NullString(searchPattern),
			})
		},
		func() (int64, error) {
			return r.SQLite().CountSearchMoviesByTitle(ctx, sqlc_sqlite.CountSearchMoviesByTitleParams{
				LibraryID:     libraryID,
				Title:         searchPattern,
				OriginalTitle: common.NullString(searchPattern),
			})
		},
	)
}

// SearchMoviesByTitlePaginated searches for movies by title with pagination
func (r *Repository) SearchMoviesByTitlePaginated(ctx context.Context, libraryID int64, query string, pagination *domainCommon.PaginationParams) ([]*media.Movie, error) {
	if pagination == nil {
		pagination = domainCommon.DefaultPaginationParams()
	}

	searchPattern := "%" + query + "%"

	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.SearchMoviesByTitlePaginatedRow, error) {
			return r.Postgres().SearchMoviesByTitlePaginated(ctx, sqlc_postgres.SearchMoviesByTitlePaginatedParams{
				LibraryID:     int32(libraryID),
				Title:         searchPattern,
				OriginalTitle: common.NullString(searchPattern),
				Limit:         int32(pagination.Limit),
				Offset:        int32(pagination.Offset),
			})
		},
		func() ([]sqlc_sqlite.SearchMoviesByTitlePaginatedRow, error) {
			return r.SQLite().SearchMoviesByTitlePaginated(ctx, sqlc_sqlite.SearchMoviesByTitlePaginatedParams{
				LibraryID:     libraryID,
				Title:         searchPattern,
				OriginalTitle: common.NullString(searchPattern),
				Limit:         int64(pagination.Limit),
				Offset:        int64(pagination.Offset),
			})
		},
		func(row sqlc_postgres.SearchMoviesByTitlePaginatedRow) *media.Movie {
			return postgresSearchMovieToDomain(sqlc_postgres.SearchMoviesByTitleRow(row))
		},
		func(row sqlc_sqlite.SearchMoviesByTitlePaginatedRow) *media.Movie {
			return sqliteSearchMovieToDomain(sqlc_sqlite.SearchMoviesByTitleRow(row))
		},
	)
}

// ListMovieIDsByLibraryPaginated retrieves only movie IDs in a library with pagination
func (r *Repository) ListMovieIDsByLibraryPaginated(ctx context.Context, libraryID int64, pagination *domainCommon.PaginationParams) ([]int64, error) {
	if pagination == nil {
		pagination = domainCommon.DefaultPaginationParams()
	}

	sortBy := pagination.SortBy
	if sortBy == "" {
		sortBy = "title_asc"
	}

	if sortBy == "title_desc" {
		return common.QueryMany(
			r.BaseRepository, ctx,
			func() ([]int64, error) {
				ids, err := r.Postgres().ListMovieIDsByLibraryPaginatedDesc(ctx, sqlc_postgres.ListMovieIDsByLibraryPaginatedDescParams{
					LibraryID: int32(libraryID),
					Limit:     int32(pagination.Limit),
					Offset:    int32(pagination.Offset),
				})
				if err != nil {
					return nil, err
				}
				// Convert int32 to int64
				result := make([]int64, len(ids))
				for i, id := range ids {
					result[i] = int64(id)
				}
				return result, nil
			},
			func() ([]int64, error) {
				return r.SQLite().ListMovieIDsByLibraryPaginatedDesc(ctx, sqlc_sqlite.ListMovieIDsByLibraryPaginatedDescParams{
					LibraryID: libraryID,
					Limit:     int64(pagination.Limit),
					Offset:    int64(pagination.Offset),
				})
			},
			func(id int64) int64 { return id },
			func(id int64) int64 { return id },
		)
	}

	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]int64, error) {
			ids, err := r.Postgres().ListMovieIDsByLibraryPaginated(ctx, sqlc_postgres.ListMovieIDsByLibraryPaginatedParams{
				LibraryID: int32(libraryID),
				Limit:     int32(pagination.Limit),
				Offset:    int32(pagination.Offset),
			})
			if err != nil {
				return nil, err
			}
			// Convert int32 to int64
			result := make([]int64, len(ids))
			for i, id := range ids {
				result[i] = int64(id)
			}
			return result, nil
		},
		func() ([]int64, error) {
			return r.SQLite().ListMovieIDsByLibraryPaginated(ctx, sqlc_sqlite.ListMovieIDsByLibraryPaginatedParams{
				LibraryID: libraryID,
				Limit:     int64(pagination.Limit),
				Offset:    int64(pagination.Offset),
			})
		},
		func(id int64) int64 { return id },
		func(id int64) int64 { return id },
	)
}
