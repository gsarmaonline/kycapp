-- name: CreateAppScopeType :one
INSERT INTO app_scope_types (id, organisation_id, kind, label)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListAppScopeTypes :many
SELECT * FROM app_scope_types
WHERE organisation_id = $1
ORDER BY kind;

-- name: DeleteAppScopeType :exec
DELETE FROM app_scope_types WHERE id = $1 AND organisation_id = $2;

-- name: CreateAppCapability :one
INSERT INTO app_capabilities (id, organisation_id, key, description)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListAppCapabilities :many
SELECT * FROM app_capabilities
WHERE organisation_id = $1
ORDER BY key;

-- name: DeleteAppCapability :exec
DELETE FROM app_capabilities WHERE id = $1 AND organisation_id = $2;

-- name: CreateAppRole :one
INSERT INTO app_roles (id, organisation_id, key, name, description, own_capabilities)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateAppRole :one
UPDATE app_roles
SET name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    own_capabilities = COALESCE(sqlc.narg('own_capabilities'), own_capabilities),
    updated_at = now()
WHERE id = sqlc.arg('id') AND organisation_id = sqlc.arg('organisation_id')
RETURNING *;

-- name: SetAppRoleEffectiveCapabilities :exec
UPDATE app_roles SET effective_capabilities = $2, updated_at = now() WHERE id = $1;

-- name: ListAppRoles :many
SELECT * FROM app_roles WHERE organisation_id = $1 ORDER BY key;

-- name: GetAppRole :one
SELECT * FROM app_roles WHERE id = $1;

-- name: DeleteAppRole :exec
DELETE FROM app_roles WHERE id = $1 AND organisation_id = $2;

-- name: ListAppRoleExtends :many
SELECT e.role_id, e.parent_id
FROM app_role_extends e
JOIN app_roles r ON r.id = e.role_id
WHERE r.organisation_id = $1;

-- name: ReplaceAppRoleExtends :exec
DELETE FROM app_role_extends WHERE role_id = $1;

-- name: AddAppRoleExtends :exec
INSERT INTO app_role_extends (role_id, parent_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: CreateAppUserGrant :one
INSERT INTO app_grants (id, organisation_id, app_user_id, role_id, scope_kind, scope_id, expires_at, granted_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (app_user_id, role_id, scope_kind, scope_id) WHERE app_user_id IS NOT NULL
DO UPDATE SET expires_at = EXCLUDED.expires_at, granted_by = EXCLUDED.granted_by
RETURNING *;

-- name: CreateAppGroupGrant :one
INSERT INTO app_grants (id, organisation_id, group_id, role_id, scope_kind, scope_id, expires_at, granted_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (group_id, role_id, scope_kind, scope_id) WHERE group_id IS NOT NULL
DO UPDATE SET expires_at = EXCLUDED.expires_at, granted_by = EXCLUDED.granted_by
RETURNING *;

-- ListAppGrantsForUser returns a customer's live grants with the role's
-- materialised capabilities, so the caller never walks the inheritance graph.
--
-- Both subjects in one query: grants made directly to the user, and grants made
-- to any group they belong to. The result is a union, which is why a group can
-- only ever add access.
-- name: ListAppGrantsForUser :many
SELECT g.id, g.scope_kind, g.scope_id, g.expires_at, g.role_id,
       r.key AS role_key, r.effective_capabilities,
       g.group_id, COALESCE(grp.key, '')::text AS group_key
FROM app_grants g
JOIN app_roles r ON r.id = g.role_id
LEFT JOIN app_user_groups grp ON grp.id = g.group_id
LEFT JOIN app_user_group_members m
       ON m.group_id = g.group_id AND m.app_user_id = sqlc.arg('app_user_id')
WHERE (g.expires_at IS NULL OR g.expires_at > now())
  AND (g.app_user_id = sqlc.arg('app_user_id') OR m.app_user_id IS NOT NULL)
ORDER BY g.scope_kind, g.scope_id, r.key;

-- name: DeleteAppGrant :exec
DELETE FROM app_grants WHERE id = $1 AND organisation_id = $2;

-- AppAccessVersion changes whenever anything a customer's grant set depends on
-- changes, so a merchant can cache by version instead of polling.
-- AppAccessVersion changes whenever anything a customer's grant set depends on
-- changes, so a merchant can cache by version instead of polling.
--
-- Grants are taken organisation-wide rather than per user, because a grant made
-- to a group affects members without touching their rows. Over-invalidating is
-- the safe direction: a stale cache silently serves the wrong permissions.
-- name: AppAccessVersion :one
SELECT COALESCE(EXTRACT(EPOCH FROM GREATEST(
    (SELECT MAX(g.created_at) FROM app_grants g WHERE g.organisation_id = sqlc.arg('organisation_id')),
    (SELECT MAX(r.updated_at) FROM app_roles r WHERE r.organisation_id = sqlc.arg('organisation_id')),
    (SELECT MAX(m.created_at) FROM app_user_group_members m WHERE m.app_user_id = sqlc.arg('app_user_id'))
))::bigint, 0)::bigint AS version;

-- name: CreateAppUserGroup :one
INSERT INTO app_user_groups (id, organisation_id, key, name, description)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListAppUserGroups :many
SELECT g.*, (SELECT COUNT(*) FROM app_user_group_members m WHERE m.group_id = g.id)::bigint AS member_count
FROM app_user_groups g
WHERE g.organisation_id = $1
ORDER BY g.key;

-- name: GetAppUserGroup :one
SELECT * FROM app_user_groups WHERE id = $1;

-- name: UpdateAppUserGroup :one
UPDATE app_user_groups
SET name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    updated_at = now()
WHERE id = sqlc.arg('id') AND organisation_id = sqlc.arg('organisation_id')
RETURNING *;

-- name: DeleteAppUserGroup :exec
DELETE FROM app_user_groups WHERE id = $1 AND organisation_id = $2;

-- name: AddAppUserToGroup :exec
INSERT INTO app_user_group_members (group_id, app_user_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: RemoveAppUserFromGroup :exec
DELETE FROM app_user_group_members WHERE group_id = $1 AND app_user_id = $2;

-- name: ListAppUserGroupMembers :many
SELECT u.id, u.email, u.display_name, u.status
FROM app_user_group_members m
JOIN app_users u ON u.id = m.app_user_id
WHERE m.group_id = $1
ORDER BY u.email;

-- name: ListGroupsForAppUser :many
SELECT g.id, g.key, g.name
FROM app_user_group_members m
JOIN app_user_groups g ON g.id = m.group_id
WHERE m.app_user_id = $1
ORDER BY g.key;

-- name: ListAppGrantsForOrg :many
SELECT g.id, g.scope_kind, g.scope_id, g.expires_at, g.app_user_id, g.group_id,
       r.key AS role_key, COALESCE(grp.key, '')::text AS group_key,
       COALESCE(u.email, '')::text AS app_user_email
FROM app_grants g
JOIN app_roles r ON r.id = g.role_id
LEFT JOIN app_user_groups grp ON grp.id = g.group_id
LEFT JOIN app_users u ON u.id = g.app_user_id
WHERE g.organisation_id = $1
ORDER BY g.created_at DESC
LIMIT sqlc.arg('limit');
