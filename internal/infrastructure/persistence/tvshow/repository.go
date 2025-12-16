package tvshow

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	domainCommon "github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/media"
	sqlc_postgres "github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// Repository implements media.TVRepository using sqlc.
// It embeds BaseRepository for dual-database support.
// Handles the three-level hierarchy: Shows -> Seasons -> Episodes
type Repository struct {
	*common.BaseRepository
	mediaRepo media.Repository
}

// NewRepository creates a new TV repository with the specified database driver.
// The driver parameter should be "sqlite", "sqlite3", "postgres", or "postgresql".
func NewRepository(db *common.BaseRepository, mediaRepo media.Repository) *Repository {
	return &Repository{
		BaseRepository: db,
		mediaRepo:      mediaRepo,
	}
}

// CreateTVEpisode adds a new TV episode to the repository
// This method auto-creates show and season records if they don't exist
func (r *Repository) CreateTVEpisode(ctx context.Context, episode *media.TVEpisode) error {
	// First, create or get the TV show
	show, err := r.ensureTVShowExists(ctx, episode)
	if err != nil {
		return fmt.Errorf("failed to ensure show exists: %w", err)
	}

	// Then, create or get the TV season
	season, err := r.ensureTVSeasonExists(ctx, show.ID, episode)
	if err != nil {
		return fmt.Errorf("failed to ensure season exists: %w", err)
	}

	// Create the base media record
	if err := r.mediaRepo.Create(ctx, &episode.Media); err != nil {
		return fmt.Errorf("failed to create base media record: %w", err)
	}

	// Create the episode record
	err = common.ExecuteCommand(
		r.BaseRepository, ctx,
		func() error {
			return r.Postgres().CreateTVEpisode(ctx, buildPostgresCreateEpisodeParams(episode, show.ID, season.ID))
		},
		func() error {
			return r.SQLite().CreateTVEpisode(ctx, buildSQLiteCreateEpisodeParams(episode, show.ID, season.ID))
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create episode record: %w", err)
	}

	// Increment the season's episode count
	if err := r.incrementSeasonEpisodeCount(ctx, season.ID); err != nil {
		return fmt.Errorf("failed to increment season episode count: %w", err)
	}

	return nil
}

// GetTVEpisodeByID retrieves a TV episode by its media ID
func (r *Repository) GetTVEpisodeByID(ctx context.Context, id int64) (*media.TVEpisode, error) {
	episode, err := common.QuerySingle(
		r.BaseRepository, ctx,
		func() (sqlc_postgres.GetTVEpisodeByMediaIDRow, error) {
			return r.Postgres().GetTVEpisodeByMediaID(ctx, int32(id))
		},
		func() (sqlc_sqlite.GetTVEpisodeByMediaIDRow, error) {
			return r.SQLite().GetTVEpisodeByMediaID(ctx, id)
		},
		postgresEpisodeToDomain,
		sqliteEpisodeToDomain,
	)
	return episode, r.ConvertNotFoundError(err)
}

// ListTVEpisodesByLibrary retrieves all TV episodes in a specific library
func (r *Repository) ListTVEpisodesByLibrary(ctx context.Context, libraryID int64) ([]*media.TVEpisode, error) {
	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.ListTVEpisodesByLibraryRow, error) {
			return r.Postgres().ListTVEpisodesByLibrary(ctx, int32(libraryID))
		},
		func() ([]sqlc_sqlite.ListTVEpisodesByLibraryRow, error) {
			return r.SQLite().ListTVEpisodesByLibrary(ctx, libraryID)
		},
		postgresEpisodeRowToDomain[sqlc_postgres.ListTVEpisodesByLibraryRow],
		sqliteGenericEpisodeRowToDomain[sqlc_sqlite.ListTVEpisodesByLibraryRow],
	)
}

// ListTVEpisodesByShow retrieves all episodes of a specific show by title
func (r *Repository) ListTVEpisodesByShow(ctx context.Context, libraryID int64, showTitle string) ([]*media.TVEpisode, error) {
	// First, get the show by title
	show, err := r.GetTVShowByTitle(ctx, libraryID, showTitle)
	if err != nil {
		return nil, err
	}

	return r.ListTVEpisodesByShowID(ctx, show.ID)
}

