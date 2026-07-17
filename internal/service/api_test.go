package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	httpserver "github.com/gsarmaonline/kyc/internal/http"
	"github.com/gsarmaonline/kyc/internal/service"
	"github.com/gsarmaonline/kyc/internal/store"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const testSvcToken = "test-service-token"

func testServer(t *testing.T, db *store.Store) http.Handler {
	t.Helper()
	svc := service.New(db)
	return httpserver.New(db, httpserver.Options{
		Service:             svc,
		APITokens:           []string{testSvcToken},
		AuthRateLimitPerMin: 0,
		AuthDevLogin:        true,
		AppOrigin:           "http://localhost:8080",
	}).Handler()
}

func userAuth(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

func svcAuth() map[string]string {
	return map[string]string{"Authorization": "Bearer " + testSvcToken}
}

func doDevLogin(t *testing.T, h http.Handler, email, name string) (map[string]any, string) {
	t.Helper()
	res := doJSON(t, h, http.MethodPost, "/v1/auth/dev-login", map[string]any{
		"email": email, "name": name,
	}, nil)
	if res.Code != http.StatusOK {
		t.Fatalf("dev-login status=%d body=%s", res.Code, res.Body.String())
	}
	var out map[string]any
	decodeBody(t, res, &out)
	token, _ := out["token"].(string)
	if token == "" {
		t.Fatal("dev-login missing session token")
	}
	return out, token
}

func doBootstrapOrg(t *testing.T, h http.Handler, email, name, orgName, slug string) (map[string]any, string, string) {
	t.Helper()
	login, token := doDevLogin(t, h, email, name)
	userID := login["user"].(map[string]any)["id"].(string)
	res := doJSON(t, h, http.MethodPost, "/v1/organisations", map[string]any{
		"name": orgName, "slug": slug,
	}, userAuth(token))
	if res.Code != http.StatusCreated {
		t.Fatalf("create org status=%d body=%s", res.Code, res.Body.String())
	}
	var org map[string]any
	decodeBody(t, res, &org)
	return map[string]any{
		"user":         login["user"],
		"organisation": org,
	}, token, userID
}

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

func TestAppUsersAndAttributeSchema(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	first, token, _ := doBootstrapOrg(t, h, "owner@acme.com", "Owner", "Acme", "acme-users")
	orgID := first["organisation"].(map[string]any)["id"].(string)

	resDef := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/attribute-definitions", map[string]any{
		"key": "country", "label": "Country", "value_type": "dropdown",
		"section": "identity", "required": true,
		"enum_values": []string{"AU", "NZ"},
	}, userAuth(token))
	if resDef.Code != http.StatusCreated {
		t.Fatalf("create attr status=%d body=%s", resDef.Code, resDef.Body.String())
	}

	resBad := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/app-users", map[string]any{
		"display_name": "Pat",
		"attributes":   map[string]any{},
	}, userAuth(token))
	if resBad.Code != http.StatusBadRequest {
		t.Fatalf("missing required attr want 400 got %d body=%s", resBad.Code, resBad.Body.String())
	}

	resUser := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/app-users", map[string]any{
		"email":        "pat@example.com",
		"display_name": "Pat",
		"attributes":   map[string]any{"country": "AU"},
	}, userAuth(token))
	if resUser.Code != http.StatusCreated {
		t.Fatalf("create app user status=%d body=%s", resUser.Code, resUser.Body.String())
	}

	resList := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/app-users", nil, userAuth(token))
	if resList.Code != http.StatusOK {
		t.Fatalf("list app users status=%d body=%s", resList.Code, resList.Body.String())
	}
	var listed struct {
		Items []map[string]any `json:"items"`
	}
	decodeBody(t, resList, &listed)
	if len(listed.Items) != 1 {
		t.Fatalf("want 1 app user, got %d", len(listed.Items))
	}
	attrs, _ := listed.Items[0]["attributes"].(map[string]any)
	if attrs["country"] != "AU" {
		t.Fatalf("country=%v", attrs["country"])
	}

	resDefs := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/attribute-definitions", nil, userAuth(token))
	if resDefs.Code != http.StatusOK {
		t.Fatalf("list defs status=%d body=%s", resDefs.Code, resDefs.Body.String())
	}
	var defs struct {
		Items []map[string]any `json:"items"`
	}
	decodeBody(t, resDefs, &defs)
	if len(defs.Items) != 1 || defs.Items[0]["section"] != "identity" {
		t.Fatalf("defs=%v", defs.Items)
	}
}

