package media

import (
	"context"
	"database/sql"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// InsertAudioTrack inserts an audio track for a media item.
func (r *Repository) InsertAudioTrack(ctx context.Context, track *media.AudioTrack) error {
	result, err := r.Q().InsertAudioTrack(ctx, buildInsertAudioParams(track))
	if err != nil {
		return err
	}
	track.ID = result.ID
	track.CreatedAt = common.ParseNullTime(result.CreatedAt)
	return nil
}

// GetAudioTracksByMediaID retrieves all audio tracks for a media item.
func (r *Repository) GetAudioTracksByMediaID(ctx context.Context, mediaID int64) ([]*media.AudioTrack, error) {
	rows, err := r.Q().GetAudioTracksByMediaID(ctx, mediaID)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, audioTrackToDomain), nil
}

// DeleteAudioTracksByMediaID deletes all audio tracks for a media item.
func (r *Repository) DeleteAudioTracksByMediaID(ctx context.Context, mediaID int64) error {
	return r.Q().DeleteAudioTracksByMediaID(ctx, mediaID)
}

// InsertSubtitleTrack inserts a subtitle track for a media item.
func (r *Repository) InsertSubtitleTrack(ctx context.Context, track *media.SubtitleTrack) error {
	result, err := r.Q().InsertSubtitleTrack(ctx, buildInsertSubtitleParams(track))
	if err != nil {
		return err
	}
	track.ID = result.ID
	track.CreatedAt = common.ParseNullTime(result.CreatedAt)
	return nil
}

// GetSubtitleTracksByMediaID retrieves all subtitle tracks for a media item.
func (r *Repository) GetSubtitleTracksByMediaID(ctx context.Context, mediaID int64) ([]*media.SubtitleTrack, error) {
	rows, err := r.Q().GetSubtitleTracksByMediaID(ctx, mediaID)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, subtitleTrackToDomain), nil
}

// DeleteSubtitleTracksByMediaID deletes all subtitle tracks for a media item.
func (r *Repository) DeleteSubtitleTracksByMediaID(ctx context.Context, mediaID int64) error {
	return r.Q().DeleteSubtitleTracksByMediaID(ctx, mediaID)
}

// DeleteExternalSubtitlesByMediaID deletes only external subtitle tracks for a media item.
func (r *Repository) DeleteExternalSubtitlesByMediaID(ctx context.Context, mediaID int64) error {
	return r.Q().DeleteExternalSubtitlesByMediaID(ctx, mediaID)
}

// buildInsertAudioParams builds insert params for audio track.
func buildInsertAudioParams(t *media.AudioTrack) unified.InsertAudioTrackParams {
	return unified.InsertAudioTrackParams{
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

// buildInsertSubtitleParams builds insert params for subtitle track.
func buildInsertSubtitleParams(t *media.SubtitleTrack) unified.InsertSubtitleTrackParams {
	params := unified.InsertSubtitleTrackParams{
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

// audioTrackToDomain converts database audio track to domain entity.
func audioTrackToDomain(row unified.MediaAudioTrack) *media.AudioTrack {
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

// subtitleTrackToDomain converts database subtitle track to domain entity.
func subtitleTrackToDomain(row unified.MediaSubtitleTrack) *media.SubtitleTrack {
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
