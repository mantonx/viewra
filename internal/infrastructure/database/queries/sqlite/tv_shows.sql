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
    ?, ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?, ?,
    ?, ?, ?, ?
) RETURNING *;

-- name: UpsertTVShow :one
-- Atomically creates a TV show or returns existing one if title already exists.
-- Uses ON CONFLICT to handle race conditions during concurrent episode scans.
-- On conflict, updates directory if it was previously empty.
INSERT INTO tv_shows (
    library_id, title, sort_title, directory
) VALUES (
    ?1, ?2, ?3, ?4
)
ON CONFLICT(library_id, LOWER(title)) DO UPDATE SET
    directory = CASE
        WHEN tv_shows.directory IS NULL OR tv_shows.directory = ''
        THEN excluded.directory
        ELSE tv_shows.directory
    END,
    updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: GetTVShowByID :one
SELECT * FROM tv_shows
WHERE id = ?;

-- name: GetTVShowByTitle :one
SELECT * FROM tv_shows
WHERE library_id = ? AND LOWER(title) = LOWER(?)
LIMIT 1;

-- name: GetTVShowByDirectory :one
-- Find a TV show by its directory path. Used to prevent duplicate shows
-- when different episodes parse to different titles but share the same directory.
SELECT * FROM tv_shows
WHERE library_id = ? AND directory = ?
LIMIT 1;

-- name: ListTVShowsByLibrary :many
SELECT * FROM tv_shows
WHERE library_id = ?
ORDER BY sort_title, title;

-- name: UpdateTVShow :exec
UPDATE tv_shows
SET title = ?,
    original_title = ?,
    sort_title = ?,
    year = ?,
    first_air_date = ?,
    last_air_date = ?,
    genre = ?,
    plot = ?,
    status = ?,
    content_rating = ?,
    maturity_rating = ?,
    network = ?,
    original_language = ?,
    country_of_origin = ?,
    imdb_id = ?,
    tmdb_id = ?,
    tvdb_id = ?,
    directory = ?,
    rating = ?,
    rating_votes = ?,
    tagline = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeleteTVShow :exec
DELETE FROM tv_shows
WHERE id = ?;

-- name: SearchTVShowsByTitle :many
SELECT * FROM tv_shows
WHERE library_id = ?
  AND (title LIKE ? OR original_title LIKE ?)
ORDER BY sort_title, title;

-- ============================================================================
-- TV Seasons
-- ============================================================================

-- name: CreateTVSeason :one
INSERT INTO tv_seasons (
    show_id, season_number, name, overview, air_date, poster_path, episode_count
) VALUES (
    ?, ?, ?, ?, ?, ?, ?
) RETURNING *;

-- name: GetTVSeasonByID :one
SELECT * FROM tv_seasons
WHERE id = ?;

-- name: GetTVSeasonByShowAndNumber :one
SELECT * FROM tv_seasons
WHERE show_id = ? AND season_number = ?
LIMIT 1;

-- name: ListTVSeasonsByShow :many
SELECT * FROM tv_seasons
WHERE show_id = ?
ORDER BY season_number;

-- name: UpdateTVSeason :exec
UPDATE tv_seasons
SET name = ?,
    overview = ?,
    air_date = ?,
    poster_path = ?,
    episode_count = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeleteTVSeason :exec
DELETE FROM tv_seasons
WHERE id = ?;

-- name: IncrementSeasonEpisodeCount :exec
UPDATE tv_seasons
SET episode_count = episode_count + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

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
    ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?, ?, ?,
    ?, ?, ?
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
WHERE e.media_id = ?;

-- name: GetEpisodeWithShowTitle :one
-- Returns episode details with the parent show's title for AI indexing
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
WHERE e.media_id = ?;

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
WHERE med.library_id = ?
  AND med.is_extra = 0
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
WHERE e.show_id = ?
  AND med.is_extra = 0
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
WHERE e.season_id = ?
  AND med.is_extra = 0
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
WHERE e.show_id = ? AND e.season_number = ? AND e.episode_number = ?
LIMIT 1;

