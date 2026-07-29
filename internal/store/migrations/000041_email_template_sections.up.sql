-- Per-template body sections + optional From override.

ALTER TABLE email_templates
    ADD COLUMN body_sections JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN from_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN from_address TEXT NOT NULL DEFAULT '';

-- Backfill one section from legacy body_html when present.
UPDATE email_templates
SET body_sections = jsonb_build_array(
    jsonb_build_object(
        'id', 'sec_legacy',
        'content_html', body_html,
        'style', '{}'::jsonb
    )
)
WHERE trim(body_html) <> ''
  AND (body_sections = '[]'::jsonb OR body_sections IS NULL);
