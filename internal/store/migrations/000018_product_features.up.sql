-- Org-owned product features (entitlements.scope = product + organisation_id)
-- and product plans that package those features for end-user gating.

ALTER TABLE entitlements
    ADD COLUMN organisation_id TEXT REFERENCES organisations (id);

ALTER TABLE entitlements
    DROP CONSTRAINT IF EXISTS entitlements_key_key;

CREATE UNIQUE INDEX entitlements_global_key_uidx
    ON entitlements (key)
    WHERE organisation_id IS NULL;

CREATE UNIQUE INDEX entitlements_org_key_uidx
    ON entitlements (organisation_id, key)
    WHERE organisation_id IS NOT NULL;

ALTER TABLE entitlements
    ADD CONSTRAINT entitlements_org_scope_chk
    CHECK (organisation_id IS NULL OR scope = 'product');

CREATE TABLE product_plans (
    id               TEXT PRIMARY KEY,
    organisation_id  TEXT NOT NULL REFERENCES organisations (id),
    key              TEXT NOT NULL,
    name             TEXT NOT NULL,
    status           TEXT NOT NULL CHECK (status IN ('active', 'archived')),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organisation_id, key)
);

CREATE TABLE product_plan_features (
    product_plan_id  TEXT NOT NULL REFERENCES product_plans (id) ON DELETE CASCADE,
    entitlement_id   TEXT NOT NULL REFERENCES entitlements (id) ON DELETE CASCADE,
    PRIMARY KEY (product_plan_id, entitlement_id)
);

CREATE TABLE organisation_product_plans (
    organisation_id  TEXT PRIMARY KEY REFERENCES organisations (id),
    product_plan_id  TEXT NOT NULL REFERENCES product_plans (id),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO permissions (id, key, resource, action, category, description, is_system) VALUES
    ('perm_product_features_read',   'product_features:read',   'product_features', 'read',   'Product', 'View product features and plans', true),
    ('perm_product_features_manage', 'product_features:manage', 'product_features', 'manage', 'Product', 'Manage product features and plans', true);

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.key IN ('owner', 'admin')
  AND p.key IN ('product_features:read', 'product_features:manage')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.key = 'member'
  AND p.key = 'product_features:read'
ON CONFLICT DO NOTHING;
