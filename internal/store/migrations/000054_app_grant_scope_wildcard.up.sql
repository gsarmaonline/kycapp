-- The scope wildcard.
--
-- Two of the three axes carried a wildcard and the third did not:
--
--   subject       everyone          except_app_user_ids
--   capabilities  all_capabilities  except_capabilities
--   scope         nothing           except_scopes
--
-- So "every project this customer works on" could not be written at all. A
-- merchant had to issue one grant per project and reissue on every new one,
-- which is the bookkeeping the everyone subject and the capability wildcard
-- both exist to remove.
--
-- Two levels, because a scope has instances and a capability does not:
--
--   scope_id = '*'    every instance of one declared kind: every project
--   all_scopes        every kind at once, the whole organisation
--
-- The organisation is deliberately not a scope kind. Scope is a (kind, id)
-- pair, and organisation has exactly one instance, already carried by
-- app_grants.organisation_id. Declaring it would be a second way to say what
-- every grant already says. It is the ceiling, not a name.
--
-- 'global' stays reserved and unusable. A merchant's world ends at their
-- organisation, so a grant reaching past it would cross into another tenant.
-- That reservation is the boundary working, not a gap.
--
-- No constraint is added tying except_scopes to a wildcard, and that is
-- deliberate. An excepted scope may name a different kind from the grant's, so
-- "tenant acme, except project salaries" is meaningful against a containment
-- the merchant's backend resolves. The exception was never waiting for this
-- column.

ALTER TABLE app_grants ADD COLUMN IF NOT EXISTS all_scopes BOOLEAN NOT NULL DEFAULT FALSE;

-- An organisation-wide grant names no kind and no id, the same way a wildcard
-- capability grant carries no role. Anything else is a contradiction: a grant
-- cannot be both everywhere and somewhere. Existing rows all satisfy the
-- second branch, so this is checkable without a backfill.
ALTER TABLE app_grants DROP CONSTRAINT IF EXISTS app_grants_scope_matches_wildcard;
ALTER TABLE app_grants ADD CONSTRAINT app_grants_scope_matches_wildcard CHECK (
       (all_scopes     AND scope_kind = '' AND scope_id = '')
    OR (NOT all_scopes AND scope_kind <> '' AND scope_id <> '')
);
