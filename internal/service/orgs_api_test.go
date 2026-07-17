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

	denied := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgA, nil, userAuth(tokenB))
	if denied.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d %s", denied.Code, denied.Body.String())
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
