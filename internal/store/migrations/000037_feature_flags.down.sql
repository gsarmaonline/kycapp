DELETE FROM role_permissions
WHERE permission_id IN ('perm_feature_flags_read', 'perm_feature_flags_manage');

DELETE FROM permissions
WHERE id IN ('perm_feature_flags_read', 'perm_feature_flags_manage');

DROP TABLE IF EXISTS feature_flag_overrides;
DROP TABLE IF EXISTS feature_flags;
