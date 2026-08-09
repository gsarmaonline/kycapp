package access

import (
	"math/rand"
	"testing"
	"time"
)

var (
	now      = time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	kycCaps  = mustRegistry(NamespaceKYC, "app_users:read", "app_users:write", "members:invite", "billing:manage")
	capRead  = kycCaps.MustParse("app_users:read")
	capWrite = kycCaps.MustParse("app_users:write")
	capInvit = kycCaps.MustParse("members:invite")
)

func mustRegistry(ns string, keys ...string) *Registry {
	r, err := NewRegistry(ns, keys...)
	if err != nil {
		panic(err)
	}
	return r
}

func grant(id string, s Scope, caps ...Capability) Grant {
	return Grant{ID: id, Scope: s, Capabilities: caps}
}

func acmeRes() Resource { return Resource{Scope: Ref(ScopeOrganisation, "acme")} }

func TestDecideAllowsWithinScope(t *testing.T) {
	gs := GrantSet{Grants: []Grant{grant("g1", OrgScope("acme"), capRead)}}

	d := Decide(gs, capRead, acmeRes(), now)
	if !d.Allowed || d.Reason != ReasonAllowed {
		t.Fatalf("want allowed, got %+v", d)
	}
	if d.GrantID != "g1" {
		t.Errorf("want the deciding grant recorded for audit, got %q", d.GrantID)
	}
}

// The three denial reasons must stay distinguishable: callers map out-of-scope
// to 404 so tenants cannot be enumerated, and the others to 403.
func TestDecideDenialReasons(t *testing.T) {
	selfGrant := grant("g3", OrgScope("acme"), capWrite)
	selfGrant.Constraint = SelfSubject

	tests := []struct {
		name string
		gs   GrantSet
		cap  Capability
		res  Resource
		want Reason
	}{
		{
			name: "no grant reaches the organisation",
			gs:   GrantSet{Grants: []Grant{grant("g1", OrgScope("other"), capRead)}},
			cap:  capRead,
			res:  acmeRes(),
			want: ReasonOutOfScope,
		},
		{
			name: "reaches but lacks the capability",
			gs:   GrantSet{Grants: []Grant{grant("g2", OrgScope("acme"), capRead)}},
			cap:  capWrite,
			res:  acmeRes(),
			want: ReasonMissingCapability,
		},
		{
			name: "has the capability but the constraint rejects the resource",
			gs:   GrantSet{Subject: "u1", Grants: []Grant{selfGrant}},
			cap:  capWrite,
			res:  Resource{Scope: Ref(ScopeOrganisation, "acme"), Subject: "someone-else"},
			want: ReasonConstraintFailed,
		},
		{
			name: "empty grant set denies",
			gs:   GrantSet{},
			cap:  capRead,
			res:  acmeRes(),
			want: ReasonOutOfScope,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := Decide(tc.gs, tc.cap, tc.res, now)
			if d.Allowed {
				t.Fatalf("want denial, got allowed via %q", d.GrantID)
			}
			if d.Reason != tc.want {
				t.Errorf("reason = %q, want %q", d.Reason, tc.want)
			}
		})
	}
}

func TestDecideSelfSubject(t *testing.T) {
	g := grant("g1", OrgScope("acme"), capWrite)
	g.Constraint = SelfSubject
	gs := GrantSet{Subject: "u1", Grants: []Grant{g}}

	own := Resource{Scope: Ref(ScopeOrganisation, "acme"), Subject: "u1"}
	if d := Decide(gs, capWrite, own, now); !d.Allowed {
		t.Errorf("own record: want allowed, got %+v", d)
	}

	// A grant set with no subject must never satisfy SelfSubject, or an
	// unset field would silently widen the grant.
	anon := GrantSet{Grants: []Grant{g}}
	if d := Decide(anon, capWrite, Resource{Scope: Ref(ScopeOrganisation, "acme")}, now); d.Allowed {
		t.Errorf("empty subject must not match empty resource subject, got %+v", d)
	}
}

func TestDecideExpiry(t *testing.T) {
	past, future := now.Add(-time.Hour), now.Add(time.Hour)

	expired := grant("g1", OrgScope("acme"), capRead)
	expired.ExpiresAt = &past
	if d := Decide(GrantSet{Grants: []Grant{expired}}, capRead, acmeRes(), now); d.Allowed {
		t.Error("expired grant must not allow")
	} else if d.Reason != ReasonOutOfScope {
		t.Errorf("an expired grant must not even count as reaching: got %q", d.Reason)
	}

	live := grant("g2", OrgScope("acme"), capRead)
	live.ExpiresAt = &future
	if d := Decide(GrantSet{Grants: []Grant{live}}, capRead, acmeRes(), now); !d.Allowed {
		t.Error("unexpired grant must allow")
	}
}

