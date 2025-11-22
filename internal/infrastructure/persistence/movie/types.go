package movie

import (
	"database/sql"
	"strings"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// sqliteMovieToDomain converts a GetMovieByMediaIDRow to domain Movie
func sqliteMovieToDomain(row sqlc_sqlite.GetMovieByMediaIDRow) *media.Movie {
	return toMovieDomain(
		row.MediaID_2, row.LibraryID, row.Title, row.Type, row.FilePath,
		row.FileSize, row.ContainerFormat, row.Duration, row.Width, row.Height,
		row.Codec, row.AudioCodec, row.BitRate, row.FrameRate, row.IsExtra,
		row.CreatedAt, row.UpdatedAt,
		row.Year, row.ReleaseDate, row.Genre, row.Director, row.Cast,
		row.ContentRating, row.MaturityRating, row.ContentAdvisories,
		row.Plot, row.Tagline, row.OriginalTitle, row.SortTitle,
		row.ImdbID, row.TmdbID, row.RuntimeMinutes, row.Budget,
		row.Revenue, row.OriginalLanguage, row.CountryOfOrigin, row.AwardsSummary,
	)
}

// sqliteListMovieToDomain converts a ListMoviesByLibraryRow to domain Movie
func sqliteListMovieToDomain(row sqlc_sqlite.ListMoviesByLibraryRow) *media.Movie {
	return toMovieDomain(
		row.MediaID_2, row.LibraryID, row.Title, row.Type, row.FilePath,
		row.FileSize, row.ContainerFormat, row.Duration, row.Width, row.Height,
		row.Codec, row.AudioCodec, row.BitRate, row.FrameRate, row.IsExtra,
		row.CreatedAt, row.UpdatedAt,
		row.Year, row.ReleaseDate, row.Genre, row.Director, row.Cast,
		row.ContentRating, row.MaturityRating, row.ContentAdvisories,
		row.Plot, row.Tagline, row.OriginalTitle, row.SortTitle,
		row.ImdbID, row.TmdbID, row.RuntimeMinutes, row.Budget,
		row.Revenue, row.OriginalLanguage, row.CountryOfOrigin, row.AwardsSummary,
	)
}

// sqliteSearchMovieToDomain converts a SearchMoviesByTitleRow to domain Movie
func sqliteSearchMovieToDomain(row sqlc_sqlite.SearchMoviesByTitleRow) *media.Movie {
	return toMovieDomain(
		row.MediaID_2, row.LibraryID, row.Title, row.Type, row.FilePath,
		row.FileSize, row.ContainerFormat, row.Duration, row.Width, row.Height,
		row.Codec, row.AudioCodec, row.BitRate, row.FrameRate, row.IsExtra,
		row.CreatedAt, row.UpdatedAt,
		row.Year, row.ReleaseDate, row.Genre, row.Director, row.Cast,
		row.ContentRating, row.MaturityRating, row.ContentAdvisories,
		row.Plot, row.Tagline, row.OriginalTitle, row.SortTitle,
		row.ImdbID, row.TmdbID, row.RuntimeMinutes, row.Budget,
		row.Revenue, row.OriginalLanguage, row.CountryOfOrigin, row.AwardsSummary,
	)
}

// sqliteGenreMovieToDomain converts a ListMoviesByGenreRow to domain Movie
func sqliteGenreMovieToDomain(row sqlc_sqlite.ListMoviesByGenreRow) *media.Movie {
	return toMovieDomain(
		row.MediaID_2, row.LibraryID, row.Title, row.Type, row.FilePath,
		row.FileSize, row.ContainerFormat, row.Duration, row.Width, row.Height,
		row.Codec, row.AudioCodec, row.BitRate, row.FrameRate, row.IsExtra,
		row.CreatedAt, row.UpdatedAt,
		row.Year, row.ReleaseDate, row.Genre, row.Director, row.Cast,
		row.ContentRating, row.MaturityRating, row.ContentAdvisories,
		row.Plot, row.Tagline, row.OriginalTitle, row.SortTitle,
		row.ImdbID, row.TmdbID, row.RuntimeMinutes, row.Budget,
		row.Revenue, row.OriginalLanguage, row.CountryOfOrigin, row.AwardsSummary,
	)
}

// sqliteYearMovieToDomain converts a ListMoviesByYearRow to domain Movie
func sqliteYearMovieToDomain(row sqlc_sqlite.ListMoviesByYearRow) *media.Movie {
	return toMovieDomain(
		row.MediaID_2, row.LibraryID, row.Title, row.Type, row.FilePath,
		row.FileSize, row.ContainerFormat, row.Duration, row.Width, row.Height,
		row.Codec, row.AudioCodec, row.BitRate, row.FrameRate, row.IsExtra,
		row.CreatedAt, row.UpdatedAt,
		row.Year, row.ReleaseDate, row.Genre, row.Director, row.Cast,
		row.ContentRating, row.MaturityRating, row.ContentAdvisories,
		row.Plot, row.Tagline, row.OriginalTitle, row.SortTitle,
		row.ImdbID, row.TmdbID, row.RuntimeMinutes, row.Budget,
		row.Revenue, row.OriginalLanguage, row.CountryOfOrigin, row.AwardsSummary,
	)
}

// toMovieDomain is the single source of truth for converting database fields to domain Movie.
// All row-specific converters delegate to this function.
// NOTE: This function has 35 parameters - this is unavoidable due to sqlc generating separate
// types for each query. The thin wrapper functions above keep repository code clean.
func toMovieDomain(
	// Media fields (17 parameters)
	mediaID, libraryID int64, title, mediaType, filePath string,
	fileSize sql.NullInt64, containerFormat sql.NullString, duration sql.NullFloat64,
	width, height sql.NullInt64, codec, audioCodec sql.NullString,
	bitRate sql.NullInt64, frameRate sql.NullFloat64, isExtra bool,
	createdAt, updatedAt sql.NullTime,
	// Movie-specific fields (18 parameters)
	year sql.NullInt64, releaseDate sql.NullTime,
	genre, director, cast, contentRating sql.NullString,
	maturityRating sql.NullInt64, contentAdvisories sql.NullString,
	plot, tagline, originalTitle, sortTitle, imdbID sql.NullString,
	tmdbID, runtimeMinutes, budget, revenue sql.NullInt64,
	originalLanguage, countryOfOrigin, awardsSummary sql.NullString,
) *media.Movie {
	return &media.Movie{
		Media: media.Media{
			ID:              mediaID,
			LibraryID:       libraryID,
			Title:           title,
			Type:            mediaType,
			FilePath:        filePath,
			FileSize:        common.ParseNullInt64(fileSize),
			Duration:        int(common.ParseNullFloat64(duration)),
			IsExtra:         isExtra,
			Width:           int(common.ParseNullInt64(width)),
			Height:          int(common.ParseNullInt64(height)),
			VideoCodec:      common.ParseNullString(codec),
			AudioCodec:      common.ParseNullString(audioCodec),
			Bitrate:         common.ParseNullInt64(bitRate),
			FrameRate:       common.ParseNullFloat64(frameRate),
			ContainerFormat: common.ParseNullString(containerFormat),
			CreatedAt:       common.ParseNullTime(createdAt),
			UpdatedAt:       common.ParseNullTime(updatedAt),
		},
		Year:              int(common.ParseNullInt64(year)),
		ReleaseDate:       common.ParseNullTime(releaseDate),
		OriginalTitle:     common.ParseNullString(originalTitle),
		SortTitle:         common.ParseNullString(sortTitle),
		RuntimeMinutes:    int(common.ParseNullInt64(runtimeMinutes)),
		IMDbID:            common.ParseNullString(imdbID),
		TMDbID:            int(common.ParseNullInt64(tmdbID)),
		Director:          common.ParseNullString(director),
		Cast:              parseCast(common.ParseNullString(cast)),
		Genre:             parseGenres(common.ParseNullString(genre)),
		Plot:              common.ParseNullString(plot),
		Tagline:           common.ParseNullString(tagline),
		ContentRating:     common.ParseNullString(contentRating),
		MaturityRating:    int(common.ParseNullInt64(maturityRating)),
		ContentAdvisories: parseAdvisories(common.ParseNullString(contentAdvisories)),
		Budget:            common.ParseNullInt64(budget),
		Revenue:           common.ParseNullInt64(revenue),
		OriginalLanguage:  common.ParseNullString(originalLanguage),
		CountryOfOrigin:   common.ParseNullString(countryOfOrigin),
		AwardsSummary:     common.ParseNullString(awardsSummary),
	}
}

// buildSQLiteCreateMovieParams builds CreateMovieParams for SQLite from a domain Movie entity
func buildSQLiteCreateMovieParams(m *media.Movie) sqlc_sqlite.CreateMovieParams {
	return sqlc_sqlite.CreateMovieParams{
		MediaID:           m.Media.ID,
		Year:              common.NullInt64(int64(m.Year)),
		ReleaseDate:       common.NullTime(m.ReleaseDate),
		Genre:             common.NullString(serializeGenres(m.Genre)),
		Director:          common.NullString(m.Director),
		Cast:              common.NullString(serializeCast(m.Cast)),
		ContentRating:     common.NullString(m.ContentRating),
		MaturityRating:    common.NullInt64(int64(m.MaturityRating)),
		ContentAdvisories: common.NullString(serializeAdvisories(m.ContentAdvisories)),
		Plot:              common.NullString(m.Plot),
		Tagline:           common.NullString(m.Tagline),
		OriginalTitle:     common.NullString(m.OriginalTitle),
		SortTitle:         common.NullString(m.SortTitle),
		ImdbID:            common.NullString(m.IMDbID),
		TmdbID:            common.NullInt64(int64(m.TMDbID)),
		RuntimeMinutes:    common.NullInt64(int64(m.RuntimeMinutes)),
		Budget:            common.NullInt64(m.Budget),
		Revenue:           common.NullInt64(m.Revenue),
		OriginalLanguage:  common.NullString(m.OriginalLanguage),
		CountryOfOrigin:   common.NullString(m.CountryOfOrigin),
		AwardsSummary:     common.NullString(m.AwardsSummary),
	}
}

// buildSQLiteUpdateMovieParams builds UpdateMovieParams for SQLite from a domain Movie entity
func buildSQLiteUpdateMovieParams(m *media.Movie) sqlc_sqlite.UpdateMovieParams {
	params := buildSQLiteCreateMovieParams(m)
	return sqlc_sqlite.UpdateMovieParams{
		Year:              params.Year,
		ReleaseDate:       params.ReleaseDate,
		Genre:             params.Genre,
		Director:          params.Director,
		Cast:              params.Cast,
		ContentRating:     params.ContentRating,
		MaturityRating:    params.MaturityRating,
		ContentAdvisories: params.ContentAdvisories,
		Plot:              params.Plot,
		Tagline:           params.Tagline,
		OriginalTitle:     params.OriginalTitle,
		SortTitle:         params.SortTitle,
		ImdbID:            params.ImdbID,
		TmdbID:            params.TmdbID,
		RuntimeMinutes:    params.RuntimeMinutes,
		Budget:            params.Budget,
		Revenue:           params.Revenue,
		OriginalLanguage:  params.OriginalLanguage,
		CountryOfOrigin:   params.CountryOfOrigin,
		AwardsSummary:     params.AwardsSummary,
		MediaID:           m.Media.ID,
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

// serializeGenres converts a genre slice to a comma-separated string
func serializeGenres(genres []string) string {
	if len(genres) == 0 {
		return ""
	}
	return strings.Join(genres, ", ")
}

// parseCast converts a comma-separated cast string to a slice
func parseCast(castStr string) []string {
	if castStr == "" {
		return []string{}
	}

	var cast []string
	parts := strings.Split(castStr, ",")
	for _, part := range parts {
		member := strings.TrimSpace(part)
		if member != "" {
			cast = append(cast, member)
		}
	}
	return cast
}

// serializeCast converts a cast slice to a comma-separated string
func serializeCast(cast []string) string {
	if len(cast) == 0 {
		return ""
	}
	return strings.Join(cast, ", ")
}

// parseAdvisories converts a comma-separated advisories string to a slice
func parseAdvisories(advisoriesStr string) []string {
	if advisoriesStr == "" {
		return []string{}
	}

	var advisories []string
	parts := strings.Split(advisoriesStr, ",")
	for _, part := range parts {
		advisory := strings.TrimSpace(part)
		if advisory != "" {
			advisories = append(advisories, advisory)
		}
	}
	return advisories
}

// serializeAdvisories converts an advisories slice to a comma-separated string
func serializeAdvisories(advisories []string) string {
	if len(advisories) == 0 {
		return ""
	}
	return strings.Join(advisories, ", ")
}
