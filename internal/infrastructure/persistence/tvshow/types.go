package tvshow

import (
	"database/sql"
	"strings"

	domainCommon "github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/media"
	sqlc_postgres "github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// ========================================
// Generic TV Episode Mapper
// ========================================

// mapEpisodeToDomain converts any TV episode query row to domain TVEpisode
// Handles both SQLite and PostgreSQL row types using reflection-based field getters
func mapEpisodeToDomain(row interface{}) *media.TVEpisode {
	return &media.TVEpisode{
		Media: media.Media{
			ID:              common.IntFieldGetter(row, "MediaID_2"),
			LibraryID:       common.IntFieldGetter(row, "LibraryID"),
			Title:           common.StringFieldGetter(row, "Title"),
			Type:            common.StringFieldGetter(row, "Type"),
			FilePath:        common.StringFieldGetter(row, "FilePath"),
			FileSize:        common.ParseNullInt64(common.NullIntFieldGetter(row, "FileSize")),
			Duration:        int(common.Float64FieldGetter(row, "Duration")),
			IsExtra:         common.BoolFieldGetter(row, "IsExtra"),
			Width:           int(common.ParseNullInt64(common.NullIntFieldGetter(row, "Width"))),
			Height:          int(common.ParseNullInt64(common.NullIntFieldGetter(row, "Height"))),
			VideoCodec:      common.ParseNullString(common.NullStringFieldGetter(row, "Codec")),
			AudioCodec:      common.ParseNullString(common.NullStringFieldGetter(row, "AudioCodec")),
			Bitrate:         common.ParseNullInt64(common.NullIntFieldGetter(row, "BitRate")),
			FrameRate:       common.Float64FieldGetter(row, "FrameRate"),
			ContainerFormat: common.ParseNullString(common.NullStringFieldGetter(row, "ContainerFormat")),
			CreatedAt:       common.ParseNullTime(common.TimeFieldGetter(row, "CreatedAt")),
			UpdatedAt:       common.ParseNullTime(common.TimeFieldGetter(row, "UpdatedAt")),
		},
		ShowID:         common.IntFieldGetter(row, "ShowID"),
		ShowTitle:      "", // Will be populated from show lookup if needed
		SeasonID:       common.IntFieldGetter(row, "SeasonID"),
		Season:         int(common.IntFieldGetter(row, "SeasonNumber")),
		Episode:        int(common.IntFieldGetter(row, "EpisodeNumber")),
		EpisodeTitle:   common.ParseNullString(common.NullStringFieldGetter(row, "EpisodeTitle")),
		TVDbID:         int(common.ParseNullInt64(common.NullIntFieldGetter(row, "TvdbID"))),
		IMDbID:         common.ParseNullString(common.NullStringFieldGetter(row, "ImdbID")),
		AirDate:        common.FormatNullDate(common.TimeFieldGetter(row, "AirDate")),
		Description:    common.ParseNullString(common.NullStringFieldGetter(row, "Plot")),
		RuntimeMinutes: int(common.ParseNullInt64(common.NullIntFieldGetter(row, "RuntimeMinutes"))),
		Rating:         float32(common.Float64FieldGetter(row, "Rating")),
		RatingVotes:    int(common.ParseNullInt64(common.NullIntFieldGetter(row, "RatingVotes"))),
	}
}

// ========================================
// Generic TV Show Mapper
// ========================================

// mapShowToDomain converts any TV show row to domain TVShow
// Handles both SQLite and PostgreSQL row types using reflection-based field getters
func mapShowToDomain(row interface{}) media.TVShow {
	return media.TVShow{
		ID:               common.IntFieldGetter(row, "ID"),
		LibraryID:        common.IntFieldGetter(row, "LibraryID"),
		Title:            common.StringFieldGetter(row, "Title"),
		OriginalTitle:    common.ParseNullString(common.NullStringFieldGetter(row, "OriginalTitle")),
		SortTitle:        common.ParseNullString(common.NullStringFieldGetter(row, "SortTitle")),
		Year:             int(common.ParseNullInt64(common.NullIntFieldGetter(row, "Year"))),
		FirstAirDate:     common.FormatNullDate(common.TimeFieldGetter(row, "FirstAirDate")),
		LastAirDate:      common.FormatNullDate(common.TimeFieldGetter(row, "LastAirDate")),
		Genre:            parseGenres(common.ParseNullString(common.NullStringFieldGetter(row, "Genre"))),
		Plot:             common.ParseNullString(common.NullStringFieldGetter(row, "Plot")),
		Status:           common.ParseNullString(common.NullStringFieldGetter(row, "Status")),
		ContentRating:    common.ParseNullString(common.NullStringFieldGetter(row, "ContentRating")),
		Network:          common.ParseNullString(common.NullStringFieldGetter(row, "Network")),
		OriginalLanguage: common.ParseNullString(common.NullStringFieldGetter(row, "OriginalLanguage")),
		CountryOfOrigin:  common.ParseNullString(common.NullStringFieldGetter(row, "CountryOfOrigin")),
		IMDbID:           common.ParseNullString(common.NullStringFieldGetter(row, "ImdbID")),
		TMDbID:           int(common.ParseNullInt64(common.NullIntFieldGetter(row, "TmdbID"))),
		TVDbID:           int(common.ParseNullInt64(common.NullIntFieldGetter(row, "TvdbID"))),
		Directory:        common.ParseNullString(common.NullStringFieldGetter(row, "Directory")),
		Tagline:         common.ParseNullString(common.NullStringFieldGetter(row, "Tagline")),
		Rating:          float32(common.Float64FieldGetter(row, "Rating")),
		RatingVotes:     int(common.ParseNullInt64(common.NullIntFieldGetter(row, "RatingVotes"))),
	}
}

// ========================================
// Generic TV Season Mapper
// ========================================

// mapSeasonToDomain converts any TV season row to domain TVSeason
// Handles both SQLite and PostgreSQL row types using reflection-based field getters
func mapSeasonToDomain(row interface{}) media.TVSeason {
	return media.TVSeason{
		ID:           common.IntFieldGetter(row, "ID"),
		ShowID:       common.IntFieldGetter(row, "ShowID"),
		SeasonNumber: common.IntFieldGetter(row, "SeasonNumber"),
		EpisodeCount: int(common.ParseNullInt64(common.NullIntFieldGetter(row, "EpisodeCount"))),
	}
}

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
	return mapEpisodeToDomain(row)
}

