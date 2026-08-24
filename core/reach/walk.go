package reach

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

// DefaultMaxDepth bounds the walk. It is a circuit breaker in the runtime, not
// a limit in the model: no depth cap belongs in a schema, and a cap that denies
// silently hides a modelling mistake for months. This one errors loudly.
const DefaultMaxDepth = 64

var (
	// ErrUnknownType means the resource names a type the schema does not declare.
	ErrUnknownType = errors.New("reach: unknown type")
	// ErrUnknownAction means the request names an action outside the namespace
	// vocabulary. An unknown action must fail rather than be treated as absent.
	ErrUnknownAction = errors.New("reach: unknown action")
	// ErrDepthExceeded means the walk hit the runtime bound.
	ErrDepthExceeded = errors.New("reach: walk depth exceeded")
)

// Reason explains a decision. The mapping to a status code matters: unreachable
// must be indistinguishable from a resource that does not exist, or the tenants
// of the system become enumerable by status code alone.
type Reason string

const (
	// ReasonAllowed means a path reached the resource and a rule granted the action.
	ReasonAllowed Reason = "allowed"
	// ReasonUnreachable means no path of any kind reaches the resource. Map to 404.
	ReasonUnreachable Reason = "unreachable"
	// ReasonNoRule means a path reaches it, but no rule grants this action. Map to 403.
	ReasonNoRule Reason = "no_rule"
	// ReasonExcluded means a rule matched and schema subtraction removed it. Map to 403.
	ReasonExcluded Reason = "excluded"
)

// Step is one edge the walk crossed.
type Step struct {
	Object   NodeRef
	Relation string
	Subject  SubjectRef
}

func (s Step) String() string {
	return s.Object.String() + " #" + s.Relation + " " + s.Subject.String()
}

// Decision is the result of a check.
//
// Path is the primary debugging tool, not an audit nicety. Groups, roles,
// folders, tags and actions are all nodes joined by edges, so "why can this
// person reach this?" is a path through undifferentiated structure and cannot
// be answered without one.
type Decision struct {
	Allowed bool
	Reason  Reason
	Path    []Step
}

// Request is the question: may this subject perform this action on this resource?
type Request struct {
	Subject  NodeRef
	Action   string
	Resource NodeRef
}

// Evaluator answers requests against a schema and a resolver.
type Evaluator struct {
	schema   *Schema
	resolver Resolver
	maxDepth int
}

// Option configures an Evaluator.
type Option func(*Evaluator)

// WithMaxDepth overrides the runtime walk bound.
func WithMaxDepth(n int) Option {
	return func(e *Evaluator) {
		if n > 0 {
			e.maxDepth = n
		}
	}
}

