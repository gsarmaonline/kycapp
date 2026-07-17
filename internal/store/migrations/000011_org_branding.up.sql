-- Organisation email branding (logo URL, colors, footer).

ALTER TABLE organisations
    ADD COLUMN logo_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN primary_color TEXT NOT NULL DEFAULT '#1f4d3a',
    ADD COLUMN accent_color TEXT NOT NULL DEFAULT '',
    ADD COLUMN email_footer TEXT NOT NULL DEFAULT '';
