-- Org-scoped email templates for app-user messaging (workflows later).

CREATE TABLE email_templates (
    id               TEXT PRIMARY KEY,
    organisation_id  TEXT NOT NULL REFERENCES organisations (id),
    key              TEXT NOT NULL,
    name             TEXT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    subject          TEXT NOT NULL,
    body_text        TEXT NOT NULL,
    body_html        TEXT NOT NULL DEFAULT '',
    status           TEXT NOT NULL CHECK (status IN ('active', 'archived')),
    is_system        BOOLEAN NOT NULL DEFAULT false,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organisation_id, key)
);

CREATE INDEX email_templates_org_idx ON email_templates (organisation_id, key);

INSERT INTO permissions (id, key, resource, action, category, description, is_system) VALUES
    ('perm_email_templates_read',   'email_templates:read',   'email_templates', 'read',   'Messaging', 'View email templates', true),
    ('perm_email_templates_manage', 'email_templates:manage', 'email_templates', 'manage', 'Messaging', 'Create and update email templates', true);

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.key IN ('owner', 'admin')
  AND p.key IN ('email_templates:read', 'email_templates:manage')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.key = 'member'
  AND p.key = 'email_templates:read'
ON CONFLICT DO NOTHING;
