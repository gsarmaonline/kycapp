DROP TABLE IF EXISTS product_feature_overrides;

ALTER TABLE entitlements
    DROP COLUMN IF EXISTS rollout_percentage,
    DROP COLUMN IF EXISTS enabled;

-- Restore separate feature_flags (000037) on rollback.
CREATE TABLE feature_flags (
    id                  TEXT PRIMARY KEY,
    organisation_id     TEXT NOT NULL REFERENCES organisations (id) ON DELETE CASCADE,
    key                 TEXT NOT NULL,
    description         TEXT NOT NULL DEFAULT '',
    enabled             BOOLEAN NOT NULL DEFAULT true,
    rollout_percentage  INTEGER NOT NULL DEFAULT 0
        CHECK (rollout_percentage >= 0 AND rollout_percentage <= 100),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organisation_id, key)
);

CREATE TABLE feature_flag_overrides (
    feature_flag_id  TEXT NOT NULL REFERENCES feature_flags (id) ON DELETE CASCADE,
    subject_id       TEXT NOT NULL,
    effect           TEXT NOT NULL CHECK (effect IN ('include', 'exclude')),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (feature_flag_id, subject_id)
);

CREATE INDEX feature_flag_overrides_flag_idx ON feature_flag_overrides (feature_flag_id);

INSERT INTO permissions (id, key, resource, action, category, description, is_system) VALUES
    ('perm_feature_flags_read',   'feature_flags:read',   'feature_flags', 'read',   'Product', 'View feature flags', true),
    ('perm_feature_flags_manage', 'feature_flags:manage', 'feature_flags', 'manage', 'Product', 'Manage feature flags and overrides', true)
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.key IN ('owner', 'admin')
  AND p.key IN ('feature_flags:read', 'feature_flags:manage')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.key = 'member'
  AND p.key = 'feature_flags:read'
ON CONFLICT DO NOTHING;
