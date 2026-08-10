package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	httpserver "github.com/gsarmaonline/kyc/internal/http"
	"github.com/gsarmaonline/kyc/internal/service"
	"github.com/gsarmaonline/kyc/internal/store"
)

// platformOrgID is seeded by migration 000043. Tests reference it directly
// because the seed is the contract; application code reads it from
// system_state instead.
const platformOrgID = "org_platform"

// serverWithPlatformAdmins builds a handler over an existing database with a
// specific PLATFORM_ADMIN_EMAILS list, so a test can change the list the way a
// redeploy would while keeping the same users and sessions.
func serverWithPlatformAdmins(t *testing.T, db *store.Store, emails ...string) http.Handler {
	t.Helper()
	return httpserver.New(db, httpserver.Options{
		Service:             service.New(db),
		APITokens:           []string{testSvcToken},
		PlatformAdminEmails: emails,
		AuthRateLimitPerMin: 0,
		AuthDevLogin:        true,
		AppOrigin:           "http://localhost:8080",
	}).Handler()
}

func meIsPlatformAdmin(t *testing.T, h http.Handler, token string) bool {
	t.Helper()
	res := doJSON(t, h, http.MethodGet, "/v1/me", nil, userAuth(token))
	if res.Code != http.StatusOK {
		t.Fatalf("/v1/me status=%d body=%s", res.Code, res.Body.String())
	}
	var body struct {
		PlatformAdmin bool `json:"platform_admin"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode /v1/me: %v", err)
	}
	return body.PlatformAdmin
}

// platformMembershipID returns the active membership of the platform
// organisation for userID, or "" when there is none.
func platformMembershipID(t *testing.T, h http.Handler, userID string) string {
	t.Helper()
	res := doJSON(t, h, http.MethodGet, "/v1/organisations/"+platformOrgID+"/memberships", nil, svcAuth())
	if res.Code != http.StatusOK {
		t.Fatalf("list platform memberships: %d %s", res.Code, res.Body.String())
	}
	var body struct {
		Items []struct {
			ID     string `json:"id"`
			UserID string `json:"user_id"`
			Status string `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode memberships: %v", err)
	}
	for _, m := range body.Items {
		if m.UserID == userID && m.Status == "active" {
			return m.ID
		}
	}
	return ""
}

// userIDFrom pulls the user id out of a login response.
func userIDFrom(t *testing.T, login map[string]any) string {
	t.Helper()
	user, ok := login["user"].(map[string]any)
	if !ok {
		t.Fatalf("login response has no user: %#v", login)
	}
	id, _ := user["id"].(string)
	if id == "" {
		t.Fatalf("login response has no user id: %#v", user)
	}
	return id
}

// Staff status must always be revocable. It used to be a one-way latch:
// matching PLATFORM_ADMIN_EMAILS wrote users.platform_admin = true and nothing
// ever cleared it, so offboarding silently failed.
//
// It is now data. The env list only bootstraps the first staff member; after
// that, staff are members of the platform organisation, and revoking that
// membership is what demotes.
func TestStaffStatusIsRevocable(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := serverWithPlatformAdmins(t, db, "boss@kyc.com")

	login, token := doDevLogin(t, h, "boss@kyc.com", "Boss")
	bossID := userIDFrom(t, login)
	if !meIsPlatformAdmin(t, h, token) {
		t.Fatal("the bootstrap address must become staff on first login")
	}

	// Staff status is backed by a real membership, not a flag.
	membershipID := platformMembershipID(t, h, bossID)
	if membershipID == "" {
		t.Fatal("bootstrap must create a membership of the platform organisation")
	}

	// The env list is bootstrap-only. Removing the address does not demote,
	// because staff status now comes from the membership.
	stripped := serverWithPlatformAdmins(t, db)
	if !meIsPlatformAdmin(t, stripped, token) {
		t.Fatal("after bootstrap, staff status comes from the membership, not the env list")
	}

	// Revoking the membership is what demotes, and it must be immediate.
	revoke := doJSON(t, stripped, http.MethodDelete, "/v1/memberships/"+membershipID, nil, svcAuth())
	if revoke.Code != http.StatusOK && revoke.Code != http.StatusNoContent {
		t.Fatalf("revoke membership: %d %s", revoke.Code, revoke.Body.String())
	}
	if meIsPlatformAdmin(t, stripped, token) {
		t.Fatal("revoking the platform membership must demote")
	}

	// Demotion must be real, not cosmetic.
	denied := doJSON(t, stripped, http.MethodGet, "/v1/api-keys", nil, userAuth(token))
	if denied.Code != http.StatusForbidden {
		t.Fatalf("demoted staff on a platform route: want 403, got %d %s", denied.Code, denied.Body.String())
	}
}

