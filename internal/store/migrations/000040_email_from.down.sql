ALTER TABLE organisations
    DROP COLUMN IF EXISTS email_from_name,
    DROP COLUMN IF EXISTS email_from_address;
