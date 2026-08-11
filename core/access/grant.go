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
//
// Two of its fields describe sets it cannot enumerate, and each carries an
// exclusion list beside it. A wildcard says "I cannot list this set"; an
// exclusion says "but I know members that do not belong". They are one feature,
// and a wildcard without its exclusion is half of it.
//
// Every exclusion narrows THIS grant and nothing else. That is what keeps the
// model additive: no grant subtracts from another, so grants stay unordered,
// evaluation stays first-match, and deleting a grant still removes access
// rather than adding it.
type Grant struct {
	ID    string
	Scope Scope
	// Except are scopes this grant does not reach, despite Scope covering them.
	// For the case positive scoping cannot express: ten thousand projects, one
	// of them confidential, and no appetite for 9,999 grants.
	Except []Scope
	// Capabilities are the concrete verbs this grant carries.
	Capabilities []Capability
	// AllCapabilitiesIn names a namespace whose every capability this grant
	// carries, including ones declared after it was written. Empty means no
	// wildcard, which is the norm.
	//
	// The registry still refuses to let anyone DECLARE a capability named "*".
	// The wildcard lives on the grant, never in the vocabulary.
	AllCapabilitiesIn string
	// ExceptCapabilities are carved out of the wildcard. Meaningless without
	// one, since a concrete list simply omits what it does not grant.
	ExceptCapabilities []Capability
	Constraint         Constraint
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
	if len(g.Capabilities) == 0 && g.AllCapabilitiesIn == "" {
		// Invariant 1: empty grants nothing. An empty capability list is
		// almost always a mistake, so refuse it rather than store a grant
		// that can never allow anything. A wildcard is the one way to carry
		// no explicit list and still allow something.
		return fmt.Errorf("%w: no capabilities", ErrInvalidGrant)
	}
	for _, c := range g.Capabilities {
		if c.Namespace == "" || c.Key == "" {
			return fmt.Errorf("%w: incomplete capability %q", ErrInvalidGrant, c)
		}
	}
	for _, c := range g.ExceptCapabilities {
		if c.Namespace == "" || c.Key == "" {
			return fmt.Errorf("%w: incomplete excluded capability %q", ErrInvalidGrant, c)
		}
		if g.AllCapabilitiesIn == "" {
			// Without a wildcard an exclusion can only mislead: the reader
			// assumes it is doing something, and the concrete list already
			// decides everything.
			return fmt.Errorf("%w: excluded capability %q without a wildcard", ErrInvalidGrant, c)
		}
	}
	for _, s := range g.Except {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("%w: excluded scope: %s", ErrInvalidGrant, err)
		}
		if s.IsGlobal() {
			// Excluding everything leaves a grant that reaches nothing, which
			// is a deleted grant written the long way round.
			return fmt.Errorf("%w: excluded scope must not be global", ErrInvalidGrant)
		}
	}
	return nil
}

// Active reports whether the grant has not expired at now.
func (g Grant) Active(now time.Time) bool {
	return g.ExpiresAt == nil || now.Before(*g.ExpiresAt)
}

// Allows reports whether the grant carries the capability.
//
// Exclusions are checked before the wildcard, never after the concrete list: a
// carve-out only ever removes what the wildcard would have added, so an
// explicitly listed capability is always carried.
func (g Grant) Allows(c Capability) bool {
	for _, have := range g.Capabilities {
		if have == c {
			return true
		}
	}
	if g.AllCapabilitiesIn == "" || g.AllCapabilitiesIn != c.Namespace {
		return false
	}
	for _, ex := range g.ExceptCapabilities {
		if ex == c {
			return false
		}
	}
	return true
}

// Reaches reports whether the grant covers the resource: inside its scope and
// outside every exclusion.
func (g Grant) Reaches(r ScopeRef) bool {
	if !g.Scope.Contains(r) {
		return false
	}
	for _, ex := range g.Except {
		if ex.Contains(r) {
			return false
		}
	}
	return true
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
// which is what an admin UI shows.
//
// It lists concrete capabilities only. A wildcard grant holds capabilities that
// do not exist yet, so no enumeration can be complete — ask Holds about a
// specific one, or HoldsAllIn about the wildcard itself. Callers that must not
// under-report use those instead; this one is for display.
func (gs GrantSet) Capabilities(ref ScopeRef, now time.Time) []Capability {
	var out []Capability
	seen := map[Capability]struct{}{}
	for _, g := range gs.Grants {
		if !g.Active(now) || !g.Reaches(ref) {
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

// Holds reports whether the set carries one capability at ref, wildcards and
// exclusions included. This is the question to ask when the answer must be
// exact; Capabilities cannot be, because a wildcard is not enumerable.
func (gs GrantSet) Holds(c Capability, ref ScopeRef, now time.Time) bool {
	for _, g := range gs.Grants {
		if g.Active(now) && g.Reaches(ref) && g.Allows(c) {
			return true
		}
	}
	return false
}

// HoldsAllIn reports whether the set carries a wildcard over a namespace at ref,
// and returns everything that wildcard excludes.
//
// Delegation needs both halves: a granter may only issue a wildcard if it holds
// one, and the grant it issues must exclude at least what the granter's own
// wildcard excludes. Otherwise "everything except refunds" could hand out
// refunds by writing "everything".
func (gs GrantSet) HoldsAllIn(namespace string, ref ScopeRef, now time.Time) (bool, []Capability) {
	for _, g := range gs.Grants {
		if !g.Active(now) || !g.Reaches(ref) || g.AllCapabilitiesIn != namespace {
			continue
		}
		return true, g.ExceptCapabilities
	}
	return false, nil
}