// Bootstrap is gated on a marker, not on a count of staff, so it fires exactly
// once. Otherwise revoking every staff membership would reopen the door.
func TestBootstrapFiresOnlyOnce(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := serverWithPlatformAdmins(t, db, "first@kyc.com", "second@kyc.com")

	firstLogin, firstToken := doDevLogin(t, h, "first@kyc.com", "First")
	firstID := userIDFrom(t, firstLogin)
	if !meIsPlatformAdmin(t, h, firstToken) {
		t.Fatal("first listed address must bootstrap")
	}

	// A second listed address signing in later gets nothing: the marker is
	// consumed. Staff are added by granting a role from here on.
	_, secondToken := doDevLogin(t, h, "second@kyc.com", "Second")
	if meIsPlatformAdmin(t, h, secondToken) {
		t.Fatal("bootstrap must fire once; later addresses are not auto-promoted")
	}

	// Even after the first staff member is revoked, the door stays shut.
	membershipID := platformMembershipID(t, h, firstID)
	doJSON(t, h, http.MethodDelete, "/v1/memberships/"+membershipID, nil, svcAuth())

	_, thirdToken := doDevLogin(t, h, "second@kyc.com", "Second")
	if meIsPlatformAdmin(t, h, thirdToken) {
		t.Fatal("revoking every staff membership must not reopen bootstrap")
	}
}

// Break-glass is the root of trust: it resolves from the environment before any
// query, so it works on an empty or mis-seeded database and is unaffected by
// the email list or by any membership.
func TestBreakGlassTokenIsAlwaysPlatform(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := serverWithPlatformAdmins(t, db)

	_, token := doDevLogin(t, h, "nobody@acme.com", "Nobody")
	if meIsPlatformAdmin(t, h, token) {
		t.Fatal("an ordinary user must not be staff")
	}
	if denied := doJSON(t, h, http.MethodGet, "/v1/api-keys", nil, userAuth(token)); denied.Code != http.StatusForbidden {
		t.Fatalf("ordinary user on a platform route: want 403, got %d", denied.Code)
	}

	if allowed := doJSON(t, h, http.MethodGet, "/v1/api-keys", nil, svcAuth()); allowed.Code != http.StatusOK {
		t.Fatalf("break-glass must reach platform routes, got %d %s", allowed.Code, allowed.Body.String())
	}
}

// A merchant must never see the platform organisation. Listing is membership
// scoped, so this holds structurally, but it is the boundary that matters most
// and deserves a test that fails loudly if listing ever widens.
func TestMerchantCannotSeePlatformOrganisation(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := serverWithPlatformAdmins(t, db)

	_, token, _ := doBootstrapOrg(t, h, "merchant@acme.com", "Merchant", "Acme", "acme")

	list := doJSON(t, h, http.MethodGet, "/v1/organisations", nil, userAuth(token))
	if list.Code != http.StatusOK {
		t.Fatalf("list organisations: %d %s", list.Code, list.Body.String())
	}
	var body struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode organisations: %v", err)
	}
	for _, o := range body.Items {
		if o.ID == platformOrgID {
			t.Fatal("a merchant must not see the platform organisation")
		}
	}

	// Nor reach it directly, and the denial must be indistinguishable from a
	// missing organisation.
	direct := doJSON(t, h, http.MethodGet, "/v1/organisations/"+platformOrgID, nil, userAuth(token))
	if direct.Code != http.StatusNotFound {
		t.Fatalf("direct read of the platform organisation: want 404, got %d %s", direct.Code, direct.Body.String())
	}
}

