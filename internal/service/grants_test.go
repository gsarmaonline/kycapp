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

// An ownerless key confers nothing. Keys predating ownership fail closed rather
// than keeping the unrestricted access they used to have. This path returns
// before touching the database, which the nil store proves.
func TestOwnerlessKeyConfersNothing(t *testing.T) {
	svc := nilStoreService()
	gs, err := svc.grantsFor(context.Background(), authn.Principal{
		Kind: authn.KindService, OrganisationID: "org_a", APIKeyID: "k1", Actor: "api-key:legacy",
	}, "org_a")
	if err != nil {
		t.Fatalf("grantsFor: %v", err)
	}
	if len(gs.Grants) != 0 {
		t.Fatalf("want no grants, got %+v", gs.Grants)
	}
	d := access.Decide(gs, capMember, access.Resource{Scope: orgRef("org_a")}, decideNow())
	if d.Reason != access.ReasonOutOfScope {
		t.Errorf("want out_of_scope, got %s", d.Reason)
	}
}
