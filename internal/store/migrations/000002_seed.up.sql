-- Seed system permission catalog, trial plan, and sample entitlements

INSERT INTO permissions (id, key, resource, action, category, description, is_system) VALUES
    ('perm_organisation_read',  'organisation:read',   'organisation', 'read',   'Admin',   'View organisation details', true),
    ('perm_organisation_update','organisation:update', 'organisation', 'update', 'Admin',   'Update organisation details', true),
    ('perm_members_read',       'members:read',        'members',      'read',   'Access',  'List organisation members', true),
    ('perm_members_invite',     'members:invite',      'members',      'invite', 'Access',  'Invite users to the organisation', true),
    ('perm_members_remove',     'members:remove',      'members',      'remove', 'Access',  'Remove members from the organisation', true),
    ('perm_roles_read',         'roles:read',          'roles',        'read',   'Access',  'View roles and permissions', true),
    ('perm_roles_manage',       'roles:manage',        'roles',        'manage', 'Access',  'Create and update roles', true),
    ('perm_billing_read',       'billing:read',        'billing',      'read',   'Billing', 'View billing and subscription', true),
    ('perm_billing_manage',     'billing:manage',      'billing',      'manage', 'Billing', 'Manage billing and subscription', true);

INSERT INTO entitlements (id, key, description) VALUES
    ('ent_sso',        'sso',        'Single sign-on'),
    ('ent_api_access', 'api_access', 'API access');

INSERT INTO plans (id, key, name, status) VALUES
    ('plan_trial', 'trial', 'Trial', 'active');

INSERT INTO plan_entitlements (plan_id, entitlement_id) VALUES
    ('plan_trial', 'ent_api_access');
