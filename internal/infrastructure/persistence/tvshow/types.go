package tvshow

import (
	"database/sql"
	"unsafe"

	"github.com/mantonx/viewra/internal/domain/media"
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
