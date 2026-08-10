-- Merchant-hosted access control: a merchant declares their own scope types,
-- capabilities and roles, and grants them to their app users.
--
-- Two tiers, deliberately separate. KYC's RBAC decides who may *administer* a
-- merchant's access model; the merchant's model decides what their customers
-- may do. A merchant operator administers the model without being in it, which
-- is why the subset rule applies within a namespace and never across the
-- boundary.
--
-- Everything here lives in the merchant's namespace. Nothing in these tables
-- can name a KYC capability.

-- Scope kinds the merchant uses: project, environment, workspace.
--
-- The *ids* are deliberately not stored. A grant referencing a project that
-- does not exist simply never matches a resource, because no resource carries
-- that coordinate, so it fails closed and validation would only add coupling to
-- the merchant's own product structure. Kinds are declared so a typo is
-- rejected at write time instead of silently matching nothing.
CREATE TABLE IF NOT EXISTS app_scope_types (
    id              TEXT PRIMARY KEY,
    organisation_id TEXT NOT NULL REFERENCES organisations (id) ON DELETE CASCADE,
    kind            TEXT NOT NULL,
    label           TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organisation_id, kind)
);

CREATE TABLE IF NOT EXISTS app_capabilities (
    id              TEXT PRIMARY KEY,
    organisation_id TEXT NOT NULL REFERENCES organisations (id) ON DELETE CASCADE,
    key             TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organisation_id, key)
);

-- effective_capabilities is inheritance materialised at write time. The
-- decision path reads this flat set and never walks the extends graph, however
-- deep a merchant builds it.
CREATE TABLE IF NOT EXISTS app_roles (
    id                     TEXT PRIMARY KEY,
    organisation_id        TEXT NOT NULL REFERENCES organisations (id) ON DELETE CASCADE,
    key                    TEXT NOT NULL,
    name                   TEXT NOT NULL,
    description            TEXT NOT NULL DEFAULT '',
    own_capabilities       TEXT[] NOT NULL DEFAULT '{}',
    effective_capabilities TEXT[] NOT NULL DEFAULT '{}',
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organisation_id, key)
);

-- Multiple parents are allowed. Because capabilities only ever add, a union is
-- commutative and a diamond resolves the same way whatever order it is walked.
-- That property disappears the moment a deny rule exists, which is why there
-- are none.
CREATE TABLE IF NOT EXISTS app_role_extends (
    role_id   TEXT NOT NULL REFERENCES app_roles (id) ON DELETE CASCADE,
    parent_id TEXT NOT NULL REFERENCES app_roles (id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, parent_id),
    CHECK (role_id <> parent_id)
);

-- A grant binds a role to one app user at one scope. scope_id is opaque: it is
-- an identifier in the merchant's system, which KYC never resolves.
CREATE TABLE IF NOT EXISTS app_grants (
    id              TEXT PRIMARY KEY,
    organisation_id TEXT NOT NULL REFERENCES organisations (id) ON DELETE CASCADE,
    app_user_id     TEXT NOT NULL REFERENCES app_users (id) ON DELETE CASCADE,
    role_id         TEXT NOT NULL REFERENCES app_roles (id) ON DELETE CASCADE,
    scope_kind      TEXT NOT NULL,
    scope_id        TEXT NOT NULL,
    expires_at      TIMESTAMPTZ,
    granted_by      TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (app_user_id, role_id, scope_kind, scope_id)
);

CREATE INDEX IF NOT EXISTS app_grants_user_idx ON app_grants (app_user_id);
CREATE INDEX IF NOT EXISTS app_roles_org_idx ON app_roles (organisation_id);

-- Administering a merchant's access model is a KYC permission, held by their
-- operators. It is not a capability in their own namespace: they administer the
-- model without being inside it.
INSERT INTO permissions (id, key, resource, action, category, description, is_system) VALUES
    ('perm_app_access_read',   'app_access:read',   'app_access', 'read',   'Users', 'View app user roles and grants', true),
    ('perm_app_access_manage', 'app_access:manage', 'app_access', 'manage', 'Users', 'Define app user scopes, capabilities and roles, and grant them', true)
ON CONFLICT (id) DO NOTHING;

-- Existing owner and admin roles get the new permissions, so a merchant can use
-- the feature without reconfiguring.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.key IN ('owner', 'admin')
  AND p.key IN ('app_access:read', 'app_access:manage')
ON CONFLICT DO NOTHING;
