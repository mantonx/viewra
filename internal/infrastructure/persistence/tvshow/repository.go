package tvshow

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	domainCommon "github.com/viewra/viewra/internal/domain/common"
	"github.com/viewra/viewra/internal/domain/media"
	"github.com/viewra/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/viewra/viewra/internal/infrastructure/persistence/common"
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
	_, err = r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return nil, r.SQLite().CreateTVEpisode(ctx, buildSQLiteCreateEpisodeParams(episode, show.ID, season.ID))
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
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().GetTVEpisodeByMediaID(ctx, id)
		},
	)
	if err != nil {
		return nil, r.ConvertNotFoundError(err)
	}

	// Convert to domain episode
	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}
	return sqliteEpisodeToDomain(result.(sqlc_sqlite.GetTVEpisodeByMediaIDRow)), nil
}

// ListTVEpisodesByLibrary retrieves all TV episodes in a specific library
func (r *Repository) ListTVEpisodesByLibrary(ctx context.Context, libraryID int64) ([]*media.TVEpisode, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().ListTVEpisodesByLibrary(ctx, libraryID)
		},
	)
	if err != nil {
		return nil, err
	}

	// Convert to domain episodes
	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	sqResults := result.([]sqlc_sqlite.ListTVEpisodesByLibraryRow)
	episodes := make([]*media.TVEpisode, len(sqResults))
	for i, sqResult := range sqResults {
		episodes[i] = sqliteEpisodeRowToDomain(convertToEpisodeFields(sqResult))
	}
	return episodes, nil
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
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().ListTVEpisodesByShow(ctx, showID)
		},
	)
	if err != nil {
		return nil, err
	}

	// Convert to domain episodes
	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	sqResults := result.([]sqlc_sqlite.ListTVEpisodesByShowRow)
	episodes := make([]*media.TVEpisode, len(sqResults))
	for i, sqResult := range sqResults {
		episodes[i] = sqliteEpisodeRowToDomain(convertToEpisodeFields(sqResult))
	}
	return episodes, nil
}

// UpdateTVEpisode modifies an existing TV episode
func (r *Repository) UpdateTVEpisode(ctx context.Context, episode *media.TVEpisode) error {
	// First, update the base media record
	if err := r.mediaRepo.Update(ctx, &episode.Media); err != nil {
		return fmt.Errorf("failed to update base media record: %w", err)
	}

	// Get the show to find its ID
	show, err := r.GetTVShowByTitle(ctx, episode.LibraryID, episode.ShowTitle)
	if err != nil {
		return fmt.Errorf("failed to find show: %w", err)
	}

	// Get the season to find its ID
	season, err := r.GetTVSeasonByShowAndNumber(ctx, show.ID, int64(episode.Season))
	if err != nil {
		return fmt.Errorf("failed to find season: %w", err)
	}

	// Update the episode record
	_, err = r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return nil, r.SQLite().UpdateTVEpisode(ctx, buildSQLiteUpdateEpisodeParams(episode, show.ID, season.ID))
		},
	)
	if err != nil {
		return fmt.Errorf("failed to update episode record: %w", err)
	}

	return nil
}

// SearchTVEpisodes searches for TV episodes by show title or episode title
func (r *Repository) SearchTVEpisodes(ctx context.Context, libraryID int64, query string) ([]*media.TVEpisode, error) {
	// Add wildcards for LIKE search
	searchPattern := "%" + query + "%"

	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().SearchTVEpisodesByTitle(ctx, sqlc_sqlite.SearchTVEpisodesByTitleParams{
				LibraryID:    libraryID,
				EpisodeTitle: common.NullString(searchPattern),
				Title:        searchPattern,
			})
		},
	)
	if err != nil {
		return nil, err
	}

	// Convert to domain episodes
	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	sqResults := result.([]sqlc_sqlite.SearchTVEpisodesByTitleRow)
	episodes := make([]*media.TVEpisode, len(sqResults))
	for i, sqResult := range sqResults {
		episodes[i] = sqliteEpisodeRowToDomain(convertToEpisodeFields(sqResult))
	}
	return episodes, nil
}

// --- TV Show Methods ---

