package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

// createOrgKey mints an organisation API key as the caller, optionally scoped.
func createOrgKey(t *testing.T, h http.Handler, orgID, token, name string, scopes []string) string {
	t.Helper()
	body := map[string]any{"name": name}
	if scopes != nil {
		body["scopes"] = scopes
	}
	res := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/api-keys", body, userAuth(token))
	if res.Code != http.StatusCreated {
		t.Fatalf("create key %q: %d %s", name, res.Code, res.Body.String())
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode key: %v", err)
	}
	if out.Token == "" {
		t.Fatalf("no token in response: %s", res.Body.String())
	}
	return out.Token
}

func keyAuth(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

// A key's capabilities are the intersection of its owner's grants and its own
// scopes, so an explicit scope list narrows it and an empty list means
// "everything my owner can do" rather than everything.
func TestKeyIsBoundedByItsScopes(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	org, ownerToken, _ := doBootstrapOrg(t, h, "owner@acme.com", "Owner", "Acme", "acme-keys")
	orgID := org["organisation"].(map[string]any)["id"].(string)

	readOnly := createOrgKey(t, h, orgID, ownerToken, "read-only", []string{"app_users:read"})
	unscoped := createOrgKey(t, h, orgID, ownerToken, "full", nil)

	list := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/app-users", nil, keyAuth(readOnly))
	if list.Code != http.StatusOK {
		t.Fatalf("scoped key must read: %d %s", list.Code, list.Body.String())
	}
	write := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/app-users", map[string]any{
		"email": "someone@acme.com",
	}, keyAuth(readOnly))
	if write.Code != http.StatusForbidden {
		t.Fatalf("scoped key must not write: want 403, got %d %s", write.Code, write.Body.String())
	}

	// The unscoped key inherits the owner's full set, which for an owner is
	// everything in their organisation.
	full := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/app-users", map[string]any{
		"email": "created-by-key@acme.com",
	}, keyAuth(unscoped))
	if full.Code != http.StatusCreated {
		t.Fatalf("unscoped key must inherit the owner's capabilities: %d %s", full.Code, full.Body.String())
	}
}

// A key can never exceed the person who holds it, even when its scopes ask for
// more. This is the escalation that used to be possible: a member with
// api_keys:manage could mint an unscoped key and act beyond their own role.
func TestKeyCannotExceedItsOwner(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	org, ownerToken, _ := doBootstrapOrg(t, h, "owner2@acme.com", "Owner", "Acme2", "acme-bounded")
	orgID := org["organisation"].(map[string]any)["id"].(string)

	// A member whose role can manage keys but cannot write app users.
	roles := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/roles", map[string]any{
		"key": "keykeeper", "name": "Key keeper",
		"permission_keys": []string{"organisation:read", "api_keys:manage", "app_users:read"},
	}, userAuth(ownerToken))
	if roles.Code != http.StatusCreated {
		t.Fatalf("create role: %d %s", roles.Code, roles.Body.String())
	}
	var role struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(roles.Body.Bytes(), &role); err != nil {
		t.Fatalf("decode role: %v", err)
	}

	_, mateToken := doDevLogin(t, h, "keykeeper@acme.com", "Key Keeper")
	me := doJSON(t, h, http.MethodGet, "/v1/me", nil, userAuth(mateToken))
	var meBody struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := json.Unmarshal(me.Body.Bytes(), &meBody); err != nil {
		t.Fatalf("decode me: %v", err)
	}
	invite := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/memberships", map[string]any{
		"user_id": meBody.User.ID, "role_id": role.ID, "status": "active",
	}, userAuth(ownerToken))
	if invite.Code != http.StatusCreated && invite.Code != http.StatusOK {
		t.Fatalf("add member: %d %s", invite.Code, invite.Body.String())
	}

	// They mint an unscoped key, which under the old model meant full
	// organisation access.
	key := createOrgKey(t, h, orgID, mateToken, "keykeeper-key", nil)

	// The key reads, because its owner can.
	if read := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/app-users", nil, keyAuth(key)); read.Code != http.StatusOK {
		t.Fatalf("key must inherit the owner's read: %d %s", read.Code, read.Body.String())
	}
	// It cannot write, because its owner cannot.
	write := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/app-users", map[string]any{
		"email": "escalated@acme.com",
	}, keyAuth(key))
	if write.Code != http.StatusForbidden {
		t.Fatalf("key exceeded its owner: want 403, got %d %s", write.Code, write.Body.String())
	}
}

