# Authorisation

> **This describes the system in production today, and it stays authoritative
> until it is replaced.** A successor is designed and its engine is built and
> tested in [`core/reach`](../core/reach): see
> [access-by-reachability.md](access-by-reachability.md). Nothing is wired to it
> yet, and the migration sequence is at the end of that document. Change this
> system, not the successor, until step 4 of that sequence begins.

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
6. [Worked examples](#worked-examples) — the common authorisation models, written as grants
7. [Status and remaining work](#status-and-remaining-work)
8. [Known defects](#known-defects)

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
| 6 | **No grant subtracts from another.** | Union is commutative, so multiple role inheritance is safe and diamonds need no precedence rules. |

Invariant 6 is what makes the rest tractable. A single deny rule reintroduces ordering, priority and conflict resolution.

It used to read *"additive only, no deny rules, ever"*, and the wording was too strong for what it protects. A grant may now carry **exclusions** — scopes it does not reach, capabilities carved out of its wildcard, customers an everyone-grant skips. Each narrows **the grant it sits on** and nothing else, so grants stay unordered, `Decide` stays a first-match loop, and deleting a grant still removes access rather than adding it. What remains forbidden is a rule that vetoes a *different* grant, which is the thing that would bring back precedence.

The practical consequence: an exclusion is not a lock. If a second grant reaches an excluded resource, that grant allows. For a hard "nobody reaches this", no grant may reach it.

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

## Worked examples

One model expressed as many. Each of the well-known authorisation styles is a
particular way of filling in a grant, not a different engine — which is the
claim this section is here to make good on.

Throughout: `S` is the scope, `C` the capabilities, `K` the constraint,
`X` the exclusions. A grant carries no principal, so "who" is a property of the
grant set it lands in.

### 1. Plain RBAC — a role over a tenant

The ordinary case, and the one everything else is a variation of.

```
subject:    Priya
role:       editor  →  { docs:read, docs:write }
scope:      organisation:acme
```

`Decide` answers by scope containment then capability membership. Nothing else
runs. Changing what `editor` means changes it for every holder, because the
grant carries the role and the role's resolved set is recomputed on write.

### 2. Role hierarchy — build on, never override

```
viewer        →  { docs:read }
editor        extends viewer  →  { docs:read, docs:write }
billing_admin extends editor  →  { docs:read, docs:write, billing:refund }
```

Resolved at write time and stored flat, so a check never walks the chain and a
diamond needs no precedence rule — two paths to the same capability union to
the same set. Depth is capped at `MaxRoleDepth`; cycles are rejected.

Roles never subtract. That is what lets inheritance stay a union rather than an
ordered override list.

### 3. Ownership — "your own rows"

Not a scope. The scope says which resources the grant can reach at all; the
constraint requires the resource to belong to the holder.

```
subject:    every customer
role:       self_manager  →  { profile:read, profile:write }
scope:      tenant:acme
constraint: self_subject
```

One row, every customer, present and future. `Decide` compares
`GrantSet.Subject` against `Resource.Subject`, both supplied by whoever runs the
evaluator.

Note what this does **not** do. It has no opinion on which verbs are allowed —
`account:delete` is stopped by leaving it out of the role, never by the
constraint. The constraint only ever answers "is this thing yours?".

### 4. Multi-tenancy — the boundary is just a scope

```
scope: organisation:acme
```

There is no separate tenancy mechanism. A resource in Acme carries the
`organisation: [acme]` coordinate; a grant scoped to another organisation does
not contain it, and the denial reads `out_of_scope`, which is
indistinguishable from the resource not existing. That is invariant 5, and it
is why tenants cannot be enumerated by probing.

### 5. Sub-org structure — project scoping, without a tree

A person on three projects, and an object in two of them:

```
grants:   (project:apollo, editor), (project:borealis, viewer), (project:ceres, viewer)
resource: { organisation: [acme], project: [apollo, ceres] }
```

Containment is a map lookup per grant, never a graph walk. The resource declares
every container it belongs to, so nesting lives in the data. Matching any one
coordinate is enough, which keeps it additive: adding the object to a fourth
project can only widen access, never narrow it.

This is why scope kinds may be added freely but must never nest inside
themselves — the moment they do, "who can reach this?" stops being a lookup.

### 6. ABAC-shaped rules — pushed to write time

There is no attribute predicate in the evaluator, deliberately. An attribute
rule evaluated at read time makes every attribute write a permission change.

The same intent, written as data:

```
group:  au_customers       (explicit membership)
grant:  (au_customers, regional_reader, region:apac)
```

The rule that decides membership runs where rules belong — in the merchant's own
onboarding, or an automation on `app_user.created`. What lands here is the
decision, at a known moment, with an author. Access changes when someone changes
access, not when someone edits a phone number.

### 7. Time-boxed access — expiry on the grant

```
subject:    Priya
role:       incident_responder
scope:      organisation:acme
expires_at: 2026-08-12T02:00:00Z
```

`Grant.Active(now)` is checked before scope, so an expired grant is invisible
rather than denied-with-a-reason. This is the mechanism behind just-in-time
staff access; what is missing is an API that issues it in one click, which is
why standing access is still the default in practice. See
[known defects](#known-defects).

### 8. Delegation — bounded by what the granter holds

Invariant 2, enforced in `CanGrant` rather than by review:

```
granter holds:  (organisation:acme, { docs:read, docs:write })
proposes:       (project:apollo,   { docs:read })            ✓ narrower, held
proposes:       (project:apollo,   { billing:refund })       ✗ not held
proposes:       (global,           { docs:read })            ✗ global is issued, never assigned
```

A wildcard is delegable only by a wildcard holder, and it must carry forward
every carve-out the granter has — otherwise "everything except refunds" issues
"everything" and grants itself refunds.

### 9. Machine principals — the same shape as a person

An API key is not a special case. It derives its grants from its owner on every
request, intersected with its scopes:

```
key.grants = owner.grants ∩ key.scopes
```

So a key can never exceed the person who made it, and demoting them demotes it.
Empty scopes mean "everything my owner can do" — a statement about the owner,
not a blank cheque. This is structural rather than checked at creation, which is
what makes it hold as the owner's role changes over time.

### 10. Everyone — a baseline without per-customer rows

```
subject: everyone
scope:   tenant:acme
role:    self_manager
```

One row covers every customer of the organisation, including tomorrow's
signups. The alternative — materialising a membership per customer — costs a row
and a queue job per person and expresses exactly the same thing.

This is a wildcard over the subject axis, and it widens as the population grows.
That is the deal, and it is why it carries an exclusion list.

### 11. Exceptions — what positive scoping cannot say

Ten thousand projects, one confidential:

```
subject: everyone
scope:   organisation:acme
role:    reader
except:  project:salaries
```

Positive scoping would need 9,999 grants to express the same thing. The
exclusion narrows **this grant only** — a separate grant reaching
`project:salaries` still allows, so the outcome never depends on grant order.

Every axis with a wildcard has one, because a wildcard is a claim about a set
nobody can enumerate and no such claim is ever exactly right:

| Wildcard | Exclusion | Evaluated in |
| --- | --- | --- |
| subject `everyone` | `except_app_user_ids` | assembly, in SQL |
| capabilities `all_capabilities` | `except_capabilities` | `Grant.Allows` |
| scope | `except_scopes` | `Grant.Reaches`, after containment |

### 12. Capability wildcards — and what they cost

```
capabilities: every capability in org:acme
except:       account:delete
```

A concrete list is a statement about capabilities that exist. A wildcard is a
standing instruction that re-evaluates whenever the vocabulary changes — declare
`billing:refund` next quarter and every holder gains it, with no edit to any
grant.

The carve-out list does not fix that. It lets you name the dangerous verbs you
know of today; tomorrow's arrive granted. A merchant who wants "everything"
without that property ticks every box in the role form instead, which expands at
authoring time and stores concrete.

The registry still refuses to let anyone **declare** a capability named `*`
([capability.go](../core/access/capability.go)). The wildcard lives on the
grant, never in the vocabulary.

### 13. What this model will not express

Worth stating plainly, because the workaround is always "do it in your own
backend":

| Wanted | Why not |
| --- | --- |
| "May delete accounts, but never their own" | A deny rule. Grants only add; there is no way to subtract from a grant that already allows. |
| "Nobody may ever reach this resource" | An exclusion narrows one grant. Reach the resource with a second grant and it allows. Guarantee it by granting nothing that reaches it. |
| "Read only" as a flag | A capability set containing no write verbs. Adding a flag would put two mechanisms in the same job. |
| "Approve if two people agree" | Workflow, not authorisation. Nothing here has a notion of pending state. |

Every one of these is expressible in a policy language. That is the thing being
avoided: the model above is decidable by lookup, which is what keeps "who can
reach this?" answerable at all.

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
