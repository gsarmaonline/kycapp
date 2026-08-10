# Authorisation

**May this caller do this, here?** This document covers the access decision and the model behind it.

How the caller was identified is [authentication](authentication.md). Authorisation never asks how someone signed in: an operator, a staff member, an API key and break-glass all take the same evaluation path.

Related: [authentication](authentication.md) · [data model](data-model.md) · [api](api.md) · [`core/access`](../core/access)

> Sections are marked **Shipped** or **Proposed**. Nothing here describes intent as if it were behaviour.

## Contents

1. [Not the same as entitlements](#not-the-same-as-entitlements)
2. [The model](#the-model)
3. [The gates](#the-gates)
4. [Invariants](#invariants)
5. [Merchant-hosted access control](#merchant-hosted-access-control) — guide in [customer-access.md](customer-access.md)
6. [Status and remaining work](#status-and-remaining-work)
7. [Known defects](#known-defects)

---

## Not the same as entitlements

**Shipped.** The most common confusion in this codebase. Two different questions, two different owners, two different denials.

| | Authorisation | Entitlement |
| --- | --- | --- |
| Asks | May this **caller** do this? | Did this **organisation** buy this? |
| Written by | Merchant operators, via roles | KYC and Stripe |
| Denial means | *Ask your admin* | *Upgrade your plan* |
| Lives in | `service/access.go` | `service/billing.go` |

Both write paths for entitlements are platform-gated (`billing_handlers.go`), so a merchant operator holding every permission in their organisation still cannot grant their organisation an entitlement. That wall is deliberate: merging the two would turn an RBAC misconfiguration into a billing bypass.

Product features add a third layer — what the merchant may unlock for *their* customers. See [billing-plans.md](billing-plans.md).

---

## The model

**Shipped.** Four concepts, in layers.

| Layer | Is | Lives in | Example |
| --- | --- | --- | --- |
| **Capability** | An atomic verb | Code, closed set | `app_users:write` |
| **Role** | A named set of capabilities | Data, per organisation | `admin` |
| **Grant** | Binds capabilities to a principal and a scope | Derived from relationships | *(alice, org:acme, admin)* |
| **Principal** | The caller plus its assembled grants | Runtime, per request | — |

### A role is not a grant

A role is a grant with the *who* and *where* left unbound:

```
Grant = bind(Role, principal, scope)
```

That is what makes roles reusable. The same role can be held by different people in different organisations. Roles are authored and versioned; grants are issued and revoked. Editing a role reaches everyone holding it; editing a grant reaches one principal.

### Grants are derived, not stored

There is no grants table. Every grant comes from a relationship that already exists:

| Source | Produces |
| --- | --- |
| Break-glass (environment) | Global scope, every capability |
| Recovery credential row | Global scope, every capability, expiring with the credential |
| Org API key row | Its **owner's** grants, narrowed by the key's `Scopes` and to the key's organisation |
| Active membership | That organisation, with the role's capabilities |
| Active membership **of the platform organisation** | **Global** scope, with the role's capabilities |

Membership stays the single source of truth for organisation access, so there is no backfill and no dual-write. Assembly runs once per request (`service/grants.go`); the decision is then a set lookup.

Time-boxing is a column, `memberships.expires_at`, honoured by assembly. Expired memberships confer nothing.

### Reaching an organisation is itself a capability

`organisation:member` is inherent to holding any grant in an organisation. It is not a row in the permissions catalog, because it is not something a role hands out. That is how a member whose role carries no permissions is still a member rather than invisible.

### Scope is a fixed ladder, not a tree

`global` contains every organisation; `org:<id>` contains one. Containment is a key comparison, never a traversal, because every resource carries its scope coordinates directly.

A resource may carry **several ids for one kind**, so an object shared into two projects is reached by a grant on either:

```
object in acme, projects p1 and p4
  -> { organisation: [acme], project: [p1, p4] }
```

**Union semantics, decided deliberately.** A principal holding different roles in different containers gets the union. Someone who maintains p1 and only reads p2 can write an object shared into both, because the p1 grant allows it. Adding a resource to another container therefore only ever widens access, never narrows it.

The alternative, most-restrictive-wins, requires deny rules — and with them, ordering, precedence and conflict resolution. Invariant 6 exists to avoid exactly that. If a merchant expects "put it in the locked project to lock it down", the model will fight them; that is worth saying out loud in their UI rather than solving in the evaluator.

The rule to hold: **add scope kinds, never add depth.** A level that nests inside itself turns containment into a graph walk on every request and makes "who can reach this?" unanswerable.

### The evaluator is portable

[`core/access`](../core/access) is a separate module with **zero dependencies**, asserted in CI. The same logic has to run in the API, the Go SDK, and eventually the TypeScript SDK, because merchants will cache a grant set and evaluate locally. A dependency would make it unembeddable.

```
Decide(grantSet, capability, resource, now) -> decision
```

Pure function. No database, no ambient clock, no KYC types.

---

## The gates

**Shipped.** Handlers never inspect the principal directly.

| Gate | Requires |
| --- | --- |
| `RequirePrincipal` | Anyone authenticated |
| `RequireUser` | A human session; refuses every API key |
| `RequirePlatformCapability(key)` | Staff **and** the named capability, at global scope |
| `RequireOrgMember(org)` | The organisation is in reach |
| `RequireOrgPermission(org, key)` | The above, plus the capability |

The last two share one implementation, `requireOrgAccess`, which assembles grants and asks the evaluator.

### Only break-glass short-circuits

Staff do not. A global-reach role carries exactly the capabilities it was granted, so a read-only support role stays read-only inside a merchant's organisation. Staff skip only the organisation *visibility* check, so they can still act on an archived tenant.

Before this, any staff member bypassed permission checks entirely and least privilege for KYC's own staff was impossible to express.

### `RequireUser` is where the human/machine line matters

Four routes refuse API keys, because each needs a person present:

| Route | Why |
| --- | --- |
| `/v1/me`, logout | Both concern the session. A key has none. |
| Accept a membership invite | Accepting is consent. |
| Start Stripe checkout | Ends with someone typing card details. |

Everywhere else, a human and a machine with the same reach and the same capabilities are treated identically. That is deliberate.

### Denials

| Reason | Status | Meaning |
| --- | --- | --- |
| Out of scope | **404** | No grant reaches the resource |
| Missing capability | 403 | A grant reaches it but lacks the verb |
| Constraint failed | 403 | The grant's narrowing rejected this resource |

Out of scope is a 404, byte-identical to a resource that does not exist. Otherwise organisations are enumerable by reading status codes. A suspended or archived organisation is likewise 404, even to its own members.

---

## Invariants

**Partly shipped.** These are enforced in [`core/access`](../core/access) and tested as properties rather than examples.

| # | Invariant | Why |
| --- | --- | --- |
| 1 | **Deny by default.** Empty grants nothing. | A permissive default is the most common real escalation. |
| 2 | **No principal may grant what it does not hold.** | Makes escalation structurally impossible, not review-dependent. |
| 3 | **Capabilities are a closed set per namespace.** | A typo cannot reach a grant. |
| 4 | **Global reach is derived, never set.** | It comes from membership of the platform organisation, so obtaining it means being invited there. No column to guard. |
| 5 | **Out of scope is indistinguishable from absent.** | Tenants must not be enumerable. |
| 6 | **Additive only. No deny rules, ever.** | Union is commutative, so multiple role inheritance is safe and diamonds need no precedence rules. |

Invariant 6 is what makes the rest tractable. A single deny rule reintroduces ordering, priority and conflict resolution.

Invariant 2 has one structural carve-out: **break-glass**, which holds everything by definition, so the subset rule is satisfied rather than bypassed. A recovery credential is not a carve-out: minting one requires already holding global reach, so it is ordinary delegation. Organisation creation was a second carve-out and is now ordinary delegation when a staff member creates the tenant; self-serve signup still mints a founding owner grant from the system.

`TestCapabilityRegistryMatchesSeededPermissions` enforces invariant 3 against the migrations. It has already caught real drift.

---

## Merchant-hosted access control

> The operator-facing guide — the five objects, the flow, and the read endpoint — is [customer-access.md](customer-access.md). This section is the rationale.

**Shipped.** Merchants declare their own scope kinds, capabilities and roles, and grant those roles to their **app users**. KYC stores the model and returns an assembled grant set; the merchant's backend evaluates it locally.

Subject is app users only. A merchant's own KYC operators keep organisation-wide roles; scoping *them* to projects would mean KYC's gates learning which project every resource belongs to, which is a much larger change and is not done.

Same engine, **hard namespace partition**:

| | KYC's own | The merchant's |
| --- | --- | --- |
| Governs | Who may operate KYC | Who among their app users may do what |
| Capabilities | **Closed**, defined in KYC's code | **Open**, declared by the merchant |
| Scope kinds | Fixed: `global`, `organisation` | Declared by them: `project`, `environment`, … |
| Enforced by | KYC handlers | The merchant's backend |

Invariant 3 becomes *closed per namespace*: KYC's set stays typo-proof, the merchant's is whatever they declare, contained to their tenancy. A merchant admin must never mint a capability in KYC's namespace; `CanGrantInNamespace` already enforces this and needs wiring.

This mirrors a pattern KYC already has. Merchants declare their app-user **attribute schema**; declaring scope types, capabilities and roles is the same move applied to access instead of data.

### Ship the grant set, not a per-request check

```
GET /v1/app-users/{id}/access  ->  { grants: [...], version: 47 }
```

The merchant's backend caches it and evaluates locally through the SDK, with a webhook on change. A per-request `POST /v1/access/check` stays available for simple integrations, but it must not be the default: it would put a network hop inside every request of their app and make KYC own their latency.

### Note on scope

The README lists *"not Auth0/Clerk-for-your-customers"* as a non-goal. That is about **authentication** — KYC is not their login provider. Offering **authorisation** for their app users does not violate it, but it is a real product expansion and should be a deliberate decision.

---

## Status and remaining work

| Phase | Work | Status |
| --- | --- | --- |
| 1 | [`core/access`](../core/access): capability, scope, grant set, role expansion, `Decide`, delegation. | **Built** |
| 2 | Grant assembly from existing relationships, plus KYC's capability registry. | **Built** |
| 3 | Org-scoped gates evaluate through `Decide`. | **Built** |
| 4 | KYC as an organisation: staff are members of `org_platform`, reach is derived from that membership, memberships can expire, bootstrap is marker-gated. | **Built** |
| 5 | Platform routes gated by capability instead of staff status. | **Built** |
| 6 | API and UI for issuing time-boxed staff access, so just-in-time becomes the default rather than something the schema merely allows. | Not started — see [todo](todo.md#access-control-follow-ups) |
| 7 | Merchant-hosted access control: merchants declare scope kinds, capabilities and roles for their app users, with materialised inheritance, and read back a cached grant set. | **Built** |

Shadow mode was skipped deliberately: it de-risks a live system, and this one is not deployed, so the existing API suite served the same purpose. The gates were swapped outright and every authorisation test passed unchanged.

---

## Known defects

### API key escalation — fixed

Both defects here had the same root: a key's power was independent of any person. An empty `Scopes` array granted the whole organisation, and `createAPIKey` never compared the requested scopes against what the creator held, so a member whose role carried only `api_keys:manage` could mint an unscoped key and act well beyond their own role.

Keys now belong to a user and carry the intersection of that owner's grants and their scopes. The subset rule is not checked at creation, it is **structural**: a key derives from its owner on every request, so it can never exceed them, and demoting them demotes it. Empty scopes mean "everything my owner can do".

See [authentication](authentication.md#api-keys-belong-to-a-user--settled) for the offboarding consequence.

### Staff reach is ambient

A staff member carries their full role in every request, including while browsing an ordinary screen. `memberships.expires_at` supports time-boxed access, but nothing issues it, so just-in-time access is possible rather than default. That is phase 6, and without one-click issuing people will simply ask for standing access.

### `RequirePlatform` is gone

All twelve call sites moved to `RequirePlatformCapability`, and the coarse gate was deleted rather than left available, since a bypass nobody calls is a bypass someone eventually calls.
