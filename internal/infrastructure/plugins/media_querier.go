package plugins

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"

	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// DBMediaQuerier implements MediaQuerier using SQLC-generated queries.
// It supports both SQLite and PostgreSQL through the QueryRouter pattern.
type DBMediaQuerier struct {
	db       *sql.DB
	router   *common.QueryRouter
	sqlite   *sqlc_sqlite.Queries
	postgres *sqlc_postgres.Queries
}

// NewDBMediaQuerier creates a new DBMediaQuerier with the specified database.
func NewDBMediaQuerier(db *sql.DB, driver string) *DBMediaQuerier {
	q := &DBMediaQuerier{
		db:     db,
		router: common.NewQueryRouter(driver),
	}

	if common.IsPostgres(driver) {
		q.postgres = sqlc_postgres.New(db)
	} else {
		q.sqlite = sqlc_sqlite.New(db)
	}

	return q
}

// GetMediaByID returns a media item by its database ID.
func (q *DBMediaQuerier) GetMediaByID(ctx context.Context, id int64) (*MediaInfo, error) {
	result, err := q.router.Route(
		func() (any, error) {
			return q.postgres.GetMediaByID(ctx, int32(id))
		},
		func() (any, error) {
			return q.sqlite.GetMediaByID(ctx, id)
		},
	)
	if err != nil {
		return nil, err
	}

	return q.mediaToInfo(result), nil
}

// GetMediaByExternalID returns a media item by an external ID.
func (q *DBMediaQuerier) GetMediaByExternalID(ctx context.Context, provider, externalID string) (*MediaInfo, error) {
	// First, get the entity_id from external_ids table
	entityResult, err := q.router.Route(
		func() (any, error) {
			return q.postgres.GetMediaByExternalID(ctx, sqlc_postgres.GetMediaByExternalIDParams{
				Provider:   provider,
				ExternalID: externalID,
			})
		},
		func() (any, error) {
			return q.sqlite.GetMediaByExternalID(ctx, sqlc_sqlite.GetMediaByExternalIDParams{
				Provider:   provider,
				ExternalID: externalID,
			})
		},
	)
	if err != nil {
		return nil, err
	}

	// Extract entity_id based on DB type
	var entityID int64
	if q.router.IsPostgresDB() {
		entityID = entityResult.(int64)
	} else {
		entityID = entityResult.(int64)
	}

	// Now get the full media record
	return q.GetMediaByID(ctx, entityID)
}

// SearchMedia searches for media by title and optional year.
func (q *DBMediaQuerier) SearchMedia(ctx context.Context, title string, year int, mediaType string, limit int) ([]*MediaInfo, error) {
	// Use title search pattern
	pattern := "%" + title + "%"

	// Route based on media type
	switch mediaType {
	case "movie":
		return q.searchMovies(ctx, pattern, year, limit)
	case "tv", "tv_show":
		return q.searchTVShows(ctx, pattern, year, limit)
	default:
		// Search across all types and merge results
		return q.searchAll(ctx, title, pattern, year, limit)
	}
}

// searchAll searches both movies and TV shows, merging and ranking results.
func (q *DBMediaQuerier) searchAll(ctx context.Context, title, pattern string, year int, limit int) ([]*MediaInfo, error) {
	// Search both types concurrently
	movieResults, movieErr := q.searchMovies(ctx, pattern, year, limit)
	tvResults, tvErr := q.searchTVShows(ctx, pattern, year, limit)

	// Combine results (prefer partial results over total failure)
	var combined []*MediaInfo
	if movieErr == nil {
		combined = append(combined, movieResults...)
	}
	if tvErr == nil {
		combined = append(combined, tvResults...)
	}

	// If both failed, return the first error
	if movieErr != nil && tvErr != nil {
		return nil, movieErr
	}

	// Rank by title match quality
	rankByTitleMatch(combined, title)

	// Apply limit
	if len(combined) > limit {
		combined = combined[:limit]
	}

	return combined, nil
}

