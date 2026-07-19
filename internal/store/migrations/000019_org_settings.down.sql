DROP TABLE IF EXISTS organisation_integrations;
DROP INDEX IF EXISTS api_keys_org_idx;
ALTER TABLE api_keys DROP COLUMN IF EXISTS organisation_id;
