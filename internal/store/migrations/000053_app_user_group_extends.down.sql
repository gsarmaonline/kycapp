ALTER TABLE app_user_groups DROP COLUMN IF EXISTS effective_parent_ids;
DROP INDEX IF EXISTS app_user_group_extends_parent_idx;
DROP TABLE IF EXISTS app_user_group_extends;
