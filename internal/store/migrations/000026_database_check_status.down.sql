ALTER TABLE organisation_databases
    DROP COLUMN IF EXISTS last_error,
    DROP COLUMN IF EXISTS last_checked_at;

ALTER TABLE organisation_databases
    DROP CONSTRAINT IF EXISTS organisation_databases_status_check;

ALTER TABLE organisation_databases
    ADD CONSTRAINT organisation_databases_status_check
        CHECK (status IN ('connected', 'disconnected'));
