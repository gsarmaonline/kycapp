package reach

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// --- Denial reasons ---

func TestUnreachableIsDistinctFromNoRule(t *testing.T) {
	// The distinction is what keeps a 404 honest. A caller with no reach at all
	// must not be able to tell an existing resource from a missing one.
	e := evalFor(t, schemaTree,
		edge(Node("folder", "f1"), "editor", Subject(Node("user", "u9"))),
	)

	// u9 reaches f1 through write, but read has no rule on folder at all.
	mustDeny(t, e, Node("user", "u9"), "read", Node("folder", "f1"), ReasonNoRule)
	// u12 reaches nothing.
	mustDeny(t, e, Node("user", "u12"), "read", Node("folder", "f1"), ReasonUnreachable)
}

func TestDenyByDefault(t *testing.T) {
	e := evalFor(t, schemaTree)
	mustDeny(t, e, Node("user", "u9"), "write", Node("folder", "f1"), ReasonUnreachable)
}

func TestUnknownActionFailsRatherThanDenies(t *testing.T) {
	e := evalFor(t, schemaTree)
	_, err := e.Check(context.Background(), Request{
		Subject: Node("user", "u9"), Action: "raed", Resource: Node("folder", "f1"),
	}, at)
	if !errors.Is(err, ErrUnknownAction) {
		t.Fatalf("err %v, wanted ErrUnknownAction", err)
	}
}

func TestUnknownTypeFailsRatherThanDenies(t *testing.T) {
	e := evalFor(t, schemaTree)
	_, err := e.Check(context.Background(), Request{
		Subject: Node("user", "u9"), Action: "write", Resource: Node("widget", "w1"),
	}, at)
	if !errors.Is(err, ErrUnknownType) {
		t.Fatalf("err %v, wanted ErrUnknownType", err)
	}
}

// --- Order independence ---

func TestReachabilityIsOrderIndependent(t *testing.T) {
	// Reachability only ever adds, so the order edges are written in cannot
	// change the answer. This is the property that removes precedence rules.
	forward := evalFor(t, schemaTree,
		edge(Node("folder", "f1"), "editor", Subject(Node("user", "u9"))),
		edge(Node("folder", "f1"), "editor", Userset(Node("group", "ops"), "member")),
		edge(Node("group", "ops"), "member", Subject(Node("user", "u12"))),
	)
	reverse := evalFor(t, schemaTree,
		edge(Node("group", "ops"), "member", Subject(Node("user", "u12"))),
		edge(Node("folder", "f1"), "editor", Userset(Node("group", "ops"), "member")),
		edge(Node("folder", "f1"), "editor", Subject(Node("user", "u9"))),
	)
	for _, u := range []string{"u9", "u12"} {
		mustAllow(t, forward, Node("user", u), "write", Node("folder", "f1"))
		mustAllow(t, reverse, Node("user", u), "write", Node("folder", "f1"))
	}
}

// --- Cycles and depth ---

func TestCycleTerminates(t *testing.T) {
	// A cycle in the data must terminate the walk rather than hang it. Writers
	// should reject the closing edge, but concurrent writes can still produce
	// one, so the walk carries a visited set.
	e := evalFor(t, schemaRole,
		edge(Node("group", "a"), "member", Subject(Node("group", "b"))),
		edge(Node("group", "b"), "member", Subject(Node("group", "a"))),
		edge(Node("org", "acme"), "admin", Userset(Node("group", "a"), "member")),
	)
	done := make(chan struct{})
	go func() {
		defer close(done)
		mustDeny(t, e, Node("user", "u9"), "read", Node("org", "acme"), ReasonUnreachable)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("walk did not terminate on a cycle")
	}
}

