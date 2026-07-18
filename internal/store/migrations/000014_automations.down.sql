DELETE FROM role_permissions
WHERE permission_id IN ('perm_automations_read', 'perm_automations_manage');

DELETE FROM permissions
WHERE id IN ('perm_automations_read', 'perm_automations_manage');

DROP TABLE IF EXISTS automation_runs;
DROP TABLE IF EXISTS automations;
