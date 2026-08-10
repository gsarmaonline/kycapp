ALTER TABLE roles ADD COLUMN IF NOT EXISTS grants_global_reach BOOLEAN NOT NULL DEFAULT FALSE;
UPDATE roles SET grants_global_reach = TRUE WHERE organisation_id = 'org_platform';
