package reach

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

// The reverse index: what can this subject reach, and who reaches this object.
//
// Check answers one question. A product asks a different one on every listing
// page and every share dialog, and answering it with a Check per row is what
// makes an authorisation service unusable at any real size: a page of fifty
// documents becomes fifty walks, and a page over ten thousand cannot be built
// at all.
//
// # Candidates, then verification
//
// Both directions work the same way, and the shape is deliberate. A walk
// gathers candidates cheaply, and every candidate is then confirmed by the same
// Check that answers a single question.
//
// Running the graph backwards exactly is not possible in general. Subtraction
// and intersection do not invert: knowing an edge grants something says nothing
// about whether another rule took it away, and inverting a rule that requires
// two things at once means intersecting two candidate sets that are each cheap
// to over-approximate and expensive to compute exactly. So the backward walk is
// allowed to be generous, and correctness comes from the verify step. There can
// be no false positives, because the authority is the same engine.
//
// The win over checking every row is what the walk touches: only nodes the
// subject actually has some edge toward, rather than every object of a type. A
// customer on three projects generates a handful of candidates whether the
// table holds a hundred rows or a hundred million.
//
// # What this cannot see
//
// Only objects KYC holds an edge for. A resource with no edges at all is
// invisible, which is correct for anything reached through containment, since
// it needs a parent edge to be reachable. It is not correct for a wildcard
// grant: T:* covers objects nobody has written an edge about. That case is
// reported rather than guessed at, through ObjectList.All.

// ErrNoReverseIndex means the resolver cannot answer reverse queries.
var ErrNoReverseIndex = errors.New("reach: resolver has no reverse index")

// ReverseResolver is a Resolver that can also read edges by subject. The
// separate interface keeps the reverse index optional: a store that only ever
// answers Check does not have to build one.
type ReverseResolver interface {
	Resolver
	// EdgesForSubject returns every edge naming this node as its subject,
	// whatever the userset relation. Callers filter.
	EdgesForSubject(ctx context.Context, subject NodeRef) ([]Edge, error)
}

// DefaultMaxCandidates bounds one reverse query. Generous for a listing page
// and small enough that a pathological graph cannot hold a connection open.
const DefaultMaxCandidates = 1000

// ObjectList is what a subject can reach.
type ObjectList struct {
	// Objects are the concrete nodes, sorted, each confirmed by Check.
	Objects []NodeRef
	// All reports that a wildcard grant covers every object of this type,
	// including ones this system holds no edge for. Objects is then a lower
	// bound rather than the answer, and a caller that filters a list by it
	// would wrongly hide rows.
	All bool
	// Truncated reports that the candidate walk hit its bound. The result is a
	// subset, and saying so beats returning a short list that reads as complete.
	Truncated bool
}

// SubjectList is who can reach an object.
type SubjectList struct {
	// Subjects are the concrete principals, sorted, each confirmed by Check.
	Subjects []NodeRef
	// All reports that a subject wildcard grants this to everyone of a type,
	// so Subjects is a lower bound.
	All bool
	// Truncated reports that the expansion hit its bound.
	Truncated bool
}

// ListObjects returns the objects of one type the subject may act on.
//
// The candidate walk runs backwards from the subject: every edge naming it,
// then every edge naming what those led to. That is why membership and
// containment both fall out of one loop, and it is why the cost tracks what the
// subject touches rather than how many objects exist.
func (e *Evaluator) ListObjects(ctx context.Context, subject NodeRef, action, resourceType string, now time.Time) (ObjectList, error) {
	if err := subject.Validate(); err != nil {
		return ObjectList{}, err
	}
	rev, ok := e.resolver.(ReverseResolver)
	if !ok {
		return ObjectList{}, ErrNoReverseIndex
	}
	if _, known := e.schema.Type(resourceType); !known {
		return ObjectList{}, fmt.Errorf("%w: %q", ErrUnknownType, resourceType)
	}

	var out ObjectList

	// A wildcard grant is answered by one Check rather than by the walk. It
	// covers objects that have no edges, which no traversal can discover.
	star := Star(resourceType)
	if d, err := e.Check(ctx, Request{Subject: subject, Action: action, Resource: star}, now); err != nil {
		return ObjectList{}, err
	} else if d.Allowed {
		out.All = true
	}

	candidates, truncated, err := e.candidateObjects(ctx, rev, subject, resourceType)
	if err != nil {
		return ObjectList{}, err
	}
	out.Truncated = truncated

	for _, node := range candidates {
		d, err := e.Check(ctx, Request{Subject: subject, Action: action, Resource: node}, now)
		if err != nil {
			return ObjectList{}, err
		}
		if d.Allowed {
			out.Objects = append(out.Objects, node)
		}
	}
	sortNodes(out.Objects)
	return out, nil
}