// ListTVEpisodesByShowID retrieves all episodes of a specific show by ID
func (r *Repository) ListTVEpisodesByShowID(ctx context.Context, showID int64) ([]*media.TVEpisode, error) {
	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.ListTVEpisodesByShowRow, error) {
			return r.Postgres().ListTVEpisodesByShow(ctx, int32(showID))
		},
		func() ([]sqlc_sqlite.ListTVEpisodesByShowRow, error) {
			return r.SQLite().ListTVEpisodesByShow(ctx, showID)
		},
		postgresEpisodeRowToDomain[sqlc_postgres.ListTVEpisodesByShowRow],
		sqliteGenericEpisodeRowToDomain[sqlc_sqlite.ListTVEpisodesByShowRow],
	)
}

// UpdateTVEpisode modifies an existing TV episode
func (r *Repository) UpdateTVEpisode(ctx context.Context, episode *media.TVEpisode) error {
	// First, update the base media record
	if err := r.mediaRepo.Update(ctx, &episode.Media); err != nil {
		return fmt.Errorf("failed to update base media record: %w", err)
	}

	// Use the ShowID and SeasonID that are already stored in the episode record.
	// These are populated when the episode is retrieved from the database.
	// This avoids the need to look up the show/season by title, which fails
	// when enrichment runs before the parent show/season metadata is populated.
	showID := episode.ShowID
	seasonID := episode.SeasonID

	// If ShowID or SeasonID is 0, the episode struct was likely constructed fresh
	// (e.g., during a scan update) without retrieving from DB first.
	// In this case, we need to ensure the parent entities exist and get their IDs.
	if showID == 0 || seasonID == 0 {
		// Ensure the show exists (creates if needed)
		show, err := r.ensureTVShowExists(ctx, episode)
		if err != nil {
			return fmt.Errorf("failed to ensure show exists for update: %w", err)
		}
		showID = show.ID

		// Ensure the season exists (creates if needed)
		season, err := r.ensureTVSeasonExists(ctx, showID, episode)
		if err != nil {
			return fmt.Errorf("failed to ensure season exists for update: %w", err)
		}
		seasonID = season.ID
	}

	// Update the episode record
	return common.ExecuteCommand(
		r.BaseRepository, ctx,
		func() error {
			return r.Postgres().UpdateTVEpisode(ctx, buildPostgresUpdateEpisodeParams(episode, showID, seasonID))
		},
		func() error {
			return r.SQLite().UpdateTVEpisode(ctx, buildSQLiteUpdateEpisodeParams(episode, showID, seasonID))
		},
	)
}

// SearchTVEpisodes searches for TV episodes by show title or episode title
func (r *Repository) SearchTVEpisodes(ctx context.Context, libraryID int64, query string) ([]*media.TVEpisode, error) {
	// Add wildcards for LIKE search
	searchPattern := "%" + query + "%"

	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.SearchTVEpisodesByTitleRow, error) {
			return r.Postgres().SearchTVEpisodesByTitle(ctx, sqlc_postgres.SearchTVEpisodesByTitleParams{
				LibraryID:    int32(libraryID),
				EpisodeTitle: common.NullString(searchPattern),
				Title:        searchPattern,
			})
		},
		func() ([]sqlc_sqlite.SearchTVEpisodesByTitleRow, error) {
			return r.SQLite().SearchTVEpisodesByTitle(ctx, sqlc_sqlite.SearchTVEpisodesByTitleParams{
				LibraryID:    libraryID,
				EpisodeTitle: common.NullString(searchPattern),
				Title:        searchPattern,
			})
		},
		postgresEpisodeRowToDomain[sqlc_postgres.SearchTVEpisodesByTitleRow],
		sqliteGenericEpisodeRowToDomain[sqlc_sqlite.SearchTVEpisodesByTitleRow],
	)
}

// --- TV Show Methods ---

// CreateTVShow adds a new TV show to the repository
func (r *Repository) CreateTVShow(ctx context.Context, libraryID int64, title, directory string) (media.TVShow, error) {
	return common.QuerySingle(
		r.BaseRepository, ctx,
		func() (sqlc_postgres.TvShow, error) {
			return r.Postgres().CreateTVShow(ctx, buildPostgresCreateTVShowParams(libraryID, title, directory))
		},
		func() (sqlc_sqlite.TvShow, error) {
			return r.SQLite().CreateTVShow(ctx, buildSQLiteCreateTVShowParams(libraryID, title, directory))
		},
		postgresShowToDomain,
		sqliteShowToDomain,
	)
}

