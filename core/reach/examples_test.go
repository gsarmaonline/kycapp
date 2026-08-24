package reach

import (
	"context"
	"testing"
	"time"
)

// The thirteen worked examples from the design, executed. If one of these stops
// passing, the design document is wrong, not just the code.

var at = time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)

func evalFor(t *testing.T, src string, edges ...Edge) *Evaluator {
	t.Helper()
	schema, err := Parse(src)
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	store := NewMemoryStore()
	if err := store.Write(edges...); err != nil {
		t.Fatalf("write edges: %v", err)
	}
	e, err := New(schema, store)
	if err != nil {
		t.Fatalf("new evaluator: %v", err)
	}
	return e
}

func edge(obj NodeRef, relation string, subject SubjectRef) Edge {
	return Edge{Object: obj, Relation: relation, Subject: subject}
}

func expiring(obj NodeRef, relation string, subject SubjectRef, when time.Time) Edge {
	return Edge{Object: obj, Relation: relation, Subject: subject, ExpiresAt: &when}
}

func mustAllow(t *testing.T, e *Evaluator, subject NodeRef, action string, resource NodeRef) Decision {
	t.Helper()
	d, err := e.Check(context.Background(), Request{Subject: subject, Action: action, Resource: resource}, at)
	if err != nil {
		t.Fatalf("check %s %s %s: %v", subject, action, resource, err)
	}
	if !d.Allowed {
		t.Fatalf("check %s %s %s: denied as %s, wanted allowed", subject, action, resource, d.Reason)
	}
	if len(d.Path) == 0 {
		t.Fatalf("check %s %s %s: allowed with an empty path", subject, action, resource)
	}
	return d
}

func mustDeny(t *testing.T, e *Evaluator, subject NodeRef, action string, resource NodeRef, want Reason) Decision {
	t.Helper()
	d, err := e.Check(context.Background(), Request{Subject: subject, Action: action, Resource: resource}, at)
	if err != nil {
		t.Fatalf("check %s %s %s: %v", subject, action, resource, err)
	}
	if d.Allowed {
		t.Fatalf("check %s %s %s: allowed, wanted %s", subject, action, resource, want)
	}
	if d.Reason != want {
		t.Fatalf("check %s %s %s: reason %s, wanted %s", subject, action, resource, d.Reason, want)
	}
	return d
}

// --- 01. A role over a tenant ---

const schemaRole = `
namespace org:acme
action read, write

relation member : transitive
relation admin  : direct

type user

type group
  relation member -> user | group#member

type org
  relation admin -> user | group#member
  rule read  = admin
  rule write = admin
`

func TestExample01RoleOverTenant(t *testing.T) {
	e := evalFor(t, schemaRole,
		edge(Node("org", "acme"), "admin", Userset(Node("group", "ops"), "member")),
		edge(Node("group", "ops"), "member", Subject(Node("user", "u9"))),
	)
	d := mustAllow(t, e, Node("user", "u9"), "write", Node("org", "acme"))
	if len(d.Path) != 2 {
		t.Fatalf("path %v, wanted two steps", d.Path)
	}
	mustDeny(t, e, Node("user", "u12"), "write", Node("org", "acme"), ReasonUnreachable)
}

// --- 02. Nesting, at any depth ---

func TestExample02NestingAtAnyDepth(t *testing.T) {
	e := evalFor(t, schemaRole,
		edge(Node("org", "acme"), "admin", Userset(Node("group", "ops"), "member")),
		edge(Node("group", "ops"), "member", Subject(Node("group", "platform"))),
		edge(Node("group", "platform"), "member", Subject(Node("group", "oncall"))),
		edge(Node("group", "oncall"), "member", Subject(Node("user", "u9"))),
	)
	mustAllow(t, e, Node("user", "u9"), "read", Node("org", "acme"))
}

// --- 03. Inheritance down a tree ---

const schemaTree = `
namespace org:acme
action read, write

relation member : transitive
relation parent : transitive
relation editor : direct

type user

type group
  relation member -> user | group#member

type folder
  relation parent -> folder
  relation editor -> user | group#member
  rule write = editor + parent->write

type document
  relation parent -> folder
  rule write = parent->write
`

