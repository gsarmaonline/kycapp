-- Google OAuth identity; drop password auth

ALTER TABLE users
    ADD COLUMN google_sub TEXT UNIQUE;

ALTER TABLE users
    DROP COLUMN IF EXISTS password_hash;
