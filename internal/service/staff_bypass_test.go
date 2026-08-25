package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Staff do not short-circuit.
//
// RequirePlatform was deleted so that a global-reach role would carry exactly
// the capabilities it was granted. Principal.IsPlatform survived it as a method
// and six handlers used it as the same bypass, so a read-only support role
// reached everything those handlers guarded. PlatformAdmin is set by any live
// membership of the platform organisation, whatever the role holds, so
// "read-only" meant nothing on those routes.
//
// The user routes are the sharp ones: the status field is how an account is
// disabled, and the guard against changing it sat inside the branch the bypass
// skipped.

// A read-only platform role holds every read permission and no write. It must
// therefore see a user and be unable to touch one.
func TestSupportRoleCannotWriteToUsers(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := serverWithPlatformAdmins(t, db)

	victimLogin, _ := doDevLogin(t, h, "victim@acme.com", "Victim")
	victimID := userIDFrom(t, victimLogin)

	login, token := doDevLogin(t, h, "support@kyc.com", "Support")
	grantPlatformRole(t, h, userIDFrom(t, login), "support")

	// members:read is held, so reading stays available.
	read := doJSON(t, h, http.MethodGet, "/v1/users/"+victimID, nil, userAuth(token))
	if read.Code != http.StatusOK {
		t.Fatalf("support holds members:read and must read a user: %d %s", read.Code, read.Body.String())
	}

	// Renaming somebody else is a write, and support holds no write.
	rename := doJSON(t, h, http.MethodPatch, "/v1/users/"+victimID, map[string]any{
		"name": "Renamed By Support",
	}, userAuth(token))
	if rename.Code != http.StatusForbidden {
		t.Fatalf("support must not edit another user: want 403, got %d %s", rename.Code, rename.Body.String())
	}

	// Disabling an account is the one that mattered.
	suspend := doJSON(t, h, http.MethodPatch, "/v1/users/"+victimID, map[string]any{
		"status": "disabled",
	}, userAuth(token))
	if suspend.Code != http.StatusForbidden {
		t.Fatalf("support must not change a user's status: want 403, got %d %s",
			suspend.Code, suspend.Body.String())
	}

	// Reading somebody else's memberships is members:read, which support holds.
	memberships := doJSON(t, h, http.MethodGet, "/v1/users/"+victimID+"/memberships", nil, userAuth(token))
	if memberships.Code != http.StatusOK {
		t.Fatalf("support must list a user's memberships: %d %s",
			memberships.Code, memberships.Body.String())
	}
}

// Root holds every permission, so the routes above stay open to it. Otherwise
// the fix would have replaced a bypass with a wall.
func TestRootRoleStillWritesToUsers(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := serverWithPlatformAdmins(t, db, "root@kyc.com")

	victimLogin, _ := doDevLogin(t, h, "victim@acme.com", "Victim")
	victimID := userIDFrom(t, victimLogin)

	_, token := doDevLogin(t, h, "root@kyc.com", "Root")

	suspend := doJSON(t, h, http.MethodPatch, "/v1/users/"+victimID, map[string]any{
		"status": "disabled",
	}, userAuth(token))
	if suspend.Code != http.StatusOK {
		t.Fatalf("root must be able to disable a user: %d %s", suspend.Code, suspend.Body.String())
	}
}

// An ordinary user keeps their own profile, and still may not disable
// themselves or read anybody else.
func TestOrdinaryUserKeepsTheirOwnProfile(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := serverWithPlatformAdmins(t, db)

	login, token := doDevLogin(t, h, "person@acme.com", "Person")
	userID := userIDFrom(t, login)

	otherLogin, _ := doDevLogin(t, h, "other@acme.com", "Other")
	otherID := userIDFrom(t, otherLogin)

	rename := doJSON(t, h, http.MethodPatch, "/v1/users/"+userID, map[string]any{
		"name": "Renamed",
	}, userAuth(token))
	if rename.Code != http.StatusOK {
		t.Fatalf("a user must be able to rename themselves: %d %s", rename.Code, rename.Body.String())
	}

	own := doJSON(t, h, http.MethodPatch, "/v1/users/"+userID, map[string]any{
		"status": "disabled",
	}, userAuth(token))
	if own.Code != http.StatusForbidden {
		t.Fatalf("a user must not change their own status: want 403, got %d %s", own.Code, own.Body.String())
	}

	peek := doJSON(t, h, http.MethodGet, "/v1/users/"+otherID, nil, userAuth(token))
	if peek.Code != http.StatusForbidden {
		t.Fatalf("a user must not read another: want 403, got %d %s", peek.Code, peek.Body.String())
	}
}

// Listing organisations used to branch on the same flag. It now asks the graph
// whether the caller reaches the organisation star node, which is the question
// every other gate asks.
func TestOrganisationListingFollowsReachNotAFlag(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := serverWithPlatformAdmins(t, db)

	merchant, merchantToken, _ := doBootstrapOrg(t, h, "owner@acme.com", "Owner", "Acme", "acme-listing")
	orgID := merchant["organisation"].(map[string]any)["id"].(string)

	login, supportToken := doDevLogin(t, h, "support@kyc.com", "Support")
	grantPlatformRole(t, h, userIDFrom(t, login), "support")

	// Support reaches every tenant, so it lists the merchant it never joined.
	staff := doJSON(t, h, http.MethodGet, "/v1/organisations", nil, userAuth(supportToken))
	if staff.Code != http.StatusOK {
		t.Fatalf("list organisations as support: %d %s", staff.Code, staff.Body.String())
	}
	if !listingContains(t, staff, orgID) {
		t.Fatal("a role reaching every organisation must list a tenant it has no membership in")
	}

	// The merchant sees only their own, and never the platform organisation.
	own := doJSON(t, h, http.MethodGet, "/v1/organisations", nil, userAuth(merchantToken))
	if own.Code != http.StatusOK {
		t.Fatalf("list organisations as merchant: %d %s", own.Code, own.Body.String())
	}
	if listingContains(t, own, platformOrgID) {
		t.Fatal("a merchant must never see the platform organisation")
	}
	if !listingContains(t, own, orgID) {
		t.Fatal("a merchant must see their own organisation")
	}
}

func listingContains(t *testing.T, res *httptest.ResponseRecorder, orgID string) bool {
	t.Helper()
	var body struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode organisations: %v", err)
	}
	for _, o := range body.Items {
		if o.ID == orgID {
			return true
		}
	}
	return false
}
