package media

import (
	"context"
	"database/sql"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// InsertAudioTrack inserts an audio track for a media item.
func (r *Repository) InsertAudioTrack(ctx context.Context, track *media.AudioTrack) error {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.InsertAudioTrack(ctx, buildPostgresInsertAudioParams(track))
		},
		func() (any, error) {
			return r.sqlite.InsertAudioTrack(ctx, buildSQLiteInsertAudioParams(track))
		},
	)
	if err != nil {
		return err
	}

	if r.router.IsPostgresDB() {
		pgResult := result.(sqlc_postgres.InsertAudioTrackRow)
		track.ID = int64(pgResult.ID)
		track.CreatedAt = common.ParseNullTime(pgResult.CreatedAt)
	} else {
		sqResult := result.(sqlc_sqlite.InsertAudioTrackRow)
		track.ID = sqResult.ID
		track.CreatedAt = common.ParseNullTime(sqResult.CreatedAt)
	}

	return nil
}

// GetAudioTracksByMediaID retrieves all audio tracks for a media item.
func (r *Repository) GetAudioTracksByMediaID(ctx context.Context, mediaID int64) ([]*media.AudioTrack, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetAudioTracksByMediaID(ctx, mediaID)
		},
		func() (any, error) {
			return r.sqlite.GetAudioTracksByMediaID(ctx, mediaID)
		},
	)
	if err != nil {
		return nil, err
	}

	if r.router.IsPostgresDB() {
		rows := result.([]sqlc_postgres.MediaAudioTrack)
		tracks := make([]*media.AudioTrack, len(rows))
		for i, row := range rows {
			tracks[i] = pgAudioTrackToDomain(row)
		}
		return tracks, nil
	}

	rows := result.([]sqlc_sqlite.MediaAudioTrack)
	tracks := make([]*media.AudioTrack, len(rows))
	for i, row := range rows {
		tracks[i] = sqliteAudioTrackToDomain(row)
	}
	return tracks, nil
}

// DeleteAudioTracksByMediaID deletes all audio tracks for a media item.
func (r *Repository) DeleteAudioTracksByMediaID(ctx context.Context, mediaID int64) error {
	_, err := r.router.Route(
		func() (any, error) {
			return nil, r.postgres.DeleteAudioTracksByMediaID(ctx, mediaID)
		},
		func() (any, error) {
			return nil, r.sqlite.DeleteAudioTracksByMediaID(ctx, mediaID)
		},
	)
	return err
}

// InsertSubtitleTrack inserts a subtitle track for a media item.
func (r *Repository) InsertSubtitleTrack(ctx context.Context, track *media.SubtitleTrack) error {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.InsertSubtitleTrack(ctx, buildPostgresInsertSubtitleParams(track))
		},
		func() (any, error) {
			return r.sqlite.InsertSubtitleTrack(ctx, buildSQLiteInsertSubtitleParams(track))
		},
	)
	if err != nil {
		return err
	}

	if r.router.IsPostgresDB() {
		pgResult := result.(sqlc_postgres.InsertSubtitleTrackRow)
		track.ID = int64(pgResult.ID)
		track.CreatedAt = common.ParseNullTime(pgResult.CreatedAt)
	} else {
		sqResult := result.(sqlc_sqlite.InsertSubtitleTrackRow)
		track.ID = sqResult.ID
		track.CreatedAt = common.ParseNullTime(sqResult.CreatedAt)
	}

	return nil
}

// GetSubtitleTracksByMediaID retrieves all subtitle tracks for a media item.
func (r *Repository) GetSubtitleTracksByMediaID(ctx context.Context, mediaID int64) ([]*media.SubtitleTrack, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetSubtitleTracksByMediaID(ctx, mediaID)
		},
		func() (any, error) {
			return r.sqlite.GetSubtitleTracksByMediaID(ctx, mediaID)
		},
	)
	if err != nil {
		return nil, err
	}

	if r.router.IsPostgresDB() {
		rows := result.([]sqlc_postgres.MediaSubtitleTrack)
		tracks := make([]*media.SubtitleTrack, len(rows))
		for i, row := range rows {
			tracks[i] = pgSubtitleTrackToDomain(row)
		}
		return tracks, nil
	}

	rows := result.([]sqlc_sqlite.MediaSubtitleTrack)
	tracks := make([]*media.SubtitleTrack, len(rows))
	for i, row := range rows {
		tracks[i] = sqliteSubtitleTrackToDomain(row)
	}
	return tracks, nil
}

// DeleteSubtitleTracksByMediaID deletes all subtitle tracks for a media item.
func (r *Repository) DeleteSubtitleTracksByMediaID(ctx context.Context, mediaID int64) error {
	_, err := r.router.Route(
		func() (any, error) {
			return nil, r.postgres.DeleteSubtitleTracksByMediaID(ctx, mediaID)
		},
		func() (any, error) {
			return nil, r.sqlite.DeleteSubtitleTracksByMediaID(ctx, mediaID)
		},
	)
	return err
}

