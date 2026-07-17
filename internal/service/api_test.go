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

func TestSignupIdempotencyAndMultiOrg(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	svc := service.New(db)
	srv := httpserver.New(db, httpserver.Options{Service: svc})
	h := srv.Handler()

	body := map[string]any{
		"user":         map[string]string{"email": "ada@acme.com", "name": "Ada"},
		"organisation": map[string]string{"name": "Acme", "slug": "acme"},
		"plan_key":     "trial",
	}

	res1 := doJSON(t, h, http.MethodPost, "/v1/signup", body, map[string]string{
		"Idempotency-Key": "signup-1",
	})
	if res1.Code != http.StatusCreated {
		t.Fatalf("signup status=%d body=%s", res1.Code, res1.Body.String())
	}
	var first map[string]any
	decodeBody(t, res1, &first)
	userID := first["user"].(map[string]any)["id"].(string)
	org1 := first["organisation"].(map[string]any)["id"].(string)

	// Idempotent replay
	res2 := doJSON(t, h, http.MethodPost, "/v1/signup", body, map[string]string{
		"Idempotency-Key": "signup-1",
	})
	if res2.Code != http.StatusCreated {
		t.Fatalf("replay status=%d body=%s", res2.Code, res2.Body.String())
	}
	var second map[string]any
	decodeBody(t, res2, &second)
	if second["organisation"].(map[string]any)["id"] != org1 {
		t.Fatal("idempotent signup should return same organisation")
	}

	// Same key, different body → conflict
	bad := map[string]any{
		"user":         map[string]string{"email": "ada@acme.com", "name": "Ada"},
		"organisation": map[string]string{"name": "Other", "slug": "other"},
	}
	res3 := doJSON(t, h, http.MethodPost, "/v1/signup", bad, map[string]string{
		"Idempotency-Key": "signup-1",
	})
	if res3.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", res3.Code, res3.Body.String())
	}

	// Second org for same user
	body2 := map[string]any{
		"user":         map[string]string{"email": "ada@acme.com", "name": "Ada"},
		"organisation": map[string]string{"name": "Beta", "slug": "beta"},
	}
	res4 := doJSON(t, h, http.MethodPost, "/v1/signup", body2, map[string]string{
		"Idempotency-Key": "signup-2",
	})
	if res4.Code != http.StatusCreated {
		t.Fatalf("second org status=%d body=%s", res4.Code, res4.Body.String())
	}

	resMem := doJSON(t, h, http.MethodGet, "/v1/users/"+userID+"/memberships", nil, nil)
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
}

func TestInviteAcceptRevoke(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	svc := service.New(db)
	h := httpserver.New(db, httpserver.Options{Service: svc}).Handler()

	signup := doJSON(t, h, http.MethodPost, "/v1/signup", map[string]any{
		"user":         map[string]string{"email": "owner@acme.com", "name": "Owner"},
		"organisation": map[string]string{"name": "Acme", "slug": "acme-invite"},
	}, map[string]string{"Idempotency-Key": "invite-flow"})
	if signup.Code != http.StatusCreated {
		t.Fatalf("signup: %s", signup.Body.String())
	}
	var signed map[string]any
	decodeBody(t, signup, &signed)
	orgID := signed["organisation"].(map[string]any)["id"].(string)

	rolesRes := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/roles", nil, nil)
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
		"email":   "teammate@acme.com",
		"role_id": memberRoleID,
	}, nil)
	if inv.Code != http.StatusCreated {
		t.Fatalf("invite: %s", inv.Body.String())
	}
	var membership map[string]any
	decodeBody(t, inv, &membership)
	if membership["status"] != "invited" {
		t.Fatalf("status=%v", membership["status"])
	}
	mid := membership["id"].(string)

	acc := doJSON(t, h, http.MethodPost, "/v1/memberships/"+mid+"/accept", nil, nil)
	if acc.Code != http.StatusOK {
		t.Fatalf("accept: %s", acc.Body.String())
	}
	decodeBody(t, acc, &membership)
	if membership["status"] != "active" {
		t.Fatalf("after accept status=%v", membership["status"])
	}

	rev := doJSON(t, h, http.MethodDelete, "/v1/memberships/"+mid, nil, nil)
	if rev.Code != http.StatusOK {
		t.Fatalf("revoke: %s", rev.Body.String())
	}
	decodeBody(t, rev, &membership)
	if membership["status"] != "revoked" {
		t.Fatalf("after revoke status=%v", membership["status"])
	}
}