// Demoting or offboarding the owner reaches the key immediately, because the
// key holds nothing of its own.
//
// This is the cost of owner-derived keys and the reason ownership is meant to
// be transferable: a key that must outlive its owner's involvement has to be
// moved before they are offboarded.
func TestRevokingTheOwnerStopsTheirKey(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	org, ownerToken, _ := doBootstrapOrg(t, h, "owner3@acme.com", "Owner", "Acme3", "acme-revoke")
	orgID := org["organisation"].(map[string]any)["id"].(string)

	_, mateToken := doDevLogin(t, h, "integrator@acme.com", "Integrator")
	me := doJSON(t, h, http.MethodGet, "/v1/me", nil, userAuth(mateToken))
	var meBody struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := json.Unmarshal(me.Body.Bytes(), &meBody); err != nil {
		t.Fatalf("decode me: %v", err)
	}

	roles := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/roles", nil, userAuth(ownerToken))
	var roleList struct {
		Items []struct {
			ID  string `json:"id"`
			Key string `json:"key"`
		} `json:"items"`
	}
	if err := json.Unmarshal(roles.Body.Bytes(), &roleList); err != nil {
		t.Fatalf("decode roles: %v", err)
	}
	adminRole := ""
	for _, r := range roleList.Items {
		if r.Key == "admin" {
			adminRole = r.ID
		}
	}
	if adminRole == "" {
		t.Fatalf("no admin role seeded: %#v", roleList.Items)
	}

	invite := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/memberships", map[string]any{
		"user_id": meBody.User.ID, "role_id": adminRole, "status": "active",
	}, userAuth(ownerToken))
	if invite.Code != http.StatusCreated && invite.Code != http.StatusOK {
		t.Fatalf("add member: %d %s", invite.Code, invite.Body.String())
	}
	var membership struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(invite.Body.Bytes(), &membership); err != nil {
		t.Fatalf("decode membership: %v", err)
	}

	key := createOrgKey(t, h, orgID, mateToken, "integration", nil)
	if read := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/app-users", nil, keyAuth(key)); read.Code != http.StatusOK {
		t.Fatalf("key must work while its owner is a member: %d %s", read.Code, read.Body.String())
	}

	revoke := doJSON(t, h, http.MethodDelete, "/v1/memberships/"+membership.ID, nil, userAuth(ownerToken))
	if revoke.Code != http.StatusOK && revoke.Code != http.StatusNoContent {
		t.Fatalf("revoke membership: %d %s", revoke.Code, revoke.Body.String())
	}

	// Out of scope rather than forbidden: with the owner gone the key reaches
	// nothing, so the organisation is indistinguishable from one that does not
	// exist.
	after := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/app-users", nil, keyAuth(key))
	if after.Code != http.StatusNotFound {
		t.Fatalf("key outlived its owner's membership: want 404, got %d %s", after.Code, after.Body.String())
	}
}

// A platform key lives in the database and has no organisation, which is the
// same shape as break-glass. It must not be mistaken for it: break-glass is an
// environment credential resolved before any query, while a platform key still
// derives everything from its owner.
func TestPlatformKeyIsNotBreakGlass(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := serverWithPlatformAdmins(t, db, "ops@kyc.com")

	merchant, _, _ := doBootstrapOrg(t, h, "owner@zeta.com", "Owner", "Zeta", "zeta")
	orgID := merchant["organisation"].(map[string]any)["id"].(string)

	login, opsToken := doDevLogin(t, h, "ops@kyc.com", "Ops")
	opsID := userIDFrom(t, login)

	res := doJSON(t, h, http.MethodPost, "/v1/api-keys", map[string]any{"name": "ops-key"}, userAuth(opsToken))
	if res.Code != http.StatusCreated {
		t.Fatalf("create platform key: %d %s", res.Code, res.Body.String())
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode key: %v", err)
	}

	// While its owner is staff, the key reaches merchants.
	if read := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/app-users", nil, keyAuth(out.Token)); read.Code != http.StatusOK {
		t.Fatalf("platform key must inherit its owner's reach: %d %s", read.Code, read.Body.String())
	}

	// Revoke the owner's staff membership and the key loses everything with
	// them. Break-glass would be unaffected, which is the difference.
	membershipID := platformMembershipID(t, h, opsID)
	if membershipID == "" {
		t.Fatal("expected a platform membership for the bootstrapped staff member")
	}
	if rev := doJSON(t, h, http.MethodDelete, "/v1/memberships/"+membershipID, nil, svcAuth()); rev.Code != http.StatusOK && rev.Code != http.StatusNoContent {
		t.Fatalf("revoke: %d %s", rev.Code, rev.Body.String())
	}

	after := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/app-users", nil, keyAuth(out.Token))
	if after.Code != http.StatusNotFound {
		t.Fatalf("platform key kept access after its owner was revoked: want 404, got %d %s", after.Code, after.Body.String())
	}

	// Break-glass still works, because it is not data.
	if bg := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/app-users", nil, svcAuth()); bg.Code != http.StatusOK {
		t.Fatalf("break-glass must be unaffected: %d %s", bg.Code, bg.Body.String())
	}
}