// rankByTitleMatch sorts results by how well they match the search title.
// Exact matches come first, then prefix matches, then contains matches.
func rankByTitleMatch(results []*MediaInfo, searchTitle string) {
	searchLower := strings.ToLower(searchTitle)

	sort.SliceStable(results, func(i, j int) bool {
		titleI := strings.ToLower(results[i].Title)
		titleJ := strings.ToLower(results[j].Title)

		scoreI := titleMatchScore(titleI, searchLower)
		scoreJ := titleMatchScore(titleJ, searchLower)

		return scoreI > scoreJ
	})
}

// titleMatchScore returns a score for how well a title matches the search.
// Higher scores indicate better matches.
func titleMatchScore(title, search string) int {
	// Exact match (highest priority)
	if title == search {
		return 100
	}

	// Exact match ignoring "the" prefix
	titleNoThe := strings.TrimPrefix(title, "the ")
	searchNoThe := strings.TrimPrefix(search, "the ")
	if titleNoThe == searchNoThe {
		return 95
	}

	// Starts with search term
	if strings.HasPrefix(title, search) {
		return 80
	}
	if strings.HasPrefix(titleNoThe, searchNoThe) {
		return 75
	}

	// Contains as whole word
	if strings.Contains(" "+title+" ", " "+search+" ") {
		return 60
	}

	// Contains anywhere
	if strings.Contains(title, search) {
		return 40
	}

	// Default (matched via SQL LIKE but no special ranking)
	return 10
}

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

// searchTVShows searches for TV shows by title pattern across all libraries.
func (q *DBMediaQuerier) searchTVShows(ctx context.Context, pattern string, year int, limit int) ([]*MediaInfo, error) {
	results, err := q.router.Route(
		func() (any, error) {
			return q.postgres.SearchTVShowsGlobal(ctx, sqlc_postgres.SearchTVShowsGlobalParams{
				Title:         pattern,
				OriginalTitle: sql.NullString{String: pattern, Valid: true},
				Limit:         int32(limit),
			})
		},
		func() (any, error) {
			return q.sqlite.SearchTVShowsGlobal(ctx, sqlc_sqlite.SearchTVShowsGlobalParams{
				Title:         pattern,
				OriginalTitle: sql.NullString{String: pattern, Valid: true},
				Limit:         int64(limit),
			})
		},
	)
	if err != nil {
		return nil, err
	}

	return q.tvShowResultsToInfo(results, year), nil
}

// GetFilePath returns the file path for a media item.
func (q *DBMediaQuerier) GetFilePath(ctx context.Context, mediaID int64) (string, error) {
	info, err := q.GetMediaByID(ctx, mediaID)
	if err != nil {
		return "", err
	}
	return info.FilePath, nil
}

// GetExternalIDs returns all external IDs for a media item.
func (q *DBMediaQuerier) GetExternalIDs(ctx context.Context, mediaID int64) (map[string]string, error) {
	results, err := q.router.Route(
		func() (any, error) {
			return q.postgres.GetExternalIDsByMediaID(ctx, sql.NullInt32{Int32: int32(mediaID), Valid: true})
		},
		func() (any, error) {
			return q.sqlite.GetExternalIDsByMediaID(ctx, sql.NullInt64{Int64: mediaID, Valid: true})
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return make(map[string]string), nil
		}
		return nil, err
	}

	ids := make(map[string]string)
	if q.router.IsPostgresDB() {
		for _, row := range results.([]sqlc_postgres.MediaExternalID) {
			ids[row.Provider] = row.ExternalID
		}
	} else {
		for _, row := range results.([]sqlc_sqlite.MediaExternalID) {
			ids[row.Provider] = row.ExternalID
		}
	}

	return ids, nil
}

