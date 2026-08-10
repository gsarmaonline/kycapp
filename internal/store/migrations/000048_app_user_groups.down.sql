DROP INDEX IF EXISTS app_grants_group_subject_idx;
DROP INDEX IF EXISTS app_grants_user_subject_idx;
ALTER TABLE app_grants DROP CONSTRAINT IF EXISTS app_grants_one_subject;
DELETE FROM app_grants WHERE app_user_id IS NULL;
ALTER TABLE app_grants DROP COLUMN IF EXISTS group_id;
ALTER TABLE app_grants ALTER COLUMN app_user_id SET NOT NULL;
DROP TABLE IF EXISTS app_user_group_members;
DROP TABLE IF EXISTS app_user_groups;
