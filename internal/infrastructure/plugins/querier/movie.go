package querier

import (
	"context"
	"database/sql"

	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
)

// searchMovies searches for movies by title pattern across all libraries.
func (q *DBMediaQuerier) searchMovies(ctx context.Context, pattern string, year int, limit int) ([]*MediaInfo, error) {
	results, err := q.querier.SearchMoviesGlobal(ctx, unified.SearchMoviesGlobalParams{
		Title:         pattern,
		OriginalTitle: sql.NullString{String: pattern, Valid: true},
		Limit:         int64(limit),
	})
	if err != nil {
		return nil, err
	}

	return q.movieResultsToInfo(results, year), nil
}

// movieResultsToInfo converts movie search results to MediaInfo slice.
func (q *DBMediaQuerier) movieResultsToInfo(results []unified.SearchMoviesGlobalRow, yearFilter int) []*MediaInfo {
	var infos []*MediaInfo

	for _, row := range results {
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

	return infos
}

// getMovieDetailsDirectly fetches movie details directly from the movies table.
// It also fetches credits from the credits table for proper ordering.
func (q *DBMediaQuerier) getMovieDetailsDirectly(ctx context.Context, id int64, externalIDs map[string]string) (*MediaDetailsInfo, error) {
	result, err := q.querier.GetMovieByMediaID(ctx, id)
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

	// Fetch composers and cinematographers for specialized searches
	composers, cinematographers := q.getCrewForEntity(ctx, "movie", id)
	if len(composers) > 0 {
		details.Composers = composers
	}
	if len(cinematographers) > 0 {
		details.Cinematographers = cinematographers
	}

	// Fetch similar titles for "movies like X" searches
	similarTitles := q.getSimilarTitlesForEntity(ctx, "movie", id)
	if len(similarTitles) > 0 {
		details.SimilarTitles = similarTitles
	}

	// Fetch playback info for technical filtering (4K, HDR, subtitles, etc.)
	details.PlaybackInfo = q.getPlaybackInfoForMedia(ctx, id)

	return details, nil
}

func (q *DBMediaQuerier) getMovieDetails(ctx context.Context, id int64, basic *MediaInfo, externalIDs map[string]string) (*MediaDetailsInfo, error) {
	result, err := q.querier.GetMovieByMediaID(ctx, id)
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

func (q *DBMediaQuerier) movieRowToDetails(row unified.GetMovieByMediaIDRow, externalIDs map[string]string) *MediaDetailsInfo {
	info := &MediaDetailsInfo{
		MediaType:   "movie",
		ExternalIDs: externalIDs,
		ID:          row.MediaID,
		Title:       row.Title,
		LibraryID:   row.LibraryID,
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

	return info
}

func (q *DBMediaQuerier) listMovieDetailsByLibrary(ctx context.Context, libraryID int64, limit, offset int) ([]*MediaDetailsInfo, int, error) {
	results, err := q.querier.ListMoviesByLibraryPaginated(ctx, unified.ListMoviesByLibraryPaginatedParams{
		LibraryID: libraryID,
		Limit:     int64(limit),
		Offset:    int64(offset),
	})
	if err != nil {
		return nil, 0, err
	}

	total, err := q.querier.CountMoviesByLibrary(ctx, libraryID)
	if err != nil {
		return nil, 0, err
	}

	return q.moviePaginatedRowsToDetails(results), int(total), nil
}

func (q *DBMediaQuerier) moviePaginatedRowsToDetails(results []unified.ListMoviesByLibraryPaginatedRow) []*MediaDetailsInfo {
	var infos []*MediaDetailsInfo

	for _, row := range results {
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

	return infos
}