func TestAuthzCheckKeyAndResourceAction(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	svc := service.New(db)
	h := httpserver.New(db, httpserver.Options{Service: svc}).Handler()

	signup := doJSON(t, h, http.MethodPost, "/v1/signup", map[string]any{
		"user":         map[string]string{"email": "owner@acme.com", "name": "Owner"},
		"organisation": map[string]string{"name": "Acme", "slug": "acme-authz"},
	}, map[string]string{"Idempotency-Key": "authz-flow"})
	if signup.Code != http.StatusCreated {
		t.Fatalf("signup: %s", signup.Body.String())
	}
	var signed map[string]any
	decodeBody(t, signup, &signed)
	orgID := signed["organisation"].(map[string]any)["id"].(string)
	ownerID := signed["user"].(map[string]any)["id"].(string)

	// Owner allowed via permission key
	res := doJSON(t, h, http.MethodPost, "/v1/authz/check", map[string]any{
		"organisation_id": orgID,
		"user_id":         ownerID,
		"permission":      "members:invite",
	}, nil)
	if res.Code != http.StatusOK {
		t.Fatalf("check: %s", res.Body.String())
	}
	var check map[string]any
	decodeBody(t, res, &check)
	if check["allowed"] != true {
		t.Fatalf("owner should be allowed, got %#v", check)
	}

	// Same via resource+action
	res2 := doJSON(t, h, http.MethodPost, "/v1/authz/check", map[string]any{
		"organisation_id": orgID,
		"user_id":         ownerID,
		"resource":        "members",
		"action":          "invite",
	}, nil)
	decodeBody(t, res2, &check)
	if check["allowed"] != true {
		t.Fatalf("resource+action should allow, got %#v", check)
	}

	// Invite member (read-only role), accept, then check invite denied
	rolesRes := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/roles", nil, nil)
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
		"email":   "member@acme.com",
		"role_id": memberRoleID,
		"status":  "active",
	}, nil)
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
	}, nil)
	decodeBody(t, denied, &check)
	if check["allowed"] != false {
		t.Fatalf("member should be denied invite, got %#v", check)
	}

	// Suspend org → owner denied
	patch := doJSON(t, h, http.MethodPatch, "/v1/organisations/"+orgID, map[string]any{
		"status": "suspended",
	}, nil)
	if patch.Code != http.StatusOK {
		t.Fatalf("suspend: %s", patch.Body.String())
	}
	suspended := doJSON(t, h, http.MethodPost, "/v1/authz/check", map[string]any{
		"organisation_id": orgID,
		"user_id":         ownerID,
		"permission":      "members:invite",
	}, nil)
	decodeBody(t, suspended, &check)
	if check["allowed"] != false {
		t.Fatalf("suspended org should deny, got %#v", check)
	}

	// Owner permissions locked
	var ownerRoleID string
	for _, r := range roles.Items {
		if r["key"] == "owner" {
			ownerRoleID = r["id"].(string)
		}
	}
	lock := doJSON(t, h, http.MethodPatch, "/v1/roles/"+ownerRoleID, map[string]any{
		"permission_keys": []string{"members:read"},
	}, nil)
	if lock.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 changing owner perms, got %d %s", lock.Code, lock.Body.String())
	}
}