// grantPlatformRole makes userID a member of the platform organisation with the
// given role key, using break-glass. This is how staff are added once bootstrap
// has run: by granting a role, not by editing an environment variable.
func grantPlatformRole(t *testing.T, h http.Handler, userID, roleKey string) {
	t.Helper()
	roles := doJSON(t, h, http.MethodGet, "/v1/organisations/"+platformOrgID+"/roles", nil, svcAuth())
	if roles.Code != http.StatusOK {
		t.Fatalf("list platform roles: %d %s", roles.Code, roles.Body.String())
	}
	var body struct {
		Items []struct {
			ID  string `json:"id"`
			Key string `json:"key"`
		} `json:"items"`
	}
	if err := json.Unmarshal(roles.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode roles: %v", err)
	}
	roleID := ""
	for _, r := range body.Items {
		if r.Key == roleKey {
			roleID = r.ID
		}
	}
	if roleID == "" {
		t.Fatalf("platform role %q not seeded; got %#v", roleKey, body.Items)
	}
	res := doJSON(t, h, http.MethodPost, "/v1/organisations/"+platformOrgID+"/memberships", map[string]any{
		"user_id": userID, "role_id": roleID, "status": "active",
	}, svcAuth())
	if res.Code != http.StatusCreated && res.Code != http.StatusOK {
		t.Fatalf("grant platform role %s: %d %s", roleKey, res.Code, res.Body.String())
	}
}

// The point of scoping platform access: a read-only support role reaches every
// organisation but cannot write anywhere. Previously any staff member bypassed
// permission checks entirely, so least privilege for KYC's own staff was
// impossible to express.
func TestSupportRoleReachesEveryOrgButCannotWrite(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := serverWithPlatformAdmins(t, db)

	// A merchant with data of its own.
	merchant, _, _ := doBootstrapOrg(t, h, "owner@acme.com", "Owner", "Acme", "acme")
	orgID := merchant["organisation"].(map[string]any)["id"].(string)

	// A staff member holding only the read-only support role.
	login, token := doDevLogin(t, h, "support@kyc.com", "Support")
	grantPlatformRole(t, h, userIDFrom(t, login), "support")

	if !meIsPlatformAdmin(t, h, token) {
		t.Fatal("a global-reach role must count as staff")
	}

	// Reaches a merchant it has no membership in.
	read := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/app-users", nil, userAuth(token))
	if read.Code != http.StatusOK {
		t.Fatalf("support must read any organisation: %d %s", read.Code, read.Body.String())
	}

	// But cannot write there.
	write := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/app-users", map[string]any{
		"email": "victim@acme.com",
	}, userAuth(token))
	if write.Code != http.StatusForbidden {
		t.Fatalf("support must not write into a merchant: want 403, got %d %s", write.Code, write.Body.String())
	}

	// Nor reach platform routes that need a manage capability.
	plan := doJSON(t, h, http.MethodPost, "/v1/plans", map[string]any{"key": "evil", "name": "Evil"}, userAuth(token))
	if plan.Code != http.StatusForbidden {
		t.Fatalf("support must not create plans: want 403, got %d %s", plan.Code, plan.Body.String())
	}

	// The read-only platform route it does hold stays available.
	users := doJSON(t, h, http.MethodGet, "/v1/users", nil, userAuth(token))
	if users.Code != http.StatusOK {
		t.Fatalf("support holds members:read and must list users: %d %s", users.Code, users.Body.String())
	}
}

