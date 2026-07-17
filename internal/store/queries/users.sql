-- name: CreateUser :one
INSERT INTO users (id, email, name, status, platform_admin, google_sub)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetUser :one
SELECT * FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1;

-- name: GetUserByGoogleSub :one
SELECT * FROM users
WHERE google_sub = $1;

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
  platform_admin = COALESCE(sqlc.narg('platform_admin'), platform_admin),
  google_sub = COALESCE(sqlc.narg('google_sub'), google_sub),
  updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING *;