// episodeRowLike is a constraint interface that describes the common fields
// present in all TV episode query row types
type episodeRowLike interface {
	sqlc_sqlite.ListTVEpisodesByLibraryRow |
		sqlc_sqlite.ListTVEpisodesByShowRow |
		sqlc_sqlite.SearchTVEpisodesByTitleRow
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
		Rating:         common.NullFloat64FromFloat32(e.Rating),
		RatingVotes:    common.NullInt64(int64(e.RatingVotes)),
		RuntimeMinutes: common.NullInt64(int64(e.RuntimeMinutes)),
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
		Rating:         params.Rating,
		RatingVotes:    params.RatingVotes,
		RuntimeMinutes: params.RuntimeMinutes,
		MediaID:        e.Media.ID,
	}
}

// ========================================
// PostgreSQL Mappers
// ========================================

// postgresEpisodeToDomain converts a PostgreSQL GetTVEpisodeByMediaIDRow to domain TVEpisode
func postgresEpisodeToDomain(row sqlc_postgres.GetTVEpisodeByMediaIDRow) *media.TVEpisode {
	return mapEpisodeToDomain(row)
}

// postgresEpisodeRow is a generic interface for all PostgreSQL TV episode query row types
type postgresEpisodeRow interface {
	sqlc_postgres.ListTVEpisodesByLibraryRow |
		sqlc_postgres.ListTVEpisodesByShowRow |
		sqlc_postgres.SearchTVEpisodesByTitleRow
}

// postgresEpisodeRowToDomain converts any PostgreSQL episode row type to domain TVEpisode
func postgresEpisodeRowToDomain[T postgresEpisodeRow](row T) *media.TVEpisode {
	return mapEpisodeToDomain(row)
}

// sqliteGenericEpisodeRowToDomain converts any SQLite episode row type to domain TVEpisode
func sqliteGenericEpisodeRowToDomain[T episodeRowLike](row T) *media.TVEpisode {
	return mapEpisodeToDomain(row)
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
		Rating:         common.NullFloat64FromFloat32(e.Rating),
		RatingVotes:    common.NullInt32FromInt64(int64(e.RatingVotes)),
		RuntimeMinutes: common.NullInt32FromInt64(int64(e.RuntimeMinutes)),
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
		Rating:         params.Rating,
		RatingVotes:    params.RatingVotes,
		RuntimeMinutes: params.RuntimeMinutes,
		MediaID:        int32(e.Media.ID),
	}
}