// GetTVShowByID retrieves a TV show by its ID
func (r *Repository) GetTVShowByID(ctx context.Context, id int64) (media.TVShow, error) {
	show, err := common.QuerySingle(
		r.BaseRepository, ctx,
		func() (sqlc_postgres.TvShow, error) {
			return r.Postgres().GetTVShowByID(ctx, int32(id))
		},
		func() (sqlc_sqlite.TvShow, error) {
			return r.SQLite().GetTVShowByID(ctx, id)
		},
		postgresShowToDomain,
		sqliteShowToDomain,
	)
	if err != nil {
		return media.TVShow{}, r.ConvertNotFoundError(err)
	}
	return show, nil
}

// GetTVShowByTitle retrieves a TV show by its title in a library.
// The title is normalized before lookup to match how titles are stored.
func (r *Repository) GetTVShowByTitle(ctx context.Context, libraryID int64, title string) (media.TVShow, error) {
	// Normalize title to match how it's stored in the database
	normalizedTitle := domainCommon.NormalizeTitle(title)

	show, err := common.QuerySingle(
		r.BaseRepository, ctx,
		func() (sqlc_postgres.TvShow, error) {
			return r.Postgres().GetTVShowByTitle(ctx, sqlc_postgres.GetTVShowByTitleParams{
				LibraryID: int32(libraryID),
				Lower:     normalizedTitle,
			})
		},
		func() (sqlc_sqlite.TvShow, error) {
			return r.SQLite().GetTVShowByTitle(ctx, sqlc_sqlite.GetTVShowByTitleParams{
				LibraryID: libraryID,
				LOWER:     normalizedTitle,
			})
		},
		postgresShowToDomain,
		sqliteShowToDomain,
	)
	if err != nil {
		return media.TVShow{}, r.ConvertNotFoundError(err)
	}
	return show, nil
}

// UpdateTVShow updates an existing TV show's metadata
func (r *Repository) UpdateTVShow(ctx context.Context, show media.TVShow) error {
	return common.ExecuteCommand(
		r.BaseRepository, ctx,
		func() error {
			return r.Postgres().UpdateTVShow(ctx, buildPostgresUpdateTVShowParams(show))
		},
		func() error {
			return r.SQLite().UpdateTVShow(ctx, buildSQLiteUpdateTVShowParams(show))
		},
	)
}

// ListTVShowsByLibrary retrieves all TV shows in a library
func (r *Repository) ListTVShowsByLibrary(ctx context.Context, libraryID int64) ([]media.TVShow, error) {
	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.TvShow, error) {
			return r.Postgres().ListTVShowsByLibrary(ctx, int32(libraryID))
		},
		func() ([]sqlc_sqlite.TvShow, error) {
			return r.SQLite().ListTVShowsByLibrary(ctx, libraryID)
		},
		postgresShowToDomain,
		sqliteShowToDomain,
	)
}

// --- TV Season Methods ---

// CreateTVSeason adds a new TV season to the repository
func (r *Repository) CreateTVSeason(ctx context.Context, showID, seasonNumber int64) (media.TVSeason, error) {
	return common.QuerySingle(
		r.BaseRepository, ctx,
		func() (sqlc_postgres.TvSeason, error) {
			return r.Postgres().CreateTVSeason(ctx, buildPostgresCreateTVSeasonParams(showID, seasonNumber))
		},
		func() (sqlc_sqlite.TvSeason, error) {
			return r.SQLite().CreateTVSeason(ctx, buildSQLiteCreateTVSeasonParams(showID, seasonNumber))
		},
		postgresSeasonToDomain,
		sqliteSeasonToDomain,
	)
}

// GetTVSeasonByID retrieves a TV season by its ID
func (r *Repository) GetTVSeasonByID(ctx context.Context, id int64) (media.TVSeason, error) {
	season, err := common.QuerySingle(
		r.BaseRepository, ctx,
		func() (sqlc_postgres.TvSeason, error) {
			return r.Postgres().GetTVSeasonByID(ctx, int32(id))
		},
		func() (sqlc_sqlite.TvSeason, error) {
			return r.SQLite().GetTVSeasonByID(ctx, id)
		},
		postgresSeasonToDomain,
		sqliteSeasonToDomain,
	)
	if err != nil {
		return media.TVSeason{}, r.ConvertNotFoundError(err)
	}
	return season, nil
}

