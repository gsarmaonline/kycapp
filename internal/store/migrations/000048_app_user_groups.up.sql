-- Groups of app users, so a role can be granted to a set rather than one person
-- at a time.
--
-- A group answers "which principals"; a scope answers "which resources". They
-- are orthogonal, so a grant gains a subject rather than changing shape.
--
-- Membership is static here: an explicit list. Rules over attributes are a
-- separate step, deliberately, because they turn every attribute write into a
-- permission change and need their own audit story first.
CREATE TABLE IF NOT EXISTS app_user_groups (
    id              TEXT PRIMARY KEY,
    organisation_id TEXT NOT NULL REFERENCES organisations (id) ON DELETE CASCADE,
    key             TEXT NOT NULL,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organisation_id, key)
);

CREATE TABLE IF NOT EXISTS app_user_group_members (
    group_id    TEXT NOT NULL REFERENCES app_user_groups (id) ON DELETE CASCADE,
    app_user_id TEXT NOT NULL REFERENCES app_users (id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, app_user_id)
);

CREATE INDEX IF NOT EXISTS app_user_group_members_user_idx
    ON app_user_group_members (app_user_id);

-- A grant now targets exactly one subject: an app user or a group.
ALTER TABLE app_grants ALTER COLUMN app_user_id DROP NOT NULL;
ALTER TABLE app_grants ADD COLUMN IF NOT EXISTS group_id TEXT
    REFERENCES app_user_groups (id) ON DELETE CASCADE;

-- Exactly one, never both and never neither. A grant with no subject would
-- apply to nobody and a grant with two would be ambiguous.
ALTER TABLE app_grants DROP CONSTRAINT IF EXISTS app_grants_one_subject;
ALTER TABLE app_grants ADD CONSTRAINT app_grants_one_subject
    CHECK ((app_user_id IS NULL) <> (group_id IS NULL));

-- The old uniqueness assumed a user subject. Replace it with one index per
-- subject kind, since NULLs do not compare equal in a unique constraint.
ALTER TABLE app_grants DROP CONSTRAINT IF EXISTS app_grants_app_user_id_role_id_scope_kind_scope_id_key;
CREATE UNIQUE INDEX IF NOT EXISTS app_grants_user_subject_idx
    ON app_grants (app_user_id, role_id, scope_kind, scope_id) WHERE app_user_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS app_grants_group_subject_idx
    ON app_grants (group_id, role_id, scope_kind, scope_id) WHERE group_id IS NOT NULL;
