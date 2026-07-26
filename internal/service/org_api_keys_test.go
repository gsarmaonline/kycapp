package service_test

import (
	"context"
	"net/http"
	"testing"
)

func TestOrgAPIKeyScopesLastUsedAndEntitlement(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	signed, token, _ := doBootstrapOrg(t, h, "keys@acme.com", "Keys", "Keys Org", "keys-org")
	orgID := signed["organisation"].(map[string]any)["id"].(string)
	auth := userAuth(token)

	scoped := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/api-keys", map[string]any{
		"name":   "read-users",
		"scopes": []string{"app_users:read"},
	}, auth)
	if scoped.Code != http.StatusCreated {
		t.Fatalf("create scoped key: %s", scoped.Body.String())
	}
	var scopedBody map[string]any
	decodeBody(t, scoped, &scopedBody)
	scopedToken, _ := scopedBody["token"].(string)
	scopedID, _ := scopedBody["id"].(string)
	scopes, _ := scopedBody["scopes"].([]any)
	if len(scopes) != 1 || scopes[0] != "app_users:read" {
		t.Fatalf("scopes=%#v", scopedBody["scopes"])
	}

	full := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/api-keys", map[string]any{
		"name": "full-access",
	}, auth)
	if full.Code != http.StatusCreated {
		t.Fatalf("create full key: %s", full.Body.String())
	}
	var fullBody map[string]any
	decodeBody(t, full, &fullBody)
	fullToken, _ := fullBody["token"].(string)

	listUsers := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/app-users", nil, map[string]string{
		"Authorization": "Bearer " + scopedToken,
	})
	if listUsers.Code != http.StatusOK {
		t.Fatalf("scoped read want 200, got %d %s", listUsers.Code, listUsers.Body.String())
	}

	createUser := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/app-users", map[string]any{
		"email": "pat@keys.test", "display_name": "Pat",
	}, map[string]string{"Authorization": "Bearer " + scopedToken})
	if createUser.Code != http.StatusForbidden {
		t.Fatalf("scoped write want 403, got %d %s", createUser.Code, createUser.Body.String())
	}

	createWithFull := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/app-users", map[string]any{
		"email": "pat@keys.test", "display_name": "Pat",
	}, map[string]string{"Authorization": "Bearer " + fullToken})
	if createWithFull.Code != http.StatusCreated {
		t.Fatalf("full key write want 201, got %d %s", createWithFull.Code, createWithFull.Body.String())
	}

	listed := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/api-keys", nil, auth)
	if listed.Code != http.StatusOK {
		t.Fatalf("list keys: %s", listed.Body.String())
	}
	var listBody struct {
		Items []map[string]any `json:"items"`
	}
	decodeBody(t, listed, &listBody)
	var foundScoped bool
	for _, item := range listBody.Items {
		if item["id"] == scopedID {
			foundScoped = true
			if item["last_used_at"] == nil || item["last_used_at"] == "" {
				t.Fatalf("expected last_used_at after auth: %#v", item)
			}
		}
	}
	if !foundScoped {
		t.Fatal("scoped key missing from list")
	}

	deny := doJSON(t, h, http.MethodPut, "/v1/organisations/"+orgID+"/entitlements", map[string]any{
		"overrides": []map[string]string{
			{"key": "api_access", "effect": "deny"},
		},
	}, svcAuth())
	if deny.Code != http.StatusOK {
		t.Fatalf("deny api_access: %s", deny.Body.String())
	}

	blockedAuth := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/app-users", nil, map[string]string{
		"Authorization": "Bearer " + fullToken,
	})
	if blockedAuth.Code != http.StatusUnauthorized {
		t.Fatalf("denied entitlement want 401, got %d %s", blockedAuth.Code, blockedAuth.Body.String())
	}

	blockedCreate := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/api-keys", map[string]any{
		"name": "should-fail",
	}, auth)
	if blockedCreate.Code != http.StatusForbidden {
		t.Fatalf("create without api_access want 403, got %d %s", blockedCreate.Code, blockedCreate.Body.String())
	}
}

func TestOrgAPIKeyManagePermissionRequired(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	signed, ownerToken, _ := doBootstrapOrg(t, h, "owner-keys@acme.com", "Owner", "Keys RBAC", "keys-rbac")
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
		"email": "member-keys@acme.com", "role_id": memberRoleID,
	}, userAuth(ownerToken))
	if inv.Code != http.StatusCreated {
		t.Fatalf("invite: %s", inv.Body.String())
	}
	var membership map[string]any
	decodeBody(t, inv, &membership)
	mid := membership["id"].(string)

	_, mateToken := doDevLogin(t, h, "member-keys@acme.com", "Member")
	acc := doJSON(t, h, http.MethodPost, "/v1/memberships/"+mid+"/accept", nil, userAuth(mateToken))
	if acc.Code != http.StatusOK {
		t.Fatalf("accept: %s", acc.Body.String())
	}

	denied := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/api-keys", map[string]any{
		"name": "member-key",
	}, userAuth(mateToken))
	if denied.Code != http.StatusForbidden {
		t.Fatalf("member create want 403, got %d %s", denied.Code, denied.Body.String())
	}

	listDenied := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/api-keys", nil, userAuth(mateToken))
	if listDenied.Code != http.StatusForbidden {
		t.Fatalf("member list want 403, got %d %s", listDenied.Code, listDenied.Body.String())
	}
}
