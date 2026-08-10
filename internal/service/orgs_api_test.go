package service_test

import (
	"context"
	"net/http"
	"testing"
)

func TestMultiOrgAndMembershipList(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	first, token, userID := doBootstrapOrg(t, h, "ada@acme.com", "Ada", "Acme", "acme")
	org1 := first["organisation"].(map[string]any)["id"].(string)

	res4 := doJSON(t, h, http.MethodPost, "/v1/organisations", map[string]any{
		"name": "Beta", "slug": "beta",
	}, userAuth(token))
	if res4.Code != http.StatusCreated {
		t.Fatalf("second org status=%d body=%s", res4.Code, res4.Body.String())
	}

	resMem := doJSON(t, h, http.MethodGet, "/v1/users/"+userID+"/memberships", nil, userAuth(token))
	if resMem.Code != http.StatusOK {
		t.Fatalf("memberships status=%d body=%s", resMem.Code, resMem.Body.String())
	}
	var mems struct {
		Items []map[string]any `json:"items"`
	}
	decodeBody(t, resMem, &mems)
	if len(mems.Items) != 2 {
		t.Fatalf("want 2 memberships, got %d", len(mems.Items))
	}
	_ = org1
}

func TestTenancyBlocksCrossOrgAccess(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	a, tokenA, _ := doBootstrapOrg(t, h, "a@acme.com", "A", "OrgA", "org-a")
	_, tokenB, _ := doBootstrapOrg(t, h, "b@acme.com", "B", "OrgB", "org-b")
	orgA := a["organisation"].(map[string]any)["id"].(string)

	// 404, not 403. A caller with no reach into an organisation must not be
	// able to tell it apart from one that does not exist, or organisation ids
	// become enumerable by reading status codes.
	denied := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgA, nil, userAuth(tokenB))
	if denied.Code != http.StatusNotFound {
		t.Fatalf("cross-org read want 404, got %d %s", denied.Code, denied.Body.String())
	}

	// A missing organisation must be indistinguishable from one that exists but
	// is out of reach.
	absent := doJSON(t, h, http.MethodGet, "/v1/organisations/org_does_not_exist", nil, userAuth(tokenB))
	if absent.Code != denied.Code || absent.Body.String() != denied.Body.String() {
		t.Fatalf("out-of-reach and absent must be identical: %d %s vs %d %s",
			denied.Code, denied.Body.String(), absent.Code, absent.Body.String())
	}

	list := doJSON(t, h, http.MethodGet, "/v1/organisations", nil, userAuth(tokenA))
	var orgs struct {
		Items []map[string]any `json:"items"`
	}
	decodeBody(t, list, &orgs)
	if len(orgs.Items) != 1 || orgs.Items[0]["id"] != orgA {
		t.Fatalf("user A should only see own org, got %#v", orgs.Items)
	}

	unauth := doJSON(t, h, http.MethodGet, "/v1/organisations", nil, nil)
	if unauth.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", unauth.Code)
	}
}

// Suspending or archiving an organisation must be reversible.
//
// Status is settable through PATCH, but every organisation route except delete
// required an *active* organisation, so setting either status made the tenant
// unreachable, including by the very route that would restore it. The only
// remaining operation was deletion, which turned "suspend for non-payment" into
// a one-way door.
//
// Lifecycle routes (read, update, delete) therefore work on any status; every
// other route stays active-only, which is the point of suspending.
func TestSuspendedOrganisationCanBeRestored(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	org, token, _ := doBootstrapOrg(t, h, "owner@susp.com", "Owner", "Suspendable", "suspendable")
	orgID := org["organisation"].(map[string]any)["id"].(string)

	suspend := doJSON(t, h, http.MethodPatch, "/v1/organisations/"+orgID,
		map[string]any{"status": "suspended"}, userAuth(token))
	if suspend.Code != http.StatusOK {
		t.Fatalf("suspend: %d %s", suspend.Code, suspend.Body.String())
	}

	// Still visible to its own owner, so the state can be seen and acted on.
	if read := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID, nil, userAuth(token)); read.Code != http.StatusOK {
		t.Fatalf("a suspended organisation must remain readable by its members: %d %s", read.Code, read.Body.String())
	}

	// Data operations are closed while suspended. That is what suspension is.
	if closed := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/app-users", nil, userAuth(token)); closed.Code != http.StatusNotFound {
		t.Errorf("suspension must close data routes: want 404, got %d %s", closed.Code, closed.Body.String())
	}

	restore := doJSON(t, h, http.MethodPatch, "/v1/organisations/"+orgID,
		map[string]any{"status": "active"}, userAuth(token))
	if restore.Code != http.StatusOK {
		t.Fatalf("a suspended organisation must be restorable: %d %s", restore.Code, restore.Body.String())
	}
	if reopened := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/app-users", nil, userAuth(token)); reopened.Code != http.StatusOK {
		t.Fatalf("restoring must reopen data routes: %d %s", reopened.Code, reopened.Body.String())
	}
}
