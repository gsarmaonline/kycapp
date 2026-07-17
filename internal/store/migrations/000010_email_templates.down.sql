DELETE FROM role_permissions
WHERE permission_id IN ('perm_email_templates_read', 'perm_email_templates_manage');

DELETE FROM permissions
WHERE id IN ('perm_email_templates_read', 'perm_email_templates_manage');

DROP TABLE IF EXISTS email_templates;
