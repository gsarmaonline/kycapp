# Deploy on Railway

Recommended layout: **Postgres** + **api** (`kycapp`) + **web** (`kycweb`) (web is the public URL; it proxies `/v1` to the API).

Merchant automations use a **worker** (`kycworker`) on the same Postgres (River) — see [automations.md](automations.md).

## Auto-deploy from GitHub

`kycapp`, `kycweb`, and `kycworker` deploy from **`gsarmaonline/kycapp` `main`**. A push to `main` triggers a new deployment for each connected service.

Each service needs a distinct Dockerfile. Prefer **Settings → Config-as-code** paths (below). As a fallback, these variables are also set:

| Service | `RAILWAY_DOCKERFILE_PATH` |
| --- | --- |
| `kycapp` | `Dockerfile.api` |
| `kycweb` | `Dockerfile.web` |
| `kycworker` | `Dockerfile.worker` |

Reconnect later with:

```bash
railway link   # project kycapp / production
railway service source connect --repo gsarmaonline/kycapp --branch main --service kycapp
railway service source connect --repo gsarmaonline/kycapp --branch main --service kycweb
railway service source connect --repo gsarmaonline/kycapp --branch main --service kycworker
```

Manual upload (`railway up`) still works and does **not** require a git push.

The web UI shows a short git SHA under the signed-in user (`build <sha>`). Railway injects `RAILWAY_GIT_COMMIT_SHA` at image build time into `VITE_GIT_SHA` (see `Dockerfile.web`). Local Vite shows `build dev`.

## Config as code

There is **no** single root `railway.toml` on purpose: Railway applies that file to every service, and this repo has multiple Dockerfiles.

| Service | Config file |
| --- | --- |
| api (`kycapp`) | [`railway.api.toml`](../railway.api.toml) → `Dockerfile.api` |
| web (`kycweb`) | [`railway.web.toml`](../railway.web.toml) → `Dockerfile.web` |
| worker (`kycworker`) | [`railway.worker.toml`](../railway.worker.toml) → `Dockerfile.worker` |

For each service: **Settings → Config-as-code** → set the config file path to `/railway.api.toml`, `/railway.web.toml`, or `/railway.worker.toml`. Leave **Root Directory** empty so the Docker build context stays the repo root. That also enables `watchPatterns` so unrelated paths do not rebuild every service.

## 1. Create a project