// DeleteExternalSubtitlesByMediaID deletes only external subtitle tracks for a media item.
func (r *Repository) DeleteExternalSubtitlesByMediaID(ctx context.Context, mediaID int64) error {
	_, err := r.router.Route(
		func() (any, error) {
			return nil, r.postgres.DeleteExternalSubtitlesByMediaID(ctx, mediaID)
		},
		func() (any, error) {
			return nil, r.sqlite.DeleteExternalSubtitlesByMediaID(ctx, mediaID)
		},
	)
	return err
}

// buildSQLiteInsertAudioParams builds insert params for SQLite.
func buildSQLiteInsertAudioParams(t *media.AudioTrack) sqlc_sqlite.InsertAudioTrackParams {
	return sqlc_sqlite.InsertAudioTrackParams{
		MediaID:       t.MediaID,
		StreamIndex:   int64(t.StreamIndex),
		Codec:         t.Codec,
		CodecProfile:  common.NullString(t.CodecProfile),
		Channels:      int64(t.Channels),
		ChannelLayout: common.NullString(t.ChannelLayout),
		SampleRate:    common.NullInt64(int64(t.SampleRate)),
		BitRate:       common.NullInt64(int64(t.BitRate)),
		Language:      common.NullString(t.Language),
		Title:         common.NullString(t.Title),
		IsDefault:     common.NullInt64FromBool(t.IsDefault),
		IsCommentary:  common.NullInt64FromBool(t.IsCommentary),
		IsDescriptive: common.NullInt64FromBool(t.IsDescriptive),
	}
}

// buildPostgresInsertAudioParams builds insert params for PostgreSQL.
func buildPostgresInsertAudioParams(t *media.AudioTrack) sqlc_postgres.InsertAudioTrackParams {
	return sqlc_postgres.InsertAudioTrackParams{
		MediaID:       t.MediaID,
		StreamIndex:   int64(t.StreamIndex),
		Codec:         t.Codec,
		CodecProfile:  common.NullString(t.CodecProfile),
		Channels:      int64(t.Channels),
		ChannelLayout: common.NullString(t.ChannelLayout),
		SampleRate:    common.NullInt64(int64(t.SampleRate)),
		BitRate:       common.NullInt64(int64(t.BitRate)),
		Language:      common.NullString(t.Language),
		Title:         common.NullString(t.Title),
		IsDefault:     common.NullInt64FromBool(t.IsDefault),
		IsCommentary:  common.NullInt64FromBool(t.IsCommentary),
		IsDescriptive: common.NullInt64FromBool(t.IsDescriptive),
	}
}

// buildSQLiteInsertSubtitleParams builds insert params for SQLite.
func buildSQLiteInsertSubtitleParams(t *media.SubtitleTrack) sqlc_sqlite.InsertSubtitleTrackParams {
	params := sqlc_sqlite.InsertSubtitleTrackParams{
		MediaID:      t.MediaID,
		SourceType:   string(t.SourceType),
		Codec:        common.NullString(t.Codec),
		Language:     common.NullString(t.Language),
		Title:        common.NullString(t.Title),
		IsDefault:    common.NullInt64FromBool(t.IsDefault),
		IsForced:     common.NullInt64FromBool(t.IsForced),
		IsSdh:        common.NullInt64FromBool(t.IsSDH),
		IsCommentary: common.NullInt64FromBool(t.IsCommentary),
		IsBitmap:     common.NullInt64FromBool(t.IsBitmap),
	}
	if t.StreamIndex != nil {
		params.StreamIndex = sql.NullInt64{Int64: int64(*t.StreamIndex), Valid: true}
	}
	if t.FilePath != nil {
		params.FilePath = sql.NullString{String: *t.FilePath, Valid: true}
	}
	return params
}

// buildPostgresInsertSubtitleParams builds insert params for PostgreSQL.
func buildPostgresInsertSubtitleParams(t *media.SubtitleTrack) sqlc_postgres.InsertSubtitleTrackParams {
	params := sqlc_postgres.InsertSubtitleTrackParams{
		MediaID:      t.MediaID,
		SourceType:   string(t.SourceType),
		Codec:        common.NullString(t.Codec),
		Language:     common.NullString(t.Language),
		Title:        common.NullString(t.Title),
		IsDefault:    common.NullInt64FromBool(t.IsDefault),
		IsForced:     common.NullInt64FromBool(t.IsForced),
		IsSdh:        common.NullInt64FromBool(t.IsSDH),
		IsCommentary: common.NullInt64FromBool(t.IsCommentary),
		IsBitmap:     common.NullInt64FromBool(t.IsBitmap),
	}
	if t.StreamIndex != nil {
		params.StreamIndex = sql.NullInt64{Int64: int64(*t.StreamIndex), Valid: true}
	}
	if t.FilePath != nil {
		params.FilePath = sql.NullString{String: *t.FilePath, Valid: true}
	}
	return params
}

