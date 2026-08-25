package service_test

import (
	"net/http"
	"testing"
)

// The grant set's version, and what it fails to notice.
//
// A merchant caches the grant set and refreshes when the version moves, which
// authorisation.md names the default integration. AppAccessVersion answers that
// question with MAX(created_at) over app_grants and over group members, and
// MAX(updated_at) over app_roles. Only the role arm moves when a row changes.
//
// So a delete is invisible. Worse than invisible: removing the newest grant
// lowers MAX(created_at), and the version goes *backwards*, which a cache
// holding the higher number reads as "still current". The stale answer is
// always the wider one, because the thing that vanished was a revocation.

func (f appAccessFixture) accessVersion(t *testing.T, userID string) float64 {
	t.Helper()
	set := f.accessFor(t, userID)
	v, ok := set["version"].(float64)
	if !ok {
		t.Fatalf("access set has no version: %v", set)
	}
	return v
}

func (f appAccessFixture) grant(t *testing.T, userID, roleID, scopeID string) string {
	t.Helper()
	code, out := f.post(t, "/app-grants", map[string]any{
		"app_user_id": userID, "role_id": roleID,
		"scope_kind": "project", "scope_id": scopeID,
	})
	if code != http.StatusCreated {
		t.Fatalf("grant at %s: %d %v", scopeID, code, out)
	}
	id, _ := out["id"].(string)
	if id == "" {
		t.Fatalf("grant response has no id: %v", out)
	}
	return id
}

func (f appAccessFixture) revoke(t *testing.T, grantID string) {
	t.Helper()
	res := doJSON(t, f.h, http.MethodDelete,
		"/v1/organisations/"+f.orgID+"/app-grants/"+grantID, nil, userAuth(f.token))
	if res.Code != http.StatusOK && res.Code != http.StatusNoContent {
		t.Fatalf("revoke grant: %d %s", res.Code, res.Body.String())
	}
}

// Revoking access must move the version. This is the whole contract: a cache
// that does not refresh here keeps serving a permission that was taken away.
func TestRevokingAGrantMovesTheAccessVersion(t *testing.T) {
	f := newAppAccess(t, "avrevoke")
	f.declareScope(t, "project")
	f.declareCapability(t, "deploy:read")
	roleID := f.createRole(t, "maintainer", []string{"deploy:read"}, nil)
	userID := f.createAppUser(t, "alice@customer.com")

	keep := f.grant(t, userID, roleID, "p1")
	drop := f.grant(t, userID, roleID, "p2")
	_ = keep

	granted := f.accessVersion(t, userID)
	f.revoke(t, drop)
	revoked := f.accessVersion(t, userID)

	if revoked == granted {
		t.Fatalf("a revocation must move the version, stayed at %v", granted)
	}
	if revoked < granted {
		t.Fatalf("the version went backwards on revoke: %v -> %v; a cache holding the higher number reads that as current", granted, revoked)
	}
}

// Removing somebody from a group revokes everything the group carried, so it
// has to move the version too.
func TestRemovingAGroupMemberMovesTheAccessVersion(t *testing.T) {
	f := newAppAccess(t, "avgroup")
	f.declareScope(t, "project")
	f.declareCapability(t, "deploy:read")
	roleID := f.createRole(t, "maintainer", []string{"deploy:read"}, nil)
	userID := f.createAppUser(t, "alice@customer.com")

	code, out := f.post(t, "/app-user-groups", map[string]any{"key": "eng", "name": "Engineering"})
	if code != http.StatusCreated {
		t.Fatalf("create group: %d %v", code, out)
	}
	groupID := out["id"].(string)

	if code, out := f.post(t, "/app-user-groups/"+groupID+"/members",
		map[string]any{"app_user_id": userID}); code != http.StatusCreated && code != http.StatusOK {
		t.Fatalf("add member: %d %v", code, out)
	}
	if code, out := f.post(t, "/app-grants", map[string]any{
		"group_id": groupID, "role_id": roleID,
		"scope_kind": "project", "scope_id": "p1",
	}); code != http.StatusCreated {
		t.Fatalf("grant to group: %d %v", code, out)
	}

	joined := f.accessVersion(t, userID)

	res := doJSON(t, f.h, http.MethodDelete,
		"/v1/organisations/"+f.orgID+"/app-user-groups/"+groupID+"/members/"+userID, nil, userAuth(f.token))
	if res.Code != http.StatusOK && res.Code != http.StatusNoContent {
		t.Fatalf("remove member: %d %s", res.Code, res.Body.String())
	}
	left := f.accessVersion(t, userID)

	if left == joined {
		t.Fatalf("removing a group member must move the version, stayed at %v", joined)
	}
	if left < joined {
		t.Fatalf("the version went backwards when a member left: %v -> %v", joined, left)
	}
}