// CreateTVShow adds a new TV show to the repository
func (r *Repository) CreateTVShow(ctx context.Context, libraryID int64, title string) (media.TVShow, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return media.TVShow{}, r.PostgresNotImplemented()
		},
		func() (any, error) {
			// Normalize the title for sorting
			sortTitle := domainCommon.NormalizeSortTitle(title)
			return r.SQLite().CreateTVShow(ctx, sqlc_sqlite.CreateTVShowParams{
				LibraryID:        libraryID,
				Title:            title,
				OriginalTitle:    sql.NullString{Valid: false},
				SortTitle:        sql.NullString{String: sortTitle, Valid: true},
				Year:             sql.NullInt64{Valid: false},
				FirstAirDate:     sql.NullTime{Valid: false},
				LastAirDate:      sql.NullTime{Valid: false},
				Genre:            sql.NullString{Valid: false},
				Plot:             sql.NullString{Valid: false},
				Status:           sql.NullString{Valid: false},
				ContentRating:    sql.NullString{Valid: false},
				MaturityRating:   sql.NullInt64{Valid: false},
				Network:          sql.NullString{Valid: false},
				OriginalLanguage: sql.NullString{Valid: false},
				CountryOfOrigin:  sql.NullString{Valid: false},
				ImdbID:           sql.NullString{Valid: false},
				TmdbID:           sql.NullInt64{Valid: false},
				TvdbID:           sql.NullInt64{Valid: false},
			})
		},
	)
	if err != nil {
		return media.TVShow{}, err
	}

	if r.Router().IsPostgresDB() {
		return media.TVShow{}, r.PostgresNotImplemented()
	}

	sqShow := result.(sqlc_sqlite.TvShow)
	return media.TVShow{
		ID:        sqShow.ID,
		LibraryID: sqShow.LibraryID,
		Title:     sqShow.Title,
	}, nil
}

// GetTVShowByID retrieves a TV show by its ID
func (r *Repository) GetTVShowByID(ctx context.Context, id int64) (media.TVShow, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().GetTVShowByID(ctx, id)
		},
	)
	if err != nil {
		return media.TVShow{}, r.ConvertNotFoundError(err)
	}

	if r.Router().IsPostgresDB() {
		return media.TVShow{}, r.PostgresNotImplemented()
	}

	sqShow := result.(sqlc_sqlite.TvShow)
	return media.TVShow{
		ID:        sqShow.ID,
		LibraryID: sqShow.LibraryID,
		Title:     sqShow.Title,
	}, nil
}

// GetTVShowByTitle retrieves a TV show by its title in a library
func (r *Repository) GetTVShowByTitle(ctx context.Context, libraryID int64, title string) (media.TVShow, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().GetTVShowByTitle(ctx, sqlc_sqlite.GetTVShowByTitleParams{
				LibraryID: libraryID,
				Title:     title,
			})
		},
	)
	if err != nil {
		return media.TVShow{}, r.ConvertNotFoundError(err)
	}

	if r.Router().IsPostgresDB() {
		return media.TVShow{}, r.PostgresNotImplemented()
	}

	sqShow := result.(sqlc_sqlite.TvShow)
	return media.TVShow{
		ID:        sqShow.ID,
		LibraryID: sqShow.LibraryID,
		Title:     sqShow.Title,
	}, nil
}

// ListTVShowsByLibrary retrieves all TV shows in a library
func (r *Repository) ListTVShowsByLibrary(ctx context.Context, libraryID int64) ([]media.TVShow, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().ListTVShowsByLibrary(ctx, libraryID)
		},
	)
	if err != nil {
		return nil, err
	}

	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	sqShows := result.([]sqlc_sqlite.TvShow)
	shows := make([]media.TVShow, len(sqShows))
	for i, sqShow := range sqShows {
		shows[i] = media.TVShow{
			ID:        sqShow.ID,
			LibraryID: sqShow.LibraryID,
			Title:     sqShow.Title,
		}
	}
	return shows, nil
}

// --- TV Season Methods ---

