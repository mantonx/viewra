-- name: CreateMedia :one
INSERT INTO media (
    library_id, title, file_path, file_size, file_hash,
    container_format, duration, width, height, aspect_ratio,
    codec, codec_profile, bit_rate, frame_rate, scan_type, hdr_format,
    color_space, color_primaries, thumbnail_path, type, source_type,
    resolution_label, quality_score, is_3d, stereo_mode, has_dash,
    dash_manifest_path, transcoding_status
) VALUES (
    ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?,
    ?, ?
) RETURNING *;

-- name: GetMediaByID :one
SELECT * FROM media
WHERE id = ?;

-- name: GetMediaByFilePath :one
SELECT * FROM media
WHERE library_id = ? AND file_path = ?;

-- name: ListMediaByLibrary :many
SELECT * FROM media
WHERE library_id = ?
ORDER BY title;

-- name: ListMediaByType :many
SELECT * FROM media
WHERE library_id = ? AND type = ?
ORDER BY title;

-- name: UpdateMedia :one
UPDATE media
SET library_id = ?,
    title = ?,
    file_path = ?,
    file_size = ?,
    file_hash = ?,
    container_format = ?,
    duration = ?,
    width = ?,
    height = ?,
    aspect_ratio = ?,
    codec = ?,
    codec_profile = ?,
    bit_rate = ?,
    frame_rate = ?,
    scan_type = ?,
    hdr_format = ?,
    color_space = ?,
    color_primaries = ?,
    thumbnail_path = ?,
    type = ?,
    source_type = ?,
    resolution_label = ?,
    quality_score = ?,
    is_3d = ?,
    stereo_mode = ?,
    has_dash = ?,
    dash_manifest_path = ?,
    transcoding_status = ?,
    date_modified = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: DeleteMedia :exec
DELETE FROM media
WHERE id = ?;

-- name: MediaExistsInLibrary :one
SELECT COUNT(*) FROM media
WHERE library_id = ? AND file_path = ?;

-- name: CountMediaInLibrary :one
SELECT COUNT(*) FROM media
WHERE library_id = ?;

-- name: CountMediaByType :one
SELECT COUNT(*) FROM media
WHERE library_id = ? AND type = ?;
