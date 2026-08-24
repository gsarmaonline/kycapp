package reach

import (
	"errors"
	"fmt"
	"sort"
)

// DefaultMaxSetDepth bounds how far named sets may nest when they are flattened
// ahead of time. It is generous for real hierarchies and keeps both the
// recomputation and the human reasoning bounded.
const DefaultMaxSetDepth = 5

var (
	// ErrSetCycle means the extends graph contains a cycle.
	ErrSetCycle = errors.New("reach: set inheritance cycle")
	// ErrSetDepth means inheritance nests deeper than the cap.
	ErrSetDepth = errors.New("reach: set inheritance too deep")
	// ErrUnknownSet means a set extends something that was not supplied.
	ErrUnknownSet = errors.New("reach: unknown set")
)

// Set is a named collection of members that may build on other sets.
//
// The walk resolves inheritance on the fly through usersets, which is the right
// trade when a chain is short and edited by hand. ExpandSets is for the other
// case: when the flattened result has to be *stored*, because something outside
// this process will read it without the graph. A tenant caching an assembled
// answer and evaluating locally is exactly that.
type Set struct {
	ID string
	// Own is what this set adds beyond its parents.
	Own []string
	// Extends lists parent ids. Multiple parents are allowed: because members
	// only ever add, a union is commutative and a diamond resolves the same way
	// regardless of traversal order. That property disappears the moment a
	// subtraction is introduced, which is why this function has none.
	Extends []string
}

// ExpandSets flattens inheritance into a member list per set.
//
// It runs at write time, when a set is edited, never on a decision path. The
// result is sorted and deduplicated, so the same input always produces the same
// output and a stored expansion can be compared for drift.
//
// Cycles, depth beyond maxDepth, and a parent that was not supplied are all
// errors rather than silently dropped members. Pass maxDepth <= 0 for
// DefaultMaxSetDepth.
func ExpandSets(sets []Set, maxDepth int) (map[string][]string, error) {
	if maxDepth <= 0 {
		maxDepth = DefaultMaxSetDepth
	}

	byID := make(map[string]Set, len(sets))
	for _, s := range sets {
		if s.ID == "" {
			return nil, fmt.Errorf("%w: set with empty id", ErrUnknownSet)
		}
		if _, dup := byID[s.ID]; dup {
			return nil, fmt.Errorf("%w: duplicate set id %q", ErrUnknownSet, s.ID)
		}
		byID[s.ID] = s
	}

	const (
		unvisited = iota
		inProgress
		done
	)
	state := make(map[string]int, len(sets))
	out := make(map[string][]string, len(sets))
	// heights must be memoised alongside out. Returning zero for an already
	// resolved set would make the depth cap depend on the order sets happen to
	// be visited in, and a deep chain would slip through.
	heights := make(map[string]int, len(sets))

	var resolve func(id string) ([]string, int, error)
	resolve = func(id string) ([]string, int, error) {
		switch state[id] {
		case inProgress:
			return nil, 0, fmt.Errorf("%w: at %q", ErrSetCycle, id)
		case done:
			return out[id], heights[id], nil
		}
		set, ok := byID[id]
		if !ok {
			return nil, 0, fmt.Errorf("%w: %q", ErrUnknownSet, id)
		}

		state[id] = inProgress
		members := map[string]struct{}{}
		for _, m := range set.Own {
			members[m] = struct{}{}
		}
		height := 0
		for _, parent := range set.Extends {
			inherited, h, err := resolve(parent)
			if err != nil {
				return nil, 0, err
			}
			if h+1 > height {
				height = h + 1
			}
			for _, m := range inherited {
				members[m] = struct{}{}
			}
		}
		if height > maxDepth {
			return nil, 0, fmt.Errorf("%w: %q nests %d levels, limit is %d", ErrSetDepth, id, height, maxDepth)
		}

		flat := make([]string, 0, len(members))
		for m := range members {
			flat = append(flat, m)
		}
		sort.Strings(flat)

		state[id] = done
		heights[id] = height
		out[id] = flat
		return flat, height, nil
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
