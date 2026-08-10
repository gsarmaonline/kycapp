package service

import (
	"context"
	"testing"

	"github.com/gsarmaonline/kyc/core/access"
	"github.com/gsarmaonline/kyc/internal/authn"
)

// These tests construct the service with a nil store on purpose. The platform
// and API-key paths must assemble their grants from the principal alone; if
// either ever reaches for the database, this panics rather than passing quietly.
func nilStoreService() *Service { return New(nil) }

// A gate naming a permission that is not registered must deny. Treating an
// unknown key as absent would turn a typo in a gate into an open door.
func TestUnknownPermissionKeyDenies(t *testing.T) {
	svc := nilStoreService()
	ctx := authn.WithPrincipal(context.Background(), authn.Principal{
		Kind: authn.KindUser, UserID: "u1", Actor: "user:u1",
	})

	if _, err := svc.RequireOrgPermission(ctx, "org_a", "typo:nope"); err == nil {
		t.Fatal("an unregistered permission key must deny")
	}
}

// Platform reaches every organisation, and does so as an ordinary grant at
// global scope rather than as a branch inside the gate.
func TestPlatformGrantIsGlobalAndComplete(t *testing.T) {
	svc := nilStoreService()
	gs, err := svc.grantsFor(context.Background(), authn.Principal{
		Kind: authn.KindService, PlatformAdmin: true, Actor: "env-token",
	}, "any-org")
	if err != nil {
		t.Fatalf("grantsFor: %v", err)
	}
	if len(gs.Grants) != 1 || !gs.Grants[0].Scope.IsGlobal() {
		t.Fatalf("want one global grant, got %+v", gs.Grants)
	}
	for _, key := range append([]string{CapOrganisationMember}, kycPermissionKeys...) {
		d := access.Decide(gs, KYCCapabilities.MustParse(key), access.Resource{Scope: orgRef("any-org")}, decideNow())
		if !d.Allowed {
			t.Errorf("platform must hold %s, got %s", key, d.Reason)
		}
	}
}

// An org API key reaches only its own organisation, and an explicit scope list
// narrows what it may do there.
func TestOrgKeyGrantIsScopedToItsOrganisation(t *testing.T) {
	svc := nilStoreService()
	ctx := context.Background()
	scoped := authn.Principal{
		Kind: authn.KindService, OrganisationID: "org_a", APIKeyID: "k1",
		Scopes: []string{"app_users:read"}, Actor: "api-key:test",
	}

	gs, err := svc.grantsFor(ctx, scoped, "org_a")
	if err != nil {
		t.Fatalf("grantsFor: %v", err)
	}
	ref := access.Resource{Scope: orgRef("org_a")}
	read := KYCCapabilities.MustParse("app_users:read")
	write := KYCCapabilities.MustParse("app_users:write")

	if d := access.Decide(gs, read, ref, decideNow()); !d.Allowed {
		t.Errorf("scoped key must hold app_users:read, got %s", d.Reason)
	}
	if d := access.Decide(gs, write, ref, decideNow()); d.Allowed {
		t.Error("scoped key must not hold app_users:write")
	}
	// Reaching the organisation is inherent, so a narrowly-scoped key is still
	// a member rather than invisible.
	if d := access.Decide(gs, capMember, ref, decideNow()); !d.Allowed {
		t.Errorf("scoped key must still reach its organisation, got %s", d.Reason)
	}

	other, err := svc.grantsFor(ctx, scoped, "org_b")
	if err != nil {
		t.Fatalf("grantsFor(other org): %v", err)
	}
	if d := access.Decide(other, read, access.Resource{Scope: orgRef("org_b")}, decideNow()); d.Allowed {
		t.Error("an org key must not reach another organisation")
	}
	if d := access.Decide(other, capMember, access.Resource{Scope: orgRef("org_b")}, decideNow()); d.Reason != access.ReasonOutOfScope {
		t.Errorf("reaching nothing must read as out-of-scope, got %s", d.Reason)
	}
}

// An unscoped key keeps full organisation access. This is today's behaviour,
// recorded as a defect in docs/access-control.md rather than changed here, so
// the gate swap stays behaviour-preserving.
func TestUnscopedOrgKeyStillHasFullAccess(t *testing.T) {
	svc := nilStoreService()
	gs, err := svc.grantsFor(context.Background(), authn.Principal{
		Kind: authn.KindService, OrganisationID: "org_a", APIKeyID: "k1", Actor: "api-key:test",
	}, "org_a")
	if err != nil {
		t.Fatalf("grantsFor: %v", err)
	}
	ref := access.Resource{Scope: orgRef("org_a")}
	for _, key := range kycPermissionKeys {
		if d := access.Decide(gs, KYCCapabilities.MustParse(key), ref, decideNow()); !d.Allowed {
			t.Errorf("unscoped key must hold %s, got %s", key, d.Reason)
		}
	}
}
