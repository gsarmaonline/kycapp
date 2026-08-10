-- KYC becomes an organisation in its own system. Staff are members of it with
-- ordinary roles, so there is no separate staff mechanism to reason about and
-- no role names in application code.

-- A role may reach every organisation. This is a property of the role row, not
-- of any particular role name: authorisation code never asks "is this root?",
-- it asks whether the role carries global reach.
--
-- Invariant 4 (global scope is issued, never assigned) is enforced in the
-- service layer: only a principal that already holds global reach may set this.
ALTER TABLE roles ADD COLUMN IF NOT EXISTS grants_global_reach BOOLEAN NOT NULL DEFAULT FALSE;

-- Membership may be time-boxed. This is what makes staff access to a merchant
-- an event with an end rather than standing permission, and merchants get the
-- same capability for contractors.
ALTER TABLE memberships ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS memberships_expires_at_idx
    ON memberships (expires_at) WHERE expires_at IS NOT NULL;

-- Single-row table holding pointers the application needs but must not hardcode.
CREATE TABLE IF NOT EXISTS system_state (
    id                      INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    platform_organisation_id TEXT REFERENCES organisations (id),
    bootstrap_role_id       TEXT REFERENCES roles (id),
    -- Set once, when the first staff member is minted. Gated on this marker
    -- rather than on "are there zero global grants", so revoking every staff
    -- membership cannot reopen the bootstrap door.
    bootstrapped_at         TIMESTAMPTZ
);

INSERT INTO organisations (id, name, slug, status)
VALUES ('org_platform', 'KYC', 'kyc-platform', 'active')
ON CONFLICT (id) DO NOTHING;

-- Platform roles are seed data, exactly like a merchant's own roles.
INSERT INTO roles (id, organisation_id, key, name, description, is_system, grants_global_reach)
VALUES
    ('role_platform_root',    'org_platform', 'root',    'Root',    'Full reach over every organisation', true, true),
    ('role_platform_support', 'org_platform', 'support', 'Support', 'Read-only reach over every organisation', true, true),
    ('role_platform_billing', 'org_platform', 'billing', 'Billing', 'Billing administration across organisations', true, true)
ON CONFLICT (organisation_id, key) DO NOTHING;

-- Root holds every permission in the catalog.
INSERT INTO role_permissions (role_id, permission_id)
SELECT 'role_platform_root', id FROM permissions
ON CONFLICT DO NOTHING;

-- Support is read-only: every permission whose action is a read.
INSERT INTO role_permissions (role_id, permission_id)
SELECT 'role_platform_support', id FROM permissions WHERE action = 'read'
ON CONFLICT DO NOTHING;

-- Billing covers the billing surface plus organisation reads.
INSERT INTO role_permissions (role_id, permission_id)
SELECT 'role_platform_billing', id FROM permissions
WHERE resource = 'billing' OR key = 'organisation:read'
ON CONFLICT DO NOTHING;

INSERT INTO system_state (id, platform_organisation_id, bootstrap_role_id)
VALUES (1, 'org_platform', 'role_platform_root')
ON CONFLICT (id) DO UPDATE
SET platform_organisation_id = EXCLUDED.platform_organisation_id,
    bootstrap_role_id = EXCLUDED.bootstrap_role_id;
