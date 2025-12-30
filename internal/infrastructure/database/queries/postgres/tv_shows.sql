-- ============================================================================
-- TV Shows
-- ============================================================================

-- name: CreateTVShow :one
INSERT INTO tv_shows (
    library_id, title, original_title, sort_title, year, first_air_date,
    last_air_date, genre, plot, status, content_rating, maturity_rating,
    network, original_language, country_of_origin, imdb_id, tmdb_id, tvdb_id,
    directory, rating, rating_votes, tagline
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11, $12,
    $13, $14, $15, $16, $17, $18,
    $19, $20, $21, $22
) RETURNING *;

-- name: UpsertTVShow :one
-- Atomically creates a TV show or returns existing one if title already exists.
-- Uses ON CONFLICT to handle race conditions during concurrent episode scans.
-- On conflict, updates directory if it was previously empty.
INSERT INTO tv_shows (
    library_id, title, sort_title, directory
) VALUES (
    $1, $2, $3, $4
)
ON CONFLICT(library_id, LOWER(title)) DO UPDATE SET
    directory = CASE
        WHEN tv_shows.directory IS NULL OR tv_shows.directory = ''
        THEN EXCLUDED.directory
        ELSE tv_shows.directory
    END,
    updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: GetTVShowByID :one
SELECT * FROM tv_shows
WHERE id = $1;

-- name: GetTVShowByTitle :one
SELECT * FROM tv_shows
WHERE library_id = $1 AND LOWER(title) = LOWER($2)
LIMIT 1;

-- name: GetTVShowByDirectory :one
-- Find a TV show by its directory path. Used to prevent duplicate shows
-- when different episodes parse to different titles but share the same directory.
SELECT * FROM tv_shows
WHERE library_id = $1 AND directory = $2
LIMIT 1;

-- name: ListTVShowsByLibrary :many
SELECT * FROM tv_shows
WHERE library_id = $1
ORDER BY COALESCE(NULLIF(sort_title, ''), title);

-- name: UpdateTVShow :exec
UPDATE tv_shows
SET title = $1,
    original_title = $2,
    sort_title = $3,
    year = $4,
    first_air_date = $5,
    last_air_date = $6,
    genre = $7,
    plot = $8,
    status = $9,
    content_rating = $10,
    maturity_rating = $11,
    network = $12,
    original_language = $13,
    country_of_origin = $14,
    imdb_id = $15,
    tmdb_id = $16,
    tvdb_id = $17,
    directory = $18,
    rating = $19,
    rating_votes = $20,
    tagline = $21,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $22;

-- name: DeleteTVShow :exec
DELETE FROM tv_shows
WHERE id = $1;

-- name: SearchTVShowsByTitle :many
SELECT * FROM tv_shows
WHERE library_id = $1
  AND (title ILIKE $2 OR original_title ILIKE $3)
ORDER BY COALESCE(NULLIF(sort_title, ''), title);

-- ============================================================================
-- TV Seasons
-- ============================================================================

