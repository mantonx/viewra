package plugins

import (
	"context"
	"database/sql"

	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
)

// searchMovies searches for movies by title pattern across all libraries.
func (q *DBMediaQuerier) searchMovies(ctx context.Context, pattern string, year int, limit int) ([]*MediaInfo, error) {
	results, err := q.router.Route(
		func() (any, error) {
			return q.postgres.SearchMoviesGlobal(ctx, sqlc_postgres.SearchMoviesGlobalParams{
				Title:         pattern,
				OriginalTitle: sql.NullString{String: pattern, Valid: true},
				Limit:         int32(limit),
			})
		},
		func() (any, error) {
			return q.sqlite.SearchMoviesGlobal(ctx, sqlc_sqlite.SearchMoviesGlobalParams{
				Title:         pattern,
				OriginalTitle: sql.NullString{String: pattern, Valid: true},
				Limit:         int64(limit),
			})
		},
	)
	if err != nil {
		return nil, err
	}

	return q.movieResultsToInfo(results, year), nil
}

// movieResultsToInfo converts movie search results to MediaInfo slice.
func (q *DBMediaQuerier) movieResultsToInfo(results any, yearFilter int) []*MediaInfo {
	var infos []*MediaInfo

	if q.router.IsPostgresDB() {
		for _, row := range results.([]sqlc_postgres.SearchMoviesGlobalRow) {
			year := 0
			if row.Year.Valid {
				year = int(row.Year.Int32)
			}
			// Filter by year if specified
			if yearFilter > 0 && year != yearFilter {
				continue
			}
			infos = append(infos, &MediaInfo{
				ID:        int64(row.MediaID),
				MediaType: "movie",
				Title:     row.Title,
				Year:      year,
				FilePath:  row.FilePath,
				LibraryID: int64(row.LibraryID),
			})
		}
	} else {
		for _, row := range results.([]sqlc_sqlite.SearchMoviesGlobalRow) {
			year := 0
			if row.Year.Valid {
				year = int(row.Year.Int64)
			}
			// Filter by year if specified
			if yearFilter > 0 && year != yearFilter {
				continue
			}
			infos = append(infos, &MediaInfo{
				ID:        row.MediaID,
				MediaType: "movie",
				Title:     row.Title,
				Year:      year,
				FilePath:  row.FilePath,
				LibraryID: row.LibraryID,
			})
		}
	}

	return infos
}

// getMovieDetailsDirectly fetches movie details directly from the movies table.
// It also fetches credits from the credits table for proper ordering.
func (q *DBMediaQuerier) getMovieDetailsDirectly(ctx context.Context, id int64, externalIDs map[string]string) (*MediaDetailsInfo, error) {
	result, err := q.router.Route(
		func() (any, error) {
			return q.postgres.GetMovieByMediaID(ctx, int32(id))
		},
		func() (any, error) {
			return q.sqlite.GetMovieByMediaID(ctx, id)
		},
	)
	if err != nil {
		// Movie metadata not found, return minimal info
		return &MediaDetailsInfo{
			ID:          id,
			MediaType:   "movie",
			ExternalIDs: externalIDs,
		}, nil
	}

	details := q.movieRowToDetails(result, externalIDs)

	// Fetch credits from the credits table for proper billing order
	cast, directors, writers, producers := q.getCreditsForEntity(ctx, "movie", id)
	if len(cast) > 0 {
		details.Cast = cast
	}
	if len(directors) > 0 {
		details.Directors = directors
	}
	if len(writers) > 0 {
		details.Writers = writers
	}
	if len(producers) > 0 {
		details.Producers = producers
	}

	// Fetch studios
	studios := q.getStudiosForEntity(ctx, "movie", id)
	if len(studios) > 0 {
		details.Studios = studios
	}

	// Fallback: if no original_language, try to get from primary audio track
	if details.OriginalLanguage == "" {
		if audioLang := q.getPrimaryAudioLanguage(ctx, id); audioLang != "" {
			details.OriginalLanguage = audioLang
		}
	}

	// Fetch location keywords for setting-based searches
	locationKeywords := q.getLocationKeywordsForEntity(ctx, "movie", id)
	if len(locationKeywords) > 0 {
		details.LocationKeywords = locationKeywords
	}

	// Fetch theme keywords for thematic searches (non-location keywords)
	themeKeywords := q.getThemeKeywordsForEntity(ctx, "movie", id)
	if len(themeKeywords) > 0 {
		details.ThemeKeywords = themeKeywords
	}

	return details, nil
}

