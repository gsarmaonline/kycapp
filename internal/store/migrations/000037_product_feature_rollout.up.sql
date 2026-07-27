-- Rollout controls on org-owned product features (entitlements.scope = product).
-- Platform entitlements keep defaults (enabled=true, rollout_percentage=100); rollout is ignored at check time.

ALTER TABLE entitlements
    ADD COLUMN enabled BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN rollout_percentage INTEGER NOT NULL DEFAULT 100
        CHECK (rollout_percentage >= 0 AND rollout_percentage <= 100);

CREATE TABLE product_feature_overrides (
    entitlement_id  TEXT NOT NULL REFERENCES entitlements (id) ON DELETE CASCADE,
    subject_id      TEXT NOT NULL,
    effect          TEXT NOT NULL CHECK (effect IN ('include', 'exclude')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (entitlement_id, subject_id)
);

CREATE INDEX product_feature_overrides_entitlement_idx ON product_feature_overrides (entitlement_id);
