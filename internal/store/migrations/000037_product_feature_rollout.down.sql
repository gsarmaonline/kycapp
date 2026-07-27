DROP TABLE IF EXISTS product_feature_overrides;

ALTER TABLE entitlements
    DROP COLUMN IF EXISTS rollout_percentage,
    DROP COLUMN IF EXISTS enabled;
