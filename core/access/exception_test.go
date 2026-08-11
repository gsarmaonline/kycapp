package access

import (
	"errors"
	"testing"
	"time"
)

// Exclusions and wildcards are one feature: a wildcard claims a set nobody can
// enumerate, and an exclusion names the members that do not belong. These tests
// pin the property that makes them safe — an exclusion narrows its own grant and
// never another, so grants stay unordered and nothing subtracts.

const testNS = "org:acme"

func capOf(key string) Capability { return Capability{Namespace: testNS, Key: key} }

func wildcardGrant(except ...Capability) Grant {
	return Grant{
		ID:                 "g-wild",
		Scope:              Scope{Kind: "organisation", ID: "acme"},
		AllCapabilitiesIn:  testNS,
		ExceptCapabilities: except,
	}
}

func TestWildcardAllowsAnythingInItsNamespace(t *testing.T) {
	g := wildcardGrant()
	if !g.Allows(capOf("docs:read")) {
		t.Fatal("wildcard must allow a capability in its namespace")
	}
	// The whole point: a capability that did not exist when the grant was
	// written is still carried.
	if !g.Allows(capOf("invented:later")) {
		t.Error("wildcard must allow capabilities declared after the grant")
	}
	if g.Allows(Capability{Namespace: "kyc", Key: "members:write"}) {
		t.Error("wildcard must not cross its namespace")
	}
}

func TestWildcardExclusionRemovesOne(t *testing.T) {
	g := wildcardGrant(capOf("account:delete"))
	if g.Allows(capOf("account:delete")) {
		t.Error("excluded capability must not be allowed")
	}
	if !g.Allows(capOf("account:read")) {
		t.Error("exclusion must not remove anything else")
	}
}

func TestConcreteCapabilityBeatsExclusion(t *testing.T) {
	// A carve-out only removes what the wildcard would have added. Listing a
	// capability explicitly is an unambiguous statement and must win, or the
	// grant would contradict itself.
	g := wildcardGrant(capOf("account:delete"))
	g.Capabilities = []Capability{capOf("account:delete")}
	if !g.Allows(capOf("account:delete")) {
		t.Error("an explicitly listed capability must be carried")
	}
}

func TestScopeExclusionNarrowsOnlyItsOwnGrant(t *testing.T) {
	secret := Ref("organisation", "acme", "project", "salaries")
	ordinary := Ref("organisation", "acme", "project", "apollo")

	wide := Grant{
		ID:           "g-wide",
		Scope:        Scope{Kind: "organisation", ID: "acme"},
		Except:       []Scope{{Kind: "project", ID: "salaries"}},
		Capabilities: []Capability{capOf("docs:read")},
	}
	gs := GrantSet{PrincipalID: "u1", Grants: []Grant{wide}}
	now := time.Now()

	if d := Decide(gs, capOf("docs:read"), Resource{Scope: ordinary}, now); !d.Allowed {
		t.Fatalf("ordinary project must be allowed, got %s", d.Reason)
	}
	if d := Decide(gs, capOf("docs:read"), Resource{Scope: secret}, now); d.Allowed {
		t.Error("excluded project must be denied")
	}

	// The invariant that makes exclusions safe: they narrow one grant, they do
	// not veto others. A second grant reaching the same resource still allows,
	// so no grant subtracts from another and order never matters.
	narrow := Grant{
		ID:           "g-narrow",
		Scope:        Scope{Kind: "project", ID: "salaries"},
		Capabilities: []Capability{capOf("docs:read")},
	}
	gs.Grants = append(gs.Grants, narrow)
	if d := Decide(gs, capOf("docs:read"), Resource{Scope: secret}, now); !d.Allowed {
		t.Error("a separate grant must still reach an excluded resource")
	}
	gs.Grants[0], gs.Grants[1] = gs.Grants[1], gs.Grants[0]
	if d := Decide(gs, capOf("docs:read"), Resource{Scope: secret}, now); !d.Allowed {
		t.Error("the outcome must not depend on grant order")
	}
}

func TestExcludedScopeReadsAsOutOfScope(t *testing.T) {
	// A carve-out must be indistinguishable from never having had access, or
	// the denial reason leaks the existence of the resource.
	g := Grant{
		Scope:        Scope{Kind: "organisation", ID: "acme"},
		Except:       []Scope{{Kind: "project", ID: "salaries"}},
		Capabilities: []Capability{capOf("docs:read")},
	}
	gs := GrantSet{PrincipalID: "u1", Grants: []Grant{g}}
	d := Decide(gs, capOf("docs:read"), Resource{Scope: Ref("project", "salaries")}, time.Now())
	if d.Reason != ReasonOutOfScope {
		t.Errorf("want %s, got %s", ReasonOutOfScope, d.Reason)
	}
}

