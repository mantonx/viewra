-- name: GetPersonByID :one
SELECT * FROM people WHERE id = ?;

-- name: GetPersonByName :one
SELECT * FROM people WHERE name = ? LIMIT 1;

-- name: GetPersonByTMDbID :one
SELECT * FROM people WHERE tmdb_id = ?;

-- name: GetPersonByIMDbID :one
SELECT * FROM people WHERE imdb_id = ?;

-- name: CreatePerson :one
INSERT INTO people (
    name, sort_name, photo_path, photo_url, imdb_id, tmdb_id
) VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdatePerson :exec
UPDATE people
SET name = ?,
    sort_name = ?,
    photo_path = ?,
    photo_url = ?,
    imdb_id = ?,
    tmdb_id = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: ListPeople :many
SELECT * FROM people
ORDER BY sort_name, name
LIMIT ? OFFSET ?;

-- name: SearchPeopleByName :many
SELECT * FROM people
WHERE name LIKE ?
ORDER BY sort_name, name
LIMIT ?;

-- name: DeletePerson :exec
DELETE FROM people WHERE id = ?;

-- name: GetCreditsForEntity :many
SELECT
    c.*,
    p.id as person_id,
    p.name as person_name,
    p.sort_name as person_sort_name,
    p.photo_path as person_photo_path,
    p.photo_url as person_photo_url,
    p.imdb_id as person_imdb_id,
    p.tmdb_id as person_tmdb_id
FROM credits c
JOIN people p ON c.person_id = p.id
WHERE c.media_type = ? AND c.entity_id = ?
ORDER BY c.credit_type, c.billing_order, p.name;

-- name: GetCreditsForEntityByType :many
SELECT
    c.*,
    p.id as person_id,
    p.name as person_name,
    p.sort_name as person_sort_name,
    p.photo_path as person_photo_path,
    p.photo_url as person_photo_url,
    p.imdb_id as person_imdb_id,
    p.tmdb_id as person_tmdb_id
FROM credits c
JOIN people p ON c.person_id = p.id
WHERE c.media_type = ? AND c.entity_id = ? AND c.credit_type = ?
ORDER BY c.billing_order, p.name;

-- name: GetCreditsForPerson :many
SELECT
    c.*,
    p.id as person_id,
    p.name as person_name,
    p.sort_name as person_sort_name,
    p.photo_path as person_photo_path,
    p.photo_url as person_photo_url,
    p.imdb_id as person_imdb_id,
    p.tmdb_id as person_tmdb_id
FROM credits c
JOIN people p ON c.person_id = p.id
WHERE c.person_id = ?
ORDER BY c.media_type, c.entity_id;

-- name: CreateCredit :one
INSERT INTO credits (
    person_id, media_type, entity_id, credit_type,
    character_name, department, job, billing_order
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: DeleteCreditsForEntity :exec
DELETE FROM credits
WHERE media_type = ? AND entity_id = ?;

-- name: DeleteCreditsForEntityByType :exec
DELETE FROM credits
WHERE media_type = ? AND entity_id = ? AND credit_type = ?;

-- name: DeleteCredit :exec
DELETE FROM credits WHERE id = ?;

-- name: CountCreditsForEntity :one
SELECT COUNT(*) FROM credits
WHERE media_type = ? AND entity_id = ?;

-- name: GetCastForEntity :many
SELECT
    c.*,
    p.id as person_id,
    p.name as person_name,
    p.sort_name as person_sort_name,
    p.photo_path as person_photo_path,
    p.photo_url as person_photo_url,
    p.imdb_id as person_imdb_id,
    p.tmdb_id as person_tmdb_id
FROM credits c
JOIN people p ON c.person_id = p.id
WHERE c.media_type = ? AND c.entity_id = ? AND c.credit_type = 'cast'
ORDER BY c.billing_order, p.name
LIMIT ?;

-- name: GetDirectorsForEntity :many
SELECT
    c.*,
    p.id as person_id,
    p.name as person_name,
    p.sort_name as person_sort_name,
    p.photo_path as person_photo_path,
    p.photo_url as person_photo_url,
    p.imdb_id as person_imdb_id,
    p.tmdb_id as person_tmdb_id
FROM credits c
JOIN people p ON c.person_id = p.id
WHERE c.media_type = ? AND c.entity_id = ? AND c.credit_type = 'director'
ORDER BY c.billing_order, p.name;

-- name: GetWritersForEntity :many
SELECT
    c.*,
    p.id as person_id,
    p.name as person_name,
    p.sort_name as person_sort_name,
    p.photo_path as person_photo_path,
    p.photo_url as person_photo_url,
    p.imdb_id as person_imdb_id,
    p.tmdb_id as person_tmdb_id
FROM credits c
JOIN people p ON c.person_id = p.id
WHERE c.media_type = ? AND c.entity_id = ? AND c.credit_type = 'writer'
ORDER BY c.billing_order, p.name;

-- name: GetCreatorsForEntity :many
SELECT
    c.*,
    p.id as person_id,
    p.name as person_name,
    p.sort_name as person_sort_name,
    p.photo_path as person_photo_path,
    p.photo_url as person_photo_url,
    p.imdb_id as person_imdb_id,
    p.tmdb_id as person_tmdb_id
FROM credits c
JOIN people p ON c.person_id = p.id
WHERE c.media_type = ? AND c.entity_id = ? AND c.credit_type = 'creator'
ORDER BY c.billing_order, p.name;

-- name: GetCrewByJob :many
-- Fetches crew members by department and job (e.g., Sound/Original Music Composer, Camera/Director of Photography)
SELECT
    c.*,
    p.id as person_id,
    p.name as person_name,
    p.sort_name as person_sort_name,
    p.photo_path as person_photo_path,
    p.photo_url as person_photo_url,
    p.imdb_id as person_imdb_id,
    p.tmdb_id as person_tmdb_id
FROM credits c
JOIN people p ON c.person_id = p.id
WHERE c.media_type = ? AND c.entity_id = ? AND c.department = ? AND c.job = ?
ORDER BY c.billing_order, p.name;
