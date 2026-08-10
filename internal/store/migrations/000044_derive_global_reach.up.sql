-- Global reach is no longer a boolean anyone could set. It is derived from
-- membership of the platform organisation, so gaining cross-tenant reach means
-- being made a member of that organisation, which requires members:invite
-- *inside it*. Invariant 4 is then enforced by the access model rather than by
-- a check somebody has to remember to write.
ALTER TABLE roles DROP COLUMN IF EXISTS grants_global_reach;
