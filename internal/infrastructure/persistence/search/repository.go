package search

import (
	"context"
	"strings"

	"github.com/mantonx/viewra/internal/domain/search"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// Repository implements search.Repository using sqlc.
type Repository struct {
	*common.BaseRepository
}

// NewRepository creates a new search repository.
func NewRepository(db *common.BaseRepository) *Repository {
	return &Repository{
		BaseRepository: db,
	}
}

// Search performs a global text search across all media.
func (r *Repository) Search(ctx context.Context, req *search.Request) ([]search.Result, error) {
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 100 {
		req.Limit = 100
	}

	query := strings.TrimSpace(req.Query)
	if query == "" {
		return []search.Result{}, nil
	}

	searchPattern := "%" + query + "%"
	results := make([]search.Result, 0)

	// Check which media types to search
	searchMovies := len(req.MediaTypes) == 0 || contains(req.MediaTypes, "movie")
	searchTVShows := len(req.MediaTypes) == 0 || contains(req.MediaTypes, "tv_show")

	// Search movies
	if searchMovies {
		movieResults, err := r.searchMovies(ctx, searchPattern, req.Limit)
		if err != nil {
			return nil, err
		}
		results = append(results, movieResults...)
	}

	// Search TV shows
	if searchTVShows {
		tvResults, err := r.searchTVShows(ctx, searchPattern, req.Limit)
		if err != nil {
			return nil, err
		}
		results = append(results, tvResults...)
	}

	// Sort by score and limit
	sortByScore(results)
	if len(results) > req.Limit {
		results = results[:req.Limit]
	}

	return results, nil
}

func (r *Repository) searchMovies(ctx context.Context, pattern string, limit int) ([]search.Result, error) {
	rows, err := r.Q().SearchMoviesGlobal(ctx, unified.SearchMoviesGlobalParams{
		Title:         pattern,
		OriginalTitle: common.NullString(pattern),
		Limit:         int64(limit),
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, movieRowToResult), nil
}

func (r *Repository) searchTVShows(ctx context.Context, pattern string, limit int) ([]search.Result, error) {
	rows, err := r.Q().SearchTVShowsGlobal(ctx, unified.SearchTVShowsGlobalParams{
		Title:         pattern,
		OriginalTitle: common.NullString(pattern),
		Limit:         int64(limit),
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, tvShowRowToResult), nil
}

func movieRowToResult(row unified.SearchMoviesGlobalRow) search.Result {
	year := 0
	if row.Year.Valid {
		year = int(row.Year.Int64)
	}
	return search.Result{
		ID:        row.MediaID,
		MediaType: "movie",
		Title:     row.Title,
		Year:      year,
		LibraryID: row.LibraryID,
		Score:     1.0, // Will be recalculated
	}
}

func tvShowRowToResult(row unified.SearchTVShowsGlobalRow) search.Result {
	year := 0
	if row.Year.Valid {
		year = int(row.Year.Int64)
	}
	return search.Result{
		ID:        row.ID,
		MediaType: "tv_show",
		Title:     row.Title,
		Year:      year,
		LibraryID: row.LibraryID,
		Score:     1.0, // Will be recalculated
	}
}

func mapSlice[TFrom, TTo any](from []TFrom, mapper func(TFrom) TTo) []TTo {
	result := make([]TTo, len(from))
	for i, v := range from {
		result[i] = mapper(v)
	}
	return result
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func sortByScore(results []search.Result) {
	// Simple insertion sort - results are small
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].Score > results[j-1].Score; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}
}
