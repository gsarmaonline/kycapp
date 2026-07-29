-- Org activity timeline and usage meter read permissions.

INSERT INTO permissions (id, key, resource, action, category, description, is_system) VALUES
    ('perm_activity_read', 'activity:read', 'activity', 'read', 'Admin', 'View organisation activity timeline', true),
    ('perm_usage_read',    'usage:read',    'usage',    'read', 'Admin', 'View organisation usage meters', true);

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.key IN ('owner', 'admin')
  AND p.key IN ('activity:read', 'usage:read')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.key = 'member'
  AND p.key IN ('activity:read', 'usage:read')
ON CONFLICT DO NOTHING;
