-- Media queries for PostgreSQL

-- name: CreateMedia :one
INSERT INTO media (
    library_id, title, file_path, file_size, file_hash,
    container_format, duration, width, height, aspect_ratio,
    codec, audio_codec, codec_profile, bit_rate, frame_rate, scan_type, hdr_format,
    color_space, color_primaries, thumbnail_path, type, source_type,
    resolution_label, quality_score, is_3d, stereo_mode, has_dash,
    dash_manifest_path, transcoding_status, is_extra
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10,
    $11, $12, $13, $14, $15, $16, $17,
    $18, $19, $20, $21, $22,
    $23, $24, $25, $26, $27,
    $28, $29, $30
) RETURNING *;

-- name: GetMediaByID :one
SELECT * FROM media
WHERE id = $1;

-- name: GetMediaByFilePath :one
SELECT * FROM media
WHERE library_id = $1 AND file_path = $2;

-- name: ListAllMedia :many
SELECT * FROM media
ORDER BY title;

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
    audio_codec = $12,
    codec_profile = $13,
    bit_rate = $14,
    frame_rate = $15,
    scan_type = $16,
    hdr_format = $17,
    color_space = $18,
    color_primaries = $19,
    thumbnail_path = $20,
    type = $21,
    source_type = $22,
    resolution_label = $23,
    quality_score = $24,
    is_3d = $25,
    stereo_mode = $26,
    has_dash = $27,
    dash_manifest_path = $28,
    transcoding_status = $29,
    is_extra = $30,
    date_modified = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $31
RETURNING *;

-- name: DeleteMedia :exec
DELETE FROM media
WHERE id = $1;

-- name: MediaExistsInLibrary :one
SELECT COUNT(*) FROM media
WHERE library_id = $1 AND file_path = $2;

-- name: CountMediaInLibrary :one
SELECT COUNT(*) FROM media
WHERE library_id = $1;

-- name: CountMediaByType :one
SELECT COUNT(*) FROM media
WHERE library_id = $1 AND type = $2;

-- name: GetFilePathCache :many
SELECT id, file_path FROM media
WHERE library_id = $1;
