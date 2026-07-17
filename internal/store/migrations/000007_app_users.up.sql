-- Org end-users (app users) + schema-backed profile attribute definitions.
-- Distinct from memberships (operators of KYC).

CREATE TABLE attribute_definitions (
    id               TEXT PRIMARY KEY,
    organisation_id  TEXT NOT NULL REFERENCES organisations (id),
    key              TEXT NOT NULL,
    label            TEXT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    value_type       TEXT NOT NULL CHECK (value_type IN ('string', 'number', 'boolean', 'date', 'dropdown')),
    section          TEXT NOT NULL DEFAULT 'general',
    sort_order       INTEGER NOT NULL DEFAULT 0,
    required         BOOLEAN NOT NULL DEFAULT false,
    enum_values      JSONB NOT NULL DEFAULT '[]'::jsonb,
    is_pii           BOOLEAN NOT NULL DEFAULT false,
    status           TEXT NOT NULL CHECK (status IN ('active', 'archived')),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organisation_id, key)
);

CREATE INDEX attribute_definitions_org_section_idx
    ON attribute_definitions (organisation_id, section, sort_order);

CREATE TABLE app_users (
    id               TEXT PRIMARY KEY,
    organisation_id  TEXT NOT NULL REFERENCES organisations (id),
    external_id      TEXT,
    email            TEXT,
    display_name     TEXT NOT NULL DEFAULT '',
    status           TEXT NOT NULL CHECK (status IN ('active', 'disabled', 'archived')),
    attributes       JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX app_users_org_email_uidx
    ON app_users (organisation_id, lower(email))
    WHERE email IS NOT NULL AND email <> '';

CREATE UNIQUE INDEX app_users_org_external_id_uidx
    ON app_users (organisation_id, external_id)
    WHERE external_id IS NOT NULL AND external_id <> '';

CREATE INDEX app_users_org_created_idx ON app_users (organisation_id, created_at DESC);
CREATE INDEX app_users_attributes_gin ON app_users USING gin (attributes jsonb_path_ops);

-- Permissions for schema + end-users
INSERT INTO permissions (id, key, resource, action, category, description, is_system) VALUES
    ('perm_attributes_read',   'attributes:read',   'attributes', 'read',   'Users', 'View attribute definitions', true),
    ('perm_attributes_manage', 'attributes:manage', 'attributes', 'manage', 'Users', 'Create and update attribute definitions', true),
    ('perm_app_users_read',    'app_users:read',    'app_users',  'read',   'Users', 'List and view organisation app users', true),
    ('perm_app_users_write',   'app_users:write',   'app_users',  'write',  'Users', 'Create and update organisation app users', true);

-- Grant to existing owner/admin roles; read to member
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.key IN ('owner', 'admin')
  AND p.key IN ('attributes:read', 'attributes:manage', 'app_users:read', 'app_users:write')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.key = 'member'
  AND p.key IN ('attributes:read', 'app_users:read')
ON CONFLICT DO NOTHING;
