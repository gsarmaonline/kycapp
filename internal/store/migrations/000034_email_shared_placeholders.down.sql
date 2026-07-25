-- Best-effort reverse of shared-path rewrite (may not match custom edits).

UPDATE email_templates
SET
    subject = replace(replace(replace(subject,
        '{{app_user.display_name}}', '{{display_name}}'),
        '{{organisation.name}}', '{{org_name}}'),
        '{{app_user.email}}', '{{email}}'),
    body_text = replace(replace(replace(body_text,
        '{{app_user.display_name}}', '{{display_name}}'),
        '{{organisation.name}}', '{{org_name}}'),
        '{{app_user.email}}', '{{email}}'),
    body_html = replace(replace(replace(body_html,
        '{{app_user.display_name}}', '{{display_name}}'),
        '{{organisation.name}}', '{{org_name}}'),
        '{{app_user.email}}', '{{email}}'),
    updated_at = now();
