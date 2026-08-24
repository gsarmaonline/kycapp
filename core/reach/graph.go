package reach

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Wildcard is the id of the node that stands for every object of a type.
// Every type has exactly one, and it is written T:* in the DSL.
const Wildcard = "*"

// ErrInvalidNode means a node reference is malformed.
var ErrInvalidNode = errors.New("reach: invalid node")

// NodeRef names one node: a type and an id.
type NodeRef struct {
	Type string
	ID   string
}

// Node builds a reference to one concrete object.
func Node(typ, id string) NodeRef { return NodeRef{Type: typ, ID: id} }

// Star builds the wildcard node for a type: every object of that type,
// including the ones that do not exist yet.
func Star(typ string) NodeRef { return NodeRef{Type: typ, ID: Wildcard} }

// IsWildcard reports whether the reference is a type's star node.
func (n NodeRef) IsWildcard() bool { return n.ID == Wildcard }

// IsZero reports whether the reference is unset.
func (n NodeRef) IsZero() bool { return n.Type == "" && n.ID == "" }

func (n NodeRef) String() string { return n.Type + ":" + n.ID }

// Validate rejects malformed references. A node whose *type* is the wildcard is
// refused everywhere: reach over every type at once is the environment-derived
// root of trust, and it stays outside the data where it can be counted.
func (n NodeRef) Validate() error {
	if strings.TrimSpace(n.Type) == "" {
		return fmt.Errorf("%w: empty type", ErrInvalidNode)
	}
	if strings.TrimSpace(n.ID) == "" {
		return fmt.Errorf("%w: %s has an empty id", ErrInvalidNode, n.Type)
	}
	if n.Type == Wildcard {
		return fmt.Errorf("%w: *:* is not writable", ErrInvalidNode)
	}
	return nil
}

// SubjectRef is the far end of an edge. It is either a node, or a userset: a
// node plus the relation to follow at it.
//
//	user:u9            a node
//	group:eng#member   a userset - whoever is a member of eng
type SubjectRef struct {
	Node NodeRef
	// Relation is empty for a plain node.
	Relation string
}

// Subject builds a plain node target.
func Subject(n NodeRef) SubjectRef { return SubjectRef{Node: n} }

// Userset builds a target that resolves through a relation at the node.
func Userset(n NodeRef, relation string) SubjectRef {
	return SubjectRef{Node: n, Relation: relation}
}

// IsUserset reports whether the target resolves through a relation.
func (s SubjectRef) IsUserset() bool { return s.Relation != "" }

func (s SubjectRef) String() string {
	if s.Relation == "" {
		return s.Node.String()
	}
	return s.Node.String() + "#" + s.Relation
}

// Edge is one fact: A's relation is B. This is the entire data model.
type Edge struct {
	Object   NodeRef
	Relation string
	Subject  SubjectRef
	// ExpiresAt is nil for a standing edge. An expiry sits here rather than in
	// a revocation job, so time-boxed access is the cheap option rather than
	// the diligent one.
	ExpiresAt *time.Time
}

func (e Edge) String() string {
	s := e.Object.String() + " #" + e.Relation + " " + e.Subject.String()
	if e.ExpiresAt != nil {
		s += " expires " + e.ExpiresAt.UTC().Format(time.RFC3339)
	}
	return s
}

// Active reports whether the edge has not expired at now.
func (e Edge) Active(now time.Time) bool {
	return e.ExpiresAt == nil || now.Before(*e.ExpiresAt)
}

// Validate rejects malformed edges at the boundary, so a bad row cannot become
// a silent allow deep in the walk.
func (e Edge) Validate() error {
	if err := e.Object.Validate(); err != nil {
		return fmt.Errorf("object: %w", err)
	}
	if strings.TrimSpace(e.Relation) == "" {
		return fmt.Errorf("%w: %s has an empty relation", ErrInvalidNode, e.Object)
	}
	if err := e.Subject.Node.Validate(); err != nil {
		return fmt.Errorf("subject: %w", err)
	}
	if e.Subject.IsUserset() && e.Subject.Node.IsWildcard() {
		// "every group's members" names no one in particular and would make
		// the walk fan out over a whole type for no expressible reason.
		return fmt.Errorf("%w: a userset may not sit on a wildcard node", ErrInvalidNode)
	}
	return nil
}

// Resolver supplies the edges out of one node under one relation.
//
// It exists because traversal means there is no finite answer set to ship. The
// platform holds the schema and the subject, group and role edges; a tenant's
// backend holds its own resource graph. The same walk runs on both sides, and
// the platform never learns what a resource id means.
type Resolver interface {
	Edges(ctx context.Context, object NodeRef, relation string) ([]Edge, error)
}

// MemoryStore is an in-memory Resolver. It is the reference implementation and
// what the tests run against.
type MemoryStore struct {
	byKey map[string][]Edge
}

// NewMemoryStore returns an empty store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{byKey: map[string][]Edge{}}
}

func edgeKey(object NodeRef, relation string) string {
	return object.Type + ":" + object.ID + "#" + relation
}

// Write adds one edge. A duplicate is a no-op rather than an error, so writing
// the same fact twice is safe for a caller that cannot tell.
func (m *MemoryStore) Write(edges ...Edge) error {
	for _, e := range edges {
		if err := e.Validate(); err != nil {
			return err
		}
		k := edgeKey(e.Object, e.Relation)
		if dup := indexOfEdge(m.byKey[k], e); dup >= 0 {
			m.byKey[k][dup] = e
			continue
		}
		m.byKey[k] = append(m.byKey[k], e)
	}
	return nil
}

// MustWrite is Write for tests and fixtures.
func (m *MemoryStore) MustWrite(edges ...Edge) {
	if err := m.Write(edges...); err != nil {
		panic(err)
	}
}

// Delete removes one edge, ignoring its expiry. Reports whether it was present.
func (m *MemoryStore) Delete(e Edge) bool {
	k := edgeKey(e.Object, e.Relation)
	i := indexOfEdge(m.byKey[k], e)
	if i < 0 {
		return false
	}
	m.byKey[k] = append(m.byKey[k][:i], m.byKey[k][i+1:]...)
	return true
}

func indexOfEdge(list []Edge, want Edge) int {
	for i, e := range list {
		if e.Subject == want.Subject {
			return i
		}
	}
	return -1
}

// Edges implements Resolver.
func (m *MemoryStore) Edges(_ context.Context, object NodeRef, relation string) ([]Edge, error) {
	return m.byKey[edgeKey(object, relation)], nil
}

// Len reports how many edges the store holds.
func (m *MemoryStore) Len() int {
	n := 0
	for _, list := range m.byKey {
		n += len(list)
	}
	return n
}