func (q *DBMediaQuerier) getMovieDetails(ctx context.Context, id int64, basic *MediaInfo, externalIDs map[string]string) (*MediaDetailsInfo, error) {
	result, err := q.router.Route(
		func() (any, error) {
			return q.postgres.GetMovieByMediaID(ctx, int32(id))
		},
		func() (any, error) {
			return q.sqlite.GetMovieByMediaID(ctx, id)
		},
	)
	if err != nil {
		// Movie metadata not found, return basic info
		return &MediaDetailsInfo{
			ID:          basic.ID,
			MediaType:   basic.MediaType,
			Title:       basic.Title,
			Year:        basic.Year,
			LibraryID:   basic.LibraryID,
			ExternalIDs: externalIDs,
		}, nil
	}

	return q.movieRowToDetails(result, externalIDs), nil
}

func (q *DBMediaQuerier) movieRowToDetails(result any, externalIDs map[string]string) *MediaDetailsInfo {
	info := &MediaDetailsInfo{
		MediaType:   "movie",
		ExternalIDs: externalIDs,
	}

	if q.router.IsPostgresDB() {
		row := result.(sqlc_postgres.GetMovieByMediaIDRow)
		info.ID = int64(row.MediaID)
		info.Title = row.Title
		info.LibraryID = int64(row.LibraryID)
		if row.Year.Valid {
			info.Year = int(row.Year.Int32)
		}
		if row.Plot.Valid {
			info.Plot = row.Plot.String
		}
		if row.Tagline.Valid {
			info.Tagline = row.Tagline.String
		}
		if row.Genre.Valid {
			info.Genres = splitAndTrim(row.Genre.String)
		}
		if row.Director.Valid {
			info.Directors = splitAndTrim(row.Director.String)
		}
		if row.Cast.Valid {
			info.Cast = parseCastString(row.Cast.String)
		}
		if row.ContentRating.Valid {
			info.ContentRating = row.ContentRating.String
		}
		if row.RuntimeMinutes.Valid {
			info.RuntimeMinutes = int(row.RuntimeMinutes.Int32)
		}
		if row.OriginalLanguage.Valid {
			info.OriginalLanguage = row.OriginalLanguage.String
		}
		if row.CountryOfOrigin.Valid {
			info.CountryOfOrigin = row.CountryOfOrigin.String
		}
	} else {
		row := result.(sqlc_sqlite.GetMovieByMediaIDRow)
		info.ID = row.MediaID
		info.Title = row.Title
		info.LibraryID = row.LibraryID
		if row.Year.Valid {
			info.Year = int(row.Year.Int64)
		}
		if row.Plot.Valid {
			info.Plot = row.Plot.String
		}
		if row.Tagline.Valid {
			info.Tagline = row.Tagline.String
		}
		if row.Genre.Valid {
			info.Genres = splitAndTrim(row.Genre.String)
		}
		if row.Director.Valid {
			info.Directors = splitAndTrim(row.Director.String)
		}
		if row.Cast.Valid {
			info.Cast = parseCastString(row.Cast.String)
		}
		if row.ContentRating.Valid {
			info.ContentRating = row.ContentRating.String
		}
		if row.RuntimeMinutes.Valid {
			info.RuntimeMinutes = int(row.RuntimeMinutes.Int64)
		}
		if row.OriginalLanguage.Valid {
			info.OriginalLanguage = row.OriginalLanguage.String
		}
		if row.CountryOfOrigin.Valid {
			info.CountryOfOrigin = row.CountryOfOrigin.String
		}
	}

	return info
}

