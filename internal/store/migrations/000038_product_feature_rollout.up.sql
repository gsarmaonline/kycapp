-- Fold separate feature_flags into org-owned product features (entitlements.scope = product).

DROP TABLE IF EXISTS feature_flag_overrides;
DROP TABLE IF EXISTS feature_flags;

DELETE FROM role_permissions
WHERE permission_id IN ('perm_feature_flags_read', 'perm_feature_flags_manage');

DELETE FROM permissions
WHERE key IN ('feature_flags:read', 'feature_flags:manage');

ALTER TABLE entitlements
    ADD COLUMN IF NOT EXISTS enabled BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS rollout_percentage INTEGER NOT NULL DEFAULT 100
        CHECK (rollout_percentage >= 0 AND rollout_percentage <= 100);

CREATE TABLE product_feature_overrides (
    entitlement_id  TEXT NOT NULL REFERENCES entitlements (id) ON DELETE CASCADE,
    subject_id      TEXT NOT NULL,
    effect          TEXT NOT NULL CHECK (effect IN ('include', 'exclude')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (entitlement_id, subject_id)
);

CREATE INDEX product_feature_overrides_entitlement_idx ON product_feature_overrides (entitlement_id);
