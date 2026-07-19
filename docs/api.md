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

Update `{ "name"?, "status"?, "primary_color"?, "accent_color"?, "email_footer"?, "email_font"? }`.  
Colors must be `#RGB` or `#RRGGBB`.  
`email_font` is one of: `arial`, `helvetica`, `verdana`, `trebuchet`, `georgia`, `times`, `courier` (email-safe stacks). Requires `organisation:update`.

Organisation JSON also includes `logo_url` (read-only; set via logo upload) and `email_font`.

### `POST /v1/organisations/{id}/archive`

Soft-archive (`status=archived`).

### Branding / logo

- `POST /v1/organisations/{id}/branding/logo` — multipart field `logo` (png/jpeg/webp, ≤1MB). Requires `organisation:update`. Sets `logo_url`.
- `DELETE /v1/organisations/{id}/branding/logo` — clears logo file and `logo_url`. Requires `organisation:update`.
- `GET /v1/public/organisations/{id}/branding/logo` — **unauthenticated** image bytes for email clients.  
  Configure `PUBLIC_BASE_URL` (and `UPLOAD_DIR`) so `logo_url` is reachable from outside.

Email template `body_html` is **inner content**. Preview/send wrap it with org chrome (header, logo, colors, footer) via `emailtemplates.Wrap` unless the body is already a full HTML document.

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

## App users & attribute schema

End users of a merchant’s product (not KYC team **members**). Profile fields are defined per organisation.

### Attribute definitions

System defaults (`phone`, `location`, `country`, …) are seeded per org on create and lazily on list (same pattern as email templates). Seeded rows are fully editable and deletable; `is_system` is informational. Re-seeding skips keys that already exist (including archived), so a deleted default is not recreated unless the row is removed.

- `POST /v1/organisations/{id}/attribute-definitions` — requires `attributes:manage`  
  `{ "key", "label", "value_type", "section"?, "sort_order"?, "required"?, "enum_values"?, "description"?, "is_pii"? }`  
  `value_type`: `string` \| `number` \| `boolean` \| `date` \| `dropdown`  
  `section`: UI grouping label (default `general`)  
  For `dropdown`, pass `enum_values` (allowed options).
- `GET /v1/organisations/{id}/attribute-definitions` — requires `attributes:read`  
  Seeds system defaults if missing. Query: `status` (`active` \| `archived`)
- `PATCH /v1/attribute-definitions/{id}` — requires `attributes:manage`  
  `{ "label"?, "description"?, "value_type"?, "section"?, "sort_order"?, "required"?, "enum_values"?, "is_pii"?, "status"? }`
- `DELETE /v1/attribute-definitions/{id}` — requires `attributes:manage`  
  Archives the definition (including seeded defaults).

### App users

- `POST /v1/organisations/{id}/app-users` — requires `app_users:write`  
  `{ "email"?, "external_id"?, "display_name"?, "status"?, "attributes"? }`  
  `attributes` keys must match active definitions; types and required fields are validated.
- `GET /v1/organisations/{id}/app-users` — requires `app_users:read`  
  Query: `status`
- `GET /v1/app-users/{id}` — requires `app_users:read`
- `PATCH /v1/app-users/{id}` — requires `app_users:write`

---

## Email templates

Org-scoped message copy for **app users** (not KYC member invites). Domain helpers live in `core/emailtemplates` for easier extraction later. No send provider in v1.

- `GET /v1/organisations/{id}/email-templates` — requires `email_templates:read`  
  Seeds system defaults (`welcome`, `payment_thank_you`, `profile_incomplete`) if missing. Query: `status`
- `POST /v1/organisations/{id}/email-templates` — requires `email_templates:manage`  
  Custom template: `{ "key", "name", "subject", "body_text"?, "body_html"?, "description"? }`  
  Placeholders: `{{display_name}}`, `{{org_name}}`, etc.  
  Store inner HTML only; branding chrome comes from organisation branding (see above). Visual builder deferred.
- `GET /v1/email-templates/{id}` — requires `email_templates:read`
- `PATCH /v1/email-templates/{id}` — requires `email_templates:manage`  
  `{ "name"?, "description"?, "subject"?, "body_text"?, "body_html"?, "status"? }`  
  Key is immutable (including system templates).
- `DELETE /v1/email-templates/{id}` — archive; requires `email_templates:manage` (system templates cannot be deleted)

---

## Automations

Org-scoped rules: trigger → simple AND conditions → ordered actions. Executed by the River worker (`cmd/worker`). Domain: `core/automations`. See [automations.md](automations.md).

- `GET /v1/organisations/{id}/automations` — requires `automations:read`
- `POST /v1/organisations/{id}/automations` — requires `automations:manage`  
  `{ "name"?, "trigger", "enabled"?, "conditions": { "all": [{ "field", "op", "value"? }] }, "actions": [{ "type": "send_email", "template_key" }] }`  
  Triggers: `app_user.created`, `app_user.updated`. Ops: `eq`, `neq`, `exists`, `not_exists`.
- `GET /v1/automations/{id}` — requires `automations:read`
- `PATCH /v1/automations/{id}` — requires `automations:manage`
- `DELETE /v1/automations/{id}` — requires `automations:manage`
- `GET /v1/organisations/{id}/automation-runs` — requires `automations:read`  
  Query: `automation_id`

Email delivery is stubbed (logged) until an ESP is wired.

---

## Billing / entitlements

See [billing-plans.md](./billing-plans.md) for the Stripe executor design.

### Plans

- `POST /v1/plans` — `{ "key", "name" }` (platform)
- `GET /v1/plans`
- `GET /v1/plans/{id}`
- `PUT /v1/plans/{id}/entitlements` — `{ "entitlement_keys": string[] }` replace set (platform)
- `PUT /v1/plans/{id}/price` — link Stripe Price (platform)  
  `{ "interval": "month"|"year", "currency"?, "unit_amount", "processor_price_ref", "status"? }`
- `GET /v1/plans/{id}/prices`

### Entitlements (catalog)

Each entitlement has `scope`: `platform` (KYC / platform capabilities) or `product` (customer product features).

- `POST /v1/entitlements` — `{ "key", "description", "scope"? }` (`scope` defaults to `platform`)
- `GET /v1/entitlements` — items include `scope`

### Organisation subscription

- `PUT /v1/organisations/{id}/subscription` — `{ "plan_id", "status"? }` upsert (platform / comps)
- `GET /v1/organisations/{id}/subscription` — requires `billing:read`

### Self-serve billing (Stripe executor)

- `POST /v1/organisations/{id}/billing/checkout` — requires `billing:manage`  
  `{ "plan_id", "interval"?, "success_url"?, "cancel_url"? }` → `{ "url" }`
- `POST /v1/organisations/{id}/billing/portal` — requires `billing:manage`  
  `{ "return_url"? }` → `{ "url" }`
- `POST /v1/billing/webhooks/{provider}` — public; `provider` must match `PAYMENTS_PROVIDER` (`stripe` or `noop`)

### Organisation entitlement overrides

- `PUT /v1/organisations/{id}/entitlements` — `{ "overrides": [{ "key", "effect": "grant"|"deny" }] }`
- `GET /v1/organisations/{id}/entitlements` — **effective** set (plan ∪ grants − denies), split by scope:

```json
{
  "entitlements": ["api_access", "sso"],
  "platform_capabilities": ["api_access", "sso"],
  "product_features": []
}
```

Plans also expose `platform_capability_keys` and `product_feature_keys` alongside flat `entitlement_keys`.

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
