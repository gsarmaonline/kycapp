-- Org API key scopes, last-used tracking, and dedicated manage permissions.

ALTER TABLE api_keys
    ADD COLUMN scopes text[] NOT NULL DEFAULT '{}',
    ADD COLUMN last_used_at TIMESTAMPTZ;

INSERT INTO permissions (id, key, resource, action, category, description, is_system) VALUES
    ('perm_api_keys_read',   'api_keys:read',   'api_keys', 'read',   'Admin', 'View organisation API keys', true),
    ('perm_api_keys_manage', 'api_keys:manage', 'api_keys', 'manage', 'Admin', 'Create and revoke organisation API keys', true);

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.key IN ('owner', 'admin')
  AND p.key IN ('api_keys:read', 'api_keys:manage')
ON CONFLICT DO NOTHING;
