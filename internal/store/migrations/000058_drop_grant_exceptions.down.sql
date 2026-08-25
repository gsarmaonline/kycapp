-- Restores the columns, empty. The contents cannot come back: dropping a column
-- discards it, and nothing else in the schema recorded what was in these.
ALTER TABLE app_grants ADD COLUMN IF NOT EXISTS except_app_user_ids TEXT[] NOT NULL DEFAULT '{}';

ALTER TABLE app_grants ADD COLUMN IF NOT EXISTS except_scopes JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE app_grants ADD COLUMN IF NOT EXISTS except_capabilities TEXT[] NOT NULL DEFAULT '{}';

ALTER TABLE app_grants ADD CONSTRAINT app_grants_capability_exceptions_need_wildcard CHECK (
    all_capabilities = TRUE OR cardinality(except_capabilities) = 0
);
