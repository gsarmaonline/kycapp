package reach

import (
	"context"
	"testing"
	"time"
)

const reverseSchema = `
namespace docs

action read, write

relation member_of : transitive
relation holder    : direct
relation parent    : direct
relation banned    : direct
relation can_read  : direct, wildcard both
relation can_write : direct, wildcard both

type user
type group
  relation member_of -> user | group#member_of
type role
  relation holder -> user | group#member_of

type project
  relation can_read  -> user | group#member_of | role#holder
  relation can_write -> user | group#member_of | role#holder
  rule read  = can_read
  rule write = can_write

type document
  relation parent    -> project
  relation banned    -> user
  relation can_read  -> user | group#member_of | role#holder
  relation can_write -> user | group#member_of | role#holder
  // No parentheses in the grammar: operators associate left, so this
  // already reads as (can_read + parent->read) - banned.
  rule read  = can_read + parent->read - banned
  rule write = can_write + parent->write
`

func reverseFixture(t *testing.T) *Evaluator {
	t.Helper()
	schema, err := Parse(reverseSchema)
	if err != nil {
		t.Fatal(err)
	}
	store := NewMemoryStore()
	store.MustWrite(
		// Ana holds the editor role; the role is granted read on one project.
		Edge{Object: Node("role", "editor"), Relation: "holder", Subject: Subject(Node("user", "ana"))},
		Edge{Object: Node("project", "apollo"), Relation: "can_read", Subject: Userset(Node("role", "editor"), "holder")},
		// Two documents live in that project, one directly granted to Bob.
		Edge{Object: Node("document", "d1"), Relation: "parent", Subject: Subject(Node("project", "apollo"))},
		Edge{Object: Node("document", "d2"), Relation: "parent", Subject: Subject(Node("project", "apollo"))},
		Edge{Object: Node("document", "d3"), Relation: "can_read", Subject: Subject(Node("user", "bob"))},
		// A document in a project nobody reached.
		Edge{Object: Node("document", "d4"), Relation: "parent", Subject: Subject(Node("project", "zeus"))},
	)
	e, err := New(schema, store)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func ids(nodes []NodeRef) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.ID)
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestListObjectsReachesThroughRoleAndContainment(t *testing.T) {
	e := reverseFixture(t)
	got, err := e.ListObjects(context.Background(), Node("user", "ana"), "read", "document", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	// Two hops of indirection, neither of them naming a document: ana holds a
	// role, the role is granted on a project, and the documents are in it.
	if want := []string{"d1", "d2"}; !equal(ids(got.Objects), want) {
		t.Errorf("objects = %v, wanted %v", ids(got.Objects), want)
	}
	if got.All {
		t.Error("no wildcard grant exists")
	}
	if got.Truncated {
		t.Error("the walk should not have truncated")
	}
}

// The candidate walk is allowed to be generous; Check is the authority. A
// subtraction is the case that proves it, because an exclusion cannot be run
// backwards but must still be honoured.
func TestListObjectsHonoursASubtraction(t *testing.T) {
	e := reverseFixture(t)
	store := e.resolver.(*MemoryStore)
	store.MustWrite(Edge{
		Object: Node("document", "d2"), Relation: "banned", Subject: Subject(Node("user", "ana")),
	})

	got, err := e.ListObjects(context.Background(), Node("user", "ana"), "read", "document", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"d1"}; !equal(ids(got.Objects), want) {
		t.Errorf("objects = %v, wanted %v: the exclusion must survive the reverse walk", ids(got.Objects), want)
	}
}

// An expired edge grants nothing, and the reverse query must agree with Check
// about that rather than listing what used to be reachable.
func TestListObjectsRespectsExpiry(t *testing.T) {
	e := reverseFixture(t)
	store := e.resolver.(*MemoryStore)
	past := time.Now().Add(-time.Hour)
	store.MustWrite(Edge{
		Object: Node("project", "zeus"), Relation: "can_read",
		Subject: Subject(Node("user", "ana")), ExpiresAt: &past,
	})

	got, err := e.ListObjects(context.Background(), Node("user", "ana"), "read", "document", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range got.Objects {
		if n.ID == "d4" {
			t.Error("an expired grant was listed as reachable")
		}
	}
}

// A wildcard grant covers objects nobody has written an edge about, so no walk
// can enumerate it. Saying so is the only honest answer.
func TestListObjectsReportsAWildcardRatherThanGuessing(t *testing.T) {
	e := reverseFixture(t)
	store := e.resolver.(*MemoryStore)
	store.MustWrite(Edge{
		Object: Star("document"), Relation: "can_read", Subject: Subject(Node("user", "zoe")),
	})

	got, err := e.ListObjects(context.Background(), Node("user", "zoe"), "read", "document", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !got.All {
		t.Fatal("a wildcard grant must be reported, not silently enumerated")
	}
	// The star node itself is never returned as an object: it is not a thing a
	// caller can open.
	for _, n := range got.Objects {
		if n.IsWildcard() {
			t.Errorf("the star node was returned as an object: %v", n)
		}
	}
}

func TestListObjectsIsEmptyForAStranger(t *testing.T) {
	e := reverseFixture(t)
	got, err := e.ListObjects(context.Background(), Node("user", "nobody"), "read", "document", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Objects) != 0 || got.All {
		t.Errorf("a stranger reaches nothing, got %v", got)
	}
}

func TestListSubjectsFindsEveryoneWhoReaches(t *testing.T) {
	e := reverseFixture(t)
	got, err := e.ListSubjects(context.Background(), Node("document", "d1"), "read", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	// Ana reaches d1 only through role then project. Nothing names her on it.
	if want := []string{"ana"}; !equal(ids(got.Subjects), want) {
		t.Errorf("subjects = %v, wanted %v", ids(got.Subjects), want)
	}
}

func TestListSubjectsHonoursASubtraction(t *testing.T) {
	e := reverseFixture(t)
	store := e.resolver.(*MemoryStore)
	store.MustWrite(Edge{
		Object: Node("document", "d1"), Relation: "banned", Subject: Subject(Node("user", "ana")),
	})
	got, err := e.ListSubjects(context.Background(), Node("document", "d1"), "read", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Subjects) != 0 {
		t.Errorf("a banned subject was listed: %v", ids(got.Subjects))
	}
}

// A subject wildcard is unbounded by construction, so it is reported rather
// than expanded into a list that could never be complete.
func TestListSubjectsReportsASubjectWildcard(t *testing.T) {
	e := reverseFixture(t)
	store := e.resolver.(*MemoryStore)
	store.MustWrite(Edge{
		Object: Node("document", "d3"), Relation: "can_read", Subject: Subject(Star("user")),
	})
	got, err := e.ListSubjects(context.Background(), Node("document", "d3"), "read", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !got.All {
		t.Error("an everyone grant must be reported")
	}
	for _, n := range got.Subjects {
		if n.IsWildcard() {
			t.Errorf("the star node was returned as a subject: %v", n)
		}
	}
}

// A type that answers nothing is reached by nobody. Asking is legitimate, so it
// is an empty list rather than an error.
func TestListSubjectsOnATypeThatDoesNotAnswer(t *testing.T) {
	e := reverseFixture(t)
	got, err := e.ListSubjects(context.Background(), Node("group", "eng"), "read", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Subjects) != 0 || got.All {
		t.Errorf("got %v, wanted nothing", got)
	}
}

func TestReverseQueriesNeedAReverseIndex(t *testing.T) {
	schema, err := Parse(reverseSchema)
	if err != nil {
		t.Fatal(err)
	}
	e, err := New(schema, forwardOnly{NewMemoryStore()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.ListObjects(context.Background(), Node("user", "ana"), "read", "document", time.Now()); err == nil {
		t.Error("a resolver without a reverse index must say so rather than return nothing")
	}
}

// forwardOnly hides EdgesForSubject, which is the point: the reverse index is
// optional and a store that lacks one has to fail loudly rather than answer
// "nothing reachable".
type forwardOnly struct{ inner *MemoryStore }

func (f forwardOnly) Edges(ctx context.Context, object NodeRef, relation string) ([]Edge, error) {
	return f.inner.Edges(ctx, object, relation)
}

func TestListObjectsRejectsAnUnknownType(t *testing.T) {
	e := reverseFixture(t)
	if _, err := e.ListObjects(context.Background(), Node("user", "ana"), "read", "spaceship", time.Now()); err == nil {
		t.Error("an unknown type must be an error, not an empty list")
	}
}
