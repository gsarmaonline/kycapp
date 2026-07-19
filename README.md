# KYC

**The system of record for organisations** (customers in the business sense).

One place to create and manage organisations, their users, authorisation, and billing — so tenant state lives here, not scattered across auth providers, Stripe dashboards, and spreadsheets.

## What this is

KYC owns the organisation lifecycle:

| Domain | Responsibility |
| --- | --- |
| **Organisations** | Tenants as first-class records — identity, status, relationships |
| **Users / Members** | People who log into KYC and belong to orgs via membership + RBAC |
| **App users** | Org-scoped end users of the merchant product; profile schema with sections |
| **Authorisation** | Roles and permissions scoped to the organisation |
| **Billing** | Plans, entitlements, and subscription state tied to the organisation |

To **use this app**, people must log in. Whether KYC later sells auth services to customers is a separate product question.

## Design principles

- **Organisation is the hub.** Users, authz, and billing hang off the organisation record.
- **Login required.** Session tokens authenticate humans; service API keys are for platform/integrations only.
- **Tenancy by membership.** Normal users can only access organisations they belong to.
- **Permissions ≠ entitlements.** Permissions gate what a *user* may do; entitlements gate what an *organisation* may use on its plan.

## Specs

| Doc | Contents |
| --- | --- |
| [docs/saas-rethink.md](docs/saas-rethink.md) | SaaS gap analysis and revised roadmap |
| [docs/data-model.md](docs/data-model.md) | Objects, relationships, permission catalog |
| [docs/api.md](docs/api.md) | REST `/v1` surface |
| [docs/flows.md](docs/flows.md) | Signup, invite, ops-provision, runtime checks |
| [docs/testing.md](docs/testing.md) | Testing expectations |
| [docs/deploy-railway.md](docs/deploy-railway.md) | Deploy Postgres + API + web on Railway |
| [docs/automations.md](docs/automations.md) | Merchant automations (rules UI + River) |

## Deploy

**Railway** is the recommended host (Go API + Postgres + logo volume). See [docs/deploy-railway.md](docs/deploy-railway.md). Per-service configs: [`railway.api.toml`](railway.api.toml), [`railway.web.toml`](railway.web.toml), [`railway.worker.toml`](railway.worker.toml) (automations).

## Status

**App login + API tenancy complete:** Google OAuth (passwordless), sessions, `GET /v1/me`, membership-scoped org APIs, platform-only catalog/API-key routes, login-gated UI.

Billing v1: Stripe executor (Checkout / Portal / webhooks) behind KYC APIs — see [docs/billing-plans.md](docs/billing-plans.md). Still later: invite email delivery, full platform-admin UI.

## Run locally

### Docker (recommended)

```bash
docker compose up --build -d
```

- **App + API:** http://localhost:8080  
  Sign up (creates org + session) or sign in. The UI stores a session Bearer token; nginx forwards `Authorization`.
- Postgres: `localhost:5432`
- Optional service token for platform/ops scripts: `API_TOKENS` (default `dev-local-token`)
- Local compose enables `AUTH_DEV_LOGIN=true` so you can sign in without Google credentials. Set `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` for real OAuth.

```bash
docker compose down
```

### Local (without Docker for the app)

```bash
docker compose up -d postgres

export DATABASE_URL='postgres://kyc:kyc@localhost:5432/kyc?sslmode=disable'
# Google OAuth (required for production human login):
# export GOOGLE_CLIENT_ID=...
# export GOOGLE_CLIENT_SECRET=...
# export OAUTH_REDIRECT_URL='http://localhost:8080/v1/auth/google/callback'
# export APP_ORIGIN='http://localhost:8080'
# Local-only bypass when Google is not configured:
export AUTH_DEV_LOGIN=true
# Optional platform/service tokens:
# export API_TOKENS='my-secret'
# export PLATFORM_ADMIN_EMAILS='you@example.com'
go run ./cmd/api

# App UI — http://localhost:5173
cd web && npm run dev
```

### Auth & security

| Env | Purpose |
| --- | --- |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | Google OAuth app credentials |
| `OAUTH_REDIRECT_URL` | Must match Google console (default `http://localhost:8080/v1/auth/google/callback`) |
| `APP_ORIGIN` | Where to redirect after login with `#token=` |
| `OAUTH_STATE_SECRET` | HMAC secret for OAuth CSRF state |
| `AUTH_DEV_LOGIN` | If `true`, enables `POST /v1/auth/dev-login` for local/tests (**never in prod**) |
| `API_TOKENS` | Comma-separated **platform** service tokens |
| `PLATFORM_ADMIN_EMAILS` | Emails granted `platform_admin` on Google/dev login |
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
| User session (Google or dev-login) | Own profile, orgs they belong to, RBAC-gated mutations |
| Platform admin / service token | All orgs, plan catalog, API keys, audit, entitlement overrides |

Public (no Bearer): `GET /v1/auth/providers`, `GET /v1/auth/google`, `GET /v1/auth/google/callback`, `POST /v1/auth/dev-login` (if enabled), `GET /v1/public/organisations/{id}/branding/logo`, health endpoints.

**Human login is Google-only.** Create an organisation after sign-in via `POST /v1/organisations`. Invited users sign in with Google using the invited email to claim the account.

## Test

```bash
make test-go    # Go unit + integration (Docker for Testcontainers)
make test-web   # Vitest for UI
make test       # both
```

Regenerate sqlc after query changes:

```bash
make sqlc
```

## Non-goals (v1)

- Not a full CRM
- Not Auth0-for-your-customers (auth-as-a-service)
- Not a payment processor (Stripe later)
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
docs/                    # data model, API, flows, testing
```
