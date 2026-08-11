-- Everyone-subject and wildcard grants cannot be expressed without these
-- columns, so they are dropped rather than rewritten. A wildcard grant has no
-- role to fall back to.
DELETE FROM app_grants WHERE subject_kind = 'everyone' OR all_capabilities = TRUE;

DROP INDEX IF EXISTS app_grants_everyone_subject_idx;
DROP INDEX IF EXISTS app_grants_everyone_idx;
DROP INDEX IF EXISTS app_grants_user_subject_idx;
DROP INDEX IF EXISTS app_grants_group_subject_idx;

ALTER TABLE app_grants DROP CONSTRAINT IF EXISTS app_grants_known_constraint;
ALTER TABLE app_grants DROP CONSTRAINT IF EXISTS app_grants_capability_exceptions_need_wildcard;
ALTER TABLE app_grants DROP CONSTRAINT IF EXISTS app_grants_role_or_wildcard;
ALTER TABLE app_grants DROP CONSTRAINT IF EXISTS app_grants_subject_matches_kind;

ALTER TABLE app_grants DROP COLUMN IF EXISTS constraint_kind;
ALTER TABLE app_grants DROP COLUMN IF EXISTS except_capabilities;
ALTER TABLE app_grants DROP COLUMN IF EXISTS all_capabilities;
ALTER TABLE app_grants DROP COLUMN IF EXISTS except_scopes;
ALTER TABLE app_grants DROP COLUMN IF EXISTS except_app_user_ids;
ALTER TABLE app_grants DROP COLUMN IF EXISTS subject_kind;

ALTER TABLE app_grants ALTER COLUMN role_id SET NOT NULL;

ALTER TABLE app_grants ADD CONSTRAINT app_grants_one_subject
    CHECK ((app_user_id IS NULL) <> (group_id IS NULL));

CREATE UNIQUE INDEX IF NOT EXISTS app_grants_user_subject_idx
    ON app_grants (app_user_id, role_id, scope_kind, scope_id) WHERE app_user_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS app_grants_group_subject_idx
    ON app_grants (group_id, role_id, scope_kind, scope_id) WHERE group_id IS NOT NULL;
