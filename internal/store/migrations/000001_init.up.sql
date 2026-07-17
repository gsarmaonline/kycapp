-- Phase 1 schema: organisations hub + users, authz, billing

CREATE TABLE organisations (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL UNIQUE,
    status      TEXT NOT NULL CHECK (status IN ('active', 'suspended', 'archived')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id          TEXT PRIMARY KEY,
    email       TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    status      TEXT NOT NULL CHECK (status IN ('active', 'disabled')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE permissions (
    id          TEXT PRIMARY KEY,
    key         TEXT NOT NULL UNIQUE,
    resource    TEXT NOT NULL,
    action      TEXT NOT NULL,
    category    TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    is_system   BOOLEAN NOT NULL DEFAULT true,
    UNIQUE (resource, action)
);

CREATE TABLE roles (
    id               TEXT PRIMARY KEY,
    organisation_id  TEXT NOT NULL REFERENCES organisations (id),
    key              TEXT NOT NULL,
    name             TEXT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    is_system        BOOLEAN NOT NULL DEFAULT false,
    UNIQUE (organisation_id, key)
);

CREATE TABLE role_permissions (
    role_id        TEXT NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    permission_id  TEXT NOT NULL REFERENCES permissions (id),
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE memberships (
    id               TEXT PRIMARY KEY,
    organisation_id  TEXT NOT NULL REFERENCES organisations (id),
    user_id          TEXT NOT NULL REFERENCES users (id),
    role_id          TEXT NOT NULL REFERENCES roles (id),
    status           TEXT NOT NULL CHECK (status IN ('invited', 'active', 'revoked')),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organisation_id, user_id)
);

CREATE INDEX memberships_user_id_idx ON memberships (user_id);
CREATE INDEX memberships_organisation_id_idx ON memberships (organisation_id);

CREATE TABLE plans (
    id      TEXT PRIMARY KEY,
    key     TEXT NOT NULL UNIQUE,
    name    TEXT NOT NULL,
    status  TEXT NOT NULL CHECK (status IN ('active', 'archived'))
);

CREATE TABLE entitlements (
    id           TEXT PRIMARY KEY,
    key          TEXT NOT NULL UNIQUE,
    description  TEXT NOT NULL DEFAULT ''
);

CREATE TABLE plan_entitlements (
    plan_id         TEXT NOT NULL REFERENCES plans (id) ON DELETE CASCADE,
    entitlement_id  TEXT NOT NULL REFERENCES entitlements (id),
    PRIMARY KEY (plan_id, entitlement_id)
);

CREATE TABLE subscriptions (
    id                   TEXT PRIMARY KEY,
    organisation_id      TEXT NOT NULL UNIQUE REFERENCES organisations (id),
    plan_id              TEXT NOT NULL REFERENCES plans (id),
    status               TEXT NOT NULL CHECK (status IN ('trialing', 'active', 'past_due', 'canceled')),
    current_period_end   TIMESTAMPTZ
);

CREATE TABLE organisation_entitlements (
    organisation_id  TEXT NOT NULL REFERENCES organisations (id),
    entitlement_id   TEXT NOT NULL REFERENCES entitlements (id),
    effect           TEXT NOT NULL CHECK (effect IN ('grant', 'deny')),
    PRIMARY KEY (organisation_id, entitlement_id)
);
