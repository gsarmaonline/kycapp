DELETE FROM role_permissions
WHERE permission_id IN (
    'perm_attributes_read',
    'perm_attributes_manage',
    'perm_app_users_read',
    'perm_app_users_write'
);

DELETE FROM permissions WHERE id IN (
    'perm_attributes_read',
    'perm_attributes_manage',
    'perm_app_users_read',
    'perm_app_users_write'
);

DROP TABLE IF EXISTS app_users;
DROP TABLE IF EXISTS attribute_definitions;