func TestEntitlementsEffectiveAndCheck(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	svc := service.New(db)
	h := httpserver.New(db, httpserver.Options{Service: svc}).Handler()

	signup := doJSON(t, h, http.MethodPost, "/v1/signup", map[string]any{
		"user":         map[string]string{"email": "bill@acme.com", "name": "Bill"},
		"organisation": map[string]string{"name": "Acme Billing", "slug": "acme-billing"},
	}, map[string]string{"Idempotency-Key": "billing-flow"})
	if signup.Code != http.StatusCreated {
		t.Fatalf("signup: %s", signup.Body.String())
	}
	var signed map[string]any
	decodeBody(t, signup, &signed)
	orgID := signed["organisation"].(map[string]any)["id"].(string)

	// Trial plan includes api_access only
	checkAPI := doJSON(t, h, http.MethodPost, "/v1/entitlements/check", map[string]any{
		"organisation_id": orgID,
		"entitlement":     "api_access",
	}, nil)
	var allowed map[string]any
	decodeBody(t, checkAPI, &allowed)
	if allowed["allowed"] != true {
		t.Fatalf("api_access should be allowed on trial: %#v", allowed)
	}
	checkSSO := doJSON(t, h, http.MethodPost, "/v1/entitlements/check", map[string]any{
		"organisation_id": orgID,
		"entitlement":     "sso",
	}, nil)
	decodeBody(t, checkSSO, &allowed)
	if allowed["allowed"] != false {
		t.Fatalf("sso should be denied on trial: %#v", allowed)
	}

	// Grant SSO override
	put := doJSON(t, h, http.MethodPut, "/v1/organisations/"+orgID+"/entitlements", map[string]any{
		"overrides": []map[string]string{{"key": "sso", "effect": "grant"}},
	}, nil)
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
		"organisation_id": orgID,
		"entitlement":     "sso",
	}, nil), &allowed)
	if allowed["allowed"] != true {
		t.Fatalf("sso should be granted: %#v", allowed)
	}

	// Deny api_access
	doJSON(t, h, http.MethodPut, "/v1/organisations/"+orgID+"/entitlements", map[string]any{
		"overrides": []map[string]string{
			{"key": "sso", "effect": "grant"},
			{"key": "api_access", "effect": "deny"},
		},
	}, nil)
	decodeBody(t, doJSON(t, h, http.MethodPost, "/v1/entitlements/check", map[string]any{
		"organisation_id": orgID,
		"entitlement":     "api_access",
	}, nil), &allowed)
	if allowed["allowed"] != false {
		t.Fatalf("api_access denied override failed: %#v", allowed)
	}

	// Create pro plan with both, assign subscription
	pro := doJSON(t, h, http.MethodPost, "/v1/plans", map[string]any{
		"key": "pro", "name": "Pro",
	}, nil)
	if pro.Code != http.StatusCreated {
		t.Fatalf("create plan: %s", pro.Body.String())
	}
	var plan map[string]any
	decodeBody(t, pro, &plan)
	planID := plan["id"].(string)
	setEnt := doJSON(t, h, http.MethodPut, "/v1/plans/"+planID+"/entitlements", map[string]any{
		"entitlement_keys": []string{"api_access", "sso"},
	}, nil)
	if setEnt.Code != http.StatusOK {
		t.Fatalf("set plan ents: %s", setEnt.Body.String())
	}
	// Clear overrides then upgrade
	doJSON(t, h, http.MethodPut, "/v1/organisations/"+orgID+"/entitlements", map[string]any{
		"overrides": []any{},
	}, nil)
	sub := doJSON(t, h, http.MethodPut, "/v1/organisations/"+orgID+"/subscription", map[string]any{
		"plan_id": planID,
		"status":  "active",
	}, nil)
	if sub.Code != http.StatusOK {
		t.Fatalf("subscription: %s", sub.Body.String())
	}
	decodeBody(t, doJSON(t, h, http.MethodPost, "/v1/entitlements/check", map[string]any{
		"organisation_id": orgID,
		"entitlement":     "sso",
	}, nil), &allowed)
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
