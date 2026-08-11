-- Wildcards and exceptions on a merchant's grants.
--
-- A wildcard names a set nobody can enumerate; an exception names the members
-- that do not belong. They are one feature. Positive enumeration handles the
-- ordinary case, but it collapses when the include set is huge and the
-- exclusion tiny: ten thousand projects, one confidential, and 9,999 grants to
-- say so.
--
-- Every exception narrows the grant it sits on and nothing else. No grant
-- subtracts from another, so grants stay unordered, evaluation stays
-- first-match, and deleting a grant still removes access rather than adding it.

-- Subject gains a third kind. 'everyone' covers every customer of the
-- organisation, present and future, from one row. Its two id columns are null:
-- it names no one because it names all of them.
ALTER TABLE app_grants ADD COLUMN IF NOT EXISTS subject_kind TEXT NOT NULL DEFAULT 'app_user';

UPDATE app_grants SET subject_kind = CASE
    WHEN group_id IS NOT NULL THEN 'group'
    ELSE 'app_user'
END;

-- Replaces the old exactly-one-of check, which predates the third kind.
ALTER TABLE app_grants DROP CONSTRAINT IF EXISTS app_grants_one_subject;
ALTER TABLE app_grants ADD CONSTRAINT app_grants_subject_matches_kind CHECK (
    (subject_kind = 'app_user' AND app_user_id IS NOT NULL AND group_id IS NULL)
 OR (subject_kind = 'group'    AND group_id    IS NOT NULL AND app_user_id IS NULL)
 OR (subject_kind = 'everyone' AND group_id    IS NULL     AND app_user_id IS NULL)
);

-- Which customers an everyone-grant skips. The counterpart to the subject
-- wildcard: offboard one person without enumerating the rest.
ALTER TABLE app_grants ADD COLUMN IF NOT EXISTS except_app_user_ids TEXT[] NOT NULL DEFAULT '{}';

-- Scopes this grant does not reach, despite its scope covering them.
-- JSONB rather than two arrays so a kind and an id stay paired.
ALTER TABLE app_grants ADD COLUMN IF NOT EXISTS except_scopes JSONB NOT NULL DEFAULT '[]'::jsonb;

-- The capability wildcard. A grant may carry every capability in the
-- organisation's namespace, including ones declared after it was written, minus
-- the carve-outs. The registry still refuses to let anyone DECLARE a capability
-- named '*'; the wildcard lives on the grant, never in the vocabulary.
ALTER TABLE app_grants ADD COLUMN IF NOT EXISTS all_capabilities BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE app_grants ADD COLUMN IF NOT EXISTS except_capabilities TEXT[] NOT NULL DEFAULT '{}';

-- A wildcard grant carries no role, so role_id can no longer be required.
ALTER TABLE app_grants ALTER COLUMN role_id DROP NOT NULL;
ALTER TABLE app_grants ADD CONSTRAINT app_grants_role_or_wildcard CHECK (
    (role_id IS NOT NULL AND all_capabilities = FALSE)
 OR (role_id IS NULL     AND all_capabilities = TRUE)
);

-- An exclusion with no wildcard beside it does nothing, and reads as though it
-- does. Refuse it in the schema rather than leave a misleading row.
ALTER TABLE app_grants ADD CONSTRAINT app_grants_capability_exceptions_need_wildcard CHECK (
    all_capabilities = TRUE OR cardinality(except_capabilities) = 0
);

-- Narrows a grant using something only the request knows. 'self_subject' means
-- the grant applies only to resources belonging to the holder, which is how
-- "everyone may manage their own things" is written as one row.
ALTER TABLE app_grants ADD COLUMN IF NOT EXISTS constraint_kind TEXT NOT NULL DEFAULT '';
ALTER TABLE app_grants ADD CONSTRAINT app_grants_known_constraint CHECK (
    constraint_kind IN ('', 'self_subject')
);

-- Assembly reads every everyone-grant for the organisation on each access
-- lookup, so it needs its own index rather than riding the user one.
CREATE INDEX IF NOT EXISTS app_grants_everyone_idx
    ON app_grants (organisation_id) WHERE subject_kind = 'everyone';

-- The old per-subject uniqueness assumed a role, which a wildcard grant has
-- not. COALESCE covers both: NULL role means the wildcard, and the check above
-- already ties the two together, so all_capabilities would add nothing here.
DROP INDEX IF EXISTS app_grants_user_subject_idx;
DROP INDEX IF EXISTS app_grants_group_subject_idx;
CREATE UNIQUE INDEX IF NOT EXISTS app_grants_user_subject_idx
    ON app_grants (app_user_id, COALESCE(role_id, ''), scope_kind, scope_id)
    WHERE app_user_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS app_grants_group_subject_idx
    ON app_grants (group_id, COALESCE(role_id, ''), scope_kind, scope_id)
    WHERE group_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS app_grants_everyone_subject_idx
    ON app_grants (organisation_id, COALESCE(role_id, ''), scope_kind, scope_id)
    WHERE subject_kind = 'everyone';
