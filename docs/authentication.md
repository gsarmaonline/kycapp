# Authentication

**Who is calling?** This document covers producing a principal from a credential, and nothing else.

What that principal may *do* is [authorisation](authorisation.md). The split matters: authentication decides nothing about access, and every access rule works the same way regardless of how the caller got in.

Related: [authorisation](authorisation.md) · [api](api.md) · [flows](flows.md)

> Sections are marked **Shipped** or **Proposed**. Nothing here describes intent as if it were behaviour.

## Contents

1. [The principal](#the-principal)
2. [Four callers](#four-callers)
3. [Credentials](#credentials)
4. [Resolution order](#resolution-order)
5. [Public paths](#public-paths)
6. [Bootstrap: the chain of trust](#bootstrap-the-chain-of-trust)
7. [Billing sits inside the auth path](#billing-sits-inside-the-auth-path)
8. [Open questions](#open-questions)

---

## The principal

**Shipped.** A **principal** is the authenticated caller, resolved once by middleware and attached to the request context (`internal/authn/principal.go`).

```go
type Principal struct {
    Kind           Kind     // user | service
    UserID         string
    OrganisationID string   // set for org-scoped API keys
    Scopes         []string // API key permission scopes
    PlatformAdmin  bool     // reaches every organisation
    SessionID      string
    Actor          string   // audit label
}
```

Authentication's entire job is to build this. It never decides whether the request is allowed.

---

## Four callers

**Shipped.** The four are a 2×2 over two independent questions: **is a human present**, and **how far do they reach**.

|  | Reaches **one** organisation | Reaches **every** organisation |
| --- | --- | --- |
| **Human**, Google session | **Operator** | **Staff** |
| **Machine**, token | **Org API key** | **Break-glass** |

Concretely:

| Caller | What it is |
| --- | --- |
| **Operator** | Priya at Acme. Signs in with Google, manages Acme. Sees only Acme. |
| **Staff** | Dana at KYC. Signs in the same way. Because she is a member of the platform organisation, she reaches every merchant — limited to her role's capabilities. |
| **Org API key** | Acme's backend calling `entitlements/check` at 3am. No human involved. |
| **Break-glass** | An ops script, or you recovering a broken database. Resolves from the environment, not from data. |

### Operator and Staff are the same mechanism

Both are `KindUser`: same Google login, same session, same code path. The only difference is **which organisation they are a member of**. Staff belong to the seeded platform organisation (`org_platform`); that membership is what confers reach.

There is no staff flag, no staff table, and no role name in Go. Reach is **derived** from which organisation the membership is in, so a merchant can configure their own roles however they like and never reach another tenant.

### Reach is not power

"Staff" sounds like more power than "Operator", but the two are separate:

- **Reach** — which organisations you can touch. Staff get all of them.
- **Power** — what you may do there. That comes from your role's capabilities.

Dana can open any merchant and change nothing. Priya, as an owner, has full power inside Acme alone.

---

## Credentials

**Shipped.**

| Credential | Format | Stored as | Lifetime |
| --- | --- | --- | --- |
| User session | `kyc_sess_…` | SHA-256 hash | 30 days, revocable |
| Org API key | `kyc_` + 24 random bytes | SHA-256 hash | Until revoked |
| Break-glass | Arbitrary, from `API_TOKENS` | Environment, constant-time compared | Until redeployed |

No credential is stored in plaintext. Login starts are rate limited per IP (`AUTH_RATE_LIMIT_PER_MIN`, default 20), and every mutating request is written to the audit log with its actor label.

Human login is **Google-only**. `POST /v1/auth/dev-login` exists for local work and tests, and is gated on `AUTH_DEV_LOGIN`.

### Why break-glass is not just an API key with no organisation

An API key is a database row. When the database is empty, mis-seeded, or freshly restored there is no row to read, and no way to create one, because creating it would need permission you cannot yet hold.

Break-glass resolves from an environment variable **before any query runs**. That is the whole point: it is the one credential that is not data. It must never become removable.

---

## Resolution order

**Shipped.** `AuthenticateBearer` (`service/auth.go:28`) tries three things; the first match wins.

1. **Session** — SHA-256 lookup in `sessions`. The SQL enforces liveness (`revoked_at IS NULL AND expires_at > now()`) and the joined user must be `active`. Staff status is then read from the data: a live membership of the platform organisation.
2. **Break-glass** — constant-time compare against `API_TOKENS`. Reaches everything, belongs to no organisation.
3. **API key** — SHA-256 lookup in `api_keys`. With an organisation it is org-scoped; without one it is platform. `last_used_at` is touched on every use.

---

## Public paths

**Shipped.** These bypass authentication entirely (`internal/http/middleware.go:33`). Two carry their own secret instead.

| Path | How it is protected |
| --- | --- |
| `/v1/public/…` | Nothing; public by design (organisation logo) |
| `/v1/billing/webhooks/…` | Stripe signature |
| `/v1/hooks/inbound/…` | `X-KYC-Webhook-Secret` header, or a path token |
| `/v1/auth/providers`, `/v1/auth/google`, `/v1/auth/google/callback`, `/v1/auth/dev-login` | Public by necessity |

---

## Bootstrap: the chain of trust

**Shipped.** An empty database has no staff, and no principal can grant what it does not hold, so something outside the data has to start the chain.

1. **Break-glass** resolves from the environment before any query. Root of trust.
2. **First staff member** is minted when an address in `PLATFORM_ADMIN_EMAILS` signs in: a membership of the platform organisation with the role the migration nominated. Gated on a `system_state` marker, **not** on a count of staff, so revoking every staff membership cannot reopen the door.
3. **Everything after** is ordinary delegation. `PLATFORM_ADMIN_EMAILS` confers nothing on its own; staff are added by granting a role.

Migration `000043` seeds the platform organisation, its roles, and the marker. Nothing else is seeded: no users, no memberships.

---

## Billing sits inside the auth path

**Shipped.** An org API key does **not authenticate** unless the organisation's plan grants `api_access` (`service/auth.go:70`):

```go
allowed, entErr := s.CheckEntitlement(ctx, p.OrganisationID, "api_access")
if entErr != nil || !allowed {
    return authn.Principal{}, false   // 401, not 403
}
```

API access is something KYC sells, so the check sits here rather than in every handler. A human signing into the UI has no such gate.

**Consequence to know:** a billing lapse reaches the merchant as a **401**, not as a payment problem. Moving it to a gate would make it a 403 with a reason, at the cost of a per-handler check.

---

## Open questions

### API keys have no owner

`api_keys` has no user column, not even `created_by`. A key belongs to an organisation and to nobody in particular.

That means:

- A person leaves, their membership is revoked and sessions die, but **keys they created keep working**.
- Audit shows `api-key:nightly-sync` and cannot answer "whose is this?"
- A key may hold permissions **no current member has**.

This blocks framing the operator/key difference as *manual versus programmatic*, because the programmatic path has no person behind it.

Two distinct things are wanted, and most mature systems ship both:

| | Personal token | Service identity |
| --- | --- | --- |
| Acts as | The user | The organisation |
| Permissions | A subset of theirs | Its own |
| Dies when | They leave | Someone revokes it |
| Right for | Scripts, one-off automation | Production integrations |

KYC has only the second. Adding the first would make "manual versus programmatic" literally true, but it must not replace org keys: a production integration that dies because an engineer left is a worse failure than the one it fixes.

**Cheapest useful step:** add `created_by` to `api_keys`. One column, and it answers the question that actually comes up.

### No principal for a merchant's customer

`Kind` has no app-user value, so a merchant's customer cannot call KYC from a browser. This blocks the settings embed and any browser-side SDK.

It does **not** block [merchant-hosted access control](authorisation.md#merchant-hosted-access-control-proposed), where the merchant's own backend performs the check.

### Staff reach is ambient

Staff carry their full role in every request, including while browsing an ordinary screen. `memberships.expires_at` supports time-boxed access, but there is no API or UI for issuing it, so just-in-time access is possible rather than default.
