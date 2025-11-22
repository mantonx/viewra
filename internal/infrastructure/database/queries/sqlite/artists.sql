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
    updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
    updated_at = ?
WHERE id = ?;

-- name: DeleteArtist :exec
DELETE FROM music_artists
WHERE id = ?;

-- name: CountArtistsInLibrary :one
SELECT COUNT(*) FROM music_artists
WHERE library_id = ?;
