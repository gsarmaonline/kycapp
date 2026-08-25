package service_test

import (
	"context"
	"net/http"
	"testing"
)

// No principal grants what it does not hold.
//
// Every test here failed before the subset rule was wired in. The rule was
// written down as invariant 2 and stated in reach.CanWrite, but nothing called
// it, and KYC's own write paths do not write edges: they write roles,
// role_permissions and memberships. So the escalations below were one call each,
// and the existing suite could not see them, because every test drove the API as
// an owner, and an owner holds everything.

// seededMemberPermissions is what seedSystemRoles gives the member role: the
// read side of every resource. Assigning that role confers all of it, so a
// caller who may hand it out has to hold all of it.
func seededMemberPermissions() []string {
	return []string{
		"organisation:read",
		"members:read",
		"roles:read",
		"billing:read",
		"attributes:read",
		"app_users:read",
		"email_templates:read",
		"automations:read",
		"product_features:read",
		"activity:read",
		"usage:read",
	}
}

// roleIDByKey returns one of an organisation's roles by key.
func roleIDByKey(t *testing.T, h http.Handler, token, orgID, key string) string {
	t.Helper()
	res := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/roles", nil, userAuth(token))
	if res.Code != http.StatusOK {
		t.Fatalf("list roles: %d %s", res.Code, res.Body.String())
	}
	var body struct {
		Items []map[string]any `json:"items"`
	}
	decodeBody(t, res, &body)
	for _, r := range body.Items {
		if r["key"] == key {
			return r["id"].(string)
		}
	}
	t.Fatalf("role %q not found in %s", key, orgID)
	return ""
}

// memberWithRole creates a role holding exactly the named permissions, invites a
// user into it and accepts on their behalf. It returns the new member's session
// token and membership id.
func memberWithRole(t *testing.T, h http.Handler, ownerToken, orgID, roleKey, email string, perms []string) (string, string) {
	t.Helper()
	create := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/roles", map[string]any{
		"key": roleKey, "name": roleKey, "permission_keys": perms,
	}, userAuth(ownerToken))
	if create.Code != http.StatusCreated {
		t.Fatalf("create role %s: %d %s", roleKey, create.Code, create.Body.String())
	}
	var role map[string]any
	decodeBody(t, create, &role)

	inv := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/memberships", map[string]any{
		"email": email, "role_id": role["id"],
	}, userAuth(ownerToken))
	if inv.Code != http.StatusCreated {
		t.Fatalf("invite %s: %d %s", email, inv.Code, inv.Body.String())
	}
	var membership map[string]any
	decodeBody(t, inv, &membership)
	membershipID := membership["id"].(string)

	_, token := doDevLogin(t, h, email, email)
	acc := doJSON(t, h, http.MethodPost, "/v1/memberships/"+membershipID+"/accept", nil, userAuth(token))
	if acc.Code != http.StatusOK {
		t.Fatalf("accept %s: %d %s", email, acc.Code, acc.Body.String())
	}
	return token, membershipID
}

// roles:manage says who may author roles. It used to say what those roles could
// contain as well, which made it the only permission anyone needed: mint a role
// carrying billing:manage, then hold it.
func TestRolesManageCannotMintAPermissionTheCallerLacks(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	signed, ownerToken, _ := doBootstrapOrg(t, h, "owner@acme.com", "Owner", "Acme", "acme-grant")
	orgID := signed["organisation"].(map[string]any)["id"].(string)

	token, _ := memberWithRole(t, h, ownerToken, orgID, "role_editor", "editor@acme.com",
		[]string{"roles:manage", "roles:read", "members:read"})

	escalate := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/roles", map[string]any{
		"key": "sneaky", "name": "Sneaky", "permission_keys": []string{"billing:manage"},
	}, userAuth(token))
	if escalate.Code != http.StatusForbidden {
		t.Fatalf("roles:manage must not mint billing:manage: want 403, got %d %s",
			escalate.Code, escalate.Body.String())
	}

	// The rule narrows, it does not forbid. A permission the caller does hold
	// stays authorable, or roles:manage would mean nothing at all.
	ok := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/roles", map[string]any{
		"key": "reader", "name": "Reader", "permission_keys": []string{"members:read"},
	}, userAuth(token))
	if ok.Code != http.StatusCreated {
		t.Fatalf("a permission the caller holds must stay grantable: %d %s", ok.Code, ok.Body.String())
	}
}