func TestExample03InheritanceDownATree(t *testing.T) {
	e := evalFor(t, schemaTree,
		edge(Node("folder", "root"), "editor", Subject(Node("user", "u9"))),
		edge(Node("folder", "a"), "parent", Subject(Node("folder", "root"))),
		edge(Node("folder", "b"), "parent", Subject(Node("folder", "a"))),
		edge(Node("document", "d1"), "parent", Subject(Node("folder", "b"))),
	)
	d := mustAllow(t, e, Node("user", "u9"), "write", Node("document", "d1"))
	if len(d.Path) != 4 {
		t.Fatalf("path %v, wanted four steps", d.Path)
	}
	mustDeny(t, e, Node("user", "u12"), "write", Node("document", "d1"), ReasonUnreachable)
}

// --- 04. Only your own rows ---

const schemaOwn = `
namespace org:acme
action read, write

relation owner : direct

type user

type profile
  relation owner -> user
  rule read  = owner
  rule write = owner
`

func TestExample04OnlyYourOwnRows(t *testing.T) {
	e := evalFor(t, schemaOwn,
		edge(Node("profile", "p7"), "owner", Subject(Node("user", "u9"))),
	)
	mustAllow(t, e, Node("user", "u9"), "write", Node("profile", "p7"))
	mustDeny(t, e, Node("user", "u12"), "write", Node("profile", "p7"), ReasonUnreachable)
}

// --- 05. Everyone else's rows, never your own ---

const schemaClose = `
namespace org:acme
action close

relation member : transitive
relation owner  : direct
relation staff  : direct

type user

type group
  relation member -> user | group#member

type account
  relation owner -> user
  relation staff -> user | group#member
  rule close = staff - owner
`

func TestExample05NeverYourOwn(t *testing.T) {
	e := evalFor(t, schemaClose,
		edge(Node("account", "a1"), "owner", Subject(Node("user", "u9"))),
		edge(Node("account", "a1"), "staff", Userset(Node("group", "support"), "member")),
		edge(Node("account", "a2"), "staff", Userset(Node("group", "support"), "member")),
		edge(Node("group", "support"), "member", Subject(Node("user", "u9"))),
	)
	mustAllow(t, e, Node("user", "u9"), "close", Node("account", "a2"))
	mustDeny(t, e, Node("user", "u9"), "close", Node("account", "a1"), ReasonExcluded)
}

// --- 06. A classification that withholds ---

const schemaClassified = `
namespace org:acme
action read

relation member     : transitive
relation viewer     : direct
relation withheld   : direct
relation classified : direct

type user

type group
  relation member -> user | group#member

type clearance
  relation withheld -> user | group#member

type document
  relation viewer     -> user | group#member
  relation classified -> clearance
  rule read = viewer - classified->withheld
`

func TestExample06ClassificationWithholds(t *testing.T) {
	e := evalFor(t, schemaClassified,
		edge(Node("document", "d1"), "classified", Subject(Node("clearance", "secret"))),
		edge(Node("clearance", "secret"), "withheld", Userset(Node("group", "contractors"), "member")),
		edge(Node("group", "contractors"), "member", Subject(Node("user", "u4"))),
		edge(Node("document", "d1"), "viewer", Subject(Node("user", "u4"))),
		edge(Node("document", "d1"), "viewer", Subject(Node("user", "u9"))),
	)
	mustAllow(t, e, Node("user", "u9"), "read", Node("document", "d1"))
	mustDeny(t, e, Node("user", "u4"), "read", Node("document", "d1"), ReasonExcluded)
}

// --- 07. Access that expires by itself ---

func TestExample07AccessExpires(t *testing.T) {
	future := at.Add(24 * time.Hour)
	past := at.Add(-time.Second)

	live := evalFor(t, schemaTree,
		expiring(Node("folder", "payroll"), "editor", Subject(Node("user", "u9")), future),
	)
	mustAllow(t, live, Node("user", "u9"), "write", Node("folder", "payroll"))

	lapsed := evalFor(t, schemaTree,
		expiring(Node("folder", "payroll"), "editor", Subject(Node("user", "u9")), past),
	)
	mustDeny(t, lapsed, Node("user", "u9"), "write", Node("folder", "payroll"), ReasonUnreachable)
}

