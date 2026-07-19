DELETE FROM entitlements WHERE id = 'ent_premium_reports';

ALTER TABLE entitlements DROP COLUMN IF EXISTS scope;
