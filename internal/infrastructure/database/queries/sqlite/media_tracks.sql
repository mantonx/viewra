-- Audio and subtitle track queries for multi-language support

-- name: InsertAudioTrack :one
INSERT INTO media_audio_tracks (
    media_id, stream_index, codec, codec_profile, channels, channel_layout,
    sample_rate, bit_rate, language, title, is_default, is_commentary, is_descriptive
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(media_id, stream_index) DO UPDATE SET
    codec = excluded.codec,
    codec_profile = excluded.codec_profile,
    channels = excluded.channels,
    channel_layout = excluded.channel_layout,
    sample_rate = excluded.sample_rate,
    bit_rate = excluded.bit_rate,
    language = excluded.language,
    title = excluded.title,
    is_default = excluded.is_default,
    is_commentary = excluded.is_commentary,
    is_descriptive = excluded.is_descriptive
RETURNING id, created_at;

-- name: GetAudioTracksByMediaID :many
SELECT id, media_id, stream_index, codec, codec_profile, channels, channel_layout,
       sample_rate, bit_rate, language, title, is_default, is_commentary, is_descriptive, created_at
FROM media_audio_tracks
WHERE media_id = ?
ORDER BY stream_index;

-- name: DeleteAudioTracksByMediaID :exec
DELETE FROM media_audio_tracks WHERE media_id = ?;

-- name: InsertSubtitleTrack :one
INSERT INTO media_subtitle_tracks (
    media_id, stream_index, source_type, codec, language, title, file_path,
    is_default, is_forced, is_sdh, is_commentary, is_bitmap
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, created_at;

-- name: GetSubtitleTracksByMediaID :many
SELECT id, media_id, stream_index, source_type, codec, language, title, file_path,
       is_default, is_forced, is_sdh, is_commentary, is_bitmap, created_at
FROM media_subtitle_tracks
WHERE media_id = ?
ORDER BY source_type DESC, stream_index;

-- name: GetEmbeddedSubtitlesByMediaID :many
SELECT id, media_id, stream_index, source_type, codec, language, title, file_path,
       is_default, is_forced, is_sdh, is_commentary, is_bitmap, created_at
FROM media_subtitle_tracks
WHERE media_id = ? AND source_type = 'embedded'
ORDER BY stream_index;

-- name: GetExternalSubtitlesByMediaID :many
SELECT id, media_id, stream_index, source_type, codec, language, title, file_path,
       is_default, is_forced, is_sdh, is_commentary, is_bitmap, created_at
FROM media_subtitle_tracks
WHERE media_id = ? AND source_type = 'external'
ORDER BY file_path;

-- name: DeleteSubtitleTracksByMediaID :exec
DELETE FROM media_subtitle_tracks WHERE media_id = ?;

-- name: DeleteExternalSubtitlesByMediaID :exec
DELETE FROM media_subtitle_tracks WHERE media_id = ? AND source_type = 'external';
