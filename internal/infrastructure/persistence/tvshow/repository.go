package tvshow

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	domainCommon "github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
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
	if err := r.Q().CreateTVEpisode(ctx, buildCreateEpisodeParams(episode, show.ID, season.ID)); err != nil {
		return fmt.Errorf("failed to create episode record: %w", err)
	}

	// Increment the season's episode count
	if err := r.Q().IncrementSeasonEpisodeCount(ctx, season.ID); err != nil {
		return fmt.Errorf("failed to increment season episode count: %w", err)
	}

	return nil
}

// GetTVEpisodeByID retrieves a TV episode by its media ID
func (r *Repository) GetTVEpisodeByID(ctx context.Context, id int64) (*media.TVEpisode, error) {
	row, err := r.Q().GetTVEpisodeByMediaID(ctx, id)
	if err != nil {
		return nil, r.ConvertNotFoundError(err)
	}
	return episodeRowToDomain(row), nil
}

// ListTVEpisodesByLibrary retrieves all TV episodes in a specific library
func (r *Repository) ListTVEpisodesByLibrary(ctx context.Context, libraryID int64) ([]*media.TVEpisode, error) {
	rows, err := r.Q().ListTVEpisodesByLibrary(ctx, libraryID)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, listEpisodeRowToDomain), nil
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
	rows, err := r.Q().ListTVEpisodesByShow(ctx, showID)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, func(row unified.ListTVEpisodesByShowRow) *media.TVEpisode {
		return listEpisodeRowToDomain(unified.ListTVEpisodesByLibraryRow(row))
	}), nil
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
	return r.Q().UpdateTVEpisode(ctx, buildUpdateEpisodeParams(episode, showID, seasonID))
}

// SearchTVEpisodes searches for TV episodes by show title or episode title
func (r *Repository) SearchTVEpisodes(ctx context.Context, libraryID int64, query string) ([]*media.TVEpisode, error) {
	// Add wildcards for LIKE search
	searchPattern := "%" + query + "%"

	rows, err := r.Q().SearchTVEpisodesByTitle(ctx, unified.SearchTVEpisodesByTitleParams{
		LibraryID:    libraryID,
		EpisodeTitle: common.NullString(searchPattern),
		Title:        searchPattern,
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, searchEpisodeRowToDomain), nil
}

// --- TV Show Methods ---

// CreateTVShow adds a new TV show to the repository
func (r *Repository) CreateTVShow(ctx context.Context, libraryID int64, title, directory string) (media.TVShow, error) {
	row, err := r.Q().CreateTVShow(ctx, buildCreateTVShowParams(libraryID, title, directory))
	if err != nil {
		return media.TVShow{}, err
	}
	return showToDomain(row), nil
}

// GetTVShowByID retrieves a TV show by its ID
func (r *Repository) GetTVShowByID(ctx context.Context, id int64) (media.TVShow, error) {
	row, err := r.Q().GetTVShowByID(ctx, id)
	if err != nil {
		return media.TVShow{}, r.ConvertNotFoundError(err)
	}
	return showToDomain(row), nil
}

// GetTVShowByTitle retrieves a TV show by its title in a library.
// The title is normalized before lookup to match how titles are stored.
func (r *Repository) GetTVShowByTitle(ctx context.Context, libraryID int64, title string) (media.TVShow, error) {
	// Normalize title to match how it's stored in the database
	normalizedTitle := domainCommon.NormalizeTitle(title)

	row, err := r.Q().GetTVShowByTitle(ctx, unified.GetTVShowByTitleParams{
		LibraryID: libraryID,
		Title:     normalizedTitle,
	})
	if err != nil {
		return media.TVShow{}, r.ConvertNotFoundError(err)
	}
	return showToDomain(row), nil
}

// UpdateTVShow updates an existing TV show's metadata
func (r *Repository) UpdateTVShow(ctx context.Context, show media.TVShow) error {
	return r.Q().UpdateTVShow(ctx, buildUpdateTVShowParams(show))
}

// ListTVShowsByLibrary retrieves all TV shows in a library
func (r *Repository) ListTVShowsByLibrary(ctx context.Context, libraryID int64) ([]media.TVShow, error) {
	rows, err := r.Q().ListTVShowsByLibrary(ctx, libraryID)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, showToDomain), nil
}

