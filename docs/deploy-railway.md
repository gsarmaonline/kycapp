# Deploy on Railway

Recommended layout: **Postgres** + **api** + **web** (web is the public URL; it proxies `/v1` to the API).

## 1. Create a project

1. [Railway](https://railway.app) → New Project → **Deploy from GitHub** (this repo).
2. Add a **PostgreSQL** plugin/service.

## 2. API service

- **Root directory:** repo root  
- **Dockerfile:** `Dockerfile.api`  
- **Variables** (link `DATABASE_URL` from the Postgres service):

| Variable | Value |
| --- | --- |
| `DATABASE_URL` | `${{Postgres.DATABASE_URL}}` (Railway reference) |
| `AUTH_DEV_LOGIN` | `false` |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | from Google Cloud Console |
| `OAUTH_STATE_SECRET` | long random string |
| `API_TOKENS` | long random platform token(s) |
| `UPLOAD_DIR` | `/data/uploads` |
| `CORS_ORIGIN` | public web URL (set after web is live), e.g. `https://web-production-xxxx.up.railway.app` |
| `APP_ORIGIN` | same as public web URL |
| `PUBLIC_BASE_URL` | same as public web URL (logos go through nginx `/v1/public/...`) |
| `OAUTH_REDIRECT_URL` | `{APP_ORIGIN}/v1/auth/google/callback` |

`PORT` is set by Railway; the API listens on it automatically.

**Volume:** mount a volume at `/data/uploads` so logos survive redeploys.

**Networking:** private network is enough for the API (no public domain required if web proxies). Note the service name (e.g. `api`) and the port Railway assigns (`PORT`).

## 3. Web service

- **Root directory:** repo root  
- **Dockerfile:** `Dockerfile.web`  
- **Variables:**

| Variable | Value |
| --- | --- |
| `API_UPSTREAM` | `{api-service-name}.railway.internal:{api PORT}` — e.g. `api.railway.internal:8080` (use the API service’s private host + its `PORT`) |

Generate a **public domain** for web. That URL is your `APP_ORIGIN` / `PUBLIC_BASE_URL` / `CORS_ORIGIN`.

## 4. Google OAuth

In Google Cloud Console → OAuth client → Authorized redirect URIs, add:

`https://<your-web-domain>/v1/auth/google/callback`

## 5. Smoke check

1. Open the web public URL → sign in with Google.  
2. Create an organisation.  
3. Branding → upload a logo → confirm it appears in an email template preview.

## Local compose vs Railway

| Concern | Compose | Railway |
| --- | --- | --- |
| API upstream for nginx | `api:8080` (default) | `API_UPSTREAM=…railway.internal:PORT` |
| Listen port (web) | `80` | Railway `PORT` |
| Listen port (api) | `:8080` | Railway `PORT` |
| Logos | named volume | attach volume at `UPLOAD_DIR` |
