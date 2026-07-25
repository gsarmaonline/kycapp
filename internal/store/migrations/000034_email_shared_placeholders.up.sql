-- Rewrite legacy flat email placeholders to the shared path vocabulary.
-- Runtime still accepts display_name / org_name / email via NormalizeFieldPath.

UPDATE email_templates
SET
    subject = replace(replace(replace(subject,
        '{{display_name}}', '{{app_user.display_name}}'),
        '{{org_name}}', '{{organisation.name}}'),
        '{{email}}', '{{app_user.email}}'),
    body_text = replace(replace(replace(body_text,
        '{{display_name}}', '{{app_user.display_name}}'),
        '{{org_name}}', '{{organisation.name}}'),
        '{{email}}', '{{app_user.email}}'),
    body_html = replace(replace(replace(body_html,
        '{{display_name}}', '{{app_user.display_name}}'),
        '{{org_name}}', '{{organisation.name}}'),
        '{{email}}', '{{app_user.email}}'),
    updated_at = now();