// New returns an Evaluator. The schema must already validate.
func New(s *Schema, r Resolver, opts ...Option) (*Evaluator, error) {
	if s == nil {
		return nil, fmt.Errorf("%w: nil schema", ErrInvalidSchema)
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	if r == nil {
		return nil, errors.New("reach: nil resolver")
	}
	e := &Evaluator{schema: s, resolver: r, maxDepth: DefaultMaxDepth}
	for _, o := range opts {
		o(e)
	}
	return e, nil
}

// Schema returns the evaluator's schema.
func (e *Evaluator) Schema() *Schema { return e.schema }

// Check answers one request at now.
//
// Evaluation walks inward from the resource. Reachability only ever adds, so
// the order edges are visited in does not change the answer.
func (e *Evaluator) Check(ctx context.Context, req Request, now time.Time) (Decision, error) {
	if err := req.Subject.Validate(); err != nil {
		return Decision{}, fmt.Errorf("subject: %w", err)
	}
	if err := req.Resource.Validate(); err != nil {
		return Decision{}, fmt.Errorf("resource: %w", err)
	}
	if !e.schema.HasAction(req.Action) {
		return Decision{}, fmt.Errorf("%w: %q", ErrUnknownAction, req.Action)
	}
	t, ok := e.schema.Type(req.Resource.Type)
	if !ok {
		return Decision{}, fmt.Errorf("%w: %q", ErrUnknownType, req.Resource.Type)
	}

	subjects, err := e.expandSubject(ctx, req.Subject, now)
	if err != nil {
		return Decision{}, err
	}

	if expr, ok := t.Rules[req.Action]; ok {
		r := e.newRun(ctx, subjects, now)
		matched, path, err := r.satisfies(req.Resource, expr, 0)
		if err != nil {
			return Decision{}, err
		}
		if matched {
			return Decision{Allowed: true, Reason: ReasonAllowed, Path: path}, nil
		}
		if r.excluded {
			return Decision{Reason: ReasonExcluded}, nil
		}
	}

	// Reached but not permitted, or not reached at all? The distinction is what
	// keeps a 404 honest, so it is worth one extra pass over the type's other
	// rules on the denial path.
	reached, err := e.reachesAtAll(ctx, subjects, t, req, now)
	if err != nil {
		return Decision{}, err
	}
	if reached {
		return Decision{Reason: ReasonNoRule}, nil
	}
	return Decision{Reason: ReasonUnreachable}, nil
}

// Allowed is Check reduced to a boolean, for call sites that do not report a
// reason.
func (e *Evaluator) Allowed(ctx context.Context, req Request, now time.Time) (bool, error) {
	d, err := e.Check(ctx, req, now)
	return d.Allowed, err
}

func (e *Evaluator) reachesAtAll(ctx context.Context, subjects map[NodeRef]bool, t *TypeDef, req Request, now time.Time) (bool, error) {
	actions := make([]string, 0, len(t.Rules))
	for a := range t.Rules {
		if a != req.Action {
			actions = append(actions, a)
		}
	}
	sort.Strings(actions)

	for _, a := range actions {
		r := e.newRun(ctx, subjects, now)
		matched, _, err := r.satisfies(req.Resource, t.Rules[a], 0)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

// expandSubject follows identity relations outward from the caller, so a key
// carries the reach of whoever it acts as. It is the one direction the walk
// runs away from the resource.
func (e *Evaluator) expandSubject(ctx context.Context, subject NodeRef, now time.Time) (map[NodeRef]bool, error) {
	out := map[NodeRef]bool{subject: true}
	identity := e.schema.identityRelations()
	if len(identity) == 0 {
		return out, nil
	}

	queue := []NodeRef{subject}
	for depth := 0; len(queue) > 0; depth++ {
		if depth > e.maxDepth {
			return nil, fmt.Errorf("%w: expanding subject %s", ErrDepthExceeded, subject)
		}
		var next []NodeRef
		for _, node := range queue {
			for _, rel := range identity {
				edges, err := e.resolver.Edges(ctx, node, rel)
				if err != nil {
					return nil, err
				}
				for _, ed := range edges {
					if !ed.Active(now) || ed.Subject.IsUserset() {
						continue
					}
					if out[ed.Subject.Node] {
						continue
					}
					out[ed.Subject.Node] = true
					next = append(next, ed.Subject.Node)
				}
			}
		}
		queue = next
	}
	return out, nil
}

// --- one evaluation ---

const (
	stateOpen = 1
	stateDead = 2
)

type run struct {
	e        *Evaluator
	ctx      context.Context
	subjects map[NodeRef]bool
	now      time.Time
	state    map[string]int
	// excluded records that a subtraction removed a match that would otherwise
	// have allowed. It changes only the reported reason, never the answer.
	excluded bool
}

func (e *Evaluator) newRun(ctx context.Context, subjects map[NodeRef]bool, now time.Time) *run {
	return &run{e: e, ctx: ctx, subjects: subjects, now: now, state: map[string]int{}}
}

func (r *run) satisfies(obj NodeRef, expr Expr, depth int) (bool, []Step, error) {
	if depth > r.e.maxDepth {
		return false, nil, fmt.Errorf("%w: at %s", ErrDepthExceeded, obj)
	}
	switch x := expr.(type) {
	case RelationTerm:
		return r.memo(obj, x.String(), depth, func(d int) (bool, []Step, error) {
			return r.relation(obj, x.Relation, d)
		})
	case RuleTerm:
		return r.memo(obj, "rule:"+x.Action, depth, func(d int) (bool, []Step, error) {
			return r.rule(obj, x.Action, d)
		})
	case ArrowTerm:
		return r.memo(obj, x.String(), depth, func(d int) (bool, []Step, error) {
			return r.arrow(obj, x.Relation, x.Target, d)
		})
	case Union:
		for _, term := range x.Terms {
			ok, path, err := r.satisfies(obj, term, depth)
			if err != nil {
				return false, nil, err
			}
			if ok {
				return true, path, nil
			}
		}
		return false, nil, nil
	case Exclude:
		ok, path, err := r.satisfies(obj, x.Base, depth)
		if err != nil || !ok {
			return false, nil, err
		}
		removed, _, err := r.satisfies(obj, x.Subtract, depth)
		if err != nil {
			return false, nil, err
		}
		if removed {
			r.excluded = true
			return false, nil, nil
		}
		return true, path, nil
	default:
		return false, nil, fmt.Errorf("reach: unknown expression %T", expr)
	}
}

// memo breaks cycles and avoids repeating a negative answer. A term that is
// already in progress returns false: the data contains a loop, and the walk
// terminates rather than hanging. Positive answers return before they are
// cached, because a match ends the search.
func (r *run) memo(obj NodeRef, term string, depth int, fn func(int) (bool, []Step, error)) (bool, []Step, error) {
	key := obj.String() + "|" + term
	switch r.state[key] {
	case stateOpen, stateDead:
		return false, nil, nil
	}
	r.state[key] = stateOpen
	ok, path, err := fn(depth)
	if err != nil {
		return false, nil, err
	}
	if ok {
		delete(r.state, key)
		return true, path, nil
	}
	r.state[key] = stateDead
	return false, nil, nil
}

func (r *run) relation(obj NodeRef, rel string, depth int) (bool, []Step, error) {
	def, ok := r.e.schema.Relation(rel)
	if !ok {
		// An undeclared relation reaches nothing rather than everything.
		return false, nil, nil
	}
	edges, err := r.edgesFor(obj, rel, def)
	if err != nil {
		return false, nil, err
	}

	for _, ed := range edges {
		if !ed.Active(r.now) {
			continue
		}
		step := Step{Object: ed.Object, Relation: ed.Relation, Subject: ed.Subject}

		if ed.Subject.IsUserset() {
			ok, sub, err := r.satisfies(ed.Subject.Node, RelationTerm{Relation: ed.Subject.Relation}, depth+1)
			if err != nil {
				return false, nil, err
			}
			if ok {
				return true, append([]Step{step}, sub...), nil
			}
			continue
		}

		if r.matchesSubject(ed.Subject.Node, def) {
			return true, []Step{step}, nil
		}
		if def.Transitive {
			ok, sub, err := r.satisfies(ed.Subject.Node, RelationTerm{Relation: rel}, depth+1)
			if err != nil {
				return false, nil, err
			}
			if ok {
				return true, append([]Step{step}, sub...), nil
			}
		}
	}
	return false, nil, nil
}

// arrow walks a relation and asks again at the far end. The far term is that
// type's rule when it has one, and otherwise its relation, so a marker node
// carrying no rules still answers.
func (r *run) arrow(obj NodeRef, rel, target string, depth int) (bool, []Step, error) {
	def, ok := r.e.schema.Relation(rel)
	if !ok {
		return false, nil, nil
	}
	edges, err := r.edgesFor(obj, rel, def)
	if err != nil {
		return false, nil, err
	}
	for _, ed := range edges {
		if !ed.Active(r.now) {
			continue
		}
		step := Step{Object: ed.Object, Relation: ed.Relation, Subject: ed.Subject}

		var far Expr = RelationTerm{Relation: target}
		if t, ok := r.e.schema.Type(ed.Subject.Node.Type); ok {
			if _, hasRule := t.Rules[target]; hasRule {
				far = RuleTerm{Action: target}
			}
		}
		ok, sub, err := r.satisfies(ed.Subject.Node, far, depth+1)
		if err != nil {
			return false, nil, err
		}
		if ok {
			return true, append([]Step{step}, sub...), nil
		}
	}
	return false, nil, nil
}

func (r *run) rule(obj NodeRef, action string, depth int) (bool, []Step, error) {
	t, ok := r.e.schema.Type(obj.Type)
	if !ok {
		return false, nil, nil
	}
	expr, ok := t.Rules[action]
	if !ok {
		return false, nil, nil
	}
	return r.satisfies(obj, expr, depth)
}

// edgesFor reads the object's own edges, and the type's star node when the
// relation admits a wildcard there. The declaration is the enforcement point:
// a star edge on a structural relation is never even read.
func (r *run) edgesFor(obj NodeRef, rel string, def RelationDef) ([]Edge, error) {
	own, err := r.e.resolver.Edges(r.ctx, obj, rel)
	if err != nil {
		return nil, err
	}
	if obj.IsWildcard() || !def.Wildcard.allowsObject() {
		return own, nil
	}
	star, err := r.e.resolver.Edges(r.ctx, Star(obj.Type), rel)
	if err != nil {
		return nil, err
	}
	if len(star) == 0 {
		return own, nil
	}
	out := make([]Edge, 0, len(own)+len(star))
	return append(append(out, own...), star...), nil
}

func (r *run) matchesSubject(n NodeRef, def RelationDef) bool {
	if r.subjects[n] {
		return true
	}
	if !n.IsWildcard() || !def.Wildcard.allowsSubject() {
		return false
	}
	for s := range r.subjects {
		if s.Type == n.Type {
			return true
		}
	}
	return false
}
