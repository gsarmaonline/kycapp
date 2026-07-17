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
| Auth (v1) | Bearer token (ops or service); finer IdP later |

---

## Onboarding

### `POST /v1/signup`

Atomic create: user + organisation + owner membership + trial subscription.

**Headers:** `Idempotency-Key` (required)

**Request**

```json
{
  "user": { "email": "ada@acme.com", "name": "Ada Lovelace" },
  "organisation": { "name": "Acme Pty Ltd", "slug": "acme" },
  "plan_key": "trial"
}
```

`slug` optional (derived from name if omitted). `plan_key` defaults to `trial`.

**Response** `201`

```json
{
  "user": { "id": "...", "email": "ada@acme.com", "name": "Ada Lovelace", "status": "active" },
  "organisation": { "id": "...", "name": "Acme Pty Ltd", "slug": "acme", "status": "active" },
  "membership": { "id": "...", "organisation_id": "...", "user_id": "...", "role_id": "...", "status": "active" },
  "subscription": { "id": "...", "organisation_id": "...", "plan_id": "...", "status": "trialing" }
}
```

Also seeds system roles `owner`, `admin`, `member` on the organisation.

### `POST /v1/memberships/{id}/accept`

Invitee accepts: membership `invited` → `active`.

**Response** `200` — updated membership.

---

## Organisations

### `POST /v1/organisations`

Create `{ "name": string, "slug": string? }`.

**Response** `201` — organisation. Caller should attach an owner membership separately (or use signup).

### `GET /v1/organisations`

List. Query: `status`, `q` (name/slug search), `limit`, `cursor`.

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

## Error codes (illustrative)

| code | When |
| --- | --- |
| `not_found` | Unknown id |
| `conflict` | Unique violation (email, slug, membership) |
| `validation_error` | Bad body / missing fields |
| `idempotency_conflict` | Same key, different body |
| `forbidden` | Caller not allowed (later) |
