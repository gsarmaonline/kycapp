# Customer access (merchant-hosted authorisation)

Authorisation a merchant runs **for their own customers**. The merchant declares the vocabulary, KYC stores the grants, and the merchant's backend decides.

Related: [authorisation](authorisation.md) · [authentication](authentication.md) · [data model](data-model.md) · [api](api.md) · in-app **Documentation → Concepts → Customer access**

## Contents

1. [What it is, and what it is not](#what-it-is-and-what-it-is-not)
2. [The flow](#the-flow)
3. [The objects](#the-objects)
4. [Reading access back](#reading-access-back)
5. [Asking the graph](#asking-the-graph)
6. [Rules that hold](#rules-that-hold)
7. [Where it lives](#where-it-lives)

---

## What it is, and what it is not

KYC already answers two access questions. Customer access is the third.

| Question | Governed by | Enforced by |
| --- | --- | --- |
| May this operator do this in KYC? | Members and permissions | KYC's gates |
| Did this organisation buy this? | Plans and entitlements | KYC's entitlement checks |
| **May this customer do this in the merchant's product?** | **Customer access** | **The merchant's backend** |

**KYC evaluates the third one now.** A merchant writes the ownership and containment edges of their own product, and asks. That is the point of the tier: a merchant should not be building an authorisation layer for every app they ship.

`POST /v1/organisations/{id}/edges` records what they own. `POST /v1/organisations/{id}/check` answers a question against it, with the route the walk took. The schema those questions resolve against is derived from the vocabulary they already declared, so scope kinds are types, capabilities are actions, and roles and groups are named sets: nothing new to author, and no migration when they add a kind.

One grant on a container reaches everything inside it. `project:apollo #can_read role:editor#holder` plus `document:d1 #parent project:apollo` means Ana reads d1, without any grant naming d1. That is the thing an exported grant set could never do, because KYC had never heard of the document.

The tenancy boundary is the namespace an edge is written in and nothing else. Every edge query filters on it and a resolver carries exactly one, so a walk physically cannot read another merchant's edges. No type name is reserved, because a name was never what kept them apart: a scope kind called `global` inside `org:acme` reaches nothing outside it.

### Listing pages and share dialogs

`POST /v1/organisations/{id}/list-objects` answers *what can this customer see?* and `POST .../list-subjects` answers *who can see this?*. They are what make a listing page and a share dialog possible without a check per row: fifty documents would be fifty walks, and ten thousand could not be rendered at all.

Both work the same way. A cheap walk gathers candidates, and every candidate is confirmed by the same check that answers a single question. Running the graph backwards exactly is not possible in general, because subtraction and intersection do not invert, so the walk is allowed to be generous and correctness comes from the verify step. There are no false positives, because the authority is the same engine.

Two fields on the answer matter:

| Field | Means |
| --- | --- |
| `all` | A wildcard grant covers every object of this type, including ones KYC holds no edge for. The list is a **lower bound**, and filtering a page by it would hide rows. |
| `truncated` | The candidate walk hit its bound. The answer is a subset, said out loud rather than returned as a short list that reads as complete. |

The cost tracks what the subject touches, not how many objects exist. A customer on three projects generates a handful of candidates whether the table holds a hundred rows or a hundred million.

The older model still works and is unchanged. `GET /v1/app-users/{id}/access` returns an assembled grant set for a backend that wants to decide locally, which is now an optimisation rather than the only route.

### What KYC could not do before

It had no idea what a *project* of the merchant's was. Putting a check on the request path would also make KYC own the merchant's latency. So KYC stores the model and returns an assembled grant set; evaluation happens in the merchant's own process.

The subject is **app users only**. A merchant's own KYC operators keep organisation-wide roles. Scoping *them* to projects would mean KYC's gates learning which project every resource belongs to, which is a much larger change and is not done.

## The flow

1. **Declare scope kinds** — the levels the product has (`project`, `region`). The kind is registered; the ids stay in the merchant's system.
2. **Declare capabilities** — the verbs the backend checks, as `resource:action`.
3. **Compose roles** — named capability sets, which may build on other roles.
4. **Group customers** *(optional)* — grant to a set once instead of one at a time.
5. **Issue grants** — one subject, one set of capabilities, one scope, optionally with an expiry.
6. **Write your own facts** — containment, and ownership where you need it. KYC cannot know which of your documents lives in which project, so a walk cannot reach one until you say.
7. **Read the grant set back** — cache it against `version`, decide locally. Or ask directly with `POST /check`, which answers from the same facts.

## The objects

### Scope kinds

A scope kind names a level in the merchant's product. A grant then reads as *the editor role over project apollo*, where `project` is the kind and `apollo` is the merchant's own id, which KYC never sees.

Kinds are **flat and independent, not a tree**. When a resource belongs to several containers at once, the check lists all of them: an object in two projects is reachable through either, and adding it to a third only widens access. Containment is a map lookup, never a traversal.

### Capabilities

One thing a customer may do: `invoices:read`, `docs:write`.

**Nothing is seeded.** A new organisation starts with an empty vocabulary, deliberately: this set is the merchant's own, so a default shipped by KYC is a guess about a product it has never seen, and a wrong guess is one merchants work around rather than delete. The empty Capabilities page instead offers **starter templates** a merchant applies with one click, after seeing the full list. A template carries roles and the inheritance between them as well as capabilities, because the shape is the part that generalises: almost every product has admin extending member extending viewer, and building that chain by hand was work every merchant repeated.

**A template never issues a grant.** A role confers nothing until one carries it, so seeding roles is a starting point you edit and seeding a grant would be granting access nobody authorised. The line is held in code and pinned by a test. The rule the whole thing follows: ship structure, offer content, never issue access. Every row a template creates records `source`, so "why does this capability exist?" stays answerable once the person who applied it has moved on. An authored capability keeps its authored provenance even when a template later names the same key. The set is **open** — whatever the merchant declares is valid — but a role may use nothing outside it. That is the reason to declare them at all: a mistyped capability is rejected when the role is built, instead of quietly granting nothing at the moment it matters.

The merchant's capabilities live in the namespace `org:<id>`. KYC's own set stays **closed** and code-defined in the `kyc` namespace. A merchant admin can never mint a capability in KYC's namespace.

**Stored as a pair.** `resource:action` is the name you write and read, but the two halves are the columns and the name derives from them, so they cannot drift apart. That is not bookkeeping: a key with a missing half used to pass this form and fail later when the schema was generated, which is a different request on a different page, so you saw the mistake nowhere near the thing that caused it. It is refused here now.

### Roles

A capability set with a name. Grants carry roles rather than raw capabilities, so *maintainer* can change in one place and every holder follows.

A role may **build on** others. What it resolves to is materialised at write time, so a check never walks a chain and editing a base role updates everything built on it. Roles never subtract, so the resolved set stays predictable however deep the chain runs. The depth cap is `MaxAppRoleDepth`.

### Groups

A named set of the merchant's customers. Granting a role to a group reaches every member, so onboarding a person becomes adding them to a group rather than reissuing grants.

Membership is an **explicit list**, not a query over attributes. A rule that recomputed silently would change who has access without anyone issuing anything, and the reason for the change would be invisible on the day it mattered. Group access and direct access add together.

**Groups nest.** A group may extend other groups, and a member of the child counts as a member of every parent, so a grant written on the parent reaches them. "Enterprise customers are also beta customers" is one declaration rather than two membership lists kept in step by hand.

This is the same relation roles have always had. Groups lacked it only because `app_role_extends` got built and nothing equivalent did, which made grouping mean two different things depending on which object you were looking at. One mechanism now covers both: a named set that confers something through membership, nesting either way.

Nesting adds, it never replaces. A grant on the child does not reach the parent's members, or extending a group would silently widen it. Multiple parents are allowed, because membership only ever adds, so a diamond resolves the same way whatever order it is walked. Cycles are refused when the group is saved.

### Grants

The only object that actually gives access; everything above is vocabulary. A grant binds one subject to one set of capabilities over one scope, optionally until a date.

Grants are **issued and revoked, never edited**. Editing in place would rewrite what someone held at a past moment; revoke-and-reissue leaves a history that can be read.

**Subject** is one customer, one group, or **everyone** — every customer of the organisation, present and future, from a single row. The everyone subject exists so a baseline needs no per-customer bookkeeping: materialising a membership per person costs a row and a queue job each and says exactly the same thing.

**Capabilities** come from a role, or from the wildcard: every capability in your namespace, including ones you declare later.

**Scope** has a wildcard too, at two levels. `project:*` reaches every project you have now or add later. `all_scopes` reaches every scope of every kind: the widest a grant can be.

The organisation is that ceiling, and it is deliberately not a scope kind you declare. A scope is a `(kind, id)` pair, and an organisation has exactly one instance, already carried by the grant itself. Declaring it would be a second way to say what every grant already says.

`global` and `organisation` stay reserved and undeclarable for the same reason from the other side: your world ends at your organisation, so a scope reaching past it would cross into another merchant's.

**A grant cannot narrow itself.** There were three exclusion lists, one per wildcard, and they are gone. They could not survive the move onto the edge graph, and the reason is worth keeping: an exception has to become a subtraction in a rule, a rule belongs to a **type** rather than to one grant, and it would therefore veto every other grant reaching that type. That is the veto [invariant 6](authorisation.md#invariants) forbids, and the thing that brings ordering, precedence and conflict resolution back with it.

So grants only ever add, which is what keeps them unordered and evaluation first-match. To say the things the exclusions used to say:

| You want | Say it as |
| --- | --- |
| everyone but a few | grant to a group, and leave those customers out of it |
| everything but one verb | declare that verb on a type the grant does not reach |
| everywhere but one place | grant at the level you mean, not at the ceiling |

The rule underneath is the one the model always had: for a hard "nobody reaches this", issue nothing that reaches it. That was already the only kind of lock that held, because an exclusion never stopped a second grant from allowing.

**Ownership is an edge now, not a constraint.** "Customers may manage their own things" used to be a `self_subject` constraint on a grant, answered by comparing two ids on the read path. It has no edge form that KYC can derive: it said *your own rows*, and KYC never learned which rows exist, let alone who owns them. A `scope_id` is an opaque string it never resolves.

So the merchant writes the fact instead, when the resource is created:

```
document:d1  #owner  app_user:ana
```

One edge, and the walk answers "is this thing yours?" with the comparison it already makes at the leaves for every other principal. No second mechanism.

Two consequences, and both are real.

**You now write an owner edge on every resource create.** The constraint cost no writes at all, so this is the largest thing the move asks of an integration. It is the same dual write containment already needs — a document has to say which project it is in before any walk can reach it — so it belongs in the same place in your create path.

**Ownership confers every action its type answers.** The constraint had no opinion on which verbs were allowed: `account:delete` was withheld by leaving it out of the role. An owner edge cannot do that. Preserving it would need *owner AND the grant*, and the grammar has no parentheses — terms associate strictly left to right, so `can_write + owner & self_write` parses as `(can_write + owner) & self_write` and the intersection swallows the union. If owners should not delete their own rows, declare `delete` on a type owners do not reach, rather than trimming a role.

The `self_subject` constraint is **gone**, not deprecated. It coexisted with the owner edge briefly and the two did not agree: the constraint was returned by `GET /access` and skipped by the projection, the owner edge was read by `POST /check` and invisible to `GET /access`. Same customer, two answers, depending which surface you asked, and both looked authoritative. A grant carrying `constraint` is now refused.

## Reading access back

```
GET /v1/app-users/{id}/access
```

```json
{
  "app_user_id": "01J…",
  "namespace": "org:01J…",
  "version": 1786374167,
  "grants": [
    {
      "id": "01J…",
      "scope_kind": "project",
      "scope_id": "apollo",
      "capabilities": ["docs:read", "docs:write"],
      "source": "group:au_customers app-role:editor",
      "all_capabilities": false,
      "constraint": ""
    }
  ]
}
```

**Every field here is load-bearing, and your backend must honour all of them.**
A grant with `all_capabilities` lists no capabilities and carries the most, and
`all_scopes` reaches further than any `scope_kind` it might have named. Code that
reads `capabilities` alone will allow more than was granted, and KYC cannot stop
it: the evaluation happens in your process. Read every field, not just
`capabilities`.

`expires_at` appears only on a grant that has one. The backend caches the set against `version` and evaluates locally through the SDK.

**`version` is a counter, and it moves on revocation.** It used to be the newest timestamp across your grants, roles and group memberships, which meant a *delete* moved nothing — revoking a grant, or removing somebody from a group, left the number where it was and your cache went on serving the permission that had just been taken away. Deleting the newest grant could even move it backwards, which a cache holding the higher value reads as current. Anything that changes what a customer holds now moves it, including declaring a capability, because a new capability widens every wildcard grant that already exists.

`source` records provenance — which role, and which group it arrived through. The customer's page in KYC renders the same thing as a sentence, which is what makes *why does this person have this?* answerable.

## Asking the graph

The cached grant set is still the **default** integration, for the reason it always was: a per-request check puts a network hop inside every request of your app and makes KYC own your latency.

But it is no longer the only surface, and it is no longer a separate world. Your roles, their inheritance, your groups, who is in them and every grant are projected into edges in your own namespace, so these three answer from the same facts the grant set is assembled from:

| Endpoint | Answers |
| --- | --- |
| `POST /v1/organisations/{id}/check` | May this subject do this to this resource? |
| `POST /v1/organisations/{id}/list-objects` | What can this subject reach? |
| `POST /v1/organisations/{id}/list-subjects` | Who can reach this? |

That was not true before. The Grants page wrote `app_grants` and the check read `reach_edges`, and nothing joined them: a grant you issued did not affect `POST /check`, and an edge you wrote never appeared in `GET /access`. There was no way to tell from the UI, and using one made the other lie.

**The check needs facts only you have.** KYC stores an opaque `scope_id` and never resolves it, so a walk cannot arrive at one of your documents unless you have said where it lives:

```
document:d1  #parent  project:apollo      containment
document:d1  #owner   app_user:ana        ownership
```

Write those as your resources are created. Everything else — roles, groups, grants — KYC already holds.

### The capability wildcard

A grant carrying *every capability* becomes one edge rather than a list:

```
project:apollo  #can_all  app_user:ana
```

It stays **standing**: declare a capability next quarter and this grant carries it, with nothing rewritten. That is the point of a wildcard, and it is why it is not expanded into a concrete list at projection time — a list would quietly stop covering the administrator it was issued to the moment your vocabulary grew.

The other two wildcards need no translation, because the star already lives in a node id: `app_user:*` is the everyone grant and `project:*` is every scope of a kind.

## Rules that hold

These are the [invariants](authorisation.md#invariants) as they apply to this namespace.

1. **Deny by default.** No grant, no access. A new customer holds nothing until a grant reaches them.
2. **No grant subtracts from another.** A grant cannot narrow itself and cannot veto a different one. That is what keeps grants unordered and evaluation first-match, and it is why the exclusion lists were removed rather than carried onto the graph.
3. **Capabilities are closed per namespace.** Open for the merchant, closed for KYC. The boundary is structural rather than checked: a merchant capability is stored in a table scoped to the organisation and stamped `org:<id>` whenever it is read, so it has no way to name anything in `kyc`. `CreateAppCapability` additionally validates the key through the evaluator's own registry, so a merchant cannot declare a shape KYC would reject.
4. **Out of scope is indistinguishable from absent.** A customer with no grant for a scope cannot tell it apart from a scope that does not exist.
5. **No principal grants what it does not hold.** Still not enforced on this tier, and the reason has changed. It used to be *waiting for the tier to be modelled on the graph*; the tier is modelled now and the rule is still not wired, so the boundary rests on point 3 alone. KYC's own write paths have it — `requireCanGrant` in `service/delegation.go`, stated for one edge as `CanWrite` in [`core/reach`](../core/reach) — and an operator issuing an app grant is bounded by the namespace rather than by what they hold. Tracked as phase 9 in [authorisation.md](authorisation.md#status-and-remaining-work).

## Where it lives

| Piece | Location |
| --- | --- |
| The engine | [`core/reach`](../core/reach) — a zero-dependency module, no database and no clock |
| Your schema, derived from your vocabulary | `internal/accessmodel/merchant.go` (`MerchantSchema`) |
| Your model, as edges | `internal/accessmodel/merchant_projection.sql` |
| Wire format (`AppGrant`, `AppScope`, `AppConstraint`) | `internal/service/app_grant.go` — inert types, no evaluator |
| Storage and assembly | `internal/service/app_access.go` |
| Check, list-objects, list-subjects | `internal/service/merchant_graph.go` |
| HTTP surface | `internal/http/app_access_handlers.go`, `internal/http/merchant_graph_handlers.go` |
| UI | Each organisation's **Customer access** section: Model, Roles & groups, Access, Playground |
| In-app docs | **Documentation → Concepts → Customer access** |

Design rationale, and how this fits KYC's own authorisation, are in [authorisation.md](authorisation.md#merchant-hosted-access-control).
