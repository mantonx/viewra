-- name: CreateMovie :exec
INSERT INTO movies (
    media_id, year, release_date, genre, director, cast,
    content_rating, maturity_rating, content_advisories, plot, tagline,
    original_title, sort_title, imdb_id, tmdb_id, runtime_minutes,
    budget, revenue, original_language, country_of_origin, awards_summary
) VALUES (
    ?, ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?
);

-- name: GetMovieByMediaID :one
SELECT
    m.*,
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
FROM movies m
JOIN media med ON m.media_id = med.id
WHERE m.media_id = ?;

-- name: ListMoviesByLibrary :many
SELECT
    m.*,
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
FROM movies m
JOIN media med ON m.media_id = med.id
WHERE med.library_id = ?
ORDER BY m.sort_title, med.title;

-- name: UpdateMovie :exec
UPDATE movies
SET year = ?,
    release_date = ?,
    genre = ?,
    director = ?,
    cast = ?,
    content_rating = ?,
    maturity_rating = ?,
    content_advisories = ?,
    plot = ?,
    tagline = ?,
    original_title = ?,
    sort_title = ?,
    imdb_id = ?,
    tmdb_id = ?,
    runtime_minutes = ?,
    budget = ?,
    revenue = ?,
    original_language = ?,
    country_of_origin = ?,
    awards_summary = ?
WHERE media_id = ?;

-- name: SearchMoviesByTitle :many
SELECT
    m.*,
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
FROM movies m
JOIN media med ON m.media_id = med.id
WHERE med.library_id = ?
  AND (med.title LIKE ? OR m.original_title LIKE ?)
ORDER BY m.sort_title, med.title;

-- name: ListMoviesByGenre :many
SELECT
    m.*,
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
FROM movies m
JOIN media med ON m.media_id = med.id
WHERE med.library_id = ?
  AND m.genre LIKE ?
ORDER BY m.sort_title, med.title;

-- name: ListMoviesByYear :many
SELECT
    m.*,
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
FROM movies m
JOIN media med ON m.media_id = med.id
WHERE med.library_id = ?
  AND m.year = ?
ORDER BY m.sort_title, med.title;

-- name: ListMoviesByLibraryPaginated :many
SELECT
    m.*,
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
FROM movies m
JOIN media med ON m.media_id = med.id
WHERE med.library_id = ?
ORDER BY COALESCE(m.sort_title, med.title) COLLATE NOCASE ASC
LIMIT ? OFFSET ?;

-- name: ListMoviesByLibraryPaginatedDesc :many
SELECT
    m.*,
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
FROM movies m
JOIN media med ON m.media_id = med.id
WHERE med.library_id = ?
ORDER BY COALESCE(m.sort_title, med.title) COLLATE NOCASE DESC
LIMIT ? OFFSET ?;

-- name: CountMoviesByLibrary :one
SELECT COUNT(*)
FROM movies m
JOIN media med ON m.media_id = med.id
WHERE med.library_id = ?;

-- name: CountSearchMoviesByTitle :one
SELECT COUNT(*)
FROM movies m
JOIN media med ON m.media_id = med.id
WHERE med.library_id = ?
  AND (med.title LIKE ? OR m.original_title LIKE ?);

-- name: SearchMoviesByTitlePaginated :many
SELECT
    m.*,
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
FROM movies m
JOIN media med ON m.media_id = med.id
WHERE med.library_id = ?
  AND (med.title LIKE ? OR m.original_title LIKE ?)
ORDER BY COALESCE(m.sort_title, med.title) COLLATE NOCASE ASC
LIMIT ? OFFSET ?;

-- name: DeleteMovie :exec
DELETE FROM movies
WHERE media_id = ?;

-- name: ListMovieIDsByLibraryPaginated :many
SELECT med.id
FROM movies m
JOIN media med ON m.media_id = med.id
WHERE med.library_id = ?
ORDER BY COALESCE(m.sort_title, med.title) COLLATE NOCASE ASC
LIMIT ? OFFSET ?;

-- name: ListMovieIDsByLibraryPaginatedDesc :many
SELECT med.id
FROM movies m
JOIN media med ON m.media_id = med.id
WHERE med.library_id = ?
ORDER BY COALESCE(m.sort_title, med.title) COLLATE NOCASE DESC
LIMIT ? OFFSET ?;
