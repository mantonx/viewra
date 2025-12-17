package plugins

import (
	"context"
	"database/sql"
	"errors"

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
		// Search across all types - just do movies for now
		// TODO: Combine movie + TV results
		return q.searchMovies(ctx, pattern, year, limit)
	}
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

// Ensure DBMediaQuerier implements MediaQuerier
var _ MediaQuerier = (*DBMediaQuerier)(nil)
