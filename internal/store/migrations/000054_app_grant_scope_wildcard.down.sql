ALTER TABLE app_grants DROP CONSTRAINT IF EXISTS app_grants_scope_matches_wildcard;
ALTER TABLE app_grants DROP COLUMN IF EXISTS all_scopes;
