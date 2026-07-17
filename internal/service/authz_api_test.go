package service_test

import (
	"context"
	"net/http"
	"testing"
)

func TestAuthzCheckKeyAndResourceAction(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	signed, token, _ := doBootstrapOrg(t, h, "owner@acme.com", "Owner", "Acme", "acme-authz")
	orgID := signed["organisation"].(map[string]any)["id"].(string)
	ownerID := signed["user"].(map[string]any)["id"].(string)

	res := doJSON(t, h, http.MethodPost, "/v1/authz/check", map[string]any{
		"organisation_id": orgID,
		"user_id":         ownerID,
		"permission":      "members:invite",
	}, userAuth(token))
	if res.Code != http.StatusOK {
		t.Fatalf("check: %s", res.Body.String())
	}
	var check map[string]any
	decodeBody(t, res, &check)
	if check["allowed"] != true {
		t.Fatalf("owner should be allowed, got %#v", check)
	}

	res2 := doJSON(t, h, http.MethodPost, "/v1/authz/check", map[string]any{
		"organisation_id": orgID,
		"user_id":         ownerID,
		"resource":        "members",
		"action":          "invite",
	}, userAuth(token))
	decodeBody(t, res2, &check)
	if check["allowed"] != true {
		t.Fatalf("resource+action should allow, got %#v", check)
	}

	rolesRes := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/roles", nil, userAuth(token))
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
	inv := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/memberships", map[string]any{
		"email": "member@acme.com", "role_id": memberRoleID, "status": "active",
	}, userAuth(token))
	if inv.Code != http.StatusCreated {
		t.Fatalf("invite: %s", inv.Body.String())
	}
	var membership map[string]any
	decodeBody(t, inv, &membership)
	memberUserID := membership["user_id"].(string)

	denied := doJSON(t, h, http.MethodPost, "/v1/authz/check", map[string]any{
		"organisation_id": orgID,
		"user_id":         memberUserID,
		"permission":      "members:invite",
	}, userAuth(token))
	decodeBody(t, denied, &check)
	if check["allowed"] != false {
		t.Fatalf("member should be denied invite, got %#v", check)
	}

	var ownerRoleID string
	for _, r := range roles.Items {
		if r["key"] == "owner" {
			ownerRoleID = r["id"].(string)
		}
	}
	lock := doJSON(t, h, http.MethodPatch, "/v1/roles/"+ownerRoleID, map[string]any{
		"permission_keys": []string{"members:read"},
	}, userAuth(token))
	if lock.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 changing owner perms, got %d %s", lock.Code, lock.Body.String())
	}

	patch := doJSON(t, h, http.MethodPatch, "/v1/organisations/"+orgID, map[string]any{
		"status": "suspended",
	}, userAuth(token))
	if patch.Code != http.StatusOK {
		t.Fatalf("suspend: %s", patch.Body.String())
	}
	suspended := doJSON(t, h, http.MethodPost, "/v1/authz/check", map[string]any{
		"organisation_id": orgID,
		"user_id":         ownerID,
		"permission":      "members:invite",
	}, userAuth(token))
	decodeBody(t, suspended, &check)
	if check["allowed"] != false {
		t.Fatalf("suspended org should deny, got %#v", check)
	}
}
