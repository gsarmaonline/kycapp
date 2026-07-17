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