func TestCycleStillFindsAMemberBeyondIt(t *testing.T) {
	e := evalFor(t, schemaRole,
		edge(Node("group", "a"), "member", Subject(Node("group", "b"))),
		edge(Node("group", "b"), "member", Subject(Node("group", "a"))),
		edge(Node("group", "b"), "member", Subject(Node("user", "u9"))),
		edge(Node("org", "acme"), "admin", Userset(Node("group", "a"), "member")),
	)
	mustAllow(t, e, Node("user", "u9"), "read", Node("org", "acme"))
}

func TestDepthBoundErrorsLoudly(t *testing.T) {
	// A bound that denies silently hides a modelling mistake for months.
	store := NewMemoryStore()
	for i := 0; i < 40; i++ {
		store.MustWrite(edge(
			Node("group", string(rune('a'+i%26))+string(rune('0'+i/26))),
			"member",
			Subject(Node("group", string(rune('a'+(i+1)%26))+string(rune('0'+(i+1)/26)))),
		))
	}
	store.MustWrite(edge(Node("org", "acme"), "admin", Userset(Node("group", "a0"), "member")))

	e, err := New(MustParse(schemaRole), store, WithMaxDepth(5))
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.Check(context.Background(), Request{
		Subject: Node("user", "u9"), Action: "read", Resource: Node("org", "acme"),
	}, at)
	if !errors.Is(err, ErrDepthExceeded) {
		t.Fatalf("err %v, wanted ErrDepthExceeded", err)
	}
}

// --- Nodes and edges ---

func TestStarOfStarIsNotWritable(t *testing.T) {
	// Reach over every type at once is the environment-derived root of trust,
	// and it stays outside the data where it can be counted.
	err := NewMemoryStore().Write(edge(Node(Wildcard, "x"), "viewer", Subject(Node("user", "u9"))))
	if !errors.Is(err, ErrInvalidNode) {
		t.Fatalf("err %v, wanted ErrInvalidNode", err)
	}
}

func TestUsersetOnAWildcardIsRejected(t *testing.T) {
	err := NewMemoryStore().Write(edge(Node("document", "d1"), "viewer", Userset(Star("group"), "member")))
	if !errors.Is(err, ErrInvalidNode) {
		t.Fatalf("err %v, wanted ErrInvalidNode", err)
	}
}

func TestEmptyIdIsRejected(t *testing.T) {
	if err := (Node("document", "")).Validate(); !errors.Is(err, ErrInvalidNode) {
		t.Fatalf("err %v, wanted ErrInvalidNode", err)
	}
}

// --- Wildcard declarations ---

const schemaStructural = `
namespace org:acme
action read

relation parent : transitive, wildcard none
relation viewer : direct, wildcard subject

type user

type folder
  relation parent -> folder
  relation viewer -> user
  rule read = viewer + parent->read
`

func TestWildcardObjectIsRefusedOnAStructuralRelation(t *testing.T) {
	// document:* #parent folder:public would reparent every document in the
	// tenant with one write. The declaration is what makes that impossible.
	e := evalFor(t, schemaStructural)
	err := e.CanWrite(context.Background(), Node("user", "root"),
		edge(Star("folder"), "parent", Subject(Node("folder", "public"))), CarveNone, at)
	if err == nil || !strings.Contains(err.Error(), "wildcard object") {
		t.Fatalf("err %v, wanted a refusal naming the wildcard object", err)
	}
}

func TestWildcardObjectEdgeIsNotReadOnARelationThatForbidsIt(t *testing.T) {
	// Even if a resolver hands one back, the walk must not follow it.
	store := NewMemoryStore()
	store.MustWrite(
		edge(Star("folder"), "parent", Subject(Node("folder", "public"))),
		edge(Node("folder", "public"), "viewer", Subject(Node("user", "u9"))),
	)
	e, err := New(MustParse(schemaStructural), store)
	if err != nil {
		t.Fatal(err)
	}
	mustDeny(t, e, Node("user", "u9"), "read", Node("folder", "f1"), ReasonUnreachable)
}

