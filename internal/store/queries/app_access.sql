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
INSERT INTO app_capabilities (id, organisation_id, key, description, source)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- Applying a template twice must not fail, and must not relabel a capability
-- the merchant has since written by hand. DO NOTHING rather than an upsert:
-- a key that already exists is already declared, whoever declared it.
-- name: CreateAppCapabilityFromTemplate :exec
INSERT INTO app_capabilities (id, organisation_id, key, description, source)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (organisation_id, key) DO NOTHING;

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

-- Three inserts rather than one, because each subject kind conflicts against a
-- different partial index and a statement may name only one.
--
-- The upsert keeps re-granting idempotent: issuing the same role at the same
-- scope refreshes the expiry and the exceptions rather than failing.

-- name: CreateAppUserGrant :one
INSERT INTO app_grants (
    id, organisation_id, subject_kind, app_user_id, role_id, scope_kind, scope_id,
    expires_at, granted_by, all_capabilities, except_capabilities, except_scopes,
    except_app_user_ids, constraint_kind, all_scopes
) VALUES ($1, $2, 'app_user', $3, $4, $5, $6, $7, $8, $9, $10, $11, '{}', $12, $13)
ON CONFLICT (app_user_id, COALESCE(role_id, ''), scope_kind, scope_id) WHERE app_user_id IS NOT NULL
DO UPDATE SET expires_at = EXCLUDED.expires_at, granted_by = EXCLUDED.granted_by,
    except_capabilities = EXCLUDED.except_capabilities, except_scopes = EXCLUDED.except_scopes,
    constraint_kind = EXCLUDED.constraint_kind
RETURNING *;

