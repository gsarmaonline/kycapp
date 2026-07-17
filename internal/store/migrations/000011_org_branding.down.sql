ALTER TABLE organisations
    DROP COLUMN IF EXISTS email_footer,
    DROP COLUMN IF EXISTS accent_color,
    DROP COLUMN IF EXISTS primary_color,
    DROP COLUMN IF EXISTS logo_url;
