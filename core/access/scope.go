package access

import (
	"errors"
	"fmt"
	"strings"
)

// ScopeGlobal is the scope kind that reaches every resource. It is issued, never
// assigned: no role may carry it, so standing cross-tenant access always has a
// visible origin.
const ScopeGlobal = "global"

// ScopeOrganisation is KYC's own tenancy boundary.
const ScopeOrganisation = "organisation"

// ErrInvalidScope means the scope is malformed.
var ErrInvalidScope = errors.New("access: invalid scope")

// Scope names a set of resources a grant reaches: a kind plus an id.
//
// Kinds are open on purpose. KYC uses global and organisation; a merchant
// declares their own (project, environment, workspace) for their app users.
// Nothing here knows or cares which is which.
type Scope struct {
	Kind string
	ID   string // empty if and only if Kind is global
}

// GlobalScope reaches everything.
func GlobalScope() Scope { return Scope{Kind: ScopeGlobal} }

// OrgScope reaches one organisation.
func OrgScope(orgID string) Scope { return Scope{Kind: ScopeOrganisation, ID: orgID} }

func (s Scope) String() string {
	if s.Kind == ScopeGlobal {
		return ScopeGlobal
	}
	return s.Kind + ":" + s.ID
}

// IsGlobal reports whether the scope reaches every resource.
func (s Scope) IsGlobal() bool { return s.Kind == ScopeGlobal }

// Validate rejects malformed scopes. Global must carry no id; everything else
// must carry one, so a missing id can never widen a scope by accident.
func (s Scope) Validate() error {
	if strings.TrimSpace(s.Kind) == "" {
		return fmt.Errorf("%w: empty kind", ErrInvalidScope)
	}
	if s.Kind == ScopeGlobal {
		if s.ID != "" {
			return fmt.Errorf("%w: global scope must not carry an id", ErrInvalidScope)
		}
		return nil
	}
	if s.ID == "" {
		return fmt.Errorf("%w: %s scope requires an id", ErrInvalidScope, s.Kind)
	}
	return nil
}

// ScopeRef is the set of scope coordinates a resource carries, e.g.
// {"organisation": ["acme"], "project": ["p1", "p4"]}.
//
// Resources declare every coordinate they belong to, which is why containment
// needs no hierarchy logic: an organisation-scoped grant reaches a resource in
// one of its projects because the resource carries the organisation coordinate
// too. The nesting lives in the data, not in a traversal.
//
// A kind holds many ids because a resource can belong to several containers at
// once: a shared library in two projects, a document in two folders. Matching
// any one of them is enough, which keeps containment additive like everything
// else here.
type ScopeRef map[string][]string

// Ref builds a ScopeRef from alternating kind/id pairs. Repeating a kind adds
// another id to it, so Ref("project", "p1", "project", "p4") describes a
// resource in both projects. It panics on an odd number of arguments, which can
// only be a programming error at a call site.
func Ref(kv ...string) ScopeRef {
	if len(kv)%2 != 0 {
		panic("access: Ref requires alternating kind and id")
	}
	out := make(ScopeRef, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		out[kv[i]] = append(out[kv[i]], kv[i+1])
	}
	return out
}

// Contains reports whether the scope covers the resource.
//
// This is a map lookup, never a graph walk. That is the property that keeps
// "who can reach this?" answerable, and it is why scope kinds may be added
// freely but must never nest inside themselves.
func (s Scope) Contains(r ScopeRef) bool {
	if s.Kind == ScopeGlobal {
		return true
	}
	if s.ID == "" {
		return false // malformed; never match rather than match everything
	}
	for _, id := range r[s.Kind] {
		if id == s.ID {
			return true
		}
	}
	return false
}

// SelfRef returns the coordinates of the scope itself, so a scope can be tested
// for containment in another. An organisation admin granting project scope
// supplies the fuller ref (organisation and project) to Contains; this helper
// covers the simple case where no wider coordinate applies.
func (s Scope) SelfRef() ScopeRef {
	if s.Kind == ScopeGlobal {
		return ScopeRef{}
	}
	return ScopeRef{s.Kind: {s.ID}}
}
