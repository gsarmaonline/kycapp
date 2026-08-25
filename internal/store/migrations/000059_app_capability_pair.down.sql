DROP INDEX IF EXISTS app_capabilities_org_pair_idx;

ALTER TABLE app_capabilities DROP COLUMN IF EXISTS key;

ALTER TABLE app_capabilities ADD COLUMN key TEXT NOT NULL DEFAULT '';

UPDATE app_capabilities SET key = resource || ':' || action;

ALTER TABLE app_capabilities ADD CONSTRAINT app_capabilities_organisation_id_key_key
    UNIQUE (organisation_id, key);

ALTER TABLE app_capabilities DROP CONSTRAINT IF EXISTS app_capabilities_pair_present;

ALTER TABLE app_capabilities DROP COLUMN IF EXISTS resource;

ALTER TABLE app_capabilities DROP COLUMN IF EXISTS action;