// ========================================
// TV Show Converters
// ========================================

// postgresShowToDomain converts a PostgreSQL TvShow row to domain TVShow
func postgresShowToDomain(row sqlc_postgres.TvShow) media.TVShow {
	return mapShowToDomain(row)
}

// sqliteShowToDomain converts a SQLite TvShow row to domain TVShow
func sqliteShowToDomain(row sqlc_sqlite.TvShow) media.TVShow {
	return mapShowToDomain(row)
}

// buildPostgresCreateTVShowParams builds CreateTVShowParams for PostgreSQL
func buildPostgresCreateTVShowParams(libraryID int64, title, directory string) sqlc_postgres.CreateTVShowParams {
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
		Directory:        sql.NullString{String: directory, Valid: directory != ""},
		Rating:           sql.NullFloat64{Valid: false},
		RatingVotes:      sql.NullInt32{Valid: false},
		Tagline:          sql.NullString{Valid: false},
	}
}

// buildSQLiteCreateTVShowParams builds CreateTVShowParams for SQLite
func buildSQLiteCreateTVShowParams(libraryID int64, title, directory string) sqlc_sqlite.CreateTVShowParams {
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
		Directory:        sql.NullString{String: directory, Valid: directory != ""},
		Rating:           sql.NullFloat64{Valid: false},
		RatingVotes:      sql.NullInt64{Valid: false},
		Tagline:          sql.NullString{Valid: false},
	}
}

// buildPostgresUpsertTVShowParams builds UpsertTVShowParams for PostgreSQL
func buildPostgresUpsertTVShowParams(libraryID int64, title, directory string) sqlc_postgres.UpsertTVShowParams {
	// Normalize title to handle punctuation variants (e.g., "Star Trek: Voyager" vs "Star Trek Voyager")
	normalizedTitle := domainCommon.NormalizeTitle(title)
	sortTitle := domainCommon.NormalizeSortTitle(normalizedTitle)
	return sqlc_postgres.UpsertTVShowParams{
		LibraryID: int32(libraryID),
		Title:     normalizedTitle,
		SortTitle: sql.NullString{String: sortTitle, Valid: true},
		Directory: sql.NullString{String: directory, Valid: directory != ""},
	}
}

// buildSQLiteUpsertTVShowParams builds UpsertTVShowParams for SQLite
func buildSQLiteUpsertTVShowParams(libraryID int64, title, directory string) sqlc_sqlite.UpsertTVShowParams {
	// Normalize title to handle punctuation variants (e.g., "Star Trek: Voyager" vs "Star Trek Voyager")
	normalizedTitle := domainCommon.NormalizeTitle(title)
	sortTitle := domainCommon.NormalizeSortTitle(normalizedTitle)
	return sqlc_sqlite.UpsertTVShowParams{
		LibraryID: libraryID,
		Title:     normalizedTitle,
		SortTitle: sql.NullString{String: sortTitle, Valid: true},
		Directory: sql.NullString{String: directory, Valid: directory != ""},
	}
}

// buildPostgresUpdateTVShowParams builds UpdateTVShowParams for Postgres from domain TVShow
func buildPostgresUpdateTVShowParams(show media.TVShow) sqlc_postgres.UpdateTVShowParams {
	// Join genres back to comma-separated string
	genreStr := strings.Join(show.Genre, ", ")

	// Helper to convert string to sql.NullString
	toNullString := func(s string) sql.NullString {
		return sql.NullString{String: s, Valid: s != ""}
	}

	// Helper to convert int to sql.NullInt32
	toNullInt32 := func(i int) sql.NullInt32 {
		return sql.NullInt32{Int32: int32(i), Valid: i != 0}
	}

	return sqlc_postgres.UpdateTVShowParams{
		ID:               int32(show.ID),
		Title:            show.Title,
		OriginalTitle:    toNullString(show.OriginalTitle),
		SortTitle:        toNullString(show.SortTitle),
		Year:             toNullInt32(show.Year),
		FirstAirDate:     common.ParseDateString(show.FirstAirDate),
		LastAirDate:      common.ParseDateString(show.LastAirDate),
		Genre:            toNullString(genreStr),
		Plot:             toNullString(show.Plot),
		Status:           toNullString(show.Status),
		ContentRating:    toNullString(show.ContentRating),
		MaturityRating:   sql.NullInt32{Valid: false}, // Not exposed via enrichment proto
		Network:          toNullString(show.Network),
		OriginalLanguage: toNullString(show.OriginalLanguage),
		CountryOfOrigin:  toNullString(show.CountryOfOrigin),
		ImdbID:           toNullString(show.IMDbID),
		TmdbID:           toNullInt32(show.TMDbID),
		TvdbID:           toNullInt32(show.TVDbID),
		Directory:        toNullString(show.Directory),
		Rating:           common.NullFloat64FromFloat32(show.Rating),
		RatingVotes:      toNullInt32(show.RatingVotes),
		Tagline:          toNullString(show.Tagline),
	}
}

