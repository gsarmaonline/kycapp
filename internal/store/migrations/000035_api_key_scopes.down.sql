DELETE FROM role_permissions
WHERE permission_id IN ('perm_api_keys_read', 'perm_api_keys_manage');

DELETE FROM permissions
WHERE id IN ('perm_api_keys_read', 'perm_api_keys_manage');

ALTER TABLE api_keys
    DROP COLUMN IF EXISTS last_used_at,
    DROP COLUMN IF EXISTS scopes;
