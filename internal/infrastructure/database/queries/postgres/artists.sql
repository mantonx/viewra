-- name: CreateArtist :one
INSERT INTO music_artists (
    library_id,
    name,
    sort_name,
    musicbrainz_artist_id,
    bio,
    country,
    formed_year,
    genre,
    image_path,
    created_at,
    updated_at,
    directory
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: GetArtistByID :one
SELECT * FROM music_artists
WHERE id = $1;

-- name: GetArtistByMusicBrainzID :one
SELECT * FROM music_artists
WHERE musicbrainz_artist_id = $1;

-- name: FindArtistByName :one
SELECT * FROM music_artists
WHERE library_id = $1 AND name = $2
LIMIT 1;

-- name: ListArtistsByLibrary :many
SELECT * FROM music_artists
WHERE library_id = $1
ORDER BY sort_name, name;

-- name: UpdateArtist :exec
UPDATE music_artists
SET name = $1,
    sort_name = $2,
    musicbrainz_artist_id = $3,
    bio = $4,
    country = $5,
    formed_year = $6,
    genre = $7,
    image_path = $8,
    directory = $9,
    updated_at = $10
WHERE id = $11;

-- name: DeleteArtist :exec
DELETE FROM music_artists
WHERE id = $1;

-- name: CountArtistsInLibrary :one
SELECT COUNT(*) FROM music_artists
WHERE library_id = $1;

-- name: SearchArtistsByName :many
SELECT * FROM music_artists
WHERE library_id = $1
  AND (name ILIKE $2 OR sort_name ILIKE $3)
ORDER BY sort_name, name
LIMIT sqlc.arg('limit')::bigint OFFSET sqlc.arg('offset')::bigint;

-- name: CountSearchArtistsByName :one
SELECT COUNT(*) FROM music_artists
WHERE library_id = $1
  AND (name ILIKE $2 OR sort_name ILIKE $3);

-- ============================================================================
-- Aggregation Queries for API (optimized)
-- ============================================================================

-- name: GetArtistsWithCountsByLibrary :many
SELECT
    a.id,
    a.library_id,
    a.name,
    a.sort_name,
    a.musicbrainz_artist_id,
    a.bio,
    a.country,
    a.formed_year,
    a.genre,
    a.image_path,
    COUNT(DISTINCT al.id) as album_count,
    COUNT(DISTINCT mt.media_id) as track_count
FROM music_artists a
LEFT JOIN music_albums al ON a.id = al.artist_id
LEFT JOIN music_tracks mt ON a.id = mt.artist_id
WHERE a.library_id = $1
GROUP BY a.id, a.library_id, a.name, a.sort_name, a.musicbrainz_artist_id, a.bio, a.country, a.formed_year, a.genre, a.image_path
ORDER BY a.sort_name, a.name;

-- name: GetArtistsWithCountsByLibraryPaginated :many
SELECT
    a.id,
    a.library_id,
    a.name,
    a.sort_name,
    a.musicbrainz_artist_id,
    a.bio,
    a.country,
    a.formed_year,
    a.genre,
    a.image_path,
    a.created_at,
    COUNT(DISTINCT al.id) as album_count,
    COUNT(DISTINCT mt.media_id) as track_count
FROM music_artists a
LEFT JOIN music_albums al ON a.id = al.artist_id
LEFT JOIN music_tracks mt ON a.id = mt.artist_id
WHERE a.library_id = $1
GROUP BY a.id, a.library_id, a.name, a.sort_name, a.musicbrainz_artist_id, a.bio, a.country, a.formed_year, a.genre, a.image_path, a.created_at
ORDER BY COALESCE(a.sort_name, a.name) ASC
LIMIT sqlc.arg('limit')::bigint OFFSET sqlc.arg('offset')::bigint;

-- name: GetArtistsWithCountsByLibraryPaginatedDesc :many
SELECT
    a.id,
    a.library_id,
    a.name,
    a.sort_name,
    a.musicbrainz_artist_id,
    a.bio,
    a.country,
    a.formed_year,
    a.genre,
    a.image_path,
    a.created_at,
    COUNT(DISTINCT al.id) as album_count,
    COUNT(DISTINCT mt.media_id) as track_count
FROM music_artists a
LEFT JOIN music_albums al ON a.id = al.artist_id
LEFT JOIN music_tracks mt ON a.id = mt.artist_id
WHERE a.library_id = $1
GROUP BY a.id, a.library_id, a.name, a.sort_name, a.musicbrainz_artist_id, a.bio, a.country, a.formed_year, a.genre, a.image_path, a.created_at
ORDER BY COALESCE(a.sort_name, a.name) DESC
LIMIT sqlc.arg('limit')::bigint OFFSET sqlc.arg('offset')::bigint;

-- name: SearchArtistsWithCountsByNamePaginated :many
SELECT
    a.id,
    a.library_id,
    a.name,
    a.sort_name,
    a.musicbrainz_artist_id,
    a.bio,
    a.country,
    a.formed_year,
    a.genre,
    a.image_path,
    a.created_at,
    COUNT(DISTINCT al.id) as album_count,
    COUNT(DISTINCT mt.media_id) as track_count
FROM music_artists a
LEFT JOIN music_albums al ON a.id = al.artist_id
LEFT JOIN music_tracks mt ON a.id = mt.artist_id
WHERE a.library_id = $1
  AND (a.name ILIKE $2 OR a.sort_name ILIKE $3)
GROUP BY a.id, a.library_id, a.name, a.sort_name, a.musicbrainz_artist_id, a.bio, a.country, a.formed_year, a.genre, a.image_path, a.created_at
ORDER BY COALESCE(a.sort_name, a.name) ASC
LIMIT sqlc.arg('limit')::bigint OFFSET sqlc.arg('offset')::bigint;