// CreateTVSeason adds a new TV season to the repository
func (r *Repository) CreateTVSeason(ctx context.Context, showID, seasonNumber int64) (media.TVSeason, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return media.TVSeason{}, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().CreateTVSeason(ctx, sqlc_sqlite.CreateTVSeasonParams{
				ShowID:       showID,
				SeasonNumber: seasonNumber,
				EpisodeCount: sql.NullInt64{Int64: 0, Valid: true},
			})
		},
	)
	if err != nil {
		return media.TVSeason{}, err
	}

	if r.Router().IsPostgresDB() {
		return media.TVSeason{}, r.PostgresNotImplemented()
	}

	sqSeason := result.(sqlc_sqlite.TvSeason)
	return media.TVSeason{
		ID:           sqSeason.ID,
		ShowID:       sqSeason.ShowID,
		SeasonNumber: sqSeason.SeasonNumber,
		EpisodeCount: int(common.ParseNullInt64(sqSeason.EpisodeCount)),
	}, nil
}

// GetTVSeasonByShowAndNumber retrieves a TV season by show ID and season number
func (r *Repository) GetTVSeasonByShowAndNumber(ctx context.Context, showID, seasonNumber int64) (media.TVSeason, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().GetTVSeasonByShowAndNumber(ctx, sqlc_sqlite.GetTVSeasonByShowAndNumberParams{
				ShowID:       showID,
				SeasonNumber: seasonNumber,
			})
		},
	)
	if err != nil {
		return media.TVSeason{}, r.ConvertNotFoundError(err)
	}

	if r.Router().IsPostgresDB() {
		return media.TVSeason{}, r.PostgresNotImplemented()
	}

	sqSeason := result.(sqlc_sqlite.TvSeason)
	return media.TVSeason{
		ID:           sqSeason.ID,
		ShowID:       sqSeason.ShowID,
		SeasonNumber: sqSeason.SeasonNumber,
		EpisodeCount: int(common.ParseNullInt64(sqSeason.EpisodeCount)),
	}, nil
}

// ListTVSeasonsByShow retrieves all seasons for a show
func (r *Repository) ListTVSeasonsByShow(ctx context.Context, showID int64) ([]media.TVSeason, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().ListTVSeasonsByShow(ctx, showID)
		},
	)
	if err != nil {
		return nil, err
	}

	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	sqSeasons := result.([]sqlc_sqlite.TvSeason)
	seasons := make([]media.TVSeason, len(sqSeasons))
	for i, sqSeason := range sqSeasons {
		seasons[i] = media.TVSeason{
			ID:           sqSeason.ID,
			ShowID:       sqSeason.ShowID,
			SeasonNumber: sqSeason.SeasonNumber,
			EpisodeCount: int(common.ParseNullInt64(sqSeason.EpisodeCount)),
		}
	}
	return seasons, nil
}

// CountTVShowsByLibrary returns the total count of TV shows in a library
func (r *Repository) CountTVShowsByLibrary(ctx context.Context, libraryID int64) (int64, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().CountTVShowsByLibrary(ctx, libraryID)
		},
	)
	if err != nil {
		return 0, err
	}

	if r.Router().IsPostgresDB() {
		return 0, r.PostgresNotImplemented()
	}

	return result.(int64), nil
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

	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			if sortBy == "title_desc" {
				return r.SQLite().GetTVShowsWithCountsByLibraryPaginatedDesc(ctx, sqlc_sqlite.GetTVShowsWithCountsByLibraryPaginatedDescParams{
					LibraryID: libraryID,
					Limit:     int64(pagination.Limit),
					Offset:    int64(pagination.Offset),
				})
			}
			return r.SQLite().GetTVShowsWithCountsByLibraryPaginated(ctx, sqlc_sqlite.GetTVShowsWithCountsByLibraryPaginatedParams{
				LibraryID: libraryID,
				Limit:     int64(pagination.Limit),
				Offset:    int64(pagination.Offset),
			})
		},
	)
	if err != nil {
		return nil, err
	}

	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	// Handle different row types based on sort order
	var shows []media.TVShowWithCounts
	if sortBy == "title_desc" {
		sqResults := result.([]sqlc_sqlite.GetTVShowsWithCountsByLibraryPaginatedDescRow)
		shows = make([]media.TVShowWithCounts, len(sqResults))
		for i, row := range sqResults {
			shows[i] = media.TVShowWithCounts{
				TVShow: media.TVShow{
					ID:        row.ID,
					LibraryID: row.LibraryID,
					Title:     row.Title,
				},
				SeasonCount:  row.SeasonCount,
				EpisodeCount: row.EpisodeCount,
			}
		}
	} else {
		sqResults := result.([]sqlc_sqlite.GetTVShowsWithCountsByLibraryPaginatedRow)
		shows = make([]media.TVShowWithCounts, len(sqResults))
		for i, row := range sqResults {
			shows[i] = media.TVShowWithCounts{
				TVShow: media.TVShow{
					ID:        row.ID,
					LibraryID: row.LibraryID,
					Title:     row.Title,
				},
				SeasonCount:  row.SeasonCount,
				EpisodeCount: row.EpisodeCount,
			}
		}
	}
	return shows, nil
}

