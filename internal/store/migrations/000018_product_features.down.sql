DELETE FROM role_permissions
WHERE permission_id IN ('perm_product_features_read', 'perm_product_features_manage');

DELETE FROM permissions
WHERE id IN ('perm_product_features_read', 'perm_product_features_manage');

DROP TABLE IF EXISTS organisation_product_plans;
DROP TABLE IF EXISTS product_plan_features;
DROP TABLE IF EXISTS product_plans;

ALTER TABLE entitlements DROP CONSTRAINT IF EXISTS entitlements_org_scope_chk;
DROP INDEX IF EXISTS entitlements_org_key_uidx;
DROP INDEX IF EXISTS entitlements_global_key_uidx;

ALTER TABLE entitlements DROP COLUMN IF EXISTS organisation_id;

CREATE UNIQUE INDEX entitlements_key_key ON entitlements (key);