// sqliteAudioTrackToDomain converts SQLite audio track to domain entity.
func sqliteAudioTrackToDomain(row sqlc_sqlite.MediaAudioTrack) *media.AudioTrack {
	return &media.AudioTrack{
		ID:            row.ID,
		MediaID:       row.MediaID,
		StreamIndex:   int(row.StreamIndex),
		Codec:         row.Codec,
		CodecProfile:  row.CodecProfile.String,
		Channels:      int(row.Channels),
		ChannelLayout: row.ChannelLayout.String,
		SampleRate:    int(row.SampleRate.Int64),
		BitRate:       int(row.BitRate.Int64),
		Language:      row.Language.String,
		Title:         row.Title.String,
		IsDefault:     common.NullInt64ToBool(row.IsDefault),
		IsCommentary:  common.NullInt64ToBool(row.IsCommentary),
		IsDescriptive: common.NullInt64ToBool(row.IsDescriptive),
		CreatedAt:     common.ParseNullTime(row.CreatedAt),
	}
}

// pgAudioTrackToDomain converts PostgreSQL audio track to domain entity.
func pgAudioTrackToDomain(row sqlc_postgres.MediaAudioTrack) *media.AudioTrack {
	return &media.AudioTrack{
		ID:            row.ID,
		MediaID:       row.MediaID,
		StreamIndex:   int(row.StreamIndex),
		Codec:         row.Codec,
		CodecProfile:  row.CodecProfile.String,
		Channels:      int(row.Channels),
		ChannelLayout: row.ChannelLayout.String,
		SampleRate:    int(row.SampleRate.Int64),
		BitRate:       int(row.BitRate.Int64),
		Language:      row.Language.String,
		Title:         row.Title.String,
		IsDefault:     common.NullInt64ToBool(row.IsDefault),
		IsCommentary:  common.NullInt64ToBool(row.IsCommentary),
		IsDescriptive: common.NullInt64ToBool(row.IsDescriptive),
		CreatedAt:     common.ParseNullTime(row.CreatedAt),
	}
}

// sqliteSubtitleTrackToDomain converts SQLite subtitle track to domain entity.
func sqliteSubtitleTrackToDomain(row sqlc_sqlite.MediaSubtitleTrack) *media.SubtitleTrack {
	track := &media.SubtitleTrack{
		ID:           row.ID,
		MediaID:      row.MediaID,
		SourceType:   media.SubtitleSourceType(row.SourceType),
		Codec:        row.Codec.String,
		Language:     row.Language.String,
		Title:        row.Title.String,
		IsDefault:    common.NullInt64ToBool(row.IsDefault),
		IsForced:     common.NullInt64ToBool(row.IsForced),
		IsSDH:        common.NullInt64ToBool(row.IsSdh),
		IsCommentary: common.NullInt64ToBool(row.IsCommentary),
		IsBitmap:     common.NullInt64ToBool(row.IsBitmap),
		CreatedAt:    common.ParseNullTime(row.CreatedAt),
	}
	if row.StreamIndex.Valid {
		idx := int(row.StreamIndex.Int64)
		track.StreamIndex = &idx
	}
	if row.FilePath.Valid {
		track.FilePath = &row.FilePath.String
	}
	return track
}

// pgSubtitleTrackToDomain converts PostgreSQL subtitle track to domain entity.
func pgSubtitleTrackToDomain(row sqlc_postgres.MediaSubtitleTrack) *media.SubtitleTrack {
	track := &media.SubtitleTrack{
		ID:           row.ID,
		MediaID:      row.MediaID,
		SourceType:   media.SubtitleSourceType(row.SourceType),
		Codec:        row.Codec.String,
		Language:     row.Language.String,
		Title:        row.Title.String,
		IsDefault:    common.NullInt64ToBool(row.IsDefault),
		IsForced:     common.NullInt64ToBool(row.IsForced),
		IsSDH:        common.NullInt64ToBool(row.IsSdh),
		IsCommentary: common.NullInt64ToBool(row.IsCommentary),
		IsBitmap:     common.NullInt64ToBool(row.IsBitmap),
		CreatedAt:    common.ParseNullTime(row.CreatedAt),
	}
	if row.StreamIndex.Valid {
		idx := int(row.StreamIndex.Int64)
		track.StreamIndex = &idx
	}
	if row.FilePath.Valid {
		track.FilePath = &row.FilePath.String
	}
	return track
}