// CountSearchTVShowsByTitle returns the count of TV shows matching a search query
func (r *Repository) CountSearchTVShowsByTitle(ctx context.Context, libraryID int64, query string) (int64, error) {
	searchPattern := "%" + query + "%"

	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().CountSearchTVShowsByTitle(ctx, sqlc_sqlite.CountSearchTVShowsByTitleParams{
				LibraryID:     libraryID,
				Title:         searchPattern,
				OriginalTitle: common.NullString(searchPattern),
			})
		},
	)
	if err != nil {
		return 0, err
	}

	if r.Router().IsPostgresDB() {
		return 0, r.PostgresNotImplemented()
	}

	return result.(int64), nil
}

// SearchTVShowsByTitlePaginated searches for TV shows by title with pagination
func (r *Repository) SearchTVShowsByTitlePaginated(ctx context.Context, libraryID int64, query string, pagination *domainCommon.PaginationParams) ([]media.TVShow, error) {
	if pagination == nil {
		pagination = domainCommon.DefaultPaginationParams()
	}

	searchPattern := "%" + query + "%"

	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().SearchTVShowsByTitlePaginated(ctx, sqlc_sqlite.SearchTVShowsByTitlePaginatedParams{
				LibraryID:     libraryID,
				Title:         searchPattern,
				OriginalTitle: common.NullString(searchPattern),
				Limit:         int64(pagination.Limit),
				Offset:        int64(pagination.Offset),
			})
		},
	)
	if err != nil {
		return nil, err
	}

	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	sqResults := result.([]sqlc_sqlite.TvShow)
	shows := make([]media.TVShow, len(sqResults))
	for i, row := range sqResults {
		shows[i] = media.TVShow{
			ID:        row.ID,
			LibraryID: row.LibraryID,
			Title:     row.Title,
		}
	}
	return shows, nil
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

	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			// Choose the appropriate query based on sort order
			if sortBy == "title_desc" {
				return r.SQLite().ListTVShowIDsByLibraryPaginatedDesc(ctx, sqlc_sqlite.ListTVShowIDsByLibraryPaginatedDescParams{
					LibraryID: libraryID,
					Limit:     int64(pagination.Limit),
					Offset:    int64(pagination.Offset),
				})
			}
			return r.SQLite().ListTVShowIDsByLibraryPaginated(ctx, sqlc_sqlite.ListTVShowIDsByLibraryPaginatedParams{
				LibraryID: libraryID,
				Limit:     int64(pagination.Limit),
				Offset:    int64(pagination.Offset),
			})
		},
	)
	if err != nil {
		return nil, err
	}

	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	return result.([]int64), nil
}

// --- Helper Methods ---

// ensureTVShowExists creates a TV show if it doesn't exist, or returns the existing one
func (r *Repository) ensureTVShowExists(ctx context.Context, episode *media.TVEpisode) (media.TVShow, error) {
	// Try to get existing show
	show, err := r.GetTVShowByTitle(ctx, episode.LibraryID, episode.ShowTitle)
	if err == nil {
		return show, nil
	}

	// If not found, create it
	if errors.Is(err, media.ErrMediaNotFound) {
		return r.CreateTVShow(ctx, episode.LibraryID, episode.ShowTitle)
	}

	return media.TVShow{}, err
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
	return r.Router().RouteVoid(
		func() error {
			return r.PostgresNotImplemented()
		},
		func() error {
			return r.SQLite().IncrementSeasonEpisodeCount(ctx, seasonID)
		},
	)
}
