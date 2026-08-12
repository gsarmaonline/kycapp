package access

import "time"

// Reason explains a decision. Callers map it to a status code, and the mapping
// matters: out-of-scope must be indistinguishable from a resource that does not
// exist, or merchants can enumerate each other by reading status codes.
type Reason string

const (
	// ReasonAllowed means a grant permitted the action.
	ReasonAllowed Reason = "allowed"
	// ReasonOutOfScope means no live grant reaches the resource at all.
	// Callers map this to 404.
	ReasonOutOfScope Reason = "out_of_scope"
	// ReasonMissingCapability means a grant reaches the resource but does not
	// carry the capability. Callers map this to 403.
	ReasonMissingCapability Reason = "missing_capability"
	// ReasonConstraintFailed means a grant carries the capability but its
	// constraint rejected this particular resource. Callers map this to 403.
	ReasonConstraintFailed Reason = "constraint_failed"
)

// Resource is what an action is being attempted on.
type Resource struct {
	// Scope is every coordinate the resource belongs to.
	Scope ScopeRef
	// Subject is who the resource belongs to, for SelfSubject. Empty when the
	// resource has no owner.
	Subject string
}

// Decision is the result of an authorisation check.
type Decision struct {
	Allowed bool
	Reason  Reason
	// GrantID is the grant that allowed the action, for audit. Empty on denial.
	GrantID string
}

// Decide reports whether the grant set permits cap on r at now.
//
// Pure: no database, no ambient clock, no KYC types. The same call runs in the
// KYC API and inside a merchant's backend against a cached grant set.
//
// Evaluation is a union over grants, and grants only ever add, so order does not
// matter and no precedence rules are needed. The denial reason reported is the
// furthest any grant got, which is what makes 404-versus-403 correct: a caller
// with no reach at all learns only that nothing is there.
func Decide(gs GrantSet, cap Capability, r Resource, now time.Time) Decision {
	reached := false // some live grant covers the resource
	capable := false // ...and carries the capability

	for _, g := range gs.Grants {
		if !g.Active(now) {
			continue
		}
		if !g.Reaches(r.Scope) {
			// Out of scope and carved out of scope are the same answer here.
			// A grant that excludes a resource simply does not cover it, which
			// is why exclusions need no precedence rule against other grants.
			continue
		}
		reached = true

		if !g.Allows(cap) {
			continue
		}
		capable = true

		if !constraintSatisfied(g.Constraint, gs, r) {
			continue
		}
		return Decision{Allowed: true, Reason: ReasonAllowed, GrantID: g.ID}
	}

	switch {
	case capable:
		return Decision{Reason: ReasonConstraintFailed}
	case reached:
		return Decision{Reason: ReasonMissingCapability}
	default:
		return Decision{Reason: ReasonOutOfScope}
	}
}

// constraintSatisfied evaluates the narrowing predicates. An unrecognised
// constraint denies: an evaluator that does not understand a constraint must
// never treat it as absent.
func constraintSatisfied(c Constraint, gs GrantSet, r Resource) bool {
	switch c {
	case NoConstraint:
		return true
	case SelfSubject:
		return gs.Subject != "" && gs.Subject == r.Subject
	default:
		return false
	}
}
