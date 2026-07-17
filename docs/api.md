# API

Base path: `/v1`. JSON request/response bodies.

Related: [data model](data-model.md) · [flows](flows.md)

## Conventions

| Concern | Rule |
| --- | --- |
| IDs | ULID or UUID strings |
| Errors | `{ "error": { "code": string, "message": string } }` |
| Pagination | `?limit=&cursor=` on list endpoints |
| Idempotency | `Idempotency-Key` header on signup, membership invite, subscription upsert |
| Auth (v1) | Bearer **session** (`kyc_sess_…`) or **service/platform** token; humans sign in with **Google OAuth** |

---

## App auth

Public (no Bearer): `GET /v1/auth/providers`, `GET /v1/auth/google`, `GET /v1/auth/google/callback`, and (if enabled) `POST /v1/auth/dev-login`.

All other `/v1/*` require `Authorization: Bearer <token>`.

| Endpoint | Purpose |
| --- | --- |
| `GET /v1/auth/providers` | `{ "google": bool, "dev_login": bool }` |
| `GET /v1/auth/google` | Start Google OAuth (redirect) |
| `GET /v1/auth/google/callback` | OAuth callback → issue session → redirect to `APP_ORIGIN/#token=…` |
| `POST /v1/auth/dev-login` | Local/test only when `AUTH_DEV_LOGIN=true` — `{ "email", "name?" }` |
| `POST /v1/auth/logout` | Revoke current session |
| `GET /v1/me` | Current user + memberships |

**Session response** includes `token`, `expires_at`, `user`.

**Tenancy:** user sessions may only access organisations with an **active** membership (plus RBAC on mutations). Service tokens and `platform_admin` users bypass membership for platform ops. Plan catalog writes, API keys, audit, and entitlement overrides are **platform-only**.

**Onboarding:** sign in with Google, then `POST /v1/organisations` (caller becomes owner + trial). Invited users sign in with Google using the invited email to link `google_sub` and accept the invite.

### `POST /v1/memberships/{id}/accept`

Invitee accepts (must be logged in as the invited user): membership `invited` → `active`.

**Response** `200` — updated membership.

---

## Organisations

### `POST /v1/organisations`

Create `{ "name": string, "slug": string? }`. Authenticated **users** become owner and get a trial subscription. Service/platform tokens may create orgs without an owner membership.

**Response** `201` — organisation.

### `GET /v1/organisations`

List. Users see only orgs they belong to; platform/service see all. Query: `status`, `q`, `limit`, `cursor`.

### `GET /v1/organisations/{id}`

Get one.

### `PATCH /v1/organisations/{id}`

Update `{ "name"?, "status"? }`.

### `POST /v1/organisations/{id}/archive`

Soft-archive (`status=archived`).

---

## Users

### `POST /v1/users`

Create `{ "email": string, "name": string }`.

### `GET /v1/users`

List / search. Query: `q`, `limit`, `cursor`.

### `GET /v1/users/{id}`

Get one.

### `PATCH /v1/users/{id}`

Update `{ "name"?, "status"? }`.

### `GET /v1/users/{id}/memberships`

Organisations this person belongs to (memberships with org summary).

---

## Memberships

### `POST /v1/organisations/{id}/memberships`

Invite or attach. Body: `{ "user_id"?: string, "email"?: string, "role_id": string }`.

One of `user_id` or `email` required. Creates a user shell if email is unknown. Status starts as `invited` unless policy sets `active` for ops-provisioned users.

**Headers:** `Idempotency-Key` recommended.

### `GET /v1/organisations/{id}/memberships`

List members of an organisation.

### `PATCH /v1/memberships/{id}`

Change `{ "role_id"?, "status"? }`.

### `DELETE /v1/memberships/{id}`

Revoke (`status=revoked`) or hard-remove per implementation; prefer revoke.

---

## Authorisation

### `GET /v1/permissions`

Permission catalog. Query: `category`, `resource`.

### `GET /v1/permissions/{key}`

Get one permission by key (e.g. `members:invite`).

### `POST /v1/organisations/{id}/roles`

Create role `{ "key", "name", "description"?, "permission_keys": string[] }`.

### `GET /v1/organisations/{id}/roles`

List roles (include permission keys).

### `PATCH /v1/roles/{id}`

Update `{ "name"?, "description"?, "permission_keys"? }`. System roles may reject unsafe edits (e.g. deleting `owner` or stripping all permissions).

### `POST /v1/authz/check`

Check whether a user has a permission in an organisation.

**Request** (either form)

```json
{ "organisation_id": "...", "user_id": "...", "permission": "members:invite" }
```

```json
{ "organisation_id": "...", "user_id": "...", "resource": "members", "action": "invite" }
```

**Response** `200`

```json
{ "allowed": true }
```

Requires an **active** membership. Suspended orgs / revoked memberships → `allowed: false`.

---

## Billing / entitlements

### Plans

- `POST /v1/plans` — `{ "key", "name" }`
- `GET /v1/plans`
- `GET /v1/plans/{id}`
- `PUT /v1/plans/{id}/entitlements` — `{ "entitlement_keys": string[] }` replace set

### Entitlements (catalog)

- `POST /v1/entitlements` — `{ "key", "description" }`
- `GET /v1/entitlements`

### Organisation subscription

- `PUT /v1/organisations/{id}/subscription` — `{ "plan_id", "status"? }` upsert (`Idempotency-Key` recommended)
- `GET /v1/organisations/{id}/subscription`

### Organisation entitlement overrides

- `PUT /v1/organisations/{id}/entitlements` — `{ "overrides": [{ "key", "effect": "grant"|"deny" }] }`
- `GET /v1/organisations/{id}/entitlements` — **effective** set (plan ∪ grants − denies)

### `POST /v1/entitlements/check`

**Request**

```json
{ "organisation_id": "...", "entitlement": "sso" }
```

**Response** `200`

```json
{ "allowed": true }
```

---

## Auth & hardening

```http
Authorization: Bearer <session-or-service-token>
```

`/healthz` and `/readyz` stay public. Auth/signup routes are public; everything else under `/v1` requires a valid Bearer.

### API keys (platform only)

- `POST /v1/api-keys` — `{ "name" }` → returns `{ token }` once (store hashed)
- `GET /v1/api-keys`
- `DELETE /v1/api-keys/{id}` — revoke

Env `API_TOKENS` are bootstrap service principals (same privilege as platform).

### Audit (platform only)

- `GET /v1/audit-events` — recent mutating `/v1` requests (`actor`, `method`, `path`, `status_code`)

### Rate limits

- Check endpoints: `CHECK_RATE_LIMIT_PER_MIN` per actor
- Login/OAuth starts: `AUTH_RATE_LIMIT_PER_MIN` per IP

Exceeding either returns `429` with `rate_limited`.

---

## Error codes (illustrative)

| code | When |
| --- | --- |
| `not_found` | Unknown id |
| `conflict` | Unique violation (email, slug, membership) |
| `validation_error` | Bad body / missing fields |
| `idempotency_conflict` | Same key, different body |
| `unauthorized` | Missing/invalid bearer token |
| `forbidden` | Authenticated but not allowed (tenancy / RBAC / platform) |
| `rate_limited` | Auth or check endpoint quota exceeded |
