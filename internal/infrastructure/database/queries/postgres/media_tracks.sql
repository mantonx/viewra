-- Audio and subtitle track queries for multi-language support

-- name: InsertAudioTrack :one
INSERT INTO media_audio_tracks (
    media_id, stream_index, codec, codec_profile, channels, channel_layout,
    sample_rate, bit_rate, language, title, is_default, is_commentary, is_descriptive
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
ON CONFLICT(media_id, stream_index) DO UPDATE SET
    codec = EXCLUDED.codec,
    codec_profile = EXCLUDED.codec_profile,
    channels = EXCLUDED.channels,
    channel_layout = EXCLUDED.channel_layout,
    sample_rate = EXCLUDED.sample_rate,
    bit_rate = EXCLUDED.bit_rate,
    language = EXCLUDED.language,
    title = EXCLUDED.title,
    is_default = EXCLUDED.is_default,
    is_commentary = EXCLUDED.is_commentary,
    is_descriptive = EXCLUDED.is_descriptive
RETURNING id, created_at;

-- name: GetAudioTracksByMediaID :many
SELECT id, media_id, stream_index, codec, codec_profile, channels, channel_layout,
       sample_rate, bit_rate, language, title, is_default, is_commentary, is_descriptive, created_at
FROM media_audio_tracks
WHERE media_id = $1
ORDER BY stream_index;

-- name: DeleteAudioTracksByMediaID :exec
DELETE FROM media_audio_tracks WHERE media_id = $1;

-- name: InsertSubtitleTrack :one
INSERT INTO media_subtitle_tracks (
    media_id, stream_index, source_type, codec, language, title, file_path,
    is_default, is_forced, is_sdh, is_commentary, is_bitmap
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING id, created_at;

-- name: GetSubtitleTracksByMediaID :many
SELECT id, media_id, stream_index, source_type, codec, language, title, file_path,
       is_default, is_forced, is_sdh, is_commentary, is_bitmap, created_at
FROM media_subtitle_tracks
WHERE media_id = $1
ORDER BY source_type DESC, stream_index;

-- name: GetEmbeddedSubtitlesByMediaID :many
SELECT id, media_id, stream_index, source_type, codec, language, title, file_path,
       is_default, is_forced, is_sdh, is_commentary, is_bitmap, created_at
FROM media_subtitle_tracks
WHERE media_id = $1 AND source_type = 'embedded'
ORDER BY stream_index;

-- name: GetExternalSubtitlesByMediaID :many
SELECT id, media_id, stream_index, source_type, codec, language, title, file_path,
       is_default, is_forced, is_sdh, is_commentary, is_bitmap, created_at
FROM media_subtitle_tracks
WHERE media_id = $1 AND source_type = 'external'
ORDER BY file_path;

-- name: DeleteSubtitleTracksByMediaID :exec
DELETE FROM media_subtitle_tracks WHERE media_id = $1;

-- name: DeleteExternalSubtitlesByMediaID :exec
DELETE FROM media_subtitle_tracks WHERE media_id = $1 AND source_type = 'external';