func TestValidateRejectsMisleadingGrants(t *testing.T) {
	base := Grant{Scope: Scope{Kind: "organisation", ID: "acme"}}

	t.Run("exclusion without a wildcard", func(t *testing.T) {
		g := base
		g.Capabilities = []Capability{capOf("docs:read")}
		g.ExceptCapabilities = []Capability{capOf("docs:write")}
		if err := g.Validate(); !errors.Is(err, ErrInvalidGrant) {
			t.Error("an exclusion with no wildcard does nothing and must be refused")
		}
	})

	t.Run("global exclusion", func(t *testing.T) {
		g := base
		g.Capabilities = []Capability{capOf("docs:read")}
		g.Except = []Scope{GlobalScope()}
		if err := g.Validate(); !errors.Is(err, ErrInvalidGrant) {
			t.Error("excluding everything is a deleted grant written the long way")
		}
	})

	t.Run("wildcard needs no explicit list", func(t *testing.T) {
		if err := wildcardGrant().Validate(); err != nil {
			t.Errorf("a wildcard grant carries capabilities: %v", err)
		}
	})
}

func TestCanGrantWildcardRequiresWildcard(t *testing.T) {
	now := time.Now()
	at := Ref("organisation", "acme")
	concrete := GrantSet{Grants: []Grant{{
		Scope:        Scope{Kind: "organisation", ID: "acme"},
		Capabilities: []Capability{capOf("docs:read")},
	}}}

	if err := CanGrant(concrete, wildcardGrant(), at, CarveNone, now); !errors.Is(err, ErrEscalation) {
		t.Error("a granter without a wildcard must not issue one")
	}
}

func TestCanGrantWildcardCarriesForwardExclusions(t *testing.T) {
	now := time.Now()
	at := Ref("organisation", "acme")
	granter := GrantSet{Grants: []Grant{wildcardGrant(capOf("billing:refund"))}}

	// The escalation this closes: hold "everything except refunds", then issue
	// "everything" and grant yourself refunds through the new grant.
	if err := CanGrant(granter, wildcardGrant(), at, CarveNone, now); !errors.Is(err, ErrEscalation) {
		t.Error("dropping the granter's own carve-out is an escalation")
	}
	if err := CanGrant(granter, wildcardGrant(capOf("billing:refund")), at, CarveNone, now); err != nil {
		t.Errorf("carrying the carve-out forward is legitimate: %v", err)
	}
	if err := CanGrant(granter, wildcardGrant(capOf("billing:refund"), capOf("account:delete")), at, CarveNone, now); err != nil {
		t.Errorf("excluding more than required is legitimate: %v", err)
	}
}

func TestCanGrantConcreteFromWildcard(t *testing.T) {
	now := time.Now()
	at := Ref("organisation", "acme")
	granter := GrantSet{Grants: []Grant{wildcardGrant(capOf("billing:refund"))}}

	proposed := Grant{
		Scope:        Scope{Kind: "organisation", ID: "acme"},
		Capabilities: []Capability{capOf("docs:read")},
	}
	if err := CanGrant(granter, proposed, at, CarveNone, now); err != nil {
		t.Errorf("a wildcard holder may grant a concrete capability: %v", err)
	}

	// Enumeration would miss this: the granter's capability list is empty, so
	// the old index-based check would have refused everything.
	proposed.Capabilities = []Capability{capOf("billing:refund")}
	if err := CanGrant(granter, proposed, at, CarveNone, now); !errors.Is(err, ErrEscalation) {
		t.Error("a capability carved out of the wildcard is not held, so it cannot be granted")
	}
}

func TestCanGrantRespectsScopeExclusion(t *testing.T) {
	now := time.Now()
	granter := GrantSet{Grants: []Grant{{
		Scope:        Scope{Kind: "organisation", ID: "acme"},
		Except:       []Scope{{Kind: "project", ID: "salaries"}},
		Capabilities: []Capability{capOf("docs:read")},
	}}}

	proposed := Grant{
		Scope:        Scope{Kind: "project", ID: "salaries"},
		Capabilities: []Capability{capOf("docs:read")},
	}
	at := Ref("organisation", "acme", "project", "salaries")
	if err := CanGrant(granter, proposed, at, CarveNone, now); !errors.Is(err, ErrEscalation) {
		t.Error("a granter cannot delegate into a scope its own grant excludes")
	}
}
