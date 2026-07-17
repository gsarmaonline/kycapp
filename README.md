# KYC

**The system of record for organisations** (customers in the business sense).

One place to create and manage organisations, their users, authorisation, and billing — so tenant state lives here, not scattered across auth providers, Stripe dashboards, and spreadsheets.

## What this is

KYC owns the organisation lifecycle:

| Domain | Responsibility |
| --- | --- |
| **Organisations** | Tenants as first-class records — identity, status, relationships |
| **Users** | People who can belong to many organisations via membership |
| **Authorisation** | Roles and permissions scoped to the organisation |
| **Billing** | Plans, entitlements, and subscription state tied to the organisation |

Auth providers and payment processors are integrations. This repo is the source of truth.

## Who it's for

- **Ops / platform teams** — provision organisations, users, plans
- **Product services** — call authz and entitlement checks at runtime

## Design principles

- **Organisation is the hub.** Users, authz, and billing hang off the organisation record.
- **Own the model, integrate the rails.** Stripe moves money; an IdP may authenticate; KYC decides who the organisation is and what it's entitled to.
- **Permissions ≠ entitlements.** Permissions gate what a *user* may do; entitlements gate what an *organisation* may use on its plan.
- **Entitlements over plan checks.** Apps ask "can this organisation do X?" — not "are they on plan Pro?"

## Specs

| Doc | Contents |
| --- | --- |
| [docs/data-model.md](docs/data-model.md) | Objects, relationships, permission catalog |
| [docs/api.md](docs/api.md) | REST `/v1` surface |
| [docs/flows.md](docs/flows.md) | Signup, invite, ops-provision, runtime checks |
| [docs/testing.md](docs/testing.md) | Testing expectations |

## Status

**Phase 4 complete:** plans, entitlements, subscriptions, overrides, and `POST /v1/entitlements/check`, plus ops billing panel.

Next: Phase 5 — service auth, audit log, hardening.

## Run locally

### Docker (recommended)

```bash
# Stop any local process already using :8080, then:
docker compose up --build -d
```

- **Ops UI + API:** http://localhost:8080  
  (nginx serves the UI and proxies `/v1`, `/healthz`, `/readyz` to the API)
- Postgres: `localhost:5432`

```bash
docker compose down
```

### Local (without Docker for the app)

```bash
docker compose up -d postgres

export DATABASE_URL='postgres://kyc:kyc@localhost:5432/kyc?sslmode=disable'
go run ./cmd/api

# Ops UI (separate terminal) — http://localhost:5173
cd web && npm run dev
```

## Test

```bash
make test-go    # Go unit + integration (Docker for Testcontainers)
make test-web   # Vitest for ops UI
make test       # both
```

Regenerate sqlc after query changes:

```bash
make sqlc
```

## Non-goals (v1)

- Not a full CRM (pipeline, deals, marketing) — may grow later on top of Organisation
- Not a general-purpose IdP
- Not a payment processor
- No separate Account entity yet

## Layout

```
cmd/api/                 # HTTP server entrypoint
core/                    # domain helpers
internal/config/         # env config
internal/http/           # HTTP handlers
internal/service/        # application services
internal/store/          # Postgres, migrations, sqlc queries
web/                     # Vite + React ops console
docs/                    # data model, API, flows, testing
```
