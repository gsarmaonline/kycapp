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

-- name: AddRolePermission :exec
INSERT INTO role_permissions (role_id, permission_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: ListPermissions :many
SELECT * FROM permissions
ORDER BY category, key;

-- name: ListPermissionIDs :many
SELECT id FROM permissions
ORDER BY key;
