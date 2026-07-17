-- name: CreateUser :one
INSERT INTO users (id, email, name, status)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetUser :one
SELECT * FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1;

-- name: ListUsers :many
SELECT * FROM users
WHERE (
    sqlc.narg('q')::text IS NULL
    OR name ILIKE '%' || sqlc.narg('q') || '%'
    OR email ILIKE '%' || sqlc.narg('q') || '%'
  )
  AND (sqlc.narg('cursor')::text IS NULL OR id > sqlc.narg('cursor'))
ORDER BY id
LIMIT sqlc.arg('limit');

-- name: UpdateUser :one
UPDATE users
SET
  name = COALESCE(sqlc.narg('name'), name),
  status = COALESCE(sqlc.narg('status'), status),
  updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING *;