// buildSQLiteUpdateTVShowParams builds UpdateTVShowParams for SQLite from domain TVShow
func buildSQLiteUpdateTVShowParams(show media.TVShow) sqlc_sqlite.UpdateTVShowParams {
	// Join genres back to comma-separated string
	genreStr := strings.Join(show.Genre, ", ")

	// Helper to convert string to sql.NullString
	toNullString := func(s string) sql.NullString {
		return sql.NullString{String: s, Valid: s != ""}
	}

	// Helper to convert int to sql.NullInt64
	toNullInt64 := func(i int) sql.NullInt64 {
		return sql.NullInt64{Int64: int64(i), Valid: i != 0}
	}

	return sqlc_sqlite.UpdateTVShowParams{
		ID:               show.ID,
		Title:            show.Title,
		OriginalTitle:    toNullString(show.OriginalTitle),
		SortTitle:        toNullString(show.SortTitle),
		Year:             toNullInt64(show.Year),
		FirstAirDate:     common.ParseDateString(show.FirstAirDate),
		LastAirDate:      common.ParseDateString(show.LastAirDate),
		Genre:            toNullString(genreStr),
		Plot:             toNullString(show.Plot),
		Status:           toNullString(show.Status),
		ContentRating:    toNullString(show.ContentRating),
		MaturityRating:   sql.NullInt64{Valid: false}, // Not exposed via enrichment proto
		Network:          toNullString(show.Network),
		OriginalLanguage: toNullString(show.OriginalLanguage),
		CountryOfOrigin:  toNullString(show.CountryOfOrigin),
		ImdbID:           toNullString(show.IMDbID),
		TmdbID:           toNullInt64(show.TMDbID),
		TvdbID:           toNullInt64(show.TVDbID),
		Directory:        toNullString(show.Directory),
		Rating:           common.NullFloat64FromFloat32(show.Rating),
		RatingVotes:      toNullInt64(show.RatingVotes),
		Tagline:          toNullString(show.Tagline),
	}
}

// parseGenres converts a comma-separated genre string to a slice
func parseGenres(genreStr string) []string {
	if genreStr == "" {
		return []string{}
	}

	var genres []string
	parts := strings.Split(genreStr, ",")
	for _, part := range parts {
		genre := strings.TrimSpace(part)
		if genre != "" {
			genres = append(genres, genre)
		}
	}

	return genres
}

// postgresShowWithCountsToDomain converts PostgreSQL rows with counts to domain TVShowWithCounts
func postgresShowWithCountsToDomain(row sqlc_postgres.GetTVShowsWithCountsByLibraryPaginatedRow) media.TVShowWithCounts {
	return media.TVShowWithCounts{
		TVShow: media.TVShow{
			ID:            int64(row.ID),
			LibraryID:     int64(row.LibraryID),
			Title:         row.Title,
			Year:          int(common.ParseNullInt32(row.Year)),
			Genre:         parseGenres(common.ParseNullString(row.Genre)),
			Plot:          common.ParseNullString(row.Plot),
			IMDbID:        common.ParseNullString(row.ImdbID),
			TMDbID:        int(common.ParseNullInt32(row.TmdbID)),
			ContentRating: common.ParseNullString(row.ContentRating),
		},
		SeasonCount:  row.SeasonCount,
		EpisodeCount: row.EpisodeCount,
	}
}

