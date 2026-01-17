-- name: CreateMovie :exec
INSERT INTO movies (
    media_id, year, release_date, genre, director, "cast",
    content_rating, maturity_rating, content_advisories, plot, tagline,
    original_title, sort_title, imdb_id, tmdb_id, runtime_minutes,
    budget, revenue, original_language, country_of_origin, awards_summary,
    rating, rating_votes
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11,
    $12, $13, $14, $15, $16,
    $17, $18, $19, $20, $21,
    $22, $23
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
  AND med.is_extra = 0
ORDER BY COALESCE(NULLIF(m.sort_title, ''), med.title);

-- name: UpdateMovie :exec
UPDATE movies
SET year = $1,
    release_date = $2,
    genre = $3,
    director = $4,
    "cast" = $5,
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
    awards_summary = $20,
    rating = $21,
    rating_votes = $22
WHERE media_id = $23;

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
  AND med.is_extra = 0
  AND (med.title ILIKE $2 OR m.original_title ILIKE $3)
ORDER BY COALESCE(NULLIF(m.sort_title, ''), med.title);

-- name: ListMoviesByGenre :many
-- Lists movies matching a genre pattern with optional library filter and exclusion list.
-- library_id: 0 means all libraries
-- genre: genre pattern to match (will be wrapped in % for ILIKE)
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
WHERE (med.library_id = sqlc.arg('library_id')::bigint OR sqlc.arg('library_id')::bigint = 0)
  AND med.is_extra = 0
  AND m.genre ILIKE '%' || sqlc.arg('genre') || '%'
  AND NOT (med.id = ANY(sqlc.arg('exclude_ids')::bigint[]))
ORDER BY COALESCE(m.rating, 0) * LOG(COALESCE(m.rating_votes, 0) + 1) DESC, med.date_added DESC
LIMIT sqlc.arg('limit')::bigint;

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
  AND med.is_extra = 0
  AND m.year = $2
ORDER BY COALESCE(NULLIF(m.sort_title, ''), med.title);

-- name: DeleteMovie :exec
DELETE FROM movies
WHERE media_id = $1;

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
WHERE (med.library_id = sqlc.arg('library_id')::bigint OR sqlc.arg('library_id')::bigint = 0)
  AND med.is_extra = 0
ORDER BY COALESCE(NULLIF(m.sort_title, ''), med.title) ASC
LIMIT sqlc.arg('limit')::bigint OFFSET sqlc.arg('offset')::bigint;

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
WHERE (med.library_id = sqlc.arg('library_id')::bigint OR sqlc.arg('library_id')::bigint = 0)
  AND med.is_extra = 0
ORDER BY COALESCE(NULLIF(m.sort_title, ''), med.title) DESC
LIMIT sqlc.arg('limit')::bigint OFFSET sqlc.arg('offset')::bigint;

-- name: CountMoviesByLibrary :one
-- library_id: 0 means all libraries
SELECT COUNT(*)
FROM movies m
JOIN media med ON m.media_id = med.id
WHERE (med.library_id = sqlc.arg('library_id')::bigint OR sqlc.arg('library_id')::bigint = 0)
  AND med.is_extra = 0;

-- name: CountSearchMoviesByTitle :one
-- library_id: 0 means all libraries
SELECT COUNT(*)
FROM movies m
JOIN media med ON m.media_id = med.id
WHERE (med.library_id = sqlc.arg('library_id')::bigint OR sqlc.arg('library_id')::bigint = 0)
  AND med.is_extra = 0
  AND (med.title ILIKE sqlc.arg('query') OR m.original_title ILIKE sqlc.arg('query'));

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
WHERE (med.library_id = sqlc.arg('library_id')::bigint OR sqlc.arg('library_id')::bigint = 0)
  AND med.is_extra = 0
  AND (med.title ILIKE sqlc.arg('query') OR m.original_title ILIKE sqlc.arg('query'))
ORDER BY COALESCE(NULLIF(m.sort_title, ''), med.title) ASC
LIMIT sqlc.arg('limit')::bigint OFFSET sqlc.arg('offset')::bigint;

-- name: ListMovieIDsByLibraryPaginated :many
-- library_id: 0 means all libraries
SELECT med.id
FROM movies m
JOIN media med ON m.media_id = med.id
WHERE (med.library_id = sqlc.arg('library_id')::bigint OR sqlc.arg('library_id')::bigint = 0)
  AND med.is_extra = 0
ORDER BY COALESCE(NULLIF(m.sort_title, ''), med.title) ASC
LIMIT sqlc.arg('limit')::bigint OFFSET sqlc.arg('offset')::bigint;

-- name: ListMovieIDsByLibraryPaginatedDesc :many
-- library_id: 0 means all libraries
SELECT med.id
FROM movies m
JOIN media med ON m.media_id = med.id
WHERE (med.library_id = sqlc.arg('library_id')::bigint OR sqlc.arg('library_id')::bigint = 0)
  AND med.is_extra = 0
ORDER BY COALESCE(NULLIF(m.sort_title, ''), med.title) DESC
LIMIT sqlc.arg('limit')::bigint OFFSET sqlc.arg('offset')::bigint;

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
  AND (med.title ILIKE $1 OR m.original_title ILIKE $2)
ORDER BY m.sort_title, med.title
LIMIT sqlc.arg('limit')::bigint;

-- name: ListRecentlyAddedMovies :many
-- Returns recently added movies across all libraries, ordered by most recently updated
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
ORDER BY med.updated_at DESC
LIMIT $1::bigint;

-- name: ListDistinctMovieGenres :many
-- Returns distinct genres from all movies (genres are comma-separated in the genre column)
SELECT DISTINCT TRIM(g.genre) as genre
FROM movies m
JOIN media med ON m.media_id = med.id
CROSS JOIN LATERAL unnest(string_to_array(m.genre, ', ')) AS g(genre)
WHERE med.is_extra = 0
  AND m.genre IS NOT NULL
  AND m.genre != ''
  AND TRIM(g.genre) != ''
ORDER BY TRIM(g.genre)
LIMIT $1::bigint;

-- name: FindMovieByTitleAndYear :one
-- Finds a movie by normalized title and year for deduplication (file replacement detection)
-- Used when a new file is detected that might be a replacement for an existing movie
-- Note: year comparison uses coalesce to handle NULL consistently
SELECT
    m.media_id,
    med.file_path
FROM movies m
JOIN media med ON m.media_id = med.id
WHERE med.library_id = sqlc.arg(library_id)
  AND med.is_extra = 0
  AND LOWER(med.title) = LOWER(sqlc.arg(title))
  AND COALESCE(m.year, 0) = COALESCE(sqlc.arg(year), 0)
LIMIT 1;
