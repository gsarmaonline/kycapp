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

-- name: ListOrganisationsForUser :many
SELECT o.*
FROM organisations o
JOIN memberships m ON m.organisation_id = o.id
WHERE m.user_id = sqlc.arg('user_id')
  AND m.status IN ('active', 'invited')
  AND (sqlc.narg('status')::text IS NULL OR o.status = sqlc.narg('status'))
  AND (
    sqlc.narg('q')::text IS NULL
    OR o.name ILIKE '%' || sqlc.narg('q') || '%'
    OR o.slug ILIKE '%' || sqlc.narg('q') || '%'
  )
  AND (sqlc.narg('cursor')::text IS NULL OR o.id > sqlc.narg('cursor'))
ORDER BY o.id
LIMIT sqlc.arg('limit');

-- name: UpdateOrganisation :one
UPDATE organisations
SET
  name = COALESCE(sqlc.narg('name'), name),
  status = COALESCE(sqlc.narg('status'), status),
  primary_color = COALESCE(sqlc.narg('primary_color'), primary_color),
  accent_color = COALESCE(sqlc.narg('accent_color'), accent_color),
  email_footer = COALESCE(sqlc.narg('email_footer'), email_footer),
  email_font = COALESCE(sqlc.narg('email_font'), email_font),
  updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: SetOrganisationLogoURL :one
UPDATE organisations
SET logo_url = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ArchiveOrganisation :one
UPDATE organisations
SET status = 'archived', updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteOrganisation :exec
DELETE FROM organisations
WHERE id = $1;
