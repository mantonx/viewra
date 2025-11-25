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
    updated_at,
    artist_id
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetAlbumByID :one
SELECT * FROM music_albums
WHERE id = ?;

-- name: GetAlbumByMusicBrainzID :one
SELECT * FROM music_albums
WHERE musicbrainz_album_id = ?;

-- name: FindAlbumByTitle :one
SELECT * FROM music_albums
WHERE library_id = ? AND title = ? AND album_artist = ?
LIMIT 1;

-- name: ListAlbumsByLibrary :many
SELECT * FROM music_albums
WHERE library_id = ?
ORDER BY sort_title, title;

-- name: ListAlbumsByArtist :many
SELECT * FROM music_albums
WHERE library_id = ? AND (album_artist = ? OR artist = ?)
ORDER BY year DESC, sort_title, title;

-- name: UpdateAlbum :exec
UPDATE music_albums
SET title = ?,
    album_artist = ?,
    artist = ?,
    year = ?,
    release_date = ?,
    genre = ?,
    total_tracks = ?,
    total_discs = ?,
    record_label = ?,
    release_type = ?,
    compilation = ?,
    musicbrainz_album_id = ?,
    cover_art_path = ?,
    sort_title = ?,
    updated_at = ?
WHERE id = ?;

-- name: DeleteAlbum :exec
DELETE FROM music_albums
WHERE id = ?;

-- name: CountAlbumsInLibrary :one
SELECT COUNT(*) FROM music_albums
WHERE library_id = ?;
