-- name: CreateMovie :exec
INSERT INTO movies (
    media_id, year, release_date, genre, director, cast,
    content_rating, maturity_rating, content_advisories, plot, tagline,
    original_title, sort_title, imdb_id, tmdb_id, runtime_minutes,
    budget, revenue, original_language, country_of_origin, awards_summary,
    rating, rating_votes
) VALUES (
    ?, ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?,
    ?, ?
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
  AND med.is_extra = 0
ORDER BY COALESCE(NULLIF(m.sort_title, ''), med.title) COLLATE NOCASE;

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
    awards_summary = ?,
    rating = ?,
    rating_votes = ?
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
  AND med.is_extra = 0
  AND (med.title LIKE ? OR m.original_title LIKE ?)
ORDER BY COALESCE(NULLIF(m.sort_title, ''), med.title) COLLATE NOCASE;

-- name: ListMoviesByGenre :many
-- Lists movies matching a genre pattern with optional library filter and exclusion list.
-- library_id: 0 means all libraries
-- genre: genre pattern to match (will be wrapped in % for LIKE)
-- exclude_ids: array of media IDs to exclude
-- limit: maximum number of results
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
WHERE (med.library_id = sqlc.arg(library_id) OR sqlc.arg(library_id) = 0)
  AND med.is_extra = 0
  AND m.genre LIKE '%' || sqlc.arg(genre) || '%'
  AND med.id NOT IN (sqlc.slice('exclude_ids'))
ORDER BY COALESCE(m.rating, 0) * (COALESCE(m.rating_votes, 0) + 1) DESC, med.date_added DESC
LIMIT sqlc.arg(limit);

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
  AND med.is_extra = 0
  AND m.year = ?
ORDER BY COALESCE(NULLIF(m.sort_title, ''), med.title) COLLATE NOCASE;

-- name: ListMoviesByLibraryPaginated :many
-- library_id: 0 means all libraries
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
WHERE (med.library_id = sqlc.arg(library_id) OR sqlc.arg(library_id) = 0)
  AND med.is_extra = 0
ORDER BY COALESCE(NULLIF(m.sort_title, ''), med.title) COLLATE NOCASE ASC
LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset);

-- name: ListMoviesByLibraryPaginatedDesc :many
-- library_id: 0 means all libraries
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
WHERE (med.library_id = sqlc.arg(library_id) OR sqlc.arg(library_id) = 0)
  AND med.is_extra = 0
ORDER BY COALESCE(NULLIF(m.sort_title, ''), med.title) COLLATE NOCASE DESC
LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset);

-- name: CountMoviesByLibrary :one
-- library_id: 0 means all libraries
SELECT COUNT(*)
FROM movies m
JOIN media med ON m.media_id = med.id
WHERE (med.library_id = sqlc.arg(library_id) OR sqlc.arg(library_id) = 0)
  AND med.is_extra = 0;

-- name: CountSearchMoviesByTitle :one
-- library_id: 0 means all libraries
SELECT COUNT(*)
FROM movies m
JOIN media med ON m.media_id = med.id
WHERE (med.library_id = sqlc.arg(library_id) OR sqlc.arg(library_id) = 0)
  AND med.is_extra = 0
  AND (med.title LIKE sqlc.arg(query) OR m.original_title LIKE sqlc.arg(query));

-- name: SearchMoviesByTitlePaginated :many
-- library_id: 0 means all libraries
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
WHERE (med.library_id = sqlc.arg(library_id) OR sqlc.arg(library_id) = 0)
  AND med.is_extra = 0
  AND (med.title LIKE sqlc.arg(query) OR m.original_title LIKE sqlc.arg(query))
ORDER BY COALESCE(NULLIF(m.sort_title, ''), med.title) COLLATE NOCASE ASC
LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset);

-- name: DeleteMovie :exec
DELETE FROM movies
WHERE media_id = ?;

-- name: ListMovieIDsByLibraryPaginated :many
-- library_id: 0 means all libraries
SELECT med.id
FROM movies m
JOIN media med ON m.media_id = med.id
WHERE (med.library_id = sqlc.arg(library_id) OR sqlc.arg(library_id) = 0)
  AND med.is_extra = 0
ORDER BY COALESCE(NULLIF(m.sort_title, ''), med.title) COLLATE NOCASE ASC
LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset);

-- name: ListMovieIDsByLibraryPaginatedDesc :many
-- library_id: 0 means all libraries
SELECT med.id
FROM movies m
JOIN media med ON m.media_id = med.id
WHERE (med.library_id = sqlc.arg(library_id) OR sqlc.arg(library_id) = 0)
  AND med.is_extra = 0
ORDER BY COALESCE(NULLIF(m.sort_title, ''), med.title) COLLATE NOCASE DESC
LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset);

-- name: SearchMoviesGlobal :many
-- Searches movies across all libraries (for plugin use)
SELECT
    med.id as media_id,
    med.library_id,
    med.title,
    med.file_path,
    m.year,
    m.original_title
FROM movies m
JOIN media med ON m.media_id = med.id
WHERE med.is_extra = 0
  AND (med.title LIKE ? OR m.original_title LIKE ?)
ORDER BY m.sort_title, med.title
LIMIT ?;

-- name: ListRecentlyAddedMovies :many
-- Returns recently added movies across all libraries, ordered by creation date
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
WHERE med.is_extra = 0
ORDER BY med.created_at DESC
LIMIT ?;

-- name: ListDistinctMovieGenres :many
-- Returns distinct genres from all movies (genres are comma-separated in the genre column)
SELECT DISTINCT TRIM(j.value) as genre
FROM movies m
JOIN media med ON m.media_id = med.id
CROSS JOIN json_each('["' || REPLACE(m.genre, ', ', '","') || '"]') j
WHERE med.is_extra = 0
  AND m.genre IS NOT NULL
  AND m.genre != ''
  AND TRIM(j.value) != ''
ORDER BY genre
LIMIT ?;