-- name: UpdateTVEpisode :exec
UPDATE tv_episodes
SET show_id = ?,
    season_id = ?,
    season_number = ?,
    episode_number = ?,
    absolute_number = ?,
    dvd_season = ?,
    dvd_episode = ?,
    episode_title = ?,
    original_title = ?,
    air_date = ?,
    plot = ?,
    content_rating = ?,
    maturity_rating = ?,
    imdb_id = ?,
    tmdb_id = ?,
    tvdb_id = ?,
    rating = ?,
    rating_votes = ?,
    runtime_minutes = ?
WHERE media_id = ?;

-- name: DeleteTVEpisode :exec
DELETE FROM tv_episodes
WHERE media_id = ?;

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
WHERE med.library_id = ?
  AND med.is_extra = 0
  AND (e.episode_title LIKE ? OR s.title LIKE ?)
ORDER BY s.sort_title, e.season_number, e.episode_number;

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
WHERE s.library_id = ?
  AND (med.is_extra = 0 OR med.is_extra IS NULL)
GROUP BY s.id, s.library_id, s.title, s.year, s.genre, s.plot, s.content_rating, s.imdb_id, s.tmdb_id
ORDER BY s.sort_title, s.title;

-- ============================================================================
-- Pagination Support Queries
-- ============================================================================

-- name: CountTVShowsByLibrary :one
SELECT COUNT(*)
FROM tv_shows
WHERE library_id = ?;

-- name: ListTVShowsByLibraryPaginated :many
SELECT * FROM tv_shows
WHERE library_id = ?
ORDER BY COALESCE(sort_title, title) COLLATE NOCASE ASC
LIMIT ? OFFSET ?;

-- name: ListTVShowsByLibraryPaginatedDesc :many
SELECT * FROM tv_shows
WHERE library_id = ?
ORDER BY COALESCE(sort_title, title) COLLATE NOCASE DESC
LIMIT ? OFFSET ?;

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
WHERE s.library_id = ?
  AND (med.is_extra = 0 OR med.is_extra IS NULL)
GROUP BY s.id, s.library_id, s.title, s.year, s.genre, s.plot, s.content_rating, s.imdb_id, s.tmdb_id, s.created_at
ORDER BY COALESCE(s.sort_title, s.title) COLLATE NOCASE ASC
LIMIT ? OFFSET ?;

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
WHERE s.library_id = ?
  AND (med.is_extra = 0 OR med.is_extra IS NULL)
GROUP BY s.id, s.library_id, s.title, s.year, s.genre, s.plot, s.content_rating, s.imdb_id, s.tmdb_id, s.created_at
ORDER BY COALESCE(s.sort_title, s.title) COLLATE NOCASE DESC
LIMIT ? OFFSET ?;

-- name: CountSearchTVShowsByTitle :one
SELECT COUNT(*)
FROM tv_shows
WHERE library_id = ?
  AND (title LIKE ? OR original_title LIKE ?);

-- name: SearchTVShowsByTitlePaginated :many
SELECT * FROM tv_shows
WHERE library_id = ?
  AND (title LIKE ? OR original_title LIKE ?)
ORDER BY sort_title, title
LIMIT ? OFFSET ?;

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
WHERE s.library_id = ?
  AND (s.title LIKE ? OR s.original_title LIKE ?)
  AND (med.is_extra = 0 OR med.is_extra IS NULL)
GROUP BY s.id, s.library_id, s.title, s.year, s.genre, s.plot, s.content_rating, s.imdb_id, s.tmdb_id, s.created_at
ORDER BY COALESCE(s.sort_title, s.title) COLLATE NOCASE ASC
LIMIT ? OFFSET ?;

-- name: ListTVShowIDsByLibraryPaginated :many
SELECT id
FROM tv_shows
WHERE library_id = ?
ORDER BY COALESCE(sort_title, title) COLLATE NOCASE ASC
LIMIT ? OFFSET ?;

-- name: ListTVShowIDsByLibraryPaginatedDesc :many
SELECT id
FROM tv_shows
WHERE library_id = ?
ORDER BY COALESCE(sort_title, title) COLLATE NOCASE DESC
LIMIT ? OFFSET ?;

-- name: SearchTVShowsGlobal :many
-- Searches TV shows across all libraries (for plugin use)
SELECT
    id,
    library_id,
    title,
    year,
    original_title
FROM tv_shows
WHERE (title LIKE ? OR original_title LIKE ?)
ORDER BY sort_title, title
LIMIT ?;
