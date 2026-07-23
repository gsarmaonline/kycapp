-- name: CreateRole :one
INSERT INTO roles (id, organisation_id, key, name, description, is_system)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetRole :one
SELECT * FROM roles
WHERE id = $1;

-- name: GetRoleByOrgAndKey :one
SELECT * FROM roles
WHERE organisation_id = $1 AND key = $2;

-- name: ListRolesByOrganisation :many
SELECT * FROM roles
WHERE organisation_id = $1
ORDER BY key;

-- name: UpdateRole :one
UPDATE roles
SET
  name = COALESCE(sqlc.narg('name'), name),
  description = COALESCE(sqlc.narg('description'), description)
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: DeleteRole :exec
DELETE FROM roles
WHERE id = $1 AND is_system = false;

-- name: AddRolePermission :exec
INSERT INTO role_permissions (role_id, permission_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: DeleteRolePermissions :exec
DELETE FROM role_permissions
WHERE role_id = $1;

-- name: ListPermissionKeysByRole :many
SELECT p.key
FROM role_permissions rp
JOIN permissions p ON p.id = rp.permission_id
WHERE rp.role_id = $1
ORDER BY p.key;

-- name: ListPermissions :many
SELECT * FROM permissions
ORDER BY category, key;

-- name: ListPermissionsFiltered :many
SELECT * FROM permissions
WHERE (sqlc.narg('category')::text IS NULL OR category = sqlc.narg('category'))
  AND (sqlc.narg('resource')::text IS NULL OR resource = sqlc.narg('resource'))
ORDER BY category, key;

-- name: GetPermissionByKey :one
SELECT * FROM permissions
WHERE key = $1;

-- name: GetPermissionByResourceAction :one
SELECT * FROM permissions
WHERE resource = $1 AND action = $2;

-- name: ListPermissionIDs :many
SELECT id FROM permissions
ORDER BY key;

-- name: ListPermissionIDsByKeys :many
SELECT id, key FROM permissions
WHERE key = ANY(sqlc.arg('keys')::text[])
ORDER BY key;

-- name: CheckUserPermission :one
SELECT EXISTS (
    SELECT 1
    FROM memberships m
    JOIN role_permissions rp ON rp.role_id = m.role_id
    JOIN permissions p ON p.id = rp.permission_id
    WHERE m.organisation_id = sqlc.arg('organisation_id')
      AND m.user_id = sqlc.arg('user_id')
      AND m.status = 'active'
      AND p.key = sqlc.arg('permission_key')
) AS allowed;
