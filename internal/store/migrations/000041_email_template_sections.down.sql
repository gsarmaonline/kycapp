ALTER TABLE email_templates
    DROP COLUMN IF EXISTS body_sections,
    DROP COLUMN IF EXISTS from_name,
    DROP COLUMN IF EXISTS from_address;
