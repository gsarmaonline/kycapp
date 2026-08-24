-- Projection of the current authorisation model into edges.
--
-- Every statement is derived from a relationship that already exists, and every
-- one is ON CONFLICT DO NOTHING, so this is idempotent and safe to re-run. That
-- matters twice: it runs again at cutover to pick up anything written since,
-- and its correctness is the whole basis for trusting the new engine, which
-- means it has to be testable rather than buried in a schema migration.
--
-- The mapping it implements is declared in schema.go. TestProjectionMatchesTheTable
-- checks the two against each other.

-- A role_permissions row becomes one grant edge on the area it governs.
--
-- Whether the role sits in the platform organisation decides the object id, and
-- that is the whole of staff reach: such a role writes its edges on the star
-- node, so it holds exactly the actions it was granted in every tenant, present
-- and future. A read-only support role therefore stays read-only, which a
-- boolean flag on the principal could not express.
--
-- Global reach is derived here, never stored, exactly as ListUserGrantSources
-- derives it. The COALESCE keeps it failing closed: with no system_state row the
-- comparison is NULL and nobody gets reach.
INSERT INTO reach_edges (namespace, object_type, object_id, relation, subject_type, subject_id, subject_relation, source)
SELECT DISTINCT
    'kyc',
    CASE p.resource WHEN 'roles' THEN 'org_roles' ELSE p.resource END,
    CASE WHEN COALESCE(r.organisation_id = (SELECT platform_organisation_id FROM system_state WHERE id = 1), false) THEN '*' ELSE r.organisation_id END,
    'can_' || p.action,
    'role',
    r.id,
    'holder',
    'role_permissions'
FROM role_permissions rp
JOIN roles r ON r.id = rp.role_id
JOIN permissions p ON p.id = rp.permission_id
WHERE p.resource = ANY ($1::text[])
ON CONFLICT DO NOTHING;

-- Reaching an organisation at all is not a permission a role grants. It is what
-- holding any role there means, so it comes from the role rather than from
-- role_permissions.
INSERT INTO reach_edges (namespace, object_type, object_id, relation, subject_type, subject_id, subject_relation, source)
SELECT DISTINCT 'kyc', 'organisation', r.organisation_id, 'belongs', 'role', r.id, 'holder', 'roles'
FROM roles r
WHERE NOT COALESCE(r.organisation_id = (SELECT platform_organisation_id FROM system_state WHERE id = 1), false)
ON CONFLICT DO NOTHING;

-- A global role oversees every tenant. A different relation from belongs on
-- purpose: `member = belongs - suspended + oversees` puts it outside the
-- subtraction, so a suspended tenant stays visible to platform staff and
-- suspension does not become a one-way door into deletion.
INSERT INTO reach_edges (namespace, object_type, object_id, relation, subject_type, subject_id, subject_relation, source)
SELECT DISTINCT 'kyc', 'organisation', '*', 'oversees', 'role', r.id, 'holder', 'roles'
FROM roles r
WHERE COALESCE(r.organisation_id = (SELECT platform_organisation_id FROM system_state WHERE id = 1), false)
ON CONFLICT DO NOTHING;

-- Role inheritance. role_id extends parent_id, so whoever holds the child also
-- holds the parent: an admin picks up everything granted to member. Stored as a
-- userset rather than expanded, so the walk resolves the chain and editing a
-- base role reaches everything built on it with no recomputation.
INSERT INTO reach_edges (namespace, object_type, object_id, relation, subject_type, subject_id, subject_relation, source)
SELECT DISTINCT 'kyc', 'role', re.parent_id, 'holder', 'role', re.role_id, 'holder', 'role_extends'
FROM role_extends re
ON CONFLICT DO NOTHING;

-- A membership becomes a holder edge, carrying its expiry unchanged. Nothing
-- has to run on time for a time-boxed membership to lapse.
INSERT INTO reach_edges (namespace, object_type, object_id, relation, subject_type, subject_id, subject_relation, expires_at, source)
SELECT DISTINCT 'kyc', 'role', m.role_id, 'holder', 'user', m.user_id, '', m.expires_at, 'memberships'
FROM memberships m
WHERE m.status = 'active'
ON CONFLICT DO NOTHING;

-- Organisation status becomes edges. A suspended or archived tenant is hidden
-- from its own members, one edge per role that would otherwise reach it.
INSERT INTO reach_edges (namespace, object_type, object_id, relation, subject_type, subject_id, subject_relation, source)
SELECT DISTINCT 'kyc', 'organisation', o.id, 'suspended', 'role', r.id, 'holder', 'organisations'
FROM organisations o
JOIN roles r ON r.organisation_id = o.id
WHERE o.status <> 'active'
ON CONFLICT DO NOTHING;

-- An API key becomes an ordinary principal holding its own edges.
--
-- The current model recomputes a key's reach from its owner on every request
-- and narrows it by a scope list. That derivation is not carried forward: it is
-- projected once into the edges the key actually holds. What a key can do is
-- then readable rather than simulated, and revoking it is deleting rows.
--
-- Empty scopes keep their meaning at the moment of projection: everything the
-- owner could do. An organisation-scoped key stays inside its own organisation
-- whatever else its owner reaches, which is why the object id prefers
-- k.organisation_id.
INSERT INTO reach_edges (namespace, object_type, object_id, relation, subject_type, subject_id, subject_relation, source)
SELECT DISTINCT
    'kyc',
    CASE p.resource WHEN 'roles' THEN 'org_roles' ELSE p.resource END,
    COALESCE(k.organisation_id, CASE WHEN COALESCE(r.organisation_id = (SELECT platform_organisation_id FROM system_state WHERE id = 1), false) THEN '*' ELSE r.organisation_id END),
    'can_' || p.action,
    'key',
    k.id,
    '',
    'api_keys'
FROM api_keys k
JOIN memberships m ON m.user_id = k.user_id AND m.status = 'active'
JOIN roles r ON r.id = m.role_id
JOIN role_permissions rp ON rp.role_id = r.id
JOIN permissions p ON p.id = rp.permission_id
WHERE k.revoked_at IS NULL
  AND k.user_id IS NOT NULL
  AND (m.expires_at IS NULL OR m.expires_at > now())
  AND (cardinality(k.scopes) = 0 OR p.key = ANY (k.scopes))
  AND (k.organisation_id IS NULL OR COALESCE(r.organisation_id = (SELECT platform_organisation_id FROM system_state WHERE id = 1), false) OR r.organisation_id = k.organisation_id)
  AND p.resource = ANY ($1::text[])
ON CONFLICT DO NOTHING;

-- Ownership is lifecycle, not authority. It confers nothing, and exists so a
-- departing person's keys can be found and swept.
INSERT INTO reach_edges (namespace, object_type, object_id, relation, subject_type, subject_id, subject_relation, source)
SELECT DISTINCT 'kyc', 'key', k.id, 'owner', 'user', k.user_id, '', 'api_keys'
FROM api_keys k
WHERE k.user_id IS NOT NULL AND k.revoked_at IS NULL
ON CONFLICT DO NOTHING;
