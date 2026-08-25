-- A capability was a matrix wearing a string.
--
-- app_capabilities.key held 'document:read', and MerchantSchema immediately cut
-- every key on ':' and rebuilt a grid: actions as a set, resources as a map of
-- resource to its actions. The grid was the model; the string was the lossy
-- form, and the lossiness had costs.
--
-- The key could be malformed, and the failure surfaced late: a bad key parsed
-- fine at write time and blew up when the schema was generated, which is a
-- different request on a different page. The reserved-name check ran per key
-- rather than once per resource. And nothing could ask the obvious questions --
-- which actions exist, which resources answer this action -- without splitting
-- strings in SQL.
--
-- So the two halves get columns. key stays as a generated column because it is
-- the merchant-facing name of the thing, it is what roles carry in
-- own_capabilities and effective_capabilities, and it is on the wire in the
-- grant set. Deriving it means the pair and the name cannot disagree.
--
-- Not in scope here: app_scope_types. A scope kind and a capability's resource
-- already collide in one namespace of type names -- MerchantSchema drops the
-- resource declaration when a scope kind of that name exists, so which page you
-- used decides whether the type answers every action or only its own. That is a
-- real problem and a separate one. Splitting the key surfaces it; it does not
-- fix it.

ALTER TABLE app_capabilities ADD COLUMN IF NOT EXISTS resource TEXT NOT NULL DEFAULT '';

ALTER TABLE app_capabilities ADD COLUMN IF NOT EXISTS action TEXT NOT NULL DEFAULT '';

UPDATE app_capabilities
SET resource = split_part(key, ':', 1),
    action   = split_part(key, ':', 2)
WHERE resource = '' OR action = '';

-- Both halves are required. A key that could not be split would have reached
-- MerchantSchema and failed there, one page away from where it was written.
ALTER TABLE app_capabilities DROP CONSTRAINT IF EXISTS app_capabilities_pair_present;

ALTER TABLE app_capabilities ADD CONSTRAINT app_capabilities_pair_present CHECK (
    resource <> '' AND action <> ''
);

-- key becomes derived, so the pair and the name cannot drift apart. Dropping
-- and re-adding rather than altering in place: a plain column cannot be
-- converted to a generated one.
ALTER TABLE app_capabilities DROP CONSTRAINT IF EXISTS app_capabilities_organisation_id_key_key;

ALTER TABLE app_capabilities DROP COLUMN IF EXISTS key;

-- NOT NULL is not decoration. Both halves are NOT NULL and non-empty by the
-- constraint above, so the derived value never is, and saying so keeps the
-- generated Go type a plain string rather than a nullable one that every caller
-- would have to unwrap.
ALTER TABLE app_capabilities
    ADD COLUMN key TEXT GENERATED ALWAYS AS (resource || ':' || action) STORED NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS app_capabilities_org_pair_idx
    ON app_capabilities (organisation_id, resource, action);
