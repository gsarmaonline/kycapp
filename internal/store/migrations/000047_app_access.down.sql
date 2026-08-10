DELETE FROM role_permissions WHERE permission_id IN ('perm_app_access_read', 'perm_app_access_manage');
DELETE FROM permissions WHERE id IN ('perm_app_access_read', 'perm_app_access_manage');
DROP TABLE IF EXISTS app_grants;
DROP TABLE IF EXISTS app_role_extends;
DROP TABLE IF EXISTS app_roles;
DROP TABLE IF EXISTS app_capabilities;
DROP TABLE IF EXISTS app_scope_types;