func TestEmailTemplatesDefaultsAndCustom(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	first, token, _ := doBootstrapOrg(t, h, "owner@mail.com", "Owner", "MailCo", "mailco")
	orgID := first["organisation"].(map[string]any)["id"].(string)

	list := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/email-templates", nil, userAuth(token))
	if list.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
	var listed struct {
		Items []map[string]any `json:"items"`
	}
	decodeBody(t, list, &listed)
	if len(listed.Items) < 3 {
		t.Fatalf("want seeded defaults, got %d", len(listed.Items))
	}
	keys := map[string]bool{}
	var welcomeID string
	for _, item := range listed.Items {
		k, _ := item["key"].(string)
		keys[k] = true
		if k == "welcome" {
			welcomeID, _ = item["id"].(string)
		}
	}
	for _, want := range []string{"welcome", "payment_thank_you", "profile_incomplete"} {
		if !keys[want] {
			t.Fatalf("missing default %s in %#v", want, keys)
		}
	}

	created := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/email-templates", map[string]any{
		"key": "seasonal_offer", "name": "Seasonal offer",
		"subject": "A special offer", "body_text": "Hello {{display_name}}",
	}, userAuth(token))
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}

	if welcomeID == "" {
		t.Fatal("welcome id missing")
	}
	patched := doJSON(t, h, http.MethodPatch, "/v1/email-templates/"+welcomeID, map[string]any{
		"subject": "Welcome aboard, {{display_name}}",
	}, userAuth(token))
	if patched.Code != http.StatusOK {
		t.Fatalf("patch status=%d body=%s", patched.Code, patched.Body.String())
	}
	var welcome map[string]any
	decodeBody(t, patched, &welcome)
	if welcome["subject"] != "Welcome aboard, {{display_name}}" {
		t.Fatalf("subject=%v", welcome["subject"])
	}
	if welcome["is_system"] != true {
		t.Fatalf("welcome should remain system")
	}
}

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

func TestLoginLogoutMe(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	_, token := doDevLogin(t, h, "login@acme.com", "Login")

	me := doJSON(t, h, http.MethodGet, "/v1/me", nil, userAuth(token))
	if me.Code != http.StatusOK {
		t.Fatalf("me: %s", me.Body.String())
	}

	logout := doJSON(t, h, http.MethodPost, "/v1/auth/logout", nil, userAuth(token))
	if logout.Code != http.StatusOK {
		t.Fatalf("logout: %s", logout.Body.String())
	}
	after := doJSON(t, h, http.MethodGet, "/v1/me", nil, userAuth(token))
	if after.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session want 401, got %d", after.Code)
	}

	_, token2 := doDevLogin(t, h, "login@acme.com", "Login")
	me2 := doJSON(t, h, http.MethodGet, "/v1/me", nil, userAuth(token2))
	if me2.Code != http.StatusOK {
		t.Fatalf("re-login me: %s", me2.Body.String())
	}
}