func (q *DBMediaQuerier) listMovieDetailsByLibrary(ctx context.Context, libraryID int64, limit, offset int) ([]*MediaDetailsInfo, int, error) {
	results, err := q.router.Route(
		func() (any, error) {
			return q.postgres.ListMoviesByLibraryPaginated(ctx, sqlc_postgres.ListMoviesByLibraryPaginatedParams{
				LibraryID: int32(libraryID),
				Limit:     int32(limit),
				Offset:    int32(offset),
			})
		},
		func() (any, error) {
			return q.sqlite.ListMoviesByLibraryPaginated(ctx, sqlc_sqlite.ListMoviesByLibraryPaginatedParams{
				LibraryID: libraryID,
				Limit:     int64(limit),
				Offset:    int64(offset),
			})
		},
	)
	if err != nil {
		return nil, 0, err
	}

	countResult, err := q.router.Route(
		func() (any, error) {
			return q.postgres.CountMoviesByLibrary(ctx, int32(libraryID))
		},
		func() (any, error) {
			return q.sqlite.CountMoviesByLibrary(ctx, libraryID)
		},
	)
	if err != nil {
		return nil, 0, err
	}

	total := int(countResult.(int64))
	return q.moviePaginatedRowsToDetails(results), total, nil
}

func (q *DBMediaQuerier) moviePaginatedRowsToDetails(results any) []*MediaDetailsInfo {
	var infos []*MediaDetailsInfo

	if q.router.IsPostgresDB() {
		for _, row := range results.([]sqlc_postgres.ListMoviesByLibraryPaginatedRow) {
			info := &MediaDetailsInfo{
				ID:        int64(row.MediaID),
				MediaType: "movie",
				Title:     row.Title,
				LibraryID: int64(row.LibraryID),
			}
			if row.Year.Valid {
				info.Year = int(row.Year.Int32)
			}
			if row.Plot.Valid {
				info.Plot = row.Plot.String
			}
			if row.Tagline.Valid {
				info.Tagline = row.Tagline.String
			}
			if row.Genre.Valid {
				info.Genres = splitAndTrim(row.Genre.String)
			}
			if row.Director.Valid {
				info.Directors = splitAndTrim(row.Director.String)
			}
			if row.Cast.Valid {
				info.Cast = parseCastString(row.Cast.String)
			}
			if row.ContentRating.Valid {
				info.ContentRating = row.ContentRating.String
			}
			if row.RuntimeMinutes.Valid {
				info.RuntimeMinutes = int(row.RuntimeMinutes.Int32)
			}
			if row.OriginalLanguage.Valid {
				info.OriginalLanguage = row.OriginalLanguage.String
			}
			if row.CountryOfOrigin.Valid {
				info.CountryOfOrigin = row.CountryOfOrigin.String
			}
			infos = append(infos, info)
		}
	} else {
		for _, row := range results.([]sqlc_sqlite.ListMoviesByLibraryPaginatedRow) {
			info := &MediaDetailsInfo{
				ID:        row.MediaID,
				MediaType: "movie",
				Title:     row.Title,
				LibraryID: row.LibraryID,
			}
			if row.Year.Valid {
				info.Year = int(row.Year.Int64)
			}
			if row.Plot.Valid {
				info.Plot = row.Plot.String
			}
			if row.Tagline.Valid {
				info.Tagline = row.Tagline.String
			}
			if row.Genre.Valid {
				info.Genres = splitAndTrim(row.Genre.String)
			}
			if row.Director.Valid {
				info.Directors = splitAndTrim(row.Director.String)
			}
			if row.Cast.Valid {
				info.Cast = parseCastString(row.Cast.String)
			}
			if row.ContentRating.Valid {
				info.ContentRating = row.ContentRating.String
			}
			if row.RuntimeMinutes.Valid {
				info.RuntimeMinutes = int(row.RuntimeMinutes.Int64)
			}
			if row.OriginalLanguage.Valid {
				info.OriginalLanguage = row.OriginalLanguage.String
			}
			if row.CountryOfOrigin.Valid {
				info.CountryOfOrigin = row.CountryOfOrigin.String
			}
			infos = append(infos, info)
		}
	}

	return infos
}
