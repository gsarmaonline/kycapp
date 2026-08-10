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


-- ListUserGrantSources returns everything a user's memberships confer that is
-- relevant to one organisation: that organisation's own membership, plus any
-- membership of the platform organisation, which is what makes someone staff.
--
-- Global reach is derived here, never stored. A role in a merchant organisation
-- can therefore never produce one, whatever anybody sets on it.
--
-- COALESCE keeps this failing closed: with no system_state row the comparison
-- is NULL, and nobody gets reach.
--
-- LEFT JOIN on purpose: a membership with a permissionless role still confers
-- reach, and dropping it would make such a member invisible rather than
-- powerless.
-- name: ListUserGrantSources :many
SELECT
    m.organisation_id,
    COALESCE(m.organisation_id = (SELECT platform_organisation_id FROM system_state WHERE id = 1), false)::boolean AS global_reach,
    p.key AS permission_key
FROM memberships m
LEFT JOIN role_permissions rp ON rp.role_id = m.role_id
LEFT JOIN permissions p ON p.id = rp.permission_id
WHERE m.user_id = sqlc.arg('user_id')
  AND m.status = 'active'
  AND (m.expires_at IS NULL OR m.expires_at > now())
  AND (
        m.organisation_id = sqlc.arg('organisation_id')
     OR m.organisation_id = (SELECT platform_organisation_id FROM system_state WHERE id = 1)
  )
ORDER BY m.organisation_id;

-- ListUserGlobalReach returns the user's live memberships of the platform
-- organisation. Used to report staff status without naming any role.
-- name: ListUserGlobalReach :many
SELECT m.id
FROM memberships m
WHERE m.user_id = sqlc.arg('user_id')
  AND m.status = 'active'
  AND (m.expires_at IS NULL OR m.expires_at > now())
  AND m.organisation_id = (SELECT platform_organisation_id FROM system_state WHERE id = 1);
