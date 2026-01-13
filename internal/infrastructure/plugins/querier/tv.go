package querier

import (
	"context"
	"database/sql"

	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
)

// searchTVShows searches for TV shows by title pattern across all libraries.
func (q *DBMediaQuerier) searchTVShows(ctx context.Context, pattern string, year int, limit int) ([]*MediaInfo, error) {
	results, err := q.querier.SearchTVShowsGlobal(ctx, unified.SearchTVShowsGlobalParams{
		Title:         pattern,
		OriginalTitle: sql.NullString{String: pattern, Valid: true},
		Limit:         int64(limit),
	})
	if err != nil {
		return nil, err
	}

	return q.tvShowResultsToInfo(results, year), nil
}

// tvShowResultsToInfo converts TV show search results to MediaInfo slice.
func (q *DBMediaQuerier) tvShowResultsToInfo(results []unified.SearchTVShowsGlobalRow, yearFilter int) []*MediaInfo {
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
			ID:        row.ID,
			MediaType: "tv_show",
			Title:     row.Title,
			Year:      year,
			LibraryID: row.LibraryID,
		})
	}

	return infos
}

// getTVShowDetailsDirectly fetches TV show details directly from the tv_shows table.
// It also fetches credits from the credits table for proper ordering.
func (q *DBMediaQuerier) getTVShowDetailsDirectly(ctx context.Context, id int64, externalIDs map[string]string) (*MediaDetailsInfo, error) {
	result, err := q.querier.GetTVShowByID(ctx, id)
	if err != nil {
		return &MediaDetailsInfo{
			ID:          id,
			MediaType:   "tv_show",
			ExternalIDs: externalIDs,
		}, nil
	}

	details := q.tvShowRowToDetails(result, externalIDs)

	// Fetch credits from the credits table for proper billing order
	cast, _, writers, producers := q.getCreditsForEntity(ctx, "tv_show", id)
	if len(cast) > 0 {
		details.Cast = cast
	}
	// TV shows use "creator" instead of "director", fetch separately if needed
	if len(writers) > 0 {
		details.Writers = writers
	}
	if len(producers) > 0 {
		details.Producers = producers
	}

	// Fetch studios
	studios := q.getStudiosForEntity(ctx, "tv_show", id)
	if len(studios) > 0 {
		details.Studios = studios
	}

	// Fetch location keywords for setting-based searches
	locationKeywords := q.getLocationKeywordsForEntity(ctx, "tv_show", id)
	if len(locationKeywords) > 0 {
		details.LocationKeywords = locationKeywords
	}

	// Fetch theme keywords for thematic searches (non-location keywords)
	themeKeywords := q.getThemeKeywordsForEntity(ctx, "tv_show", id)
	if len(themeKeywords) > 0 {
		details.ThemeKeywords = themeKeywords
	}

	// Fetch composers and cinematographers for specialized searches
	composers, cinematographers := q.getCrewForEntity(ctx, "tv_show", id)
	if len(composers) > 0 {
		details.Composers = composers
	}
	if len(cinematographers) > 0 {
		details.Cinematographers = cinematographers
	}

	return details, nil
}

func (q *DBMediaQuerier) getTVShowDetails(ctx context.Context, id int64, basic *MediaInfo, externalIDs map[string]string) (*MediaDetailsInfo, error) {
	result, err := q.querier.GetTVShowByID(ctx, id)
	if err != nil {
		return &MediaDetailsInfo{
			ID:          basic.ID,
			MediaType:   basic.MediaType,
			Title:       basic.Title,
			Year:        basic.Year,
			LibraryID:   basic.LibraryID,
			ExternalIDs: externalIDs,
		}, nil
	}

	return q.tvShowRowToDetails(result, externalIDs), nil
}