1. [Railway](https://railway.app) → New Project → **Deploy from GitHub** (this repo).
2. Add a **PostgreSQL** plugin/service.
3. Add empty **api** and **web** services from the same repo; attach the config files above.

## 2. API service

- **Config file:** `/railway.api.toml`  
- **Variables** (link `DATABASE_URL` from the Postgres service):

| Variable | Value |
| --- | --- |
| `DATABASE_URL` | `${{Postgres.DATABASE_URL}}` (Railway reference) |
| `OBSERVABILITY_DATABASE_URL` | `${{Postgres-Obs.DATABASE_URL}}` (second Postgres service) |
| `AUTH_DEV_LOGIN` | `false` |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | from Google Cloud Console |
| `OAUTH_STATE_SECRET` | long random string |
| `API_TOKENS` | long random platform token(s) |
| `UPLOAD_DIR` | `/data/uploads` |
| `CORS_ORIGIN` | public web URL (set after web is live), e.g. `https://web-production-xxxx.up.railway.app` |
| `APP_ORIGIN` | same as public web URL |
| `PUBLIC_BASE_URL` | same as public web URL (logos go through nginx `/v1/public/...`) |
| `OAUTH_REDIRECT_URL` | `{APP_ORIGIN}/v1/auth/google/callback` |
| `EMAIL_PROVIDER` | `noop` (default) or `resend` |
| `RESEND_API_KEY` | Resend API key (required if `EMAIL_PROVIDER=resend`) |
| `EMAIL_FROM` | e.g. `KYC <mail@yourdomain.com>` (verified domain in Resend) |

`PORT` is set by Railway; the API listens on it automatically.

**Volume:** mount a volume at `/data/uploads` so logos survive redeploys. Set `UPLOAD_DIR=/data/uploads`. The API entrypoint `chown`s that path for the non-root process — without a volume (or with a root-only mount), logo `POST` returns 500.

**Networking:** private network is enough for the API (no public domain required if web proxies). Note the service name (e.g. `api`) and the port Railway assigns (`PORT`).

## 3. Web service

- **Config file:** `/railway.web.toml`  
- **Variables:**

| Variable | Value |
| --- | --- |
| `API_UPSTREAM` | `kycapp.railway.internal:8080` (pin API with `HTTP_ADDR=:8080`; `${{kycapp.PORT}}` is often empty) |
| `RAILWAY_DOCKERFILE_PATH` | `Dockerfile.web` |

Generate a **public domain** only on **web** (`kycweb`; keep API private). That URL is your `APP_ORIGIN` / `PUBLIC_BASE_URL` / `CORS_ORIGIN` (e.g. `https://kycweb-production.up.railway.app`).

## 4. Security checklist

- **Do not** give the API a public domain; only web is public and proxies `/v1`.
- Set `AUTH_DEV_LOGIN=false` once Google OAuth works. Dev-login is for bootstrap only.
- Use long random `OAUTH_STATE_SECRET` and `API_TOKENS` (never commit them).
- Mount a volume at `UPLOAD_DIR=/data/uploads` for logos.
- Lock `CORS_ORIGIN` / `APP_ORIGIN` / `PUBLIC_BASE_URL` to the web HTTPS origin.

## 5. Google OAuth

In Google Cloud Console → OAuth client → Authorized redirect URIs, add:

`https://<your-web-domain>/v1/auth/google/callback`

Then set `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` on the API service and set `AUTH_DEV_LOGIN=false`.

## 5b. Billing (Stripe executor)

Default is `PAYMENTS_PROVIDER=noop` (Checkout/Portal unavailable; comps via `PUT …/subscription`).

To enable Stripe on the API service:

| Variable | Value |
| --- | --- |
| `PAYMENTS_PROVIDER` | `stripe` |
| `STRIPE_SECRET_KEY` | Stripe secret key |
| `STRIPE_WEBHOOK_SECRET` | Endpoint signing secret |
| `STRIPE_SUCCESS_URL` / `STRIPE_CANCEL_URL` | Optional; defaults to `/orgs/{id}/billing` on `APP_ORIGIN` |

Point a Stripe webhook at `https://<api-or-web-domain>/v1/billing/webhooks/stripe` (events: `checkout.session.completed`, `customer.subscription.*`, `invoice.paid`, `invoice.payment_failed`). Link each sellable plan with `PUT /v1/plans/{id}/price` (`processor_price_ref` = Stripe Price id).

## 6. Smoke check

1. Open the web public URL → sign in (Google, or Dev login while enabled).  
2. Create an organisation.  
3. Branding → upload a logo → confirm it appears in an email template preview.

## 7. Automation worker (optional until you use Automations)

- Add a **worker** service; Config-as-code → `/railway.worker.toml`.
- Set `DATABASE_URL=${{Postgres.DATABASE_URL}}` (same DB as API). No public domain.
- For real email: `EMAIL_PROVIDER=resend`, `RESEND_API_KEY`, `EMAIL_FROM` (and `PUBLIC_BASE_URL` so branded logos resolve in HTML).

## Local compose vs Railway

| Concern | Compose | Railway |
| --- | --- | --- |
| API upstream for nginx | `api:8080` (default) | `API_UPSTREAM=…railway.internal:PORT` |
| Listen port (web) | `80` | Railway `PORT` |
| Listen port (api) | `:8080` | Railway `PORT` |
| Logos | named volume | attach volume at `UPLOAD_DIR` |
| Automations worker | compose `worker` | `/railway.worker.toml` + same `DATABASE_URL` |