func postgresShowWithCountsDescToDomain(row sqlc_postgres.GetTVShowsWithCountsByLibraryPaginatedDescRow) media.TVShowWithCounts {
	return media.TVShowWithCounts{
		TVShow: media.TVShow{
			ID:            int64(row.ID),
			LibraryID:     int64(row.LibraryID),
			Title:         row.Title,
			Year:          int(common.ParseNullInt32(row.Year)),
			Genre:         parseGenres(common.ParseNullString(row.Genre)),
			Plot:          common.ParseNullString(row.Plot),
			IMDbID:        common.ParseNullString(row.ImdbID),
			TMDbID:        int(common.ParseNullInt32(row.TmdbID)),
			ContentRating: common.ParseNullString(row.ContentRating),
		},
		SeasonCount:  row.SeasonCount,
		EpisodeCount: row.EpisodeCount,
	}
}

func sqliteShowWithCountsToDomain(row sqlc_sqlite.GetTVShowsWithCountsByLibraryPaginatedRow) media.TVShowWithCounts {
	return media.TVShowWithCounts{
		TVShow: media.TVShow{
			ID:            row.ID,
			LibraryID:     row.LibraryID,
			Title:         row.Title,
			Year:          int(common.ParseNullInt64(row.Year)),
			Genre:         parseGenres(common.ParseNullString(row.Genre)),
			Plot:          common.ParseNullString(row.Plot),
			IMDbID:        common.ParseNullString(row.ImdbID),
			TMDbID:        int(common.ParseNullInt64(row.TmdbID)),
			ContentRating: common.ParseNullString(row.ContentRating),
		},
		SeasonCount:  row.SeasonCount,
		EpisodeCount: row.EpisodeCount,
	}
}

func sqliteShowWithCountsDescToDomain(row sqlc_sqlite.GetTVShowsWithCountsByLibraryPaginatedDescRow) media.TVShowWithCounts {
	return media.TVShowWithCounts{
		TVShow: media.TVShow{
			ID:            row.ID,
			LibraryID:     row.LibraryID,
			Title:         row.Title,
			Year:          int(common.ParseNullInt64(row.Year)),
			Genre:         parseGenres(common.ParseNullString(row.Genre)),
			Plot:          common.ParseNullString(row.Plot),
			IMDbID:        common.ParseNullString(row.ImdbID),
			TMDbID:        int(common.ParseNullInt64(row.TmdbID)),
			ContentRating: common.ParseNullString(row.ContentRating),
		},
		SeasonCount:  row.SeasonCount,
		EpisodeCount: row.EpisodeCount,
	}
}

// Search with counts converters
func postgresSearchShowWithCountsToDomain(row sqlc_postgres.SearchTVShowsWithCountsByTitlePaginatedRow) media.TVShowWithCounts {
	return media.TVShowWithCounts{
		TVShow: media.TVShow{
			ID:            int64(row.ID),
			LibraryID:     int64(row.LibraryID),
			Title:         row.Title,
			Year:          int(common.ParseNullInt32(row.Year)),
			Genre:         parseGenres(common.ParseNullString(row.Genre)),
			Plot:          common.ParseNullString(row.Plot),
			IMDbID:        common.ParseNullString(row.ImdbID),
			TMDbID:        int(common.ParseNullInt32(row.TmdbID)),
			ContentRating: common.ParseNullString(row.ContentRating),
		},
		SeasonCount:  row.SeasonCount,
		EpisodeCount: row.EpisodeCount,
	}
}

func sqliteSearchShowWithCountsToDomain(row sqlc_sqlite.SearchTVShowsWithCountsByTitlePaginatedRow) media.TVShowWithCounts {
	return media.TVShowWithCounts{
		TVShow: media.TVShow{
			ID:            row.ID,
			LibraryID:     row.LibraryID,
			Title:         row.Title,
			Year:          int(common.ParseNullInt64(row.Year)),
			Genre:         parseGenres(common.ParseNullString(row.Genre)),
			Plot:          common.ParseNullString(row.Plot),
			IMDbID:        common.ParseNullString(row.ImdbID),
			TMDbID:        int(common.ParseNullInt64(row.TmdbID)),
			ContentRating: common.ParseNullString(row.ContentRating),
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
	return mapSeasonToDomain(row)
}

// sqliteSeasonToDomain converts a SQLite TvSeason row to domain TVSeason
func sqliteSeasonToDomain(row sqlc_sqlite.TvSeason) media.TVSeason {
	return mapSeasonToDomain(row)
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