// Root holds every capability, so it is the role that can still do anything.
func TestRootRoleHasFullReach(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := serverWithPlatformAdmins(t, db, "root@kyc.com")

	merchant, _, _ := doBootstrapOrg(t, h, "owner2@acme.com", "Owner", "Acme2", "acme2")
	orgID := merchant["organisation"].(map[string]any)["id"].(string)

	_, token := doDevLogin(t, h, "root@kyc.com", "Root")

	write := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/app-users", map[string]any{
		"email": "created-by-root@acme.com",
	}, userAuth(token))
	if write.Code != http.StatusCreated {
		t.Fatalf("root must write into any organisation: %d %s", write.Code, write.Body.String())
	}
	plan := doJSON(t, h, http.MethodPost, "/v1/plans", map[string]any{"key": "pro", "name": "Pro"}, userAuth(token))
	if plan.Code != http.StatusCreated {
		t.Fatalf("root must create plans: %d %s", plan.Code, plan.Body.String())
	}
}

// Global reach is derived from membership of the platform organisation, never
// stored on a role. A merchant can therefore configure their own roles however
// they like and never reach another tenant.
//
// This replaced a roles.grants_global_reach column. That column was safe only
// because no handler exposed it: adding the field to the role patch body would
// have handed any org admin cross-tenant reach, with nothing to catch it.
func TestMerchantRoleCannotProduceGlobalReach(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := serverWithPlatformAdmins(t, db)

	// Two merchants, each with an owner holding every permission in their org.
	a, tokenA, _ := doBootstrapOrg(t, h, "owner-a@acme.com", "Owner A", "Alpha", "alpha")
	b, _, _ := doBootstrapOrg(t, h, "owner-b@beta.com", "Owner B", "Beta", "beta")
	orgA := a["organisation"].(map[string]any)["id"].(string)
	orgB := b["organisation"].(map[string]any)["id"].(string)

	// Owner A creates the most powerful role their organisation allows.
	perms := doJSON(t, h, http.MethodGet, "/v1/permissions", nil, userAuth(tokenA))
	if perms.Code != http.StatusOK {
		t.Fatalf("list permissions: %d %s", perms.Code, perms.Body.String())
	}
	var catalog struct {
		Items []struct {
			Key string `json:"key"`
		} `json:"items"`
	}
	if err := json.Unmarshal(perms.Body.Bytes(), &catalog); err != nil {
		t.Fatalf("decode permissions: %v", err)
	}
	keys := make([]string, 0, len(catalog.Items))
	for _, p := range catalog.Items {
		keys = append(keys, p.Key)
	}

	created := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgA+"/roles", map[string]any{
		"key": "superpowers", "name": "Superpowers", "permission_keys": keys,
	}, userAuth(tokenA))
	if created.Code != http.StatusCreated {
		t.Fatalf("create role: %d %s", created.Code, created.Body.String())
	}

	// A client cannot smuggle a reach field in either: unknown fields are
	// rejected outright, so this fails at decode rather than being ignored.
	smuggled := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgA+"/roles", map[string]any{
		"key": "sneaky", "name": "Sneaky", "grants_global_reach": true,
	}, userAuth(tokenA))
	if smuggled.Code != http.StatusBadRequest {
		t.Fatalf("unknown role fields must be rejected: got %d %s", smuggled.Code, smuggled.Body.String())
	}

	// Owner A still cannot see Beta, and the denial is a 404 like any other
	// unreachable tenant.
	reach := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgB, nil, userAuth(tokenA))
	if reach.Code != http.StatusNotFound {
		t.Fatalf("merchant reached another tenant: want 404, got %d %s", reach.Code, reach.Body.String())
	}
	read := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgB+"/app-users", nil, userAuth(tokenA))
	if read.Code != http.StatusNotFound {
		t.Fatalf("merchant read another tenant's app users: want 404, got %d %s", read.Code, read.Body.String())
	}
}
