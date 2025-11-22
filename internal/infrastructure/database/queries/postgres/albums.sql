-- name: CreateAlbum :one
INSERT INTO music_albums (
    library_id,
    title,
    album_artist,
    artist,
    year,
    release_date,
    genre,
    total_tracks,
    total_discs,
    record_label,
    release_type,
    compilation,
    musicbrainz_album_id,
    cover_art_path,
    sort_title,
    created_at,
    updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
RETURNING *;

-- name: GetAlbumByID :one
SELECT * FROM music_albums
WHERE id = $1;

-- name: GetAlbumByMusicBrainzID :one
SELECT * FROM music_albums
WHERE musicbrainz_album_id = $1;

-- name: FindAlbumByTitle :one
SELECT * FROM music_albums
WHERE library_id = $1 AND title = $2 AND album_artist = $3
LIMIT 1;

-- name: ListAlbumsByLibrary :many
SELECT * FROM music_albums
WHERE library_id = $1
ORDER BY sort_title, title;

-- name: ListAlbumsByArtist :many
SELECT * FROM music_albums
WHERE library_id = $1 AND (album_artist = $2 OR artist = $3)
ORDER BY year DESC, sort_title, title;

-- name: UpdateAlbum :exec
UPDATE music_albums
SET title = $1,
    album_artist = $2,
    artist = $3,
    year = $4,
    release_date = $5,
    genre = $6,
    total_tracks = $7,
    total_discs = $8,
    record_label = $9,
    release_type = $10,
    compilation = $11,
    musicbrainz_album_id = $12,
    cover_art_path = $13,
    sort_title = $14,
    updated_at = $15
WHERE id = $16;

-- name: DeleteAlbum :exec
DELETE FROM music_albums
WHERE id = $1;

-- name: CountAlbumsInLibrary :one
SELECT COUNT(*) FROM music_albums
WHERE library_id = $1;