func TestWildcardSubjectIsRefusedWhereUndeclared(t *testing.T) {
	e := evalFor(t, schemaStructural)
	err := e.CanWrite(context.Background(), Node("user", "root"),
		edge(Node("folder", "f1"), "parent", Subject(Star("folder"))), CarveNone, at)
	if err == nil {
		t.Fatal("a wildcard subject was accepted on a relation that forbids it")
	}
}

// --- Delegation ---

func TestCanWriteRefusesAnIllegalTarget(t *testing.T) {
	e := evalFor(t, schemaTree)
	err := e.CanWrite(context.Background(), Node("user", "root"),
		edge(Node("folder", "f1"), "editor", Subject(Node("folder", "other"))), CarveRootOfTrust, at)
	if err == nil {
		t.Fatal("an edge pointing at an undeclared target type was accepted")
	}
}

func TestActionsFedIgnoresTheSubtractedSide(t *testing.T) {
	// Writing an edge that only ever removes access is not an escalation, so
	// the subset rule must not demand the granter hold it.
	e := evalFor(t, schemaClassified)
	if fed := e.ActionsFed("document", "classified"); len(fed) != 0 {
		t.Fatalf("ActionsFed = %v, wanted none for a subtracted relation", fed)
	}
	if fed := e.ActionsFed("document", "viewer"); len(fed) != 1 || fed[0] != "read" {
		t.Fatalf("ActionsFed = %v, wanted [read]", fed)
	}
}

func TestCanWriteFollowsRuleReferences(t *testing.T) {
	// folder.write feeds through editor, and document.write arrives by arrow.
	e := evalFor(t, schemaTree)
	if fed := e.ActionsFed("folder", "editor"); len(fed) != 1 || fed[0] != "write" {
		t.Fatalf("ActionsFed = %v, wanted [write]", fed)
	}
	if fed := e.ActionsFed("document", "parent"); len(fed) != 1 || fed[0] != "write" {
		t.Fatalf("ActionsFed = %v, wanted [write]", fed)
	}
}

// --- Schema validation ---

func TestSchemaRejects(t *testing.T) {
	cases := map[string]string{
		"undeclared relation on a type": `
namespace n
action read
type user
type doc
  relation viewer -> user
  rule read = viewer`,

		"undeclared action": `
namespace n
action read
relation viewer : direct
type user
type doc
  relation viewer -> user
  rule write = viewer`,

		"undeclared target type": `
namespace n
action read
relation viewer : direct
type doc
  relation viewer -> user
  rule read = viewer`,

		"term that is neither relation nor rule": `
namespace n
action read
relation viewer : direct
type user
type doc
  relation viewer -> user
  rule read = editor`,

		"rule that references itself": `
namespace n
action read, write
relation viewer : direct
type user
type doc
  relation viewer -> user
  rule read  = write
  rule write = read`,

		"identity relation carrying a wildcard": `
namespace n
action read
relation actor  : identity, wildcard subject
relation viewer : direct
type user
type doc
  relation viewer -> user
  rule read = viewer`,

		"arrow to a type that cannot answer it": `
namespace n
action read
relation viewer : direct
relation parent : transitive
type user
type doc
  relation viewer -> user
  relation parent -> user
  rule read = viewer + parent->read`,
	}

	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(src); err == nil {
				t.Fatal("schema was accepted, wanted a refusal")
			}
		})
	}
}

