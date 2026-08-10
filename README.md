# KYC

**The system of record for organisations** (customers in the business sense).

One place to create and manage organisations, their users, authorisation, and billing — so tenant state lives here, not scattered across auth providers, Stripe dashboards, and spreadsheets.

**For merchants:** configure in KYC, enforce in your backend, collect customer profile data with your UI or ours — we store the record. See [How merchants integrate](#how-merchants-integrate).

## Contents

1. [What this is](#what-this-is)
2. [How merchants integrate](#how-merchants-integrate)
3. [Design principles](#design-principles)
4. [Specs](#specs)
5. [Deploy](#deploy)
6. [Status](#status)
7. [Run locally](#run-locally)
8. [Test](#test)
9. [Non-goals (v1)](#non-goals-v1)
10. [Layout](#layout)

## What this is

KYC owns the organisation lifecycle:

| Domain | Responsibility |
| --- | --- |
| **Organisations** | Tenants as first-class records — identity, status, relationships |
| **Users / Members** | People who log into KYC and belong to orgs via membership + RBAC |
| **App users** | Org-scoped end users of the merchant product; profile schema with sections |
| **Authorisation** | Roles and permissions scoped to the organisation |
| **Billing / entitlements** | KYC plans (platform capabilities) + merchant product features/plans for their customers |

To **use this app**, people must log in. Whether KYC later sells auth services to customers is a separate product question.

## How merchants integrate

How **merchants** (KYC customers) use KYC to deliver features to **their customers** (app users).

### Positioning

> **Configure in KYC. Enforce in your backend. Collect customer profile data with our form (or yours). We store the record — or project it from yours.**

KYC is **not** their login provider (not Clerk/Auth0 for their app). KYC is the **system of record** for org config, product packaging, and lifecycle automation. Customer profiles default to KYC-owned, but an org can set **external** authority and ingest users/attributes via API (`PUT …/app-users/ingest`), with optional schema discovery.

### Actors

| Who | Who they are | Where they work |
| --- | --- | --- |
| **KYC** | This platform | KYC product |
| **Merchant** | Your customer (the organisation) | KYC admin + their backend |
| **Operator** | Merchant teammate (member + role) | KYC UI |
| **Customer** | Merchant’s end user | Merchant’s app → stored as **app user** in KYC |

| Concept | Gates |
| --- | --- |
| **Permissions** | What an *operator* may do inside KYC |
| **Platform capabilities** | What the *org* may use inside KYC (from KYC plan) |
| **Product features** | What the org may unlock for *their customers* (from product plan + KYC plan product keys) |

### North-star journey

```text
1. Merchant joins KYC
2. Configures product surface (schema, features, messages, rules)
3. Connects their app (org API key + optional SDK)
4. Customers use the merchant app
5. Merchant app reads/writes KYC and gates features
6. KYC runs automations / email on those events
```

```mermaid
sequenceDiagram
  participant Op as Merchant operator
  participant KYC as KYC admin/API
  participant App as Merchant backend
  participant Cust as Customer

  Op->>KYC: Create org, attributes, features, plan, automation
  Op->>KYC: Create org API key (API keys)
  App->>KYC: Integrate with key / SDK

  Cust->>App: Sign up (merchant auth)
  App->>KYC: Create app_user (+ attributes)
  KYC->>KYC: Automation e.g. welcome email

  Cust->>App: Open settings
  App->>KYC: Load schema / save profile (API, headless SDK, or embed)
  Cust->>App: Use premium feature
  App->>KYC: entitlements.check
  KYC-->>App: allowed / denied
```

## Design principles

- **Organisation is the hub.** Users, authz, billing, and product packaging hang off the organisation record.
- **Login required (to KYC).** Session tokens authenticate operators; **org API keys** authenticate the merchant backend; a break-glass env token exists only for recovery.
- **Tenancy by membership.** Normal users can only access organisations they belong to.
- **Permissions ≠ entitlements.** Permissions gate what an *operator* may do in KYC; entitlements gate what the *organisation* may use — **platform capabilities** (KYC itself) vs **product features** (their customers).
- **Configure in KYC, enforce in their API.** Never trust the browser alone for feature gates.

## Specs

| Doc | Contents |
| --- | --- |
| [docs/saas-rethink.md](docs/saas-rethink.md) | SaaS gap analysis and revised roadmap |
| [docs/authentication.md](docs/authentication.md) | Principals, credentials, resolution, bootstrap |
| [docs/authorisation.md](docs/authorisation.md) | Capabilities, roles, grants, gates, invariants |
| [docs/data-model.md](docs/data-model.md) | Objects, relationships, permission catalog |
| [docs/api.md](docs/api.md) | REST `/v1` surface |
| [docs/flows.md](docs/flows.md) | Signup, invite, ops-provision, runtime checks |
| [docs/billing-plans.md](docs/billing-plans.md) | Stripe executor / KYC billing |
| [docs/automations.md](docs/automations.md) | Merchant automations (rules UI + River + Resend) |
| [docs/testing.md](docs/testing.md) | Testing expectations |
| [docs/deploy-railway.md](docs/deploy-railway.md) | Deploy Postgres + API + web + worker on Railway |

## Deploy

**Railway** is the recommended host (Go API + Postgres + logo volume). See [docs/deploy-railway.md](docs/deploy-railway.md). Per-service configs: [`railway.api.toml`](railway.api.toml), [`railway.web.toml`](railway.web.toml), [`railway.worker.toml`](railway.worker.toml) (automations).

## Status

**App login + API tenancy complete:** Google OAuth, sessions, membership-scoped org APIs, org-scoped API keys (Platform → API keys), login-gated UI.

**Merchant product surface:** app users, attributes, product features/plans, branding, email templates, automations (River + Resend), KYC billing via Stripe executor. In-app **Documentation** (OpenAPI + [variable referencing](docs/variables.md)) is under each organisation sidebar.

**Merchant access control:** merchants declare their own scope kinds, capabilities and roles for their **app users**, with role inheritance, grant them to a customer or to a **group** of customers, and read back a cached grant set to evaluate in their own backend. Configured under each organisation's **Customer access** page. See [authorisation](docs/authorisation.md#merchant-hosted-access-control).

**Merchant SDK:** the generated **transport layer** ships for Go and TypeScript ([sdk/](sdk/README.md)) — types plus a typed client for the 25 Integration API paths, regenerated from the spec by `make sdk` and drift-checked in CI.

Still later: the SDK ergonomic facade, settings embed, invite email polish, full platform-admin UI.

## Run locally

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) (recommended path)
- Or: Go **1.26+**, Node **20+**, and Docker only for Postgres

### 1. Clone and configure env

```bash
git clone https://github.com/gsarmaonline/kycapp.git
cd kycapp
cp .env.example .env   # optional; compose has local defaults
```

Fill `.env` only when you need Google OAuth, Stripe, or Resend. See [Auth & security](#auth--security) for the full variable list.

### 2. Docker (recommended)

```bash
docker compose up --build -d
```

| What | Where |
| --- | --- |
| App + API | http://localhost:8080 |
| Postgres (primary) | `localhost:5432` |
| Postgres (observability) | `localhost:5433` |

- Sign in with **dev login** (compose sets `AUTH_DEV_LOGIN=true`) or set `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` for real OAuth.
- Platform/ops scripts can use `API_TOKENS` (default `dev-local-token`).
- The UI stores a session Bearer token; nginx forwards `Authorization`.

```bash
docker compose down
```

### 3. Local (API + UI without Docker for the app)

```bash
# Postgres only
docker compose up -d postgres postgres-obs

export DATABASE_URL='postgres://kyc:kyc@localhost:5432/kyc?sslmode=disable'
export OBSERVABILITY_DATABASE_URL='postgres://kyc:kyc@localhost:5433/kyc_obs?sslmode=disable'
export AUTH_DEV_LOGIN=true
# Optional: source .env instead, or set Google OAuth / API_TOKENS / PLATFORM_ADMIN_EMAILS

go run ./cmd/api

# Separate terminal — UI at http://localhost:5173
cd web && npm install && npm run dev
```

### Auth & security

| Env | Purpose |
| --- | --- |
| `DATABASE_URL` | Primary Postgres (orgs, plans, entitlements) |
| `OBSERVABILITY_DATABASE_URL` | Separate Postgres for activity + usage meters (optional; noop if unset) |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | Google OAuth app credentials |
| `OAUTH_REDIRECT_URL` | Must match Google console (default `http://localhost:8080/v1/auth/google/callback`) |
| `APP_ORIGIN` | Where to redirect after login with `#token=` |
| `OAUTH_STATE_SECRET` | HMAC secret for OAuth CSRF state |
| `AUTH_DEV_LOGIN` | If `true`, enables `POST /v1/auth/dev-login` for local/tests (**never in prod**) |
| `API_TOKENS` | Comma-separated **platform** service tokens |
| `PLATFORM_ADMIN_EMAILS` | Emails treated as KYC staff. Evaluated per request, so removing an address demotes immediately |
| `CHECK_RATE_LIMIT_PER_MIN` | Max check-endpoint calls per actor/minute (default 120; `0` disables) |
| `AUTH_RATE_LIMIT_PER_MIN` | Max OAuth/dev-login starts per IP/minute (default 20; `0` disables) |
| `UPLOAD_DIR` | Local directory for org logo files (default `data/uploads`) |
| `PUBLIC_BASE_URL` | Absolute origin for public logo URLs (default `APP_ORIGIN`) |
| `PAYMENTS_PROVIDER` | `noop` (default) or `stripe` |
| `STRIPE_SECRET_KEY` | Stripe API secret (required when provider is `stripe`) |
| `STRIPE_WEBHOOK_SECRET` | Webhook signing secret |
| `STRIPE_SUCCESS_URL` / `STRIPE_CANCEL_URL` | Optional Checkout return URL defaults |
| `EMAIL_PROVIDER` | `noop` (default, log only) or `resend` |
| `RESEND_API_KEY` | Resend API key (required when provider is `resend`) |
| `EMAIL_FROM` | From address, e.g. `KYC <mail@yourdomain.com>` (verified in Resend) |

| Principal | Can do |
| --- | --- |
| Operator — user session (Google or dev-login) | Own profile, plus the organisations they belong to, gated by their role |
| Staff — user session, member of the platform organisation | Every organisation, but only what their role's permissions allow |
| Org API key (Platform → API keys) | That organisation only, and never more than its **owner** can do; requires `api_access` |
| Recovery credential (`kyc_recovery_…`) | Everything, minted by staff with a reason and a required expiry, revocable |
| Last resort — unscoped `API_TOKENS` service token | Everything. Normally unset; the only credential that survives a broken database |

Staff are ordinary members of a seeded platform organisation, so reach is derived from membership rather than from a flag. See [access control](docs/authentication.md#four-callers).

Public (no Bearer): `GET /v1/auth/providers`, `GET /v1/auth/google`, `GET /v1/auth/google/callback`, `POST /v1/auth/dev-login` (if enabled), `GET /v1/public/organisations/{id}/branding/logo`, health endpoints.

**Human login is Google-only.** Create an organisation after sign-in via `POST /v1/organisations`. Invited users sign in with Google using the invited email to claim the account.

## Test

```bash
make test-go    # Go unit + integration (Docker for Testcontainers)
make test-e2e   # Local API happy-path e2e (Docker Postgres; noop Stripe + recording mailer)
make test-web   # Vitest for UI
make test       # both
```

Regenerate sqlc after query changes:

```bash
make sqlc
```

## Non-goals (v1)

- Not a full CRM
- Not Auth0/Clerk-for-your-customers (auth-as-a-service)
- Not a payment processor (Stripe is the executor; KYC owns access state)
- No separate Account entity yet

## Layout

```
cmd/api/                 # HTTP server entrypoint
core/                    # domain helpers
internal/authn/          # request principal
internal/config/         # env config
internal/http/           # HTTP handlers + auth middleware
internal/service/        # application services
internal/store/          # Postgres, migrations, sqlc queries
web/                     # Vite + React app (login-gated)
sdk/go/ sdk/ts/          # merchant SDK transport (generated from the spec)
docs/                    # data model, API, flows, testing
```
