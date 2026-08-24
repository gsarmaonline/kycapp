package reach

import (
	"errors"
	"reflect"
	"testing"
)

func TestExpandSetsInheritsFromParents(t *testing.T) {
	out, err := ExpandSets([]Set{
		{ID: "viewer", Own: []string{"docs:read"}},
		{ID: "editor", Own: []string{"docs:write"}, Extends: []string{"viewer"}},
		{ID: "admin", Own: []string{"docs:delete"}, Extends: []string{"editor"}},
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := out["admin"], []string{"docs:delete", "docs:read", "docs:write"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("admin = %v, wanted %v", got, want)
	}
}

func TestExpandSetsResolvesADiamondOnce(t *testing.T) {
	// Members only ever add, so a union is commutative and both paths to the
	// base collapse to the same answer.
	out, err := ExpandSets([]Set{
		{ID: "base", Own: []string{"a"}},
		{ID: "left", Own: []string{"b"}, Extends: []string{"base"}},
		{ID: "right", Own: []string{"c"}, Extends: []string{"base"}},
		{ID: "top", Extends: []string{"left", "right"}},
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := out["top"], []string{"a", "b", "c"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("top = %v, wanted %v", got, want)
	}
}

func TestExpandSetsIsOrderIndependent(t *testing.T) {
	forward := []Set{
		{ID: "a", Own: []string{"1"}},
		{ID: "b", Own: []string{"2"}, Extends: []string{"a"}},
		{ID: "c", Own: []string{"3"}, Extends: []string{"b"}},
	}
	reverse := []Set{forward[2], forward[1], forward[0]}

	one, err := ExpandSets(forward, 0)
	if err != nil {
		t.Fatal(err)
	}
	two, err := ExpandSets(reverse, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(one, two) {
		t.Fatalf("%v != %v", one, two)
	}
}

func TestExpandSetsRejectsCycles(t *testing.T) {
	_, err := ExpandSets([]Set{
		{ID: "a", Extends: []string{"b"}},
		{ID: "b", Extends: []string{"a"}},
	}, 0)
	if !errors.Is(err, ErrSetCycle) {
		t.Fatalf("err %v, wanted ErrSetCycle", err)
	}
}

func TestExpandSetsEnforcesTheDepthCap(t *testing.T) {
	// The memoised height matters here: without it the cap would depend on the
	// order sets happen to be visited in, and this chain would slip through.
	sets := []Set{{ID: "s0", Own: []string{"m"}}}
	for i := 1; i <= 8; i++ {
		sets = append(sets, Set{
			ID:      string(rune('a' + i)),
			Extends: []string{sets[i-1].ID},
		})
	}
	if _, err := ExpandSets(sets, 5); !errors.Is(err, ErrSetDepth) {
		t.Fatalf("err %v, wanted ErrSetDepth", err)
	}
}

func TestExpandSetsRejectsUnknownAndDuplicateIDs(t *testing.T) {
	if _, err := ExpandSets([]Set{{ID: "a", Extends: []string{"ghost"}}}, 0); !errors.Is(err, ErrUnknownSet) {
		t.Errorf("unknown parent was accepted")
	}
	if _, err := ExpandSets([]Set{{ID: "a"}, {ID: "a"}}, 0); !errors.Is(err, ErrUnknownSet) {
		t.Errorf("duplicate id was accepted")
	}
	if _, err := ExpandSets([]Set{{ID: ""}}, 0); !errors.Is(err, ErrUnknownSet) {
		t.Errorf("empty id was accepted")
	}
}
