DROP INDEX IF EXISTS api_keys_user_idx;
ALTER TABLE api_keys DROP COLUMN IF EXISTS user_id;
