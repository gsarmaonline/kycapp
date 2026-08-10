package access

import (
	"errors"
	"fmt"
	"sort"
)

// MaxRoleDepth bounds how far role inheritance may nest. Five is generous for
// real hierarchies and keeps both recomputation and human reasoning bounded.
const MaxRoleDepth = 5

var (
	// ErrRoleCycle means the extends graph contains a cycle.
	ErrRoleCycle = errors.New("access: role inheritance cycle")
	// ErrRoleDepth means inheritance nests deeper than MaxRoleDepth.
	ErrRoleDepth = errors.New("access: role inheritance too deep")
	// ErrUnknownRole means a role extends something that was not supplied.
	ErrUnknownRole = errors.New("access: unknown role")
)

// Role is a named capability set. It carries no principal and no scope: that is
// exactly what a Grant adds. Editing a role reaches everyone holding it, which
// is why inheritance belongs here and never on a grant.
type Role struct {
	ID string
	// Own is what this role adds beyond its parents.
	Own []Capability
	// Extends lists parent role ids. Multiple parents are allowed: because
	// capabilities only ever add, a union is commutative and a diamond
	// resolves the same way regardless of traversal order. That property
	// disappears the moment a deny rule is introduced.
	Extends []string
}

// ExpandRoles resolves inheritance into a flat capability set per role.
//
// This runs at write time, when a role is edited, not on the request path. The
// evaluator only ever reads the flattened result, so a decision never traverses
// the graph however deep a merchant builds it.
//
// Returns an error for cycles, for depth beyond MaxRoleDepth, and for a parent
// that was not supplied, rather than silently dropping capabilities.
func ExpandRoles(roles []Role) (map[string][]Capability, error) {
	byID := make(map[string]Role, len(roles))
	for _, r := range roles {
		if r.ID == "" {
			return nil, fmt.Errorf("%w: role with empty id", ErrUnknownRole)
		}
		if _, dup := byID[r.ID]; dup {
			return nil, fmt.Errorf("%w: duplicate role id %q", ErrUnknownRole, r.ID)
		}
		byID[r.ID] = r
	}

	const (
		unvisited  = 0
		inProgress = 1
		done       = 2
	)
	state := make(map[string]int, len(roles))
	out := make(map[string][]Capability, len(roles))
	// heights must be memoised alongside out. Returning a height of zero for an
	// already-resolved role would make the depth cap depend on the order roles
	// happen to be visited in, and a deep chain would slip through.
	heights := make(map[string]int, len(roles))

	// resolve returns the effective set for id and the height of its subtree.
	// Recursion is bounded by the cycle check: no path can revisit a role.
	var resolve func(id string) ([]Capability, int, error)
	resolve = func(id string) ([]Capability, int, error) {
		switch state[id] {
		case inProgress:
			return nil, 0, fmt.Errorf("%w: at %q", ErrRoleCycle, id)
		case done:
			return out[id], heights[id], nil
		}
		role, ok := byID[id]
		if !ok {
			return nil, 0, fmt.Errorf("%w: %q", ErrUnknownRole, id)
		}

		state[id] = inProgress
		set := map[Capability]struct{}{}
		for _, c := range role.Own {
			set[c] = struct{}{}
		}
		height := 0
		for _, parent := range role.Extends {
			caps, h, err := resolve(parent)
			if err != nil {
				return nil, 0, err
			}
			if h+1 > height {
				height = h + 1
			}
			for _, c := range caps {
				set[c] = struct{}{}
			}
		}
		if height > MaxRoleDepth {
			return nil, 0, fmt.Errorf("%w: %q nests %d levels, limit is %d", ErrRoleDepth, id, height, MaxRoleDepth)
		}
		state[id] = done
		heights[id] = height
		out[id] = sortCaps(set)
		return out[id], height, nil
	}

	// Iterate in a stable order so errors are deterministic across runs.
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if _, _, err := resolve(id); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func sortCaps(set map[Capability]struct{}) []Capability {
	out := make([]Capability, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Key < out[j].Key
	})
	return out
}
