package movie

import (
	"context"
	"fmt"

	domainCommon "github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
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

	if err := r.Q().CreateMovie(ctx, buildCreateMovieParams(movie)); err != nil {
		return fmt.Errorf("failed to create movie record: %w", err)
	}

	return nil
}

// GetMovieByID retrieves a movie by its media ID
func (r *Repository) GetMovieByID(ctx context.Context, id int64) (*media.Movie, error) {
	row, err := r.Q().GetMovieByMediaID(ctx, id)
	if err != nil {
		return nil, r.ConvertNotFoundError(err)
	}
	return movieRowToDomain(row), nil
}

// ListMoviesByLibrary retrieves all movies in a specific library
func (r *Repository) ListMoviesByLibrary(ctx context.Context, libraryID int64) ([]*media.Movie, error) {
	rows, err := r.Q().ListMoviesByLibrary(ctx, libraryID)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, listMovieRowToDomain), nil
}

// UpdateMovie modifies an existing movie
func (r *Repository) UpdateMovie(ctx context.Context, movie *media.Movie) error {
	// First, update the base media record
	if err := r.mediaRepo.Update(ctx, &movie.Media); err != nil {
		return fmt.Errorf("failed to update base media record: %w", err)
	}

	if err := r.Q().UpdateMovie(ctx, buildUpdateMovieParams(movie)); err != nil {
		return fmt.Errorf("failed to update movie record: %w", err)
	}

	return nil
}

// SearchMovies searches for movies by title
func (r *Repository) SearchMovies(ctx context.Context, libraryID int64, query string) ([]*media.Movie, error) {
	searchPattern := "%" + query + "%"

	rows, err := r.Q().SearchMoviesByTitle(ctx, unified.SearchMoviesByTitleParams{
		LibraryID:     libraryID,
		Title:         searchPattern,
		OriginalTitle: common.NullString(searchPattern),
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, searchMovieRowToDomain), nil
}

// ListMoviesByGenre retrieves all movies in a specific library with a given genre
func (r *Repository) ListMoviesByGenre(ctx context.Context, libraryID int64, genre string) ([]*media.Movie, error) {
	genrePattern := "%" + genre + "%"

	rows, err := r.Q().ListMoviesByGenre(ctx, unified.ListMoviesByGenreParams{
		LibraryID: libraryID,
		Genre:     common.NullString(genrePattern),
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, genreMovieRowToDomain), nil
}

// ListMoviesByYear retrieves all movies in a specific library from a given year
func (r *Repository) ListMoviesByYear(ctx context.Context, libraryID int64, year int) ([]*media.Movie, error) {
	rows, err := r.Q().ListMoviesByYear(ctx, unified.ListMoviesByYearParams{
		LibraryID: libraryID,
		Year:      common.NullInt64(int64(year)),
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, yearMovieRowToDomain), nil
}

// DeleteMovie removes a movie from the database (does not delete the file)
func (r *Repository) DeleteMovie(ctx context.Context, mediaID int64) error {
	if err := r.Q().DeleteMovie(ctx, mediaID); err != nil {
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
	return r.Q().CountMoviesByLibrary(ctx, libraryID)
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
		rows, err := r.Q().ListMoviesByLibraryPaginatedDesc(ctx, unified.ListMoviesByLibraryPaginatedDescParams{
			LibraryID: libraryID,
			Limit:     int64(pagination.Limit),
			Offset:    int64(pagination.Offset),
		})
		if err != nil {
			return nil, err
		}
		return mapSlice(rows, func(row unified.ListMoviesByLibraryPaginatedDescRow) *media.Movie {
			return listMovieRowToDomain(unified.ListMoviesByLibraryRow(row))
		}), nil
	}

	rows, err := r.Q().ListMoviesByLibraryPaginated(ctx, unified.ListMoviesByLibraryPaginatedParams{
		LibraryID: libraryID,
		Limit:     int64(pagination.Limit),
		Offset:    int64(pagination.Offset),
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, func(row unified.ListMoviesByLibraryPaginatedRow) *media.Movie {
		return listMovieRowToDomain(unified.ListMoviesByLibraryRow(row))
	}), nil
}

// CountSearchMoviesByTitle returns the count of movies matching a search query
func (r *Repository) CountSearchMoviesByTitle(ctx context.Context, libraryID int64, query string) (int64, error) {
	searchPattern := "%" + query + "%"

	return r.Q().CountSearchMoviesByTitle(ctx, unified.CountSearchMoviesByTitleParams{
		LibraryID:     libraryID,
		Title:         searchPattern,
		OriginalTitle: common.NullString(searchPattern),
	})
}

// SearchMoviesByTitlePaginated searches for movies by title with pagination
func (r *Repository) SearchMoviesByTitlePaginated(ctx context.Context, libraryID int64, query string, pagination *domainCommon.PaginationParams) ([]*media.Movie, error) {
	if pagination == nil {
		pagination = domainCommon.DefaultPaginationParams()
	}

	searchPattern := "%" + query + "%"

	rows, err := r.Q().SearchMoviesByTitlePaginated(ctx, unified.SearchMoviesByTitlePaginatedParams{
		LibraryID:     libraryID,
		Title:         searchPattern,
		OriginalTitle: common.NullString(searchPattern),
		Limit:         int64(pagination.Limit),
		Offset:        int64(pagination.Offset),
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, func(row unified.SearchMoviesByTitlePaginatedRow) *media.Movie {
		return searchMovieRowToDomain(unified.SearchMoviesByTitleRow(row))
	}), nil
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
		return r.Q().ListMovieIDsByLibraryPaginatedDesc(ctx, unified.ListMovieIDsByLibraryPaginatedDescParams{
			LibraryID: libraryID,
			Limit:     int64(pagination.Limit),
			Offset:    int64(pagination.Offset),
		})
	}

	return r.Q().ListMovieIDsByLibraryPaginated(ctx, unified.ListMovieIDsByLibraryPaginatedParams{
		LibraryID: libraryID,
		Limit:     int64(pagination.Limit),
		Offset:    int64(pagination.Offset),
	})
}

// ListRecentlyAdded returns recently added movies across all libraries
func (r *Repository) ListRecentlyAdded(ctx context.Context, limit int) ([]*media.Movie, error) {
	rows, err := r.Q().ListRecentlyAddedMovies(ctx, int64(limit))
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, recentlyAddedRowToDomain), nil
}

// ListDistinctGenres returns distinct genres across all movies
func (r *Repository) ListDistinctGenres(ctx context.Context, limit int) ([]string, error) {
	return r.Q().ListDistinctMovieGenres(ctx, int64(limit))
}

// mapSlice converts a slice of one type to another using the provided mapper function.
func mapSlice[TFrom, TTo any](from []TFrom, mapper func(TFrom) TTo) []TTo {
	result := make([]TTo, len(from))
	for i, v := range from {
		result[i] = mapper(v)
	}
	return result
}