// mediaToInfo converts a SQLC Medium to MediaInfo.
func (q *DBMediaQuerier) mediaToInfo(result any) *MediaInfo {
	if result == nil {
		return nil
	}

	if q.router.IsPostgresDB() {
		m := result.(sqlc_postgres.Medium)
		return &MediaInfo{
			ID:        int64(m.ID),
			MediaType: m.Type,
			Title:     m.Title,
			FilePath:  m.FilePath,
			LibraryID: int64(m.LibraryID),
		}
	}

	m := result.(sqlc_sqlite.Medium)
	return &MediaInfo{
		ID:        m.ID,
		MediaType: m.Type,
		Title:     m.Title,
		FilePath:  m.FilePath,
		LibraryID: m.LibraryID,
	}
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

// tvShowResultsToInfo converts TV show search results to MediaInfo slice.
func (q *DBMediaQuerier) tvShowResultsToInfo(results any, yearFilter int) []*MediaInfo {
	var infos []*MediaInfo

	if q.router.IsPostgresDB() {
		for _, row := range results.([]sqlc_postgres.SearchTVShowsGlobalRow) {
			year := 0
			if row.Year.Valid {
				year = int(row.Year.Int32)
			}
			// Filter by year if specified
			if yearFilter > 0 && year != yearFilter {
				continue
			}
			infos = append(infos, &MediaInfo{
				ID:        int64(row.ID),
				MediaType: "tv_show",
				Title:     row.Title,
				Year:      year,
				LibraryID: int64(row.LibraryID),
			})
		}
	} else {
		for _, row := range results.([]sqlc_sqlite.SearchTVShowsGlobalRow) {
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
	}

	return infos
}

// GetLibrary returns library information by ID.
func (q *DBMediaQuerier) GetLibrary(ctx context.Context, id int64) (*LibraryInfo, error) {
	result, err := q.router.Route(
		func() (any, error) {
			return q.postgres.GetLibraryByID(ctx, int32(id))
		},
		func() (any, error) {
			return q.sqlite.GetLibraryByID(ctx, id)
		},
	)
	if err != nil {
		return nil, err
	}

	return q.libraryToInfo(result), nil
}

// libraryToInfo converts a SQLC Library to LibraryInfo.
func (q *DBMediaQuerier) libraryToInfo(result any) *LibraryInfo {
	if result == nil {
		return nil
	}

	if q.router.IsPostgresDB() {
		lib := result.(sqlc_postgres.Library)
		return &LibraryInfo{
			ID:        int64(lib.ID),
			Name:      lib.Name,
			Path:      lib.Path,
			MediaType: lib.Type,
		}
	}

	lib := result.(sqlc_sqlite.Library)
	return &LibraryInfo{
		ID:        lib.ID,
		Name:      lib.Name,
		Path:      lib.Path,
		MediaType: lib.Type,
	}
}

// GetMediaDetails returns full metadata for a media item.
// If mediaType is provided, it queries the appropriate table directly.
// If mediaType is empty, it falls back to looking up in the media table first.
func (q *DBMediaQuerier) GetMediaDetails(ctx context.Context, id int64, mediaType string) (*MediaDetailsInfo, error) {
	externalIDs, _ := q.getExternalIDsForEntity(ctx, id, mediaType)

	var details *MediaDetailsInfo
	var err error

	// Use provided mediaType if available, otherwise try to determine from media table
	if mediaType == "" {
		// Fallback: try to get basic info from media table to determine type
		basic, basicErr := q.GetMediaByID(ctx, id)
		if basicErr != nil {
			return nil, basicErr
		}
		mediaType = basic.MediaType
	}

	switch mediaType {
	case "movie":
		details, err = q.getMovieDetailsDirectly(ctx, id, externalIDs)
	case "tv_show":
		details, err = q.getTVShowDetailsDirectly(ctx, id, externalIDs)
	case "tv_episode":
		details, err = q.getTVEpisodeDetailsDirectly(ctx, id, externalIDs)
	case "music_artist":
		details, err = q.getMusicArtistDetailsDirectly(ctx, id, externalIDs)
	case "music_album":
		details, err = q.getMusicAlbumDetailsDirectly(ctx, id, externalIDs)
	case "music_track":
		details, err = q.getMusicTrackDetailsDirectly(ctx, id, externalIDs)
	default:
		// Return minimal info for unsupported types
		details = &MediaDetailsInfo{
			ID:          id,
			MediaType:   mediaType,
			ExternalIDs: externalIDs,
		}
	}

	if err != nil {
		return nil, err
	}

	// Fetch and attach mood tags
	moodTags, _ := q.GetMoodTags(ctx, mediaType, id)
	if len(moodTags) > 0 {
		details.MoodTags = make([]string, len(moodTags))
		for i, t := range moodTags {
			details.MoodTags[i] = t.Tag
		}
	}

	return details, nil
}

// getExternalIDsForEntity fetches external IDs for an entity.
// It uses different entity_type values based on the media type.
func (q *DBMediaQuerier) getExternalIDsForEntity(ctx context.Context, id int64, mediaType string) (map[string]string, error) {
	// For now, just use the existing method which queries by media_id
	// The external_ids table uses entity_id and entity_type columns
	return q.GetExternalIDs(ctx, id)
}

// getMovieDetailsDirectly fetches movie details directly from the movies table.
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

	return q.movieRowToDetails(result, externalIDs), nil
}

// getTVShowDetailsDirectly fetches TV show details directly from the tv_shows table.
func (q *DBMediaQuerier) getTVShowDetailsDirectly(ctx context.Context, id int64, externalIDs map[string]string) (*MediaDetailsInfo, error) {
	result, err := q.router.Route(
		func() (any, error) {
			return q.postgres.GetTVShowByID(ctx, int32(id))
		},
		func() (any, error) {
			return q.sqlite.GetTVShowByID(ctx, id)
		},
	)
	if err != nil {
		return &MediaDetailsInfo{
			ID:          id,
			MediaType:   "tv_show",
			ExternalIDs: externalIDs,
		}, nil
	}

	return q.tvShowRowToDetails(result, externalIDs), nil
}

// getTVEpisodeDetailsDirectly fetches TV episode details directly.
func (q *DBMediaQuerier) getTVEpisodeDetailsDirectly(ctx context.Context, id int64, externalIDs map[string]string) (*MediaDetailsInfo, error) {
	result, err := q.router.Route(
		func() (any, error) {
			return q.postgres.GetEpisodeWithShowTitle(ctx, int32(id))
		},
		func() (any, error) {
			return q.sqlite.GetEpisodeWithShowTitle(ctx, id)
		},
	)
	if err != nil {
		return &MediaDetailsInfo{
			ID:          id,
			MediaType:   "tv_episode",
			ExternalIDs: externalIDs,
		}, nil
	}

	return q.tvEpisodeRowToDetails(result, externalIDs), nil
}

// getMusicArtistDetailsDirectly fetches music artist details.
func (q *DBMediaQuerier) getMusicArtistDetailsDirectly(ctx context.Context, id int64, externalIDs map[string]string) (*MediaDetailsInfo, error) {
	result, err := q.router.Route(
		func() (any, error) {
			return q.postgres.GetArtistByID(ctx, int32(id))
		},
		func() (any, error) {
			return q.sqlite.GetArtistByID(ctx, id)
		},
	)
	if err != nil {
		return &MediaDetailsInfo{
			ID:          id,
			MediaType:   "music_artist",
			ExternalIDs: externalIDs,
		}, nil
	}

	return q.musicArtistRowToDetails(result, externalIDs), nil
}

// getMusicAlbumDetailsDirectly fetches music album details.
func (q *DBMediaQuerier) getMusicAlbumDetailsDirectly(ctx context.Context, id int64, externalIDs map[string]string) (*MediaDetailsInfo, error) {
	result, err := q.router.Route(
		func() (any, error) {
			return q.postgres.GetAlbumByID(ctx, int32(id))
		},
		func() (any, error) {
			return q.sqlite.GetAlbumByID(ctx, id)
		},
	)
	if err != nil {
		return &MediaDetailsInfo{
			ID:          id,
			MediaType:   "music_album",
			ExternalIDs: externalIDs,
		}, nil
	}

	return q.musicAlbumRowToDetails(result, externalIDs), nil
}

// getMusicTrackDetailsDirectly fetches music track details.
func (q *DBMediaQuerier) getMusicTrackDetailsDirectly(ctx context.Context, id int64, externalIDs map[string]string) (*MediaDetailsInfo, error) {
	result, err := q.router.Route(
		func() (any, error) {
			return q.postgres.GetMusicTrackByMediaID(ctx, int32(id))
		},
		func() (any, error) {
			return q.sqlite.GetMusicTrackByMediaID(ctx, id)
		},
	)
	if err != nil {
		return &MediaDetailsInfo{
			ID:          id,
			MediaType:   "music_track",
			ExternalIDs: externalIDs,
		}, nil
	}

	return q.musicTrackRowToDetails(result, externalIDs), nil
}

// musicArtistRowToDetails converts a music artist row to details.
func (q *DBMediaQuerier) musicArtistRowToDetails(result any, externalIDs map[string]string) *MediaDetailsInfo {
	info := &MediaDetailsInfo{
		MediaType:   "music_artist",
		ExternalIDs: externalIDs,
	}

	if q.router.IsPostgresDB() {
		row := result.(sqlc_postgres.MusicArtist)
		info.ID = int64(row.ID)
		info.Title = row.Name
		info.LibraryID = int64(row.LibraryID)
		if row.Bio.Valid {
			info.Biography = row.Bio.String
		}
		if row.Country.Valid {
			info.Country = row.Country.String
		}
		if row.Genre.Valid {
			info.Genres = splitAndTrim(row.Genre.String)
		}
	} else {
		row := result.(sqlc_sqlite.MusicArtist)
		info.ID = row.ID
		info.Title = row.Name
		info.LibraryID = row.LibraryID
		if row.Bio.Valid {
			info.Biography = row.Bio.String
		}
		if row.Country.Valid {
			info.Country = row.Country.String
		}
		if row.Genre.Valid {
			info.Genres = splitAndTrim(row.Genre.String)
		}
	}

	return info
}

// musicAlbumRowToDetails converts a music album row to details.
func (q *DBMediaQuerier) musicAlbumRowToDetails(result any, externalIDs map[string]string) *MediaDetailsInfo {
	info := &MediaDetailsInfo{
		MediaType:   "music_album",
		ExternalIDs: externalIDs,
	}

	if q.router.IsPostgresDB() {
		row := result.(sqlc_postgres.MusicAlbum)
		info.ID = int64(row.ID)
		info.Title = row.Title
		info.LibraryID = int64(row.LibraryID)
		if row.Year.Valid {
			info.Year = int(row.Year.Int32)
		}
		if row.Genre.Valid {
			info.Genres = splitAndTrim(row.Genre.String)
		}
		if row.ReleaseType.Valid {
			info.ReleaseType = row.ReleaseType.String
		}
	} else {
		row := result.(sqlc_sqlite.MusicAlbum)
		info.ID = row.ID
		info.Title = row.Title
		info.LibraryID = row.LibraryID
		if row.Year.Valid {
			info.Year = int(row.Year.Int64)
		}
		if row.Genre.Valid {
			info.Genres = splitAndTrim(row.Genre.String)
		}
		if row.ReleaseType.Valid {
			info.ReleaseType = row.ReleaseType.String
		}
	}

	return info
}

// musicTrackRowToDetails converts a music track row to details.
func (q *DBMediaQuerier) musicTrackRowToDetails(result any, externalIDs map[string]string) *MediaDetailsInfo {
	info := &MediaDetailsInfo{
		MediaType:   "music_track",
		ExternalIDs: externalIDs,
	}

	if q.router.IsPostgresDB() {
		row := result.(sqlc_postgres.GetMusicTrackByMediaIDRow)
		info.ID = int64(row.MediaID)
		info.Title = row.Title
		info.LibraryID = int64(row.LibraryID)
		if row.Artist.Valid {
			info.ArtistName = row.Artist.String
		}
		if row.Album.Valid {
			info.AlbumTitle = row.Album.String
		}
		if row.Genre.Valid {
			info.Genres = splitAndTrim(row.Genre.String)
		}
	} else {
		row := result.(sqlc_sqlite.GetMusicTrackByMediaIDRow)
		info.ID = row.MediaID
		info.Title = row.Title
		info.LibraryID = row.LibraryID
		if row.Artist.Valid {
			info.ArtistName = row.Artist.String
		}
		if row.Album.Valid {
			info.AlbumTitle = row.Album.String
		}
		if row.Genre.Valid {
			info.Genres = splitAndTrim(row.Genre.String)
		}
	}

	return info
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

func (q *DBMediaQuerier) getTVShowDetails(ctx context.Context, id int64, basic *MediaInfo, externalIDs map[string]string) (*MediaDetailsInfo, error) {
	result, err := q.router.Route(
		func() (any, error) {
			return q.postgres.GetTVShowByID(ctx, int32(id))
		},
		func() (any, error) {
			return q.sqlite.GetTVShowByID(ctx, id)
		},
	)
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

func (q *DBMediaQuerier) tvShowRowToDetails(result any, externalIDs map[string]string) *MediaDetailsInfo {
	info := &MediaDetailsInfo{
		MediaType:   "tv_show",
		ExternalIDs: externalIDs,
	}

	if q.router.IsPostgresDB() {
		row := result.(sqlc_postgres.TvShow)
		info.ID = int64(row.ID)
		info.Title = row.Title
		info.LibraryID = int64(row.LibraryID)
		if row.Year.Valid {
			info.Year = int(row.Year.Int32)
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
	} else {
		row := result.(sqlc_sqlite.TvShow)
		info.ID = row.ID
		info.Title = row.Title
		info.LibraryID = row.LibraryID
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
	}

	return info
}

func (q *DBMediaQuerier) getTVEpisodeDetails(ctx context.Context, id int64, basic *MediaInfo, externalIDs map[string]string) (*MediaDetailsInfo, error) {
	result, err := q.router.Route(
		func() (any, error) {
			return q.postgres.GetEpisodeWithShowTitle(ctx, int32(id))
		},
		func() (any, error) {
			return q.sqlite.GetEpisodeWithShowTitle(ctx, id)
		},
	)
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

func (q *DBMediaQuerier) tvEpisodeRowToDetails(result any, externalIDs map[string]string) *MediaDetailsInfo {
	info := &MediaDetailsInfo{
		MediaType:   "tv_episode",
		ExternalIDs: externalIDs,
	}

	if q.router.IsPostgresDB() {
		row := result.(sqlc_postgres.GetEpisodeWithShowTitleRow)
		info.ID = int64(row.MediaID)
		info.Title = row.Title
		info.LibraryID = int64(row.LibraryID)
		if row.Plot.Valid {
			info.Plot = row.Plot.String
		}
		info.SeasonNumber = int(row.SeasonNumber)
		info.EpisodeNumber = int(row.EpisodeNumber)
		info.ShowTitle = row.ShowTitle
		if row.ShowGenre.Valid {
			info.Genres = splitAndTrim(row.ShowGenre.String)
		}
	} else {
		row := result.(sqlc_sqlite.GetEpisodeWithShowTitleRow)
		info.ID = row.MediaID
		info.Title = row.Title
		info.LibraryID = row.LibraryID
		if row.Plot.Valid {
			info.Plot = row.Plot.String
		}
		info.SeasonNumber = int(row.SeasonNumber)
		info.EpisodeNumber = int(row.EpisodeNumber)
		info.ShowTitle = row.ShowTitle
		if row.ShowGenre.Valid {
			info.Genres = splitAndTrim(row.ShowGenre.String)
		}
	}

	return info
}

// ListMediaByLibrary lists all media in a library with pagination.
func (q *DBMediaQuerier) ListMediaByLibrary(ctx context.Context, libraryID int64, limit, offset int) ([]*MediaDetailsInfo, int, error) {
	lib, err := q.GetLibrary(ctx, libraryID)
	if err != nil {
		return nil, 0, err
	}

	switch lib.MediaType {
	case "movies":
		return q.listMovieDetailsByLibrary(ctx, libraryID, limit, offset)
	case "tv":
		return q.listTVShowDetailsByLibrary(ctx, libraryID, limit, offset)
	case "music":
		return q.listMusicDetailsByLibrary(ctx, libraryID, limit, offset)
	default:
		return nil, 0, errors.New("unsupported library type: " + lib.MediaType)
	}
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

func (q *DBMediaQuerier) listTVShowDetailsByLibrary(ctx context.Context, libraryID int64, limit, offset int) ([]*MediaDetailsInfo, int, error) {
	results, err := q.router.Route(
		func() (any, error) {
			return q.postgres.ListTVShowsByLibraryPaginated(ctx, sqlc_postgres.ListTVShowsByLibraryPaginatedParams{
				LibraryID: int32(libraryID),
				Limit:     int32(limit),
				Offset:    int32(offset),
			})
		},
		func() (any, error) {
			return q.sqlite.ListTVShowsByLibraryPaginated(ctx, sqlc_sqlite.ListTVShowsByLibraryPaginatedParams{
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
			return q.postgres.CountTVShowsByLibrary(ctx, int32(libraryID))
		},
		func() (any, error) {
			return q.sqlite.CountTVShowsByLibrary(ctx, libraryID)
		},
	)
	if err != nil {
		return nil, 0, err
	}

	total := int(countResult.(int64))
	return q.tvShowPaginatedRowsToDetails(results), total, nil
}

func (q *DBMediaQuerier) tvShowPaginatedRowsToDetails(results any) []*MediaDetailsInfo {
	var infos []*MediaDetailsInfo

	if q.router.IsPostgresDB() {
		for _, row := range results.([]sqlc_postgres.TvShow) {
			info := &MediaDetailsInfo{
				ID:        int64(row.ID),
				MediaType: "tv_show",
				Title:     row.Title,
				LibraryID: int64(row.LibraryID),
			}
			if row.Year.Valid {
				info.Year = int(row.Year.Int32)
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
	} else {
		for _, row := range results.([]sqlc_sqlite.TvShow) {
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
	}

	return infos
}

func (q *DBMediaQuerier) listMusicDetailsByLibrary(ctx context.Context, libraryID int64, limit, offset int) ([]*MediaDetailsInfo, int, error) {
	// Music libraries contain tracks - list them with album/artist context
	results, err := q.router.Route(
		func() (any, error) {
			return q.postgres.ListMusicTracksByLibraryPaginated(ctx, sqlc_postgres.ListMusicTracksByLibraryPaginatedParams{
				LibraryID: int32(libraryID),
				Limit:     int32(limit),
				Offset:    int32(offset),
			})
		},
		func() (any, error) {
			return q.sqlite.ListMusicTracksByLibraryPaginated(ctx, sqlc_sqlite.ListMusicTracksByLibraryPaginatedParams{
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
			return q.postgres.CountMusicTracksByLibrary(ctx, int32(libraryID))
		},
		func() (any, error) {
			return q.sqlite.CountMusicTracksByLibrary(ctx, libraryID)
		},
	)
	if err != nil {
		return nil, 0, err
	}

	total := int(countResult.(int64))
	return q.musicTrackRowsToDetails(results), total, nil
}

func (q *DBMediaQuerier) musicTrackRowsToDetails(results any) []*MediaDetailsInfo {
	var infos []*MediaDetailsInfo

	if q.router.IsPostgresDB() {
		for _, row := range results.([]sqlc_postgres.ListMusicTracksByLibraryPaginatedRow) {
			info := &MediaDetailsInfo{
				ID:        int64(row.MediaID),
				MediaType: "music_track",
				Title:     row.Title,
				LibraryID: int64(row.LibraryID),
			}
			if row.Artist.Valid {
				info.ArtistName = row.Artist.String
			}
			if row.Album.Valid {
				info.AlbumTitle = row.Album.String
			}
			if row.Genre.Valid {
				info.Genres = splitAndTrim(row.Genre.String)
			}
			infos = append(infos, info)
		}
	} else {
		for _, row := range results.([]sqlc_sqlite.ListMusicTracksByLibraryPaginatedRow) {
			info := &MediaDetailsInfo{
				ID:        row.MediaID,
				MediaType: "music_track",
				Title:     row.Title,
				LibraryID: row.LibraryID,
			}
			if row.Artist.Valid {
				info.ArtistName = row.Artist.String
			}
			if row.Album.Valid {
				info.AlbumTitle = row.Album.String
			}
			if row.Genre.Valid {
				info.Genres = splitAndTrim(row.Genre.String)
			}
			infos = append(infos, info)
		}
	}

	return infos
}

// splitAndTrim splits a comma-separated string and trims whitespace.
func splitAndTrim(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// parseCastString parses a cast string (format varies by source).
// Returns simplified cast member info.
func parseCastString(s string) []CastMemberInfo {
	if s == "" {
		return nil
	}
	// Cast is typically stored as comma-separated names
	names := splitAndTrim(s)
	cast := make([]CastMemberInfo, len(names))
	for i, name := range names {
		cast[i] = CastMemberInfo{
			Name:  name,
			Order: i,
		}
	}
	return cast
}

// GetMoodTags retrieves mood tags for an entity.
func (q *DBMediaQuerier) GetMoodTags(ctx context.Context, entityType string, entityID int64) ([]*MoodTagInfo, error) {
	if q.router.IsPostgresDB() {
		rows, err := q.postgres.GetMoodTagsByEntity(ctx, sqlc_postgres.GetMoodTagsByEntityParams{
			EntityType: entityType,
			EntityID:   int32(entityID),
		})
		if err != nil {
			return nil, err
		}
		tags := make([]*MoodTagInfo, len(rows))
		for i, row := range rows {
			confidence := float32(1.0)
			if row.Confidence.Valid {
				confidence = float32(row.Confidence.Float64)
			}
			tags[i] = &MoodTagInfo{
				Tag:        row.Tag,
				Confidence: confidence,
			}
		}
		return tags, nil
	}

	rows, err := q.sqlite.GetMoodTagsByEntity(ctx, sqlc_sqlite.GetMoodTagsByEntityParams{
		EntityType: entityType,
		EntityID:   entityID,
	})
	if err != nil {
		return nil, err
	}
	tags := make([]*MoodTagInfo, len(rows))
	for i, row := range rows {
		confidence := float32(1.0)
		if row.Confidence.Valid {
			confidence = float32(row.Confidence.Float64)
		}
		tags[i] = &MoodTagInfo{
			Tag:        row.Tag,
			Confidence: confidence,
		}
	}
	return tags, nil
}

// SetMoodTags stores mood tags for an entity (replaces existing).
func (q *DBMediaQuerier) SetMoodTags(ctx context.Context, entityType string, entityID int64, tags []*MoodTagInfo) error {
	// Delete existing tags first
	if err := q.DeleteMoodTags(ctx, entityType, entityID); err != nil {
		return err
	}

	// Insert new tags
	for _, tag := range tags {
		if q.router.IsPostgresDB() {
			err := q.postgres.InsertMoodTag(ctx, sqlc_postgres.InsertMoodTagParams{
				EntityType: entityType,
				EntityID:   int32(entityID),
				Tag:        tag.Tag,
				Confidence: sql.NullFloat64{Float64: float64(tag.Confidence), Valid: true},
			})
			if err != nil {
				return err
			}
		} else {
			err := q.sqlite.InsertMoodTag(ctx, sqlc_sqlite.InsertMoodTagParams{
				EntityType: entityType,
				EntityID:   entityID,
				Tag:        tag.Tag,
				Confidence: sql.NullFloat64{Float64: float64(tag.Confidence), Valid: true},
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// DeleteMoodTags removes all mood tags for an entity.
func (q *DBMediaQuerier) DeleteMoodTags(ctx context.Context, entityType string, entityID int64) error {
	return q.router.RouteVoid(
		func() error {
			return q.postgres.DeleteMoodTagsByEntity(ctx, sqlc_postgres.DeleteMoodTagsByEntityParams{
				EntityType: entityType,
				EntityID:   int32(entityID),
			})
		},
		func() error {
			return q.sqlite.DeleteMoodTagsByEntity(ctx, sqlc_sqlite.DeleteMoodTagsByEntityParams{
				EntityType: entityType,
				EntityID:   entityID,
			})
		},
	)
}

// Ensure DBMediaQuerier implements MediaQuerier
var _ MediaQuerier = (*DBMediaQuerier)(nil)
