# Access by reachability

> **Status: cut over. Every KYC gate now evaluates through this engine.**
> [`core/reach`](../core/reach) is a standalone Go module with no dependencies
> outside the standard library. The merchant tier is not yet modelled and still
> uses the previous evaluator; see
> [Replacing the current system](#replacing-the-current-system).

The published design is [Access by Reachability][artifact]. This document is the
repository's copy, with pointers into the code.

[artifact]: https://claude.ai/code/artifact/5b8d4a0f-6373-4182-bcb7-3a0e1d27c181

---

## Contents

- [The thesis](#the-thesis)
- [Two primitives and one flag](#two-primitives-and-one-flag)
- [A decision is a path](#a-decision-is-a-path)
- [The objects](#the-objects)
- [The schema language](#the-schema-language)
- [A tag is a node](#a-tag-is-a-node)
- [Wildcards](#wildcards)
- [Exclusion without precedence](#exclusion-without-precedence)
- [Delegation](#delegation)
- [Who owns the graph](#who-owns-the-graph)
- [Costs this design owns](#costs-this-design-owns)
- [Prior art](#prior-art)
- [Replacing the current system](#replacing-the-current-system)

---

## The thesis

Every access question is a path question: **may this subject perform this action
on this resource?**

Subjects, groups, roles, containers, tags and actions are all nodes. Grouping,
containment, ownership and implication are all edges. One walk answers
everything.

---

## Two primitives and one flag

See [`graph.go`](../core/reach/graph.go) and [`schema.go`](../core/reach/schema.go).

| Primitive | Definition | Example |
| --- | --- | --- |
| **Object** | A node: a type and an id, and nothing else. | `document:d1` |
| **Edge** | One fact joining two nodes under a named relation. | `document:d1 #parent folder:f2` |
| **transitive** | A flag on a relation. The walk follows it to closure. | `relation member : transitive` |

Read an edge as *"A's relation is B"*.

That flag is the whole grouping mechanism. Nine familiar concepts collapse into
it, so none of them can drift from the others:

| What people call it | What it is here |
| --- | --- |
| Group | a node, plus `member` edges |
| Nested group | the same edges, followed to closure |
| Role | a node, plus `grants` edges |
| Collection | a node, plus `contains` edges |
| Folder hierarchy | `parent` edges |
| Tag, label, department | a node, plus `tagged` edges |
| Ownership | an `owner` edge |
| Everything of a type | the `T:*` node |
| Action cover | `implies` edges between type-action nodes |

---

## A decision is a path

See [`walk.go`](../core/reach/walk.go).

```go
Decision {
    Allowed bool
    Reason  allowed | unreachable | no_rule | excluded
    Path    []Step   // every edge walked, in order
}
```

| Reason | Meaning | Status |
| --- | --- | --- |
| `allowed` | A path reached the resource and a rule granted the action. | 200 |
| `unreachable` | No path of any kind reaches the resource. | **404** |
| `no_rule` | A path reaches it, but no rule grants this action. | 403 |
| `excluded` | A rule matched, and schema subtraction removed it. | 403 |

`unreachable` must be indistinguishable from a resource that does not exist, or
the tenants of the system become enumerable by status code alone. The evaluator
pays for that distinction with one extra pass over the type's other rules, on
the denial path only.

Because the answer is a path rather than a flag, `Decision.Path` is the primary
debugging tool. The graph is homogeneous — groups, roles, folders, tags and
actions are all nodes joined by edges — so "why can this person reach this?"
cannot be answered without one.

---

## The objects

**Schema** — declared once per namespace, validated before it runs.

| Object | What it is |
| --- | --- |
| Namespace | Tenancy boundary. Owns exactly one schema. |
| Action | A verb in the namespace vocabulary. |
| Type | A resource type. |
| Relation | An edge kind: name, legal target types, transitive flag, wildcard positions. |
| Rule | The walk specification: which relations answer which action on which type. |

**Data** — written by the application, in the same transaction as the thing it
describes.

| Object | What it is |
| --- | --- |
| Object | A node: a type and an id. |
| Edge | One fact, with an optional expiry. This is the entire data model. |

**Runtime** — never stored: `Request`, `Decision`, `Resolver`, and a version
token for safe caching.

Subjects are Objects. `user:u9` is a node; `group:eng#member` is a set of nodes
reached by an edge. The subject domain gets no vocabulary of its own.

### What is deliberately absent

`Capability`, `Role`, `Scope`, `Grant`, `Selector` and `Attribute` are not in
this model. Each was a second mechanism for a job the graph already does, and
each is reconstructed above as a node plus an edge.

---

## The schema language

See [`parse.go`](../core/reach/parse.go). Anyone declares a new type, action or
relation. Nobody adds an operator.

```
namespace org:acme

action read, write, delete, share

relation member  : transitive        # grouping
relation parent  : transitive
relation viewer  : direct, wildcard both
relation owner   : direct
relation banned  : direct
relation tagged  : direct
relation reader  : direct

type user

type group
  relation member -> user | group#member

type tag
  relation reader -> user | group#member

type folder
  relation parent -> folder
  relation editor -> user | group#member
  rule read  = viewer + editor + parent->read
  rule write = editor + parent->write

type document
  relation parent -> folder
  relation owner  -> user
  relation viewer -> user | group#member
  relation tagged -> tag
  relation banned -> user | group#member
  rule read  = viewer + write + tagged->reader
  rule write = owner + parent->write
  rule share = write - banned
```

The grammar is exactly: **union** (`+`), **intersection** (`&`),
**subtraction** (`-`), **traversal** (`->`), and a **reference to another rule
on the same type**. No comparison operator, no arithmetic, no function call.
Operators associate to the left and there are no parentheses, so `a + b & c`
reads as `(a + b) & c`.

A few rules the validator enforces, each of which has caught a real mistake in
the tests:

- A relation used by a type must be declared at namespace level.
- A relation's targets must name declared types, and a userset target must name
  a relation that type actually carries.
- `parent->read` requires the far type to answer `read`, as a rule or a relation.
- A rule may not reference itself, directly or through a chain.
- A wildcard may only sit where the relation declares it.

`Schema.Warnings()` reports declarations that are true as written but inert:
a relation nothing carries, an action no type resolves, and a relation marked
`transitive` that is only ever crossed by an arrow. The last one is a real
footgun, because transitivity is read only where a relation is evaluated on its
own; a relation appearing solely on the near side of an arrow is followed
exactly one hop, and the depth comes from the far rule naming the arrow again.
`accessmodel.Load` treats any warning as fatal.

A non-transitive relation pointing at a group needs the explicit userset form
(`group:ops#member`). A transitive relation chains plain nodes on its own.

### Type and action stay independent

An action belongs to the namespace vocabulary; a type declares which actions it
resolves and how. So the same verb means the same thing to a reader across every
type, while each type keeps control of what satisfies it. A type that never
declares `rule delete` is unreachable by any delete question.

### Reading a schema back

`Schema.Mermaid()` renders a namespace as a diagram, and `Schema.Describe()`
counts what it declares. Both are pure functions of the schema text, which is
the property that makes them worth having in the portable core rather than in
one application's admin pages: a tenant who declares their own namespace gets
their own diagram with no further code.

This draws the schema, never the edges. The distinction is what makes it safe
to call. A schema is a dozen types whatever the tenant, while the edge set is
unbounded and a picture of all of it is a grey disc at every zoom level. Bounded
views of the edges are a separate thing: a decision path, or one subject's
neighbourhood at fixed depth.

Every collection is sorted before it is written, so the same schema always
renders the same bytes and a diagram can be committed and diffed like source.

One judgement call is in the output. Drawn naively, every grant relation on
every type draws its own arrow to each of `user`, `key`, `recovery` and `role`,
which is a picture of the repetition rather than of the model. So a target set
used by more than one relation is collapsed into a single node, and relations
that agree about where they point merge onto one arrow. A set used *once* is
never collapsed, because hiding the one type it names would trade a true
picture for a tidy one.

Rendering the KYC namespace surfaces something the text hides: thirteen of the
eighteen types are structurally identical, differing only in name. That is worth
a decision rather than an accident.

---

## A tag is a node

There is no attribute mechanism and no predicate language. A tag, a department
and a classification are each a node, and to carry one is an edge.

```
// written by the application, as its data changes
document:d1  #tagged  tag:eu
group:finance #member user:u9

// written once, by an administrator
tag:eu       #reader  group:finance#member

// already in the schema
rule read = viewer + write + tagged->reader
```

One edge carries the whole policy. A predicate language would need its own
index, its own subset check for delegation, and its own second path through the
evaluator. This needs none of that.

Ownership follows the same shape. *"Only your own rows"* is `rule write = owner`.
*"Everyone else's rows, never your own"* is `rule close = staff - owner`. Neither
needs a variable or a special case.

**The cost is dual writes.** When the application tags a document it writes a
`#tagged` edge in the same transaction. A permission change and a data change
become the same event, and one policy sentence becomes one edge per resource it
touches.

---

## Wildcards

Every type has a star node. `document:*` stands for every object of that type in
the namespace, including ones that do not exist yet.

| Position | Edge | Means |
| --- | --- | --- |
| Subject | `document:d1 #viewer user:*` | every user views d1 |
| Object | `document:* #viewer group:audit#member` | audit views every document |
| Both | `document:* #viewer user:*` | this type is public |

A relation declares where a wildcard is legal:

```
relation viewer : direct,     wildcard both
relation editor : direct,     wildcard subject
relation parent : transitive, wildcard none
```

Without that declaration, `document:* #parent folder:public` would reparent
every document in a tenant with one write. The declaration is also the
enforcement point: the walk does not even read a star edge on a relation that
forbids one.

Three limits:

- **Namespace-scoped.** `user:*` is every user in this namespace, never another
  tenant's.
- **`*:*` is not writable.** Reach over every type at once is the
  environment-derived root of trust, and it stays outside the data.
- **A wildcard names a type, not a property.** "Every document" is one edge;
  "every document tagged `eu`" still needs the tag node and its per-document edges.

A wildcard is one row per type and relation however many objects exist, so it
costs the index nothing.

---

## Exclusion without precedence

Subtraction belongs in the schema, where it is a set expression evaluated at one
point:

```
rule share = write - banned
```

That resolves the same way whichever edges exist and in whatever order they
arrive. No priority field, no conflict resolution, no debugging which rule won.

A free-floating **deny edge** — a fact whose job is to veto another fact — is
the thing to refuse. It is a priority contest, and it drags ordering into every
question afterwards.

**The rule to hold:** subtraction is declared in a rule, and no edge vetoes
another edge.

> **Consequence to state in the UI.** Reachability is additive, so multiple
> containers **union**. A document in two folders is reachable through either.
> People expect *"move it into the locked folder to lock it down"*, and they will
> not get it.

---

## Delegation

See [`delegate.go`](../core/reach/delegate.go).

**No principal grants what it does not hold.** Before an edge is written, ask
whether the writer already reaches what that edge would confer. Every action the
proposed relation feeds on the object's type must already be allowed to the
granter at that same object.

Three properties keep the question answerable:

- **Traversal.** *"Is beneath Y inside beneath X"* is a walk over the graph that
  is already present.
- **Sets.** Containment falls out of the same transitive walk.
- **Rules.** A rule is a finite expression over declared relations, so what an
  edge can confer is enumerable from the schema alone.
- **Wildcards.** Only a principal that already reaches `T:*` may write an edge on
  it, so "every document" never becomes a way to reach documents you could not
  reach one at a time.

There is no predicate language anywhere in the model, so this never becomes a
satisfiability problem. Graph walks cost performance, which is an engineering
problem. Undecidability would not have been.

Only the positive side of a subtraction counts. An edge that appears under a
minus removes access rather than conferring it, so writing one is not an
escalation.

**One carve-out:** `CarveRootOfTrust`, an environment-derived credential that
holds everything by definition. The rule is satisfied rather than bypassed, and
the system stays recoverable when the store is empty or freshly restored. It is a
named type rather than a boolean so an audit can count it. A rising count is a
signal.

---

## Who owns the graph

Traversal means there is no finite answer set to ship, because reachability
depends on a graph that is unbounded and changes constantly.

| Holder | Owns |
| --- | --- |
| Platform | the schema, and the subject, group and role edges |
| Tenant backend | the resource graph — hierarchy, ownership and tags |

The evaluator walks and calls a `Resolver` for the edges it does not hold:

```go
type Resolver interface {
    Edges(ctx context.Context, object NodeRef, relation string) ([]Edge, error)
}
```

Evaluation runs inside the tenant's own backend with no network hop per
decision, and the platform never learns what a resource id means. The platform's
own resources use the same engine with a resolver over its own store. One
evaluator, two resolvers.

---

## Costs this design owns

**Cycles are possible.** A transitive relation can close a loop. Reject the
closing edge at write time so the data stays sane, *and* carry a visited set in
the walk so a cycle arriving through concurrent writes terminates the request
instead of hanging it. Both are implemented; see `TestCycleTerminates`.

**Every data write is a permission write.** Make the edge write part of the same
transaction as the change that caused it, or the two will drift and the drift
will be invisible.

**Depth is bounded by the runtime, not the model.** `DefaultMaxDepth` is a
circuit breaker that returns `ErrDepthExceeded` loudly. A bound that denies
silently hides a modelling mistake for months.

**Performance, in the order it will be needed:**

1. A reverse index over the closure, rebuilt incrementally as edges are written.
2. A consistency token, so a cache serves a bounded-stale answer without lying.
3. The bounded walk, already in place.

---

## Prior art

The core of this model is **Zanzibar** (Google, 2019), and the correspondence is
direct rather than a family resemblance:

| Here | Zanzibar |
| --- | --- |
| `Edge` | relation tuple |
| a group is a node | userset |
| `Rule` | userset rewrite rules |
| `read = viewer + write` | `computed_userset` |
| `parent->read` | `tuple_to_userset` |
| `share = write - banned` | the `exclusion` operator |
| Schema | namespace configuration |
| Version | zookie |
| Reverse index | Leopard index |
| `Decision.Path` | the Expand API's userset tree |

**Steal the schema semantics wholesale.** They are a decade-tested design, and
[SpiceDB](https://authzed.com/spicedb) and
[OpenFGA](https://openfga.dev) are mature implementations worth reading.

Four things here are ours rather than theirs:

1. **Action as a namespace vocabulary.** Zanzibar's relation name *is* the
   permission name, per object type, so "read anything across every type" is not
   a single query.
2. **The `transitive` flag.** Sugar for a recursive rewrite Zanzibar already
   supports — a better interface, not a new capability.
3. **The delegation subset rule.** Zanzibar has none; who may write a tuple is
   enforced by the calling service.
4. **Split evaluation through a `Resolver`.** Zanzibar is a centralised service.
   This is the one real reason to build rather than adopt, and it is also where
   the consistency story is hardest: zookies exist *because* Zanzibar is
   centralised.

---

## Replacing the current system

The engine is built. Nothing is wired to it, and the old system is untouched and
still authoritative. That is deliberate: the current implementation is wired into
roughly 150 routes, the service gates, three migrations and the web pages, so
deleting it before there is a proven replacement leaves the app with no working
authorisation and nothing to compare against.

**Done:**

| Phase | Work | Where |
| --- | --- | --- |
| 1 | Schema language and validator | `core/reach/schema.go`, `parse.go` |
| 2 | `Resolver` interface, in-memory implementation | `core/reach/graph.go` |
| 3 | The walk: visited set, depth bound, path, reasons | `core/reach/walk.go` |
| 4 | The delegation check | `core/reach/delegate.go` |
| 5 | KYC's own domain expressed in the schema | [`internal/accessmodel/schema.go`](../internal/accessmodel/schema.go) |
| 6 | Edge table and its constraints | [`migrations/000050_reach_edges.up.sql`](../internal/store/migrations/000050_reach_edges.up.sql) |
| 7 | Postgres resolver | [`internal/accessmodel/resolver.go`](../internal/accessmodel/resolver.go) |
| 8 | Projection of the current model into edges | [`internal/accessmodel/projection.sql`](../internal/accessmodel/projection.sql) |
| 9 | Live view of the current tables, shaped as edges | [`migrations/000051_reach_edges_live.up.sql`](../internal/store/migrations/000051_reach_edges_live.up.sql) |
| 10 | Every KYC gate evaluating through the walk | [`internal/service/access.go`](../internal/service/access.go) |

All thirteen worked examples from the design run as tests, and
`TestEveryPermissionKeyMaps` checks the projection table against the registry
the current system boots from, so a permission added on either side without the
other fails the build.

### What modelling the domain found

Expressing the current system in the new model produced three results worth
recording.

**Intersection was missing, and is now in the grammar.** Nothing else expresses
a principal narrowed by two independent things at once. It is a set operation
evaluated at one point, so it introduces no ordering and keeps the delegation
subset question decidable.

**Global reach became an edge, and least privilege came free.** Platform-org
membership projects to the same `can_<action>` edge written on a star node.
There is no flag and no privileged role name, so a read-only support role stays
read-only in every tenant, which is a defect the current system documents but
cannot fix without this change.

**A machine credential is just a principal.** The current system derives a
key's power from its owner and narrows it by a scope list, re-computed on every
request. That is not ported. A key is a node, it holds whatever edges name it,
and its bound is the subset rule at write time: `CanWrite` refuses to issue a
key anything its creator does not already hold.

That is simpler and it is better. "What can this key do?" is answered by reading
its edges instead of simulating a derivation. Revocation is deleting them.
Expiry already lives on edges, so a time-boxed key costs nothing extra. And it
removes the wart the derived model has to document: *a key that has to outlive
its owner's involvement must be moved to someone else before they are
offboarded.* An `owner` edge remains, for lifecycle only, so a departing
person's keys can still be found and swept. It confers nothing.

The knock-on is that `identity` had no remaining user and has been removed from
the engine. It was the only feature that made the walk bidirectional; every
other term runs inward from the resource. The evaluator is now uniformly
inward, which is simpler to reason about and to index.

### Storage

One table holds the entire data model. There is no capability table, no scope
table and no grant table, because a group, a role, a container, a tag and an
action are all nodes, and every fact about them is an edge.

The resolver's only query is an exact prefix of the primary key, so it stays an
index lookup however large the table grows. Expiry is not filtered in SQL: the
evaluator is handed the time to decide against, so the query and the walk cannot
disagree about what "now" means.

Two constraints are enforced by the database rather than by application code:
`object_type` and `subject_type` may not be `*`, because reach over every type
at once is the environment-derived root of trust and stays outside the data; and
a userset may not sit on a star node, because it would name nobody in particular.

### The projection

`accessmodel.Project` applies [`projection.sql`](../internal/accessmodel/projection.sql).
It is a function rather than a data migration on purpose: a projection buried in
a schema change can be neither re-run nor tested, and this one has to be both.
Every statement is `ON CONFLICT DO NOTHING`, so it is idempotent and can run
again at cutover to pick up whatever was written since.

| Source | Becomes |
| --- | --- |
| `role_permissions` | `<area>:<org> #can_<action> role:<id>#holder` |
| a role, by existing | `organisation:<org> #belongs role:<id>#holder` |
| a role in the platform organisation | the same edges on the `*` node, plus `organisation:* #oversees` |
| `memberships` | `role:<id> #holder user:<id>`, carrying its expiry |
| a non-active organisation | `organisation:<id> #suspended role:<r>#holder` |
| `api_keys` | the key's own grant edges, plus `key:<id> #owner user:<id>` |

Global reach is derived from membership of the platform organisation, exactly as
`ListUserGrantSources` derives it, and never stored. The `COALESCE` keeps it
failing closed: with no `system_state` row the comparison is NULL and nobody
gets reach.

An API key's scope list is applied **once**, here. What a key can do afterwards
is readable rather than simulated, and revoking it is deleting rows.

### How the cutover was done

Not by dual-writing. Every path that changes authorisation -- a role edit, an
invite, a removal, a suspension, a key issued or revoked -- would have needed to
write edges too, and any one missed is a silent hole.

Instead the resolver reads **`reach_edges_live`**, a view presenting the current
tables as edges, unioned with anything written directly. State cannot diverge,
because it is the same rows. What changed is only how those rows are
interpreted: by a walk that returns the path it took.

So the gates in [`access.go`](../internal/service/access.go) were swapped
outright, and the existing suite became the differential test. It passes
unchanged, which is the evidence that the two engines agree.

Three things fell out of the swap:

- **Grant assembly is gone.** `grantsFor`, `keyGrants` and
  `grantsFromMemberships` are deleted. `grants.go` is now the permission catalog
  and nothing else.
- **Recovery credentials stopped being a short-circuit.** They are edges on the
  star nodes, so they go through the same walk a membership does and appear in
  `Decision.Path`.
- **Break-glass is the only principal outside the graph**, and deliberately so:
  reach that has to survive an empty store cannot be derived from the store.

Writing it caught one real bug before it shipped. A key was given its permission
edges but no `belongs` edge, so it held permissions in a tenant it could not
see, and every gate answered 404 before ever asking about the permission. The
view now gives a key the organisations its owner reaches.

**Not done, in the order it should happen:**

1. **Model the merchant tier.** `app_access.go` still uses the previous
   evaluator. It is the half with open vocabulary, so it will exercise the
   namespace boundary harder than KYC's own tier did. on the same requests and log every
   disagreement. The current suite becomes the differential test.
2. **Retire the view's legacy branches**, one at a time. Each removal is paired
   with running the projection so those edges become rows in `reach_edges`, and
   with teaching the write path that changes them to write edges directly.
3. **Delete the old tables** once nothing reads them. `core/access` itself is
   already gone: the merchant tier now owns its wire format and borrows only
   `reach.ExpandSets` for role inheritance.
4. **The reverse index**, behind an unchanged interface. Not before there is a
   storage engine to index.