// GetTVSeasonByShowAndNumber retrieves a TV season by show ID and season number
func (r *Repository) GetTVSeasonByShowAndNumber(ctx context.Context, showID, seasonNumber int64) (media.TVSeason, error) {
	season, err := common.QuerySingle(
		r.BaseRepository, ctx,
		func() (sqlc_postgres.TvSeason, error) {
			return r.Postgres().GetTVSeasonByShowAndNumber(ctx, sqlc_postgres.GetTVSeasonByShowAndNumberParams{
				ShowID:       int32(showID),
				SeasonNumber: int32(seasonNumber),
			})
		},
		func() (sqlc_sqlite.TvSeason, error) {
			return r.SQLite().GetTVSeasonByShowAndNumber(ctx, sqlc_sqlite.GetTVSeasonByShowAndNumberParams{
				ShowID:       showID,
				SeasonNumber: seasonNumber,
			})
		},
		postgresSeasonToDomain,
		sqliteSeasonToDomain,
	)
	if err != nil {
		return media.TVSeason{}, r.ConvertNotFoundError(err)
	}
	return season, nil
}

// ListTVSeasonsByShow retrieves all seasons for a show
func (r *Repository) ListTVSeasonsByShow(ctx context.Context, showID int64) ([]media.TVSeason, error) {
	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.TvSeason, error) {
			return r.Postgres().ListTVSeasonsByShow(ctx, int32(showID))
		},
		func() ([]sqlc_sqlite.TvSeason, error) {
			return r.SQLite().ListTVSeasonsByShow(ctx, showID)
		},
		postgresSeasonToDomain,
		sqliteSeasonToDomain,
	)
}

// CountTVShowsByLibrary returns the total count of TV shows in a library
func (r *Repository) CountTVShowsByLibrary(ctx context.Context, libraryID int64) (int64, error) {
	return common.QueryScalar(
		r.BaseRepository, ctx,
		func() (int64, error) {
			return r.Postgres().CountTVShowsByLibrary(ctx, int32(libraryID))
		},
		func() (int64, error) {
			return r.SQLite().CountTVShowsByLibrary(ctx, libraryID)
		},
	)
}

// ListTVShowsByLibraryPaginated retrieves TV shows in a library with pagination
func (r *Repository) ListTVShowsByLibraryPaginated(ctx context.Context, libraryID int64, pagination *domainCommon.PaginationParams) ([]media.TVShowWithCounts, error) {
	if pagination == nil {
		pagination = domainCommon.DefaultPaginationParams()
	}

	// Default to title_asc if not specified
	sortBy := pagination.SortBy
	if sortBy == "" {
		sortBy = "title_asc"
	}

	// Use different query based on sort order
	if sortBy == "title_desc" {
		return common.QueryMany(
			r.BaseRepository, ctx,
			func() ([]sqlc_postgres.GetTVShowsWithCountsByLibraryPaginatedDescRow, error) {
				return r.Postgres().GetTVShowsWithCountsByLibraryPaginatedDesc(ctx, sqlc_postgres.GetTVShowsWithCountsByLibraryPaginatedDescParams{
					LibraryID: int32(libraryID),
					Limit:     int32(pagination.Limit),
					Offset:    int32(pagination.Offset),
				})
			},
			func() ([]sqlc_sqlite.GetTVShowsWithCountsByLibraryPaginatedDescRow, error) {
				return r.SQLite().GetTVShowsWithCountsByLibraryPaginatedDesc(ctx, sqlc_sqlite.GetTVShowsWithCountsByLibraryPaginatedDescParams{
					LibraryID: libraryID,
					Limit:     int64(pagination.Limit),
					Offset:    int64(pagination.Offset),
				})
			},
			postgresShowWithCountsDescToDomain,
			sqliteShowWithCountsDescToDomain,
		)
	}

	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.GetTVShowsWithCountsByLibraryPaginatedRow, error) {
			return r.Postgres().GetTVShowsWithCountsByLibraryPaginated(ctx, sqlc_postgres.GetTVShowsWithCountsByLibraryPaginatedParams{
				LibraryID: int32(libraryID),
				Limit:     int32(pagination.Limit),
				Offset:    int32(pagination.Offset),
			})
		},
		func() ([]sqlc_sqlite.GetTVShowsWithCountsByLibraryPaginatedRow, error) {
			return r.SQLite().GetTVShowsWithCountsByLibraryPaginated(ctx, sqlc_sqlite.GetTVShowsWithCountsByLibraryPaginatedParams{
				LibraryID: libraryID,
				Limit:     int64(pagination.Limit),
				Offset:    int64(pagination.Offset),
			})
		},
		postgresShowWithCountsToDomain,
		sqliteShowWithCountsToDomain,
	)
}

