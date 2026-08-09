package access

import (
	"errors"
	"fmt"
	"time"
)

// ErrEscalation means a grant would hand out more than the granter holds.
var ErrEscalation = errors.New("access: escalation")

// Carve-out identifies the two structural exceptions to the subset rule. They
// live here, named, so they are findable in review rather than scattered.
type Carve string

const (
	// CarveNone is the normal path: the subset rule applies.
	CarveNone Carve = ""
	// CarveBreakGlass is the env-derived root of trust. It holds everything by
	// definition, so the subset rule is satisfied rather than bypassed. It
	// exists so the system is recoverable when the database is empty,
	// mis-seeded, or freshly restored.
	CarveBreakGlass Carve = "break_glass"
	// CarveFounding is organisation creation. A new organisation has no
	// members, so nobody can delegate into it; the system issues the founding
	// owner grant.
	CarveFounding Carve = "founding"
)

// CanGrant reports whether granter may issue proposed.
//
// Invariant 2: no principal may grant what it does not hold. Both the scope and
// every capability of the proposed grant must already be held by the granter.
// This makes escalation structurally impossible instead of something reviewers
// have to catch.
//
// proposedAt is where the proposed scope sits, so containment is decidable
// without this package knowing any hierarchy. Granting project:p1 inside
// organisation acme passes Ref("organisation", "acme", "project", "p1"): an
// organisation-scoped granter reaches it, a different organisation does not.
// Pass proposed.Scope.SelfRef() when no wider coordinate applies.
func CanGrant(granter GrantSet, proposed Grant, proposedAt ScopeRef, carve Carve, now time.Time) error {
	if err := proposed.Validate(); err != nil {
		return err
	}
	switch carve {
	case CarveBreakGlass, CarveFounding:
		// Both hold by construction. Recorded so an audit can count how often
		// either path is used; a rising break-glass count is a signal.
		return nil
	case CarveNone:
	default:
		return fmt.Errorf("%w: unknown carve-out %q", ErrEscalation, carve)
	}

	if !proposed.Scope.IsGlobal() && len(proposedAt) == 0 {
		return fmt.Errorf("%w: no coordinates supplied for scope %s", ErrEscalation, proposed.Scope)
	}

	// Invariant 4: global scope is issued, never assigned. Only a granter who
	// already holds global may hand out global.
	if proposed.Scope.IsGlobal() && !holdsGlobal(granter, now) {
		return fmt.Errorf("%w: only a global grant may issue global scope", ErrEscalation)
	}

	held := granter.Capabilities(proposedAt, now)
	index := make(map[Capability]struct{}, len(held))
	for _, c := range held {
		index[c] = struct{}{}
	}
	if len(index) == 0 {
		return fmt.Errorf("%w: granter holds nothing at %s", ErrEscalation, proposed.Scope)
	}
	for _, c := range proposed.Capabilities {
		if _, ok := index[c]; !ok {
			return fmt.Errorf("%w: granter does not hold %s", ErrEscalation, c)
		}
	}
	return nil
}

// CanGrantInNamespace is CanGrant plus the namespace boundary: a merchant
// authoring roles for their own app users must never mint a capability in KYC's
// namespace, or in another merchant's.
func CanGrantInNamespace(granter GrantSet, proposed Grant, proposedAt ScopeRef, namespace string, carve Carve, now time.Time) error {
	for _, c := range proposed.Capabilities {
		if c.Namespace != namespace {
			return fmt.Errorf("%w: %s is outside namespace %q", ErrEscalation, c, namespace)
		}
	}
	return CanGrant(granter, proposed, proposedAt, carve, now)
}

func holdsGlobal(gs GrantSet, now time.Time) bool {
	for _, g := range gs.Grants {
		if g.Active(now) && g.Scope.IsGlobal() {
			return true
		}
	}
	return false
}
