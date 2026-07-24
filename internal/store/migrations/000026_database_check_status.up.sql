-- Connectivity probe metadata for organisation databases.
ALTER TABLE organisation_databases
    DROP CONSTRAINT IF EXISTS organisation_databases_status_check;

ALTER TABLE organisation_databases
    ADD CONSTRAINT organisation_databases_status_check
        CHECK (status IN ('connected', 'unreachable', 'disconnected'));

ALTER TABLE organisation_databases
    ADD COLUMN IF NOT EXISTS last_checked_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_error TEXT NOT NULL DEFAULT '';
