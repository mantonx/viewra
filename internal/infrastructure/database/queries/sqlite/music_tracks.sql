-- name: CreateMusicTrack :exec
INSERT INTO music_tracks (
    media_id, artist, album, album_artist, track_number, disc_number,
    total_tracks, total_discs, genre, year, release_date, composer,
    lyricist, record_label, isrc, release_type, compilation,
    musicbrainz_track_id, musicbrainz_album_id, musicbrainz_artist_id,
    original_title, sort_title, album_id, artist_id
) VALUES (
    ?, ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?,
    ?, ?, ?,
    ?, ?, ?, ?
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
WHERE mt.media_id = ?;

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
WHERE med.library_id = ?
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
WHERE med.library_id = ? AND mt.album = ?
ORDER BY mt.disc_number, mt.track_number;

-- name: ListMusicTracksByAlbumID :many
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
WHERE mt.album_id = ?
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
WHERE med.library_id = ?
  AND (mt.artist LIKE ? OR mt.album_artist LIKE ?)
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
WHERE med.library_id = ? AND mt.album_artist = ?
ORDER BY mt.album, mt.disc_number, mt.track_number;

-- name: UpdateMusicTrack :exec
UPDATE music_tracks
SET artist = ?,
    album = ?,
    album_artist = ?,
    track_number = ?,
    disc_number = ?,
    total_tracks = ?,
    total_discs = ?,
    genre = ?,
    year = ?,
    release_date = ?,
    composer = ?,
    lyricist = ?,
    record_label = ?,
    isrc = ?,
    release_type = ?,
    compilation = ?,
    musicbrainz_track_id = ?,
    musicbrainz_album_id = ?,
    musicbrainz_artist_id = ?,
    original_title = ?,
    sort_title = ?,
    album_id = ?,
    artist_id = ?
WHERE media_id = ?;

-- name: DeleteMusicTrack :exec
DELETE FROM music_tracks
WHERE media_id = ?;

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
WHERE med.library_id = ?
  AND (med.title LIKE ? OR mt.artist LIKE ? OR mt.album LIKE ?)
ORDER BY mt.album_artist, mt.album, mt.disc_number, mt.track_number;

-- name: ListAlbumsByLibraryGrouped :many
SELECT DISTINCT
    mt.album,
    mt.album_artist,
    mt.year,
    COUNT(*) as track_count,
    SUM(med.duration) as total_duration
FROM music_tracks mt
JOIN media med ON mt.media_id = med.id
WHERE med.library_id = ?
GROUP BY mt.album, mt.album_artist, mt.year
ORDER BY mt.album_artist, mt.album;

-- ============================================================================
-- Pagination Support Queries
-- ============================================================================

-- name: CountAlbumsByLibrary :one
SELECT COUNT(DISTINCT mt.album || mt.album_artist)
FROM music_tracks mt
JOIN media med ON mt.media_id = med.id
WHERE med.library_id = ?;

-- name: ListAlbumsByLibraryPaginated :many
SELECT DISTINCT
    mt.album,
    mt.album_artist,
    mt.year,
    COUNT(*) as track_count,
    SUM(med.duration) as total_duration
FROM music_tracks mt
JOIN media med ON mt.media_id = med.id
WHERE med.library_id = ?
GROUP BY mt.album, mt.album_artist, mt.year
ORDER BY mt.album_artist COLLATE NOCASE ASC, mt.album COLLATE NOCASE ASC
LIMIT ? OFFSET ?;

-- name: ListAlbumsByLibraryPaginatedDesc :many
SELECT DISTINCT
    mt.album,
    mt.album_artist,
    mt.year,
    COUNT(*) as track_count,
    SUM(med.duration) as total_duration
FROM music_tracks mt
JOIN media med ON mt.media_id = med.id
WHERE med.library_id = ?
GROUP BY mt.album, mt.album_artist, mt.year
ORDER BY mt.album_artist COLLATE NOCASE DESC, mt.album COLLATE NOCASE DESC
LIMIT ? OFFSET ?;

-- name: CountMusicTracksByLibrary :one
SELECT COUNT(*)
FROM music_tracks mt
JOIN media med ON mt.media_id = med.id
WHERE med.library_id = ?;

-- name: ListMusicTracksByLibraryPaginated :many
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
WHERE med.library_id = ?
ORDER BY COALESCE(mt.sort_title, med.title) COLLATE NOCASE ASC
LIMIT ? OFFSET ?;

-- name: ListMusicTracksByLibraryPaginatedDesc :many
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
WHERE med.library_id = ?
ORDER BY COALESCE(mt.sort_title, med.title) COLLATE NOCASE DESC
LIMIT ? OFFSET ?;

-- name: ListArtistIDsByLibraryPaginated :many
SELECT mt.media_id as id
FROM music_tracks mt
JOIN media med ON mt.media_id = med.id
WHERE med.library_id = ?
GROUP BY mt.artist
ORDER BY COALESCE(mt.sort_artist, mt.artist) COLLATE NOCASE ASC
LIMIT ? OFFSET ?;

-- name: ListArtistIDsByLibraryPaginatedDesc :many
SELECT mt.media_id as id
FROM music_tracks mt
JOIN media med ON mt.media_id = med.id
WHERE med.library_id = ?
GROUP BY mt.artist
ORDER BY COALESCE(mt.sort_artist, mt.artist) COLLATE NOCASE DESC
LIMIT ? OFFSET ?;
