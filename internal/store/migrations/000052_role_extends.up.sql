-- Role inheritance for KYC's own operators.
--
-- owner, admin and member read like a hierarchy and were never one. All three
-- are flat sets that happen to be seeded to overlap: owner and admin each get
-- every permission in the catalog, member gets none. Nothing linked them, so
-- narrowing admin left member untouched and a permission granted to member did
-- not reach an admin who obviously should have it.
--
-- Merchants have had this since app_role_extends. Their operators had not.
--
-- role_id extends parent_id, the same direction app_role_extends uses: the
-- child gains what the parent holds. Multiple parents are allowed, because
-- reachability only ever adds, so a diamond resolves the same way whatever
-- order it is walked.

CREATE TABLE IF NOT EXISTS role_extends (
    role_id   TEXT NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    parent_id TEXT NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, parent_id),
    -- A role extending itself would be inert rather than wrong, but it reads as
    -- though it says something. Depth beyond this is caught by the walk's
    -- visited set; this catches the case a constraint can.
    CHECK (role_id <> parent_id)
);

CREATE INDEX IF NOT EXISTS role_extends_parent_idx ON role_extends (parent_id);

-- Seed the chain the seeded roles always implied: owner extends admin extends
-- member. Within one organisation only, so a role never inherits across the
-- tenancy boundary.
INSERT INTO role_extends (role_id, parent_id)
SELECT child.id, parent.id
FROM roles child
JOIN roles parent ON parent.organisation_id = child.organisation_id
WHERE (child.key = 'admin' AND parent.key = 'member')
   OR (child.key = 'owner' AND parent.key = 'admin')
ON CONFLICT DO NOTHING;

-- Redefine the live view to read the new table. It cannot be done in 000051,
-- which runs before this table exists; the column list is unchanged, so
-- CREATE OR REPLACE is enough.

CREATE OR REPLACE VIEW reach_edges_live AS

-- A role_permissions row is a grant edge on the area it governs. A role in the
-- platform organisation writes its edges on the star node instead, which is the
-- whole of staff reach: exactly the actions granted, in every tenant.
SELECT
    'kyc'::text AS namespace,
    (CASE p.resource WHEN 'roles' THEN 'org_roles' ELSE p.resource END)::text AS object_type,
    (CASE WHEN COALESCE(r.organisation_id = (SELECT platform_organisation_id FROM system_state WHERE id = 1), false)
          THEN '*' ELSE r.organisation_id END)::text AS object_id,
    ('can_' || p.action)::text AS relation,
    'role'::text AS subject_type,
    r.id::text AS subject_id,
    'holder'::text AS subject_relation,
    NULL::timestamptz AS expires_at,
    'role_permissions'::text AS source
FROM role_permissions rp
JOIN roles r ON r.id = rp.role_id
JOIN permissions p ON p.id = rp.permission_id
WHERE p.resource IN (
    'organisation', 'members', 'roles', 'api_keys', 'app_users', 'app_access',
    'attributes', 'automations', 'billing', 'email_templates',
    'product_features', 'activity', 'usage')

UNION ALL

-- Reaching an organisation at all is what holding any role there means. It is
-- not a permission a role grants, so it comes from the role itself.
SELECT 'kyc', 'organisation', r.organisation_id, 'belongs', 'role', r.id, 'holder', NULL::timestamptz, 'roles'
FROM roles r
WHERE NOT COALESCE(r.organisation_id = (SELECT platform_organisation_id FROM system_state WHERE id = 1), false)

UNION ALL

-- A platform role oversees every tenant. A different relation from belongs on
-- purpose: member = belongs - suspended + oversees puts it outside the
-- subtraction, so a suspended tenant stays visible to staff and suspension does
-- not become a one-way door into deletion.
SELECT 'kyc', 'organisation', '*', 'oversees', 'role', r.id, 'holder', NULL::timestamptz, 'roles'
FROM roles r
WHERE COALESCE(r.organisation_id = (SELECT platform_organisation_id FROM system_state WHERE id = 1), false)

UNION ALL

-- Role inheritance. role_id extends parent_id, so whoever holds the child also
-- holds the parent: an admin picks up everything granted to member. Written as
-- a userset rather than expanded, so the walk resolves the chain and a deep one
-- costs nothing at write time.
SELECT 'kyc', 'role', re.parent_id, 'holder', 'role', re.role_id, 'holder', NULL::timestamptz, 'role_extends'
FROM role_extends re

UNION ALL

-- A membership is a holder edge, carrying its expiry unchanged.
SELECT 'kyc', 'role', m.role_id, 'holder', 'user', m.user_id, '', m.expires_at, 'memberships'
FROM memberships m
WHERE m.status = 'active'

UNION ALL

-- Organisation status is edges. A suspended or archived tenant is hidden from
-- its own members, one edge per role that would otherwise reach it.
SELECT 'kyc', 'organisation', o.id, 'suspended', 'role', r.id, 'holder', NULL::timestamptz, 'organisations'
FROM organisations o
JOIN roles r ON r.organisation_id = o.id
WHERE o.status <> 'active'

UNION ALL

