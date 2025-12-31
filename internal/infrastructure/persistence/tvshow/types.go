package tvshow

import (
	"database/sql"
	"strings"

	domainCommon "github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// ========================================
// TV Episode Converters
// ========================================

// episodeRowToDomain converts a GetTVEpisodeByMediaIDRow to domain TVEpisode
func episodeRowToDomain(row unified.GetTVEpisodeByMediaIDRow) *media.TVEpisode {
	return &media.TVEpisode{
		Media: media.Media{
			ID:              row.MediaID_2,
			LibraryID:       row.LibraryID,
			Title:           row.Title,
			Type:            row.Type,
			FilePath:        row.FilePath,
			FileSize:        common.ParseNullInt64(row.FileSize),
			Duration:        int(common.ParseNullFloat64(row.Duration)),
			IsExtra:         row.IsExtra != 0,
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
		ShowID:       row.ShowID,
		ShowTitle:    "", // Will be populated from show lookup if needed
		SeasonID:     row.SeasonID,
		Season:       int(row.SeasonNumber),
		Episode:      int(row.EpisodeNumber),
		EpisodeTitle: common.ParseNullString(row.EpisodeTitle),
		TVDbID:       int(common.ParseNullInt64(row.TvdbID)),
		TMDbID:       common.ParseNullInt64(row.TmdbID),
		IMDbID:       common.ParseNullString(row.ImdbID),
		AirDate:      common.FormatNullDate(row.AirDate),
		Description:  common.ParseNullString(row.Plot),

		// Alternative ordering
		AbsoluteNumber: int(common.ParseNullInt64(row.AbsoluteNumber)),
		DvdSeason:      int(common.ParseNullInt64(row.DvdSeason)),
		DvdEpisode:     int(common.ParseNullInt64(row.DvdEpisode)),

		// Additional metadata
		OriginalTitle:  common.ParseNullString(row.OriginalTitle),
		ContentRating:  common.ParseNullString(row.ContentRating),
		MaturityRating: int(common.ParseNullInt64(row.MaturityRating)),

		RuntimeMinutes: int(common.ParseNullInt64(row.RuntimeMinutes)),
		Rating:         float32(common.ParseNullFloat64(row.Rating)),
		RatingVotes:    int(common.ParseNullInt64(row.RatingVotes)),
	}
}

// listEpisodeRowToDomain converts a ListTVEpisodesByLibraryRow to domain TVEpisode
func listEpisodeRowToDomain(row unified.ListTVEpisodesByLibraryRow) *media.TVEpisode {
	return &media.TVEpisode{
		Media: media.Media{
			ID:              row.MediaID_2,
			LibraryID:       row.LibraryID,
			Title:           row.Title,
			Type:            row.Type,
			FilePath:        row.FilePath,
			FileSize:        common.ParseNullInt64(row.FileSize),
			Duration:        int(common.ParseNullFloat64(row.Duration)),
			IsExtra:         row.IsExtra != 0,
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
		ShowID:       row.ShowID,
		ShowTitle:    "", // Will be populated from show lookup if needed
		SeasonID:     row.SeasonID,
		Season:       int(row.SeasonNumber),
		Episode:      int(row.EpisodeNumber),
		EpisodeTitle: common.ParseNullString(row.EpisodeTitle),
		TVDbID:       int(common.ParseNullInt64(row.TvdbID)),
		TMDbID:       common.ParseNullInt64(row.TmdbID),
		IMDbID:       common.ParseNullString(row.ImdbID),
		AirDate:      common.FormatNullDate(row.AirDate),
		Description:  common.ParseNullString(row.Plot),

		// Alternative ordering
		AbsoluteNumber: int(common.ParseNullInt64(row.AbsoluteNumber)),
		DvdSeason:      int(common.ParseNullInt64(row.DvdSeason)),
		DvdEpisode:     int(common.ParseNullInt64(row.DvdEpisode)),

		// Additional metadata
		OriginalTitle:  common.ParseNullString(row.OriginalTitle),
		ContentRating:  common.ParseNullString(row.ContentRating),
		MaturityRating: int(common.ParseNullInt64(row.MaturityRating)),

		RuntimeMinutes: int(common.ParseNullInt64(row.RuntimeMinutes)),
		Rating:         float32(common.ParseNullFloat64(row.Rating)),
		RatingVotes:    int(common.ParseNullInt64(row.RatingVotes)),
	}
}

// searchEpisodeRowToDomain converts a SearchTVEpisodesByTitleRow to domain TVEpisode
func searchEpisodeRowToDomain(row unified.SearchTVEpisodesByTitleRow) *media.TVEpisode {
	return &media.TVEpisode{
		Media: media.Media{
			ID:              row.MediaID_2,
			LibraryID:       row.LibraryID,
			Title:           row.Title,
			Type:            row.Type,
			FilePath:        row.FilePath,
			FileSize:        common.ParseNullInt64(row.FileSize),
			Duration:        int(common.ParseNullFloat64(row.Duration)),
			IsExtra:         row.IsExtra != 0,
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
		ShowID:       row.ShowID,
		ShowTitle:    "", // Will be populated from show lookup if needed
		SeasonID:     row.SeasonID,
		Season:       int(row.SeasonNumber),
		Episode:      int(row.EpisodeNumber),
		EpisodeTitle: common.ParseNullString(row.EpisodeTitle),
		TVDbID:       int(common.ParseNullInt64(row.TvdbID)),
		TMDbID:       common.ParseNullInt64(row.TmdbID),
		IMDbID:       common.ParseNullString(row.ImdbID),
		AirDate:      common.FormatNullDate(row.AirDate),
		Description:  common.ParseNullString(row.Plot),

		// Alternative ordering
		AbsoluteNumber: int(common.ParseNullInt64(row.AbsoluteNumber)),
		DvdSeason:      int(common.ParseNullInt64(row.DvdSeason)),
		DvdEpisode:     int(common.ParseNullInt64(row.DvdEpisode)),

		// Additional metadata
		OriginalTitle:  common.ParseNullString(row.OriginalTitle),
		ContentRating:  common.ParseNullString(row.ContentRating),
		MaturityRating: int(common.ParseNullInt64(row.MaturityRating)),

		RuntimeMinutes: int(common.ParseNullInt64(row.RuntimeMinutes)),
		Rating:         float32(common.ParseNullFloat64(row.Rating)),
		RatingVotes:    int(common.ParseNullInt64(row.RatingVotes)),
	}
}

// buildCreateEpisodeParams builds CreateTVEpisodeParams from a domain TVEpisode entity
func buildCreateEpisodeParams(e *media.TVEpisode, showID, seasonID int64) unified.CreateTVEpisodeParams {
	var airDate sql.NullTime
	if e.AirDate != "" {
		// Parse the air date if it's provided
		// For simplicity, we'll just store it as NullTime
		airDate = sql.NullTime{Valid: false}
	}

	return unified.CreateTVEpisodeParams{
		MediaID:        e.Media.ID,
		ShowID:         showID,
		SeasonID:       seasonID,
		SeasonNumber:   int64(e.Season),
		EpisodeNumber:  int64(e.Episode),
		AbsoluteNumber: common.NullInt64(int64(e.AbsoluteNumber)),
		DvdSeason:      common.NullInt64(int64(e.DvdSeason)),
		DvdEpisode:     common.NullInt64(int64(e.DvdEpisode)),
		EpisodeTitle:   common.NullString(e.EpisodeTitle),
		OriginalTitle:  common.NullString(e.OriginalTitle),
		AirDate:        airDate,
		Plot:           common.NullString(e.Description),
		ContentRating:  common.NullString(e.ContentRating),
		MaturityRating: common.NullInt64(int64(e.MaturityRating)),
		ImdbID:         common.NullString(e.IMDbID),
		TmdbID:         common.NullInt64(e.TMDbID),
		TvdbID:         common.NullInt64(int64(e.TVDbID)),
		Rating:         common.NullFloat64FromFloat32(e.Rating),
		RatingVotes:    common.NullInt64(int64(e.RatingVotes)),
		RuntimeMinutes: common.NullInt64(int64(e.RuntimeMinutes)),
	}
}

// buildUpdateEpisodeParams builds UpdateTVEpisodeParams from a domain TVEpisode entity
func buildUpdateEpisodeParams(e *media.TVEpisode, showID, seasonID int64) unified.UpdateTVEpisodeParams {
	params := buildCreateEpisodeParams(e, showID, seasonID)
	return unified.UpdateTVEpisodeParams{
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
// TV Show Converters
// ========================================

// showToDomain converts a TvShow row to domain TVShow
func showToDomain(row unified.TvShow) media.TVShow {
	return media.TVShow{
		ID:               row.ID,
		LibraryID:        row.LibraryID,
		Title:            row.Title,
		OriginalTitle:    common.ParseNullString(row.OriginalTitle),
		SortTitle:        common.ParseNullString(row.SortTitle),
		Year:             int(common.ParseNullInt64(row.Year)),
		FirstAirDate:     common.FormatNullDate(row.FirstAirDate),
		LastAirDate:      common.FormatNullDate(row.LastAirDate),
		Genre:            parseGenres(common.ParseNullString(row.Genre)),
		Plot:             common.ParseNullString(row.Plot),
		Status:           common.ParseNullString(row.Status),
		ContentRating:    common.ParseNullString(row.ContentRating),
		Network:          common.ParseNullString(row.Network),
		OriginalLanguage: common.ParseNullString(row.OriginalLanguage),
		CountryOfOrigin:  common.ParseNullString(row.CountryOfOrigin),
		IMDbID:           common.ParseNullString(row.ImdbID),
		TMDbID:           int(common.ParseNullInt64(row.TmdbID)),
		TVDbID:           int(common.ParseNullInt64(row.TvdbID)),
		Directory:        common.ParseNullString(row.Directory),
		Tagline:          common.ParseNullString(row.Tagline),
		Rating:           float32(common.ParseNullFloat64(row.Rating)),
		RatingVotes:      int(common.ParseNullInt64(row.RatingVotes)),
	}
}

// buildCreateTVShowParams builds CreateTVShowParams
func buildCreateTVShowParams(libraryID int64, title, directory string) unified.CreateTVShowParams {
	sortTitle := domainCommon.NormalizeSortTitle(title)
	return unified.CreateTVShowParams{
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

// buildUpsertTVShowParams builds UpsertTVShowParams
func buildUpsertTVShowParams(libraryID int64, title, directory string) unified.UpsertTVShowParams {
	// Normalize title to handle punctuation variants (e.g., "Star Trek: Voyager" vs "Star Trek Voyager")
	normalizedTitle := domainCommon.NormalizeTitle(title)
	sortTitle := domainCommon.NormalizeSortTitle(normalizedTitle)
	return unified.UpsertTVShowParams{
		LibraryID: libraryID,
		Title:     normalizedTitle,
		SortTitle: sql.NullString{String: sortTitle, Valid: true},
		Directory: sql.NullString{String: directory, Valid: directory != ""},
	}
}

// buildUpdateTVShowParams builds UpdateTVShowParams from domain TVShow
func buildUpdateTVShowParams(show media.TVShow) unified.UpdateTVShowParams {
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

	return unified.UpdateTVShowParams{
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

// showWithCountsToDomain converts GetTVShowsWithCountsByLibraryPaginatedRow to domain TVShowWithCounts
func showWithCountsToDomain(row unified.GetTVShowsWithCountsByLibraryPaginatedRow) media.TVShowWithCounts {
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
			CreatedAt:     common.ParseNullTime(row.CreatedAt),
		},
		SeasonCount:  row.SeasonCount,
		EpisodeCount: row.EpisodeCount,
	}
}

// showWithCountsDescToDomain converts GetTVShowsWithCountsByLibraryPaginatedDescRow to domain TVShowWithCounts
func showWithCountsDescToDomain(row unified.GetTVShowsWithCountsByLibraryPaginatedDescRow) media.TVShowWithCounts {
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
			CreatedAt:     common.ParseNullTime(row.CreatedAt),
		},
		SeasonCount:  row.SeasonCount,
		EpisodeCount: row.EpisodeCount,
	}
}

// searchShowWithCountsToDomain converts SearchTVShowsWithCountsByTitlePaginatedRow to domain TVShowWithCounts
func searchShowWithCountsToDomain(row unified.SearchTVShowsWithCountsByTitlePaginatedRow) media.TVShowWithCounts {
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
			CreatedAt:     common.ParseNullTime(row.CreatedAt),
		},
		SeasonCount:  row.SeasonCount,
		EpisodeCount: row.EpisodeCount,
	}
}

// ========================================
// TV Season Converters
// ========================================

// seasonToDomain converts a TvSeason row to domain TVSeason
func seasonToDomain(row unified.TvSeason) media.TVSeason {
	return media.TVSeason{
		ID:           row.ID,
		ShowID:       row.ShowID,
		SeasonNumber: row.SeasonNumber,
		EpisodeCount: int(common.ParseNullInt64(row.EpisodeCount)),
	}
}

// buildCreateTVSeasonParams builds CreateTVSeasonParams
func buildCreateTVSeasonParams(showID, seasonNumber int64) unified.CreateTVSeasonParams {
	return unified.CreateTVSeasonParams{
		ShowID:       showID,
		SeasonNumber: seasonNumber,
		EpisodeCount: sql.NullInt64{Int64: 0, Valid: true},
	}
}
