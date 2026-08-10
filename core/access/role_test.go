package access

import (
	"errors"
	"math/rand"
	"reflect"
	"strconv"
	"testing"
)

func TestExpandRolesInheritsFromParents(t *testing.T) {
	roles := []Role{
		{ID: "member", Own: []Capability{capRead}},
		{ID: "admin", Own: []Capability{capWrite}, Extends: []string{"member"}},
		{ID: "owner", Own: []Capability{capInvit}, Extends: []string{"admin"}},
	}

	got, err := ExpandRoles(roles)
	if err != nil {
		t.Fatalf("ExpandRoles: %v", err)
	}
	want := map[string][]Capability{
		"member": {capRead},
		"admin":  {capRead, capWrite},
		"owner":  {capRead, capWrite, capInvit},
	}
	for id, caps := range want {
		if !reflect.DeepEqual(got[id], caps) {
			t.Errorf("%s = %v, want %v", id, got[id], caps)
		}
	}
}

// A diamond must resolve to the union with no precedence rules. This only holds
// because capabilities are additive; it is the property that makes multiple
// inheritance safe.
func TestExpandRolesDiamond(t *testing.T) {
	roles := []Role{
		{ID: "base", Own: []Capability{capRead}},
		{ID: "left", Own: []Capability{capWrite}, Extends: []string{"base"}},
		{ID: "right", Own: []Capability{capInvit}, Extends: []string{"base"}},
		{ID: "both", Extends: []string{"left", "right"}},
	}

	got, err := ExpandRoles(roles)
	if err != nil {
		t.Fatalf("ExpandRoles: %v", err)
	}
	want := []Capability{capRead, capWrite, capInvit} // sorted by key
	if !reflect.DeepEqual(got["both"], want) {
		t.Errorf("both = %v, want %v", got["both"], want)
	}
}

// Expansion must not depend on the order roles or parents are supplied in.
func TestExpandRolesIsOrderIndependent(t *testing.T) {
	roles := []Role{
		{ID: "base", Own: []Capability{capRead}},
		{ID: "left", Own: []Capability{capWrite}, Extends: []string{"base"}},
		{ID: "right", Own: []Capability{capInvit}, Extends: []string{"base"}},
		{ID: "both", Extends: []string{"left", "right"}},
	}
	want, err := ExpandRoles(roles)
	if err != nil {
		t.Fatalf("ExpandRoles: %v", err)
	}

	rng := rand.New(rand.NewSource(7))
	for i := 0; i < 200; i++ {
		shuffled := append([]Role(nil), roles...)
		rng.Shuffle(len(shuffled), func(i, j int) {
			shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
		})
		for j := range shuffled {
			ext := append([]string(nil), shuffled[j].Extends...)
			rng.Shuffle(len(ext), func(a, b int) { ext[a], ext[b] = ext[b], ext[a] })
			shuffled[j].Extends = ext
		}
		got, err := ExpandRoles(shuffled)
		if err != nil {
			t.Fatalf("ExpandRoles(shuffled): %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("order changed expansion:\n got %v\nwant %v", got, want)
		}
	}
}

func TestExpandRolesRejectsCycles(t *testing.T) {
	for _, roles := range [][]Role{
		{{ID: "a", Own: []Capability{capRead}, Extends: []string{"a"}}},
		{
			{ID: "a", Own: []Capability{capRead}, Extends: []string{"b"}},
			{ID: "b", Own: []Capability{capWrite}, Extends: []string{"a"}},
		},
	} {
		if _, err := ExpandRoles(roles); !errors.Is(err, ErrRoleCycle) {
			t.Errorf("want ErrRoleCycle, got %v", err)
		}
	}
}

func TestExpandRolesEnforcesDepthCap(t *testing.T) {
	// A chain one level deeper than the cap allows.
	var roles []Role
	for i := 0; i <= MaxRoleDepth+1; i++ {
		r := Role{ID: "r" + strconv.Itoa(i), Own: []Capability{capRead}}
		if i > 0 {
			r.Extends = []string{"r" + strconv.Itoa(i-1)}
		}
		roles = append(roles, r)
	}
	if _, err := ExpandRoles(roles); !errors.Is(err, ErrRoleDepth) {
		t.Errorf("want ErrRoleDepth, got %v", err)
	}

	// Exactly at the cap must still expand.
	if _, err := ExpandRoles(roles[:MaxRoleDepth+1]); err != nil {
		t.Errorf("depth %d must be allowed, got %v", MaxRoleDepth, err)
	}
}

// A missing parent must be an error, not a silently smaller capability set.
func TestExpandRolesRejectsUnknownParent(t *testing.T) {
	roles := []Role{{ID: "admin", Own: []Capability{capWrite}, Extends: []string{"ghost"}}}
	if _, err := ExpandRoles(roles); !errors.Is(err, ErrUnknownRole) {
		t.Errorf("want ErrUnknownRole, got %v", err)
	}
}

func TestExpandRolesRejectsDuplicateAndEmptyIDs(t *testing.T) {
	dup := []Role{{ID: "a", Own: []Capability{capRead}}, {ID: "a", Own: []Capability{capWrite}}}
	if _, err := ExpandRoles(dup); !errors.Is(err, ErrUnknownRole) {
		t.Errorf("duplicate id: want ErrUnknownRole, got %v", err)
	}
	empty := []Role{{Own: []Capability{capRead}}}
	if _, err := ExpandRoles(empty); !errors.Is(err, ErrUnknownRole) {
		t.Errorf("empty id: want ErrUnknownRole, got %v", err)
	}
}
