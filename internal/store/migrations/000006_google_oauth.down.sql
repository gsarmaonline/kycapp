ALTER TABLE users
    ADD COLUMN password_hash TEXT;

ALTER TABLE users
    DROP COLUMN IF EXISTS google_sub;
