# Customer access (merchant-hosted authorisation)

Authorisation a merchant runs **for their own customers**. The merchant declares the vocabulary, KYC stores the grants, and the merchant's backend decides.

Related: [authorisation](authorisation.md) · [authentication](authentication.md) · [data model](data-model.md) · [api](api.md) · in-app **Documentation → Concepts → Customer access**

## Contents

1. [What it is, and what it is not](#what-it-is-and-what-it-is-not)
2. [The flow](#the-flow)
3. [The objects](#the-objects)
4. [Reading access back](#reading-access-back)
5. [Rules that hold](#rules-that-hold)
6. [Where it lives](#where-it-lives)

---

## What it is, and what it is not

KYC already answers two access questions. Customer access is the third.

| Question | Governed by | Enforced by |
| --- | --- | --- |
| May this operator do this in KYC? | Members and permissions | KYC's gates |
| Did this organisation buy this? | Plans and entitlements | KYC's entitlement checks |
| **May this customer do this in the merchant's product?** | **Customer access** | **The merchant's backend** |

KYC does not enforce the third one, and cannot: it has no idea what a *project* of the merchant's is. Putting a check on the request path would also make KYC own the merchant's latency. So KYC stores the model and returns an assembled grant set; evaluation happens in the merchant's own process.

The subject is **app users only**. A merchant's own KYC operators keep organisation-wide roles. Scoping *them* to projects would mean KYC's gates learning which project every resource belongs to, which is a much larger change and is not done.

## The flow

1. **Declare scope kinds** — the levels the product has (`project`, `region`). The kind is registered; the ids stay in the merchant's system.
2. **Declare capabilities** — the verbs the backend checks, as `resource:action`.
3. **Compose roles** — named capability sets, which may build on other roles.
4. **Group customers** *(optional)* — grant to a set once instead of one at a time.
5. **Issue grants** — one subject, one set of capabilities, one scope, optionally with an expiry and exceptions.
6. **Read the grant set back** — cache it against `version`, decide locally.

## The objects

### Scope kinds

A scope kind names a level in the merchant's product. A grant then reads as *the editor role over project apollo*, where `project` is the kind and `apollo` is the merchant's own id, which KYC never sees.

Kinds are **flat and independent, not a tree**. When a resource belongs to several containers at once, the check lists all of them: an object in two projects is reachable through either, and adding it to a third only widens access. Containment is a map lookup, never a traversal.

### Capabilities

One thing a customer may do: `invoices:read`, `docs:write`.

**Nothing is seeded.** A new organisation starts with an empty vocabulary, deliberately: this set is the merchant's own, so a default shipped by KYC is a guess about a product it has never seen, and a wrong guess is one merchants work around rather than delete. The empty Capabilities page instead offers **starter templates** a merchant applies with one click, after seeing the full list. Every row a template creates records `source`, so "why does this capability exist?" stays answerable once the person who applied it has moved on. An authored capability keeps its authored provenance even when a template later names the same key. The set is **open** — whatever the merchant declares is valid — but a role may use nothing outside it. That is the reason to declare them at all: a mistyped capability is rejected when the role is built, instead of quietly granting nothing at the moment it matters.

The merchant's capabilities live in the namespace `org:<id>`. KYC's own set stays **closed** and code-defined in the `kyc` namespace. A merchant admin can never mint a capability in KYC's namespace.

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

**A grant may narrow itself.** Three exclusion lists, one per wildcard:

| Wildcard | Exclusion | Reads as |
| --- | --- | --- |
| everyone | `except_app_user_ids` | everyone except these customers |
| all capabilities | `except_capabilities` | everything except `account:delete` |
| a wide scope | `except_scopes` | all of Acme except `project:salaries` |

A wildcard is a claim about a set nobody can enumerate; an exception names the members that do not belong. They are one feature, and each exception narrows **the grant it sits on and nothing else**. So no grant subtracts from another, grants stay unordered, and deleting one still removes access rather than adding it.

The limit that follows: an exclusion is **not a lock**. If a second grant reaches an excluded resource, that grant allows. For a hard "nobody reaches this", issue nothing that reaches it.

**Constraint** narrows a grant with something only the request knows. There is one: `self_subject`, which applies the grant only to resources belonging to the holder. Combined with the everyone subject it is the whole "customers may manage their own things" rule, in one row:

```
subject: everyone   role: self_manager   scope: tenant:acme   constraint: self_subject
```

Note what the constraint does **not** do. It has no opinion on which verbs are allowed. Account deletion is prevented by leaving `account:delete` out of the role, never by the constraint, which only ever answers "is this thing yours?".

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
      "except_capabilities": [],
      "except_scopes": [{ "kind": "project", "id": "salaries" }],
      "constraint": ""
    }
  ]
}
```

**Every field here is load-bearing, and your backend must honour all of them.**
A grant with `all_capabilities` lists no capabilities and carries the most; one
with `except_scopes` reaches less than its scope suggests; one with
`"constraint": "self_subject"` applies only to rows the holder owns. Code that
reads `capabilities` alone will allow more than was granted, and KYC cannot stop
it — the evaluation happens in your process. Read every field, not just
`capabilities`.

`expires_at` appears only on a grant that has one. The backend caches the set against `version` and evaluates locally through the SDK.

This is the whole read surface: there is **no per-request check endpoint**, built or planned as the default. One would put a network hop inside every request of the merchant's app and make KYC own their latency. [authorisation.md](authorisation.md#ship-the-grant-set-not-a-per-request-check) leaves the door open to `POST /v1/access/check` for simple integrations; nothing implements it today.

`source` records provenance — which role, and which group it arrived through. The customer's page in KYC renders the same thing as a sentence, which is what makes *why does this person have this?* answerable.

## Rules that hold

These are the [invariants](authorisation.md#invariants) as they apply to this namespace.

1. **Deny by default.** No grant, no access. A new customer holds nothing until a grant reaches them.
2. **No grant subtracts from another.** A grant may narrow itself with exceptions; it can never veto a different grant. That is what keeps grants unordered and evaluation first-match.
3. **Capabilities are closed per namespace.** Open for the merchant, closed for KYC. The boundary is structural rather than checked: a merchant capability is stored in a table scoped to the organisation and stamped `org:<id>` whenever it is read, so it has no way to name anything in `kyc`. `CreateAppCapability` additionally validates the key through the evaluator's own registry, so a merchant cannot declare a shape KYC would reject.
4. **Out of scope is indistinguishable from absent.** A customer with no grant for a scope cannot tell it apart from a scope that does not exist.
5. **No principal grants what it does not hold.** Not yet enforced on this tier; the boundary rests on point 3 alone. The KYC tier has it (`CanWrite` in [`core/reach`](../core/reach)), and this tier gets it when it is modelled there.

## Where it lives

| Piece | Location |
| --- | --- |
| Role inheritance (`ExpandSets`) | [`core/reach`](../core/reach) — a zero-dependency module |
| Wire format (`AppGrant`, `AppScope`, `AppConstraint`) | `internal/service/app_grant.go` — inert types, no evaluator |
| Storage and assembly | `internal/service/app_access.go` |
| HTTP surface | `internal/http/app_access_handlers.go` |
| UI | Each organisation's **Customer access** section, one page per object |
| In-app docs | **Documentation → Concepts → Customer access** |

Design rationale, and how this fits KYC's own authorisation, are in [authorisation.md](authorisation.md#merchant-hosted-access-control).
