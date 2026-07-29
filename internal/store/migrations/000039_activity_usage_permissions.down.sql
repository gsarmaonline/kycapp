DELETE FROM role_permissions
WHERE permission_id IN ('perm_activity_read', 'perm_usage_read');

DELETE FROM permissions
WHERE id IN ('perm_activity_read', 'perm_usage_read');