func TestSchemaAcceptsAForwardRuleReference(t *testing.T) {
	// read is declared before write but references it. Resolution happens once
	// the whole type is known, so declaration order does not matter.
	s, err := Parse(`
namespace n
action read, write
relation viewer : direct
relation editor : direct
type user
type doc
  relation viewer -> user
  relation editor -> user
  rule read  = viewer + write
  rule write = editor`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, ok := s.Types["doc"].Rules["read"].(Union); !ok {
		t.Fatalf("read resolved to %T, wanted a Union", s.Types["doc"].Rules["read"])
	}
}

// --- Parser ---

func TestParseRoundTripsExpressions(t *testing.T) {
	s := MustParse(`
namespace n
action read, write, share
relation viewer : direct
relation editor : direct
relation banned : direct
relation parent : transitive
type user
type doc
  relation viewer -> user
  relation editor -> user
  relation banned -> user
  relation parent -> doc
  rule read  = viewer + editor + parent->read
  rule write = editor
  rule share = read - banned`)

	want := map[string]string{
		"read":  "viewer + editor + parent->read",
		"write": "editor",
		"share": "read - banned",
	}
	for action, text := range want {
		if got := s.Types["doc"].Rules[action].String(); got != text {
			t.Errorf("rule %q = %q, wanted %q", action, got, text)
		}
	}
}

func TestParseStripsCommentsWithoutEatingUsersets(t *testing.T) {
	s := MustParse(`
namespace n          # the tenant
action read          // one verb
relation member : transitive
relation viewer : direct
type user
type group
  relation member -> user | group#member
type doc
  relation viewer -> user | group#member   # a userset survives
  rule read = viewer`)

	targets := s.Types["doc"].Relations["viewer"]
	if len(targets) != 2 || targets[1].Type != "group" || targets[1].Relation != "member" {
		t.Fatalf("targets = %v, wanted [user group#member]", targets)
	}
}

// --- Inert declarations ---

func TestWarnsAboutDecorativeTransitivity(t *testing.T) {
	// parent appears only on the near side of an arrow, so the walk follows it
	// exactly one hop and the depth comes from folder.write referring to the
	// arrow again. The flag reads as though it does something.
	s := MustParse(`
namespace n
action write
relation parent : transitive
relation editor : direct
type user
type folder
  relation parent -> folder
  relation editor -> user
  rule write = editor + parent->write`)

	warnings := s.Warnings()
	if len(warnings) != 1 || !strings.Contains(warnings[0], "no effect") {
		t.Fatalf("warnings = %v, wanted one about an ineffective transitive flag", warnings)
	}
}

func TestNoWarningWhenTransitivityIsLoadBearing(t *testing.T) {
	// member is reached through a userset target, which evaluates it on its
	// own, so the flag is read and the chain works.
	s := MustParse(`
namespace n
action read
relation member : transitive
relation admin  : direct
type user
type group
  relation member -> user | group#member
type org
  relation admin -> user | group#member
  rule read = admin`)

	if w := s.Warnings(); len(w) != 0 {
		t.Fatalf("warnings = %v, wanted none", w)
	}
}

func TestWarnsAboutUnusedDeclarations(t *testing.T) {
	s := MustParse(`
namespace n
action read, write
relation viewer : direct
relation ghost  : direct
type user
type doc
  relation viewer -> user
  rule read = viewer`)

	warnings := strings.Join(s.Warnings(), "\n")
	if !strings.Contains(warnings, `relation "ghost"`) {
		t.Errorf("warnings %q, wanted the unused relation named", warnings)
	}
	if !strings.Contains(warnings, `action "write"`) {
		t.Errorf("warnings %q, wanted the unresolved action named", warnings)
	}
}

func TestPublishedExamplesCarryNoInertDeclarations(t *testing.T) {
	for name, src := range map[string]string{
		"role":       schemaRole,
		"tree":       schemaTree,
		"own":        schemaOwn,
		"close":      schemaClose,
		"classified": schemaClassified,
		"key":        schemaKey,
		"wildcard":   schemaWildcard,
		"tenants":    schemaTenants,
	} {
		t.Run(name, func(t *testing.T) {
			if w := MustParse(src).Warnings(); len(w) != 0 {
				t.Fatalf("warnings = %v", w)
			}
		})
	}
}

func TestParseRejectsAChainedArrow(t *testing.T) {
	_, err := Parse(`
namespace n
action read
relation parent : transitive
type user
type doc
  relation parent -> doc
  rule read = parent->parent->read`)
	if err == nil {
		t.Fatal("a chained arrow was accepted")
	}
}
