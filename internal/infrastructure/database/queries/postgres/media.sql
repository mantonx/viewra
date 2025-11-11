-- Media queries for PostgreSQL

-- name: CreateMedia :one
INSERT INTO media (
    library_id, title, file_path, file_size, file_hash,
    container_format, duration, width, height, aspect_ratio,
    codec, codec_profile, bit_rate, frame_rate, scan_type, hdr_format,
    color_space, color_primaries, thumbnail_path, type, source_type,
    resolution_label, quality_score, is_3d, stereo_mode, has_dash,
    dash_manifest_path, transcoding_status
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10,
    $11, $12, $13, $14, $15, $16,
    $17, $18, $19, $20, $21,
    $22, $23, $24, $25, $26,
    $27, $28
) RETURNING *;

-- name: GetMediaByID :one
SELECT * FROM media
WHERE id = $1;

-- name: GetMediaByFilePath :one
SELECT * FROM media
WHERE library_id = $1 AND file_path = $2;

-- name: ListMediaByLibrary :many
SELECT * FROM media
WHERE library_id = $1
ORDER BY title;

-- name: ListMediaByType :many
SELECT * FROM media
WHERE library_id = $1 AND type = $2
ORDER BY title;

-- name: UpdateMedia :one
UPDATE media
SET library_id = $1,
    title = $2,
    file_path = $3,
    file_size = $4,
    file_hash = $5,
    container_format = $6,
    duration = $7,
    width = $8,
    height = $9,
    aspect_ratio = $10,
    codec = $11,
    codec_profile = $12,
    bit_rate = $13,
    frame_rate = $14,
    scan_type = $15,
    hdr_format = $16,
    color_space = $17,
    color_primaries = $18,
    thumbnail_path = $19,
    type = $20,
    source_type = $21,
    resolution_label = $22,
    quality_score = $23,
    is_3d = $24,
    stereo_mode = $25,
    has_dash = $26,
    dash_manifest_path = $27,
    transcoding_status = $28,
    date_modified = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $29
RETURNING *;

-- name: DeleteMedia :exec
DELETE FROM media
WHERE id = $1;

-- name: MediaExistsInLibrary :one
SELECT COUNT(*) > 0 as exists FROM media
WHERE library_id = $1 AND file_path = $2;

-- name: CountMediaInLibrary :one
SELECT COUNT(*) FROM media
WHERE library_id = $1;

-- name: CountMediaByType :one
SELECT COUNT(*) FROM media
WHERE library_id = $1 AND type = $2;
