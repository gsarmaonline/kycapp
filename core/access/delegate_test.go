package access

import (
	"errors"
	"testing"
	"time"
)

func orgAdmin(org string, caps ...Capability) GrantSet {
	return GrantSet{PrincipalID: "u1", Grants: []Grant{grant("held", OrgScope(org), caps...)}}
}

// Invariant 2: a granter may only hand out capabilities it already holds.
func TestCanGrantEnforcesSubsetRule(t *testing.T) {
	granter := orgAdmin("acme", capRead, capWrite)

	ok := grant("new", OrgScope("acme"), capRead)
	if err := CanGrant(granter, ok, ok.Scope.SelfRef(), CarveNone, now); err != nil {
		t.Errorf("subset grant must be allowed: %v", err)
	}

	over := grant("new", OrgScope("acme"), capRead, capInvit)
	if err := CanGrant(granter, over, over.Scope.SelfRef(), CarveNone, now); !errors.Is(err, ErrEscalation) {
		t.Errorf("want ErrEscalation for a capability not held, got %v", err)
	}
}

func TestCanGrantRefusesOtherOrganisations(t *testing.T) {
	granter := orgAdmin("acme", capRead)
	elsewhere := grant("new", OrgScope("globex"), capRead)

	if err := CanGrant(granter, elsewhere, elsewhere.Scope.SelfRef(), CarveNone, now); !errors.Is(err, ErrEscalation) {
		t.Errorf("want ErrEscalation across organisations, got %v", err)
	}
}

// Invariant 4: global scope is issued, never assigned. Holding every capability
// inside an organisation must not let you mint global reach.
func TestCanGrantRefusesGlobalFromOrgScope(t *testing.T) {
	granter := orgAdmin("acme", capRead, capWrite, capInvit)
	global := grant("new", GlobalScope(), capRead)

	if err := CanGrant(granter, global, global.Scope.SelfRef(), CarveNone, now); !errors.Is(err, ErrEscalation) {
		t.Fatalf("want ErrEscalation, got %v", err)
	}

	staff := GrantSet{Grants: []Grant{grant("held", GlobalScope(), capRead)}}
	if err := CanGrant(staff, global, global.Scope.SelfRef(), CarveNone, now); err != nil {
		t.Errorf("a global granter may issue global: %v", err)
	}
}

// An organisation-scoped granter may issue a narrower scope inside it, because
// the caller supplies the coordinates showing the containment.
func TestCanGrantNarrowerScopeWithinOrg(t *testing.T) {
	granter := orgAdmin("acme", capRead)
	project := grant("new", Scope{Kind: "project", ID: "p1"}, capRead)
	within := Ref(ScopeOrganisation, "acme", "project", "p1")

	if err := CanGrant(granter, project, within, CarveNone, now); err != nil {
		t.Errorf("org granter may issue project scope inside it: %v", err)
	}

	elsewhere := Ref(ScopeOrganisation, "globex", "project", "p1")
	if err := CanGrant(granter, project, elsewhere, CarveNone, now); !errors.Is(err, ErrEscalation) {
		t.Errorf("must refuse a project in another organisation, got %v", err)
	}
}

func TestCanGrantRequiresCoordinates(t *testing.T) {
	granter := orgAdmin("acme", capRead)
	g := grant("new", OrgScope("acme"), capRead)

	if err := CanGrant(granter, g, ScopeRef{}, CarveNone, now); !errors.Is(err, ErrEscalation) {
		t.Errorf("missing coordinates must fail closed, got %v", err)
	}
}

// The two carve-outs are the only paths that skip the subset rule, and they
// hold by construction rather than by exception.
func TestCanGrantCarveOuts(t *testing.T) {
	nothing := GrantSet{}
	anything := grant("new", GlobalScope(), capRead, capWrite, capInvit)

	for _, carve := range []Carve{CarveBreakGlass, CarveFounding} {
		if err := CanGrant(nothing, anything, anything.Scope.SelfRef(), carve, now); err != nil {
			t.Errorf("%s must be allowed: %v", carve, err)
		}
	}
	if err := CanGrant(nothing, anything, anything.Scope.SelfRef(), Carve("made_up"), now); !errors.Is(err, ErrEscalation) {
		t.Errorf("an unrecognised carve-out must fail closed, got %v", err)
	}
	// Even a carve-out must not accept a malformed grant.
	if err := CanGrant(nothing, Grant{Scope: OrgScope("acme")}, Ref(ScopeOrganisation, "acme"), CarveBreakGlass, now); err == nil {
		t.Error("carve-outs must still validate the grant")
	}
}

func TestCanGrantExpiredGrantsConferNothing(t *testing.T) {
	past := now.Add(-time.Minute)
	held := grant("held", OrgScope("acme"), capRead)
	held.ExpiresAt = &past
	granter := GrantSet{Grants: []Grant{held}}

	g := grant("new", OrgScope("acme"), capRead)
	if err := CanGrant(granter, g, g.Scope.SelfRef(), CarveNone, now); !errors.Is(err, ErrEscalation) {
		t.Errorf("an expired grant must confer nothing, got %v", err)
	}
}

// The namespace boundary: a merchant authoring roles for their app users must
// never mint a capability in KYC's namespace.
func TestCanGrantInNamespaceRefusesCrossNamespace(t *testing.T) {
	merchantNS := OrgNamespace("acme")
	merchantCaps := mustRegistry(merchantNS, "deploy:production")
	deploy := merchantCaps.MustParse("deploy:production")

	granter := GrantSet{Grants: []Grant{grant("held", OrgScope("acme"), deploy, capRead)}}

	ok := grant("new", OrgScope("acme"), deploy)
	if err := CanGrantInNamespace(granter, ok, ok.Scope.SelfRef(), merchantNS, CarveNone, now); err != nil {
		t.Errorf("in-namespace grant must be allowed: %v", err)
	}

	// capRead is a KYC capability. Even though the granter happens to hold it,
	// it must not be issuable inside the merchant's namespace.
	crossed := grant("new", OrgScope("acme"), capRead)
	if err := CanGrantInNamespace(granter, crossed, crossed.Scope.SelfRef(), merchantNS, CarveNone, now); !errors.Is(err, ErrEscalation) {
		t.Errorf("want ErrEscalation across namespaces, got %v", err)
	}
}