// --- TV Season Methods ---

// CreateTVSeason adds a new TV season to the repository
func (r *Repository) CreateTVSeason(ctx context.Context, showID, seasonNumber int64) (media.TVSeason, error) {
	row, err := r.Q().CreateTVSeason(ctx, buildCreateTVSeasonParams(showID, seasonNumber))
	if err != nil {
		return media.TVSeason{}, err
	}
	return seasonToDomain(row), nil
}

// GetTVSeasonByID retrieves a TV season by its ID
func (r *Repository) GetTVSeasonByID(ctx context.Context, id int64) (media.TVSeason, error) {
	row, err := r.Q().GetTVSeasonByID(ctx, id)
	if err != nil {
		return media.TVSeason{}, r.ConvertNotFoundError(err)
	}
	return seasonToDomain(row), nil
}

// GetTVSeasonByShowAndNumber retrieves a TV season by show ID and season number
func (r *Repository) GetTVSeasonByShowAndNumber(ctx context.Context, showID, seasonNumber int64) (media.TVSeason, error) {
	row, err := r.Q().GetTVSeasonByShowAndNumber(ctx, unified.GetTVSeasonByShowAndNumberParams{
		ShowID:       showID,
		SeasonNumber: seasonNumber,
	})
	if err != nil {
		return media.TVSeason{}, r.ConvertNotFoundError(err)
	}
	return seasonToDomain(row), nil
}

// ListTVSeasonsByShow retrieves all seasons for a show
func (r *Repository) ListTVSeasonsByShow(ctx context.Context, showID int64) ([]media.TVSeason, error) {
	rows, err := r.Q().ListTVSeasonsByShow(ctx, showID)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, seasonToDomain), nil
}

// CountTVShowsByLibrary returns the total count of TV shows in a library
func (r *Repository) CountTVShowsByLibrary(ctx context.Context, libraryID int64) (int64, error) {
	return r.Q().CountTVShowsByLibrary(ctx, libraryID)
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
		rows, err := r.Q().GetTVShowsWithCountsByLibraryPaginatedDesc(ctx, unified.GetTVShowsWithCountsByLibraryPaginatedDescParams{
			LibraryID: libraryID,
			Limit:     int64(pagination.Limit),
			Offset:    int64(pagination.Offset),
		})
		if err != nil {
			return nil, err
		}
		return mapSlice(rows, showWithCountsDescToDomain), nil
	}

	rows, err := r.Q().GetTVShowsWithCountsByLibraryPaginated(ctx, unified.GetTVShowsWithCountsByLibraryPaginatedParams{
		LibraryID: libraryID,
		Limit:     int64(pagination.Limit),
		Offset:    int64(pagination.Offset),
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, showWithCountsToDomain), nil
}

// CountSearchTVShowsByTitle returns the count of TV shows matching a search query
func (r *Repository) CountSearchTVShowsByTitle(ctx context.Context, libraryID int64, query string) (int64, error) {
	searchPattern := "%" + query + "%"

	return r.Q().CountSearchTVShowsByTitle(ctx, unified.CountSearchTVShowsByTitleParams{
		LibraryID:     libraryID,
		Title:         searchPattern,
		OriginalTitle: common.NullString(searchPattern),
	})
}