// --- 08. A machine principal ---

const schemaKey = `
namespace org:acme
action read, write

relation member : transitive
relation editor : direct
relation parent : transitive
relation actor  : identity

type user

type group
  relation member -> user | group#member

type key
  relation actor -> user | group#member

type folder
  relation parent -> folder
  relation editor -> user | group#member
  rule write = editor + parent->write
`

func TestExample08MachinePrincipal(t *testing.T) {
	e := evalFor(t, schemaKey,
		edge(Node("folder", "f1"), "editor", Subject(Node("user", "u9"))),
		edge(Node("key", "k4"), "actor", Subject(Node("user", "u9"))),
	)
	// The key holds nothing of its own; it reaches exactly what its actor does.
	mustAllow(t, e, Node("key", "k4"), "write", Node("folder", "f1"))
	mustDeny(t, e, Node("key", "k7"), "write", Node("folder", "f1"), ReasonUnreachable)
}

func TestMachinePrincipalStopsWhenTheEdgeGoes(t *testing.T) {
	store := NewMemoryStore()
	actor := edge(Node("key", "k4"), "actor", Subject(Node("user", "u9")))
	store.MustWrite(edge(Node("folder", "f1"), "editor", Subject(Node("user", "u9"))), actor)

	e, err := New(MustParse(schemaKey), store)
	if err != nil {
		t.Fatal(err)
	}
	mustAllow(t, e, Node("key", "k4"), "write", Node("folder", "f1"))

	if !store.Delete(actor) {
		t.Fatal("actor edge was not present")
	}
	mustDeny(t, e, Node("key", "k4"), "write", Node("folder", "f1"), ReasonUnreachable)
}

// --- 09. Everyone, and everything ---

const schemaWildcard = `
namespace org:acme
action read

relation member : transitive
relation viewer : direct, wildcard both

type user

type group
  relation member -> user | group#member

type document
  relation viewer -> user | group#member
  rule read = viewer
`

func TestExample09EveryoneAndEverything(t *testing.T) {
	e := evalFor(t, schemaWildcard,
		// every user reads d1
		edge(Node("document", "d1"), "viewer", Subject(Star("user"))),
		// audit reads every document, including ones that do not exist yet
		edge(Star("document"), "viewer", Userset(Node("group", "audit"), "member")),
		edge(Node("group", "audit"), "member", Subject(Node("user", "uA"))),
	)
	mustAllow(t, e, Node("user", "anyone"), "read", Node("document", "d1"))
	mustAllow(t, e, Node("user", "uA"), "read", Node("document", "d77"))
	mustDeny(t, e, Node("user", "u9"), "read", Node("document", "d77"), ReasonUnreachable)
}

func TestWildcardIsNamespaceScopedToItsType(t *testing.T) {
	e := evalFor(t, schemaWildcard,
		edge(Node("document", "d1"), "viewer", Subject(Star("user"))),
	)
	// user:* admits users. It must not admit a node of another type that
	// happens to reach the same walk.
	mustDeny(t, e, Node("group", "audit"), "read", Node("document", "d1"), ReasonUnreachable)
}

// --- 10. Two tenants, one engine ---

const schemaTenants = `
namespace org:acme
action read

relation member : transitive
relation parent : transitive
relation editor : direct
relation admin  : direct
relation org    : direct

type user

type group
  relation member -> user | group#member

type org
  relation admin -> user | group#member

type folder
  relation parent -> folder
  relation editor -> user | group#member
  relation org    -> org
  rule read = editor + parent->read + org->admin
`

func TestExample10TwoTenantsOneEngine(t *testing.T) {
	e := evalFor(t, schemaTenants,
		edge(Node("folder", "root_a"), "org", Subject(Node("org", "acme"))),
		edge(Node("folder", "root_b"), "org", Subject(Node("org", "globex"))),
		edge(Node("org", "acme"), "admin", Subject(Node("user", "u9"))),
	)
	mustAllow(t, e, Node("user", "u9"), "read", Node("folder", "root_a"))
	mustDeny(t, e, Node("user", "u9"), "read", Node("folder", "root_b"), ReasonUnreachable)
}

// --- 11. One thing, every thing, and in between ---

