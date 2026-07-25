-- Per-region email typography (header / body / footer).

ALTER TABLE organisations
    ADD COLUMN email_typography JSONB NOT NULL DEFAULT '{}'::jsonb;

UPDATE organisations
SET email_typography = jsonb_build_object(
    'header', jsonb_build_object(
        'font', email_font,
        'size', 20,
        'weight', 700,
        'style', 'normal'
    ),
    'body', jsonb_build_object(
        'font', email_font,
        'size', 16,
        'weight', 400,
        'style', 'normal'
    ),
    'footer', jsonb_build_object(
        'font', email_font,
        'size', 12,
        'weight', 400,
        'style', 'normal'
    )
);
