package service

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// The shape of a merchant's assembled access set.
//
// These types exist so the merchant tier owns its own wire format. They are
// deliberately inert: no evaluator, no decision, just the fields
// GET /v1/app-users/{id}/access serialises. A merchant's backend is what
// decides, in its own process, and it needs the shape rather than the engine.
//
// The tier itself is not yet modelled on the relation graph. Until it is, this
// keeps the published response identical while the previous evaluator goes
// away.

// MaxAppRoleDepth bounds how far a merchant's role inheritance may nest. Five
// is generous for real hierarchies and keeps recomputation bounded.
const MaxAppRoleDepth = 5

// AppNamespace is the namespace a merchant's own vocabulary lives in. Keeping
// their capabilities here is what lets KYC's set stay closed while theirs stays
// open: a name in one can never collide with, or widen, the other.
func AppNamespace(orgID string) string { return "org:" + orgID }

// Nothing is reserved. These names were once refused, on the reasoning that a
// merchant redefining them could collide with the tenancy boundary; they cannot,
// because that boundary is the namespace an edge is written in and not the name
// of a type. See the note in CreateAppScopeType.

// There is no constraint type here any more.
//
// self_subject was the only one, and it said "your own rows" -- answered by
// comparing the holder against the resource's subject on the read path.
// Ownership is an edge now: document:d1 #owner app_user:ana, unioned into every
// rule on the type.
//
// Both existed briefly and neither crossed over. The constraint was returned by
// GET /access and skipped by the projection; the owner edge was read by
// POST /check and invisible to GET /access. Same customer, two answers,
// depending which surface you asked. The constraint is the half that went,
// because it is the half KYC cannot make true on the graph: it never learns
// which of a merchant's rows exist, let alone who owns them, so there is nothing
// to derive an edge from.

// AppScope is one of a merchant's own levels: a declared kind plus an id that
// KYC never resolves.
type AppScope struct {
	Kind string
	ID   string
}

func (s AppScope) String() string { return s.Kind + ":" + s.ID }

// AppGrant is one row of a customer's assembled access.
//
// Every field matters to the backend that reads it. A grant carrying a wildcard
// or an exception reaches something different from what its capability list
// suggests, so code that reads `capabilities` alone will allow more than was
// granted, and KYC cannot catch that because the check runs elsewhere.
type AppGrant struct {
	ID    string
	Scope AppScope
	// Capabilities are the concrete verbs this grant carries.
	Capabilities []string
	// AllCapabilities carries every capability in the organisation's namespace,
	// including ones declared after the grant was written.
	AllCapabilities bool
	// AllScopes carries every scope of every kind in the organisation. It is the
	// widest a grant can be: an organisation is where a merchant's world ends,
	// so nothing reaches past it. Scope is empty when this is set.
	//
	// The narrower wildcard needs no field of its own. Scope.ID of "*" means
	// every instance of that kind, which is the same star the rest of the system
	// uses for "everything of this type".
	AllScopes bool
	// ExpiresAt is nil for a standing grant.
	ExpiresAt *time.Time
	// Source records how the holder came to have this: a role, a group, the
	// everyone rule. Evaluation ignores it; it is the answer to "why do I have
	// this?", which is unanswerable from the grant list alone.
	Source string
}

// ErrInvalidAppCapability means a capability key is malformed.
var ErrInvalidAppCapability = errors.New("app access: invalid capability key")

// ValidAppCapabilityKey enforces the resource:action shape.
//
// The format is not decorative: it is what lets an admin UI group capabilities
// by resource. Wildcards are refused, because one would silently widen an
// existing grant the day a new capability ships. Expand when authoring, store
// concrete; the wildcard belongs on a grant, never in the vocabulary.
func ValidAppCapabilityKey(key string) error {
	if key == "" {
		return fmt.Errorf("%w: empty key", ErrInvalidAppCapability)
	}
	if key != strings.TrimSpace(key) {
		return fmt.Errorf("%w: %q has surrounding whitespace", ErrInvalidAppCapability, key)
	}
	resource, action, ok := strings.Cut(key, ":")
	if !ok || resource == "" || action == "" {
		return fmt.Errorf("%w: %q must be resource:action", ErrInvalidAppCapability, key)
	}
	if strings.Contains(action, ":") {
		return fmt.Errorf("%w: %q has more than one colon", ErrInvalidAppCapability, key)
	}
	if strings.ContainsAny(key, "*?") {
		return fmt.Errorf("%w: %q contains a wildcard", ErrInvalidAppCapability, key)
	}
	return nil
}