func TestGoogleLoginLinksInviteUser(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	svc := service.New(db)

	// Simulate invite-created user (no google_sub).
	user, err := svc.CreateUser(ctx, service.CreateUserInput{Email: "invited@acme.com", Name: "invited@acme.com"})
	if err != nil {
		t.Fatal(err)
	}

	auth, err := svc.LoginWithGoogle(ctx, service.GoogleIdentity{
		Sub: "google-sub-1", Email: "invited@acme.com", EmailVerified: true, Name: "Invited Person",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if auth.User.ID != user.ID {
		t.Fatalf("should link existing invite user")
	}
	if !auth.User.GoogleSub.Valid || auth.User.GoogleSub.String != "google-sub-1" {
		t.Fatalf("google_sub not set: %#v", auth.User.GoogleSub)
	}
}

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

func TestEntitlementsEffectiveAndCheck(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	signed, token, _ := doBootstrapOrg(t, h, "bill@acme.com", "Bill", "Acme Billing", "acme-billing")
	orgID := signed["organisation"].(map[string]any)["id"].(string)

	checkAPI := doJSON(t, h, http.MethodPost, "/v1/entitlements/check", map[string]any{
		"organisation_id": orgID, "entitlement": "api_access",
	}, userAuth(token))
	var allowed map[string]any
	decodeBody(t, checkAPI, &allowed)
	if allowed["allowed"] != true {
		t.Fatalf("api_access should be allowed on trial: %#v", allowed)
	}
	checkSSO := doJSON(t, h, http.MethodPost, "/v1/entitlements/check", map[string]any{
		"organisation_id": orgID, "entitlement": "sso",
	}, userAuth(token))
	decodeBody(t, checkSSO, &allowed)
	if allowed["allowed"] != false {
		t.Fatalf("sso should be denied on trial: %#v", allowed)
	}

	put := doJSON(t, h, http.MethodPut, "/v1/organisations/"+orgID+"/entitlements", map[string]any{
		"overrides": []map[string]string{{"key": "sso", "effect": "grant"}},
	}, svcAuth())
	if put.Code != http.StatusOK {
		t.Fatalf("overrides: %s", put.Body.String())
	}
	var eff struct {
		Entitlements []string `json:"entitlements"`
	}
	decodeBody(t, put, &eff)
	hasSSO, hasAPI := false, false
	for _, k := range eff.Entitlements {
		if k == "sso" {
			hasSSO = true
		}
		if k == "api_access" {
			hasAPI = true
		}
	}
	if !hasSSO || !hasAPI {
		t.Fatalf("effective=%v", eff.Entitlements)
	}

	decodeBody(t, doJSON(t, h, http.MethodPost, "/v1/entitlements/check", map[string]any{
		"organisation_id": orgID, "entitlement": "sso",
	}, userAuth(token)), &allowed)
	if allowed["allowed"] != true {
		t.Fatalf("sso should be granted: %#v", allowed)
	}

	doJSON(t, h, http.MethodPut, "/v1/organisations/"+orgID+"/entitlements", map[string]any{
		"overrides": []map[string]string{
			{"key": "sso", "effect": "grant"},
			{"key": "api_access", "effect": "deny"},
		},
	}, svcAuth())
	decodeBody(t, doJSON(t, h, http.MethodPost, "/v1/entitlements/check", map[string]any{
		"organisation_id": orgID, "entitlement": "api_access",
	}, userAuth(token)), &allowed)
	if allowed["allowed"] != false {
		t.Fatalf("api_access denied override failed: %#v", allowed)
	}

	pro := doJSON(t, h, http.MethodPost, "/v1/plans", map[string]any{
		"key": "pro", "name": "Pro",
	}, svcAuth())
	if pro.Code != http.StatusCreated {
		t.Fatalf("create plan: %s", pro.Body.String())
	}
	var plan map[string]any
	decodeBody(t, pro, &plan)
	planID := plan["id"].(string)
	setEnt := doJSON(t, h, http.MethodPut, "/v1/plans/"+planID+"/entitlements", map[string]any{
		"entitlement_keys": []string{"api_access", "sso"},
	}, svcAuth())
	if setEnt.Code != http.StatusOK {
		t.Fatalf("set plan ents: %s", setEnt.Body.String())
	}
	doJSON(t, h, http.MethodPut, "/v1/organisations/"+orgID+"/entitlements", map[string]any{
		"overrides": []any{},
	}, svcAuth())
	sub := doJSON(t, h, http.MethodPut, "/v1/organisations/"+orgID+"/subscription", map[string]any{
		"plan_id": planID, "status": "active",
	}, svcAuth())
	if sub.Code != http.StatusOK {
		t.Fatalf("subscription: %s", sub.Body.String())
	}
	decodeBody(t, doJSON(t, h, http.MethodPost, "/v1/entitlements/check", map[string]any{
		"organisation_id": orgID, "entitlement": "sso",
	}, userAuth(token)), &allowed)
	if allowed["allowed"] != true {
		t.Fatalf("pro should include sso: %#v", allowed)
	}
}

func openTestDB(t *testing.T, ctx context.Context) *store.Store {
	t.Helper()
	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("kyc"),
		postgres.WithUsername("kyc"),
		postgres.WithPassword("kyc"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("postgres: %v", err)
	}
	t.Cleanup(func() {
		cctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = container.Terminate(cctx)
	})
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	db, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(db.Close)
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func doJSON(t *testing.T, h http.Handler, method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(dst); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
}
