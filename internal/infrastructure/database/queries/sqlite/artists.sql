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
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetArtistByID :one
SELECT * FROM music_artists
WHERE id = ?;

-- name: GetArtistByMusicBrainzID :one
SELECT * FROM music_artists
WHERE musicbrainz_artist_id = ?;

-- name: FindArtistByName :one
SELECT * FROM music_artists
WHERE library_id = ? AND name = ?
LIMIT 1;

-- name: ListArtistsByLibrary :many
SELECT * FROM music_artists
WHERE library_id = ?
ORDER BY sort_name, name;

-- name: UpdateArtist :exec
UPDATE music_artists
SET name = ?,
    sort_name = ?,
    musicbrainz_artist_id = ?,
    bio = ?,
    country = ?,
    formed_year = ?,
    genre = ?,
    image_path = ?,
    directory = ?,
    updated_at = ?
WHERE id = ?;

-- name: DeleteArtist :exec
DELETE FROM music_artists
WHERE id = ?;

-- name: CountArtistsInLibrary :one
SELECT COUNT(*) FROM music_artists
WHERE library_id = ?;

-- name: SearchArtistsByName :many
SELECT * FROM music_artists
WHERE library_id = ?
  AND (name LIKE ? OR sort_name LIKE ?)
ORDER BY sort_name, name
LIMIT ? OFFSET ?;

-- name: CountSearchArtistsByName :one
SELECT COUNT(*) FROM music_artists
WHERE library_id = ?
  AND (name LIKE ? OR sort_name LIKE ?);

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
WHERE a.library_id = ?
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
WHERE a.library_id = ?
GROUP BY a.id, a.library_id, a.name, a.sort_name, a.musicbrainz_artist_id, a.bio, a.country, a.formed_year, a.genre, a.image_path, a.created_at
ORDER BY COALESCE(a.sort_name, a.name) COLLATE NOCASE ASC
LIMIT ? OFFSET ?;

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
WHERE a.library_id = ?
GROUP BY a.id, a.library_id, a.name, a.sort_name, a.musicbrainz_artist_id, a.bio, a.country, a.formed_year, a.genre, a.image_path, a.created_at
ORDER BY COALESCE(a.sort_name, a.name) COLLATE NOCASE DESC
LIMIT ? OFFSET ?;

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
WHERE a.library_id = ?
  AND (a.name LIKE ? OR a.sort_name LIKE ?)
GROUP BY a.id, a.library_id, a.name, a.sort_name, a.musicbrainz_artist_id, a.bio, a.country, a.formed_year, a.genre, a.image_path, a.created_at
ORDER BY COALESCE(a.sort_name, a.name) COLLATE NOCASE ASC
LIMIT ? OFFSET ?;