func (q *DBMediaQuerier) tvShowRowToDetails(row unified.TvShow, externalIDs map[string]string) *MediaDetailsInfo {
	info := &MediaDetailsInfo{
		MediaType:   "tv_show",
		ExternalIDs: externalIDs,
		ID:          row.ID,
		Title:       row.Title,
		LibraryID:   row.LibraryID,
	}

	if row.Year.Valid {
		info.Year = int(row.Year.Int64)
	}
	if row.Plot.Valid {
		info.Plot = row.Plot.String
	}
	if row.Genre.Valid {
		info.Genres = splitAndTrim(row.Genre.String)
	}
	if row.ContentRating.Valid {
		info.ContentRating = row.ContentRating.String
	}
	if row.OriginalLanguage.Valid {
		info.OriginalLanguage = row.OriginalLanguage.String
	}
	if row.CountryOfOrigin.Valid {
		info.CountryOfOrigin = row.CountryOfOrigin.String
	}

	return info
}

// getTVEpisodeDetailsDirectly fetches TV episode details directly.
func (q *DBMediaQuerier) getTVEpisodeDetailsDirectly(ctx context.Context, id int64, externalIDs map[string]string) (*MediaDetailsInfo, error) {
	result, err := q.querier.GetEpisodeWithShowTitle(ctx, id)
	if err != nil {
		return &MediaDetailsInfo{
			ID:          id,
			MediaType:   "tv_episode",
			ExternalIDs: externalIDs,
		}, nil
	}

	details := q.tvEpisodeRowToDetails(result, externalIDs)

	// Fetch playback info for technical filtering (4K, HDR, subtitles, etc.)
	// Episodes have media files, so they have playback info
	details.PlaybackInfo = q.getPlaybackInfoForMedia(ctx, result.MediaID)

	return details, nil
}

func (q *DBMediaQuerier) getTVEpisodeDetails(ctx context.Context, id int64, basic *MediaInfo, externalIDs map[string]string) (*MediaDetailsInfo, error) {
	result, err := q.querier.GetEpisodeWithShowTitle(ctx, id)
	if err != nil {
		return &MediaDetailsInfo{
			ID:          basic.ID,
			MediaType:   basic.MediaType,
			Title:       basic.Title,
			Year:        basic.Year,
			LibraryID:   basic.LibraryID,
			ExternalIDs: externalIDs,
		}, nil
	}

	return q.tvEpisodeRowToDetails(result, externalIDs), nil
}

func (q *DBMediaQuerier) tvEpisodeRowToDetails(row unified.GetEpisodeWithShowTitleRow, externalIDs map[string]string) *MediaDetailsInfo {
	info := &MediaDetailsInfo{
		MediaType:     "tv_episode",
		ExternalIDs:   externalIDs,
		ID:            row.MediaID,
		Title:         row.Title,
		LibraryID:     row.LibraryID,
		SeasonNumber:  int(row.SeasonNumber),
		EpisodeNumber: int(row.EpisodeNumber),
		ShowTitle:     row.ShowTitle,
	}

	if row.Plot.Valid {
		info.Plot = row.Plot.String
	}
	if row.ShowGenre.Valid {
		info.Genres = splitAndTrim(row.ShowGenre.String)
	}

	return info
}

func (q *DBMediaQuerier) listTVShowDetailsByLibrary(ctx context.Context, libraryID int64, limit, offset int) ([]*MediaDetailsInfo, int, error) {
	results, err := q.querier.ListTVShowsByLibraryPaginated(ctx, unified.ListTVShowsByLibraryPaginatedParams{
		LibraryID: libraryID,
		Limit:     int64(limit),
		Offset:    int64(offset),
	})
	if err != nil {
		return nil, 0, err
	}

	total, err := q.querier.CountTVShowsByLibrary(ctx, libraryID)
	if err != nil {
		return nil, 0, err
	}

	return q.tvShowPaginatedRowsToDetails(results), int(total), nil
}

func (q *DBMediaQuerier) tvShowPaginatedRowsToDetails(results []unified.TvShow) []*MediaDetailsInfo {
	var infos []*MediaDetailsInfo

	for _, row := range results {
		info := &MediaDetailsInfo{
			ID:        row.ID,
			MediaType: "tv_show",
			Title:     row.Title,
			LibraryID: row.LibraryID,
		}
		if row.Year.Valid {
			info.Year = int(row.Year.Int64)
		}
		if row.Plot.Valid {
			info.Plot = row.Plot.String
		}
		if row.Genre.Valid {
			info.Genres = splitAndTrim(row.Genre.String)
		}
		if row.ContentRating.Valid {
			info.ContentRating = row.ContentRating.String
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
