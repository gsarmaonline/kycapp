-- name: CreateOrganisation :one
INSERT INTO organisations (id, name, slug, status)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetOrganisation :one
SELECT * FROM organisations
WHERE id = $1;

-- name: ListOrganisations :many
SELECT * FROM organisations
WHERE (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (
    sqlc.narg('q')::text IS NULL
    OR name ILIKE '%' || sqlc.narg('q') || '%'
    OR slug ILIKE '%' || sqlc.narg('q') || '%'
  )
  AND (sqlc.narg('cursor')::text IS NULL OR id > sqlc.narg('cursor'))
ORDER BY id
LIMIT sqlc.arg('limit');

-- name: UpdateOrganisation :one
UPDATE organisations
SET
  name = COALESCE(sqlc.narg('name'), name),
  status = COALESCE(sqlc.narg('status'), status),
  updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: ArchiveOrganisation :one
UPDATE organisations
SET status = 'archived', updated_at = now()
WHERE id = $1
RETURNING *;