func TestDecideGlobalScopeReachesEverything(t *testing.T) {
	gs := GrantSet{Grants: []Grant{grant("g1", GlobalScope(), capRead)}}
	for _, org := range []string{"acme", "globex", "initech"} {
		if d := Decide(gs, capRead, Resource{Scope: Ref(ScopeOrganisation, org)}, now); !d.Allowed {
			t.Errorf("global scope must reach %q, got %+v", org, d)
		}
	}
}

// An organisation-scoped grant reaches a resource inside one of its projects,
// because the resource carries the organisation coordinate too. This is what
// keeps containment a map lookup instead of a tree walk.
func TestDecideOrgScopeReachesNestedResource(t *testing.T) {
	gs := GrantSet{Grants: []Grant{grant("g1", OrgScope("acme"), capRead)}}
	nested := Resource{Scope: Ref(ScopeOrganisation, "acme", "project", "p1")}

	if d := Decide(gs, capRead, nested, now); !d.Allowed {
		t.Fatalf("org scope must reach a resource in its project, got %+v", d)
	}

	projectOnly := GrantSet{Grants: []Grant{grant("g2", Scope{Kind: "project", ID: "p1"}, capRead)}}
	if d := Decide(projectOnly, capRead, nested, now); !d.Allowed {
		t.Errorf("project scope must reach its own resource, got %+v", d)
	}
	if d := Decide(projectOnly, capRead, acmeRes(), now); d.Allowed {
		t.Error("project scope must not reach a resource outside the project")
	}
}

// Invariant 6: grants are additive, so a union is commutative. Evaluation must
// not depend on the order grants happen to be assembled in.
func TestDecideIsOrderIndependent(t *testing.T) {
	base := []Grant{
		grant("a", OrgScope("acme"), capRead),
		grant("b", OrgScope("acme"), capWrite),
		grant("c", GlobalScope(), capInvit),
		grant("d", OrgScope("other"), capWrite),
	}
	res := acmeRes()
	rng := rand.New(rand.NewSource(1))

	for _, cap := range []Capability{capRead, capWrite, capInvit} {
		want := Decide(GrantSet{Grants: base}, cap, res, now).Allowed
		for i := 0; i < 200; i++ {
			shuffled := append([]Grant(nil), base...)
			rng.Shuffle(len(shuffled), func(i, j int) {
				shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
			})
			if got := Decide(GrantSet{Grants: shuffled}, cap, res, now).Allowed; got != want {
				t.Fatalf("%s: order changed the outcome (%v -> %v)", cap, want, got)
			}
		}
	}
}

// Invariant 1: deny by default. No capability is ever allowed without a grant
// that carries it, whatever else the set contains.
func TestDenyByDefault(t *testing.T) {
	unrelated := []Grant{
		grant("a", OrgScope("acme"), capRead),
		grant("b", GlobalScope(), capRead),
	}
	gs := GrantSet{Subject: "u1", Grants: unrelated}

	for _, res := range []Resource{acmeRes(), {Scope: Ref(ScopeOrganisation, "x")}, {}} {
		if d := Decide(gs, capWrite, res, now); d.Allowed {
			t.Errorf("capability never granted must never be allowed: %+v", d)
		}
	}
}

// An evaluator that does not recognise a constraint must deny. Otherwise an
// older SDK would silently ignore a narrowing a newer KYC applied.
func TestUnknownConstraintDenies(t *testing.T) {
	g := grant("g1", OrgScope("acme"), capRead)
	g.Constraint = Constraint("invented_later")

	d := Decide(GrantSet{Subject: "u1", Grants: []Grant{g}}, capRead, acmeRes(), now)
	if d.Allowed {
		t.Fatal("unknown constraint must deny, not be ignored")
	}
	if d.Reason != ReasonConstraintFailed {
		t.Errorf("reason = %q, want %q", d.Reason, ReasonConstraintFailed)
	}
}

func TestGrantValidateRejectsMalformed(t *testing.T) {
	tests := []struct {
		name string
		g    Grant
	}{
		{"empty capabilities", Grant{Scope: OrgScope("acme")}},
		{"global with an id", Grant{Scope: Scope{Kind: ScopeGlobal, ID: "x"}, Capabilities: []Capability{capRead}}},
		{"org without an id", Grant{Scope: Scope{Kind: ScopeOrganisation}, Capabilities: []Capability{capRead}}},
		{"empty scope kind", Grant{Scope: Scope{}, Capabilities: []Capability{capRead}}},
		{"unknown constraint", Grant{Scope: OrgScope("acme"), Capabilities: []Capability{capRead}, Constraint: "nope"}},
		{"incomplete capability", Grant{Scope: OrgScope("acme"), Capabilities: []Capability{{Key: "x:y"}}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.g.Validate(); err == nil {
				t.Error("want error, got nil")
			}
		})
	}
}
