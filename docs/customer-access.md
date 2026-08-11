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
5. **Issue grants** — one subject, one role, one scope, optionally with an expiry.
6. **Read the grant set back** — cache it against `version`, decide locally.

## The objects

### Scope kinds

A scope kind names a level in the merchant's product. A grant then reads as *the editor role over project apollo*, where `project` is the kind and `apollo` is the merchant's own id, which KYC never sees.

Kinds are **flat and independent, not a tree**. When a resource belongs to several containers at once, the check lists all of them: an object in two projects is reachable through either, and adding it to a third only widens access. Containment is a map lookup, never a traversal.

### Capabilities

One thing a customer may do: `invoices:read`, `docs:write`. The set is **open** — whatever the merchant declares is valid — but a role may use nothing outside it. That is the reason to declare them at all: a mistyped capability is rejected when the role is built, instead of quietly granting nothing at the moment it matters.

The merchant's capabilities live in the namespace `org:<id>`. KYC's own set stays **closed** and code-defined in the `kyc` namespace. A merchant admin can never mint a capability in KYC's namespace.

### Roles

A capability set with a name. Grants carry roles rather than raw capabilities, so *maintainer* can change in one place and every holder follows.

A role may **build on** others. What it resolves to is materialised at write time, so a check never walks a chain and editing a base role updates everything built on it. Roles never subtract, so the resolved set stays predictable however deep the chain runs. The depth cap is `MaxRoleDepth` in [`core/access`](../core/access).

### Groups

A named set of the merchant's customers. Granting a role to a group reaches every member, so onboarding a person becomes adding them to a group rather than reissuing grants.

Membership is an **explicit list**, not a query over attributes. A rule that recomputed silently would change who has access without anyone issuing anything, and the reason for the change would be invisible on the day it mattered. Group access and direct access add together.

### Grants

The only object that actually gives access; everything above is vocabulary. A grant binds one subject — a customer or a group — to one role over one scope, optionally until a date.

Grants are **issued and revoked, never edited**. Editing in place would rewrite what someone held at a past moment; revoke-and-reissue leaves a history that can be read.

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
      "source": "group:au_customers app-role:editor"
    }
  ]
}
```

`expires_at` appears only on a grant that has one. The backend caches the set against `version` and evaluates locally through the SDK.

This is the whole read surface: there is **no per-request check endpoint**, built or planned as the default. One would put a network hop inside every request of the merchant's app and make KYC own their latency. [authorisation.md](authorisation.md#ship-the-grant-set-not-a-per-request-check) leaves the door open to `POST /v1/access/check` for simple integrations; nothing implements it today.

`source` records provenance — which role, and which group it arrived through. The customer's page in KYC renders the same thing as a sentence, which is what makes *why does this person have this?* answerable.

## Rules that hold

These are the [invariants](authorisation.md#invariants) as they apply to this namespace.

1. **Deny by default.** No grant, no access.
2. **Additive only.** There are no deny rules, so no grant can take away what another gives.
3. **Capabilities are closed per namespace.** Open for the merchant, closed for KYC. The boundary is structural rather than checked: a merchant capability is stored in a table scoped to the organisation and stamped `org:<id>` whenever it is read, so it has no way to name anything in `kyc`. `CreateAppCapability` additionally validates the key through the evaluator's own registry, so a merchant cannot declare a shape KYC would reject.
4. **Out of scope is indistinguishable from absent.** A customer with no grant for a scope cannot tell it apart from a scope that does not exist.
5. **No principal grants what it does not hold.** `CanGrantInNamespace` in `core/access` implements this, but nothing calls it yet — the boundary currently rests on point 3 alone. Tracked in [authorisation.md](authorisation.md#merchant-hosted-access-control).

## Where it lives

| Piece | Location |
| --- | --- |
| Evaluator (`Decide`, `ExpandRoles`, delegation) | [`core/access`](../core/access) — a zero-dependency module, so the SDK can embed it |
| Storage and assembly | `internal/service/app_access.go` |
| HTTP surface | `internal/http/app_access_handlers.go` |
| UI | Each organisation's **Customer access** section, one page per object |
| In-app docs | **Documentation → Concepts → Customer access** |

Design rationale, and how this fits KYC's own authorisation, are in [authorisation.md](authorisation.md#merchant-hosted-access-control).
