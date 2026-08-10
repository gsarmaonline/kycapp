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
    OwnerUserID    string   // the user an API key belongs to
    RecoveryID     string   // set for a recovery credential
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
| **Machine**, token | **Org API key** | **Recovery credential**, or the last-resort `API_TOKENS` |

Concretely:

| Caller | What it is |
| --- | --- |
| **Operator** | Priya at Acme. Signs in with Google, manages Acme. Sees only Acme. |
| **Staff** | Dana at KYC. Signs in the same way. Because she is a member of the platform organisation, she reaches every merchant — limited to her role's capabilities. |
| **Org API key** | Acme's backend calling `entitlements/check` at 3am. No human involved. |
| **Recovery credential** | Minted by staff during an incident, with a reason and an expiry. Ordinary data, so it is attributable and revocable. |
| **Last-resort token** | You, recovering a database that is itself broken. Resolves from the environment, not from data. Normally unset. |

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
| Org API key | `kyc_` + 24 random bytes | SHA-256 hash | Until revoked; bounded by its owner |
| Recovery credential | `kyc_recovery_` + 32 random bytes | SHA-256 hash | Required expiry, max 7 days; revocable |
| Last-resort token | Arbitrary, from `API_TOKENS` | Environment, constant-time compared | Until redeployed |

No credential is stored in plaintext. Login starts are rate limited per IP (`AUTH_RATE_LIMIT_PER_MIN`, default 20), and every mutating request is written to the audit log with its actor label.

Human login is **Google-only**. `POST /v1/auth/dev-login` exists for local work and tests, and is gated on `AUTH_DEV_LOGIN`.

### Three tiers of elevated access

Recovery used to mean one shared environment token. It is now split by how broken things are:

| Tier | What | When |
| --- | --- | --- |
| **Staff membership** | A person in the platform organisation | Normal operation |
| **Recovery credential** | Data: minted by a named person, with a reason and a required expiry | Access is wrong but the database works |
| **`API_TOKENS`** | Environment, resolved before any query | The database itself is the problem |

**Recovery credentials are ordinary grants.** Resolving one produces a global-scope grant that `Decide` weighs like a membership. There is no bypass in the access path for them.

They must be minted by a principal that **already** reaches every organisation, so a recovery credential is delegation rather than a way around a boundary the caller is not already inside. A reason is required — one without a stated reason is indistinguishable from a back door — and so is an expiry, capped at seven days, because a permanent one is a bypass under another name.

**`API_TOKENS` remains, and should normally be unset.** It is the only credential that is not data, which is exactly its value and its cost: it survives a mis-seeded or partly-migrated database, and it cannot be attributed, expired or revoked without a deploy. Reach for it when a recovery credential cannot be minted because the data is what broke.

Since a recovery credential is a grant, none of the three needs a short-circuit in the gates. Break-glass produces a grant too; the escape latches that used to sit in `requireOrgAccess` and `RequirePlatformCapability` are gone.

---

## Resolution order

**Shipped.** `AuthenticateBearer` (`service/auth.go:28`) tries four things; the first match wins.

1. **Session** — SHA-256 lookup in `sessions`. The SQL enforces liveness (`revoked_at IS NULL AND expires_at > now()`) and the joined user must be `active`. Staff status is then read from the data: a live membership of the platform organisation.
2. **Last-resort token** — constant-time compare against `API_TOKENS`.
3. **Recovery credential** — SHA-256 lookup in `recovery_credentials`. Expiry and revocation are enforced **in the query**, so a stale credential never reaches application code.
4. **API key** — SHA-256 lookup in `api_keys`. Its capabilities come from its owner, narrowed by its scopes.

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

### API keys belong to a user — settled

`api_keys.user_id` is the owner, and a key's capabilities are the **intersection of that owner's grants and the key's scopes**. The key holds nothing of its own, which gives three properties with no extra rule to enforce:

- A key can never exceed the person who holds it.
- Demoting them demotes it on the next request.
- Revoking their membership stops it.

It also settles the operator/key question: the difference really is *manual versus programmatic*, because the same person is behind both. A key is Priya acting through a program.

Empty scopes now mean **"everything my owner can do"** — bounded — rather than the unrestricted organisation access an unscoped key used to grant.

Break-glass cannot own a key: it is an environment credential with no person behind it, and a key it created would derive nothing. A key creating another key passes its own owner along, so a chain still terminates at a person. Keys predating ownership confer nothing rather than keeping their old access.

**The cost, accepted deliberately:** offboarding stops that person's keys. Ownership is therefore transferable, and a key that must outlive its owner's involvement has to be moved before they go. `TestRevokingTheOwnerStopsTheirKey` pins the behaviour so it cannot regress quietly.

**Not yet built:** the transfer flow, and a view of keys whose owner has lost their membership.

### No principal for a merchant's customer

`Kind` has no app-user value, so a merchant's customer cannot call KYC from a browser. This blocks the settings embed and any browser-side SDK.

It does **not** block [merchant-hosted access control](authorisation.md#merchant-hosted-access-control-proposed), where the merchant's own backend performs the check.

### Staff reach is ambient

Staff carry their full role in every request, including while browsing an ordinary screen. `memberships.expires_at` supports time-boxed access, but there is no API or UI for issuing it, so just-in-time access is possible rather than default.