// candidateObjects walks backwards from the subject, collecting every node of
// the wanted type it can arrive at.
//
// Both forms of a discovered node are pushed back onto the frontier, and that
// is what makes one loop cover two different mechanisms. Reached as a plain
// node, project:apollo finds the documents whose parent it is. Reached as a
// userset, role:editor finds what that role was granted. Neither needs the
// schema consulted, because the verify step is what decides meaning.
func (e *Evaluator) candidateObjects(ctx context.Context, rev ReverseResolver, subject NodeRef, resourceType string) ([]NodeRef, bool, error) {
	type frontierItem struct {
		node     NodeRef
		relation string
		depth    int
	}

	seen := map[string]struct{}{}
	found := map[string]NodeRef{}
	queue := []frontierItem{{node: subject}}
	truncated := false

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		if item.depth >= e.maxDepth {
			truncated = true
			continue
		}
		key := item.node.String() + "#" + item.relation
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}

		edges, err := rev.EdgesForSubject(ctx, item.node)
		if err != nil {
			return nil, false, err
		}
		for _, edge := range edges {
			// An edge naming this node as a userset only carries the frontier
			// when the relation matches how we arrived. role:editor#holder is
			// not role:editor#banned.
			if edge.Subject.Relation != item.relation {
				continue
			}
			if edge.Object.Type == resourceType && !edge.Object.IsWildcard() {
				found[edge.Object.String()] = edge.Object
				if len(found) >= DefaultMaxCandidates {
					return nodesOf(found), true, nil
				}
			}
			// A star node is not a candidate: it is the wildcard case, already
			// answered by its own Check.
			if edge.Object.IsWildcard() {
				continue
			}
			queue = append(queue,
				frontierItem{node: edge.Object, depth: item.depth + 1},
				frontierItem{node: edge.Object, relation: edge.Relation, depth: item.depth + 1},
			)
		}
	}
	return nodesOf(found), truncated, nil
}

// ListSubjects returns the principals that may perform an action on one object.
//
// This runs forwards, expanding the relations that answer the action and
// following usersets and containers, then verifying every leaf. It is the share
// dialog, and the audit question: who reaches this.
func (e *Evaluator) ListSubjects(ctx context.Context, resource NodeRef, action string, now time.Time) (SubjectList, error) {
	if err := resource.Validate(); err != nil {
		return SubjectList{}, err
	}
	typ, ok := e.schema.Type(resource.Type)
	if !ok {
		return SubjectList{}, fmt.Errorf("%w: %q", ErrUnknownType, resource.Type)
	}
	if _, answers := typ.Rules[action]; !answers {
		// A type that does not answer the action is reached by nobody, which is
		// an empty list rather than an error: asking is legitimate.
		return SubjectList{}, nil
	}

	candidates, truncated, err := e.candidateSubjects(ctx, resource)
	if err != nil {
		return SubjectList{}, err
	}

	out := SubjectList{Truncated: truncated}
	for _, node := range candidates {
		if node.IsWildcard() {
			// Everyone of a type. Nothing to enumerate and nothing to verify:
			// the set is unbounded by construction.
			out.All = true
			continue
		}
		d, err := e.Check(ctx, Request{Subject: node, Action: action, Resource: resource}, now)
		if err != nil {
			return SubjectList{}, err
		}
		if d.Allowed {
			out.Subjects = append(out.Subjects, node)
		}
	}
	sortNodes(out.Subjects)
	return out, nil
}

// candidateSubjects gathers every node named on any edge reachable from the
// resource. Generous on purpose: the verify step decides which of them the
// rules actually grant, so this only has to avoid missing anyone.
func (e *Evaluator) candidateSubjects(ctx context.Context, resource NodeRef) ([]NodeRef, bool, error) {
	type frontierItem struct {
		node  NodeRef
		depth int
	}

	relations := make([]string, 0, len(e.schema.Relations))
	for name := range e.schema.Relations {
		relations = append(relations, name)
	}
	sort.Strings(relations)

	seen := map[string]struct{}{}
	found := map[string]NodeRef{}
	queue := []frontierItem{{node: resource}}
	truncated := false

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		if item.depth >= e.maxDepth {
			truncated = true
			continue
		}
		if _, dup := seen[item.node.String()]; dup {
			continue
		}
		seen[item.node.String()] = struct{}{}

		// The star node carries grants written for every object of the type, so
		// a subject reaching document:* reaches this document too.
		nodes := []NodeRef{item.node}
		if !item.node.IsWildcard() {
			nodes = append(nodes, Star(item.node.Type))
		}

		for _, object := range nodes {
			for _, relation := range relations {
				edges, err := e.resolver.Edges(ctx, object, relation)
				if err != nil {
					return nil, false, err
				}
				for _, edge := range edges {
					subject := edge.Subject.Node
					found[subject.String()] = subject
					if len(found) >= DefaultMaxCandidates {
						return nodesOf(found), true, nil
					}
					queue = append(queue, frontierItem{node: subject, depth: item.depth + 1})
				}
			}
		}
	}
	return nodesOf(found), truncated, nil
}

func nodesOf(m map[string]NodeRef) []NodeRef {
	out := make([]NodeRef, 0, len(m))
	for _, n := range m {
		out = append(out, n)
	}
	sortNodes(out)
	return out
}

func sortNodes(nodes []NodeRef) {
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Type != nodes[j].Type {
			return nodes[i].Type < nodes[j].Type
		}
		return nodes[i].ID < nodes[j].ID
	})
}