func TestExample11EveryGranularity(t *testing.T) {
	e := evalFor(t, schemaWildcard,
		edge(Node("document", "d1"), "viewer", Subject(Node("user", "u9"))),
		edge(Star("document"), "viewer", Userset(Node("group", "audit"), "member")),
		edge(Node("group", "audit"), "member", Subject(Node("user", "uA"))),
	)
	// One instance.
	mustAllow(t, e, Node("user", "u9"), "read", Node("document", "d1"))
	mustDeny(t, e, Node("user", "u9"), "read", Node("document", "d2"), ReasonUnreachable)
	// A whole type, present and future.
	mustAllow(t, e, Node("user", "uA"), "read", Node("document", "d1"))
	mustAllow(t, e, Node("user", "uA"), "read", Node("document", "d2"))
}

// --- 12. A delegation that is refused ---

func TestExample12DelegationRefused(t *testing.T) {
	e := evalFor(t, schemaTree,
		edge(Node("folder", "a"), "editor", Subject(Node("user", "u9"))),
		edge(Node("folder", "b"), "parent", Subject(Node("folder", "a"))),
	)
	ctx := context.Background()

	// u9 reaches folder:a and everything beneath it, so it may delegate there.
	within := edge(Node("folder", "b"), "editor", Subject(Node("user", "u12")))
	if err := e.CanWrite(ctx, Node("user", "u9"), within, CarveNone, at); err != nil {
		t.Fatalf("delegation within reach was refused: %v", err)
	}

	// folder:root is not beneath folder:a.
	beyond := edge(Node("folder", "root"), "editor", Subject(Node("user", "u12")))
	if err := e.CanWrite(ctx, Node("user", "u9"), beyond, CarveNone, at); err == nil {
		t.Fatal("delegation beyond reach was allowed")
	}
}

func TestDelegationOfAWildcardNeedsAWildcard(t *testing.T) {
	e := evalFor(t, schemaWildcard,
		edge(Node("document", "d1"), "viewer", Subject(Node("user", "u9"))),
	)
	ctx := context.Background()

	// u9 reaches one document, so it may not confer every document.
	whole := edge(Star("document"), "viewer", Subject(Node("user", "u12")))
	if err := e.CanWrite(ctx, Node("user", "u9"), whole, CarveNone, at); err == nil {
		t.Fatal("a wildcard was issued by a principal that holds none")
	}

	// A principal that already reaches the star node may.
	e2 := evalFor(t, schemaWildcard,
		edge(Star("document"), "viewer", Subject(Node("user", "root"))),
	)
	if err := e2.CanWrite(ctx, Node("user", "root"), whole, CarveNone, at); err != nil {
		t.Fatalf("a wildcard holder was refused: %v", err)
	}
}

func TestRootOfTrustSatisfiesTheSubsetRule(t *testing.T) {
	e := evalFor(t, schemaTree)
	beyond := edge(Node("folder", "root"), "editor", Subject(Node("user", "u12")))
	if err := e.CanWrite(context.Background(), Node("user", "nobody"), beyond, CarveRootOfTrust, at); err != nil {
		t.Fatalf("root of trust was refused: %v", err)
	}
}

// --- 13. What it will not express ---

func TestExample13ContainersUnionRatherThanNarrow(t *testing.T) {
	// "Lock it by moving it" is the expectation this defeats. A document in two
	// folders is reachable through either, so a move only ever widens.
	e := evalFor(t, schemaTree,
		edge(Node("folder", "open"), "editor", Subject(Node("user", "u9"))),
		edge(Node("document", "d1"), "parent", Subject(Node("folder", "open"))),
		edge(Node("document", "d1"), "parent", Subject(Node("folder", "locked"))),
	)
	mustAllow(t, e, Node("user", "u9"), "write", Node("document", "d1"))
}

func TestExample13NoEdgeMeansNoReach(t *testing.T) {
	// The only guarantee that nobody reaches a resource is that no edge does.
	e := evalFor(t, schemaTree,
		edge(Node("folder", "open"), "editor", Subject(Node("user", "u9"))),
	)
	mustDeny(t, e, Node("user", "u9"), "write", Node("document", "sealed"), ReasonUnreachable)
}
