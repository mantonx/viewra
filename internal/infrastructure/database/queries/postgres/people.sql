-- name: GetPersonByID :one
SELECT * FROM people WHERE id = $1;

-- name: GetPersonByName :one
SELECT * FROM people WHERE name = $1 LIMIT 1;

-- name: GetPersonByTMDbID :one
SELECT * FROM people WHERE tmdb_id = $1;

-- name: GetPersonByIMDbID :one
SELECT * FROM people WHERE imdb_id = $1;

-- name: CreatePerson :one
INSERT INTO people (
    name, sort_name, photo_path, photo_url, imdb_id, tmdb_id
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdatePerson :exec
UPDATE people
SET name = $1,
    sort_name = $2,
    photo_path = $3,
    photo_url = $4,
    imdb_id = $5,
    tmdb_id = $6,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $7;

-- name: ListPeople :many
SELECT * FROM people
ORDER BY sort_name, name
LIMIT sqlc.arg('limit')::bigint OFFSET sqlc.arg('offset')::bigint;

-- name: SearchPeopleByName :many
SELECT * FROM people
WHERE name ILIKE $1
ORDER BY sort_name, name
LIMIT sqlc.arg('limit')::bigint;

-- name: DeletePerson :exec
DELETE FROM people WHERE id = $1;

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
WHERE c.media_type = $1 AND c.entity_id = $2
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
WHERE c.media_type = $1 AND c.entity_id = $2 AND c.credit_type = $3
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
WHERE c.person_id = $1
ORDER BY c.media_type, c.entity_id;

-- name: CreateCredit :one
INSERT INTO credits (
    person_id, media_type, entity_id, credit_type,
    character_name, department, job, billing_order
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: DeleteCreditsForEntity :exec
DELETE FROM credits
WHERE media_type = $1 AND entity_id = $2;

-- name: DeleteCreditsForEntityByType :exec
DELETE FROM credits
WHERE media_type = $1 AND entity_id = $2 AND credit_type = $3;

-- name: DeleteCredit :exec
DELETE FROM credits WHERE id = $1;

-- name: CountCreditsForEntity :one
SELECT COUNT(*) FROM credits
WHERE media_type = $1 AND entity_id = $2;

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
WHERE c.media_type = $1 AND c.entity_id = $2 AND c.credit_type = 'cast'
ORDER BY c.billing_order, p.name
LIMIT sqlc.arg('limit')::bigint;

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
WHERE c.media_type = $1 AND c.entity_id = $2 AND c.credit_type = 'director'
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
WHERE c.media_type = $1 AND c.entity_id = $2 AND c.credit_type = 'writer'
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
WHERE c.media_type = $1 AND c.entity_id = $2 AND c.credit_type = 'creator'
ORDER BY c.billing_order, p.name;
