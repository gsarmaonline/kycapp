package service_test

import (
	"context"
	"net/http"
	"testing"
)

func TestInviteAcceptRevoke(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	signed, ownerToken, _ := doBootstrapOrg(t, h, "owner@acme.com", "Owner", "Acme", "acme-invite")
	orgID := signed["organisation"].(map[string]any)["id"].(string)

	rolesRes := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/roles", nil, userAuth(ownerToken))
	var roles struct {
		Items []map[string]any `json:"items"`
	}
	decodeBody(t, rolesRes, &roles)
	var memberRoleID string
	for _, r := range roles.Items {
		if r["key"] == "member" {
			memberRoleID = r["id"].(string)
		}
	}
	if memberRoleID == "" {
		t.Fatal("member role missing")
	}

	inv := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/memberships", map[string]any{
		"email": "teammate@acme.com", "role_id": memberRoleID,
	}, userAuth(ownerToken))
	if inv.Code != http.StatusCreated {
		t.Fatalf("invite: %s", inv.Body.String())
	}
	var membership map[string]any
	decodeBody(t, inv, &membership)
	if membership["status"] != "invited" {
		t.Fatalf("status=%v", membership["status"])
	}
	mid := membership["id"].(string)

	_, mateToken := doDevLogin(t, h, "teammate@acme.com", "Teammate")

	acc := doJSON(t, h, http.MethodPost, "/v1/memberships/"+mid+"/accept", nil, userAuth(mateToken))
	if acc.Code != http.StatusOK {
		t.Fatalf("accept: %s", acc.Body.String())
	}
	decodeBody(t, acc, &membership)
	if membership["status"] != "active" {
		t.Fatalf("after accept status=%v", membership["status"])
	}

	rev := doJSON(t, h, http.MethodDelete, "/v1/memberships/"+mid, nil, userAuth(ownerToken))
	if rev.Code != http.StatusOK {
		t.Fatalf("revoke: %s", rev.Body.String())
	}
	decodeBody(t, rev, &membership)
	if membership["status"] != "revoked" {
		t.Fatalf("after revoke status=%v", membership["status"])
	}
}
