package tvshow

import (
	"database/sql"
	"unsafe"

	domainCommon "github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/media"
	sqlc_postgres "github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// TVShow represents a simplified TV show record for internal repository use
type TVShow struct {
	ID        int64
	LibraryID int64
	Title     string
}

// TVSeason represents a simplified TV season record for internal repository use
type TVSeason struct {
	ID           int64
	ShowID       int64
	SeasonNumber int64
	EpisodeCount int
}

// sqliteEpisodeToDomain converts a SQLite TV episode query row to a domain TVEpisode entity
func sqliteEpisodeToDomain(row sqlc_sqlite.GetTVEpisodeByMediaIDRow) *media.TVEpisode {
	return &media.TVEpisode{
		Media: media.Media{
			ID:              row.MediaID_2,
			LibraryID:       row.LibraryID,
			Title:           row.Title,
			Type:            row.Type,
			FilePath:        row.FilePath,
			FileSize:        common.ParseNullInt64(row.FileSize),
			Duration:        int(common.ParseNullFloat64(row.Duration)),
			IsExtra:         row.IsExtra,
			Width:           int(common.ParseNullInt64(row.Width)),
			Height:          int(common.ParseNullInt64(row.Height)),
			VideoCodec:      common.ParseNullString(row.Codec),
			AudioCodec:      common.ParseNullString(row.AudioCodec),
			Bitrate:         common.ParseNullInt64(row.BitRate),
			FrameRate:       common.ParseNullFloat64(row.FrameRate),
			ContainerFormat: common.ParseNullString(row.ContainerFormat),
			CreatedAt:       common.ParseNullTime(row.CreatedAt),
			UpdatedAt:       common.ParseNullTime(row.UpdatedAt),
		},
		ShowTitle:    "", // Will be populated from show lookup if needed
		SeasonID:     row.SeasonID,
		Season:       int(row.SeasonNumber),
		Episode:      int(row.EpisodeNumber),
		EpisodeTitle: common.ParseNullString(row.EpisodeTitle),
		TVDbID:       int(common.ParseNullInt64(row.TvdbID)),
		IMDbID:       common.ParseNullString(row.ImdbID),
		AirDate:      common.ParseNullTime(row.AirDate).Format("2006-01-02"),
		Description:  common.ParseNullString(row.Plot),
	}
}

// episodeFields holds all the fields needed to construct a TVEpisode from various query row types
type episodeFields struct {
	MediaID        int64
	ShowID         int64
	SeasonID       int64
	SeasonNumber   int64
	EpisodeNumber  int64
	EpisodeTitle   sql.NullString
	TvdbID         sql.NullInt64
	ImdbID         sql.NullString
	AirDate        sql.NullTime
	Plot           sql.NullString
	MediaID2       int64
	LibraryID      int64
	Title          string
	FilePath       string
	FileSize       sql.NullInt64
	Duration       sql.NullFloat64
	Width          sql.NullInt64
	Height         sql.NullInt64
	Codec          sql.NullString
	AudioCodec     sql.NullString
	BitRate        sql.NullInt64
	FrameRate      sql.NullFloat64
	ContainerFormat sql.NullString
	Type           string
	IsExtra        bool
	CreatedAt      sql.NullTime
	UpdatedAt      sql.NullTime
}

// sqliteEpisodeRowToDomain converts episode fields to a domain TVEpisode entity
func sqliteEpisodeRowToDomain(fields episodeFields) *media.TVEpisode {
	return &media.TVEpisode{
		Media: media.Media{
			ID:              fields.MediaID2,
			LibraryID:       fields.LibraryID,
			Title:           fields.Title,
			Type:            fields.Type,
			FilePath:        fields.FilePath,
			FileSize:        common.ParseNullInt64(fields.FileSize),
			Duration:        int(common.ParseNullFloat64(fields.Duration)),
			IsExtra:         fields.IsExtra,
			Width:           int(common.ParseNullInt64(fields.Width)),
			Height:          int(common.ParseNullInt64(fields.Height)),
			VideoCodec:      common.ParseNullString(fields.Codec),
			AudioCodec:      common.ParseNullString(fields.AudioCodec),
			Bitrate:         common.ParseNullInt64(fields.BitRate),
			FrameRate:       common.ParseNullFloat64(fields.FrameRate),
			ContainerFormat: common.ParseNullString(fields.ContainerFormat),
			CreatedAt:       common.ParseNullTime(fields.CreatedAt),
			UpdatedAt:       common.ParseNullTime(fields.UpdatedAt),
		},
		ShowTitle:    "", // Will be populated from show lookup if needed
		SeasonID:     fields.SeasonID,
		Season:       int(fields.SeasonNumber),
		Episode:      int(fields.EpisodeNumber),
		EpisodeTitle: common.ParseNullString(fields.EpisodeTitle),
		TVDbID:       int(common.ParseNullInt64(fields.TvdbID)),
		IMDbID:       common.ParseNullString(fields.ImdbID),
		AirDate:      common.ParseNullTime(fields.AirDate).Format("2006-01-02"),
		Description:  common.ParseNullString(fields.Plot),
	}
}

// episodeRowLike is a constraint interface that describes the common fields
// present in all TV episode query row types
type episodeRowLike interface {
	sqlc_sqlite.ListTVEpisodesByLibraryRow |
		sqlc_sqlite.ListTVEpisodesByShowRow |
		sqlc_sqlite.SearchTVEpisodesByTitleRow
}

// convertToEpisodeFields is a generic function that converts any episode query row type
// to the common episodeFields struct. This eliminates duplication across three separate
// converter functions.
//
// Note: While we use a type constraint to ensure type safety, all three row types have
// identical field structures, so we use a helper function to access fields uniformly.
func convertToEpisodeFields[T episodeRowLike](row T) episodeFields {
	// All three row types have identical structures, so we can convert to any one
	// and use it. We convert to ListTVEpisodesByLibraryRow arbitrarily.
	var r sqlc_sqlite.ListTVEpisodesByLibraryRow

	switch typed := any(row).(type) {
	case sqlc_sqlite.ListTVEpisodesByLibraryRow:
		r = typed
	case sqlc_sqlite.ListTVEpisodesByShowRow:
		// Cast via any - safe because structures are identical
		r = *(*sqlc_sqlite.ListTVEpisodesByLibraryRow)(unsafe.Pointer(&typed))
	case sqlc_sqlite.SearchTVEpisodesByTitleRow:
		// Cast via any - safe because structures are identical
		r = *(*sqlc_sqlite.ListTVEpisodesByLibraryRow)(unsafe.Pointer(&typed))
	}

	return episodeFields{
		MediaID:         r.MediaID,
		ShowID:          r.ShowID,
		SeasonID:        r.SeasonID,
		SeasonNumber:    r.SeasonNumber,
		EpisodeNumber:   r.EpisodeNumber,
		EpisodeTitle:    r.EpisodeTitle,
		TvdbID:          r.TvdbID,
		ImdbID:          r.ImdbID,
		AirDate:         r.AirDate,
		Plot:            r.Plot,
		MediaID2:        r.MediaID_2,
		LibraryID:       r.LibraryID,
		Title:           r.Title,
		FilePath:        r.FilePath,
		FileSize:        r.FileSize,
		Duration:        r.Duration,
		Width:           r.Width,
		Height:          r.Height,
		Codec:           r.Codec,
		AudioCodec:      r.AudioCodec,
		BitRate:         r.BitRate,
		FrameRate:       r.FrameRate,
		ContainerFormat: r.ContainerFormat,
		Type:            r.Type,
		IsExtra:         r.IsExtra,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
}

// buildSQLiteCreateEpisodeParams builds CreateTVEpisodeParams for SQLite from a domain TVEpisode entity
func buildSQLiteCreateEpisodeParams(e *media.TVEpisode, showID, seasonID int64) sqlc_sqlite.CreateTVEpisodeParams {
	var airDate sql.NullTime
	if e.AirDate != "" {
		// Parse the air date if it's provided
		// For simplicity, we'll just store it as NullTime
		airDate = sql.NullTime{Valid: false}
	}

	return sqlc_sqlite.CreateTVEpisodeParams{
		MediaID:        e.Media.ID,
		ShowID:         showID,
		SeasonID:       seasonID,
		SeasonNumber:   int64(e.Season),
		EpisodeNumber:  int64(e.Episode),
		AbsoluteNumber: sql.NullInt64{},                         // TODO: Add AbsoluteNumber to domain.TVEpisode
		DvdSeason:      sql.NullInt64{},                         // TODO: Add DvdSeason to domain.TVEpisode
		DvdEpisode:     sql.NullInt64{},                         // TODO: Add DvdEpisode to domain.TVEpisode
		EpisodeTitle:   common.NullString(e.EpisodeTitle),
		OriginalTitle:  sql.NullString{},                        // TODO: Add OriginalTitle to domain.TVEpisode
		AirDate:        airDate,
		Plot:           common.NullString(e.Description),
		ContentRating:  sql.NullString{},                        // TODO: Add ContentRating to domain.TVEpisode
		MaturityRating: sql.NullInt64{},                         // TODO: Add MaturityRating to domain.TVEpisode
		ImdbID:         common.NullString(e.IMDbID),
		TmdbID:         sql.NullInt64{},                         // TODO: Add TMDbID to domain.TVEpisode
		TvdbID:         common.NullInt64(int64(e.TVDbID)),
	}
}

// buildSQLiteUpdateEpisodeParams builds UpdateTVEpisodeParams for SQLite from a domain TVEpisode entity
func buildSQLiteUpdateEpisodeParams(e *media.TVEpisode, showID, seasonID int64) sqlc_sqlite.UpdateTVEpisodeParams {
	params := buildSQLiteCreateEpisodeParams(e, showID, seasonID)
	return sqlc_sqlite.UpdateTVEpisodeParams{
		ShowID:         params.ShowID,
		SeasonID:       params.SeasonID,
		SeasonNumber:   params.SeasonNumber,
		EpisodeNumber:  params.EpisodeNumber,
		AbsoluteNumber: params.AbsoluteNumber,
		DvdSeason:      params.DvdSeason,
		DvdEpisode:     params.DvdEpisode,
		EpisodeTitle:   params.EpisodeTitle,
		OriginalTitle:  params.OriginalTitle,
		AirDate:        params.AirDate,
		Plot:           params.Plot,
		ContentRating:  params.ContentRating,
		MaturityRating: params.MaturityRating,
		ImdbID:         params.ImdbID,
		TmdbID:         params.TmdbID,
		TvdbID:         params.TvdbID,
		MediaID:        e.Media.ID,
	}
}

// ========================================
// PostgreSQL Mappers
// ========================================

// postgresEpisodeToDomain converts a PostgreSQL GetTVEpisodeByMediaIDRow to domain TVEpisode
func postgresEpisodeToDomain(row sqlc_postgres.GetTVEpisodeByMediaIDRow) *media.TVEpisode {
	return &media.TVEpisode{
		Media: media.Media{
			ID:              int64(row.MediaID_2),
			LibraryID:       int64(row.LibraryID),
			Title:           row.Title,
			Type:            row.Type,
			FilePath:        row.FilePath,
			FileSize:        common.ParseNullInt64(row.FileSize),
			Duration:        int(common.ParseNullFloat64(row.Duration)),
			IsExtra:         row.IsExtra,
			Width:           int(common.ParseNullInt64(common.ConvertInt32ToInt64(row.Width))),
			Height:          int(common.ParseNullInt64(common.ConvertInt32ToInt64(row.Height))),
			VideoCodec:      common.ParseNullString(row.Codec),
			AudioCodec:      common.ParseNullString(row.AudioCodec),
			Bitrate:         common.ParseNullInt64(row.BitRate),
			FrameRate:       common.ParseNullFloat64(row.FrameRate),
			ContainerFormat: common.ParseNullString(row.ContainerFormat),
			CreatedAt:       common.ParseNullTime(row.CreatedAt),
			UpdatedAt:       common.ParseNullTime(row.UpdatedAt),
		},
		ShowTitle:    "",
		SeasonID:     int64(row.SeasonID),
		Season:       int(row.SeasonNumber),
		Episode:      int(row.EpisodeNumber),
		EpisodeTitle: common.ParseNullString(row.EpisodeTitle),
		TVDbID:       int(common.ParseNullInt64(common.ConvertInt32ToInt64(row.TvdbID))),
		IMDbID:       common.ParseNullString(row.ImdbID),
		AirDate:      common.ParseNullTime(row.AirDate).Format("2006-01-02"),
		Description:  common.ParseNullString(row.Plot),
	}
}

// postgresEpisodeRow is a generic interface for all PostgreSQL TV episode query row types
type postgresEpisodeRow interface {
	sqlc_postgres.ListTVEpisodesByLibraryRow |
		sqlc_postgres.ListTVEpisodesByShowRow |
		sqlc_postgres.SearchTVEpisodesByTitleRow
}

// postgresEpisodeRowToDomain converts any PostgreSQL episode row type to domain TVEpisode
func postgresEpisodeRowToDomain[T postgresEpisodeRow](row T) *media.TVEpisode {
	var r sqlc_postgres.ListTVEpisodesByLibraryRow

	switch typed := any(row).(type) {
	case sqlc_postgres.ListTVEpisodesByLibraryRow:
		r = typed
	case sqlc_postgres.ListTVEpisodesByShowRow:
		r = *(*sqlc_postgres.ListTVEpisodesByLibraryRow)(unsafe.Pointer(&typed))
	case sqlc_postgres.SearchTVEpisodesByTitleRow:
		r = *(*sqlc_postgres.ListTVEpisodesByLibraryRow)(unsafe.Pointer(&typed))
	}

	return &media.TVEpisode{
		Media: media.Media{
			ID:              int64(r.MediaID_2),
			LibraryID:       int64(r.LibraryID),
			Title:           r.Title,
			Type:            r.Type,
			FilePath:        r.FilePath,
			FileSize:        common.ParseNullInt64(r.FileSize),
			Duration:        int(common.ParseNullFloat64(r.Duration)),
			IsExtra:         r.IsExtra,
			Width:           int(common.ParseNullInt64(common.ConvertInt32ToInt64(r.Width))),
			Height:          int(common.ParseNullInt64(common.ConvertInt32ToInt64(r.Height))),
			VideoCodec:      common.ParseNullString(r.Codec),
			AudioCodec:      common.ParseNullString(r.AudioCodec),
			Bitrate:         common.ParseNullInt64(r.BitRate),
			FrameRate:       common.ParseNullFloat64(r.FrameRate),
			ContainerFormat: common.ParseNullString(r.ContainerFormat),
			CreatedAt:       common.ParseNullTime(r.CreatedAt),
			UpdatedAt:       common.ParseNullTime(r.UpdatedAt),
		},
		ShowTitle:    "",
		SeasonID:     int64(r.SeasonID),
		Season:       int(r.SeasonNumber),
		Episode:      int(r.EpisodeNumber),
		EpisodeTitle: common.ParseNullString(r.EpisodeTitle),
		TVDbID:       int(common.ParseNullInt64(common.ConvertInt32ToInt64(r.TvdbID))),
		IMDbID:       common.ParseNullString(r.ImdbID),
		AirDate:      common.ParseNullTime(r.AirDate).Format("2006-01-02"),
		Description:  common.ParseNullString(r.Plot),
	}
}

// sqliteGenericEpisodeRowToDomain converts any SQLite episode row type to domain TVEpisode
func sqliteGenericEpisodeRowToDomain[T episodeRowLike](row T) *media.TVEpisode {
	fields := convertToEpisodeFields(row)
	return sqliteEpisodeRowToDomain(fields)
}

// buildPostgresCreateEpisodeParams builds CreateTVEpisodeParams for PostgreSQL from a domain TVEpisode entity
func buildPostgresCreateEpisodeParams(e *media.TVEpisode, showID, seasonID int64) sqlc_postgres.CreateTVEpisodeParams {
	var airDate sql.NullTime
	if e.AirDate != "" {
		airDate = sql.NullTime{Valid: false}
	}

	return sqlc_postgres.CreateTVEpisodeParams{
		MediaID:        int32(e.Media.ID),
		ShowID:         int32(showID),
		SeasonID:       int32(seasonID),
		SeasonNumber:   int32(e.Season),
		EpisodeNumber:  int32(e.Episode),
		AbsoluteNumber: sql.NullInt32{},
		DvdSeason:      sql.NullInt32{},
		DvdEpisode:     sql.NullInt32{},
		EpisodeTitle:   common.NullString(e.EpisodeTitle),
		OriginalTitle:  sql.NullString{},
		AirDate:        airDate,
		Plot:           common.NullString(e.Description),
		ContentRating:  sql.NullString{},
		MaturityRating: sql.NullInt32{},
		ImdbID:         common.NullString(e.IMDbID),
		TmdbID:         sql.NullInt32{},
		TvdbID:         common.NullInt32FromInt64(int64(e.TVDbID)),
	}
}

// buildPostgresUpdateEpisodeParams builds UpdateTVEpisodeParams for PostgreSQL from a domain TVEpisode entity
func buildPostgresUpdateEpisodeParams(e *media.TVEpisode, showID, seasonID int64) sqlc_postgres.UpdateTVEpisodeParams {
	params := buildPostgresCreateEpisodeParams(e, showID, seasonID)
	return sqlc_postgres.UpdateTVEpisodeParams{
		ShowID:         params.ShowID,
		SeasonID:       params.SeasonID,
		SeasonNumber:   params.SeasonNumber,
		EpisodeNumber:  params.EpisodeNumber,
		AbsoluteNumber: params.AbsoluteNumber,
		DvdSeason:      params.DvdSeason,
		DvdEpisode:     params.DvdEpisode,
		EpisodeTitle:   params.EpisodeTitle,
		OriginalTitle:  params.OriginalTitle,
		AirDate:        params.AirDate,
		Plot:           params.Plot,
		ContentRating:  params.ContentRating,
		MaturityRating: params.MaturityRating,
		ImdbID:         params.ImdbID,
		TmdbID:         params.TmdbID,
		TvdbID:         params.TvdbID,
		MediaID:        int32(e.Media.ID),
	}
}

// ========================================
// TV Show Converters
// ========================================

// postgresShowToDomain converts a PostgreSQL TvShow row to domain TVShow
func postgresShowToDomain(row sqlc_postgres.TvShow) media.TVShow {
	return media.TVShow{
		ID:        int64(row.ID),
		LibraryID: int64(row.LibraryID),
		Title:     row.Title,
	}
}

// sqliteShowToDomain converts a SQLite TvShow row to domain TVShow
func sqliteShowToDomain(row sqlc_sqlite.TvShow) media.TVShow {
	return media.TVShow{
		ID:        row.ID,
		LibraryID: row.LibraryID,
		Title:     row.Title,
	}
}

// buildPostgresCreateTVShowParams builds CreateTVShowParams for PostgreSQL
func buildPostgresCreateTVShowParams(libraryID int64, title string) sqlc_postgres.CreateTVShowParams {
	sortTitle := domainCommon.NormalizeSortTitle(title)
	return sqlc_postgres.CreateTVShowParams{
		LibraryID:        int32(libraryID),
		Title:            title,
		OriginalTitle:    sql.NullString{Valid: false},
		SortTitle:        sql.NullString{String: sortTitle, Valid: true},
		Year:             sql.NullInt32{Valid: false},
		FirstAirDate:     sql.NullTime{Valid: false},
		LastAirDate:      sql.NullTime{Valid: false},
		Genre:            sql.NullString{Valid: false},
		Plot:             sql.NullString{Valid: false},
		Status:           sql.NullString{Valid: false},
		ContentRating:    sql.NullString{Valid: false},
		MaturityRating:   sql.NullInt32{Valid: false},
		Network:          sql.NullString{Valid: false},
		OriginalLanguage: sql.NullString{Valid: false},
		CountryOfOrigin:  sql.NullString{Valid: false},
		ImdbID:           sql.NullString{Valid: false},
		TmdbID:           sql.NullInt32{Valid: false},
		TvdbID:           sql.NullInt32{Valid: false},
	}
}

// buildSQLiteCreateTVShowParams builds CreateTVShowParams for SQLite
func buildSQLiteCreateTVShowParams(libraryID int64, title string) sqlc_sqlite.CreateTVShowParams {
	sortTitle := domainCommon.NormalizeSortTitle(title)
	return sqlc_sqlite.CreateTVShowParams{
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
	}
}

// postgresShowWithCountsToDomain converts PostgreSQL rows with counts to domain TVShowWithCounts
func postgresShowWithCountsToDomain(row sqlc_postgres.GetTVShowsWithCountsByLibraryPaginatedRow) media.TVShowWithCounts {
	return media.TVShowWithCounts{
		TVShow: media.TVShow{
			ID:        int64(row.ID),
			LibraryID: int64(row.LibraryID),
			Title:     row.Title,
		},
		SeasonCount:  row.SeasonCount,
		EpisodeCount: row.EpisodeCount,
	}
}

func postgresShowWithCountsDescToDomain(row sqlc_postgres.GetTVShowsWithCountsByLibraryPaginatedDescRow) media.TVShowWithCounts {
	return media.TVShowWithCounts{
		TVShow: media.TVShow{
			ID:        int64(row.ID),
			LibraryID: int64(row.LibraryID),
			Title:     row.Title,
		},
		SeasonCount:  row.SeasonCount,
		EpisodeCount: row.EpisodeCount,
	}
}

func sqliteShowWithCountsToDomain(row sqlc_sqlite.GetTVShowsWithCountsByLibraryPaginatedRow) media.TVShowWithCounts {
	return media.TVShowWithCounts{
		TVShow: media.TVShow{
			ID:        row.ID,
			LibraryID: row.LibraryID,
			Title:     row.Title,
		},
		SeasonCount:  row.SeasonCount,
		EpisodeCount: row.EpisodeCount,
	}
}

func sqliteShowWithCountsDescToDomain(row sqlc_sqlite.GetTVShowsWithCountsByLibraryPaginatedDescRow) media.TVShowWithCounts {
	return media.TVShowWithCounts{
		TVShow: media.TVShow{
			ID:        row.ID,
			LibraryID: row.LibraryID,
			Title:     row.Title,
		},
		SeasonCount:  row.SeasonCount,
		EpisodeCount: row.EpisodeCount,
	}
}

// ========================================
// TV Season Converters
// ========================================

// postgresSeasonToDomain converts a PostgreSQL TvSeason row to domain TVSeason
func postgresSeasonToDomain(row sqlc_postgres.TvSeason) media.TVSeason {
	return media.TVSeason{
		ID:           int64(row.ID),
		ShowID:       int64(row.ShowID),
		SeasonNumber: int64(row.SeasonNumber),
		EpisodeCount: int(common.ParseNullInt32(row.EpisodeCount)),
	}
}

// sqliteSeasonToDomain converts a SQLite TvSeason row to domain TVSeason
func sqliteSeasonToDomain(row sqlc_sqlite.TvSeason) media.TVSeason {
	return media.TVSeason{
		ID:           row.ID,
		ShowID:       row.ShowID,
		SeasonNumber: row.SeasonNumber,
		EpisodeCount: int(common.ParseNullInt64(row.EpisodeCount)),
	}
}

// buildPostgresCreateTVSeasonParams builds CreateTVSeasonParams for PostgreSQL
func buildPostgresCreateTVSeasonParams(showID, seasonNumber int64) sqlc_postgres.CreateTVSeasonParams {
	return sqlc_postgres.CreateTVSeasonParams{
		ShowID:       int32(showID),
		SeasonNumber: int32(seasonNumber),
		EpisodeCount: sql.NullInt32{Int32: 0, Valid: true},
	}
}

// buildSQLiteCreateTVSeasonParams builds CreateTVSeasonParams for SQLite
func buildSQLiteCreateTVSeasonParams(showID, seasonNumber int64) sqlc_sqlite.CreateTVSeasonParams {
	return sqlc_sqlite.CreateTVSeasonParams{
		ShowID:       showID,
		SeasonNumber: seasonNumber,
		EpisodeCount: sql.NullInt64{Int64: 0, Valid: true},
	}
}
