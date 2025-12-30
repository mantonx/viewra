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

	// Note: MoodTags are now managed by the semantic-search plugin
	// and stored in plugin-owned tables. The plugin adds them to search results.

	return details, nil
}

// getExternalIDsForEntity fetches external IDs for an entity.
// It uses different entity_type values based on the media type.
func (q *DBMediaQuerier) getExternalIDsForEntity(ctx context.Context, id int64, mediaType string) (map[string]string, error) {
	// For now, just use the existing method which queries by media_id
	// The external_ids table uses entity_id and entity_type columns
	return q.GetExternalIDs(ctx, id)
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
