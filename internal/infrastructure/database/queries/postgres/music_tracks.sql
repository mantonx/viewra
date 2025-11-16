-- name: CreateMusicTrack :exec
INSERT INTO music_tracks (
    media_id, artist, album, album_artist, track_number, disc_number,
    total_tracks, total_discs, genre, year, release_date, composer,
    lyricist, record_label, isrc, release_type, compilation,
    musicbrainz_track_id, musicbrainz_album_id, musicbrainz_artist_id,
    original_title, sort_title
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11, $12,
    $13, $14, $15, $16, $17,
    $18, $19, $20,
    $21, $22
);

-- name: GetMusicTrackByMediaID :one
SELECT
    mt.*,
    med.id as media_id,
    med.library_id,
    med.title,
    med.file_path,
    med.file_size,
    med.file_hash,
    med.container_format,
    med.duration,
    med.width,
    med.height,
    med.aspect_ratio,
    med.codec,
    med.audio_codec,
    med.codec_profile,
    med.bit_rate,
    med.frame_rate,
    med.scan_type,
    med.hdr_format,
    med.color_space,
    med.color_primaries,
    med.thumbnail_path,
    med.type,
    med.source_type,
    med.resolution_label,
    med.quality_score,
    med.is_3d,
    med.stereo_mode,
    med.has_dash,
    med.dash_manifest_path,
    med.transcoding_status,
    med.is_extra,
    med.date_added,
    med.date_modified,
    med.created_at,
    med.updated_at
FROM music_tracks mt
JOIN media med ON mt.media_id = med.id
WHERE mt.media_id = $1;

-- name: ListMusicTracksByLibrary :many
SELECT
    mt.*,
    med.id as media_id,
    med.library_id,
    med.title,
    med.file_path,
    med.file_size,
    med.file_hash,
    med.container_format,
    med.duration,
    med.width,
    med.height,
    med.aspect_ratio,
    med.codec,
    med.audio_codec,
    med.codec_profile,
    med.bit_rate,
    med.frame_rate,
    med.scan_type,
    med.hdr_format,
    med.color_space,
    med.color_primaries,
    med.thumbnail_path,
    med.type,
    med.source_type,
    med.resolution_label,
    med.quality_score,
    med.is_3d,
    med.stereo_mode,
    med.has_dash,
    med.dash_manifest_path,
    med.transcoding_status,
    med.is_extra,
    med.date_added,
    med.date_modified,
    med.created_at,
    med.updated_at
FROM music_tracks mt
JOIN media med ON mt.media_id = med.id
WHERE med.library_id = $1
ORDER BY mt.album_artist, mt.album, mt.disc_number, mt.track_number;

-- name: ListMusicTracksByAlbum :many
SELECT
    mt.*,
    med.id as media_id,
    med.library_id,
    med.title,
    med.file_path,
    med.file_size,
    med.file_hash,
    med.container_format,
    med.duration,
    med.width,
    med.height,
    med.aspect_ratio,
    med.codec,
    med.audio_codec,
    med.codec_profile,
    med.bit_rate,
    med.frame_rate,
    med.scan_type,
    med.hdr_format,
    med.color_space,
    med.color_primaries,
    med.thumbnail_path,
    med.type,
    med.source_type,
    med.resolution_label,
    med.quality_score,
    med.is_3d,
    med.stereo_mode,
    med.has_dash,
    med.dash_manifest_path,
    med.transcoding_status,
    med.is_extra,
    med.date_added,
    med.date_modified,
    med.created_at,
    med.updated_at
FROM music_tracks mt
JOIN media med ON mt.media_id = med.id
WHERE med.library_id = $1 AND mt.album = $2
ORDER BY mt.disc_number, mt.track_number;

-- name: ListMusicTracksByArtist :many
SELECT
    mt.*,
    med.id as media_id,
    med.library_id,
    med.title,
    med.file_path,
    med.file_size,
    med.file_hash,
    med.container_format,
    med.duration,
    med.width,
    med.height,
    med.aspect_ratio,
    med.codec,
    med.audio_codec,
    med.codec_profile,
    med.bit_rate,
    med.frame_rate,
    med.scan_type,
    med.hdr_format,
    med.color_space,
    med.color_primaries,
    med.thumbnail_path,
    med.type,
    med.source_type,
    med.resolution_label,
    med.quality_score,
    med.is_3d,
    med.stereo_mode,
    med.has_dash,
    med.dash_manifest_path,
    med.transcoding_status,
    med.is_extra,
    med.date_added,
    med.date_modified,
    med.created_at,
    med.updated_at
