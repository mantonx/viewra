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
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
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
    updated_at = $9
WHERE id = $10;

-- name: DeleteArtist :exec
DELETE FROM music_artists
WHERE id = $1;

-- name: CountArtistsInLibrary :one
SELECT COUNT(*) FROM music_artists
WHERE library_id = $1;