// SearchTVShowsByTitlePaginated searches for TV shows by title with pagination
func (r *Repository) SearchTVShowsByTitlePaginated(ctx context.Context, libraryID int64, query string, pagination *domainCommon.PaginationParams) ([]media.TVShow, error) {
	if pagination == nil {
		pagination = domainCommon.DefaultPaginationParams()
	}

	searchPattern := "%" + query + "%"

	rows, err := r.Q().SearchTVShowsByTitlePaginated(ctx, unified.SearchTVShowsByTitlePaginatedParams{
		LibraryID:     libraryID,
		Title:         searchPattern,
		OriginalTitle: common.NullString(searchPattern),
		Limit:         int64(pagination.Limit),
		Offset:        int64(pagination.Offset),
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, showToDomain), nil
}

// SearchTVShowsWithCountsByTitlePaginated searches for TV shows by title with counts and pagination
func (r *Repository) SearchTVShowsWithCountsByTitlePaginated(ctx context.Context, libraryID int64, query string, pagination *domainCommon.PaginationParams) ([]media.TVShowWithCounts, error) {
	if pagination == nil {
		pagination = domainCommon.DefaultPaginationParams()
	}

	searchPattern := "%" + query + "%"

	rows, err := r.Q().SearchTVShowsWithCountsByTitlePaginated(ctx, unified.SearchTVShowsWithCountsByTitlePaginatedParams{
		LibraryID:     libraryID,
		Title:         searchPattern,
		OriginalTitle: common.NullString(searchPattern),
		Limit:         int64(pagination.Limit),
		Offset:        int64(pagination.Offset),
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, searchShowWithCountsToDomain), nil
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
		return r.Q().ListTVShowIDsByLibraryPaginatedDesc(ctx, unified.ListTVShowIDsByLibraryPaginatedDescParams{
			LibraryID: libraryID,
			Limit:     int64(pagination.Limit),
			Offset:    int64(pagination.Offset),
		})
	}

	return r.Q().ListTVShowIDsByLibraryPaginated(ctx, unified.ListTVShowIDsByLibraryPaginatedParams{
		LibraryID: libraryID,
		Limit:     int64(pagination.Limit),
		Offset:    int64(pagination.Offset),
	})
}

// --- Helper Methods ---

// ensureTVShowExists atomically creates a TV show if it doesn't exist, or returns the existing one.
// Uses a two-phase lookup to prevent duplicates:
// 1. First checks if a show already exists for this directory (prevents duplicates from title variations)
// 2. Falls back to title-based upsert if no directory match exists
func (r *Repository) ensureTVShowExists(ctx context.Context, episode *media.TVEpisode) (media.TVShow, error) {
	showDir := deriveShowDirectory(episode.Media.FilePath)

	// Phase 1: Check if a show already exists for this directory.
	// This prevents duplicates when different episodes in the same folder
	// parse to different titles (e.g., "Looney Tunes" vs "New Looney Tunes").
	if showDir != "" {
		dirParam := sql.NullString{String: showDir, Valid: true}
		existingShow, err := r.Q().GetTVShowByDirectory(ctx, unified.GetTVShowByDirectoryParams{
			LibraryID: episode.LibraryID,
			Directory: dirParam,
		})
		if err == nil {
			// Found existing show for this directory, use it
			return showToDomain(existingShow), nil
		}
		// If not found, fall through to title-based upsert
	}

	// Phase 2: Use atomic upsert by title - this handles the case where the show
	// doesn't exist yet or we couldn't find it by directory
	row, err := r.Q().UpsertTVShow(ctx, buildUpsertTVShowParams(episode.LibraryID, episode.ShowTitle, showDir))
	if err != nil {
		return media.TVShow{}, err
	}
	return showToDomain(row), nil
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

// ListRecentlyAddedShows returns recently added TV shows across all libraries.
func (r *Repository) ListRecentlyAddedShows(ctx context.Context, limit int) ([]media.TVShow, error) {
	rows, err := r.Q().ListRecentlyAddedTVShows(ctx, int64(limit))
	if err != nil {
		return nil, err
	}

	shows := make([]media.TVShow, len(rows))
	for i, row := range rows {
		shows[i] = media.TVShow{
			ID:            row.ID,
			LibraryID:     row.LibraryID,
			Title:         row.Title,
			OriginalTitle: common.ParseNullString(row.OriginalTitle),
			SortTitle:     common.ParseNullString(row.SortTitle),
			Year:          int(common.ParseNullInt64(row.Year)),
			Genre:         parseGenres(common.ParseNullString(row.Genre)),
			Plot:          common.ParseNullString(row.Plot),
			ContentRating: common.ParseNullString(row.ContentRating),
			IMDbID:        common.ParseNullString(row.ImdbID),
			TMDbID:        int(common.ParseNullInt64(row.TmdbID)),
			CreatedAt:     common.ParseNullTime(row.CreatedAt),
			UpdatedAt:     common.ParseNullTime(row.UpdatedAt),
		}
	}
	return shows, nil
}

// mapSlice converts a slice of one type to another using the provided mapper function.
func mapSlice[TFrom, TTo any](from []TFrom, mapper func(TFrom) TTo) []TTo {
	result := make([]TTo, len(from))
	for i, v := range from {
		result[i] = mapper(v)
	}
	return result
}
