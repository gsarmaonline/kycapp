package reach

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

// ErrEscalation means an edge would confer more than its writer holds.
var ErrEscalation = errors.New("reach: escalation")

// Carve names a structural exception to the subset rule. It is a named type
// rather than a boolean so an audit can count how often each path is used.
type Carve string

const (
	// CarveNone is the normal path: the subset rule applies.
	CarveNone Carve = ""
	// CarveRootOfTrust is the environment-derived credential. It holds
	// everything by definition, so the rule is satisfied rather than bypassed,
	// and the system stays recoverable when the store is empty or freshly
	// restored. A rising count of these is a signal.
	CarveRootOfTrust Carve = "root_of_trust"
)

// CanWrite reports whether granter may write the proposed edge.
//
// No principal grants what it does not hold. This makes escalation
// structurally impossible instead of something reviewers must catch, and it
// uses the same walk that answers an ordinary request: no separate policy
// engine decides who may grant.
//
// The check is conservative by construction. Every action the proposed
// relation feeds on the object's type must already be allowed to the granter
// at that same object. A wildcard object therefore requires the granter to
// reach the star node itself, so "every document" can never become a way to
// reach documents that could not be reached one at a time.
func (e *Evaluator) CanWrite(ctx context.Context, granter NodeRef, proposed Edge, carve Carve, now time.Time) error {
	if err := proposed.Validate(); err != nil {
		return err
	}
	if err := e.checkWritable(proposed); err != nil {
		return err
	}

	switch carve {
	case CarveRootOfTrust:
		return nil
	case CarveNone:
	default:
		return fmt.Errorf("%w: unknown carve-out %q", ErrEscalation, carve)
	}

	if err := granter.Validate(); err != nil {
		return fmt.Errorf("granter: %w", err)
	}

	for _, action := range e.ActionsFed(proposed.Object.Type, proposed.Relation) {
		d, err := e.Check(ctx, Request{
			Subject:  granter,
			Action:   action,
			Resource: proposed.Object,
		}, now)
		if err != nil {
			return err
		}
		if !d.Allowed {
			return fmt.Errorf("%w: %s does not hold %s on %s (%s)",
				ErrEscalation, granter, action, proposed.Object, d.Reason)
		}
	}
	return nil
}

// checkWritable enforces the schema's own limits on an edge, independently of
// who is writing it.
func (e *Evaluator) checkWritable(proposed Edge) error {
	def, ok := e.schema.Relation(proposed.Relation)
	if !ok {
		return fmt.Errorf("%w: undeclared relation %q", ErrInvalidSchema, proposed.Relation)
	}
	t, ok := e.schema.Type(proposed.Object.Type)
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownType, proposed.Object.Type)
	}
	targets, ok := t.Relations[proposed.Relation]
	if !ok {
		return fmt.Errorf("%w: type %q carries no relation %q",
			ErrInvalidSchema, proposed.Object.Type, proposed.Relation)
	}
	if proposed.Object.IsWildcard() && !def.Wildcard.allowsObject() {
		return fmt.Errorf("%w: relation %q does not accept a wildcard object",
			ErrInvalidSchema, proposed.Relation)
	}
	if proposed.Subject.Node.IsWildcard() && !def.Wildcard.allowsSubject() {
		return fmt.Errorf("%w: relation %q does not accept a wildcard subject",
			ErrInvalidSchema, proposed.Relation)
	}

	for _, target := range targets {
		if target.Type != proposed.Subject.Node.Type {
			continue
		}
		if target.Relation == proposed.Subject.Relation {
			return nil
		}
	}
	return fmt.Errorf("%w: %s is not a legal target for %s#%s",
		ErrInvalidSchema, proposed.Subject, proposed.Object.Type, proposed.Relation)
}

// ActionsFed returns the actions a relation can contribute to on a type,
// sorted. Only the positive side of a subtraction counts: an edge that appears
// under a minus removes access rather than conferring it, so writing one is not
// an escalation.
func (e *Evaluator) ActionsFed(typeName, relation string) []string {
	t, ok := e.schema.Type(typeName)
	if !ok {
		return nil
	}
	var out []string
	for action, expr := range t.Rules {
		if e.exprFeeds(t, expr, relation, map[string]bool{}) {
			out = append(out, action)
		}
	}
	sort.Strings(out)
	return out
}

func (e *Evaluator) exprFeeds(t *TypeDef, expr Expr, relation string, seen map[string]bool) bool {
	switch x := expr.(type) {
	case RelationTerm:
		return x.Relation == relation
	case ArrowTerm:
		return x.Relation == relation
	case RuleTerm:
		if seen[x.Action] {
			return false
		}
		seen[x.Action] = true
		defer delete(seen, x.Action)
		return e.exprFeeds(t, t.Rules[x.Action], relation, seen)
	case Union:
		for _, term := range x.Terms {
			if e.exprFeeds(t, term, relation, seen) {
				return true
			}
		}
		return false
	case Intersect:
		// Conservative on purpose. A relation inside an intersection cannot
		// grant the action alone, but requiring the granter to hold the action
		// anyway errs toward refusing a delegation, never toward allowing one.
		for _, term := range x.Terms {
			if e.exprFeeds(t, term, relation, seen) {
				return true
			}
		}
		return false
	case Exclude:
		return e.exprFeeds(t, x.Base, relation, seen)
	default:
		return false
	}
}
