-- name: CreateAttributeDefinition :one
INSERT INTO attribute_definitions (
    id, organisation_id, key, label, description, value_type,
    section, sort_order, required, enum_values, is_pii, status, is_system
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11, $12, $13
)
RETURNING *;

-- name: GetAttributeDefinition :one
SELECT * FROM attribute_definitions WHERE id = $1;

-- name: GetAttributeDefinitionByOrgKey :one
SELECT * FROM attribute_definitions
WHERE organisation_id = $1 AND key = $2;

-- name: ListAttributeDefinitions :many
SELECT * FROM attribute_definitions
WHERE organisation_id = $1
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
ORDER BY section ASC, sort_order ASC, key ASC;

-- name: UpdateAttributeDefinition :one
UPDATE attribute_definitions SET
    label = COALESCE(sqlc.narg('label'), label),
    description = COALESCE(sqlc.narg('description'), description),
    value_type = COALESCE(sqlc.narg('value_type'), value_type),
    section = COALESCE(sqlc.narg('section'), section),
    sort_order = COALESCE(sqlc.narg('sort_order'), sort_order),
    required = COALESCE(sqlc.narg('required'), required),
    enum_values = COALESCE(sqlc.narg('enum_values'), enum_values),
    is_pii = COALESCE(sqlc.narg('is_pii'), is_pii),
    status = COALESCE(sqlc.narg('status'), status),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CreateAppUser :one
INSERT INTO app_users (
    id, organisation_id, external_id, email, display_name, status, attributes
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: GetAppUser :one
SELECT * FROM app_users WHERE id = $1;

-- name: ListAppUsers :many
SELECT * FROM app_users
WHERE organisation_id = $1
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
ORDER BY created_at DESC;

-- name: UpdateAppUser :one
UPDATE app_users SET
    external_id = COALESCE(sqlc.narg('external_id'), external_id),
    email = COALESCE(sqlc.narg('email'), email),
    display_name = COALESCE(sqlc.narg('display_name'), display_name),
    status = COALESCE(sqlc.narg('status'), status),
    attributes = COALESCE(sqlc.narg('attributes'), attributes),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ArchiveAppUser :one
UPDATE app_users
SET status = 'archived', updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ArchiveAttributeDefinition :one
UPDATE attribute_definitions
SET status = 'archived', updated_at = now()
WHERE id = $1
  AND is_system = false
RETURNING *;