-- An API key is an ordinary principal holding its own edges. Its scope list
-- narrows what its owner reaches, which is where the current model applies it
-- too; the difference is that this is readable rather than recomputed.
SELECT
    'kyc',
    (CASE p.resource WHEN 'roles' THEN 'org_roles' ELSE p.resource END)::text,
    COALESCE(k.organisation_id,
             CASE WHEN COALESCE(r.organisation_id = (SELECT platform_organisation_id FROM system_state WHERE id = 1), false)
                  THEN '*' ELSE r.organisation_id END)::text,
    ('can_' || p.action)::text,
    'key', k.id, '', NULL::timestamptz, 'api_keys'
FROM api_keys k
JOIN memberships m ON m.user_id = k.user_id AND m.status = 'active'
JOIN roles r ON r.id = m.role_id
JOIN role_permissions rp ON rp.role_id = r.id
JOIN permissions p ON p.id = rp.permission_id
WHERE k.revoked_at IS NULL
  AND k.user_id IS NOT NULL
  AND (m.expires_at IS NULL OR m.expires_at > now())
  AND (cardinality(k.scopes) = 0 OR p.key = ANY (k.scopes))
  AND (k.organisation_id IS NULL
       OR COALESCE(r.organisation_id = (SELECT platform_organisation_id FROM system_state WHERE id = 1), false)
       OR r.organisation_id = k.organisation_id)
  AND p.resource IN (
    'organisation', 'members', 'roles', 'api_keys', 'app_users', 'app_access',
    'attributes', 'automations', 'billing', 'email_templates',
    'product_features', 'activity', 'usage')

UNION ALL

-- A key reaches the organisations its owner reaches. Without this it would hold
-- permissions in a tenant while being unable to see the tenant at all, and
-- every gate would answer 404 before ever asking about the permission.
SELECT DISTINCT 'kyc', 'organisation',
    COALESCE(k.organisation_id, r.organisation_id)::text,
    'belongs', 'key', k.id, '', NULL::timestamptz, 'api_keys'
FROM api_keys k
JOIN memberships m ON m.user_id = k.user_id AND m.status = 'active'
JOIN roles r ON r.id = m.role_id
WHERE k.revoked_at IS NULL
  AND k.user_id IS NOT NULL
  AND (m.expires_at IS NULL OR m.expires_at > now())
  AND NOT COALESCE(r.organisation_id = (SELECT platform_organisation_id FROM system_state WHERE id = 1), false)
  AND (k.organisation_id IS NULL OR r.organisation_id = k.organisation_id)

UNION ALL

-- A key whose owner reaches every tenant does too, unless it is bound to one.
-- oversees rather than belongs, for the same reason a platform role uses it:
-- the term sits outside the lifecycle subtraction.
SELECT DISTINCT 'kyc', 'organisation',
    COALESCE(k.organisation_id, '*')::text,
    (CASE WHEN k.organisation_id IS NULL THEN 'oversees' ELSE 'belongs' END)::text,
    'key', k.id, '', NULL::timestamptz, 'api_keys'
FROM api_keys k
JOIN memberships m ON m.user_id = k.user_id AND m.status = 'active'
JOIN roles r ON r.id = m.role_id
WHERE k.revoked_at IS NULL
  AND k.user_id IS NOT NULL
  AND (m.expires_at IS NULL OR m.expires_at > now())
  AND COALESCE(r.organisation_id = (SELECT platform_organisation_id FROM system_state WHERE id = 1), false)

UNION ALL

-- Ownership is lifecycle, not authority. It confers nothing; it exists so a
-- departing person's keys can be found and swept.
SELECT 'kyc', 'key', k.id, 'owner', 'user', k.user_id, '', NULL::timestamptz, 'api_keys'
FROM api_keys k
WHERE k.user_id IS NOT NULL AND k.revoked_at IS NULL

UNION ALL

-- A recovery credential reaches everything, as an edge on the star nodes rather
-- than as a short-circuit in code. It goes through the same walk a membership
-- does, so it appears in Decision.Path and can be audited like anything else.
SELECT 'kyc', t.object_type, '*', 'can_' || a.action, 'recovery', rc.id, '', rc.expires_at, 'recovery_credentials'
FROM recovery_credentials rc
CROSS JOIN (VALUES
    ('organisation'), ('members'), ('org_roles'), ('api_keys'), ('app_users'),
    ('app_access'), ('attributes'), ('automations'), ('billing'),
    ('email_templates'), ('product_features'), ('activity'), ('usage')
) AS t(object_type)
CROSS JOIN (VALUES
    ('read'), ('write'), ('update'), ('manage'), ('invite'), ('remove')
) AS a(action)
WHERE rc.revoked_at IS NULL

UNION ALL

SELECT 'kyc', 'organisation', '*', 'oversees', 'recovery', rc.id, '', rc.expires_at, 'recovery_credentials'
FROM recovery_credentials rc
WHERE rc.revoked_at IS NULL

UNION ALL

-- Edges with no legacy source: anything written directly, and everything once
-- the branches above are retired.
SELECT namespace, object_type, object_id, relation, subject_type, subject_id, subject_relation, expires_at, source
FROM reach_edges;
