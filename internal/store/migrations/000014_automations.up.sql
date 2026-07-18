CREATE TABLE automations (
    id               TEXT PRIMARY KEY,
    organisation_id  TEXT NOT NULL REFERENCES organisations (id),
    name             TEXT NOT NULL DEFAULT '',
    trigger          TEXT NOT NULL,
    enabled          BOOLEAN NOT NULL DEFAULT true,
    conditions       JSONB NOT NULL DEFAULT '{"all":[]}'::jsonb,
    actions          JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX automations_org_trigger_idx
    ON automations (organisation_id, trigger)
    WHERE enabled = true;

CREATE TABLE automation_runs (
    id               TEXT PRIMARY KEY,
    organisation_id  TEXT NOT NULL REFERENCES organisations (id),
    automation_id    TEXT NOT NULL REFERENCES automations (id) ON DELETE CASCADE,
    trigger          TEXT NOT NULL,
    status           TEXT NOT NULL CHECK (status IN ('success', 'skipped', 'error')),
    detail           TEXT NOT NULL DEFAULT '',
    payload          JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX automation_runs_org_created_idx
    ON automation_runs (organisation_id, created_at DESC);

CREATE INDEX automation_runs_automation_idx
    ON automation_runs (automation_id, created_at DESC);

INSERT INTO permissions (id, key, resource, action, category, description, is_system) VALUES
    ('perm_automations_read',   'automations:read',   'automations', 'read',   'Automations', 'View automations', true),
    ('perm_automations_manage', 'automations:manage', 'automations', 'manage', 'Automations', 'Create and update automations', true);

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.key IN ('owner', 'admin')
  AND p.key IN ('automations:read', 'automations:manage')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.key = 'member'
  AND p.key = 'automations:read'
ON CONFLICT DO NOTHING;