-- name: CreateTVSeason :one
INSERT INTO tv_seasons (
    show_id, season_number, name, overview, air_date, poster_path, episode_count
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetTVSeasonByID :one
SELECT * FROM tv_seasons
WHERE id = $1;

-- name: GetTVSeasonByShowAndNumber :one
SELECT * FROM tv_seasons
WHERE show_id = $1 AND season_number = $2
LIMIT 1;

-- name: ListTVSeasonsByShow :many
SELECT * FROM tv_seasons
WHERE show_id = $1
ORDER BY season_number;

-- name: UpdateTVSeason :exec
UPDATE tv_seasons
SET name = $1,
    overview = $2,
    air_date = $3,
    poster_path = $4,
    episode_count = $5,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $6;

-- name: DeleteTVSeason :exec
DELETE FROM tv_seasons
WHERE id = $1;

-- name: IncrementSeasonEpisodeCount :exec
UPDATE tv_seasons
SET episode_count = episode_count + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1;

-- ============================================================================
-- TV Episodes
-- ============================================================================

-- name: CreateTVEpisode :exec
INSERT INTO tv_episodes (
    media_id, show_id, season_id, season_number, episode_number,
    absolute_number, dvd_season, dvd_episode, episode_title, original_title,
    air_date, plot, content_rating, maturity_rating, imdb_id, tmdb_id, tvdb_id,
    rating, rating_votes, runtime_minutes
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10,
    $11, $12, $13, $14, $15, $16, $17,
    $18, $19, $20
);

-- name: GetTVEpisodeByMediaID :one
SELECT
    e.*,
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
FROM tv_episodes e
JOIN media med ON e.media_id = med.id
WHERE e.media_id = $1;

-- name: GetEpisodeWithShowTitle :one
-- Returns episode details with the parent show's title for plugin indexing
SELECT
    e.media_id,
    med.library_id,
    med.title,
    e.season_number,
    e.episode_number,
    e.episode_title,
    e.plot,
    s.title as show_title,
    s.genre as show_genre
FROM tv_episodes e
JOIN media med ON e.media_id = med.id
JOIN tv_shows s ON e.show_id = s.id
WHERE e.media_id = $1;

-- name: ListTVEpisodesByLibrary :many
SELECT
    e.*,
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
FROM tv_episodes e
JOIN media med ON e.media_id = med.id
WHERE med.library_id = $1
  AND med.is_extra = false
ORDER BY e.show_id, e.season_number, e.episode_number;

-- name: ListTVEpisodesByShow :many
SELECT
    e.*,
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
FROM tv_episodes e
JOIN media med ON e.media_id = med.id
WHERE e.show_id = $1
  AND med.is_extra = false
ORDER BY e.season_number, e.episode_number;

-- name: ListTVEpisodesBySeason :many
SELECT
    e.*,
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
FROM tv_episodes e
JOIN media med ON e.media_id = med.id
WHERE e.season_id = $1
  AND med.is_extra = false
ORDER BY e.episode_number;

-- name: GetTVEpisodeByShowSeasonEpisode :one
SELECT
    e.*,
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
FROM tv_episodes e
JOIN media med ON e.media_id = med.id
WHERE e.show_id = $1 AND e.season_number = $2 AND e.episode_number = $3
LIMIT 1;

-- name: UpdateTVEpisode :exec
UPDATE tv_episodes
SET show_id = $1,
    season_id = $2,
    season_number = $3,
    episode_number = $4,
    absolute_number = $5,
    dvd_season = $6,
    dvd_episode = $7,
    episode_title = $8,
    original_title = $9,
    air_date = $10,
    plot = $11,
    content_rating = $12,
    maturity_rating = $13,
    imdb_id = $14,
    tmdb_id = $15,
    tvdb_id = $16,
    rating = $17,
    rating_votes = $18,
    runtime_minutes = $19
WHERE media_id = $20;

-- name: DeleteTVEpisode :exec
DELETE FROM tv_episodes
WHERE media_id = $1;

-- name: SearchTVEpisodesByTitle :many
SELECT
    e.*,
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
FROM tv_episodes e
JOIN media med ON e.media_id = med.id
JOIN tv_shows s ON e.show_id = s.id
WHERE med.library_id = $1
  AND med.is_extra = false
  AND (e.episode_title ILIKE $2 OR s.title ILIKE $3)
ORDER BY COALESCE(NULLIF(s.sort_title, ''), s.title), e.season_number, e.episode_number;

-- ============================================================================
-- Aggregation Queries for API
-- ============================================================================

-- name: GetTVShowsWithCountsByLibrary :many
SELECT
    s.id,
    s.library_id,
    s.title,
    s.year,
    s.genre,
    s.plot,
    s.content_rating,
    s.imdb_id,
    s.tmdb_id,
    COUNT(DISTINCT e.season_number) as season_count,
    COUNT(*) as episode_count
FROM tv_shows s
LEFT JOIN tv_episodes e ON s.id = e.show_id
LEFT JOIN media med ON e.media_id = med.id
WHERE s.library_id = $1
  AND (med.is_extra = false OR med.is_extra IS NULL)
GROUP BY s.id, s.library_id, s.title, s.year, s.genre, s.plot, s.content_rating, s.imdb_id, s.tmdb_id
ORDER BY COALESCE(NULLIF(s.sort_title, ''), s.title);

-- ============================================================================
-- Pagination Support Queries
-- ============================================================================

-- name: CountTVShowsByLibrary :one
SELECT COUNT(*)
FROM tv_shows
WHERE library_id = $1;

-- name: ListTVShowsByLibraryPaginated :many
SELECT * FROM tv_shows
WHERE library_id = $1
ORDER BY COALESCE(NULLIF(sort_title, ''), title) ASC
LIMIT $2 OFFSET $3;

-- name: ListTVShowsByLibraryPaginatedDesc :many
SELECT * FROM tv_shows
WHERE library_id = $1
ORDER BY COALESCE(NULLIF(sort_title, ''), title) DESC
LIMIT $2 OFFSET $3;

-- name: GetTVShowsWithCountsByLibraryPaginated :many
SELECT
    s.id,
    s.library_id,
    s.title,
    s.year,
    s.genre,
    s.plot,
    s.content_rating,
    s.imdb_id,
    s.tmdb_id,
    s.created_at,
    COUNT(DISTINCT e.season_number) as season_count,
    COUNT(*) as episode_count
FROM tv_shows s
LEFT JOIN tv_episodes e ON s.id = e.show_id
LEFT JOIN media med ON e.media_id = med.id
WHERE s.library_id = $1
  AND (med.is_extra = false OR med.is_extra IS NULL)
GROUP BY s.id, s.library_id, s.title, s.year, s.genre, s.plot, s.content_rating, s.imdb_id, s.tmdb_id, s.created_at
ORDER BY COALESCE(NULLIF(s.sort_title, ''), s.title) ASC
LIMIT $2 OFFSET $3;

-- name: GetTVShowsWithCountsByLibraryPaginatedDesc :many
SELECT
    s.id,
    s.library_id,
    s.title,
    s.year,
    s.genre,
    s.plot,
    s.content_rating,
    s.imdb_id,
    s.tmdb_id,
    s.created_at,
    COUNT(DISTINCT e.season_number) as season_count,
    COUNT(*) as episode_count
FROM tv_shows s
LEFT JOIN tv_episodes e ON s.id = e.show_id
LEFT JOIN media med ON e.media_id = med.id
WHERE s.library_id = $1
  AND (med.is_extra = false OR med.is_extra IS NULL)
GROUP BY s.id, s.library_id, s.title, s.year, s.genre, s.plot, s.content_rating, s.imdb_id, s.tmdb_id, s.created_at
ORDER BY COALESCE(NULLIF(s.sort_title, ''), s.title) DESC
LIMIT $2 OFFSET $3;

-- name: CountSearchTVShowsByTitle :one
SELECT COUNT(*)
FROM tv_shows
WHERE library_id = $1
  AND (title ILIKE $2 OR original_title ILIKE $3);

-- name: SearchTVShowsByTitlePaginated :many
SELECT * FROM tv_shows
WHERE library_id = $1
  AND (title ILIKE $2 OR original_title ILIKE $3)
ORDER BY COALESCE(NULLIF(sort_title, ''), title)
LIMIT $4 OFFSET $5;

-- name: SearchTVShowsWithCountsByTitlePaginated :many
SELECT
    s.id,
    s.library_id,
    s.title,
    s.year,
    s.genre,
    s.plot,
    s.content_rating,
    s.imdb_id,
    s.tmdb_id,
    s.created_at,
    COUNT(DISTINCT e.season_number) as season_count,
    COUNT(*) as episode_count
FROM tv_shows s
LEFT JOIN tv_episodes e ON s.id = e.show_id
LEFT JOIN media med ON e.media_id = med.id
WHERE s.library_id = $1
  AND (s.title ILIKE $2 OR s.original_title ILIKE $3)
  AND (med.is_extra = false OR med.is_extra IS NULL)
GROUP BY s.id, s.library_id, s.title, s.year, s.genre, s.plot, s.content_rating, s.imdb_id, s.tmdb_id, s.created_at
ORDER BY COALESCE(NULLIF(s.sort_title, ''), s.title) ASC
LIMIT $4 OFFSET $5;

-- name: ListTVShowIDsByLibraryPaginated :many
SELECT id
FROM tv_shows
WHERE library_id = $1
ORDER BY COALESCE(NULLIF(sort_title, ''), title) ASC
LIMIT $2 OFFSET $3;

-- name: ListTVShowIDsByLibraryPaginatedDesc :many
SELECT id
FROM tv_shows
WHERE library_id = $1
ORDER BY COALESCE(NULLIF(sort_title, ''), title) DESC
LIMIT $2 OFFSET $3;

-- name: SearchTVShowsGlobal :many
-- Searches TV shows across all libraries (for plugin use)
SELECT
    id,
    library_id,
    title,
    year,
    original_title
FROM tv_shows
WHERE (title ILIKE $1 OR original_title ILIKE $2)
ORDER BY COALESCE(NULLIF(sort_title, ''), title)
LIMIT $3;