// CountSearchTVShowsByTitle returns the count of TV shows matching a search query
func (r *Repository) CountSearchTVShowsByTitle(ctx context.Context, libraryID int64, query string) (int64, error) {
	searchPattern := "%" + query + "%"

	return common.QueryScalar(
		r.BaseRepository, ctx,
		func() (int64, error) {
			return r.Postgres().CountSearchTVShowsByTitle(ctx, sqlc_postgres.CountSearchTVShowsByTitleParams{
				LibraryID:     int32(libraryID),
				Title:         searchPattern,
				OriginalTitle: common.NullString(searchPattern),
			})
		},
		func() (int64, error) {
			return r.SQLite().CountSearchTVShowsByTitle(ctx, sqlc_sqlite.CountSearchTVShowsByTitleParams{
				LibraryID:     libraryID,
				Title:         searchPattern,
				OriginalTitle: common.NullString(searchPattern),
			})
		},
	)
}

// SearchTVShowsByTitlePaginated searches for TV shows by title with pagination
func (r *Repository) SearchTVShowsByTitlePaginated(ctx context.Context, libraryID int64, query string, pagination *domainCommon.PaginationParams) ([]media.TVShow, error) {
	if pagination == nil {
		pagination = domainCommon.DefaultPaginationParams()
	}

	searchPattern := "%" + query + "%"

	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.TvShow, error) {
			return r.Postgres().SearchTVShowsByTitlePaginated(ctx, sqlc_postgres.SearchTVShowsByTitlePaginatedParams{
				LibraryID:     int32(libraryID),
				Title:         searchPattern,
				OriginalTitle: common.NullString(searchPattern),
				Limit:         int32(pagination.Limit),
				Offset:        int32(pagination.Offset),
			})
		},
		func() ([]sqlc_sqlite.TvShow, error) {
			return r.SQLite().SearchTVShowsByTitlePaginated(ctx, sqlc_sqlite.SearchTVShowsByTitlePaginatedParams{
				LibraryID:     libraryID,
				Title:         searchPattern,
				OriginalTitle: common.NullString(searchPattern),
				Limit:         int64(pagination.Limit),
				Offset:        int64(pagination.Offset),
			})
		},
		postgresShowToDomain,
		sqliteShowToDomain,
	)
}

// SearchTVShowsWithCountsByTitlePaginated searches for TV shows by title with counts and pagination
func (r *Repository) SearchTVShowsWithCountsByTitlePaginated(ctx context.Context, libraryID int64, query string, pagination *domainCommon.PaginationParams) ([]media.TVShowWithCounts, error) {
	if pagination == nil {
		pagination = domainCommon.DefaultPaginationParams()
	}

	searchPattern := "%" + query + "%"

	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.SearchTVShowsWithCountsByTitlePaginatedRow, error) {
			return r.Postgres().SearchTVShowsWithCountsByTitlePaginated(ctx, sqlc_postgres.SearchTVShowsWithCountsByTitlePaginatedParams{
				LibraryID:     int32(libraryID),
				Title:         searchPattern,
				OriginalTitle: common.NullString(searchPattern),
				Limit:         int32(pagination.Limit),
				Offset:        int32(pagination.Offset),
			})
		},
		func() ([]sqlc_sqlite.SearchTVShowsWithCountsByTitlePaginatedRow, error) {
			return r.SQLite().SearchTVShowsWithCountsByTitlePaginated(ctx, sqlc_sqlite.SearchTVShowsWithCountsByTitlePaginatedParams{
				LibraryID:     libraryID,
				Title:         searchPattern,
				OriginalTitle: common.NullString(searchPattern),
				Limit:         int64(pagination.Limit),
				Offset:        int64(pagination.Offset),
			})
		},
		postgresSearchShowWithCountsToDomain,
		sqliteSearchShowWithCountsToDomain,
	)
}

