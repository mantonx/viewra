-- name: CreateMovie :exec
INSERT INTO movies (
    media_id, year, release_date, genre, director, cast,
    content_rating, maturity_rating, content_advisories, plot, tagline,
    original_title, sort_title, imdb_id, tmdb_id, runtime_minutes,
    budget, revenue, original_language, country_of_origin, awards_summary
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11,
    $12, $13, $14, $15, $16,
    $17, $18, $19, $20, $21
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
WHERE m.media_id = $1;

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
WHERE med.library_id = $1
ORDER BY m.sort_title, med.title;

-- name: UpdateMovie :exec
UPDATE movies
SET year = $1,
    release_date = $2,
    genre = $3,
    director = $4,
    cast = $5,
    content_rating = $6,
    maturity_rating = $7,
    content_advisories = $8,
    plot = $9,
    tagline = $10,
    original_title = $11,
    sort_title = $12,
    imdb_id = $13,
    tmdb_id = $14,
    runtime_minutes = $15,
    budget = $16,
    revenue = $17,
    original_language = $18,
    country_of_origin = $19,
    awards_summary = $20
WHERE media_id = $21;

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
WHERE med.library_id = $1
  AND (med.title ILIKE $2 OR m.original_title ILIKE $3)
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
WHERE med.library_id = $1
  AND m.genre ILIKE $2
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
WHERE med.library_id = $1
  AND m.year = $2
ORDER BY m.sort_title, med.title;

-- name: DeleteMovie :exec
DELETE FROM movies
WHERE media_id = $1;
