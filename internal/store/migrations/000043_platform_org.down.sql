DROP TABLE IF EXISTS system_state;

DELETE FROM role_permissions WHERE role_id IN
    ('role_platform_root', 'role_platform_support', 'role_platform_billing');
DELETE FROM memberships WHERE organisation_id = 'org_platform';
DELETE FROM roles WHERE organisation_id = 'org_platform';
DELETE FROM organisations WHERE id = 'org_platform';

DROP INDEX IF EXISTS memberships_expires_at_idx;
ALTER TABLE memberships DROP COLUMN IF EXISTS expires_at;
ALTER TABLE roles DROP COLUMN IF EXISTS grants_global_reach;