// ListTVShowIDsByLibraryPaginated retrieves only TV show IDs in a library with pagination
func (r *Repository) ListTVShowIDsByLibraryPaginated(ctx context.Context, libraryID int64, pagination *domainCommon.PaginationParams) ([]int64, error) {
	if pagination == nil {
		pagination = domainCommon.DefaultPaginationParams()
	}

	// Determine sort order from pagination params
	sortBy := pagination.SortBy
	if sortBy == "" {
		sortBy = "title_asc"
	}

	// Choose the appropriate query based on sort order
	if sortBy == "title_desc" {
		return common.QueryMany(
			r.BaseRepository, ctx,
			func() ([]int64, error) {
				ids, err := r.Postgres().ListTVShowIDsByLibraryPaginatedDesc(ctx, sqlc_postgres.ListTVShowIDsByLibraryPaginatedDescParams{
					LibraryID: int32(libraryID),
					Limit:     int32(pagination.Limit),
					Offset:    int32(pagination.Offset),
				})
				// Convert []int32 to []int64
				result := make([]int64, len(ids))
				for i, id := range ids {
					result[i] = int64(id)
				}
				return result, err
			},
			func() ([]int64, error) {
				return r.SQLite().ListTVShowIDsByLibraryPaginatedDesc(ctx, sqlc_sqlite.ListTVShowIDsByLibraryPaginatedDescParams{
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
			ids, err := r.Postgres().ListTVShowIDsByLibraryPaginated(ctx, sqlc_postgres.ListTVShowIDsByLibraryPaginatedParams{
				LibraryID: int32(libraryID),
				Limit:     int32(pagination.Limit),
				Offset:    int32(pagination.Offset),
			})
			// Convert []int32 to []int64
			result := make([]int64, len(ids))
			for i, id := range ids {
				result[i] = int64(id)
			}
			return result, err
		},
		func() ([]int64, error) {
			return r.SQLite().ListTVShowIDsByLibraryPaginated(ctx, sqlc_sqlite.ListTVShowIDsByLibraryPaginatedParams{
				LibraryID: libraryID,
				Limit:     int64(pagination.Limit),
				Offset:    int64(pagination.Offset),
			})
		},
		func(id int64) int64 { return id },
		func(id int64) int64 { return id },
	)
}

// --- Helper Methods ---

// ensureTVShowExists atomically creates a TV show if it doesn't exist, or returns the existing one.
// Uses INSERT...ON CONFLICT to prevent race conditions during concurrent episode scans.
// If the show exists but has no directory set, the upsert will update it.
func (r *Repository) ensureTVShowExists(ctx context.Context, episode *media.TVEpisode) (media.TVShow, error) {
	showDir := deriveShowDirectory(episode.Media.FilePath)

	// Use atomic upsert - this prevents race conditions when multiple workers
	// try to create the same show concurrently
	return common.QuerySingle(
		r.BaseRepository, ctx,
		func() (sqlc_postgres.TvShow, error) {
			return r.Postgres().UpsertTVShow(ctx, buildPostgresUpsertTVShowParams(episode.LibraryID, episode.ShowTitle, showDir))
		},
		func() (sqlc_sqlite.TvShow, error) {
			return r.SQLite().UpsertTVShow(ctx, buildSQLiteUpsertTVShowParams(episode.LibraryID, episode.ShowTitle, showDir))
		},
		postgresShowToDomain,
		sqliteShowToDomain,
	)
}

// deriveShowDirectory extracts the show directory from an episode file path.
// Handles both structures:
// - /library/Show Name/Season XX/episode.mkv -> /library/Show Name
// - /library/Show Name/episode.mkv -> /library/Show Name
func deriveShowDirectory(episodePath string) string {
	episodeDir := filepath.Dir(episodePath)
	episodeDirName := filepath.Base(episodeDir)

	// Check if episode is in a season subdirectory
	if strings.HasPrefix(strings.ToLower(episodeDirName), "season") ||
		strings.HasPrefix(strings.ToLower(episodeDirName), "specials") {
		// Episode is in a season/specials subdirectory, go up one more level
		return filepath.Dir(episodeDir)
	}

	// Episode is directly in show directory
	return episodeDir
}

// ensureTVSeasonExists creates a TV season if it doesn't exist, or returns the existing one
func (r *Repository) ensureTVSeasonExists(ctx context.Context, showID int64, episode *media.TVEpisode) (media.TVSeason, error) {
	// Try to get existing season
	season, err := r.GetTVSeasonByShowAndNumber(ctx, showID, int64(episode.Season))
	if err == nil {
		return season, nil
	}

	// If not found, create it
	if errors.Is(err, media.ErrMediaNotFound) {
		return r.CreateTVSeason(ctx, showID, int64(episode.Season))
	}

	return media.TVSeason{}, err
}

// incrementSeasonEpisodeCount increments the episode count for a season
func (r *Repository) incrementSeasonEpisodeCount(ctx context.Context, seasonID int64) error {
	return common.ExecuteCommand(
		r.BaseRepository, ctx,
		func() error {
			return r.Postgres().IncrementSeasonEpisodeCount(ctx, int32(seasonID))
		},
		func() error {
			return r.SQLite().IncrementSeasonEpisodeCount(ctx, seasonID)
		},
	)
}
