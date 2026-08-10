# Access control

How KYC decides who may do what — today, and where it is going.

Related: [data model](data-model.md) · [api](api.md) · [flows](flows.md) · [saas rethink](saas-rethink.md)

> **Status.** [Part 1](#part-1--three-axes) and [Part 2](#part-2--principals-today) describe the system **as it is**.
> [Part 3](#part-3--target-model-proposed) onward is a **proposal**, not yet built. Each section says which it is.

## Contents

1. [Three axes](#part-1--three-axes)
2. [Principals today](#part-2--principals-today)
3. [Target model (proposed)](#part-3--target-model-proposed)
4. [Merchant-hosted access control (proposed)](#part-4--merchant-hosted-access-control-proposed)
5. [Known defects](#part-5--known-defects)

---

## Part 1 — Three axes

**Shipped.** Three separate questions, deliberately kept apart. Collapsing them is the most common source of confusion in this codebase.

| Axis | Question | Lives in | Denies with |
| --- | --- | --- | --- |
| **Authentication** | Who is calling? | `internal/authn`, `internal/service/auth.go` | 401 |
| **Authorisation** | May this caller do this, here? | `internal/service/access.go`, `authz.go` | 403 / 404 |
| **Entitlement** | Does this organisation's plan include it? | `internal/service/billing.go` | 403 / `allowed:false` |

The rule the code follows: **permissions gate what an _operator_ may do inside KYC; entitlements gate what the _organisation_ may use.** A permission denial means *ask your admin*. An entitlement denial means *upgrade your plan*. They are never the same check.

Authentication decides nothing. Its only job is to produce a principal.

---

## Part 2 — Principals today

**Shipped.** A **principal** is the authenticated caller, resolved once by middleware and attached to the request context (`internal/authn/principal.go`).

`Kind` has two values, but `OrganisationID` splits the service kind in half, giving three effective roles:

| Role | `Kind` | Credential | Reach |
| --- | --- | --- | --- |
| **Operator** | `KindUser` | Session `kyc_sess_…` (Google OAuth) | Organisations with an active membership, gated by RBAC |
| **Org API key** | `KindService` + `OrganisationID` | `kyc_…` | That one organisation, narrowed by `Scopes` |
| **Platform** | `KindService` without `OrganisationID`, or any principal with `PlatformAdmin` | Env service token, or an allow-listed email | Every organisation, unconditionally |

`IsPlatform()` (`principal.go:40`) encodes the last row.

### Terminology trap

| Term | What it is |
| --- | --- |
| **User** | A person who logs into **KYC**. A principal. |
| **AppUser** | A customer of the **merchant's** product. A stored record, *not* a principal. |

An `AppUser` cannot authenticate. There is no principal kind for one, which is why a merchant's customer cannot call KYC directly from a browser.

### Resolution order

`AuthenticateBearer` (`service/auth.go:28`) tries three things; the first match wins.

1. **Session** — SHA-256 lookup in `sessions`. The SQL enforces liveness (`revoked_at IS NULL AND expires_at > now()`) and the joined user must be `active`. Sessions last 30 days.
2. **Env service token** — constant-time compare against `API_TOKENS`. Platform, no organisation.
3. **API key** — SHA-256 lookup in `api_keys`. With an organisation it is org-scoped; without one it is platform. `last_used_at` is touched on every use.

No credential is stored in plaintext.

### Billing is inside the auth path

An org API key does **not authenticate** unless the organisation's plan grants `api_access` (`service/auth.go:70`). API access is a sold capability, so the check sits in authentication rather than in each handler.

Consequence to be aware of: a billing lapse surfaces to the merchant as a **401**, not as a payment problem.

### Public paths

These bypass authentication entirely (`internal/http/middleware.go:33`). Two of them carry their own secret instead.

| Path | How it is protected |
| --- | --- |
| `/v1/public/…` | Nothing; public by design (org logo) |
| `/v1/billing/webhooks/…` | Stripe signature |
| `/v1/hooks/inbound/…` | `X-KYC-Webhook-Secret` header, or a path token |
| `/v1/auth/providers`, `/v1/auth/google`, `/v1/auth/google/callback`, `/v1/auth/dev-login` | Public by necessity; login starts are rate limited per IP |

### Authorisation gates

Handlers never inspect the principal directly. They call one of these gates in `service/access.go`.

| Gate | Requires |
| --- | --- |
| `RequirePrincipal` | Anyone authenticated |
| `RequireUser` | A human session; refuses every API key |
| `RequirePlatform` | `IsPlatform()` |
| `RequireOrgMember(org)` | The organisation is in reach |
| `RequireOrgPermission(org, key)` | The above, plus the capability |

The last two now share one implementation, `requireOrgAccess`, which assembles the caller's grants and asks the evaluator. Membership, API key scopes and platform reach all take the same path instead of four hand-written branches.

Three behaviours worth knowing:

- A suspended or archived organisation returns **404, not 403**. Existence is not leaked.
- An organisation the caller cannot reach also returns **404**, and byte-identically to one that does not exist. Organisation ids are not enumerable by reading status codes.
- An org API key with an **empty** `Scopes` list has **full organisation access**. Scoping is opt-in. This is a defect, kept for now so the gate swap stayed behaviour-preserving.

Reaching an organisation is modelled as an inherent capability, `organisation:member`. It is not a row in the permissions catalog, because it is not something a role hands out — it is what holding any grant in an organisation means. That is how a member whose role carries no permissions can still be a member.

RBAC for users is one join — `user → membership → role → permission`, scoped to the organisation, requiring `membership.status = 'active'`. Assembly loads the whole set once per request; the decision is then a set lookup.

---

## Part 3 — Target model (proposed)

**Not built.** The model above works but has three structural weaknesses: platform privilege is ambient and unconditional, an unscoped key is the most powerful one, and there is no way to give KYC staff limited or temporary access.

The proposal replaces the four gate paths with one primitive.

### The grant

```
Grant = (Principal, Scope, Capabilities, Constraint, Expiry, GrantedBy)
```

| Field | Meaning |
| --- | --- |
| `Principal` | Who |
| `Scope` | Which organisations: `global`, `org:<id>`, later `group:<id>` |
| `Capabilities` | A set of `resource:action`, from a role or an explicit list |
| `Constraint` | Optional narrowing that depends on the request, e.g. `subject == self` |
| `Expiry` | Optional; absent means standing |
| `GrantedBy` | Always recorded |

Every caller becomes one row shape, with no special cases:

| Actor | Scope | Capabilities | Constraint | Expires |
| --- | --- | --- | --- | --- |
| Support engineer | `org:acme` | role `support` | — | 4 hours |
| Billing ops | `global` | role `billing` | — | standing |
| Merchant admin | `org:acme` | role `admin` | — | standing |
| Backend API key | `org:acme` | explicit list | — | standing |
| App user token | `org:acme` | `profile:read/write` | `subject == self` | 15 minutes |
| Break-glass | `global` | all | — | env-bound |

Platform stops being a bypass and becomes a grant whose scope is `global`. There is no branch to audit.

### A role is not a grant

A role is a **named capability set** — a grant with the *who* and *where* left unbound.

```
Grant = bind(Role, principal, scope, constraint, expiry)
```

Roles are authored, extended and versioned. Grants are issued, revoked and expired. Editing a role must reach everyone holding it; editing a grant must reach exactly one principal.

Roles may extend **multiple** parents. Inheritance is resolved at **write** time into `effective_capabilities`, so the decision path never traverses a graph.

### Grants come from three sources

Not every grant is a row.

| Source | Rows | Example |
| --- | --- | --- |
| **Inherent** | zero | Every authenticated principal may read its own identity |
| **Derived** | zero | An active membership *is* an org-scoped grant carrying its role |
| **Stored** | one each | Time-boxed staff access, API key capability lists, app-user tokens |

Membership stays the source of truth for org access. The grants table holds only what no existing relationship expresses, so there is no backfill and no dual-write.

> If you are about to write the same grant for many principals, it belongs in a **role** or the **inherent set** — not in the table.

### Invariants

| # | Invariant | Why |
| --- | --- | --- |
| 1 | **Deny by default.** Empty grants nothing, never everything. | Today's empty-scopes-means-everything is a fail-open default. |
| 2 | **No principal may grant what it does not hold** — capabilities and scope must be a subset of the granter's. | Makes escalation structurally impossible rather than review-dependent. |
| 3 | **Capabilities are a closed set per namespace.** | A typo cannot reach a grant row. |
| 4 | **Cross-tenant scope is issued, never assigned.** `global` is not a capability an org role can carry. | Standing global access should need a reason. |
| 5 | **Out of scope is indistinguishable from absent** — 404 for scope, 403 for capability. | Tenants must not be enumerable via status codes. |
| 6 | **Additive only. No deny rules, ever.** | Union is commutative, so multiple inheritance is safe and diamonds need no precedence rules. |

Invariant 6 is the one that makes the rest tractable. A single deny rule reintroduces ordering, priority and conflict resolution.

Invariant 2 has exactly **two** carve-outs, both structural, both belonging in one file:

- **Break-glass** — holds everything by definition, so the subset rule is satisfied rather than bypassed.
- **Organisation creation** — a new org has no members, so nobody can delegate into it. The system issues the founding owner grant.

### Scope is a fixed ladder, not a tree

`global > org > (later) project` are **named, finite levels**. Containment is a key comparison, never a traversal, because every resource carries its scope keys directly.

The line to hold: **add scope kinds, never add depth.** A level that nests inside itself turns containment into a graph walk on every request and makes "who can read this?" unanswerable.

### Cold start

With zero rows, invariant 2 says nobody can grant anything. The chain that resolves it:

1. **Migrations** seed role templates and the platform organisation. No users, no grants.
2. **Break-glass** resolves from env before any database read. It is the root of trust, and it must never be removable.
3. **First admin** is minted on first matching login, gated on a **`bootstrapped_at` marker**, not on "are there zero global grants" — otherwise revoking every global grant reopens the door.
4. **Normal delegation** takes over; every later grant descends from a prior one.

Merchant signup is the same shape per tenant: `POST /v1/organisations` mints the founding owner grant, issued by the system.

---

## Part 4 — Merchant-hosted access control (proposed)

**Not built.** Merchants building on KYC need their own access model — projects, workspaces, environments, and roles for *their* customers. This is a common requirement and KYC is already the system of record for their app users, schema, features and plans.

This is the same engine with a **hard namespace partition**.

| | KYC's own | The merchant's |
| --- | --- | --- |
| Governs | Who may operate KYC | Who among their app users may do what in their product |
| Principals | Operators, API keys, staff | App users |
| Capabilities | **Closed**, defined in KYC's code | **Open**, declared by the merchant |
| Scope kinds | Fixed: `global`, `org` | Declared by the merchant: `project`, `environment`, … |
| Enforced by | KYC handlers | The merchant's backend |
| Stored in | `grants` | `app_grants` |

Invariant 3 becomes **closed per namespace**: KYC's set is code-defined and typo-proof; the merchant's is whatever they declare, contained entirely to their tenancy.

This mirrors a pattern KYC already has. Merchants declare their app-user **attribute schema** and KYC stores and validates against it. Declaring scope types, capabilities and roles is the same move applied to access instead of data.

### Shape of the surface

```
POST /v1/organisations/{id}/scope-types    declare "project"
POST /v1/organisations/{id}/capabilities   declare "deploy:production"
POST /v1/organisations/{id}/app-roles      define roles, with extends
PUT  /v1/app-users/{id}/grants             alice, project:p1, developer
GET  /v1/app-users/{id}/access             the grant set, with a version
```

### Ship the grant set, not a per-request check

`GET /v1/app-users/{id}/access` returns the assembled grant set plus a `version`. The merchant's backend caches it and evaluates **locally** through the SDK, with a webhook on change.

A per-request `POST /v1/access/check` stays available for simple integrations, but it must not be the default: it would put a network hop inside every request of the merchant's app and make KYC own their latency and availability.

### Architectural consequence

Because the merchant's backend evaluates locally, the evaluator must be a **portable, dependency-free library** that compiles into KYC's API, the Go SDK and the TypeScript SDK.

```
Decide(grantSet, capability, scopeRef) → decision
```

Pure function. No database, no KYC types, no service dependency. This constrains how it is written from day one.

### Note on scope

The README lists *"not Auth0/Clerk-for-your-customers"* as a non-goal. That non-goal is about **authentication** — KYC is not their login provider. Offering **authorisation** for their app users does not violate it, but it is a real product expansion and should be a deliberate decision.

---

## Part 5 — Known defects

**Present in the shipped system.** Each is independent of whether the target model is adopted.

### Platform admin was a one-way latch — fixed

The flag was only ever written true, and the session read it back as
`sess.PlatformAdmin || emailInList(...)`, so **removing an address from
`PLATFORM_ADMIN_EMAILS` did not demote anyone** and offboarding silently did nothing.

Platform privilege is now derived per request from the env list and never persisted. The `users.platform_admin` column is dropped (migration `000042`), and the login paths no longer take the list at all: it is a property of the request, not of the login. `TestPlatformAdminIsDerivedNotLatched` pins this by moving the same session between servers configured with and without the address, and asserting a platform route refuses after demotion.

### Platform privilege is ambient and unconditional

Partly addressed. Platform capabilities now flow through an ordinary grant at `global` scope rather than a branch inside the permission check, so platform takes the same evaluation path as everyone else.

What remains: the gate still short-circuits before the organisation is loaded, so platform tooling can act on an archived or partly-created tenant. That short-circuit is deliberate and was preserved.

The larger problem is untouched. Platform is still **all-or-nothing** — the global grant carries every capability — so **least privilege for KYC's own staff remains impossible**. A support engineer who needs to read one merchant's activity must still be given everything, everywhere. Phase 5 splits this into scoped roles.

It also still rides on an ordinary session, so a staff member browsing a normal screen carries full reach in every request.

### Unscoped API keys are unrestricted

An empty `Scopes` array grants the whole organisation (`access.go:116`). A key created without thinking about scopes is the most permissive one available, not the least.

### No principal for a merchant's customer

`Kind` has no app-user value, so a browser cannot call KYC directly. This blocks the settings embed and any browser-side SDK. It does **not** block [Part 4](#part-4--merchant-hosted-access-control-proposed), where the merchant's backend performs the check.

---

## Migration approach (proposed)

The phase that makes the rest safe is **phase 3**.

| Phase | Work | Status |
| --- | --- | --- |
| 1 | [`core/access`](../core/access): capability, scope, grant set, role expansion, `Decide`, delegation. A separate zero-dependency module, because the same logic must run in the API and inside both SDKs. | **Built** |
| 2 | Grant assembly from existing relationships (`service/grants.go`), plus KYC's code-defined capability registry. | **Built** |
| 3 | Org-scoped gates evaluate through `Decide`. | **Built** |
| 4 | `grants` table for what no relationship expresses: time-boxed staff access, app-user tokens. | Not started |
| 5 | Split platform into scoped roles; delete `platform_admin`. The latch defect dies here. | Not started |

### No grants table yet, on purpose

Every grant today is **derived**: platform from the principal flag, API keys from their row and scopes, users from an active membership and its role. Nothing needs storing, so nothing is stored. Membership stays the single source of truth for organisation access, which means no backfill and no dual-write.

The table arrives in phase 4, when there is finally something with no relationship to derive from.

### Shadow mode was skipped

The original plan ran both gates in parallel and logged disagreements. That exists to de-risk a **live** system. KYC is not deployed, so the existing API suite serves the same purpose more cheaply: the gates were swapped outright, and every authorisation test — API keys, hardening, onboarding, settings, memberships, authz, app users, and the local e2e — passed unchanged.

Exactly one behaviour changed deliberately: an organisation the caller cannot reach now returns **404 instead of 403**, matching invariant 5.

### Test as properties, not examples

- Every assembled capability is in its namespace's set.
- Every grant-creation path satisfies the subset rule, with only the two carve-outs skipping it.
- A scope denial is 404 on **every** route. One 403 leaks a tenant's existence.
- The bootstrap branch is unreachable once the marker is set, including when all global grants are revoked.