-- name: CreateAppGroupGrant :one
INSERT INTO app_grants (
    id, organisation_id, subject_kind, group_id, role_id, scope_kind, scope_id,
    expires_at, granted_by, all_capabilities, except_capabilities, except_scopes,
    except_app_user_ids, constraint_kind, all_scopes
) VALUES ($1, $2, 'group', $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
ON CONFLICT (group_id, COALESCE(role_id, ''), scope_kind, scope_id) WHERE group_id IS NOT NULL
DO UPDATE SET expires_at = EXCLUDED.expires_at, granted_by = EXCLUDED.granted_by,
    except_capabilities = EXCLUDED.except_capabilities, except_scopes = EXCLUDED.except_scopes,
    except_app_user_ids = EXCLUDED.except_app_user_ids, constraint_kind = EXCLUDED.constraint_kind
RETURNING *;

-- CreateAppEveryoneGrant covers every customer of the organisation, present and
-- future, from one row. It names no subject because it names all of them.
-- name: CreateAppEveryoneGrant :one
INSERT INTO app_grants (
    id, organisation_id, subject_kind, role_id, scope_kind, scope_id,
    expires_at, granted_by, all_capabilities, except_capabilities, except_scopes,
    except_app_user_ids, constraint_kind, all_scopes
) VALUES ($1, $2, 'everyone', $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
ON CONFLICT (organisation_id, COALESCE(role_id, ''), scope_kind, scope_id) WHERE subject_kind = 'everyone'
DO UPDATE SET expires_at = EXCLUDED.expires_at, granted_by = EXCLUDED.granted_by,
    except_capabilities = EXCLUDED.except_capabilities, except_scopes = EXCLUDED.except_scopes,
    except_app_user_ids = EXCLUDED.except_app_user_ids, constraint_kind = EXCLUDED.constraint_kind
RETURNING *;

-- ListAppGrantsForUser returns a customer's live grants with the role's
-- materialised capabilities, so the caller never walks the inheritance graph.
--
-- Two branches unioned rather than one query with an OR. An OR spanning two
-- different access paths — a direct grant and a grant reached through group
-- membership — cannot use either index, so Postgres falls back to scanning
-- every grant in the table and discarding almost all of them. That cost grows
-- with the total across all merchants, on a path a merchant's backend calls
-- constantly. Measured at 41k grants: 16.5ms scanning, 0.125ms as a union.
--
-- UNION ALL, not UNION: a customer holding the same role at the same scope both
-- directly and through a group legitimately has two grants, and the evaluator
-- unions capabilities anyway.
-- Three branches now. The third carries the organisation's everyone-grants,
-- which name no subject and so cannot be reached by either id column.
--
-- LEFT JOIN on app_roles rather than JOIN: a wildcard grant carries no role,
-- and an inner join would silently drop exactly the grants that grant the most.
--
-- except_app_user_ids is applied here rather than in Go so an excluded customer
-- never has the row assembled for them at all.
-- name: ListAppGrantsForUser :many
SELECT g.id, g.scope_kind, g.scope_id, g.expires_at, g.role_id,
       COALESCE(r.key, '')::text AS role_key,
       COALESCE(r.effective_capabilities, '{}')::text[] AS effective_capabilities,
       g.group_id, ''::text AS group_key, 'app_user'::text AS subject_kind,
       g.all_capabilities, g.except_capabilities, g.except_scopes, g.constraint_kind, g.all_scopes
FROM app_grants g
LEFT JOIN app_roles r ON r.id = g.role_id
WHERE g.app_user_id = sqlc.arg('app_user_id')
  AND (g.expires_at IS NULL OR g.expires_at > now())
UNION ALL
SELECT DISTINCT g.id, g.scope_kind, g.scope_id, g.expires_at, g.role_id,
       COALESCE(r.key, '')::text AS role_key,
       COALESCE(r.effective_capabilities, '{}')::text[] AS effective_capabilities,
       g.group_id, grp.key AS group_key, 'group'::text AS subject_kind,
       g.all_capabilities, g.except_capabilities, g.except_scopes, g.constraint_kind, g.all_scopes
-- direct.effective_parent_ids is the group itself plus everything it extends,
-- flattened at write time. Joining through it is what makes a grant on a parent
-- group reach a member of a child, without walking anything on the read path.
FROM app_user_group_members m
JOIN app_user_groups direct ON direct.id = m.group_id
JOIN app_grants g ON g.group_id = ANY (direct.effective_parent_ids)
LEFT JOIN app_roles r ON r.id = g.role_id
JOIN app_user_groups grp ON grp.id = g.group_id
WHERE m.app_user_id = sqlc.arg('app_user_id')
  AND (g.expires_at IS NULL OR g.expires_at > now())
  AND NOT (sqlc.arg('app_user_id')::text = ANY (g.except_app_user_ids))
UNION ALL
SELECT g.id, g.scope_kind, g.scope_id, g.expires_at, g.role_id,
       COALESCE(r.key, '')::text AS role_key,
       COALESCE(r.effective_capabilities, '{}')::text[] AS effective_capabilities,
       g.group_id, ''::text AS group_key, 'everyone'::text AS subject_kind,
       g.all_capabilities, g.except_capabilities, g.except_scopes, g.constraint_kind, g.all_scopes
FROM app_grants g
LEFT JOIN app_roles r ON r.id = g.role_id
WHERE g.organisation_id = sqlc.arg('organisation_id')
  AND g.subject_kind = 'everyone'
  AND (g.expires_at IS NULL OR g.expires_at > now())
  AND NOT (sqlc.arg('app_user_id')::text = ANY (g.except_app_user_ids));

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
SELECT g.*,
       (SELECT COUNT(*) FROM app_user_group_members m WHERE m.group_id = g.id)::bigint AS member_count,
       (SELECT COUNT(*) FROM app_user_group_extends e WHERE e.group_id = g.id)::bigint AS parent_count
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

-- Group nesting, mirroring app_role_extends exactly. A role and a group are one
-- mechanism, so they get one shape: the child gains what the parent holds, and
-- what a member of the child effectively belongs to is expanded at write time.

-- name: ListAppUserGroupExtends :many
SELECT e.group_id, e.parent_id
FROM app_user_group_extends e
JOIN app_user_groups g ON g.id = e.group_id
WHERE g.organisation_id = $1;

-- name: ReplaceAppUserGroupExtends :exec
DELETE FROM app_user_group_extends WHERE group_id = $1;

-- name: AddAppUserGroupExtends :exec
INSERT INTO app_user_group_extends (group_id, parent_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: SetAppUserGroupEffectiveParents :exec
UPDATE app_user_groups SET effective_parent_ids = $2, updated_at = now() WHERE id = $1;

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
       COALESCE(r.key, '')::text AS role_key, COALESCE(grp.key, '')::text AS group_key,
       COALESCE(u.email, '')::text AS app_user_email, g.subject_kind,
       g.all_capabilities, g.except_capabilities, g.except_scopes,
       g.except_app_user_ids, g.constraint_kind, g.all_scopes
FROM app_grants g
LEFT JOIN app_roles r ON r.id = g.role_id
LEFT JOIN app_user_groups grp ON grp.id = g.group_id
LEFT JOIN app_users u ON u.id = g.app_user_id
WHERE g.organisation_id = $1
ORDER BY g.created_at DESC
LIMIT sqlc.arg('limit');

-- name: GetAppScopeType :one
SELECT * FROM app_scope_types WHERE id = $1;

-- name: UpdateAppScopeType :one
UPDATE app_scope_types
SET label = COALESCE(sqlc.narg('label'), label)
WHERE id = sqlc.arg('id') AND organisation_id = sqlc.arg('organisation_id')
RETURNING *;

-- name: GetAppCapability :one
SELECT * FROM app_capabilities WHERE id = $1;

-- name: UpdateAppCapability :one
UPDATE app_capabilities
SET description = COALESCE(sqlc.narg('description'), description)
WHERE id = sqlc.arg('id') AND organisation_id = sqlc.arg('organisation_id')
RETURNING *;

-- ListAppRoleParents returns the roles one role builds on, so an edit form can
-- show what is already selected.
-- name: ListAppRoleParents :many
SELECT parent_id FROM app_role_extends WHERE role_id = $1;

-- name: ListAppUserGroupParents :many
SELECT parent_id FROM app_user_group_extends WHERE group_id = $1 ORDER BY parent_id;

-- CountAppCapabilitiesByOrg backs the customer-access onboarding step.
--
-- Capabilities are the right thing to count. A scope kind on its own grants
-- nothing, a role with no capabilities carries nothing, and a grant cannot be
-- written until both exist. Declaring the first capability is the point where
-- the section starts to mean something.
-- name: CountAppCapabilitiesByOrg :one
SELECT COUNT(*)::bigint FROM app_capabilities WHERE organisation_id = $1;