FROM music_tracks mt
JOIN media med ON mt.media_id = med.id
WHERE med.library_id = $1
  AND (mt.artist ILIKE $2 OR mt.album_artist ILIKE $3)
ORDER BY mt.album, mt.disc_number, mt.track_number;

-- name: ListMusicTracksByAlbumArtist :many
SELECT
    mt.*,
    med.id as media_id,
    med.library_id,
    med.title,
    med.file_path,
    med.file_size,
    med.file_hash,
    med.container_format,
    med.duration,
    med.width,
    med.height,
    med.aspect_ratio,
    med.codec,
    med.audio_codec,
    med.codec_profile,
    med.bit_rate,
    med.frame_rate,
    med.scan_type,
    med.hdr_format,
    med.color_space,
    med.color_primaries,
    med.thumbnail_path,
    med.type,
    med.source_type,
    med.resolution_label,
    med.quality_score,
    med.is_3d,
    med.stereo_mode,
    med.has_dash,
    med.dash_manifest_path,
    med.transcoding_status,
    med.is_extra,
    med.date_added,
    med.date_modified,
    med.created_at,
    med.updated_at
FROM music_tracks mt
JOIN media med ON mt.media_id = med.id
WHERE med.library_id = $1 AND mt.album_artist = $2
ORDER BY mt.album, mt.disc_number, mt.track_number;

-- name: UpdateMusicTrack :exec
UPDATE music_tracks
SET artist = $1,
    album = $2,
    album_artist = $3,
    track_number = $4,
    disc_number = $5,
    total_tracks = $6,
    total_discs = $7,
    genre = $8,
    year = $9,
    release_date = $10,
    composer = $11,
    lyricist = $12,
    record_label = $13,
    isrc = $14,
    release_type = $15,
    compilation = $16,
    musicbrainz_track_id = $17,
    musicbrainz_album_id = $18,
    musicbrainz_artist_id = $19,
    original_title = $20,
    sort_title = $21
WHERE media_id = $22;

-- name: DeleteMusicTrack :exec
DELETE FROM music_tracks
WHERE media_id = $1;

-- name: SearchMusicTracks :many
SELECT
    mt.*,
    med.id as media_id,
    med.library_id,
    med.title,
    med.file_path,
    med.file_size,
    med.file_hash,
    med.container_format,
    med.duration,
    med.width,
    med.height,
    med.aspect_ratio,
    med.codec,
    med.audio_codec,
    med.codec_profile,
    med.bit_rate,
    med.frame_rate,
    med.scan_type,
    med.hdr_format,
    med.color_space,
    med.color_primaries,
    med.thumbnail_path,
    med.type,
    med.source_type,
    med.resolution_label,
    med.quality_score,
    med.is_3d,
    med.stereo_mode,
    med.has_dash,
    med.dash_manifest_path,
    med.transcoding_status,
    med.is_extra,
    med.date_added,
    med.date_modified,
    med.created_at,
    med.updated_at
FROM music_tracks mt
JOIN media med ON mt.media_id = med.id
WHERE med.library_id = $1
  AND (med.title ILIKE $2 OR mt.artist ILIKE $3 OR mt.album ILIKE $4)
ORDER BY mt.album_artist, mt.album, mt.disc_number, mt.track_number;

-- name: ListAlbumsByLibrary :many
SELECT DISTINCT
    mt.album,
    mt.album_artist,
    mt.year,
    COUNT(*) as track_count,
    SUM(med.duration) as total_duration
FROM music_tracks mt
JOIN media med ON mt.media_id = med.id
WHERE med.library_id = $1
GROUP BY mt.album, mt.album_artist, mt.year
ORDER BY mt.album_artist, mt.album;

-- name: ListArtistsByLibrary :many
SELECT DISTINCT
    mt.album_artist as artist,
    COUNT(DISTINCT mt.album) as album_count,
    COUNT(*) as track_count
FROM music_tracks mt
JOIN media med ON mt.media_id = med.id
WHERE med.library_id = $1 AND mt.album_artist IS NOT NULL AND mt.album_artist != ''
GROUP BY mt.album_artist
ORDER BY mt.album_artist;
