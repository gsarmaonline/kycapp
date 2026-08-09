package access

import (
	"errors"
	"fmt"
	"time"
)

// ErrInvalidGrant means the grant is malformed.
var ErrInvalidGrant = errors.New("access: invalid grant")

// Constraint narrows a grant using something only the request knows. It exists
// for predicates a capability set cannot express.
//
// Deliberately tiny. "Read only" is not here, because that is simply a
// capability set containing no write verbs. Every addition to this list moves
// the package closer to being a policy language, which is the thing to avoid.
type Constraint string

const (
	// NoConstraint applies no narrowing.
	NoConstraint Constraint = ""
	// SelfSubject allows the grant only when the resource belongs to the
	// principal itself. This is what lets an app user edit their own profile
	// without a role over every profile in the organisation.
	SelfSubject Constraint = "self_subject"
)

// Valid reports whether the constraint is one this package understands. An
// unrecognised constraint must deny rather than be ignored, so callers can add
// constraints without older evaluators silently allowing more.
func (c Constraint) Valid() bool {
	switch c {
	case NoConstraint, SelfSubject:
		return true
	default:
		return false
	}
}

// Grant binds a set of capabilities to a scope, optionally narrowed and
// expiring. It carries no principal: a GrantSet supplies that, which is what
// lets the same shape describe a session, an API key, and a staff loan.
type Grant struct {
	ID           string
	Scope        Scope
	Capabilities []Capability
	Constraint   Constraint
	// ExpiresAt is nil for a standing grant.
	ExpiresAt *time.Time
	// Source records where the grant came from: a role id, "membership",
	// "api-key", "break-glass". Audit reads this, evaluation ignores it.
	Source string
}

// Validate rejects malformed grants at the boundary, so a bad row cannot become
// a silent allow or a silent deny deep in evaluation.
func (g Grant) Validate() error {
	if err := g.Scope.Validate(); err != nil {
		return err
	}
	if !g.Constraint.Valid() {
		return fmt.Errorf("%w: unknown constraint %q", ErrInvalidGrant, g.Constraint)
	}
	if len(g.Capabilities) == 0 {
		// Invariant 1: empty grants nothing. An empty capability list is
		// almost always a mistake, so refuse it rather than store a grant
		// that can never allow anything.
		return fmt.Errorf("%w: no capabilities", ErrInvalidGrant)
	}
	for _, c := range g.Capabilities {
		if c.Namespace == "" || c.Key == "" {
			return fmt.Errorf("%w: incomplete capability %q", ErrInvalidGrant, c)
		}
	}
	return nil
}

// Active reports whether the grant has not expired at now.
func (g Grant) Active(now time.Time) bool {
	return g.ExpiresAt == nil || now.Before(*g.ExpiresAt)
}

// Allows reports whether the grant carries the capability.
func (g Grant) Allows(c Capability) bool {
	for _, have := range g.Capabilities {
		if have == c {
			return true
		}
	}
	return false
}

// GrantSet is everything a principal holds, assembled once per request from
// three sources: inherent (every principal of a kind), derived (a membership is
// a grant), and stored (rows). By the time it reaches this package the
// distinction is gone: they are all just grants.
type GrantSet struct {
	// PrincipalID identifies the caller, for audit.
	PrincipalID string
	// Subject is what SelfSubject compares against, e.g. an app user id.
	// Empty means no grant with SelfSubject can ever match.
	Subject string
	Grants  []Grant
}

// Validate checks every grant in the set.
func (gs GrantSet) Validate() error {
	for i, g := range gs.Grants {
		if err := g.Validate(); err != nil {
			return fmt.Errorf("grant %d: %w", i, err)
		}
	}
	return nil
}

// Capabilities returns every capability the set holds within scopes that reach
// ref, ignoring constraints. It answers "what could this principal do here?",
// which is what an admin UI shows and what the delegation rule compares against.
func (gs GrantSet) Capabilities(ref ScopeRef, now time.Time) []Capability {
	var out []Capability
	seen := map[Capability]struct{}{}
	for _, g := range gs.Grants {
		if !g.Active(now) || !g.Scope.Contains(ref) {
			continue
		}
		for _, c := range g.Capabilities {
			if _, dup := seen[c]; dup {
				continue
			}
			seen[c] = struct{}{}
			out = append(out, c)
		}
	}
	return out
}