// The same rule on the update path. Checking only the delta would let someone
// add a permission today and defend it tomorrow as "already there", so the whole
// resulting set is what gets checked.
func TestRolePatchCannotAddAPermissionTheCallerLacks(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	signed, ownerToken, _ := doBootstrapOrg(t, h, "owner@acme.com", "Owner", "Acme", "acme-patch")
	orgID := signed["organisation"].(map[string]any)["id"].(string)

	token, _ := memberWithRole(t, h, ownerToken, orgID, "role_editor", "editor@acme.com",
		[]string{"roles:manage", "roles:read", "members:read"})
	ownRoleID := roleIDByKey(t, h, token, orgID, "role_editor")

	// The sharpest version: widen your own role.
	patch := doJSON(t, h, http.MethodPatch, "/v1/roles/"+ownRoleID, map[string]any{
		"permission_keys": []string{"roles:manage", "roles:read", "members:read", "api_keys:manage"},
	}, userAuth(token))
	if patch.Code != http.StatusForbidden {
		t.Fatalf("a role must not widen itself past its holder: want 403, got %d %s",
			patch.Code, patch.Body.String())
	}
}

// members:invite says who may bring people in, not how much power they arrive
// with. Assigning a role confers it, so it takes the same subset rule as
// authoring one.
func TestMembersInviteCannotHandOutOwner(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	signed, ownerToken, _ := doBootstrapOrg(t, h, "owner@acme.com", "Owner", "Acme", "acme-invite-sub")
	orgID := signed["organisation"].(map[string]any)["id"].(string)
	ownerRoleID := roleIDByKey(t, h, ownerToken, orgID, "owner")
	memberRoleID := roleIDByKey(t, h, ownerToken, orgID, "member")

	// The inviter holds everything the seeded member role holds, plus the verb
	// that hands it out. That is the least an inviter can hold and still be
	// useful, and it is the point: a role that hands out another must already
	// hold what that other role confers.
	token, membershipID := memberWithRole(t, h, ownerToken, orgID, "inviter", "inviter@acme.com",
		append(seededMemberPermissions(), "members:invite"))

	// Inviting somebody else as owner.
	invite := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/memberships", map[string]any{
		"email": "accomplice@acme.com", "role_id": ownerRoleID,
	}, userAuth(token))
	if invite.Code != http.StatusForbidden {
		t.Fatalf("members:invite must not hand out owner: want 403, got %d %s",
			invite.Code, invite.Body.String())
	}

	// Promoting yourself, which was the one-call version.
	promote := doJSON(t, h, http.MethodPatch, "/v1/memberships/"+membershipID, map[string]any{
		"role_id": ownerRoleID,
	}, userAuth(token))
	if promote.Code != http.StatusForbidden {
		t.Fatalf("members:invite must not promote its own membership to owner: want 403, got %d %s",
			promote.Code, promote.Body.String())
	}

	// A role carrying nothing the caller lacks stays assignable, so inviting
	// still works for what it is for.
	ordinary := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/memberships", map[string]any{
		"email": "teammate@acme.com", "role_id": memberRoleID,
	}, userAuth(token))
	if ordinary.Code != http.StatusCreated {
		t.Fatalf("members:invite must still invite an ordinary member: %d %s",
			ordinary.Code, ordinary.Body.String())
	}
}

// Assigning a role confers its resolved set, so the check reads through
// role_extends. owner extends admin extends member, and both admin and owner
// carry every permission, so an inviter that holds only the member set reaches
// neither.
func TestMembersInviteCannotHandOutAdmin(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	signed, ownerToken, _ := doBootstrapOrg(t, h, "owner@acme.com", "Owner", "Acme", "acme-inherit")
	orgID := signed["organisation"].(map[string]any)["id"].(string)
	adminRoleID := roleIDByKey(t, h, ownerToken, orgID, "admin")

	token, _ := memberWithRole(t, h, ownerToken, orgID, "inviter", "inviter@acme.com",
		append(seededMemberPermissions(), "members:invite"))

	invite := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/memberships", map[string]any{
		"email": "accomplice@acme.com", "role_id": adminRoleID,
	}, userAuth(token))
	if invite.Code != http.StatusForbidden {
		t.Fatalf("members:invite must not hand out admin: want 403, got %d %s",
			invite.Code, invite.Body.String())
	}
}

// An owner holds everything in their own organisation, so the rule must be
// invisible to them. A subset check that refused the ordinary case would be
// reverted within a day.
func TestOwnerIsUnaffectedBySubsetRule(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	signed, ownerToken, _ := doBootstrapOrg(t, h, "owner@acme.com", "Owner", "Acme", "acme-owner-ok")
	orgID := signed["organisation"].(map[string]any)["id"].(string)
	ownerRoleID := roleIDByKey(t, h, ownerToken, orgID, "owner")

	create := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/roles", map[string]any{
		"key": "everything", "name": "Everything",
		"permission_keys": []string{"billing:manage", "api_keys:manage", "members:remove"},
	}, userAuth(ownerToken))
	if create.Code != http.StatusCreated {
		t.Fatalf("an owner must author any role in their own org: %d %s", create.Code, create.Body.String())
	}

	invite := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/memberships", map[string]any{
		"email": "second-owner@acme.com", "role_id": ownerRoleID,
	}, userAuth(ownerToken))
	if invite.Code != http.StatusCreated {
		t.Fatalf("an owner must be able to appoint another: %d %s", invite.Code, invite.Body.String())
	}
}
